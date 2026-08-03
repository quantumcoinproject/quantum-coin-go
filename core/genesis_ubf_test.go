// Copyright 2021 The go-ethereum Authors
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

package core

import (
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// TestUBF017_GenesisTDFromBlock covers upstream c72b16c34: ToBlock substitutes a
// default difficulty when the spec leaves it unset, so the genesis total
// difficulty has to be taken from the produced block. Writing g.Difficulty
// stored a nil (== zero) TD instead.
func TestUBF017_GenesisTDFromBlock(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	gspec := &Genesis{Config: params.TestChainConfig} // Difficulty deliberately unset

	block, err := gspec.Commit(db)
	if err != nil {
		t.Fatalf("failed to commit genesis: %v", err)
	}
	if block.Difficulty().Sign() == 0 {
		t.Fatal("test setup failed, genesis block difficulty is zero")
	}
	td := rawdb.ReadTd(db, block.Hash(), block.NumberU64())
	if td == nil {
		t.Fatal("genesis total difficulty not written")
	}
	if td.Cmp(block.Difficulty()) != 0 {
		t.Fatalf("genesis total difficulty mismatch: have %v, want %v", td, block.Difficulty())
	}
}
