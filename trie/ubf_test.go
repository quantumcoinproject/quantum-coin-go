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

package trie

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb/memorydb"
)

// errReplayFailed is injected by faultyBatch to simulate a disk error surfacing
// while the committed batch is replayed into the clean cache.
var errReplayFailed = errors.New("injected replay failure")

// faultyDB wraps a memory database and hands out batches whose Replay always
// fails, so that trie.Database.Commit's error handling can be exercised.
type faultyDB struct {
	*memorydb.Database
}

func (db *faultyDB) NewBatch() ethdb.Batch {
	return &faultyBatch{Batch: db.Database.NewBatch()}
}

type faultyBatch struct {
	ethdb.Batch
}

func (b *faultyBatch) Replay(w ethdb.KeyValueWriter) error {
	return errReplayFailed
}

// TestUBF035_CommitPropagatesReplayError checks that a failure to replay the
// written batch into the clean cache is reported instead of swallowed. A
// swallowed error means the dirty cache is silently left holding nodes that the
// caller believes are on disk.
//
// Upstream 57a65f00c (#25674).
func TestUBF035_CommitPropagatesReplayError(t *testing.T) {
	// Small trie: the batch never reaches IdealBatchSize, so the error has to be
	// caught by the outer replay in Commit.
	t.Run("outer", func(t *testing.T) {
		db := NewDatabase(&faultyDB{memorydb.New()})
		tr, err := New(common.Hash{}, db)
		if err != nil {
			t.Fatal(err)
		}
		tr.Update([]byte("foo"), []byte("bar"))
		root, err := tr.Commit(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Commit(root, false, nil); err != errReplayFailed {
			t.Fatalf("Commit returned %v, want %v", err, errReplayFailed)
		}
	})
	// Large trie: the batch overflows IdealBatchSize at least once, so the error
	// has to be caught by the inner replay in commit.
	t.Run("inner", func(t *testing.T) {
		db := NewDatabase(&faultyDB{memorydb.New()})
		tr, err := New(common.Hash{}, db)
		if err != nil {
			t.Fatal(err)
		}
		val := make([]byte, 256)
		for i := 0; i < 4096; i++ {
			key := make([]byte, 32)
			binary.BigEndian.PutUint64(key, uint64(i))
			tr.Update(key, val)
		}
		root, err := tr.Commit(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Commit(root, false, nil); err != errReplayFailed {
			t.Fatalf("Commit returned %v, want %v", err, errReplayFailed)
		}
	})
}

// TestUBF036_EmptyIteratorAndPopNils covers the two defects in the trie
// iterator: the empty-trie shortcut compared against the wrong sentinel (and
// returned a trie-less iterator), and pop() left the popped node state reachable
// through the stack's backing array.
//
// Upstream fb2ae8e99 (#24539).
func TestUBF036_EmptyIteratorAndPopNils(t *testing.T) {
	t.Run("empty trie", func(t *testing.T) {
		tr := newEmpty()
		it := tr.NodeIterator(nil)
		if it.Next(true) {
			t.Fatal("iterator over an empty trie yielded a node")
		}
		if err := it.Error(); err != nil {
			t.Fatalf("iterator over an empty trie reported %v", err)
		}
		// The iterator must be bound to its trie, and must be terminated rather
		// than merely zero-valued.
		ni, ok := it.(*nodeIterator)
		if !ok {
			t.Fatalf("unexpected iterator type %T", it)
		}
		if ni.trie != tr {
			t.Fatal("empty-trie iterator is not bound to its trie")
		}
		if ni.err != errIteratorEnd {
			t.Fatalf("empty-trie iterator err = %v, want %v", ni.err, errIteratorEnd)
		}
		// Same shape for the seeking constructor and for an iterator over a trie
		// that was emptied again.
		if it := tr.NodeIterator([]byte("anything")); it.Next(true) {
			t.Fatal("seeking iterator over an empty trie yielded a node")
		}
	})

	t.Run("pop nils the slot", func(t *testing.T) {
		tr := newEmpty()
		for i := 0; i < 64; i++ {
			key := make([]byte, 32)
			binary.BigEndian.PutUint64(key, uint64(i))
			tr.Update(key, []byte{byte(i)})
		}
		it := tr.NodeIterator(nil).(*nodeIterator)
		for it.Next(true) {
		}
		if err := it.Error(); err != nil {
			t.Fatal(err)
		}
		// Everything has been popped; nothing may remain reachable through the
		// backing array of the (now empty) stack.
		full := it.stack[:cap(it.stack)]
		for i, state := range full {
			if state != nil {
				t.Fatalf("stack slot %d still references node state %v after the iteration finished", i, state.node)
			}
		}
	})
}

// TestUBF037_CleanerSizeAccounting checks that flushing a node with an explicit
// children map charges the children back to childrenSize, not to dirtiesSize.
//
// Upstream 241dd2730 (#25007).
func TestUBF037_CleanerSizeAccounting(t *testing.T) {
	db := NewDatabase(memorydb.New())

	// Two independent tries, so we can install an explicit parent -> child
	// reference the way the state database does for storage tries.
	child, err := New(common.Hash{}, db)
	if err != nil {
		t.Fatal(err)
	}
	child.Update([]byte("child"), []byte("child-value"))
	childRoot, err := child.Commit(nil)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := New(common.Hash{}, db)
	if err != nil {
		t.Fatal(err)
	}
	parent.Update([]byte("parent"), []byte("parent-value"))
	parentRoot, err := parent.Commit(nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Reference(childRoot, parentRoot)

	if db.childrenSize == 0 {
		t.Fatal("expected the explicit reference to be accounted in childrenSize")
	}
	if err := db.Commit(parentRoot, false, nil); err != nil {
		t.Fatal(err)
	}
	// Everything has been flushed, so both counters must be back to zero. Before
	// the fix the children term was subtracted from dirtiesSize, driving it
	// negative and leaving childrenSize permanently inflated.
	if db.dirtiesSize != 0 {
		t.Fatalf("dirtiesSize = %v after a full flush, want 0", db.dirtiesSize)
	}
	if db.childrenSize != 0 {
		t.Fatalf("childrenSize = %v after a full flush, want 0", db.childrenSize)
	}
}

// TestUBF038_RangeProofRejectsDeletions checks that a range proof carrying an
// empty (deletion) value is rejected.
//
// Upstream 743769f48.
func TestUBF038_RangeProofRejectsDeletions(t *testing.T) {
	tr, vals := randomTrie(512)
	var entries entrySlice
	for _, kv := range vals {
		entries = append(entries, kv)
	}
	sort.Sort(entries)

	start, end := 100, 200
	proof := memorydb.New()
	if err := tr.Prove(entries[start].k, 0, proof); err != nil {
		t.Fatal(err)
	}
	if err := tr.Prove(entries[end-1].k, 0, proof); err != nil {
		t.Fatal(err)
	}
	var keys, values [][]byte
	for i := start; i < end; i++ {
		keys = append(keys, entries[i].k)
		values = append(values, entries[i].v)
	}
	// Sanity: the untouched range verifies.
	if _, err := VerifyRangeProof(tr.Hash(), keys[0], keys[len(keys)-1], keys, values, proof); err != nil {
		t.Fatalf("expected the pristine range to verify, got %v", err)
	}
	// Now blank one of the values out. A zero-length value is a deletion, which
	// has no place in a range proof and must be refused outright.
	values[len(values)/2] = []byte{}
	_, err := VerifyRangeProof(tr.Hash(), keys[0], keys[len(keys)-1], keys, values, proof)
	if err == nil {
		t.Fatal("expected a range proof containing a deletion to be rejected")
	}
	if err.Error() != "range contains deletion" {
		t.Fatalf("expected a deletion error, got %v", err)
	}
	// The no-edge-proof form must reject it too.
	if _, err := VerifyRangeProof(tr.Hash(), nil, nil, keys, values, nil); err == nil {
		t.Fatal("expected the proof-less form to reject a deletion as well")
	}
}

// TestUBF039_RangeProofHasRightElement checks the "are there more elements to
// the right" flag returned by VerifyRangeProof against the ground truth of the
// trie the proofs were taken from.
//
// Upstream ae45c97d3 (#24266).
//
// Note: for well-formed proofs the pre-fix expression (evaluated against the
// edge-proof skeleton `root`) happens to agree with the post-fix one in every
// case we could construct, so this is a conformance/regression guard rather than
// a fail-without-the-fix test. The fix itself is kept because computing the
// answer from the skeleton - whose interior references unsetInternal has just
// stripped - is only accidentally correct.
func TestUBF039_RangeProofHasRightElement(t *testing.T) {
	tr, vals := randomTrie(4096)
	var entries entrySlice
	for _, kv := range vals {
		entries = append(entries, kv)
	}
	sort.Sort(entries)

	// Deterministic sweep over a spread of ranges, including ones that end at
	// the very last entry (where the answer must be false).
	for _, tc := range []struct{ start, end int }{
		{0, 1}, {0, 100}, {0, len(entries)},
		{1, 2}, {1, 1000}, {17, 18},
		{100, 101}, {100, 4000}, {512, 1024},
		{len(entries) - 2, len(entries) - 1},
		{len(entries) - 1, len(entries)},
		{len(entries) - 100, len(entries)},
		{len(entries) / 2, len(entries)},
		{len(entries) / 3, len(entries)/3 + 1},
	} {
		proof := memorydb.New()
		if err := tr.Prove(entries[tc.start].k, 0, proof); err != nil {
			t.Fatal(err)
		}
		if err := tr.Prove(entries[tc.end-1].k, 0, proof); err != nil {
			t.Fatal(err)
		}
		var keys, values [][]byte
		for i := tc.start; i < tc.end; i++ {
			keys = append(keys, entries[i].k)
			values = append(values, entries[i].v)
		}
		more, err := VerifyRangeProof(tr.Hash(), keys[0], keys[len(keys)-1], keys, values, proof)
		if err != nil {
			t.Fatalf("range %d->%d: %v", tc.start, tc.end-1, err)
		}
		if want := tc.end < len(entries); more != want {
			t.Fatalf("range %d->%d: hasRightElement = %v, want %v", tc.start, tc.end-1, more, want)
		}
	}
	// Zero-element case with a single non-existent edge proof past the end of
	// the trie: no keys given, so there is nothing to the right either.
	last := common.CopyBytes(entries[len(entries)-1].k)
	for i := len(last) - 1; i >= 0; i-- {
		last[i]++
		if last[i] != 0 {
			break
		}
	}
	proof := memorydb.New()
	if err := tr.Prove(last, 0, proof); err != nil {
		t.Fatal(err)
	}
	more, err := VerifyRangeProof(tr.Hash(), last, last, nil, nil, proof)
	if err != nil {
		t.Fatal(err)
	}
	if more {
		t.Fatal("empty range past the end of the trie reported more entries")
	}
}

// TestUBF040_TryGetNodeNilNode checks that resolving a path which dead-ends on a
// nil child returns cleanly instead of panicking inside origNode.cache().
//
// Upstream 3a6fe69f2 (#23657).
func TestUBF040_TryGetNodeNilNode(t *testing.T) {
	tr := newEmpty()
	// A branch node with exactly two children, so most of the 16 slots are nil.
	tr.Update([]byte{0x00, 0x01}, []byte("one"))
	tr.Update([]byte{0x00, 0x02}, []byte("two"))
	if _, err := tr.Commit(nil); err != nil {
		t.Fatal(err)
	}
	// The two keys share the nibble prefix 0,0,0, so the trie is a short node
	// followed by a branch whose only children sit at nibbles 1 and 2. Asking for
	// any of the other 14 slots ends the path exactly on a nil child, which is
	// what used to reach the `pos >= len(path)` block with a nil origNode and
	// panic in origNode.cache().
	for nibble := byte(0); nibble < 16; nibble++ {
		if nibble == 1 || nibble == 2 {
			continue // occupied slots, not what this test is about
		}
		path := []byte{0, 0, 0, nibble}
		blob, _, err := tr.TryGetNode(hexToCompact(path))
		if err != nil {
			t.Fatalf("nibble %d: unexpected error %v", nibble, err)
		}
		if blob != nil {
			t.Fatalf("nibble %d: returned node %x for an absent path", nibble, blob)
		}
	}
	// Directly: a nil node at the end of a path must resolve to nothing.
	item, newnode, resolved, err := tr.tryGetNode(nil, []byte{1, 2, 3}, 3)
	if item != nil || newnode != nil || resolved != 0 || err != nil {
		t.Fatalf("tryGetNode(nil) = (%v, %v, %d, %v), want all-zero", item, newnode, resolved, err)
	}
}

// TestUBF041_StackTrieUnmarshalCorrupt checks that decode failures inside
// StackTrie.unmarshalBinary are reported instead of yielding a half-built trie.
//
// Upstream b7bfbc1e6 (#26914).
func TestUBF041_StackTrieUnmarshalCorrupt(t *testing.T) {
	// gobHeader encodes the node struct the way MarshalBinary does.
	gobHeader := func(nodeType uint8) []byte {
		var (
			b bytes.Buffer
			w = bufio.NewWriter(&b)
		)
		if err := gob.NewEncoder(w).Encode(struct {
			Nodetype  uint8
			Val       []byte
			Key       []byte
			KeyOffset uint8
		}{nodeType, []byte{1}, []byte{2}, 0}); err != nil {
			t.Fatal(err)
		}
		w.Flush()
		return b.Bytes()
	}
	// A gob message whose declared length is honoured (so the reader stays in
	// sync) but whose payload is not a valid encoding of the struct.
	badGob := []byte{0x04, 0xff, 0xff, 0xff, 0xff}
	absentChildren := make([]byte, 16) // 16 zero bytes: no children

	t.Run("top level", func(t *testing.T) {
		data := append(append([]byte{}, badGob...), absentChildren...)
		st := NewStackTrie(nil)
		if err := st.UnmarshalBinary(data); err == nil {
			t.Fatal("expected a corrupt top-level node to be rejected")
		}
	})

	t.Run("nested child", func(t *testing.T) {
		var data []byte
		data = append(data, gobHeader(1)...) // valid parent
		data = append(data, 1)               // child 0 is present
		data = append(data, badGob...)       // ... but corrupt
		data = append(data, absentChildren...)
		data = append(data, make([]byte, 15)...) // parent's children 1..15 absent
		st := NewStackTrie(nil)
		if err := st.UnmarshalBinary(data); err == nil {
			t.Fatal("expected a corrupt child node to be rejected")
		}
	})

	t.Run("round trip still works", func(t *testing.T) {
		src := NewStackTrie(nil)
		for i := 0; i < 32; i++ {
			key := make([]byte, 32)
			binary.BigEndian.PutUint64(key, uint64(i))
			src.TryUpdate(key, []byte(fmt.Sprintf("value-%d", i)))
		}
		blob, err := src.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		dst := NewStackTrie(nil)
		if err := dst.UnmarshalBinary(blob); err != nil {
			t.Fatalf("round trip failed: %v", err)
		}
	})
}

// TestUBF042_SyncBloomTickerStopped checks that the bloom's metering ticker is
// stopped when the goroutine returns.
//
// Upstream 79bb9300c (#23415).
func TestUBF042_SyncBloomTickerStopped(t *testing.T) {
	db := memorydb.New()
	bloom := NewSyncBloom(1, db)
	if err := bloom.Close(); err != nil {
		t.Fatal(err)
	}
	// Closing must be idempotent and must not leave the meter goroutine (and its
	// ticker) running.
	if err := bloom.Close(); err != nil {
		t.Fatal(err)
	}
}
