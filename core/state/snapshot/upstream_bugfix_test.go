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

package snapshot

import (
	"bytes"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// setAccount is a helper to construct a random account entry and assign it to
// an account slot in a snapshot.
func setAccount(accKey string) map[common.Hash][]byte {
	return map[common.Hash][]byte{
		common.HexToHash(accKey): randomAccount(),
	}
}

// TestUBF019_ReadStateDuringFlattening tests the scenario that, while the bottom
// diff layers are being merged (which tags them as stale), a read happens via a
// pre-created top snapshot layer which tries to access the state in these stale
// layers. The read must retrieve the right state back (blocking until the
// flattening is finished) instead of an unexpected staleness error.
//
// Upstream f915f6873 (#23628).
func TestUBF019_ReadStateDuringFlattening(t *testing.T) {
	// Create a starting base layer and a snapshot tree out of it
	base := &diskLayer{
		diskdb: rawdb.NewMemoryDatabase(),
		root:   common.HexToHash("0x01"),
		cache:  fastcache.New(1024 * 500),
	}
	snaps := &Tree{
		layers: map[common.Hash]snapshot{
			base.root: base,
		},
	}
	// 4 layers in total, 3 diff layers and 1 disk layer
	snaps.Update(common.HexToHash("0xa1"), common.HexToHash("0x01"), nil, setAccount("0xa1"), nil)
	snaps.Update(common.HexToHash("0xa2"), common.HexToHash("0xa1"), nil, setAccount("0xa2"), nil)
	snaps.Update(common.HexToHash("0xa3"), common.HexToHash("0xa2"), nil, setAccount("0xa3"), nil)

	// Obtain the topmost snapshot handler for state accessing
	snap := snaps.Snapshot(common.HexToHash("0xa3"))

	// Register the testing hook to access the state after flattening
	var result = make(chan *Account)
	snaps.onFlatten = func() {
		// Spin up a thread to read the account from the pre-created
		// snapshot handler. It's expected to be blocked.
		go func() {
			account, _ := snap.Account(common.HexToHash("0xa1"))
			result <- account
		}()
		select {
		case res := <-result:
			t.Errorf("Unexpected return %v", res)
		case <-time.NewTimer(time.Millisecond * 300).C:
		}
	}
	// Cap the snap tree, which will mark the bottom-most layer as stale.
	snaps.Cap(common.HexToHash("0xa3"), 1)
	select {
	case account := <-result:
		if account == nil {
			t.Fatal("Failed to retrieve account")
		}
	case <-time.NewTimer(time.Millisecond * 300).C:
		t.Fatal("Unexpected blocker")
	}
}

// TestUBF021_ParentAccessorLocked hammers diffLayer.Parent() from several
// goroutines while the tree is repeatedly capped (which re-links dl.parent).
// Without the lock in Parent() the race detector flags the unsynchronised read.
//
// Upstream 86d547707 (#24685).
func TestUBF021_ParentAccessorLocked(t *testing.T) {
	base := &diskLayer{
		diskdb: rawdb.NewMemoryDatabase(),
		root:   common.HexToHash("0x01"),
		cache:  fastcache.New(1024 * 500),
	}
	snaps := &Tree{
		layers: map[common.Hash]snapshot{
			base.root: base,
		},
	}
	snaps.Update(common.HexToHash("0xa1"), common.HexToHash("0x01"), nil, setAccount("0xa1"), nil)
	snaps.Update(common.HexToHash("0xa2"), common.HexToHash("0xa1"), nil, setAccount("0xa2"), nil)
	snaps.Update(common.HexToHash("0xa3"), common.HexToHash("0xa2"), nil, setAccount("0xa3"), nil)

	top := snaps.Snapshot(common.HexToHash("0xa3")).(*diffLayer)

	var (
		wg   sync.WaitGroup
		done = make(chan struct{})
	)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				// The flattening below re-links top.parent concurrently.
				_ = top.Parent()
			}
		}()
	}
	// Cap the tree, which flattens the layers below the top one and re-links
	// top.parent while the readers above are running.
	if err := snaps.Cap(common.HexToHash("0xa3"), 1); err != nil {
		t.Fatalf("Failed to cap snapshot tree: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()
}

// TestUBF005_StaleDiffLayerReturnsError verifies that a diff layer which was
// flattened across reports ErrSnapshotStale from the public AccountRLP/Storage
// accessors instead of silently falling through the bloom filter into the
// (already replaced) origin disk layer and returning junk.
//
// Upstream eb83e7c54 (#27255).
func TestUBF005_StaleDiffLayerReturnsError(t *testing.T) {
	base := &diskLayer{
		diskdb: rawdb.NewMemoryDatabase(),
		root:   common.HexToHash("0x01"),
		cache:  fastcache.New(1024 * 500),
	}
	snaps := &Tree{
		layers: map[common.Hash]snapshot{
			base.root: base,
		},
	}
	accounts := map[common.Hash][]byte{
		common.HexToHash("0xa1"): randomAccount(),
	}
	storage := map[common.Hash]map[common.Hash][]byte{
		common.HexToHash("0xa1"): {common.HexToHash("0x01"): randomHash().Bytes()},
	}
	snaps.Update(common.HexToHash("0xb1"), common.HexToHash("0x01"), nil, accounts, storage)
	snaps.Update(common.HexToHash("0xb2"), common.HexToHash("0xb1"), nil, setAccount("0xa2"), nil)
	snaps.Update(common.HexToHash("0xb3"), common.HexToHash("0xb2"), nil, setAccount("0xa3"), nil)

	// Grab a handle on the bottom-most diff layer before it gets flattened away.
	stale := snaps.Snapshot(common.HexToHash("0xb1")).(*diffLayer)

	// Flatten the bottom two layers, marking the handle above as stale.
	if err := snaps.Cap(common.HexToHash("0xb3"), 1); err != nil {
		t.Fatalf("Failed to cap snapshot tree: %v", err)
	}
	if !stale.Stale() {
		t.Fatal("Expected the bottom-most diff layer to be stale")
	}
	// Probe an account/slot that this layer does NOT know about, so that the
	// bloom filter misses and the unfixed code path dives into dl.origin. The
	// bloom hashers only look at 8 bytes of the hash, so small hand-written
	// hashes all collide - hunt for a genuinely missing probe.
	var probeAccount, probeSlot common.Hash
	for i := 0; ; i++ {
		probeAccount, probeSlot = randomHash(), randomHash()
		if !stale.diffed.Contains(accountBloomHasher(probeAccount)) &&
			!stale.diffed.Contains(destructBloomHasher(probeAccount)) &&
			!stale.diffed.Contains(storageBloomHasher{probeAccount, probeSlot}) {
			break
		}
		if i > 1000 {
			t.Fatal("Failed to find a probe missing from the bloom filter")
		}
	}
	if _, err := stale.AccountRLP(probeAccount); err != ErrSnapshotStale {
		t.Errorf("AccountRLP on stale layer: have error %v, want %v", err, ErrSnapshotStale)
	}
	if _, err := stale.Storage(probeAccount, probeSlot); err != ErrSnapshotStale {
		t.Errorf("Storage on stale layer: have error %v, want %v", err, ErrSnapshotStale)
	}
}

// TestUBF026_JournalDanglingStorageDetected checks that a storage entry in a
// journalled diff layer whose owning account is absent from that same layer is
// reported, and that a well-formed journal passes.
//
// Upstream 0914234d1 (#24677), folded into dangling.go by 59ac229f8 (#24811).
func TestUBF026_JournalDanglingStorageDetected(t *testing.T) {
	encode := func(storageOwner common.Hash, withAccount bool) []byte {
		buffer := new(bytes.Buffer)
		rlp.Encode(buffer, journalVersion)
		rlp.Encode(buffer, common.HexToHash("0x01")) // disk layer root
		rlp.Encode(buffer, common.HexToHash("0xa0")) // diff layer root
		rlp.Encode(buffer, []journalDestruct{})
		accounts := []journalAccount{}
		if withAccount {
			accounts = append(accounts, journalAccount{Hash: storageOwner, Blob: randomAccount()})
		}
		rlp.Encode(buffer, accounts)
		rlp.Encode(buffer, []journalStorage{{
			Hash: storageOwner,
			Keys: []common.Hash{common.HexToHash("0x01")},
			Vals: [][]byte{{0x02}},
		}})
		return buffer.Bytes()
	}
	owner := common.HexToHash("0xaa")

	db := rawdb.NewMemoryDatabase()
	rawdb.WriteSnapshotJournal(db, encode(owner, true))
	if err := checkDanglingMemStorage(db); err != nil {
		t.Fatalf("Unexpected error on a consistent journal: %v", err)
	}
	db = rawdb.NewMemoryDatabase()
	rawdb.WriteSnapshotJournal(db, encode(owner, false))
	if err := checkDanglingMemStorage(db); err == nil {
		t.Fatal("Expected a dangling journal storage error, got none")
	}
}

// incKey / decKey shift a key by one, used to plant dangling storage right
// around the accounts that do exist.
func incKey(key []byte) []byte {
	for i := len(key) - 1; i >= 0; i-- {
		key[i]++
		if key[i] != 0x0 {
			break
		}
	}
	return key
}

func decKey(key []byte) []byte {
	for i := len(key) - 1; i >= 0; i-- {
		key[i]--
		if key[i] != 0xff {
			break
		}
	}
	return key
}

// populateDangling writes storage snapshot entries for a number of accounts
// which do not exist in the state, both around the accounts that do exist and
// at the very edges of the key space.
func populateDangling(disk ethdb.KeyValueStore) {
	populate := func(accountHash common.Hash, keys []string, vals []string) {
		for i, key := range keys {
			rawdb.WriteStorageSnapshot(disk, accountHash, hashData([]byte(key)), []byte(vals[i]))
		}
	}
	keys, vals := []string{"key-1", "key-2", "key-3"}, []string{"val-1", "val-2", "val-3"}

	// Dangling storages of the "first" account
	populate(common.Hash{}, keys, vals)

	// Dangling storages of the "last" account
	populate(common.HexToHash("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), keys, vals)

	// Dangling storages around the accounts that do exist
	for _, acc := range []string{"acc-1", "acc-2", "acc-3"} {
		populate(common.BytesToHash(decKey(hashData([]byte(acc)).Bytes())), keys, vals)
		populate(common.BytesToHash(incKey(hashData([]byte(acc)).Bytes())), keys, vals)
	}
	// Dangling storages of random accounts
	for i := 0; i < 3; i++ {
		populate(randomHash(), keys, vals)
	}
}

// TestUBF022_DanglingStorageCleaned plants storage snapshot entries whose owning
// account does not exist and verifies that a full snapshot generation sweeps
// them away instead of leaving them behind forever.
//
// Upstream 59ac229f8 (#24811).
func TestUBF022_DanglingStorageCleaned(t *testing.T) {
	helper := newHelper()
	stRoot := helper.makeStorageTrie([]string{"key-1", "key-2", "key-3"}, []string{"val-1", "val-2", "val-3"})

	helper.addAccount("acc-1", &Account{Balance: big.NewInt(1), Root: stRoot, CodeHash: emptyCode.Bytes()})
	helper.addAccount("acc-2", &Account{Balance: big.NewInt(1), Root: emptyRoot.Bytes(), CodeHash: emptyCode.Bytes()})
	helper.addAccount("acc-3", &Account{Balance: big.NewInt(1), Root: stRoot, CodeHash: emptyCode.Bytes()})

	helper.addSnapStorage("acc-1", []string{"key-1", "key-2", "key-3"}, []string{"val-1", "val-2", "val-3"})
	helper.addSnapStorage("acc-3", []string{"key-1", "key-2", "key-3"}, []string{"val-1", "val-2", "val-3"})

	populateDangling(helper.diskdb)

	// Sanity check that the planted entries are actually seen as dangling.
	if err := CheckDanglingStorage(helper.diskdb); err == nil {
		t.Fatal("Expected the planted storage to be reported as dangling")
	}
	root, snap := helper.Generate()
	select {
	case <-snap.genPending:
		// Snapshot generation succeeded

	case <-time.After(10 * time.Second):
		t.Fatal("Snapshot generation failed")
	}
	checkSnapRoot(t, snap, root)

	if err := CheckDanglingStorage(helper.diskdb); err != nil {
		t.Errorf("Dangling storage survived snapshot generation: %v", err)
	}
	// Signal abortion to the generator and wait for it to tear down
	stop := make(chan *generatorStats)
	snap.genAbort <- stop
	<-stop
}

// TestUBF020_GenMarkerDoesNotGoBackwards resumes an interrupted generation from
// a marker that points *into* an account's storage (account||slot) and aborts it
// again straight away. The re-journalled marker must not regress to the bare
// account hash, otherwise the next resume re-generates storage that the disk
// layer already claims to have, which surfaces as a BAD BLOCK.
//
// Upstream 312e02bca (#23635).
func TestUBF020_GenMarkerDoesNotGoBackwards(t *testing.T) {
	helper := newHelper()
	stRoot := helper.makeStorageTrie([]string{"key-1", "key-2", "key-3"}, []string{"val-1", "val-2", "val-3"})

	helper.addAccount("acc-1", &Account{Balance: big.NewInt(1), Root: stRoot, CodeHash: emptyCode.Bytes()})
	helper.addAccount("acc-2", &Account{Balance: big.NewInt(2), Root: stRoot, CodeHash: emptyCode.Bytes()})
	helper.addAccount("acc-3", &Account{Balance: big.NewInt(3), Root: stRoot, CodeHash: emptyCode.Bytes()})
	helper.addSnapStorage("acc-1", []string{"key-1", "key-2", "key-3"}, []string{"val-1", "val-2", "val-3"})

	root, _ := helper.accTrie.Commit(nil)
	helper.triedb.Commit(root, false, nil)

	// Interrupted halfway through the storage of the first account.
	accountHash := hashData([]byte("acc-1"))
	marker := append(accountHash.Bytes(), hashData([]byte("key-2")).Bytes()...)

	stats := &generatorStats{start: time.Now()}
	batch := helper.diskdb.NewBatch()
	rawdb.WriteSnapshotRoot(batch, root)
	journalProgress(batch, marker, stats)
	if err := batch.Write(); err != nil {
		t.Fatalf("Failed to write the interrupted marker: %v", err)
	}
	dl := &diskLayer{
		diskdb:     helper.diskdb,
		triedb:     helper.triedb,
		root:       root,
		cache:      fastcache.New(1024 * 500),
		genMarker:  marker,
		genPending: make(chan struct{}),
		genAbort:   make(chan chan *generatorStats),
	}
	// Park an abort on the channel *before* the generator starts, so that the
	// very first checkAndFlush - the one for the resumed account - picks it up
	// and journals the marker it was given.
	stop := make(chan *generatorStats)
	go func() { dl.genAbort <- stop }()
	time.Sleep(100 * time.Millisecond)

	go dl.generate(stats)
	select {
	case <-stop:
	case <-time.After(10 * time.Second):
		t.Fatal("Generator failed to abort")
	}
	// Read back the journalled progress and make sure it did not regress.
	blob := rawdb.ReadSnapshotGenerator(helper.diskdb)
	if len(blob) == 0 {
		t.Fatal("Missing snapshot generator entry")
	}
	var generator journalGenerator
	if err := rlp.DecodeBytes(blob, &generator); err != nil {
		t.Fatalf("Failed to decode snapshot generator: %v", err)
	}
	if generator.Done {
		t.Skip("Generation completed before the abort was observed, marker is nil")
	}
	if bytes.Compare(generator.Marker, marker) < 0 {
		t.Errorf("Generator marker went backwards: have %#x, want >= %#x", generator.Marker, marker)
	}
}
