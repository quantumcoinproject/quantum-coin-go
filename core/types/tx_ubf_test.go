// Copyright 2026 The go-ethereum Authors
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

package types

import (
	"container/heap"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
)

// TestUBF049_PopNilsBackingArray checks that TxBySortPrefix.Pop clears the slot
// it vacates, so the backing array does not keep the popped transaction alive.
// Upstream 06632da2b (#26296).
func TestUBF049_PopNilsBackingArray(t *testing.T) {
	first, err := NewWrapperTxn(NewTransaction(0, common.BytesToAddress([]byte{0x01}), big.NewInt(0), 21000, nil, nil), []byte{0x01})
	if err != nil {
		t.Fatalf("failed to wrap transaction: %v", err)
	}
	second, err := NewWrapperTxn(NewTransaction(1, common.BytesToAddress([]byte{0x02}), big.NewInt(0), 21000, nil, nil), []byte{0x02})
	if err != nil {
		t.Fatalf("failed to wrap transaction: %v", err)
	}

	h := &TxBySortPrefix{first, second}
	heap.Init(h)
	if popped := heap.Pop(h).(*WrapperTxn); popped == nil {
		t.Fatal("expected a popped element")
	}
	if len(*h) != 1 {
		t.Fatalf("expected 1 element left, got %d", len(*h))
	}
	// Reach past the shrunk length into the (still allocated) backing array.
	backing := (*h)[:len(*h)+1]
	if backing[len(*h)] != nil {
		t.Fatal("Pop left a dangling *WrapperTxn in the backing array")
	}
}

// TestUBF050_TxCopyDoesNotAliasRecipient checks that copy() deep-copies the
// recipient address instead of sharing the caller's pointer. Addresses are 32
// bytes here, so the whole value has to be copied.
// Upstream 4e599ee46 (#23376).
func TestUBF050_TxCopyDoesNotAliasRecipient(t *testing.T) {
	t.Run("DefaultFeeTx", func(t *testing.T) {
		to := common.BytesToAddress([]byte{0x11, 0x22})
		want := to
		inner := &DefaultFeeTx{
			ChainID:    big.NewInt(DEFAULT_CHAIN_ID),
			Nonce:      3,
			To:         &to,
			Value:      big.NewInt(1),
			Gas:        21000,
			MaxGasTier: GAS_TIER_DEFAULT,
		}
		cpy := inner.copy().(*DefaultFeeTx)
		if cpy.To == inner.To {
			t.Fatal("copy shares the recipient pointer with the original")
		}
		// Mutate every byte of the caller-owned address.
		for i := range to {
			to[i] = 0xff
		}
		if *cpy.To != want {
			t.Fatalf("copy aliased the recipient: have %x, want %x", *cpy.To, want)
		}
	})

	t.Run("DynamicFeeTx", func(t *testing.T) {
		to := common.BytesToAddress([]byte{0x33, 0x44})
		want := to
		inner := &DynamicFeeTx{
			ChainID:   big.NewInt(DEFAULT_CHAIN_ID),
			Nonce:     3,
			To:        &to,
			Value:     big.NewInt(1),
			Gas:       21000,
			GasTipCap: big.NewInt(0),
			GasFeeCap: big.NewInt(0),
		}
		cpy := inner.copy().(*DynamicFeeTx)
		if cpy.To == inner.To {
			t.Fatal("copy shares the recipient pointer with the original")
		}
		for i := range to {
			to[i] = 0xff
		}
		if *cpy.To != want {
			t.Fatalf("copy aliased the recipient: have %x, want %x", *cpy.To, want)
		}
	})

	t.Run("nil recipient", func(t *testing.T) {
		inner := &DefaultFeeTx{Value: big.NewInt(0), Gas: 21000, MaxGasTier: GAS_TIER_DEFAULT}
		if cpy := inner.copy().(*DefaultFeeTx); cpy.To != nil {
			t.Fatalf("contract creation must keep a nil recipient, got %x", *cpy.To)
		}
	})
}
