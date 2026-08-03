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

package filters

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
)

// TestUBF081_UnindexedLogsRespectsCancel verifies that an unindexed log scan aborts
// as soon as the request context is cancelled.
// Upstream f53ff0ff4 (#26320).
func TestUBF081_UnindexedLogsRespectsCancel(t *testing.T) {
	backend := &testBackend{db: rawdb.NewMemoryDatabase()}
	filter := NewRangeFilter(backend, 0, 1000, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logs, err := filter.unindexedLogs(ctx, 1000)
	if len(logs) != 0 {
		t.Fatalf("unexpected logs: %d", len(logs))
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unindexedLogs error = %v, want %v", err, context.Canceled)
	}
	if filter.begin != 0 {
		t.Fatalf("scan advanced to block %d despite a cancelled context", filter.begin)
	}
}

// TestUBF084_TopicLimits verifies that filter criteria carrying an absurd number of
// topics are rejected before anything is sized by them.
// Upstream 7bcb5532a (#29535).
func TestUBF084_TopicLimits(t *testing.T) {
	topic := `"0x0000000000000000000000000000000000000000000000000000000000000000"`

	outer := make([]string, maxTopics+1)
	for i := range outer {
		outer[i] = topic
	}
	inner := make([]string, maxSubTopics+1)
	for i := range inner {
		inner[i] = topic
	}
	// More outer topics than a log can ever have.
	crit := new(FilterCriteria)
	if err := json.Unmarshal([]byte(`{"topics":[`+strings.Join(outer, ",")+`]}`), crit); !errors.Is(err, errExceedMaxTopics) {
		t.Fatalf("unmarshal with %d topics: err = %v, want %v", len(outer), err, errExceedMaxTopics)
	}
	// An over-long sub-topic list.
	crit = new(FilterCriteria)
	if err := json.Unmarshal([]byte(`{"topics":[[`+strings.Join(inner, ",")+`]]}`), crit); !errors.Is(err, errExceedMaxTopics) {
		t.Fatalf("unmarshal with %d sub-topics: err = %v, want %v", len(inner), err, errExceedMaxTopics)
	}
	// Criteria sitting exactly on the limits must still be accepted.
	crit = new(FilterCriteria)
	if err := json.Unmarshal([]byte(`{"topics":[`+strings.Join(outer[:maxTopics], ",")+`]}`), crit); err != nil {
		t.Fatalf("unmarshal with %d topics failed: %v", maxTopics, err)
	}
	crit = new(FilterCriteria)
	if err := json.Unmarshal([]byte(`{"topics":[[`+strings.Join(inner[:maxSubTopics], ",")+`]]}`), crit); err != nil {
		t.Fatalf("unmarshal with %d sub-topics failed: %v", maxSubTopics, err)
	}
}

// TestUBF086_SubscriptionInstalledBeforeReturn verifies that the newHeads and
// pending-transaction subscriptions are installed in the event system before the
// RPC call returns, so that no event fired in between is lost.
//
// The check works by wedging the event loop: while it is stuck delivering a header
// to a channel nobody reads it cannot service an install request either, so a
// correct implementation cannot possibly have answered the RPC yet.
// Upstream de0a452f7 (#33990).
func TestUBF086_SubscriptionInstalledBeforeReturn(t *testing.T) {
	for _, method := range []string{"newHeads", "newPendingTransactions"} {
		method := method
		t.Run(method, func(t *testing.T) {
			var (
				backend = &testBackend{db: rawdb.NewMemoryDatabase()}
				api     = NewPublicFilterAPI(backend, false, deadline)
			)
			// Two unbuffered header sinks that nobody drains.
			first, second := make(chan *types.Header), make(chan *types.Header)
			api.events.SubscribeNewHeads(first)
			api.events.SubscribeNewHeads(second)

			block := types.NewBlockWithHeader(&types.Header{Number: big.NewInt(1)})
			go backend.chainFeed.Send(core.ChainEvent{Block: block})

			// The loop delivers to the two sinks one after the other; draining one
			// leaves it firmly wedged on the other.
			var wedged chan *types.Header
			select {
			case <-first:
				wedged = second
			case <-second:
				wedged = first
			case <-time.After(30 * time.Second):
				t.Fatalf("event loop never delivered the chain event")
			}
			server := rpc.NewServer()
			if err := server.RegisterName("eth", api); err != nil {
				t.Fatalf("failed to register the filter API: %v", err)
			}
			defer server.Stop()

			client := rpc.DialInProc(server)
			defer client.Close()

			subscribed := make(chan error, 1)
			go func() {
				sink := make(chan interface{}, 1)
				sub, err := client.Subscribe(context.Background(), "eth", sink, method)
				if sub != nil {
					sub.Unsubscribe()
				}
				subscribed <- err
			}()
			select {
			case err := <-subscribed:
				t.Fatalf("eth_%s answered (err=%v) while the event system was wedged: the subscription is installed only after the RPC response, so events fired in between are lost", method, err)
			case <-time.After(time.Second):
				// Still blocked inside the event system, which is what we want.
			}
			// Free the event loop and let the pending subscribe complete.
			<-wedged
			select {
			case err := <-subscribed:
				if err != nil {
					t.Fatalf("eth_%s failed: %v", method, err)
				}
			case <-time.After(30 * time.Second):
				t.Fatalf("eth_%s never completed", method)
			}
		})
	}
}

// pendingTestBackend augments testBackend with the miner's pending block.
type pendingTestBackend struct {
	*testBackend
	block    *types.Block
	receipts types.Receipts
}

func (b *pendingTestBackend) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	return b.block, b.receipts
}

// TestUBF095_PendingLogsIncluded verifies that eth_getLogs serves logs out of the
// pending block, rejects a partially-pending range and survives a backend that has
// no pending block at all.
// Upstream d9566e39b (#24949).
func TestUBF095_PendingLogsIncluded(t *testing.T) {
	var (
		db      = rawdb.NewMemoryDatabase()
		addr    = common.Address{0xaa}
		log1    = &types.Log{Address: addr, Topics: []common.Hash{{0x01}}}
		receipt = &types.Receipt{Logs: []*types.Log{log1}}
	)
	pending := types.NewBlockWithHeader(&types.Header{
		Number: big.NewInt(1),
		Bloom:  types.CreateBloom(types.Receipts{receipt}),
	})
	backend := &pendingTestBackend{
		testBackend: &testBackend{db: db},
		block:       pending,
		receipts:    types.Receipts{receipt},
	}
	pendingNum := rpc.PendingBlockNumber.Int64()

	// A pending-only range must return the pending logs.
	logs, err := NewRangeFilter(backend, pendingNum, pendingNum, []common.Address{addr}, nil).Logs(context.Background())
	if err != nil {
		t.Fatalf("pending filter failed: %v", err)
	}
	if len(logs) != 1 || logs[0].Address != addr {
		t.Fatalf("pending logs = %v, want the single pending log", logs)
	}
	// A range that starts pending but ends elsewhere is nonsensical.
	if _, err := NewRangeFilter(backend, pendingNum, 5, []common.Address{addr}, nil).Logs(context.Background()); !errors.Is(err, errInvalidBlockRange) {
		t.Fatalf("half-pending range error = %v, want %v", err, errInvalidBlockRange)
	}
	// A backend with no pending block must not panic.
	empty := &pendingTestBackend{testBackend: &testBackend{db: db}}
	if logs, err = NewRangeFilter(empty, pendingNum, pendingNum, []common.Address{addr}, nil).Logs(context.Background()); err != nil {
		t.Fatalf("nil pending block filter failed: %v", err)
	} else if len(logs) != 0 {
		t.Fatalf("nil pending block yielded %d logs", len(logs))
	}
	// Neither must a backend that cannot serve a pending block at all.
	if logs, err = NewRangeFilter(&testBackend{db: db}, pendingNum, pendingNum, []common.Address{addr}, nil).Logs(context.Background()); err != nil {
		t.Fatalf("pending-unaware backend filter failed: %v", err)
	} else if len(logs) != 0 {
		t.Fatalf("pending-unaware backend yielded %d logs", len(logs))
	}
}

// TestUBF096_GetLogsRangeValidation verifies that eth_getLogs rejects an inverted
// block range and a blockHash combined with a from/to range.
// Upstream f20b334f2 (#28386) and 038ff766f (#31877).
func TestUBF096_GetLogsRangeValidation(t *testing.T) {
	var (
		db      = rawdb.NewMemoryDatabase()
		backend = &testBackend{db: db}
		api     = NewPublicFilterAPI(backend, false, deadline)
		hash    = common.Hash{0x01}
	)
	if _, err := api.GetLogs(context.Background(), FilterCriteria{
		FromBlock: big.NewInt(10),
		ToBlock:   big.NewInt(5),
	}); !errors.Is(err, errInvalidBlockRange) {
		t.Fatalf("inverted range error = %v, want %v", err, errInvalidBlockRange)
	}
	if _, err := api.GetLogs(context.Background(), FilterCriteria{
		BlockHash: &hash,
		FromBlock: big.NewInt(1),
	}); !errors.Is(err, errBlockHashWithRange) {
		t.Fatalf("blockHash+fromBlock error = %v, want %v", err, errBlockHashWithRange)
	}
	if _, err := api.GetLogs(context.Background(), FilterCriteria{
		BlockHash: &hash,
		ToBlock:   big.NewInt(1),
	}); !errors.Is(err, errBlockHashWithRange) {
		t.Fatalf("blockHash+toBlock error = %v, want %v", err, errBlockHashWithRange)
	}
	// A well formed range must still be accepted.
	if _, err := api.GetLogs(context.Background(), FilterCriteria{
		FromBlock: big.NewInt(5),
		ToBlock:   big.NewInt(10),
	}); errors.Is(err, errInvalidBlockRange) || errors.Is(err, errBlockHashWithRange) {
		t.Fatalf("valid range rejected: %v", err)
	}
}

// TestUBF128_TimeoutLoopTerminates verifies that the filter timeout loop exits when
// the event system it belongs to shuts down, instead of leaking for the lifetime of
// the process.
// Upstream 6c10996bf (#31056).
func TestUBF128_TimeoutLoopTerminates(t *testing.T) {
	// Track this API's own goroutine by id. Filter APIs created by other tests in
	// this package are never torn down, so a global count of timeoutLoop goroutines
	// is contaminated by them and makes this test order-dependent.
	//
	// Wait for the baseline to stop changing before snapshotting it. filter_system_test.go
	// creates eight filter APIs and runs first; if one of their goroutines is still
	// starting when the baseline is taken, it shows up as "new" here and gets adopted
	// as ours — and since those tests never shut down, it never exits and this test
	// times out. That is what made it fail only under `go test ./...`.
	var before map[string]bool
	for stable, start := 0, time.Now(); stable < 3; {
		prev := before
		before = timeoutLoopIDs()
		if prev != nil && len(prev) == len(before) {
			same := true
			for id := range before {
				if !prev[id] {
					same = false
					break
				}
			}
			if same {
				stable++
			} else {
				stable = 0
			}
		}
		if time.Since(start) > 30*time.Second {
			t.Fatal("goroutine baseline never stabilised")
		}
		time.Sleep(100 * time.Millisecond)
	}

	backend := &testBackend{db: rawdb.NewMemoryDatabase()}
	// A timeout long enough that the ticker can never be what wakes the loop, so
	// only the shutdown path can end it.
	api := NewPublicFilterAPI(backend, false, time.Hour)

	// Poll slowly: timeoutLoopIDs stops the world to walk every goroutine stack, so
	// polling it tightly starves the very goroutine we are waiting on. That made
	// this test fail under `go test ./...`, where package binaries run in parallel,
	// while passing in isolation.
	const (
		pollInterval = 200 * time.Millisecond
		pollBudget   = 30 * time.Second
	)

	var ourID string
	for start := time.Now(); ourID == ""; {
		for id := range timeoutLoopIDs() {
			if !before[id] {
				ourID = id
				break
			}
		}
		if ourID != "" {
			break
		}
		if time.Since(start) > pollBudget {
			t.Fatal("timeout loop was never started")
		}
		time.Sleep(pollInterval)
	}

	// Tear the event system down the way a node shutdown does.
	api.events.chainSub.Unsubscribe()

	for start := time.Now(); timeoutLoopIDs()[ourID]; {
		if time.Since(start) > pollBudget {
			t.Fatalf("timeout loop (goroutine %s) still running after the event system shut down", ourID)
		}
		time.Sleep(pollInterval)
	}
}

// timeoutLoopIDs returns the ids of the goroutines currently running the
// PublicFilterAPI timeout loop, keyed for set membership.
func timeoutLoopIDs() map[string]bool {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n == len(buf) { // truncated, retry with a larger buffer
			buf = make([]byte, 2*len(buf))
			continue
		}
		ids := make(map[string]bool)
		for _, g := range strings.Split(string(buf[:n]), "\n\ngoroutine ") {
			if !strings.Contains(g, "filters.(*PublicFilterAPI).timeoutLoop") {
				continue
			}
			// The block starts with "<id> [status]:" (the very first block in the
			// dump still carries its "goroutine " prefix).
			g = strings.TrimPrefix(g, "goroutine ")
			if i := strings.IndexAny(g, " \t"); i > 0 {
				ids[g[:i]] = true
			}
		}
		return ids
	}
}
