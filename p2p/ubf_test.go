// Copyright 2014 The go-ethereum Authors
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

package p2p

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common/mclock"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/p2p/enode"
)

// TestUBF100_PingFloodBoundedGoroutines checks that an inbound ping flood cannot make
// the node spawn an unbounded number of goroutines. Upstream 7ec60d5f0 (CVE-2023-40591)
// moved the pong reply from a per-message `go SendItems(...)` into pingLoop, which is fed
// by a small buffered channel.
func TestUBF100_PingFloodBoundedGoroutines(t *testing.T) {
	closer, rw, _, _ := testPeer(nil)
	defer closer()

	// Let the peer settle: one ping/pong round trip before we start counting.
	if err := SendItems(rw, pingMsg); err != nil {
		t.Fatal(err)
	}
	if err := ExpectMsg(rw, pongMsg, nil); err != nil {
		t.Fatal(err)
	}

	const pings = 500
	before := runtime.NumGoroutine()

	// Flood the peer with pings and never read the pongs it wants to send back. Before
	// the fix each ping started its own goroutine which then blocked writing its pong,
	// so the goroutine count grew roughly with the number of pings. With the fix the
	// reply is serialised through pingLoop and readLoop stops accepting pings once the
	// small pingRecv buffer is full.
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		for i := 0; i < pings; i++ {
			if err := SendItems(rw, pingMsg); err != nil {
				return
			}
		}
	}()

	select {
	case <-sent:
	case <-time.After(2 * time.Second):
		// Expected with the fix in place: the sender is blocked.
	}

	// Allow a generous margin for unrelated runtime goroutines.
	const maxGrowth = 100
	if growth := runtime.NumGoroutine() - before; growth > maxGrowth {
		t.Fatalf("ping flood spawned %d goroutines (limit %d), pong handling is unbounded", growth, maxGrowth)
	}
}

// TestUBF101_MalformedDiscReasonDecode feeds malformed and out-of-range disconnect
// messages and checks that nothing panics and the reason stays inside a single byte.
// Upstream 870b4505a (CVE-2022-29177).
func TestUBF101_MalformedDiscReasonDecode(t *testing.T) {
	// DiscReason must be a single byte, otherwise a peer-supplied value can be used to
	// index discReasonToString out of range (int(d) wraps negative for huge values, so
	// the length check in String does not catch it).
	var overflow uint64 = 256
	if DiscReason(overflow) != 0 {
		t.Fatalf("DiscReason is wider than a byte: DiscReason(256) = %d", uint64(DiscReason(overflow)))
	}
	for i := 0; i < 256; i++ {
		_ = DiscReason(i).String() // must not panic for any wire-representable value
	}

	tests := []struct {
		name    string
		payload interface{}
	}{
		{"huge reason", []interface{}{uint64(math.MaxUint64)}},
		{"out of range reason", []interface{}{uint(200)}},
		{"empty list", []interface{}{}},
		{"too many elements", []interface{}{uint(1), uint(2)}},
		{"not a list", "garbage"},
		{"nested list", []interface{}{[]interface{}{uint(1)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			closer, rw, _, disc := testPeer(nil)
			defer closer()

			if err := Send(rw, discMsg, test.payload); err != nil {
				t.Fatalf("send error: %v", err)
			}
			select {
			case err := <-disc:
				if err == nil {
					t.Fatal("peer returned without an error")
				}
				// Must not panic. Before the fix a huge value indexed
				// discReasonToString out of range right here.
				_ = err.Error()
				if reason, ok := err.(DiscReason); ok && uint64(reason) > math.MaxUint8 {
					t.Fatalf("decoded out-of-range disconnect reason %d", uint64(reason))
				}
			case <-time.After(5 * time.Second):
				t.Fatal("peer did not return")
			}
		})
	}
}

// resolverFunc adapts a function to the nodeResolver interface.
type resolverFunc func(*enode.Node) *enode.Node

func (f resolverFunc) Resolve(n *enode.Node) *enode.Node { return f(n) }

// TestUBF102_DialTaskDestRace runs dialTask.resolve, which rewrites the task destination
// on the task goroutine, concurrently with the scheduler-side reads of that destination.
// Upstream 758fce71f. Run with -race.
func TestUBF102_DialTaskDestRace(t *testing.T) {
	nodes := make([]*enode.Node, 16)
	for i := range nodes {
		nodes[i] = newNode(uintID(uint16(i+1)), "127.0.0.1:30303")
	}

	var next int
	d := &dialScheduler{
		dialConfig: dialConfig{
			log:   log.Root(),
			clock: mclock.System{},
			resolver: resolverFunc(func(*enode.Node) *enode.Node {
				next = (next + 1) % len(nodes)
				return nodes[next]
			}),
		},
	}
	task := newDialTask(nodes[0], staticDialedConn)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// The task goroutine: repeatedly resolve, which stores a new destination.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			task.lastResolved = 0 // defeat the resolve backoff
			task.resolve(d)
		}
	}()

	// The scheduler goroutine: read the destination the way loop/updateStaticPool do.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = task.dest().ID()
			_ = task.String()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestUBF104_WrappedErrorsRecognised checks that error comparisons unwrap.
// Upstream 138f0d749.
func TestUBF104_WrappedErrorsRecognised(t *testing.T) {
	wrapped := fmt.Errorf("protocol failed: %w", errProtocolReturned)
	if got := discReasonForError(wrapped); got != DiscQuitting {
		t.Errorf("discReasonForError(wrapped errProtocolReturned) = %v, want %v", got, DiscQuitting)
	}
	// The unwrapped case must keep working too.
	if got := discReasonForError(errProtocolReturned); got != DiscQuitting {
		t.Errorf("discReasonForError(errProtocolReturned) = %v, want %v", got, DiscQuitting)
	}
}
