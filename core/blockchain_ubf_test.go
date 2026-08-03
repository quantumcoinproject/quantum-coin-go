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
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/mockconsensus"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// newUBFTestChain spins up an in-memory blockchain with `n` canonical blocks on
// top of the genesis, returning the chain, the backing database and the blocks.
func newUBFTestChain(t *testing.T, n int, cacheConfig *CacheConfig) (*BlockChain, ethdb.Database, []*types.Block) {
	t.Helper()

	db := rawdb.NewMemoryDatabase()
	gspec := &Genesis{Config: params.TestChainConfig}
	genesis := gspec.MustCommit(db)
	engine := mockconsensus.NewMockConsensus()

	blocks, _ := GenerateChain(params.TestChainConfig, genesis, engine, db, n, func(i int, b *BlockGen) {
		b.SetCoinbase(common.Address{0x01, byte(i)})
	})
	chain, err := NewBlockChain(db, cacheConfig, params.TestChainConfig, engine, vm.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("failed to create chain: %v", err)
	}
	t.Cleanup(chain.Stop)

	if n > 0 {
		if _, err := chain.InsertChain(blocks); err != nil {
			t.Fatalf("failed to insert chain: %v", err)
		}
	}
	return chain, db, blocks
}

// TestUBF008_ReorgDeletesStaleCanonicalMarkers covers upstream f9806dc87: after a
// reorg onto a shorter side chain, the canonical-hash markers of the abandoned
// segment must be deleted. The buggy code anchored the deletion at the current
// head, which is still the *old* head when the new chain contributes no blocks
// below its own head, so every stale marker survived.
func TestUBF008_ReorgDeletesStaleCanonicalMarkers(t *testing.T) {
	chain, db, blocks := newUBFTestChain(t, 5, nil)

	// Fork off block #1 with a single, differently-hashed block. The reorg then has
	// commonBlock == #1 and newChain == [side#2], leaving markers #2..#5 stale.
	engine := mockconsensus.NewMockConsensus()
	side, _ := GenerateChain(params.TestChainConfig, blocks[0], engine, db, 1, func(i int, b *BlockGen) {
		b.SetCoinbase(common.Address{0xff})
	})
	if side[0].Hash() == blocks[1].Hash() {
		t.Fatal("side chain block is identical to the canonical one")
	}
	persistSideBlock(db, side[0])

	// Sanity check: the canonical markers of the old chain are in place.
	for i := 2; i <= 5; i++ {
		if have := rawdb.ReadCanonicalHash(db, uint64(i)); have != blocks[i-1].Hash() {
			t.Fatalf("canonical marker #%d not set up: have %x", i, have)
		}
	}
	if err := chain.reorg(chain.CurrentBlock(), side[0]); err != nil {
		t.Fatalf("reorg failed: %v", err)
	}
	chain.writeHeadBlock(side[0]) // finish the reorg the way writeBlockWithState would

	for i := 2; i <= 5; i++ {
		if have := rawdb.ReadCanonicalHash(db, uint64(i)); have == blocks[i-1].Hash() {
			t.Fatalf("stale canonical marker #%d still points at the abandoned chain (%x)", i, have)
		}
	}
}

// TestUBF009_SkipBlockRebuildsSnapshotGap covers upstream c576fa153: an already
// known block may only be skipped if skipping does not leave a hole in the
// snapshot layers. Previously insertion was skipped on ErrKnownBlock alone.
func TestUBF009_SkipBlockRebuildsSnapshotGap(t *testing.T) {
	chain, _, blocks := newUBFTestChain(t, 5, nil)

	if chain.snaps == nil {
		t.Fatal("snapshots not enabled, cannot exercise the fix")
	}
	if chain.snaps.Snapshot(blocks[4].Root()) == nil {
		t.Fatal("expected a snapshot layer for the inserted head")
	}
	// A block whose state is already known but whose snapshot layer never made it
	// to disk, e.g. because the node crashed between the two writes.
	gapped := types.NewBlockWithHeader(&types.Header{
		ParentHash: blocks[4].Hash(),
		Number:     new(big.Int).Add(blocks[4].Number(), common.Big1),
		Root:       common.Hash{0x99},
	})
	// The same, but with the parent's snapshot missing too — then there is no gap
	// to close and skipping is safe.
	orphanParent := types.NewBlockWithHeader(&types.Header{
		Number: new(big.Int).SetUint64(41),
		Root:   common.Hash{0x98},
	})
	orphan := types.NewBlockWithHeader(&types.Header{
		ParentHash: orphanParent.Hash(),
		Number:     new(big.Int).SetUint64(42),
		Root:       common.Hash{0x97},
	})
	if chain.snaps.Snapshot(gapped.Root()) != nil || chain.snaps.Snapshot(orphan.Root()) != nil {
		t.Fatal("did not expect snapshot layers for the fabricated roots")
	}

	tests := []struct {
		name string
		err  error
		it   *insertIterator
		want bool
	}{
		{
			// The fix: state is known but the snapshot layer is not, while the
			// parent's layer exists — processing must not be skipped.
			name: "snapshot gap must be processed",
			err:  ErrKnownBlock,
			it:   &insertIterator{chain: types.Blocks{gapped}},
			want: false,
		},
		{
			name: "snapshot present may be skipped",
			err:  ErrKnownBlock,
			it:   &insertIterator{chain: types.Blocks{blocks[4]}},
			want: true,
		},
		{
			name: "parent snapshot missing too may be skipped",
			err:  ErrKnownBlock,
			it:   &insertIterator{chain: types.Blocks{orphanParent, orphan}, index: 1},
			want: true,
		},
		{
			name: "other errors are never skipped",
			err:  ErrBannedHash,
			it:   &insertIterator{chain: types.Blocks{blocks[4]}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chain.skipBlock(tt.err, tt.it); got != tt.want {
				t.Fatalf("skipBlock = %v, want %v", got, tt.want)
			}
		})
	}

	// Without snapshots there is nothing to rebuild, so known blocks are skipped.
	t.Run("no snapshots skips known blocks", func(t *testing.T) {
		snaps := chain.snaps
		chain.snaps = nil
		defer func() { chain.snaps = snaps }()

		if !chain.skipBlock(ErrKnownBlock, &insertIterator{chain: types.Blocks{gapped}}) {
			t.Fatal("skipBlock = false, want true when snapshots are disabled")
		}
	})
}

// TestUBF010_SetHeadRepairFlag covers upstream d9c13d407: the repair short-circuit
// in setHeadBeyondRoot must be driven by an explicit flag. Deriving it from
// "head == current block number" wrongly turned a genuine SetHead into a no-op
// whenever the header chain ran ahead of the block chain, e.g. during fast sync.
func TestUBF010_SetHeadRepairFlag(t *testing.T) {
	chain, db, blocks := newUBFTestChain(t, 5, nil)

	// Push the header chain three headers beyond the block chain.
	engine := mockconsensus.NewMockConsensus()
	ahead, _ := GenerateChain(params.TestChainConfig, blocks[4], engine, db, 3, nil)
	headers := make([]*types.Header, len(ahead))
	for i, block := range ahead {
		headers[i] = block.Header()
	}
	if _, err := chain.InsertHeaderChain(headers, 1); err != nil {
		t.Fatalf("failed to insert headers: %v", err)
	}
	if have := chain.CurrentHeader().Number.Uint64(); have != 8 {
		t.Fatalf("header chain not extended: have #%d, want #8", have)
	}
	// SetHead to the current *block* number must still rewind the header chain.
	if err := chain.SetHead(5); err != nil {
		t.Fatalf("SetHead failed: %v", err)
	}
	if have := chain.CurrentHeader().Number.Uint64(); have != 5 {
		t.Fatalf("header chain not rewound: have #%d, want #5", have)
	}
}

// TestUBF011_InsertDuringStopDoesNotPanic covers upstream edb1937cf: chain writes
// racing a Stop used to poke a sync.WaitGroup that Stop was already waiting on,
// which could panic. The closable chainmu now fences writers off instead.
func TestUBF011_InsertDuringStopDoesNotPanic(t *testing.T) {
	chain, db, _ := newUBFTestChain(t, 0, nil)

	engine := mockconsensus.NewMockConsensus()
	blocks, _ := GenerateChain(params.TestChainConfig, chain.Genesis(), engine, db, 32, nil)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < len(blocks); j++ {
				// Errors are expected once the chain is stopped, a panic is not.
				chain.InsertChain(blocks[j : j+1])
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	chain.Stop()
	wg.Wait()

	// After Stop the mutex is closed for good: writes must be refused, not blocked.
	if _, err := chain.InsertChain(blocks); !errors.Is(err, errChainStopped) {
		t.Fatalf("InsertChain after Stop = %v, want %v", err, errChainStopped)
	}
	// Header insertion aborts inside header validation first, but it must still
	// refuse the write rather than block on or panic in the shutdown machinery.
	if _, err := chain.InsertHeaderChain([]*types.Header{blocks[0].Header()}, 1); err == nil {
		t.Fatal("InsertHeaderChain after Stop = nil, want an error")
	}
}

// TestUBF013_ReorgLogsBatched covers upstream 389021a5a: the removed/reborn log
// events of a reorg are emitted in batches of ~512 instead of being accumulated
// in full, which could exhaust memory on a large reorg.
func TestUBF013_ReorgLogsBatched(t *testing.T) {
	const (
		blocksPerSide = 4
		logsPerBlock  = 400
	)
	chain, db, blocks := newUBFTestChain(t, blocksPerSide+1, nil)

	// Attach a fat receipt to every block above the fork point. The bodies are
	// rewritten alongside so that receipt derivation keeps working.
	for _, block := range blocks[1:] {
		attachFatReceipt(t, db, block, logsPerBlock)
	}
	engine := mockconsensus.NewMockConsensus()
	side, _ := GenerateChain(params.TestChainConfig, blocks[0], engine, db, 1, func(i int, b *BlockGen) {
		b.SetCoinbase(common.Address{0xff})
	})
	persistSideBlock(db, side[0])

	removedCh := make(chan RemovedLogsEvent, 64)
	sub := chain.SubscribeRemovedLogsEvent(removedCh)
	defer sub.Unsubscribe()

	if err := chain.reorg(chain.CurrentBlock(), side[0]); err != nil {
		t.Fatalf("reorg failed: %v", err)
	}
	chain.writeHeadBlock(side[0]) // finish the reorg the way writeBlockWithState would
	sub.Unsubscribe()
	close(removedCh)

	var batches, total int
	for ev := range removedCh {
		batches++
		total += len(ev.Logs)
		if len(ev.Logs) > 512+logsPerBlock {
			t.Fatalf("removed log batch too large: %d logs", len(ev.Logs))
		}
	}
	if want := blocksPerSide * logsPerBlock; total != want {
		t.Fatalf("removed log count mismatch: have %d, want %d", total, want)
	}
	if batches < 2 {
		t.Fatalf("expected the removed logs to be split into batches, got %d event(s)", batches)
	}
}

// persistSideBlock stores a generated side-chain block so that the chain can walk
// and re-canonicalise it. GenerateChain only commits the state, not the block.
func persistSideBlock(db ethdb.Database, block *types.Block) {
	rawdb.WriteBlock(db, block)
	rawdb.WriteTd(db, block.Hash(), block.NumberU64(), block.Difficulty())
}

// attachFatReceipt rewrites the body of `block` with a single dummy transaction
// and stores a receipt carrying `logs` log entries for it, so that a reorg has a
// large amount of log events to fire.
func attachFatReceipt(t *testing.T, db ethdb.Database, block *types.Block, logs int) {
	t.Helper()

	to := common.Address{0x42}
	tx := types.NewTx(&types.DefaultFeeTx{
		ChainID: big.NewInt(1),
		Nonce:   block.NumberU64(),
		Gas:     21000,
		To:      &to,
		Value:   new(big.Int),
		V:       new(big.Int),
		R:       new(big.Int),
		S:       new(big.Int),
	})
	rawdb.WriteBody(db, block.Hash(), block.NumberU64(), &types.Body{Transactions: types.Transactions{tx}})

	receipt := &types.Receipt{
		Type:              tx.Type(),
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		Logs:              make([]*types.Log, logs),
	}
	for i := range receipt.Logs {
		receipt.Logs[i] = &types.Log{Address: common.Address{byte(i)}}
	}
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), types.Receipts{receipt})
}

// TestUBF014_TxIndexCappedAfterSetHead covers upstream 69686fa32: when the chain
// is rewound below the transaction index tail, the reindexing target must be
// capped at head+1. Otherwise the indexer keeps walking into deleted bodies,
// bails out immediately and never advances the tail.
func TestUBF014_TxIndexCappedAfterSetHead(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	gspec := &Genesis{Config: params.TestChainConfig}
	genesis := gspec.MustCommit(db)
	engine := mockconsensus.NewMockConsensus()

	blocks, _ := GenerateChain(params.TestChainConfig, genesis, engine, db, 10, nil)

	limit := uint64(0) // 0 == index the whole chain
	chain, err := NewBlockChain(db, nil, params.TestChainConfig, engine, vm.Config{}, nil, &limit)
	if err != nil {
		t.Fatalf("failed to create chain: %v", err)
	}
	defer chain.Stop()

	if _, err := chain.InsertChain(blocks); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}
	// Rewind well below the tail we are about to fake, dropping bodies #5..#10.
	if err := chain.SetHead(3); err != nil {
		t.Fatalf("SetHead failed: %v", err)
	}
	rawdb.WriteTxIndexTail(db, 10)

	// Re-import block #4 to fire a chain head event and wake up the indexer.
	if _, err := chain.InsertChain(blocks[3:4]); err != nil {
		t.Fatalf("failed to re-insert block: %v", err)
	}
	var (
		deadline = time.Now().Add(20 * time.Second)
		lastPoke = time.Now()
	)
	for {
		if tail := rawdb.ReadTxIndexTail(db); tail != nil && *tail == 0 {
			return
		}
		if time.Now().After(deadline) {
			tail := rawdb.ReadTxIndexTail(db)
			t.Fatalf("tx index tail never reached 0, stuck at %v", tail)
		}
		// The indexer subscribes to chain head events from inside its own goroutine,
		// so the re-import above is lost if it happens before that subscription
		// exists. Re-fire the event now and then so a missed wakeup heals instead of
		// hanging until the deadline. Rewind first: head must stay far below the
		// faked tail of 10, otherwise head+1 exceeds it and the uncapped code would
		// pass too, which would rob this test of its purpose.
		if time.Since(lastPoke) > 2*time.Second {
			if err := chain.SetHead(3); err != nil {
				t.Fatalf("SetHead failed: %v", err)
			}
			if _, err := chain.InsertChain(blocks[3:4]); err != nil {
				t.Fatalf("failed to re-insert block: %v", err)
			}
			lastPoke = time.Now()
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestUBF015_HasBlockRequiresHeader covers upstream 81bd99835: a racey rollback
// can leave a body on disk whose header is already gone. HasBlock (and with it
// HasFastBlock) must not claim such a block is present.
func TestUBF015_HasBlockRequiresHeader(t *testing.T) {
	chain, db, _ := newUBFTestChain(t, 1, nil)

	// A body without a header, as left behind by a rollback race.
	var (
		hash   = common.Hash{0xde, 0xad}
		number = uint64(99)
	)
	rawdb.WriteBody(db, hash, number, &types.Body{})
	rawdb.WriteReceipts(db, hash, number, nil)

	if !rawdb.HasBody(db, hash, number) {
		t.Fatal("test setup failed, body not written")
	}
	if chain.HasBlock(hash, number) {
		t.Fatal("HasBlock = true for a block without a header")
	}
	if chain.HasFastBlock(hash, number) {
		t.Fatal("HasFastBlock = true for a block without a header")
	}
}

// TestUBF016_InsertEmptyHeaderChain covers upstream e0e8bf31c: an empty header
// batch must be rejected up front. ValidateHeaderChain would otherwise index
// seals[-1] and panic.
func TestUBF016_InsertEmptyHeaderChain(t *testing.T) {
	chain, _, _ := newUBFTestChain(t, 1, nil)

	for _, headers := range [][]*types.Header{nil, {}} {
		n, err := chain.InsertHeaderChain(headers, 1)
		if err != nil {
			t.Fatalf("InsertHeaderChain(empty) = %v, want nil", err)
		}
		if n != 0 {
			t.Fatalf("InsertHeaderChain(empty) index = %d, want 0", n)
		}
	}
}
