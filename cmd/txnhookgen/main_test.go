package main

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/tokenv2"
)

// TestTipSchedule freezes the deterministic tip schedule: every
// tiplessStride-th slot carries no tip (nil), the rest cycle
// (i % 8 + 1) * tipUnit, and maxTip is the schedule's largest value.
func TestTipSchedule(t *testing.T) {
	unit := tipUnit()
	wantUnit := new(big.Int).Div(big.NewInt(defaults.DEFAULT_PRICE), big.NewInt(100))
	if unit.Cmp(wantUnit) != 0 {
		t.Fatalf("tipUnit = %s, want %s", unit, wantUnit)
	}
	tipless := 0
	for i := 0; i < 6*tipVariants; i++ {
		got := tipForIndex(i)
		if i%tiplessStride == tiplessStride-1 {
			if got != nil {
				t.Fatalf("tipForIndex(%d) = %s, want nil (tipless slot)", i, got)
			}
			tipless++
			continue
		}
		want := new(big.Int).Mul(unit, big.NewInt(int64(i%tipVariants)+1))
		if got == nil || got.Cmp(want) != 0 {
			t.Fatalf("tipForIndex(%d) = %v, want %s", i, got, want)
		}
	}
	if tipless == 0 {
		t.Fatalf("schedule produced no tipless slots")
	}
	// The tipless stride must be co-prime with the per-sender transaction
	// stride (4: 2 coin + 2 token) so tipless slots rotate across kinds.
	if tiplessStride%2 == 0 {
		t.Fatalf("tiplessStride %d must be co-prime with the per-sender stride of 4", tiplessStride)
	}
	wantMax := new(big.Int).Mul(unit, big.NewInt(tipVariants))
	if got := maxTip(); got.Cmp(wantMax) != 0 {
		t.Fatalf("maxTip = %s, want %s", got, wantMax)
	}
}

// TestTipsAllowed verifies the GasV3 tip gate: tips are only embedded when
// startBlock is at/after GasV3StartBlock of the loaded config.
func TestTipsAllowed(t *testing.T) {
	fork := defaults.DefaultConfig.PosConfig.GasV3StartBlock
	if tipsAllowed(fork - 1) {
		t.Fatalf("tipsAllowed(%d) below GasV3StartBlock %d must be false", fork-1, fork)
	}
	if !tipsAllowed(fork) {
		t.Fatalf("tipsAllowed(%d) at GasV3StartBlock must be true", fork)
	}
	if !tipsAllowed(fork + 100) {
		t.Fatalf("tipsAllowed(%d) above GasV3StartBlock must be true", fork+100)
	}
}

// TestBuildBatchPattern verifies the doubling schedule.
func TestBuildBatchPattern(t *testing.T) {
	pattern := buildBatchPattern(4)
	if len(pattern) != 4 {
		t.Fatalf("pattern length %d, want 4", len(pattern))
	}
	for i, spec := range pattern {
		wantBatch := int64(i + 1)
		wantFunded := 1 << uint(i+1)
		if spec.batchNumber != wantBatch || spec.funded != wantFunded {
			t.Fatalf("pattern[%d] = {%d, %d}, want {%d, %d}", i, spec.batchNumber, spec.funded, wantBatch, wantFunded)
		}
	}
}

// TestComputeNeeds verifies the bottom-up coin funding recurrence including the
// per-sender gas reservation (with and without token transfers).
func TestComputeNeeds(t *testing.T) {
	feeCapMax := big.NewInt(1000)
	for _, tokens := range []bool{false, true} {
		need := computeNeeds(3, tokens, feeCapMax)
		if need[3].Cmp(coins(leafCoins)) != 0 {
			t.Fatalf("tokens=%v: leaf need = %s, want %s", tokens, need[3], coins(leafCoins))
		}
		gas := senderGasWei(tokens, feeCapMax)
		wantGasUnits := uint64(2 * params.TxGas)
		if tokens {
			wantGasUnits += 2 * tokenTransferGasLimit
		}
		wantGas := new(big.Int).Mul(new(big.Int).SetUint64(wantGasUnits), feeCapMax)
		if gas.Cmp(wantGas) != 0 {
			t.Fatalf("tokens=%v: senderGasWei = %s, want %s", tokens, gas, wantGas)
		}
		for i := 2; i >= 1; i-- {
			want := new(big.Int).Add(new(big.Int).Mul(need[i+1], big.NewInt(2)), gas)
			if need[i].Cmp(want) != 0 {
				t.Fatalf("tokens=%v: need[%d] = %s, want %s", tokens, i, need[i], want)
			}
		}
	}
}

// TestComputeTokenNeeds verifies token amounts double up the tree and that the
// deployed totalSupply (2*needTok[1]) equals the sum over all leaves, so full
// distribution conserves supply with intermediates netting to zero.
func TestComputeTokenNeeds(t *testing.T) {
	levels := 5
	needTok := computeTokenNeeds(levels)
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenDecimals)), nil)
	wantLeaf := new(big.Int).Mul(big.NewInt(leafTokens), unit)
	if needTok[levels].Cmp(wantLeaf) != 0 {
		t.Fatalf("leaf tokens = %s, want %s", needTok[levels], wantLeaf)
	}
	totalSupply := new(big.Int).Mul(needTok[1], big.NewInt(2))
	leafSum := new(big.Int).Mul(wantLeaf, big.NewInt(1<<uint(levels)))
	if totalSupply.Cmp(leafSum) != 0 {
		t.Fatalf("totalSupply %s != leaf sum %s", totalSupply, leafSum)
	}
}

// TestBuildAndSignKinds builds one transaction of each kind through the real
// generator and asserts the offline classification (core.IsBasicTransfer), the
// tip caps, and pool-level fee cap validity (core.ValidateGasFeeCaps).
func TestBuildAndSignKinds(t *testing.T) {
	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	signingCtx := cryptobase.GetSigningContext()

	gen := &generator{
		signer:     types.LatestSignerForChainID(chainID),
		signingCtx: signingCtx,
		tipMode:    tipModeMixed,
		baseFee:    baseFeeForContext(signingCtx),
	}

	contractAddr := crypto.CreateAddress(addr, 0)
	parsedABI, err := tokenv2.Tokenv2MetaData.GetAbi()
	if err != nil {
		t.Fatalf("tokenv2 abi: %v", err)
	}
	transferData, err := parsedABI.Pack("transfer", addr, big.NewInt(1))
	if err != nil {
		t.Fatalf("pack transfer: %v", err)
	}

	codeSizeFn := func(a common.Address) int {
		if a == contractAddr {
			return 1
		}
		return 0
	}

	decode := func(hexStr string) *types.Transaction {
		t.Helper()
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(common.FromHex(hexStr)); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return tx
	}

	// Deploy (nil to) -> general; index 0 tip = 1 unit.
	deployHex, deployTip, err := gen.buildAndSign(key, 0, nil, big.NewInt(0), tokenDeployGasLimit, []byte{0x60, 0x80})
	if err != nil {
		t.Fatalf("sign deploy: %v", err)
	}
	// Coin transfer -> basic; index 1 tip = 2 units.
	coinHex, coinTip, err := gen.buildAndSign(key, 1, &addr, coins(1), params.TxGas, nil)
	if err != nil {
		t.Fatalf("sign coin: %v", err)
	}
	// Token transfer -> general; index 2 is a TIPLESS schedule slot.
	tokenHex, tokenTip, err := gen.buildAndSign(key, 2, &contractAddr, big.NewInt(0), tokenTransferGasLimit, transferData)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	// Second token transfer; index 3 tip = 4 units (tipped general txn).
	token2Hex, token2Tip, err := gen.buildAndSign(key, 3, &contractAddr, big.NewInt(0), tokenTransferGasLimit, transferData)
	if err != nil {
		t.Fatalf("sign token2: %v", err)
	}

	deployTx, coinTx, tokenTx, token2Tx := decode(deployHex), decode(coinHex), decode(tokenHex), decode(token2Hex)

	if core.IsBasicTransfer(deployTx, codeSizeFn) {
		t.Fatalf("deploy tx must not be basic")
	}
	if !core.IsBasicTransfer(coinTx, codeSizeFn) {
		t.Fatalf("coin tx must be basic")
	}
	if core.IsBasicTransfer(tokenTx, codeSizeFn) || core.IsBasicTransfer(token2Tx, codeSizeFn) {
		t.Fatalf("token txns must not be basic")
	}

	unit := tipUnit()
	wantTips := []*big.Int{
		new(big.Int).Mul(unit, big.NewInt(1)), // index 0
		new(big.Int).Mul(unit, big.NewInt(2)), // index 1
		nil,                                   // index 2: tipless slot
		new(big.Int).Mul(unit, big.NewInt(4)), // index 3
	}
	gotTips := []*big.Int{deployTip, coinTip, tokenTip, token2Tip}
	for i, tx := range []*types.Transaction{deployTx, coinTx, tokenTx, token2Tx} {
		wantTip := wantTips[i]
		if (gotTips[i] == nil) != (wantTip == nil) || (wantTip != nil && gotTips[i].Cmp(wantTip) != 0) {
			t.Fatalf("tx %d: returned tip %v, want %v", i, gotTips[i], wantTip)
		}
		effTip, err := tx.EffectiveGasTip(tx.BaseFee())
		if err != nil {
			t.Fatalf("tx %d: effective tip: %v", i, err)
		}
		wantEff := new(big.Int)
		if wantTip != nil {
			wantEff = wantTip
		}
		if effTip.Cmp(wantEff) != 0 {
			t.Fatalf("tx %d: effective tip %s, want %s", i, effTip, wantEff)
		}
		if err := core.ValidateGasFeeCaps(tx, tx.BaseFee()); err != nil {
			t.Fatalf("tx %d: ValidateGasFeeCaps: %v", i, err)
		}
		sender, err := types.Sender(gen.signer, tx)
		if err != nil {
			t.Fatalf("tx %d: sender: %v", i, err)
		}
		if sender != addr {
			t.Fatalf("tx %d: recovered sender %s, want %s", i, sender.Hex(), addr.Hex())
		}
	}

	// Without tips the caps must stay nil (zero effective tip).
	genNoTips := &generator{signer: gen.signer, signingCtx: signingCtx, tipMode: tipModeNone, baseFee: gen.baseFee}
	plainHex, plainTip, err := genNoTips.buildAndSign(key, 4, &addr, coins(1), params.TxGas, nil)
	if err != nil {
		t.Fatalf("sign plain: %v", err)
	}
	if plainTip != nil {
		t.Fatalf("tips-disabled generator returned tip %s, want nil", plainTip)
	}
	plainTx := decode(plainHex)
	effTip, err := plainTx.EffectiveGasTip(plainTx.BaseFee())
	if err != nil {
		t.Fatalf("plain effective tip: %v", err)
	}
	if effTip.Sign() != 0 {
		t.Fatalf("plain tx effective tip %s, want 0", effTip)
	}
}

// TestAscendingTipSchedule freezes the -tipsascending schedule: every index is
// tipped, tips are strictly increasing (pairwise distinct), and maxTipForMode
// matches the last index.
func TestAscendingTipSchedule(t *testing.T) {
	unit := ascTipUnit()
	wantUnit := new(big.Int).Div(big.NewInt(defaults.DEFAULT_PRICE), big.NewInt(10000))
	if unit.Cmp(wantUnit) != 0 {
		t.Fatalf("ascTipUnit = %s, want %s", unit, wantUnit)
	}
	prev := new(big.Int)
	for i := 0; i < 100; i++ {
		got := ascendingTipForIndex(i)
		if got == nil {
			t.Fatalf("ascendingTipForIndex(%d) = nil; ascending mode must tip every transaction", i)
		}
		if got.Cmp(prev) <= 0 {
			t.Fatalf("ascendingTipForIndex(%d) = %s not strictly greater than previous %s", i, got, prev)
		}
		want := new(big.Int).Mul(unit, big.NewInt(int64(i)+1))
		if got.Cmp(want) != 0 {
			t.Fatalf("ascendingTipForIndex(%d) = %s, want %s", i, got, want)
		}
		prev = got
	}
	if got, want := maxTipForMode(tipModeAscending, 100), ascendingTipForIndex(99); got.Cmp(want) != 0 {
		t.Fatalf("maxTipForMode(ascending, 100) = %s, want %s", got, want)
	}
	if got := maxTipForMode(tipModeMixed, 100); got.Cmp(maxTip()) != 0 {
		t.Fatalf("maxTipForMode(mixed) = %s, want %s", got, maxTip())
	}
}

// TestBuildAndSignAscending verifies the generator embeds the strictly
// increasing schedule in ascending mode.
func TestBuildAndSignAscending(t *testing.T) {
	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	signingCtx := cryptobase.GetSigningContext()
	gen := &generator{
		signer:     types.LatestSignerForChainID(chainID),
		signingCtx: signingCtx,
		tipMode:    tipModeAscending,
		baseFee:    baseFeeForContext(signingCtx),
	}
	unit := ascTipUnit()
	for i := 0; i < 3; i++ {
		hexStr, tip, err := gen.buildAndSign(key, uint64(i), &addr, coins(1), params.TxGas, nil)
		if err != nil {
			t.Fatalf("sign %d: %v", i, err)
		}
		want := new(big.Int).Mul(unit, big.NewInt(int64(i)+1))
		if tip == nil || tip.Cmp(want) != 0 {
			t.Fatalf("tx %d: tip %v, want %s", i, tip, want)
		}
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(common.FromHex(hexStr)); err != nil {
			t.Fatalf("unmarshal %d: %v", i, err)
		}
		effTip, err := tx.EffectiveGasTip(tx.BaseFee())
		if err != nil {
			t.Fatalf("effective tip %d: %v", i, err)
		}
		if effTip.Cmp(want) != 0 {
			t.Fatalf("tx %d: effective tip %s, want %s", i, effTip, want)
		}
		if err := core.ValidateGasFeeCaps(tx, tx.BaseFee()); err != nil {
			t.Fatalf("tx %d: ValidateGasFeeCaps: %v", i, err)
		}
	}
}

// TestAscendingExclusionViolations covers the strict verify check: a higher tip
// in a strictly later block is a violation; same-sender pairs and equal blocks
// are not.
func TestAscendingExclusionViolations(t *testing.T) {
	s1 := common.BytesToAddress([]byte{1})
	s2 := common.BytesToAddress([]byte{2})
	mk := func(sender common.Address, tip int64, block uint64) *vTx {
		return &vTx{sender: sender, effTip: big.NewInt(tip), block: block, committed: true}
	}

	// Correct ascending outcome: higher tips in the earlier (or same) block.
	if n, _ := ascendingExclusionViolations([]*vTx{mk(s1, 10, 5), mk(s2, 20, 5), mk(s1, 5, 6)}); n != 0 {
		t.Fatalf("expected 0 violations, got %d", n)
	}
	// Violation: s1's tip 30 landed later than s2's tip 10.
	n, example := ascendingExclusionViolations([]*vTx{mk(s2, 10, 5), mk(s1, 30, 6)})
	if n != 1 || example == "" {
		t.Fatalf("expected 1 violation with example, got %d (%q)", n, example)
	}
	// Same-sender pairs are exempt (nonce chains dominate within a sender).
	if n, _ := ascendingExclusionViolations([]*vTx{mk(s1, 10, 5), mk(s1, 30, 6)}); n != 0 {
		t.Fatalf("same-sender pair must not count, got %d", n)
	}
}

// TestReplayTwoPass mirrors the semantics of core/state_processor.go
// ProcessTransactions over synthetic block shapes (cf. core/state_processor_test.go
// TestProcessTransactionsBasicOverflowToGeneral / GeneralIsolation).
func TestReplayTwoPass(t *testing.T) {
	a := common.BytesToAddress([]byte{1})
	b := common.BytesToAddress([]byte{2})
	c := common.BytesToAddress([]byte{3})
	basic := func(sender common.Address, gas, used uint64) replayTx {
		return replayTx{sender: sender, gas: gas, gasUsed: used, basic: true, label: "b"}
	}
	general := func(sender common.Address, gas, used uint64) replayTx {
		return replayTx{sender: sender, gas: gas, gasUsed: used, basic: false, label: "g"}
	}

	tests := []struct {
		name         string
		gasLimit     uint64
		txs          []replayTx
		wantBasic    uint64
		wantGeneral  uint64
		wantProblems int
	}{
		{
			name:      "basic only fits basic pool",
			gasLimit:  84000, // basic budget 42000
			txs:       []replayTx{basic(a, 21000, 21000), basic(b, 21000, 21000)},
			wantBasic: 42000, wantGeneral: 0,
		},
		{
			name:      "basic overflow spills to general",
			gasLimit:  84000,
			txs:       []replayTx{basic(a, 21000, 21000), basic(b, 21000, 21000), basic(c, 21000, 21000)},
			wantBasic: 42000, wantGeneral: 21000,
		},
		{
			name:      "general isolation: basic pool never lent",
			gasLimit:  84000,
			txs:       []replayTx{general(a, 21000, 21000), general(b, 21000, 21000)},
			wantBasic: 0, wantGeneral: 42000,
		},
		{
			name:     "non-basic blocks the account for later nonces",
			gasLimit: 200000,
			// a's first tx is general, so a's following basic tx must also run
			// in the general pool (account blocked), like state_processor does.
			txs:       []replayTx{general(a, 21000, 21000), basic(a, 21000, 21000), basic(b, 21000, 21000)},
			wantBasic: 21000, wantGeneral: 42000,
		},
		{
			name:     "refund semantics: limit gates entry, gasUsed is consumed",
			gasLimit: 200000, // basic budget 100000
			// Two 60k-limit basic txs that each use only 30k: the second still
			// fits because the first refunded 30k back to the pool.
			txs:       []replayTx{basic(a, 60000, 30000), basic(b, 60000, 30000)},
			wantBasic: 60000, wantGeneral: 0,
		},
		{
			name:      "general pool exhaustion is a problem",
			gasLimit:  84000, // general budget 42000
			txs:       []replayTx{general(a, 30000, 30000), general(b, 30000, 30000)},
			wantBasic: 0, wantGeneral: 30000, wantProblems: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basicUsed, generalUsed, problems := replayTwoPass(tt.txs, tt.gasLimit)
			if basicUsed != tt.wantBasic || generalUsed != tt.wantGeneral {
				t.Fatalf("replay = basic %d / general %d, want %d / %d", basicUsed, generalUsed, tt.wantBasic, tt.wantGeneral)
			}
			if len(problems) != tt.wantProblems {
				t.Fatalf("problems = %v, want %d", problems, tt.wantProblems)
			}
		})
	}
}

// TestContractAddressDerivation freezes the deploy-address derivation the
// generator and verifier both rely on (crypto.CreateAddress of sender+nonce).
func TestContractAddressDerivation(t *testing.T) {
	addr := common.BytesToAddress([]byte{0xaa})
	a1 := crypto.CreateAddress(addr, 7)
	a2 := crypto.CreateAddress(addr, 7)
	if a1 != a2 {
		t.Fatalf("CreateAddress not deterministic: %s vs %s", a1.Hex(), a2.Hex())
	}
	if a1 == crypto.CreateAddress(addr, 8) {
		t.Fatalf("CreateAddress must differ across nonces")
	}
}
