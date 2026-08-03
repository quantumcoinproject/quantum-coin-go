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
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>

package snapshot

import (
	"bytes"
	"errors"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb/memorydb"
)

// generatorContext carries the global storage snapshot iterator which is walked
// in lockstep with the account iteration in order to detect and clean up
// "dangling" storage - storage snapshot entries whose owning account no longer
// exists in the state.
//
// Upstream 59ac229f8 introduced a richer generatorContext which also owns the
// account iterator, the write batch, the statistics and the log timestamp, and
// pushed those through a refactored (function-per-phase) generator. This
// generator is still the monolithic, closure-based form, so it keeps those
// locally and this context only carries the piece the dangling-storage cleanup
// actually needs. The write batch is shared with the generator so that the
// deletions land in the same atomic flush as the progress marker.
type generatorContext struct {
	stats   *generatorStats     // Generation statistic collection
	db      ethdb.KeyValueStore // Key-value store containing the snapshot data
	batch   ethdb.Batch         // Database batch owned by the generator
	storage *holdableIterator   // Iterator of storage snapshot data
}

// newGeneratorContext initializes the context for the dangling-storage cleanup.
// The storage iterator is opened at the interrupted position, because all the
// snapshot data before the marker is assumed to have been generated correctly.
func newGeneratorContext(stats *generatorStats, db ethdb.KeyValueStore, batch ethdb.Batch, storageMarker []byte) *generatorContext {
	ctx := &generatorContext{
		stats: stats,
		db:    db,
		batch: batch,
	}
	ctx.openStorageIterator(storageMarker)
	return ctx
}

// openStorageIterator constructs the global storage snapshot iterator at the
// given position.
func (ctx *generatorContext) openStorageIterator(start []byte) {
	iter := ctx.db.NewIterator(rawdb.SnapshotStoragePrefix, start)
	ctx.storage = newHoldableIterator(newKeyLengthIterator(iter, len(rawdb.SnapshotStoragePrefix)+2*common.HashLength))
}

// reopenStorageIterator releases the storage iterator and re-opens it at the
// next position. Long-lived iterators block leveldb compaction, so the
// generator drops and re-creates it every time it flushes.
func (ctx *generatorContext) reopenStorageIterator() {
	// Shift the iterator one more step, so that we can reopen it at the right
	// position.
	if !ctx.storage.Next() {
		// Iterator exhausted, release forever and create an already exhausted
		// virtual iterator.
		ctx.storage.Release()
		ctx.storage = newHoldableIterator(memorydb.New().NewIterator(nil, nil))
		return
	}
	next := common.CopyBytes(ctx.storage.Key())
	ctx.storage.Release()
	ctx.openStorageIterator(next[len(rawdb.SnapshotStoragePrefix):])
}

// close releases all the held resources.
func (ctx *generatorContext) close() {
	ctx.storage.Release()
}

// flushIfLarge writes out the batch if it grew beyond the ideal size. The
// deletions are idempotent, so an intermediate flush without a progress marker
// is safe.
func (ctx *generatorContext) flushIfLarge() {
	if ctx.batch.ValueSize() > ethdb.IdealBatchSize {
		ctx.batch.Write()
		ctx.batch.Reset()
	}
}

// removeStorageBefore deletes all storage entries which are located before
// the specified account. When the iterator touches the storage entry which
// is located in or outside the given account, it stops and holds the current
// iterated element locally.
func (ctx *generatorContext) removeStorageBefore(account common.Hash) {
	var (
		count uint64
		start = time.Now()
		iter  = ctx.storage
	)
	for iter.Next() {
		key := iter.Key()
		if bytes.Compare(key[len(rawdb.SnapshotStoragePrefix):len(rawdb.SnapshotStoragePrefix)+common.HashLength], account.Bytes()) >= 0 {
			iter.Hold()
			break
		}
		count++
		ctx.batch.Delete(key)
		ctx.flushIfLarge()
	}
	ctx.stats.dangling += count
	snapDanglingStorageMeter.Mark(int64(count))
	snapStorageCleanCounter.Inc(time.Since(start).Nanoseconds())
}

// removeStorageAt deletes all storage entries which are located in the specified
// account. When the iterator touches the storage entry which is outside the given
// account, it stops and holds the current iterated element locally. An error will
// be returned if the initial position of iterator is not in the given account.
func (ctx *generatorContext) removeStorageAt(account common.Hash) error {
	var (
		count int64
		start = time.Now()
		iter  = ctx.storage
	)
	for iter.Next() {
		key := iter.Key()
		cmp := bytes.Compare(key[len(rawdb.SnapshotStoragePrefix):len(rawdb.SnapshotStoragePrefix)+common.HashLength], account.Bytes())
		if cmp < 0 {
			return errors.New("invalid iterator position")
		}
		if cmp > 0 {
			iter.Hold()
			break
		}
		count++
		ctx.batch.Delete(key)
		ctx.flushIfLarge()
	}
	snapWipedStorageMeter.Mark(count)
	snapStorageCleanCounter.Inc(time.Since(start).Nanoseconds())
	return nil
}

// skipStorageAt advances the iterator past all the storage entries which belong
// to the specified account, without deleting anything. It is the counterpart of
// removeStorageAt for accounts which do have a storage trie: their slots are
// (re)generated by the regular range generation, so the dangling-storage sweep
// must only step over them.
//
// Upstream 59ac229f8 instead routes the whole range generation through the same
// global iterator, which this pre-refactor generator does not do; stepping over
// the account keeps the sweep in sync at the cost of iterating the account's
// storage keys a second time.
func (ctx *generatorContext) skipStorageAt(account common.Hash) error {
	iter := ctx.storage
	for iter.Next() {
		key := iter.Key()
		cmp := bytes.Compare(key[len(rawdb.SnapshotStoragePrefix):len(rawdb.SnapshotStoragePrefix)+common.HashLength], account.Bytes())
		if cmp < 0 {
			return errors.New("invalid iterator position")
		}
		if cmp > 0 {
			iter.Hold()
			break
		}
	}
	return nil
}

// removeStorageLeft deletes all storage entries which are located after
// the current iterator position.
func (ctx *generatorContext) removeStorageLeft() {
	var (
		count uint64
		start = time.Now()
		iter  = ctx.storage
	)
	for iter.Next() {
		count++
		ctx.batch.Delete(iter.Key())
		ctx.flushIfLarge()
	}
	ctx.stats.dangling += count
	snapDanglingStorageMeter.Mark(int64(count))
	snapStorageCleanCounter.Inc(time.Since(start).Nanoseconds())
}
