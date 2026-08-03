// Copyright 2026 The go-ethereum Authors
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

package fetcher

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
)

// TestUBF082_InvalidTxFloodThrottled verifies that a peer delivering a batch of
// transactions that are overwhelmingly rejected for "other" reasons is stalled,
// instead of being served at full speed.
// Upstream 7f2890a9b (#25573).
func TestUBF082_InvalidTxFloodThrottled(t *testing.T) {
	makeTxs := func(n int) []*types.Transaction {
		txs := make([]*types.Transaction, n)
		for i := 0; i < n; i++ {
			txs[i] = types.NewTx(&types.DefaultFeeTx{
				ChainID: big.NewInt(1),
				Nonce:   uint64(i),
				Gas:     21000,
				Value:   big.NewInt(0),
			})
		}
		return txs
	}
	newFetcher := func(fail error) *TxFetcher {
		f := NewTxFetcher(
			func(common.Hash) bool { return false },
			func(txs []*types.Transaction) []error {
				errs := make([]error, len(txs))
				for i := range errs {
					errs[i] = fail
				}
				return errs
			},
			func(string, []common.Hash) error { return nil },
		)
		f.Start()
		return f
	}
	// A full batch of transactions rejected for "other" reasons must throttle.
	bad := newFetcher(errors.New("invalid transaction"))
	defer bad.Stop()

	start := time.Now()
	if err := bad.Enqueue("flooder", makeTxs(txDeliveryBatchSize), false); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < txDeliveryThrottle {
		t.Fatalf("invalid transaction flood not throttled: enqueue took %v, want >= %v", elapsed, txDeliveryThrottle)
	}
	// A batch of perfectly valid transactions must not be throttled.
	good := newFetcher(nil)
	defer good.Stop()

	start = time.Now()
	if err := good.Enqueue("honest", makeTxs(txDeliveryBatchSize), false); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= txDeliveryThrottle {
		t.Fatalf("valid transactions throttled: enqueue took %v", elapsed)
	}
}

// TestUBF094_GenesisAnnouncementIgnored verifies that an announcement for block
// number 0 is discarded outright rather than scheduled for fetching.
// Upstream 410e731be (#22658).
func TestUBF094_GenesisAnnouncementIgnored(t *testing.T) {
	f := NewBlockFetcher(false,
		func(common.Hash) *types.Header { return nil },
		func(common.Hash) *types.Block { return nil },
		func(*types.Header) error { return nil },
		func(*types.Block, bool) {},
		func() uint64 { return 0 },
		func([]*types.Header) (int, error) { return 0, nil },
		func(types.Blocks) (int, error) { return 0, nil },
		func(string) {},
	)
	announced := make(chan common.Hash, 4)
	f.announceChangeHook = func(hash common.Hash, added bool) {
		if added {
			announced <- hash
		}
	}
	f.Start()
	defer f.Stop()

	genesis, regular := common.Hash{0x01}, common.Hash{0x02}
	header := func(common.Hash) error { return nil }
	body := func([]common.Hash) error { return nil }

	if err := f.Notify("peer", genesis, 0, time.Now(), header, body); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	// Push a legitimate announcement behind it; the loop is single threaded, so
	// seeing this one means the genesis announcement has already been processed.
	if err := f.Notify("peer", regular, 1, time.Now(), header, body); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	select {
	case hash := <-announced:
		if hash == genesis {
			t.Fatalf("genesis announcement was scheduled for fetching")
		}
		if hash != regular {
			t.Fatalf("unexpected announcement %x", hash)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for the announcement to be scheduled")
	}
}
