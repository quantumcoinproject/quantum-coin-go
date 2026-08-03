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

package downloader

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/metrics"
	"github.com/quantumcoinproject/quantum-coin-go/p2p/msgrate"
)

// TestUBF051_RegisterUnlocksOnTrackError verifies that peerSet.Register releases
// its write lock when the rate tracker rejects the peer.
// Upstream 656dc8cc0 (#25546).
func TestUBF051_RegisterUnlocksOnTrackError(t *testing.T) {
	ps := newPeerSet()

	// Pre-seed the rate trackers with the id so that Register's Track call fails
	// while the peer itself is still absent from ps.peers.
	if err := ps.rates.Track("bad", msgrate.NewTracker(nil, time.Second)); err != nil {
		t.Fatalf("failed to seed rate tracker: %v", err)
	}
	if err := ps.Register(&peerConnection{id: "bad"}); err == nil {
		t.Fatalf("expected Register to fail on a duplicate rate tracker")
	}
	// The peer set must still be usable: with the leaked lock any further call
	// blocks forever.
	done := make(chan error, 1)
	go func() { done <- ps.Register(&peerConnection{id: "good"}) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second Register failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("peerSet lock leaked: Register blocked after a Track error")
	}
}

// TestUBF052_StaleDeliverySlotNotReconstructed verifies that queue.deliver does
// not reconstruct into a delivery slot that has already been handed upstream.
// Upstream b32d20324 (#25861).
func TestUBF052_StaleDeliverySlotNotReconstructed(t *testing.T) {
	q := newQueue(10, 10)
	// Move the result cache past block 1, so a delivery for it is stale.
	q.Prepare(5, FullSync)

	header := &types.Header{Number: big.NewInt(1)}
	q.blockTaskPool = map[common.Hash]*types.Header{header.Hash(): header}
	pendPool := map[string]*fetchRequest{
		"peer": {Headers: []*types.Header{header}, Time: time.Now()},
	}
	reconstructed := 0
	_, err := q.deliver("peer", q.blockTaskPool, q.blockTaskQueue, pendPool,
		metrics.NewRegisteredTimer("ubf052", nil), 1,
		func(index int, header *types.Header) error { return nil },
		func(index int, result *fetchResult) { reconstructed++ },
	)
	if reconstructed != 0 {
		t.Fatalf("reconstruct called %d times for a stale delivery slot, want 0", reconstructed)
	}
	// deliver still counts the slot as consumed, so the failure surfaces as a
	// partial one wrapping errStaleDelivery.
	if err == nil || !strings.Contains(err.Error(), errStaleDelivery.Error()) {
		t.Fatalf("delivery error = %v, want one mentioning %v", err, errStaleDelivery)
	}
}

// setHeadRecorder is a LightChain stub that only records SetHead calls.
type setHeadRecorder struct {
	LightChain
	heads []uint64
}

func (r *setHeadRecorder) SetHead(head uint64) error {
	r.heads = append(r.heads, head)
	return nil
}

// TestUBF053_ResetBelowFreezerThreshold verifies that a reorg below the freezer
// threshold rewinds to the common ancestor itself, not one block above it.
// Upstream d9c13d407.
func TestUBF053_ResetBelowFreezerThreshold(t *testing.T) {
	rec := new(setHeadRecorder)
	d := &Downloader{lightchain: rec}

	// origin+1 >= frozen: no rewind at all.
	if err := d.rewindOnReorg(99, 100); err != nil {
		t.Fatalf("rewindOnReorg: %v", err)
	}
	if len(rec.heads) != 0 {
		t.Fatalf("unexpected rewind above the freezer threshold: %v", rec.heads)
	}
	// origin+1 < frozen: rewind exactly to origin.
	if err := d.rewindOnReorg(50, 100); err != nil {
		t.Fatalf("rewindOnReorg: %v", err)
	}
	if len(rec.heads) != 1 || rec.heads[0] != 50 {
		t.Fatalf("SetHead calls = %v, want [50]", rec.heads)
	}
}

// TestUBF054_SpawnSyncKeepsRealError verifies that the spawnSync error aggregator
// keeps the last meaningful error instead of letting a nil result clear it.
// Upstream 79a57d49c (#27217).
func TestUBF054_SpawnSyncKeepsRealError(t *testing.T) {
	d := &Downloader{queue: newQueue(10, 10), cancelCh: make(chan struct{})}

	// The first fetcher reports a cancellation (which must not abort the loop),
	// the second finishes cleanly. The aggregate result must still be errCanceled.
	fetchers := []func() error{
		func() error { return errCanceled },
		func() error { return nil },
	}
	if err := d.spawnSync(fetchers); err != errCanceled {
		t.Fatalf("spawnSync error = %v, want %v", err, errCanceled)
	}
}
