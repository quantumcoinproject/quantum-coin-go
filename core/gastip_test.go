package core

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

const testBasicGas = uint64(21000)

func dynBaseFee() *big.Int { return big.NewInt(defaults.DEFAULT_PRICE / 10) }

// dynTipTx builds a dynamic-fee transaction whose effective tip equals tip (gasFeeCap = base + tip).
func dynTipTx(nonce uint64, to *common.Address, gas uint64, tip int64, data []byte) *types.Transaction {
	tipBig := big.NewInt(tip)
	feeCap := new(big.Int).Add(dynBaseFee(), tipBig)
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          nonce,
		GasTipCap:      tipBig,
		GasFeeCap:      feeCap,
		Gas:            gas,
		To:             to,
		Value:          big.NewInt(0),
		Data:           data,
		SigningContext: byte(crypto.SigningContextDefault),
		V:              big.NewInt(0),
		R:              big.NewInt(0),
		S:              big.NewInt(0),
	})
}

// dynNullCapTx builds a dynamic-fee transaction with null/zero caps (the legacy opt-out case):
// it pays only the base fee and contributes no tip.
func dynNullCapTx(nonce uint64, to *common.Address, gas uint64) *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          nonce,
		GasTipCap:      big.NewInt(0),
		GasFeeCap:      big.NewInt(0),
		Gas:            gas,
		To:             to,
		Value:          big.NewInt(0),
		SigningContext: byte(crypto.SigningContextDefault),
		V:              big.NewInt(0),
		R:              big.NewInt(0),
		S:              big.NewInt(0),
	})
}

// dynBadFeeCapTx builds a dynamic-fee transaction whose gasFeeCap is below the base fee (invalid).
func dynBadFeeCapTx(nonce uint64, to *common.Address, gas uint64) *types.Transaction {
	feeCap := new(big.Int).Sub(dynBaseFee(), big.NewInt(1))
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          nonce,
		GasTipCap:      big.NewInt(0),
		GasFeeCap:      feeCap,
		Gas:            gas,
		To:             to,
		Value:          big.NewInt(0),
		SigningContext: byte(crypto.SigningContextDefault),
		V:              big.NewInt(0),
		R:              big.NewInt(0),
		S:              big.NewInt(0),
	})
}

// defFeeTx builds a default-fee transaction (effective tip always zero).
func defFeeTx(nonce uint64, to *common.Address, gas uint64, data []byte) *types.Transaction {
	return types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      nonce,
		Gas:        gas,
		MaxGasTier: types.GAS_TIER_DEFAULT,
		To:         to,
		Value:      big.NewInt(0),
		Data:       data,
		V:          big.NewInt(0),
		R:          big.NewInt(0),
		S:          big.NewInt(0),
	})
}

// dynCapTx builds a dynamic-fee transaction with explicit (possibly invalid) tip/fee caps so the
// validation rules can be exercised directly.
func dynCapTx(tipCap, feeCap *big.Int) *types.Transaction {
	to := addr(9)
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          0,
		GasTipCap:      tipCap,
		GasFeeCap:      feeCap,
		Gas:            testBasicGas,
		To:             &to,
		Value:          big.NewInt(0),
		SigningContext: byte(crypto.SigningContextDefault),
		V:              big.NewInt(0),
		R:              big.NewInt(0),
		S:              big.NewInt(0),
	})
}

// TestValidateGasFeeCaps covers the shared fee-cap rule used by both the pool and consensus.
func TestValidateGasFeeCaps(t *testing.T) {
	base := dynBaseFee()
	aboveBase := new(big.Int).Add(base, big.NewInt(100))

	tests := []struct {
		name    string
		tx      *types.Transaction
		wantErr bool
	}{
		{name: "tipped within cap ok", tx: dynCapTx(big.NewInt(50), aboveBase)},
		{name: "tip equal to feeCap ok", tx: dynCapTx(new(big.Int).Set(aboveBase), aboveBase)},
		{name: "opt-out zero caps ok", tx: dynCapTx(big.NewInt(0), big.NewInt(0))},
		{name: "feeCap equals base ok", tx: dynCapTx(big.NewInt(0), new(big.Int).Set(base))},
		{name: "tipCap without feeCap rejected", tx: dynCapTx(big.NewInt(1), big.NewInt(0)), wantErr: true},
		{name: "tipCap above feeCap rejected", tx: dynCapTx(new(big.Int).Add(aboveBase, big.NewInt(1)), aboveBase), wantErr: true},
		{name: "feeCap below base rejected", tx: dynCapTx(big.NewInt(0), new(big.Int).Sub(base, big.NewInt(1))), wantErr: true},
		{name: "negative feeCap rejected", tx: dynCapTx(big.NewInt(0), big.NewInt(-1)), wantErr: true},
		{name: "negative tipCap rejected", tx: dynCapTx(big.NewInt(-1), aboveBase), wantErr: true},
		// Default-fee transactions carry no tip/fee caps and are always accepted (no-op).
		{name: "default-fee tx ok", tx: defFeeTx(0, func() *common.Address { a := addr(9); return &a }(), testBasicGas, nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGasFeeCaps(tt.tx, base)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func addr(b byte) common.Address { return common.BytesToAddress([]byte{b}) }

func countSelected(m map[common.Address]types.Transactions) int {
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}

func TestSplitGasPools(t *testing.T) {
	tests := []struct {
		gasLimit            uint64
		wantBasic, wantGen  uint64
	}{
		{gasLimit: 100, wantBasic: 50, wantGen: 50},
		{gasLimit: 101, wantBasic: 50, wantGen: 51}, // odd unit goes to the general pool
		{gasLimit: 0, wantBasic: 0, wantGen: 0},
		{gasLimit: 42000, wantBasic: 21000, wantGen: 21000},
	}
	for _, tt := range tests {
		b, g := SplitGasPools(tt.gasLimit)
		if b != tt.wantBasic || g != tt.wantGen {
			t.Fatalf("SplitGasPools(%d) = (%d, %d), want (%d, %d)", tt.gasLimit, b, g, tt.wantBasic, tt.wantGen)
		}
		if b+g != tt.gasLimit {
			t.Fatalf("SplitGasPools(%d) pools do not sum to limit: %d + %d", tt.gasLimit, b, g)
		}
	}
}

func TestIsBasicTransfer(t *testing.T) {
	to := addr(9)

	if IsBasicTransfer(dynTipTx(0, &to, testBasicGas, 0, nil), nil) == false {
		t.Fatalf("EOA-to-EOA transfer with no data should be basic")
	}
	if IsBasicTransfer(defFeeTx(0, &to, testBasicGas, nil), nil) == false {
		t.Fatalf("default-fee EOA transfer should be basic")
	}
	if IsBasicTransfer(dynTipTx(0, nil, testBasicGas, 0, nil), nil) {
		t.Fatalf("contract creation (nil To) should not be basic")
	}
	if IsBasicTransfer(dynTipTx(0, &to, testBasicGas, 0, []byte{1, 2, 3}), nil) {
		t.Fatalf("transfer with calldata should not be basic")
	}
	codeFn := func(common.Address) int { return 1 }
	if IsBasicTransfer(dynTipTx(0, &to, testBasicGas, 0, nil), codeFn) {
		t.Fatalf("transfer to a contract recipient should not be basic")
	}
}

// TestSelectByEffectiveTipOrdering: higher-tip transactions win when the block cannot hold all of
// them, and basic transfers that overflow the basic pool spill into the general pool.
func TestSelectByEffectiveTipOrdering(t *testing.T) {
	to := addr(100)
	// gasLimit 42000 -> basic 21000 (1 tx), general 21000 (1 overflow tx) => capacity 2.
	basic, general := SplitGasPools(42000)
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 100, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 200, nil)},
		addr(3): {dynTipTx(0, &to, testBasicGas, 300, nil)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 2 {
		t.Fatalf("selected %d txns, want 2", countSelected(got))
	}
	if _, ok := got[addr(1)]; ok {
		t.Fatalf("lowest-tip account should not be selected")
	}
	if _, ok := got[addr(2)]; !ok {
		t.Fatalf("tip 200 account should be selected")
	}
	if _, ok := got[addr(3)]; !ok {
		t.Fatalf("tip 300 account should be selected")
	}
}

// TestSelectByEffectiveTipBasicFillAndOverflow: basic transfers fill the basic pool first then spill
// into the general pool; once both pools are full the remaining basic txns are dropped.
func TestSelectByEffectiveTipBasicFillAndOverflow(t *testing.T) {
	to := addr(100)
	// gasLimit 84000 -> basic 42000 (2 tx), general 42000 (2 tx) => capacity 4.
	basic, general := SplitGasPools(84000)
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 100, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 200, nil)},
		addr(3): {dynTipTx(0, &to, testBasicGas, 300, nil)},
		addr(4): {dynTipTx(0, &to, testBasicGas, 400, nil)},
		addr(5): {dynTipTx(0, &to, testBasicGas, 500, nil)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 4 {
		t.Fatalf("selected %d txns, want 4", countSelected(got))
	}
	if _, ok := got[addr(1)]; ok {
		t.Fatalf("lowest-tip account (100) should be dropped")
	}
}

// TestSelectByEffectiveTipGeneralIsolation: non-basic transactions only use the general pool; the
// basic pool is never lent to them even when it is completely idle.
func TestSelectByEffectiveTipGeneralIsolation(t *testing.T) {
	to := addr(100)
	// gasLimit 42000 -> basic 21000 (idle), general 21000 (1 tx).
	basic, general := SplitGasPools(42000)
	data := []byte{1} // makes the txns non-basic
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 100, data)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 200, data)},
		addr(3): {dynTipTx(0, &to, testBasicGas, 300, data)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 1 {
		t.Fatalf("selected %d txns, want 1 (general pool only)", countSelected(got))
	}
	if _, ok := got[addr(3)]; !ok {
		t.Fatalf("highest-tip non-basic txn should be selected")
	}
}

// TestSelectByEffectiveTipNonceOrder: a higher-tip transaction cannot jump ahead of a lower-nonce
// transaction from the same account.
func TestSelectByEffectiveTipNonceOrder(t *testing.T) {
	to := addr(100)
	// capacity 2.
	basic, general := SplitGasPools(42000)
	txnMap := map[common.Address]types.Transactions{
		// account 1: nonce0 has tip 0, nonce1 has the highest tip in the set.
		addr(1): {defFeeTx(0, &to, testBasicGas, nil), dynTipTx(1, &to, testBasicGas, 1000, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 500, nil)},
		addr(3): {dynTipTx(0, &to, testBasicGas, 400, nil)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 2 {
		t.Fatalf("selected %d txns, want 2", countSelected(got))
	}
	if _, ok := got[addr(1)]; ok {
		t.Fatalf("account 1 should not be selected: its high-tip nonce1 is behind a zero-tip nonce0")
	}
}

// TestSelectByEffectiveTipDefaultStarved: tipped dynamic-fee transactions that fill the whole block
// starve zero-tip default-fee transactions, which are never selected.
func TestSelectByEffectiveTipDefaultStarved(t *testing.T) {
	to := addr(100)
	// gasLimit 84000 -> capacity 4.
	basic, general := SplitGasPools(84000)
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 100, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 200, nil)},
		addr(3): {dynTipTx(0, &to, testBasicGas, 300, nil)},
		addr(4): {dynTipTx(0, &to, testBasicGas, 400, nil)},
		addr(5): {defFeeTx(0, &to, testBasicGas, nil)}, // tip 0
		addr(6): {defFeeTx(0, &to, testBasicGas, nil)}, // tip 0
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 4 {
		t.Fatalf("selected %d txns, want 4", countSelected(got))
	}
	if _, ok := got[addr(5)]; ok {
		t.Fatalf("default-fee tx (addr5) should be starved out")
	}
	if _, ok := got[addr(6)]; ok {
		t.Fatalf("default-fee tx (addr6) should be starved out")
	}
}

// TestSelectByEffectiveTipDefaultIncludedWhenRoom: when capacity remains after the tipped
// transactions, the zero-tip default-fee transactions are included.
func TestSelectByEffectiveTipDefaultIncludedWhenRoom(t *testing.T) {
	to := addr(100)
	// gasLimit 126000 -> capacity 6 (3 basic + 3 general).
	basic, general := SplitGasPools(126000)
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 100, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 200, nil)},
		addr(3): {dynTipTx(0, &to, testBasicGas, 300, nil)},
		addr(4): {dynTipTx(0, &to, testBasicGas, 400, nil)},
		addr(5): {defFeeTx(0, &to, testBasicGas, nil)},
		addr(6): {defFeeTx(0, &to, testBasicGas, nil)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 6 {
		t.Fatalf("selected %d txns, want 6", countSelected(got))
	}
	if _, ok := got[addr(5)]; !ok {
		t.Fatalf("default-fee tx (addr5) should be selected when capacity remains")
	}
	if _, ok := got[addr(6)]; !ok {
		t.Fatalf("default-fee tx (addr6) should be selected when capacity remains")
	}
}

func TestSelectByEffectiveTipMaxCount(t *testing.T) {
	to := addr(100)
	// Large pools so capacity is not the limit; maxCount binds instead.
	basic, general := SplitGasPools(testBasicGas * 100)
	txnMap := make(map[common.Address]types.Transactions)
	for i := byte(1); i <= 10; i++ {
		txnMap[addr(i)] = types.Transactions{dynTipTx(0, &to, testBasicGas, int64(i)*10, nil)}
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 3)
	if countSelected(got) != 3 {
		t.Fatalf("selected %d txns, want 3 (maxCount)", countSelected(got))
	}
}

// TestSelectByEffectiveTipInvalidFeeCap: a transaction whose fee cap cannot cover the base fee is
// excluded, and (by blocking the account) its later nonces are excluded too.
func TestSelectByEffectiveTipInvalidFeeCap(t *testing.T) {
	to := addr(100)
	basic, general := SplitGasPools(testBasicGas * 100)
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynBadFeeCapTx(0, &to, testBasicGas), dynTipTx(1, &to, testBasicGas, 1000, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 50, nil)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if _, ok := got[addr(1)]; ok {
		t.Fatalf("account with invalid fee cap should be excluded entirely")
	}
	if _, ok := got[addr(2)]; !ok {
		t.Fatalf("valid account should still be selected")
	}
	if countSelected(got) != 1 {
		t.Fatalf("selected %d txns, want 1", countSelected(got))
	}
}

// TestSelectByEffectiveTipNullCapsZeroTip: dynamic-fee txns with null/zero caps are treated as
// zero-tip (legacy opt-out), included when there is room and starved out like default-fee txns
// when tipped txns fill the block.
func TestSelectByEffectiveTipNullCapsZeroTip(t *testing.T) {
	to := addr(100)

	// With room for all, the null-cap (zero-tip) txn is selected.
	basic, general := SplitGasPools(84000) // capacity 4
	txnMap := map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 300, nil)},
		addr(2): {dynNullCapTx(0, &to, testBasicGas)},
	}
	got := SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 2 {
		t.Fatalf("selected %d txns, want 2", countSelected(got))
	}
	if _, ok := got[addr(2)]; !ok {
		t.Fatalf("null-cap (zero-tip) txn should be selected when capacity remains")
	}

	// When tipped txns fill the block, the null-cap (zero-tip) txn is starved out.
	basic, general = SplitGasPools(42000) // capacity 2
	txnMap = map[common.Address]types.Transactions{
		addr(1): {dynTipTx(0, &to, testBasicGas, 300, nil)},
		addr(2): {dynTipTx(0, &to, testBasicGas, 200, nil)},
		addr(3): {dynNullCapTx(0, &to, testBasicGas)},
	}
	got = SelectByEffectiveTip(txnMap, basic, general, nil, 0)
	if countSelected(got) != 2 {
		t.Fatalf("selected %d txns, want 2", countSelected(got))
	}
	if _, ok := got[addr(3)]; ok {
		t.Fatalf("null-cap (zero-tip) txn should be starved by tipped txns")
	}
}

// TestSelectByEffectiveTipEmpty: empty input yields an empty selection.
func TestSelectByEffectiveTipEmpty(t *testing.T) {
	got := SelectByEffectiveTip(map[common.Address]types.Transactions{}, 100, 100, nil, 0)
	if countSelected(got) != 0 {
		t.Fatalf("expected empty selection")
	}
}
