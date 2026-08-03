// Copyright 2022 The go-ethereum Authors
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

package state

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// TestUBF024_PrefetcherCopyNilFetch makes sure copying an inactive prefetcher
// whose fetches map carries a nil trie (subfetcher.peek() can legitimately
// return nil) does not dereference the nil trie.
//
// Upstream d46184c96 (#25575).
func TestUBF024_PrefetcherCopyNilFetch(t *testing.T) {
	db := NewDatabase(rawdb.NewMemoryDatabase())
	root := common.HexToHash("0x01")

	p := newTriePrefetcher(db, root, "test")
	// Turn it into an inactive (already copied) prefetcher holding a nil trie,
	// which is exactly what copying an active fetcher that never resolved its
	// trie produces.
	inactive := p.copy()
	inactive.fetches[common.HexToHash("0x02")] = nil

	// Without the nil guard this panics inside db.CopyTrie.
	cpy := inactive.copy()
	if _, ok := cpy.fetches[common.HexToHash("0x02")]; ok {
		t.Fatal("Nil trie should have been filtered out of the copy")
	}
}

// TestUBF028_DumpIncludesZeroAddress checks that the account living at the zero
// address is emitted by the iterative dump. The collector used to take the
// address by value, so the dump could not tell "no preimage" apart from "the
// address is all zeroes" and dropped the latter.
//
// Note the zero address here is 32 bytes of zeroes, not 20.
//
// Upstream bfded65ed (#27320).
func TestUBF028_DumpIncludesZeroAddress(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	sdb, _ := New(common.Hash{}, NewDatabaseWithConfig(db, &trie.Config{Preimages: true}), nil, nil)

	zero := common.Address{} // [32]byte{}
	obj := sdb.GetOrNewStateObject(zero)
	obj.AddBalance(big.NewInt(1337))
	other := sdb.GetOrNewStateObject(common.BytesToAddress([]byte{0x01}))
	other.AddBalance(big.NewInt(22))

	sdb.updateStateObject(obj)
	sdb.updateStateObject(other)
	if _, err := sdb.Commit(false); err != nil {
		t.Fatalf("Failed to commit state: %v", err)
	}
	// The aggregated dump must contain the zero-address account.
	dump := sdb.RawDump(nil)
	if _, ok := dump.Accounts[zero]; !ok {
		t.Errorf("RawDump is missing the zero-address account: %v", dump.Accounts)
	}
	// So must the iterative, line-by-line dump, and with its address attached.
	buf := new(bytes.Buffer)
	sdb.IterativeDump(nil, json.NewEncoder(buf))

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var account DumpAccount
		if err := json.Unmarshal([]byte(line), &account); err != nil {
			continue // the first line is the root
		}
		if account.Address != nil && *account.Address == zero {
			found = true
			if account.Balance != "1337" {
				t.Errorf("Zero-address account balance: have %s, want 1337", account.Balance)
			}
		}
	}
	if !found {
		t.Errorf("IterativeDump is missing the zero-address account:\n%s", buf.String())
	}
}
