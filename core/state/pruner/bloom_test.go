// Copyright 2020 The go-ethereum Authors
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

package pruner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
)

// TestUBF023_BloomCommitSyncs checks that committing the state bloom actually
// succeeds - the fsync used to be issued on a read-only handle, which fails on
// Windows (the platform we ship on) and aborted the whole pruning run.
//
// Upstream 3b38a8327.
func TestUBF023_BloomCommitSyncs(t *testing.T) {
	bloom, err := newStateBloomWithSize(1)
	if err != nil {
		t.Fatalf("Failed to create the state bloom: %v", err)
	}
	key := common.HexToHash("0xdeadbeef")
	if err := bloom.Put(key.Bytes(), nil); err != nil {
		t.Fatalf("Failed to fill the state bloom: %v", err)
	}
	var (
		dir      = t.TempDir()
		filename = filepath.Join(dir, "statebloom.bf.gz")
		tempname = filename + stateBloomFileTempSuffix
	)
	if err := bloom.Commit(filename, tempname); err != nil {
		t.Fatalf("Failed to commit the state bloom: %v", err)
	}
	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("State bloom was not persisted: %v", err)
	}
	if _, err := os.Stat(tempname); err == nil {
		t.Fatal("Temporary state bloom file was left behind")
	}
	// The persisted filter must be loadable and still contain the entry.
	reloaded, err := NewStateBloomFromDisk(filename)
	if err != nil {
		t.Fatalf("Failed to reload the state bloom: %v", err)
	}
	if ok, _ := reloaded.Contain(key.Bytes()); !ok {
		t.Fatal("Reloaded state bloom is missing the committed entry")
	}
}
