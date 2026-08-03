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

package leveldb

import (
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb/dbtest"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

func TestLevelDB(t *testing.T) {
	t.Run("DatabaseSuite", func(t *testing.T) {
		dbtest.TestDatabaseSuite(t, func() ethdb.KeyValueStore {
			db, err := leveldb.Open(storage.NewMemStorage(), nil)
			if err != nil {
				t.Fatal(err)
			}
			return &Database{
				db: db,
			}
		})
	})
}

// TestUBF126_BatchSizeCountsKey checks that a batched Put accounts for the key as well
// as the value. Upstream 53f81574e: Delete counted the key but Put did not, so batches
// were flushed later than the configured ideal size — badly so with 32-byte keys.
func TestUBF126_BatchSizeCountsKey(t *testing.T) {
	db, err := leveldb.Open(storage.NewMemStorage(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var (
		key   = []byte("a-32-byte-quantum-coin-key-here!")
		value = []byte("value")
	)
	b := (&Database{db: db}).NewBatch()
	if err := b.Put(key, value); err != nil {
		t.Fatal(err)
	}
	if want := len(key) + len(value); b.ValueSize() != want {
		t.Errorf("ValueSize after Put = %d, want %d", b.ValueSize(), want)
	}
	if err := b.Delete(key); err != nil {
		t.Fatal(err)
	}
	if want := 2*len(key) + len(value); b.ValueSize() != want {
		t.Errorf("ValueSize after Delete = %d, want %d", b.ValueSize(), want)
	}
}
