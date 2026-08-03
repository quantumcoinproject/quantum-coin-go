// Tests for upstream bug fixes ported into core/state.
// See docs/upstream-bugfix-audit-2026-08.md (UBF-004, UBF-006, UBF-018,
// UBF-025, UBF-027) and docs/statedb-recreated-account-fix-plan.md.

package state

import (
	"errors"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state/snapshot"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
)

// setUpstreamFixesGate points the UpstreamConsensusFixesV1 gate at the given block
// and restores the previous value when the test ends.
func setUpstreamFixesGate(t *testing.T, block uint64) {
	t.Helper()
	old := defaults.DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock
	t.Cleanup(func() {
		defaults.DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock = old
	})
	defaults.DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock = block
}

// destructTestState commits an account with a balance and one storage slot and
// returns the backing database, the committed root, and the storage root the
// account carries in that committed state.
func destructTestState(t *testing.T, addr common.Address, key, val common.Hash) (ethdb.Database, Database, common.Hash, common.Hash) {
	t.Helper()
	memdb := rawdb.NewMemoryDatabase()
	db := NewDatabase(memdb)
	base, err := New(common.Hash{}, db, nil, nil)
	if err != nil {
		t.Fatalf("New(base): %v", err)
	}
	base.SetBalance(addr, big.NewInt(1))
	base.SetState(addr, key, val)
	root, err := base.Commit(false)
	if err != nil {
		t.Fatalf("Commit(base): %v", err)
	}
	reader, err := New(root, db, nil, nil)
	if err != nil {
		t.Fatalf("New(reader): %v", err)
	}
	obj := reader.getStateObject(addr)
	if obj == nil {
		t.Fatal("committed account missing")
	}
	if obj.data.Root == emptyRoot {
		t.Fatal("committed account has an empty storage root; the scenario needs real storage")
	}
	return memdb, db, root, obj.data.Root
}

// TestUBF018_GetLogsStampsBlockNumber checks that GetLogs stamps both the block hash
// and the block number onto each log. Upstream cda051eba: without the block number,
// logs surfaced on the live feed or during tracing carried BlockNumber 0 until
// Receipts.DeriveFields ran much later.
func TestUBF018_GetLogsStampsBlockNumber(t *testing.T) {
	db := NewDatabase(rawdb.NewMemoryDatabase())
	statedb, err := New(common.Hash{}, db, nil, nil)
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
	statedb, err := New(common.Hash{}, db, nil, nil)
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
	statedb, err := New(common.Hash{}, db, nil, nil)
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
	good, err := New(root, NewDatabase(memdb), nil, nil)
	if err != nil {
		t.Fatalf("New(good): %v", err)
	}
	if got := good.GetCommittedState(addr, key); got != value {
		t.Fatalf("baseline read = %x, want %x", got, value)
	}

	// Now against a database whose storage-trie opens all fail.
	broken, err := New(root, failingStorageTrieDB{NewDatabase(memdb)}, nil, nil)
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
	writer, err := New(root, failingStorageTrieDB{NewDatabase(memdb)}, nil, nil)
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

// TestUBF004_DestructTrackedWithoutSnapshot checks that a same-block destruction is
// tracked in stateObjectsDestruct regardless of snapshot availability, and that the
// hoisted GetCommittedState check (upstream c87f321b8) answers "empty" for the
// re-created account even when the object carries a non-empty storage root.
//
// On this vintage a resurrected object is built with an empty storage root, so the
// trie path happens to read empty without the fix; the discriminating assertion
// therefore forces the object's root back to the pre-destruct storage root —
// exactly the class of state the hoisted check exists to guard — and expects the
// gated check, not root scoping, to deliver the empty answer.
func TestUBF004_DestructTrackedWithoutSnapshot(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x11})
	key := common.HexToHash("0x1")
	val := common.HexToHash("0x99")

	run := func(t *testing.T, active bool) {
		if active {
			setUpstreamFixesGate(t, 5)
		} else {
			setUpstreamFixesGate(t, 100)
		}
		_, db, root, storageRoot := destructTestState(t, addr, key, val)

		st, err := New(root, db, nil, big.NewInt(10))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if !st.Suicide(addr) {
			t.Fatal("suicide failed")
		}
		st.Finalise(true)

		if _, tracked := st.stateObjectsDestruct[addr]; !tracked {
			t.Fatal("destruction not tracked in stateObjectsDestruct without a snapshot")
		}

		// Resurrect and force the pre-destruct storage root onto the fresh object.
		st.CreateAccount(addr)
		obj := st.getStateObject(addr)
		if obj == nil {
			t.Fatal("resurrected object missing")
		}
		obj.data.Root = storageRoot

		got := st.GetCommittedState(addr, key)
		if active {
			if got != (common.Hash{}) {
				t.Fatalf("gate active: committed storage of re-created account = %x, want empty", got)
			}
		} else {
			// Below the gate the hoisted check must not fire: the trie read runs
			// against the forced root and returns the stale value, byte-identical
			// to pre-fix behaviour.
			if got != val {
				t.Fatalf("gate inactive: read = %x, want the stale value %x (legacy path changed)", got, val)
			}
		}
	}
	t.Run("active", func(t *testing.T) { run(t, true) })
	t.Run("inactive", func(t *testing.T) { run(t, false) })
}

// TestUBF004_TriePathReadsEmptyUnforced documents the accident that kept this
// vintage safe: without any root forcing, the resurrected object's storage root is
// the empty root, so the trie-backed read is empty on both sides of the gate.
func TestUBF004_TriePathReadsEmptyUnforced(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x12})
	key := common.HexToHash("0x1")
	val := common.HexToHash("0x99")

	for _, gate := range []uint64{5, 100} {
		setUpstreamFixesGate(t, gate)
		_, db, root, _ := destructTestState(t, addr, key, val)
		st, err := New(root, db, nil, big.NewInt(10))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		st.Suicide(addr)
		st.Finalise(true)
		st.CreateAccount(addr)
		if got := st.GetCommittedState(addr, key); got != (common.Hash{}) {
			t.Fatalf("gate@%d: unforced re-created account read = %x, want empty", gate, got)
		}
	}
}

// TestUBF004_CopyCarriesDestructSet checks that Copy() preserves the destruct set
// when no snapshot exists. The pre-existing snapDestructs copy sits inside
// `if s.snaps != nil`; the new set must be copied unconditionally or the miner's
// copies silently lose every destruction on trie-backed runs.
func TestUBF004_CopyCarriesDestructSet(t *testing.T) {
	setUpstreamFixesGate(t, 5)
	addr := common.BytesToAddress([]byte{0x13})
	key := common.HexToHash("0x1")
	val := common.HexToHash("0x99")

	_, db, root, storageRoot := destructTestState(t, addr, key, val)
	st, err := New(root, db, nil, big.NewInt(10))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	st.Suicide(addr)
	st.Finalise(true)

	cp := st.Copy()
	if _, tracked := cp.stateObjectsDestruct[addr]; !tracked {
		t.Fatal("Copy() dropped the destruct set with snaps == nil")
	}

	// The copy must behave like the original: a resurrected account with a forced
	// stale root still reads empty through the copy.
	cp.CreateAccount(addr)
	obj := cp.getStateObject(addr)
	obj.data.Root = storageRoot
	if got := cp.GetCommittedState(addr, key); got != (common.Hash{}) {
		t.Fatalf("copy read stale storage %x after resurrect, want empty", got)
	}
}

// TestUBF004_SnapshotAndTrieAgree runs the delete-then-re-create scenario once
// with a snapshot layer and once trie-backed and asserts both produce the same
// committed state root, with the written value depending on a post-resurrect read
// (the way a real execution would couple reads into the root). This is the
// invariant the port enforces; per the revised analysis it also holds today, so
// this is a regression fence rather than a bug reproducer.
func TestUBF004_SnapshotAndTrieAgree(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x14})
	key := common.HexToHash("0x1")
	key2 := common.HexToHash("0x2")
	val := common.HexToHash("0x99")

	run := func(t *testing.T, withSnap bool) common.Hash {
		memdb, db, root, _ := destructTestState(t, addr, key, val)
		var snaps *snapshot.Tree
		if withSnap {
			snaps, _ = snapshot.New(memdb, db.TrieDB(), 1, root, false, true, false)
			if snaps == nil {
				t.Fatal("snapshot tree not built")
			}
		}
		st, err := New(root, db, snaps, big.NewInt(10))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if withSnap && st.snap == nil {
			t.Fatal("state is not snapshot-backed despite the tree")
		}
		st.Suicide(addr)
		st.Finalise(true)
		st.CreateAccount(addr)
		st.SetBalance(addr, big.NewInt(2)) // keep the resurrected account non-empty
		// Couple a post-resurrect read into the written state, like execution would.
		st.SetState(addr, key2, st.GetState(addr, key))
		newRoot, err := st.Commit(true)
		if err != nil {
			t.Fatalf("Commit: %v", err)
		}
		return newRoot
	}

	for _, gate := range []uint64{5, 100} {
		setUpstreamFixesGate(t, gate)
		snapRoot := run(t, true)
		trieRoot := run(t, false)
		if snapRoot != trieRoot {
			t.Fatalf("gate@%d: snapshot-backed root %x != trie-backed root %x", gate, snapRoot, trieRoot)
		}
	}
}

// snapCacheTestState builds a snapshot-backed state carrying populated
// snapAccounts/snapStorage entries for addr: it writes a slot and runs a
// mid-block IntermediateRoot, which is how updateTrie populates those caches in
// real use (per-tx intermediate roots, tracing APIs). Returns the state and the
// slot written before the re-creation.
func snapCacheTestState(t *testing.T, addr common.Address, blockNumber *big.Int) (*StateDB, common.Hash, common.Hash) {
	t.Helper()
	key := common.HexToHash("0x1")
	val := common.HexToHash("0x99")
	memdb, db, root, _ := destructTestState(t, addr, key, val)
	snaps, _ := snapshot.New(memdb, db.TrieDB(), 1, root, false, true, false)
	if snaps == nil {
		t.Fatal("snapshot tree not built")
	}
	st, err := New(root, db, snaps, blockNumber)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if st.snap == nil {
		t.Fatal("state is not snapshot-backed")
	}
	staleKey := common.HexToHash("0x2")
	staleVal := common.HexToHash("0x55")
	st.SetState(addr, staleKey, staleVal)
	st.IntermediateRoot(false) // mid-block: updateTrie fills snapAccounts/snapStorage
	addrHash := crypto.Keccak256Hash(addr.Bytes())
	if _, ok := st.snapAccounts[addrHash]; !ok {
		t.Fatal("setup: snapAccounts not populated by IntermediateRoot")
	}
	if _, ok := st.snapStorage[addrHash]; !ok {
		t.Fatal("setup: snapStorage not populated by IntermediateRoot")
	}
	return st, staleKey, staleVal
}

// TestUBF006_RecreateClearsCachedSnapshotData checks that re-creating an account
// drops its cached snapshot data (upstream 380fb4e24) so the stale pre-recreate
// slot cannot be pushed into the snapshot at commit, and that below the gate the
// legacy behaviour (stale slot survives into the snapshot) is preserved
// byte-for-byte. The gated half is the UBF-006 divergence made observable:
// without the fix the snapshot ends up claiming a slot the trie does not have.
func TestUBF006_RecreateClearsCachedSnapshotData(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x21})
	addrHash := crypto.Keccak256Hash(addr.Bytes())

	run := func(t *testing.T, active bool) {
		if active {
			setUpstreamFixesGate(t, 5)
		} else {
			setUpstreamFixesGate(t, 100)
		}
		st, staleKey, staleVal := snapCacheTestState(t, addr, big.NewInt(10))

		// Re-create the account while the caches hold its pre-recreate data.
		st.CreateAccount(addr)

		_, accCached := st.snapAccounts[addrHash]
		_, storCached := st.snapStorage[addrHash]
		if active && (accCached || storCached) {
			t.Fatal("gate active: cached snapshot data survived the re-creation")
		}
		if !active && (!accCached || !storCached) {
			t.Fatal("gate inactive: cached snapshot data should be untouched (legacy path changed)")
		}

		// Write a fresh slot on the re-created account and commit, then ask the
		// snapshot what it believes about the stale slot.
		newKey := common.HexToHash("0x3")
		newVal := common.HexToHash("0x77")
		st.SetBalance(addr, big.NewInt(2))
		st.SetState(addr, newKey, newVal)
		newRoot, err := st.Commit(true)
		if err != nil {
			t.Fatalf("Commit: %v", err)
		}
		snap := st.snaps.Snapshot(newRoot)
		if snap == nil {
			t.Fatal("no snapshot layer for the committed root")
		}
		staleBlob, err := snap.Storage(addrHash, crypto.Keccak256Hash(staleKey.Bytes()))
		if err != nil {
			t.Fatalf("snapshot read (stale slot): %v", err)
		}
		freshBlob, err := snap.Storage(addrHash, crypto.Keccak256Hash(newKey.Bytes()))
		if err != nil {
			t.Fatalf("snapshot read (fresh slot): %v", err)
		}
		if len(freshBlob) == 0 {
			t.Fatal("snapshot lost the slot written after the re-creation")
		}
		if active {
			if len(staleBlob) != 0 {
				t.Fatalf("gate active: stale slot %x=%x leaked into the snapshot", staleKey, staleVal)
			}
		} else {
			if len(staleBlob) == 0 {
				t.Fatal("gate inactive: expected the legacy stale-slot leak into the snapshot (legacy path changed)")
			}
		}
	}
	t.Run("active", func(t *testing.T) { run(t, true) })
	t.Run("inactive", func(t *testing.T) { run(t, false) })
}

// TestUBF006_RevertRestoresCachedSnapshotData checks the journal half of the fix:
// reverting a re-creation puts back the cached snapshot entries it cleared and
// undoes the destruct-set entry it added (upstream 380fb4e24's journal fields).
func TestUBF006_RevertRestoresCachedSnapshotData(t *testing.T) {
	setUpstreamFixesGate(t, 5)
	addr := common.BytesToAddress([]byte{0x22})
	addrHash := crypto.Keccak256Hash(addr.Bytes())

	st, staleKey, staleVal := snapCacheTestState(t, addr, big.NewInt(10))
	wantAccount := st.snapAccounts[addrHash]

	rev := st.Snapshot()
	st.CreateAccount(addr)
	if _, ok := st.snapAccounts[addrHash]; ok {
		t.Fatal("setup: re-creation did not clear snapAccounts")
	}
	if _, ok := st.stateObjectsDestruct[addr]; !ok {
		t.Fatal("setup: re-creation did not mark the destruct set")
	}
	st.RevertToSnapshot(rev)

	gotAccount, ok := st.snapAccounts[addrHash]
	if !ok || !bytesEqual(gotAccount, wantAccount) {
		t.Fatal("revert did not restore the cached account data")
	}
	gotStorage, ok := st.snapStorage[addrHash]
	if !ok {
		t.Fatal("revert did not restore the cached storage data")
	}
	if got := gotStorage[crypto.Keccak256Hash(staleKey.Bytes())]; len(got) == 0 {
		t.Fatalf("restored storage cache lost slot %x=%x", staleKey, staleVal)
	}
	if _, ok := st.stateObjectsDestruct[addr]; ok {
		t.Fatal("revert did not undo the destruct-set entry")
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestUBF006_ResetObjectJournalled checks that re-creating an existing account
// journals the account as dirtied when the gate is active (upstream 15bd21f3c),
// and — below the gate — still does not, preserving today's behaviour.
func TestUBF006_ResetObjectJournalled(t *testing.T) {
	addr := common.BytesToAddress([]byte{0x23})
	key := common.HexToHash("0x1")
	val := common.HexToHash("0x99")

	run := func(t *testing.T, active bool) {
		if active {
			setUpstreamFixesGate(t, 5)
		} else {
			setUpstreamFixesGate(t, 100)
		}
		_, db, root, _ := destructTestState(t, addr, key, val)
		st, err := New(root, db, nil, big.NewInt(10))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// The only operation is the re-creation itself, so any dirtiness must come
		// from resetObjectChange.dirtied().
		st.CreateAccount(addr)
		if active {
			if st.journal.dirties[addr] == 0 {
				t.Fatal("gate active: re-creation not journalled as dirtying the account")
			}
		} else {
			if st.journal.dirties[addr] != 0 {
				t.Fatal("gate inactive: re-creation journalled as dirty (legacy path changed)")
			}
		}
	}
	t.Run("active", func(t *testing.T) { run(t, true) })
	t.Run("inactive", func(t *testing.T) { run(t, false) })
}
