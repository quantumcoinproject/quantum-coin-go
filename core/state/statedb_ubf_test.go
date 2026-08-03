// Tests for upstream bug fixes ported into core/state.
// See docs/upstream-bugfix-audit-2026-08.md (UBF-018, UBF-025, UBF-027).

package state

import (
	"errors"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
)

// TestUBF018_GetLogsStampsBlockNumber checks that GetLogs stamps both the block hash
// and the block number onto each log. Upstream cda051eba: without the block number,
// logs surfaced on the live feed or during tracing carried BlockNumber 0 until
// Receipts.DeriveFields ran much later.
func TestUBF018_GetLogsStampsBlockNumber(t *testing.T) {
	db := NewDatabase(rawdb.NewMemoryDatabase())
	statedb, err := New(common.Hash{}, db, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	txHash := common.HexToHash("0xdead")
	statedb.Prepare(txHash, 0)
	statedb.AddLog(&types.Log{Address: common.BytesToAddress([]byte{0x01})})
	statedb.AddLog(&types.Log{Address: common.BytesToAddress([]byte{0x02})})

	const wantNumber = uint64(1234)
	wantHash := common.HexToHash("0xbeef")

	logs := statedb.GetLogs(txHash, wantNumber, wantHash)
	if len(logs) != 2 {
		t.Fatalf("got %d logs, want 2", len(logs))
	}
	for i, l := range logs {
		if l.BlockNumber != wantNumber {
			t.Errorf("log %d: BlockNumber = %d, want %d", i, l.BlockNumber, wantNumber)
		}
		if l.BlockHash != wantHash {
			t.Errorf("log %d: BlockHash = %x, want %x", i, l.BlockHash, wantHash)
		}
	}
}

// TestUBF027_PrepareClearsAccessList checks that PrepareAccessList resets the access
// list, so a StateDB reused across calls (as the RPC paths do) cannot inherit warm
// slots from a previous execution. Upstream 48605b5f6 moved the reset out of Prepare,
// which block execution always calls, into PrepareAccessList, which reuse paths hit.
func TestUBF027_PrepareClearsAccessList(t *testing.T) {
	db := NewDatabase(rawdb.NewMemoryDatabase())
	statedb, err := New(common.Hash{}, db, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	stale := common.BytesToAddress([]byte{0xaa})
	staleSlot := common.HexToHash("0x1")
	statedb.AddAddressToAccessList(stale)
	statedb.AddSlotToAccessList(stale, staleSlot)
	if !statedb.AddressInAccessList(stale) {
		t.Fatal("setup: stale address should be warm")
	}

	// A second execution prepares its own access list; the leftovers must be gone.
	sender := common.BytesToAddress([]byte{0xbb})
	statedb.PrepareAccessList(sender, nil, nil, nil)

	if statedb.AddressInAccessList(stale) {
		t.Error("stale address still warm after PrepareAccessList")
	}
	if _, slotWarm := statedb.SlotInAccessList(stale, staleSlot); slotWarm {
		t.Error("stale slot still warm after PrepareAccessList")
	}
	if !statedb.AddressInAccessList(sender) {
		t.Error("sender should be warm after PrepareAccessList")
	}
}

// errOpenStorageTrie is returned by the stub below to simulate a corrupt or
// unavailable storage trie.
var errOpenStorageTrie = errors.New("simulated storage trie failure")

// failingStorageTrieDB wraps a Database and fails every storage-trie open.
type failingStorageTrieDB struct {
	Database
}

func (db failingStorageTrieDB) OpenStorageTrie(addrHash, root common.Hash) (Trie, error) {
	return nil, errOpenStorageTrie
}

// TestUBF025_StorageTrieOpenErrorSurfaces checks that a storage-trie open failure is
// reported instead of being papered over with an empty trie. Upstream 01808421e: the
// old code substituted an EMPTY storage trie on error, so a transient or corrupt
// database turned into silently wrong (empty) storage and therefore a wrong root.
func TestUBF025_StorageTrieOpenErrorSurfaces(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x42})
	key := common.HexToHash("0x1")
	value := common.HexToHash("0x99")

	// Build a state that genuinely has a storage slot, and commit it so the object
	// has a non-empty storage root that must be opened on the next read.
	memdb := rawdb.NewMemoryDatabase()
	db := NewDatabase(memdb)
	statedb, err := New(common.Hash{}, db, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	statedb.SetState(addr, key, value)
	root, err := statedb.Commit(false)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := db.TrieDB().Commit(root, false, nil); err != nil {
		t.Fatalf("TrieDB Commit: %v", err)
	}

	// Sanity check: with a healthy database the slot reads back.
	good, err := New(root, NewDatabase(memdb), nil)
	if err != nil {
		t.Fatalf("New(good): %v", err)
	}
	if got := good.GetCommittedState(addr, key); got != value {
		t.Fatalf("baseline read = %x, want %x", got, value)
	}

	// Now against a database whose storage-trie opens all fail.
	broken, err := New(root, failingStorageTrieDB{NewDatabase(memdb)}, nil)
	if err != nil {
		t.Fatalf("New(broken): %v", err)
	}

	// The read itself cannot succeed either way. What matters is what it leaves
	// behind: the pre-fix code opened an EMPTY storage trie and cached it on the
	// object, so StorageTrie then handed callers a plausible-looking empty trie and
	// the object would hash to the empty storage root. Post-fix there is no trie.
	if got := broken.GetCommittedState(addr, key); got != (common.Hash{}) {
		t.Fatalf("broken read = %x, want empty (the read cannot succeed)", got)
	}
	if tr := broken.StorageTrie(addr); tr != nil {
		t.Fatalf("StorageTrie returned a non-nil trie (root %x) after the open failed; "+
			"the empty-trie substitution is still present", tr.Hash())
	}

	// A write genuinely needs the storage trie, so committing one must report the
	// failure rather than persisting a root derived from an empty substitute.
	writer, err := New(root, failingStorageTrieDB{NewDatabase(memdb)}, nil)
	if err != nil {
		t.Fatalf("New(writer): %v", err)
	}
	writer.SetState(addr, common.HexToHash("0x2"), common.HexToHash("0x7"))
	if _, err := writer.Commit(false); err == nil {
		t.Error("Commit succeeded despite the storage trie being unopenable")
	} else if !errors.Is(err, errOpenStorageTrie) {
		t.Logf("Commit reported: %v", err) // a wrapped error is fine, silence is not
	}
}
