package types

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

func newDynTipTx(tipCap, feeCap *big.Int) *Transaction {
	return NewTx(&DynamicFeeTx{
		ChainID:        big.NewInt(DEFAULT_CHAIN_ID),
		Nonce:          0,
		GasTipCap:      tipCap,
		GasFeeCap:      feeCap,
		Gas:            21000,
		To:             &testAddr,
		Value:          big.NewInt(0),
		Data:           nil,
		SigningContext: byte(crypto.SigningContextDefault),
		V:              big.NewInt(0),
		R:              big.NewInt(0),
		S:              big.NewInt(0),
	})
}

func TestEffectiveGasTip(t *testing.T) {
	base := big.NewInt(1000)

	tests := []struct {
		name    string
		tipCap  *big.Int
		feeCap  *big.Int
		baseFee *big.Int
		want    *big.Int
		wantErr bool
	}{
		{name: "feeCap equals base yields zero", tipCap: big.NewInt(50), feeCap: big.NewInt(1000), baseFee: base, want: big.NewInt(0)},
		{name: "tipCap below gap returns tipCap", tipCap: big.NewInt(50), feeCap: big.NewInt(1100), baseFee: base, want: big.NewInt(50)},
		{name: "tipCap above gap clamps to gap", tipCap: big.NewInt(200), feeCap: big.NewInt(1100), baseFee: base, want: big.NewInt(100)},
		{name: "zero tipCap returns zero", tipCap: big.NewInt(0), feeCap: big.NewInt(1100), baseFee: base, want: big.NewInt(0)},
		{name: "feeCap below base errors", tipCap: big.NewInt(50), feeCap: big.NewInt(999), baseFee: base, wantErr: true},
		{name: "nil baseFee returns tipCap", tipCap: big.NewInt(77), feeCap: big.NewInt(1100), baseFee: nil, want: big.NewInt(77)},
		{name: "zero feeCap opts out of tips", tipCap: big.NewInt(0), feeCap: big.NewInt(0), baseFee: base, want: big.NewInt(0)},
		// EffectiveGasTip is intentionally lenient and returns 0 here rather than erroring; the
		// "gasTipCap set without gasFeeCap" rejection lives in core.ValidateGasFeeCaps, which both
		// the tx pool and ApplyTransaction call before this helper runs.
		{name: "zero feeCap ignores tip cap", tipCap: big.NewInt(50), feeCap: big.NewInt(0), baseFee: base, want: big.NewInt(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := newDynTipTx(tt.tipCap, tt.feeCap)
			got, err := tx.EffectiveGasTip(tt.baseFee)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got tip %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Cmp(tt.want) != 0 {
				t.Fatalf("EffectiveGasTip = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCostIncludesFeeCap verifies Cost() reserves max(baseFee, gasFeeCap) * gas + value, so the
// pool's affordability checks cover the priority fee, while staying backward compatible for the
// opt-out (zero feeCap) case where only the base fee is charged.
func TestCostIncludesFeeCap(t *testing.T) {
	gas := new(big.Int).SetUint64(21000)
	base := newDynTipTx(big.NewInt(0), big.NewInt(0)).BaseFee()

	// feeCap above the base fee -> cost is bounded by feeCap.
	feeCap := new(big.Int).Add(base, big.NewInt(500))
	tx := newDynTipTx(big.NewInt(10), feeCap)
	want := new(big.Int).Mul(feeCap, gas)
	if got := tx.Cost(); got.Cmp(want) != 0 {
		t.Fatalf("Cost() with feeCap = %v, want %v", got, want)
	}

	// Opt-out (zero feeCap) -> cost falls back to the base fee, identical to legacy behavior.
	optOut := newDynTipTx(big.NewInt(0), big.NewInt(0))
	wantOptOut := new(big.Int).Mul(base, gas)
	if got := optOut.Cost(); got.Cmp(wantOptOut) != 0 {
		t.Fatalf("opt-out Cost() = %v, want base-only %v", got, wantOptOut)
	}
}

// TestEffectiveGasTipDefaultFeeTx verifies a default-fee transaction (gasTipCap == gasFeeCap ==
// base) always has a zero effective tip.
func TestEffectiveGasTipDefaultFeeTx(t *testing.T) {
	tx := NewDefaultFeeTransaction(big.NewInt(DEFAULT_CHAIN_ID), 0, &testAddr, big.NewInt(0), 21000, GAS_TIER_DEFAULT, nil)
	base := tx.BaseFee()
	if base.Cmp(GetDefaultGasPrice()) != 0 {
		t.Fatalf("default base fee = %v, want %v", base, GetDefaultGasPrice())
	}
	tip, err := tx.EffectiveGasTip(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tip.Sign() != 0 {
		t.Fatalf("default-fee tx effective tip = %v, want 0", tip)
	}
}
