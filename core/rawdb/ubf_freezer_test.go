// Copyright 2019 The go-ethereum Authors
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

package rawdb

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/metrics"
)

// newUBFTable is a small helper creating a scratch freezer table in its own
// directory, so that the tests below can poke at the raw files on disk.
func newUBFTable(t *testing.T, dir string, maxFilesize uint32, readonly bool) *freezerTable {
	t.Helper()
	tab, err := newCustomTable(dir, "ubf", metrics.NilMeter{}, metrics.NilMeter{}, metrics.NilGauge{}, maxFilesize, true, readonly)
	if err != nil {
		t.Fatal(err)
	}
	return tab
}

func ubfTempDir(t *testing.T) string {
	t.Helper()
	dir, err := ioutil.TempDir("", "ubf-freezer")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// TestUBF029_AdvanceHeadSyncsBeforeClose verifies that rotating to a new data
// file fsyncs the outgoing head *before* releasing it, and that a failure to do
// so is reported rather than silently swallowed.
//
// Upstream e04d63ebd (#26490).
//
// The fsync itself is not directly observable, so the test makes it fail: the
// head descriptor is closed behind the table's back, after which Sync() must
// error. Without the fix the rotation happily releases the stale descriptor and
// writes into the fresh file, so Append reports success.
func TestUBF029_AdvanceHeadSyncsBeforeClose(t *testing.T) {
	dir := ubfTempDir(t)
	f := newUBFTable(t, dir, 50, false)
	defer f.Close()

	// Fill the first data file up to just below the 50 byte cutoff.
	for i := 0; i < 3; i++ {
		if err := f.Append(uint64(i), getChunk(15, i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if f.headId != 0 {
		t.Fatalf("expected to still be on the first data file, got %d", f.headId)
	}
	// Sabotage the head descriptor, then push the table over the cutoff so that
	// it has to rotate. The rotation must notice that the outgoing head cannot
	// be committed to stable storage.
	if err := f.head.Close(); err != nil {
		t.Fatal(err)
	}
	err := f.Append(3, getChunk(15, 3))
	if err == nil {
		t.Fatal("expected the rotation to report the failed fsync of the outgoing head file")
	}
	if !strings.Contains(err.Error(), "sync") {
		t.Fatalf("expected an fsync error from the outgoing head, got %v", err)
	}
}

// TestUBF030_CloseSyncsHead verifies that Close fsyncs the read/write
// descriptors before tearing them down.
//
// Upstream 0b53b2907 (#26485).
func TestUBF030_CloseSyncsHead(t *testing.T) {
	// A healthy table must still close without complaint.
	func() {
		dir := ubfTempDir(t)
		f := newUBFTable(t, dir, 50, false)
		for i := 0; i < 5; i++ {
			if err := f.Append(uint64(i), getChunk(15, i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.Close(); err != nil {
			t.Fatalf("clean close reported an error: %v", err)
		}
	}()

	// Now make the head un-syncable. Close must surface the fsync failure; before
	// the fix it only ever reported errors coming from Close() itself.
	dir := ubfTempDir(t)
	f := newUBFTable(t, dir, 50, false)
	for i := 0; i < 5; i++ {
		if err := f.Append(uint64(i), getChunk(15, i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.head.Close(); err != nil {
		t.Fatal(err)
	}
	err := f.Close()
	if err == nil {
		t.Fatal("expected Close to report the failed head fsync")
	}
	if !strings.Contains(err.Error(), "sync") {
		t.Fatalf("expected an fsync error in %q", err.Error())
	}
}

// TestUBF031_FreezeBackoffTimerReused exercises the freeze loop's backoff path
// repeatedly. The upstream fix hoists the backoff timer out of the loop (so it
// is allocated once and stopped on exit); the risk that introduces is a missing
// Reset, which would wedge the loop after the first backoff. This test walks the
// loop through many backoff cycles and then shuts it down cleanly.
//
// Upstream 83989a19b (#25776).
func TestUBF031_FreezeBackoffTimerReused(t *testing.T) {
	dir := ubfTempDir(t)
	f, err := newFreezer(filepath.Join(dir, "ancient"), "test/", false, FreezerModeSkipNone)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// An empty key-value store has no head block, so every iteration of the
	// freeze loop immediately backs off.
	db := NewMemoryDatabase()

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		f.freeze(db)
	}()

	// Drive the loop through several backoff cycles via the manual trigger. If
	// the hoisted timer were mis-handled the loop would stop making progress and
	// these sends would block until the test times out.
	for i := 0; i < 10; i++ {
		trigger := make(chan struct{}, 1)
		f.trigger <- trigger
		<-trigger
	}
	if err := f.Close(); err != nil {
		t.Fatalf("freezer shutdown failed: %v", err)
	}
}

// TestUBF032_ReadonlyFreezerDoesNotTruncate verifies that opening a table in
// read-only mode never repairs (i.e. truncates) the files on disk.
//
// Upstream 4aab440ee (#24119).
func TestUBF032_ReadonlyFreezerDoesNotTruncate(t *testing.T) {
	dir := ubfTempDir(t)

	// Build a table spanning a couple of data files, then close it cleanly.
	f := newUBFTable(t, dir, 50, false)
	for i := 0; i < 10; i++ {
		if err := f.Append(uint64(i), getChunk(15, i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash that left junk at the end of the head data file. A
	// read-write open repairs this by truncating; a read-only open must not.
	headName := filepath.Join(dir, fmt.Sprintf("ubf.%04d.rdat", f.headId))
	fd, err := os.OpenFile(headName, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fd.Write(getChunk(17, 0xff)); err != nil {
		t.Fatal(err)
	}
	fd.Close()

	stat, err := os.Stat(headName)
	if err != nil {
		t.Fatal(err)
	}
	corruptSize := stat.Size()

	// Read-only open: must refuse, and must leave the file untouched.
	ro, err := newCustomTable(dir, "ubf", metrics.NilMeter{}, metrics.NilMeter{}, metrics.NilGauge{}, 50, true, true)
	if err == nil {
		ro.Close()
		t.Fatal("expected a read-only open of a corrupted table to fail")
	}
	if !strings.Contains(err.Error(), "corrupted") {
		t.Fatalf("expected a corruption error, got %v", err)
	}
	if stat, err = os.Stat(headName); err != nil {
		t.Fatal(err)
	}
	if stat.Size() != corruptSize {
		t.Fatalf("read-only open truncated the head file: have %d, want %d", stat.Size(), corruptSize)
	}
	// Sanity check: a read-write open still repairs it.
	rw := newUBFTable(t, dir, 50, false)
	defer rw.Close()
	if stat, err = os.Stat(headName); err != nil {
		t.Fatal(err)
	}
	if stat.Size() == corruptSize {
		t.Fatal("read-write open failed to repair the dangling head")
	}
}

// TestUBF033_IndexLogDerefsBlockNumber checks that the tx-index accessors log
// the block *number*, not the address of the pointer holding it.
//
// Upstream d3e3a460e (#23328).
func TestUBF033_IndexLogDerefsBlockNumber(t *testing.T) {
	var (
		mu      sync.Mutex
		numbers []interface{}
	)
	old := log.Root().GetHandler()
	log.Root().SetHandler(log.FuncHandler(func(r *log.Record) error {
		mu.Lock()
		defer mu.Unlock()
		for i := 0; i+1 < len(r.Ctx); i += 2 {
			if key, ok := r.Ctx[i].(string); ok && key == "number" {
				numbers = append(numbers, r.Ctx[i+1])
			}
		}
		return nil
	}))
	defer log.Root().SetHandler(old)

	db := NewMemoryDatabase()
	// A lookup entry pointing at a canonical hash whose body is missing drives
	// ReadTransaction into the "Transaction referenced missing" branch.
	blockHash := common.HexToHash("0x1234")
	txHash := common.HexToHash("0x5678")
	WriteCanonicalHash(db, blockHash, 42)
	WriteTxLookupEntries(db, 42, []common.Hash{txHash})

	if tx, _, _, _ := ReadTransaction(db, txHash); tx != nil {
		t.Fatal("expected no transaction to be found")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(numbers) == 0 {
		t.Fatal("expected the missing-body path to log a block number")
	}
	for _, n := range numbers {
		if _, isPtr := n.(*uint64); isPtr {
			t.Fatalf("block number logged as a *uint64 pointer instead of its value")
		}
		if got, ok := n.(uint64); !ok || got != 42 {
			t.Fatalf("block number logged as %#v, want uint64(42)", n)
		}
	}
}

// TestUBF034_SyncTakesLockAndAggregates verifies that freezerTable.Sync takes
// the table lock (so it cannot race with append/truncate swapping descriptors)
// and that it rejects a closed table instead of dereferencing a nil descriptor.
//
// Upstream 193f350eb (#26245).
func TestUBF034_SyncTakesLockAndAggregates(t *testing.T) {
	// Sync on a closed table must report errClosed. Before the fix it walked
	// straight into a nil *os.File and panicked.
	dir := ubfTempDir(t)
	f := newUBFTable(t, dir, 50, false)
	if err := f.Append(0, getChunk(15, 0)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != errClosed {
		t.Fatalf("Sync on a closed table returned %v, want %v", err, errClosed)
	}

	// Sync must not race with the mutators that swap t.head. Run under -race.
	dir2 := ubfTempDir(t)
	g := newUBFTable(t, dir2, 50, false)
	defer g.Close()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			g.Append(uint64(i), getChunk(15, i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			g.Sync()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			g.truncate(uint64(rand.Intn(10)))
		}
	}()
	wg.Wait()
}
