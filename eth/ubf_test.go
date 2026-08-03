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

package eth

import (
	"context"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/mockconsensus"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// newStateAccessorTester spins up a bare-bones Ethereum instance backed by an
// archive-mode chain of n blocks. Only the fields stateAtBlock touches are set.
func newStateAccessorTester(t *testing.T, n int) (*Ethereum, []*types.Block) {
	t.Helper()

	var (
		gspec   = &core.Genesis{Config: params.TestChainConfig}
		gendb   = rawdb.NewMemoryDatabase()
		genesis = gspec.MustCommit(gendb)
		chaindb = rawdb.NewMemoryDatabase()
	)
	blocks, _ := core.GenerateChain(gspec.Config, genesis, mockconsensus.NewMockConsensus(), gendb, n, func(i int, b *core.BlockGen) {})

	gspec.MustCommit(chaindb)
	chain, err := core.NewBlockChain(chaindb, &core.CacheConfig{
		TrieCleanLimit:    256,
		TrieDirtyLimit:    256,
		TrieTimeLimit:     5 * time.Minute,
		SnapshotLimit:     0,
		TrieDirtyDisabled: true, // archive mode, every state root hits disk
	}, gspec.Config, mockconsensus.NewMockConsensus(), vm.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	t.Cleanup(chain.Stop)

	if _, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("failed to insert tester chain: %v", err)
	}
	return &Ethereum{chainDb: chaindb, blockchain: chain}, blocks
}

// TestUBF080_StateAtBlockHonoursPreferDisk verifies that stateAtBlock actually acts
// on the preferDisk hint: when the state is available on disk it must be picked up
// from a fresh ephemeral trie database instead of being replayed on top of the
// caller supplied (and ever-growing) in-memory base state.
//
// Before the fix preferDisk was accepted by EthAPIBackend.StateAtBlock and silently
// dropped, which is what let a long debug_traceChain run out of memory.
// Upstream 3bbeb94c1 (#23736).
func TestUBF080_StateAtBlockHonoursPreferDisk(t *testing.T) {
	eth, blocks := newStateAccessorTester(t, 4)

	var (
		parent = blocks[len(blocks)-2]
		target = blocks[len(blocks)-1]
	)
	newBase := func() *state.StateDB {
		base, err := eth.blockchain.StateAt(parent.Root())
		if err != nil {
			t.Fatalf("failed to open the base state: %v", err)
		}
		return base
	}
	// Without the hint the base state (and hence its trie database) is carried over.
	base := newBase()
	statedb, err := eth.stateAtBlock(target, 0, base, false, false)
	if err != nil {
		t.Fatalf("stateAtBlock(preferDisk=false) failed: %v", err)
	}
	if statedb.Database() != base.Database() {
		t.Fatalf("preferDisk=false abandoned the caller supplied base state")
	}
	// With the hint the state is re-opened from disk on a fresh trie database, so
	// nothing the caller accumulated in memory is kept alive.
	base = newBase()
	statedb, err = eth.stateAtBlock(target, 0, base, false, true)
	if err != nil {
		t.Fatalf("stateAtBlock(preferDisk=true) failed: %v", err)
	}
	if statedb.Database() == base.Database() {
		t.Fatalf("preferDisk=true was ignored: the caller's trie database is still in use")
	}
	if root := statedb.IntermediateRoot(true); root != target.Root() {
		t.Fatalf("preferDisk=true returned state root %x, want %x", root, target.Root())
	}
}

// TestUBF093_GetEVMSurfacesStateError verifies that the vmError closure handed out by
// GetEVM is the StateDB's own error reporter rather than a closure that always says
// "no error" — the latter silently turns a failed state read into a wrong eth_call
// result.
//
// StateDB.setError is unexported, so the assertion is on the identity of the returned
// reporter: pre-fix it was a `func() error { return nil }` literal, post-fix it is the
// state.Error method value.
// Upstream 13e698592 (#25876).
func TestUBF093_GetEVMSurfacesStateError(t *testing.T) {
	eth, blocks := newStateAccessorTester(t, 1)
	b := &EthAPIBackend{eth: eth}

	statedb, err := eth.blockchain.StateAt(blocks[0].Root())
	if err != nil {
		t.Fatalf("failed to open state: %v", err)
	}
	msg := types.NewMessage(common.Address{}, nil, 0, new(big.Int), 0, new(big.Int), nil, nil, false, 0)

	_, vmError, err := b.GetEVM(context.Background(), msg, statedb, blocks[0].Header(), nil)
	if err != nil {
		t.Fatalf("GetEVM failed: %v", err)
	}
	if got := vmError(); got != nil {
		t.Fatalf("vmError on a healthy state = %v, want nil", got)
	}
	if reflect.ValueOf(vmError).Pointer() != reflect.ValueOf(statedb.Error).Pointer() {
		t.Fatalf("GetEVM does not hand back state.Error: StateDB read errors are being swallowed")
	}
}
