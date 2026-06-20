package proofofstake

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

func tipBaseFee() *big.Int { return big.NewInt(defaults.DEFAULT_PRICE / 10) }

func tipDynTx(nonce uint64, tip int64, gas uint64) *types.Transaction {
	to := common.BytesToAddress([]byte{0x55})
	tipBig := big.NewInt(tip)
	feeCap := new(big.Int).Add(tipBaseFee(), tipBig)
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          nonce,
		GasTipCap:      tipBig,
		GasFeeCap:      feeCap,
		Gas:            gas,
		To:             &to,
		Value:          big.NewInt(0),
		SigningContext: byte(crypto.SigningContextDefault),
		V:              big.NewInt(0),
		R:              big.NewInt(0),
		S:              big.NewInt(0),
	})
}

func tipDefTx(nonce uint64, gas uint64) *types.Transaction {
	to := common.BytesToAddress([]byte{0x66})
	return types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      nonce,
		Gas:        gas,
		MaxGasTier: types.GAS_TIER_DEFAULT,
		To:         &to,
		Value:      big.NewInt(0),
		V:          big.NewInt(0),
		R:          big.NewInt(0),
		S:          big.NewInt(0),
	})
}

func receiptFor(tx *types.Transaction, gasUsed uint64) *types.Receipt {
	return &types.Receipt{TxHash: tx.Hash(), GasUsed: gasUsed}
}

func TestCalculateTxnTipTotal(t *testing.T) {
	// Single tipped dynamic-fee tx: tip * gasUsed.
	t.Run("single dynamic tip", func(t *testing.T) {
		tx := tipDynTx(0, 100, 21000)
		txs := []*types.Transaction{tx}
		receipts := []*types.Receipt{receiptFor(tx, 21000)}
		got, err := calculateTxnTipTotal(txs, receipts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := big.NewInt(100 * 21000)
		if got.Cmp(want) != 0 {
			t.Fatalf("tip total = %v, want %v", got, want)
		}
	})

	// Default-fee tx contributes zero tip.
	t.Run("default fee zero tip", func(t *testing.T) {
		tx := tipDefTx(0, 21000)
		txs := []*types.Transaction{tx}
		receipts := []*types.Receipt{receiptFor(tx, 21000)}
		got, err := calculateTxnTipTotal(txs, receipts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Sign() != 0 {
			t.Fatalf("tip total = %v, want 0", got)
		}
	})

	// Mixed default + dynamic: only the dynamic tip counts, weighted by its own gasUsed.
	t.Run("mixed default and dynamic", func(t *testing.T) {
		txDyn := tipDynTx(0, 100, 21000)
		txDef := tipDefTx(0, 21000)
		txs := []*types.Transaction{txDyn, txDef}
		receipts := []*types.Receipt{receiptFor(txDyn, 21000), receiptFor(txDef, 21000)}
		got, err := calculateTxnTipTotal(txs, receipts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := big.NewInt(100 * 21000)
		if got.Cmp(want) != 0 {
			t.Fatalf("tip total = %v, want %v", got, want)
		}
	})

	// Multiple dynamic txns: tip weighted by each tx's gasUsed.
	t.Run("multiple dynamic txns", func(t *testing.T) {
		tx1 := tipDynTx(0, 100, 21000)
		tx2 := tipDynTx(1, 50, 30000)
		txs := []*types.Transaction{tx1, tx2}
		receipts := []*types.Receipt{receiptFor(tx1, 21000), receiptFor(tx2, 30000)}
		got, err := calculateTxnTipTotal(txs, receipts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := big.NewInt(100*21000 + 50*30000)
		if got.Cmp(want) != 0 {
			t.Fatalf("tip total = %v, want %v", got, want)
		}
	})

	// Mismatched lengths is a consensus error.
	t.Run("length mismatch errors", func(t *testing.T) {
		tx := tipDynTx(0, 100, 21000)
		txs := []*types.Transaction{tx}
		if _, err := calculateTxnTipTotal(txs, nil); err == nil {
			t.Fatalf("expected error on length mismatch")
		}
	})
}
