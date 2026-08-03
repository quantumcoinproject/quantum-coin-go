// Copyright 2015 The go-ethereum Authors
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

package miner

import (
	"errors"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/mockconsensus"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/event"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// ubfBlockingSub is a subscription that stays alive until it is unsubscribed, so that
// mainLoop is not torn down by a subscription error.
func ubfBlockingSub() event.Subscription {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		<-quit
		return nil
	})
}

// TestUBF105_WorkerCloseNoRaceOnCurrent checks that close() no longer touches
// worker.current, which is owned by the mainLoop goroutine. Upstream ee120ef86.
// Run with -race.
func TestUBF105_WorkerCloseNoRaceOnCurrent(t *testing.T) {
	t.Run("close does not read current", func(t *testing.T) {
		w := &worker{exitCh: make(chan struct{})}

		// Stands in for mainLoop/makeCurrent, which is the sole writer of w.current.
		writerDone := make(chan struct{})
		go func() {
			defer close(writerDone)
			for i := 0; i < 100000; i++ {
				select {
				case <-w.exitCh:
					return
				default:
				}
				w.current = &environment{}
			}
		}()

		// Before the fix close() read w.current (and w.current.state) from this
		// goroutine, which the race detector flags against the writer above.
		w.close()
		<-writerDone
	})

	// The teardown moved into mainLoop, so exercise that path with a live environment
	// to make sure the new defer cannot panic or wedge the shutdown.
	t.Run("mainLoop teardown", func(t *testing.T) {
		w := &worker{
			exitCh:       make(chan struct{}),
			proposeCh:    make(chan *proposeReq),
			txsCh:        make(chan core.NewTxsEvent),
			chainSideCh:  make(chan core.ChainSideEvent),
			txsSub:       ubfBlockingSub(),
			chainHeadSub: ubfBlockingSub(),
			chainSideSub: ubfBlockingSub(),
		}
		// A current environment with a state whose prefetcher must be stopped on exit.
		db := rawdb.NewMemoryDatabase()
		gspec := &core.Genesis{Config: params.TestChainConfig}
		genesis := gspec.MustCommit(db)
		statedb, err := state.New(genesis.Root(), state.NewDatabase(db), nil)
		if err != nil {
			t.Fatal(err)
		}
		statedb.StartPrefetcher("miner")
		w.current = &environment{state: statedb}

		done := make(chan struct{})
		go func() {
			w.mainLoop()
			close(done)
		}()

		w.close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("mainLoop did not return after close")
		}
		// The deferred StopPrefetcher must have run; calling it again must be a no-op.
		statedb.StopPrefetcher()
	})
}

// TestUBF106_ResultLoopDeepCopiesReceipts checks that resultLoop copies the sealing
// task's receipts and logs before stamping block location fields on them.
// Upstream c113520d5: the originals (which are shared with the pending snapshot) were
// mutated in place, and the log slice was shared by the shallow receipt copy.
func TestUBF106_ResultLoopDeepCopiesReceipts(t *testing.T) {
	var (
		db          = rawdb.NewMemoryDatabase()
		gspec       = &core.Genesis{Config: params.TestChainConfig}
		chainEngine = mockconsensus.NewMockConsensus()
		engine      = ubfStubEngine{}
	)
	genesis := gspec.MustCommit(db)
	chain, err := core.NewBlockChain(db, nil, params.TestChainConfig, chainEngine, vm.Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Stop()

	blocks, _ := core.GenerateChain(params.TestChainConfig, genesis, chainEngine, db, 1, nil)
	block := blocks[0]
	statedb, err := state.New(genesis.Root(), state.NewDatabase(db), nil)
	if err != nil {
		t.Fatal(err)
	}

	origLog := &types.Log{
		Address: common.BytesToAddress([]byte{0x01}),
		Topics:  []common.Hash{common.BytesToHash([]byte{0x02})},
		Data:    []byte{0x03},
	}
	origReceipt := &types.Receipt{
		Status: types.ReceiptStatusSuccessful,
		Logs:   []*types.Log{origLog},
	}
	tsk := &task{
		receipts:  []*types.Receipt{origReceipt},
		state:     statedb,
		block:     block,
		createdAt: time.Now(),
	}

	w := &worker{
		engine:       engine,
		chain:        chain,
		mux:          new(event.TypeMux),
		unconfirmed:  newUnconfirmedBlocks(chain, miningLogAtDepth),
		pendingTasks: map[common.Hash]*task{engine.SealHash(block.Header()): tsk},
		resultCh:     make(chan *types.Block), // unbuffered: gives us a happens-before edge
		exitCh:       make(chan struct{}),
	}
	go w.resultLoop()
	defer close(w.exitCh)

	w.resultCh <- block
	// The second send only completes once resultLoop is back at the select, i.e. after
	// the block above has been fully processed.
	w.resultCh <- nil

	if origReceipt.BlockHash != (common.Hash{}) {
		t.Errorf("task receipt was mutated: BlockHash = %x", origReceipt.BlockHash)
	}
	if origReceipt.BlockNumber != nil {
		t.Errorf("task receipt was mutated: BlockNumber = %v", origReceipt.BlockNumber)
	}
	if len(origReceipt.Logs) != 1 || origReceipt.Logs[0] != origLog {
		t.Errorf("task receipt log slice was replaced")
	}
	if origLog.BlockHash != (common.Hash{}) {
		t.Errorf("task receipt log was mutated: BlockHash = %x", origLog.BlockHash)
	}

	// The copies handed to the chain must carry the block location fields.
	if stored := chain.GetReceiptsByHash(block.Hash()); len(stored) == 1 {
		if stored[0].BlockHash != block.Hash() {
			t.Errorf("stored receipt BlockHash = %x, want %x", stored[0].BlockHash, block.Hash())
		}
	}
}

// ubfStubEngine implements only the two engine methods the worker loops under test
// actually use. The real mockconsensus SealHash requires a signed Extra field, which
// these hand-built headers do not have.
type ubfStubEngine struct {
	consensus.Engine // nil: any other method call is a test bug and will panic
	sealErr          error
	onSeal           func()
}

func (e ubfStubEngine) SealHash(header *types.Header) common.Hash { return header.Hash() }

func (e ubfStubEngine) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	if e.onSeal != nil {
		e.onSeal()
	}
	return e.sealErr
}

// TestUBF107_PendingTasksClearedOnSealError checks that a task whose Seal call fails is
// removed from pendingTasks. Upstream 476fb565c: the failure was only logged, so the
// map grew without bound whenever the sealing engine refused to seal.
func TestUBF107_PendingTasksClearedOnSealError(t *testing.T) {
	var (
		sealCalled = make(chan struct{})
		release    = make(chan struct{})
	)
	engine := ubfStubEngine{
		sealErr: errors.New("sealing refused"),
		onSeal: func() {
			close(sealCalled)
			<-release
		},
	}
	w := &worker{
		engine:       engine,
		taskCh:       make(chan *task),
		resultCh:     make(chan *types.Block, resultQueueSize),
		exitCh:       make(chan struct{}),
		pendingTasks: make(map[common.Hash]*task),
	}
	go w.taskLoop()
	defer close(w.exitCh)

	header := &types.Header{Number: common.Big1, GasLimit: 1000000}
	w.taskCh <- &task{block: types.NewBlockWithHeader(header), createdAt: time.Now()}

	// Seal blocks until we release it, so the task is definitely tracked by now.
	select {
	case <-sealCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("Seal was never called")
	}
	w.pendingMu.RLock()
	tracked := len(w.pendingTasks)
	w.pendingMu.RUnlock()
	if tracked != 1 {
		t.Fatalf("pendingTasks holds %d entries while sealing, want 1", tracked)
	}
	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for {
		w.pendingMu.RLock()
		n := len(w.pendingTasks)
		w.pendingMu.RUnlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pendingTasks still holds %d entries after Seal failed", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
