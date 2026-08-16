// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/accounts/keystore"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/tokenv2"
)

const (
	// tokenDeployGasLimit is the gas limit for the tokenv2 contract deployment
	// transaction (matches the cmd/dputil createtoken precedent).
	tokenDeployGasLimit = uint64(1500000)

	// tokenTransferGasLimit is the gas limit for a tokenv2 transfer(to, value)
	// call (matches the cmd/dputil transfertokens precedent; actual usage is
	// ~51k, the surplus is refunded at the effective gas price).
	tokenTransferGasLimit = uint64(100000)

	// leafTokens is the number of whole tokens each leaf wallet ends up with
	// when -tokens is enabled.
	leafTokens = 1000

	// tokenDecimals is the decimals value the tokenv2 contract is deployed with.
	tokenDecimals = uint8(18)

	// tipVariants is the size of the deterministic tip schedule: transaction i
	// carries a tip of (i % tipVariants + 1) tip units, so effective-tip
	// ordering by the proposer (core/gastip.go SelectByEffectiveTip) is
	// observable in which blocks transactions land in.
	tipVariants = 8

	// tiplessStride makes every tiplessStride-th transaction carry NO tip even
	// in -tips mode (nil caps, the legacy opt-out), so blocks contain a
	// tipped/tipless mix and selection can be observed preferring tipped
	// transactions under contention. The stride is co-prime with the 4 (5 for
	// the root) transactions each sender emits, so tipless slots rotate across
	// both coin and token transactions rather than pinning to one kind.
	tiplessStride = 3
)

// tipMode selects the tip schedule for a generation run.
type tipMode int

const (
	// tipModeNone: no caps on any transaction (legacy opt-out).
	tipModeNone tipMode = iota
	// tipModeMixed (-tips): varied tips with periodic tipless slots, so blocks
	// carry a tipped/tipless mix.
	tipModeMixed
	// tipModeAscending (-tipsascending): every transaction is tipped and the
	// tip strictly increases with generation order, so under pool contention
	// the highest-tip transactions must be included first and the lowest-tip
	// ones excluded to later blocks — verify enforces this strictly.
	tipModeAscending
)

// tipUnit is the tip step in wei: 1% of DEFAULT_PRICE, i.e. 10% of the
// DynamicFeeTx base fee (DEFAULT_PRICE/10). Tips therefore range from 10% to
// 80% of the base fee — large enough to observe, cheap enough to fund.
func tipUnit() *big.Int {
	return new(big.Int).Div(big.NewInt(defaults.DEFAULT_PRICE), big.NewInt(100))
}

// tipForIndex returns the deterministic tip (GasTipCap) for the i-th generated
// transaction: nil (no tip — legacy opt-out) on every tiplessStride-th slot,
// otherwise (i % tipVariants + 1) tip units.
func tipForIndex(i int) *big.Int {
	if i%tiplessStride == tiplessStride-1 {
		return nil
	}
	steps := int64(i%tipVariants) + 1
	return new(big.Int).Mul(tipUnit(), big.NewInt(steps))
}

// maxTip is the largest tip tipForIndex can produce; used for conservative
// funding math.
func maxTip() *big.Int {
	return new(big.Int).Mul(tipUnit(), big.NewInt(tipVariants))
}

// ascTipUnit is the tip step for tipModeAscending: 0.01% of DEFAULT_PRICE (1%
// of the DynamicFeeTx base fee), 100x smaller than tipUnit so that even
// thousands of strictly increasing tips stay affordable for the funding tree.
func ascTipUnit() *big.Int {
	return new(big.Int).Div(big.NewInt(defaults.DEFAULT_PRICE), big.NewInt(10000))
}

// ascendingTipForIndex returns the tip for the i-th transaction in
// tipModeAscending: (i+1) ascending tip units — strictly increasing and
// pairwise distinct across the whole run, never nil.
func ascendingTipForIndex(i int) *big.Int {
	return new(big.Int).Mul(ascTipUnit(), big.NewInt(int64(i)+1))
}

// maxTipForMode returns the largest tip the given mode's schedule can produce
// over totalTxns transactions; used for conservative funding math.
func maxTipForMode(mode tipMode, totalTxns int) *big.Int {
	switch mode {
	case tipModeMixed:
		return maxTip()
	case tipModeAscending:
		return ascendingTipForIndex(totalTxns - 1)
	default:
		return new(big.Int)
	}
}

// tipsAllowed implements the tip gate: tips are only embedded when the hook's
// start block is at/after GasV3StartBlock of the loaded config (the fork where
// DefaultGasLimitV2 activates). Requirement: tips only above that fork.
func tipsAllowed(startBlock uint64) bool {
	return startBlock >= defaults.DefaultConfig.PosConfig.GasV3StartBlock
}

// baseFeeForContext returns the fixed DynamicFeeTx base fee per gas for the
// given signing context. It is deterministic (defaults.DEFAULT_PRICE/10 scaled
// by the signing-context multiplier), so it can be computed offline; deriving
// it from a probe transaction's GasPrice avoids duplicating types.calcGasFee.
func baseFeeForContext(signingCtx crypto.SigningContext) *big.Int {
	var to common.Address
	probe := types.NewDynamicFeeTransaction(chainID, 0, &to, big.NewInt(0), params.TxGas, signingCtx, nil, nil)
	return probe.GasPrice()
}

// genConfig carries the generate-mode flag values.
type genConfig struct {
	levels        int
	startNonce    uint64
	parallelism   int
	startBlock    uint64
	tokens        bool
	tips          bool
	tipsAscending bool
}

// batchSpec is one entry of the funding pattern: the BatchNumber written into
// the hook file and the number of wallets funded in that batch.
type batchSpec struct {
	batchNumber int64
	funded      int
}

// buildBatchPattern generates a doubling schedule for the given number of
// levels: batch i (1-indexed) funds 2^i wallets, each sender funding two
// children. The batch number equals the level. With -tokens each batch
// additionally carries one token transfer per funded wallet (and batch 1
// carries the deploy), but `funded` always counts wallets, not transactions.
func buildBatchPattern(levels int) []batchSpec {
	pattern := make([]batchSpec, levels)
	for i := 1; i <= levels; i++ {
		pattern[i-1] = batchSpec{batchNumber: int64(i), funded: 1 << uint(i)}
	}
	return pattern
}

// senderGasWei returns the wei a single sender must hold to cover the gas of
// all its outgoing transactions (2 coin transfers, plus 2 token transfers when
// tokens is set), reserved at the fee-cap rate. The pool admission check
// (core Transaction.Cost) reserves max(baseFee, feeCap) * gasLimit + value per
// transaction, so budgeting at feeCapMax is conservative and the unused
// portion returns as surplus.
func senderGasWei(tokens bool, feeCapMax *big.Int) *big.Int {
	gas := new(big.Int).SetUint64(2 * params.TxGas)
	if tokens {
		gas.Add(gas, new(big.Int).SetUint64(2*tokenTransferGasLimit))
	}
	return gas.Mul(gas, feeCapMax)
}

// computeNeeds returns the coin amount (wei) a wallet at each level must
// receive: need[levels] = leafCoins; need[i] = 2*need[i+1] + senderGas.
// Index 0 is unused (the root is handled separately by rootRequiredWei).
func computeNeeds(levels int, tokens bool, feeCapMax *big.Int) []*big.Int {
	need := make([]*big.Int, levels+1)
	need[levels] = coins(leafCoins)
	gasWei := senderGasWei(tokens, feeCapMax)
	for i := levels - 1; i >= 1; i-- {
		need[i] = new(big.Int).Add(new(big.Int).Mul(need[i+1], big.NewInt(2)), gasWei)
	}
	return need
}

// computeTokenNeeds returns the token amount (in base units, 10^tokenDecimals)
// a wallet at each level must receive: needTok[levels] = leafTokens whole
// tokens; needTok[i] = 2*needTok[i+1] (token transfers cost no tokens).
func computeTokenNeeds(levels int) []*big.Int {
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tokenDecimals)), nil)
	needTok := make([]*big.Int, levels+1)
	needTok[levels] = new(big.Int).Mul(big.NewInt(leafTokens), unit)
	for i := levels - 1; i >= 1; i-- {
		needTok[i] = new(big.Int).Mul(needTok[i+1], big.NewInt(2))
	}
	return needTok
}

// rootRequiredWei returns the approximate coin balance the root must hold:
// two child transfers plus the root's own gas (including the deploy when
// tokens is set).
func rootRequiredWei(need1 *big.Int, tokens bool, feeCapMax *big.Int) *big.Int {
	total := new(big.Int).Mul(need1, big.NewInt(2))
	total.Add(total, senderGasWei(tokens, feeCapMax))
	if tokens {
		deployGas := new(big.Int).SetUint64(tokenDeployGasLimit)
		total.Add(total, deployGas.Mul(deployGas, feeCapMax))
	}
	return total
}

// generator carries the shared signing state for one run.
type generator struct {
	signer     types.Signer
	signingCtx crypto.SigningContext
	tipMode    tipMode
	baseFee    *big.Int
	txIndex    int // global index driving the deterministic tip schedule
}

// tipModeName renders a tipMode for logs and the summary.
func tipModeName(m tipMode) string {
	switch m {
	case tipModeMixed:
		return "mixed"
	case tipModeAscending:
		return "ascending"
	default:
		return "none"
	}
}

// buildAndSign constructs, signs, and hex-encodes one transaction. When tips
// are enabled it uses NewDynamicFeeTransactionWithCaps with tipCap from the
// deterministic schedule and feeCap = baseFee + tipCap (satisfying
// core.ValidateGasFeeCaps: feeCap >= baseFee, tipCap <= feeCap); on tipless
// schedule slots — and always when tips are disabled — the caps stay nil (the
// legacy opt-out, zero tip). The returned tip is nil for tipless transactions.
func (g *generator) buildAndSign(sender *signaturealgorithm.PrivateKey, nonce uint64, to *common.Address, value *big.Int, gasLimit uint64, data []byte) (string, *big.Int, error) {
	var tx *types.Transaction
	var tip *big.Int
	switch g.tipMode {
	case tipModeMixed:
		tip = tipForIndex(g.txIndex)
	case tipModeAscending:
		tip = ascendingTipForIndex(g.txIndex)
	}
	if tip != nil {
		feeCap := new(big.Int).Add(g.baseFee, tip)
		tx = types.NewDynamicFeeTransactionWithCaps(chainID, nonce, to, value, gasLimit, g.signingCtx, data, nil, tip, feeCap)
	} else {
		tx = types.NewDynamicFeeTransaction(chainID, nonce, to, value, gasLimit, g.signingCtx, data, nil)
	}
	g.txIndex++
	signed, err := types.SignTx(tx, g.signer, sender)
	if err != nil {
		return "", nil, err
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return "", nil, err
	}
	return "0x" + hex.EncodeToString(raw), tip, nil
}

// tipMixCounts tracks how many transactions of each kind carry a tip, so the
// generator can prove the file contains a tipped/tipless mix of both coin and
// non-coin (token/deploy) transactions.
type tipMixCounts struct {
	tippedCoin     int
	tiplessCoin    int
	tippedGeneral  int
	tiplessGeneral int
}

func (c *tipMixCounts) add(isCoin bool, tip *big.Int) {
	switch {
	case isCoin && tip != nil:
		c.tippedCoin++
	case isCoin:
		c.tiplessCoin++
	case tip != nil:
		c.tippedGeneral++
	default:
		c.tiplessGeneral++
	}
}

func run(inputWalletPath, outputPath, password string, cfg genConfig) error {
	logf("Loading root wallet from %s", inputWalletPath)
	rootKey, err := loadRootKey(inputWalletPath, password)
	if err != nil {
		return fmt.Errorf("failed to load root wallet: %w", err)
	}
	rootAddr := cryptobase.SigAlg.PublicKeyToAddressNoError(&rootKey.PublicKey)
	logf("Root wallet loaded: %s", rootAddr.Hex())

	// Tip gate: only embed caps at/after GasV3StartBlock (see the runbook in
	// main.go). The node-side pool would accept tips from GasTipStartBlock, but
	// the requirement is the stricter GasV3 bound.
	mode := tipModeNone
	if cfg.tips {
		mode = tipModeMixed
	}
	if cfg.tipsAscending {
		mode = tipModeAscending
	}
	if mode != tipModeNone && !tipsAllowed(cfg.startBlock) {
		logf("WARNING: tips ignored: startBlock %d is below GasV3StartBlock %d of the loaded config; generating without tips",
			cfg.startBlock, defaults.DefaultConfig.PosConfig.GasV3StartBlock)
		mode = tipModeNone
	}

	batchPattern := buildBatchPattern(cfg.levels)
	totalTxns := 0
	for _, spec := range batchPattern {
		totalTxns += spec.funded
	}
	if cfg.tokens {
		totalTxns = totalTxns*2 + 1 // one token transfer per funded wallet + deploy
	}

	signingCtx := cryptobase.GetSigningContext()
	gen := &generator{
		signer:     types.LatestSignerForChainID(chainID),
		signingCtx: signingCtx,
		tipMode:    mode,
		baseFee:    baseFeeForContext(signingCtx),
	}

	// feeCapMax is the funding-math reservation rate: base fee plus the largest
	// scheduled tip when tips are on, plain base fee otherwise.
	feeCapMax := new(big.Int).Set(gen.baseFee)
	if mode != tipModeNone {
		feeCapMax.Add(feeCapMax, maxTipForMode(mode, totalTxns))
	}

	need := computeNeeds(cfg.levels, cfg.tokens, feeCapMax)

	// Token setup: deploy calldata, deterministic contract address, and the
	// per-level token amounts. The deploy is the root's first transaction, so
	// the contract address derives from (rootAddr, startNonce).
	var (
		needTok      []*big.Int
		contractAddr common.Address
		deployData   []byte
		totalSupply  *big.Int
	)
	if cfg.tokens {
		needTok = computeTokenNeeds(cfg.levels)
		totalSupply = new(big.Int).Mul(needTok[1], big.NewInt(2))
		parsedABI, err := tokenv2.Tokenv2MetaData.GetAbi()
		if err != nil {
			return fmt.Errorf("tokenv2 ABI: %w", err)
		}
		ctorArgs, err := parsedABI.Pack("", "HookToken", "HOOK", totalSupply, tokenDecimals, rootAddr)
		if err != nil {
			return fmt.Errorf("pack tokenv2 constructor: %w", err)
		}
		deployData = append(common.FromHex(tokenv2.Tokenv2MetaData.Bin), ctorArgs...)
		contractAddr = crypto.CreateAddress(rootAddr, cfg.startNonce)
		logf("Token enabled: contract address %s (deploy nonce %d), totalSupply %s base units", contractAddr.Hex(), cfg.startNonce, totalSupply.String())
	}

	logf("Generating %d transactions across %d batches (startBlockNumber=%d, tokens=%v, tipMode=%s)",
		totalTxns, len(batchPattern), cfg.startBlock, cfg.tokens, tipModeName(mode))

	txns := make([]core.TxnTestTransaction, 0, totalTxns)
	appendTx := func(batch int64, hexStr string) {
		txns = append(txns, core.TxnTestTransaction{BatchNumber: batch, TxnHex: hexStr})
	}
	var mix tipMixCounts

	var parsedABI abiPacker
	if cfg.tokens {
		p, err := tokenv2.Tokenv2MetaData.GetAbi()
		if err != nil {
			return fmt.Errorf("tokenv2 ABI: %w", err)
		}
		parsedABI = p
	}

	// level 0 senders is just the root.
	senders := []*signaturealgorithm.PrivateKey{rootKey}
	for idx, spec := range batchPattern {
		level := idx + 1
		valuePerChild := need[level]
		logf("Batch %d/%d (batchNumber=%d): funding %d wallets, %s coins each",
			level, len(batchPattern), spec.batchNumber, spec.funded, weiToCoins(valuePerChild))

		newWallets := make([]*signaturealgorithm.PrivateKey, 0, spec.funded)
		// The root is the only sender of batch 1 (level 1) and is a reused,
		// pre-funded account, so honor startNonce. Every other sender is a
		// freshly created wallet whose on-chain nonce is 0.
		isRootBatch := idx == 0

		for _, sender := range senders {
			baseNonce := uint64(0)
			if isRootBatch {
				baseNonce = cfg.startNonce
			}

			// The deploy is the root's first transaction; per-account nonce
			// ordering guarantees it executes before the root's token
			// transfers, and batch sequencing (the hook waits for each batch
			// to commit) guarantees it precedes every descendant's transfers.
			if cfg.tokens && isRootBatch {
				hexStr, tip, err := gen.buildAndSign(sender, baseNonce, nil, big.NewInt(0), tokenDeployGasLimit, deployData)
				if err != nil {
					return fmt.Errorf("failed to sign deploy tx: %w", err)
				}
				mix.add(false, tip)
				appendTx(spec.batchNumber, hexStr)
				baseNonce++
			}

			// Each sender funds exactly two children with coin transfers
			// (consecutive nonces), then optionally hands each child its token
			// tranche. Coin transfers are "basic" for the 50/50 gas split
			// (empty calldata, fresh EOA recipient); token transfers are
			// "general" (calldata + contract recipient). See core/gastip.go
			// IsBasicTransfer.
			childAddrs := make([]common.Address, 0, 2)
			for j := uint64(0); j < 2; j++ {
				childKey, err := cryptobase.SigAlg.GenerateKey()
				if err != nil {
					return fmt.Errorf("failed to generate wallet: %w", err)
				}
				childAddr := cryptobase.SigAlg.PublicKeyToAddressNoError(&childKey.PublicKey)

				hexStr, tip, err := gen.buildAndSign(sender, baseNonce+j, &childAddr, valuePerChild, params.TxGas, nil)
				if err != nil {
					return fmt.Errorf("failed to sign tx (batch %d): %w", spec.batchNumber, err)
				}
				mix.add(true, tip)
				appendTx(spec.batchNumber, hexStr)
				newWallets = append(newWallets, childKey)
				childAddrs = append(childAddrs, childAddr)
			}

			if cfg.tokens {
				for j, childAddr := range childAddrs {
					transferData, err := parsedABI.Pack("transfer", childAddr, needTok[level])
					if err != nil {
						return fmt.Errorf("pack token transfer: %w", err)
					}
					hexStr, tip, err := gen.buildAndSign(sender, baseNonce+2+uint64(j), &contractAddr, big.NewInt(0), tokenTransferGasLimit, transferData)
					if err != nil {
						return fmt.Errorf("failed to sign token tx (batch %d): %w", spec.batchNumber, err)
					}
					mix.add(false, tip)
					appendTx(spec.batchNumber, hexStr)
				}
			}
		}

		if len(newWallets) != spec.funded {
			return fmt.Errorf("internal error: batch %d produced %d wallets, expected %d", spec.batchNumber, len(newWallets), spec.funded)
		}
		logf("  batch %d complete (overall %d/%d transactions signed)", spec.batchNumber, len(txns), totalTxns)
		senders = newWallets
	}

	// Schedule sanity per mode. Mixed: some tipped and some tipless of every
	// kind present, so selection preference is observable and the legacy no-tip
	// path stays covered. Ascending: every transaction must be tipped (strictly
	// increasing schedule), so verify can enforce strict highest-tip-first
	// inclusion under contention.
	switch mode {
	case tipModeMixed:
		logf("Tip mix: coin %d tipped / %d tipless; general (token+deploy) %d tipped / %d tipless",
			mix.tippedCoin, mix.tiplessCoin, mix.tippedGeneral, mix.tiplessGeneral)
		if mix.tippedCoin == 0 || mix.tiplessCoin == 0 {
			return fmt.Errorf("tip mix degenerate: coin txns %d tipped / %d tipless — need both (raise -levels)", mix.tippedCoin, mix.tiplessCoin)
		}
		if cfg.tokens && (mix.tippedGeneral == 0 || mix.tiplessGeneral == 0) {
			return fmt.Errorf("tip mix degenerate: general txns %d tipped / %d tipless — need both (raise -levels)", mix.tippedGeneral, mix.tiplessGeneral)
		}
	case tipModeAscending:
		logf("Ascending tips: coin %d tipped, general %d tipped, max tip %s wei",
			mix.tippedCoin, mix.tippedGeneral, maxTipForMode(mode, totalTxns).String())
		if mix.tiplessCoin != 0 || mix.tiplessGeneral != 0 {
			return fmt.Errorf("internal error: ascending mode produced tipless txns (%d coin / %d general)", mix.tiplessCoin, mix.tiplessGeneral)
		}
	}

	logf("Signing complete: %d transactions. Writing %s ...", len(txns), outputPath)

	out := core.TxnTestTransactions{
		StartBlockNumber: int64(cfg.startBlock),
		Parallelism:      cfg.parallelism,
		Transactions:     txns,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputPath, err)
	}

	printSummary(outputPath, cfg, batchPattern, need, len(txns), mode, contractAddr, feeCapMax)
	return nil
}

// abiPacker is the subset of abi.ABI used here (interface for testability).
type abiPacker interface {
	Pack(name string, args ...interface{}) ([]byte, error)
}

// loadRootKey decrypts the root wallet keystore at path using the password.
func loadRootKey(path, password string) (*signaturealgorithm.PrivateKey, error) {
	keyjson, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := keystore.DecryptKey(keyjson, password)
	if err != nil {
		return nil, err
	}
	return key.PrivateKey, nil
}

// logf writes a timestamped progress line to stderr.
func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

// coins converts a whole number of coins to wei (1 coin == 1e18 wei).
func coins(n int64) *big.Int {
	return params.EtherToWei(big.NewInt(n))
}

// weiToCoins renders a wei amount as a decimal coin count for display.
func weiToCoins(wei *big.Int) string {
	c := new(big.Int).Div(wei, big.NewInt(params.Ether))
	return c.String()
}

func printSummary(outputPath string, cfg genConfig, batchPattern []batchSpec, need []*big.Int, totalTxns int, mode tipMode, contractAddr common.Address, feeCapMax *big.Int) {
	fmt.Printf("Wrote %s\n", outputPath)
	fmt.Printf("  startBlockNumber: %d\n", cfg.startBlock)
	fmt.Printf("  chainID:          %s\n", chainID.String())
	fmt.Printf("  rootStartNonce:   %d\n", cfg.startNonce)
	fmt.Printf("  parallelism:      %d\n", cfg.parallelism)
	fmt.Printf("  levels:           %d\n", len(batchPattern))
	fmt.Printf("  totalTransactions:%d\n", totalTxns)
	fmt.Printf("  tokens:           %v\n", cfg.tokens)
	if cfg.tokens {
		fmt.Printf("  tokenContract:    %s\n", contractAddr.Hex())
	}
	fmt.Printf("  tips:             %s (GasV3StartBlock=%d)\n", tipModeName(mode), defaults.DefaultConfig.PosConfig.GasV3StartBlock)
	fmt.Printf("  leaf amount:      %d coins\n", int64(leafCoins))
	fmt.Printf("  fee cap max/gas:  %s wei\n", feeCapMax.String())

	fmt.Println("  per-batch funding (coin value sent to each funded wallet):")
	for idx, spec := range batchPattern {
		level := idx + 1
		fmt.Printf("    batch %-3d: %8d wallets, %s coins each\n", spec.batchNumber, spec.funded, weiToCoins(need[level]))
	}

	rootOut := rootRequiredWei(need[1], cfg.tokens, feeCapMax)
	fmt.Printf("  required root balance (approx): %s coins\n", weiToCoins(rootOut))
}
