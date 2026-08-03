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

package core

import (
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/event"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// ubfAccount is a funded test sender.
type ubfAccount struct {
	key  *signaturealgorithm.PrivateKey
	addr common.Address
}

// newUBFPool builds a TxPool backed by an in-memory state. It reuses the
// txPoolVBlockChain harness from tx_pool_v_test.go.
func newUBFPool(t *testing.T) *TxPool {
	t.Helper()

	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	bc := &txPoolVBlockChain{
		statedb:       statedb,
		gasLimit:      300000000,
		chainHeadFeed: new(event.Feed),
	}
	config := DefaultTxPoolConfig
	config.Journal = "" // avoid touching disk

	return NewTxPool(config, txPoolVTestChainConfig, bc)
}

// newUBFAccount creates a key and credits it with the given balance in the
// pool's current state.
func newUBFAccount(t *testing.T, pool *TxPool, balance *big.Int) *ubfAccount {
	t.Helper()

	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	pool.currentState.AddBalance(addr, balance)
	return &ubfAccount{key: key, addr: addr}
}

// ubfTx signs a DefaultFeeTx from the given account.
func ubfTx(t *testing.T, pool *TxPool, acc *ubfAccount, nonce uint64, value *big.Int) *types.Transaction {
	t.Helper()

	to := common.BytesToAddress([]byte{0x11})
	tx := types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      nonce,
		To:         &to,
		Value:      value,
		Gas:        params.TxGas,
		MaxGasTier: types.GAS_TIER_DEFAULT,
	})
	signed, err := types.SignTx(tx, pool.signer, acc.key)
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}
	return signed
}

// ubfBaseCost is the cost of a zero-value ubfTx: gas * base fee.
func ubfBaseCost() *big.Int {
	return new(big.Int).Mul(types.GetDefaultGasPrice(), new(big.Int).SetUint64(params.TxGas))
}

// fillUBFPool makes the pool exactly full with `count` pending transactions from
// a single account, by shrinking the global limits to the resulting slot count.
// It returns the account whose transactions now occupy the whole pool.
func fillUBFPool(t *testing.T, pool *TxPool, count int) *ubfAccount {
	t.Helper()

	// Fund generously: the overdraft rule requires the sender to be able to pay
	// for every pending transaction simultaneously.
	balance := new(big.Int).Mul(ubfBaseCost(), big.NewInt(int64(count)*10))
	acc := newUBFAccount(t, pool, balance)

	txs := make([]*types.Transaction, 0, count)
	for i := 0; i < count; i++ {
		txs = append(txs, ubfTx(t, pool, acc, uint64(i), big.NewInt(0)))
	}
	for i, err := range pool.AddRemotesSync(txs) {
		if err != nil {
			t.Fatalf("failed to add filler tx %d: %v", i, err)
		}
	}
	if pending, queued := pool.Stats(); pending != count || queued != 0 {
		t.Fatalf("expected %d pending / 0 queued, got %d / %d", count, pending, queued)
	}

	// Shrink the pool limits so that the transactions above fill it exactly.
	pool.mu.Lock()
	slots := uint64(pool.all.Slots())
	pool.config.GlobalSlots = slots / 2
	pool.config.GlobalQueue = slots - slots/2
	pool.mu.Unlock()

	return acc
}

// TestUBF044_FutureTxCannotEvictPending checks that a gapped (future) remote
// transaction arriving at a full pool is rejected instead of churning somebody
// else's executable transactions.
// Upstream 6cf2e921a (#26648), 230df98e4 (#26907), b8ee2877c (#27404).
func TestUBF044_FutureTxCannotEvictPending(t *testing.T) {
	pool := newUBFPool(t)
	defer pool.Stop()

	victim := fillUBFPool(t, pool, 8)
	pendingBefore, _ := pool.Stats()

	attacker := newUBFAccount(t, pool, new(big.Int).Mul(ubfBaseCost(), big.NewInt(100)))
	future := ubfTx(t, pool, attacker, 1000, big.NewInt(0))

	if err := pool.addRemoteSync(future); err != ErrFutureReplacePending {
		t.Fatalf("expected ErrFutureReplacePending, got: %v", err)
	}
	if pool.Get(future.Hash()) != nil {
		t.Fatal("the rejected future transaction must not be in the pool")
	}
	pendingAfter, _ := pool.Stats()
	if pendingAfter != pendingBefore {
		t.Fatalf("future transaction evicted pending ones: have %d, want %d", pendingAfter, pendingBefore)
	}
	pending, _ := pool.ContentFrom(victim.addr)
	if len(pending) != pendingBefore {
		t.Fatalf("victim lost pending transactions: have %d, want %d", len(pending), pendingBefore)
	}
}

// TestUBF044_LocalFutureTxAllowed checks the local-transaction exemption: our
// own future transactions may still make room in a full pool.
// Upstream e6b6a8b73 (#26930).
func TestUBF044_LocalFutureTxAllowed(t *testing.T) {
	pool := newUBFPool(t)
	defer pool.Stop()

	fillUBFPool(t, pool, 8)

	local := newUBFAccount(t, pool, new(big.Int).Mul(ubfBaseCost(), big.NewInt(100)))
	future := ubfTx(t, pool, local, 1000, big.NewInt(0))

	if err := pool.AddLocal(future); err != nil {
		t.Fatalf("expected a local future transaction to be accepted, got: %v", err)
	}
	if pool.Get(future.Hash()) == nil {
		t.Fatal("the accepted local future transaction must be in the pool")
	}
}

// TestUBF044_OverdraftRejected checks that a sender cannot queue up more
// pending transactions than its balance can simultaneously fund, even though
// each individual transaction is affordable on its own.
// Upstream 6cf2e921a (#26648).
func TestUBF044_OverdraftRejected(t *testing.T) {
	pool := newUBFPool(t)
	defer pool.Stop()

	cost := ubfBaseCost()
	// Enough for two transactions, not for three.
	balance := new(big.Int).Mul(cost, big.NewInt(2))
	balance.Add(balance, new(big.Int).Div(cost, big.NewInt(2)))
	acc := newUBFAccount(t, pool, balance)

	for i := 0; i < 2; i++ {
		if err := pool.addRemoteSync(ubfTx(t, pool, acc, uint64(i), big.NewInt(0))); err != nil {
			t.Fatalf("expected affordable tx %d to be accepted, got: %v", i, err)
		}
	}
	if pending, _ := pool.ContentFrom(acc.addr); len(pending) != 2 {
		t.Fatalf("expected 2 pending transactions, got %d", len(pending))
	}

	// A third transaction is individually affordable (cost < balance) but the
	// sender cannot fund all three at once.
	third := ubfTx(t, pool, acc, 2, big.NewInt(0))
	if err := pool.addRemoteSync(third); err != ErrOverdraft {
		t.Fatalf("expected ErrOverdraft, got: %v", err)
	}
	if pool.Get(third.Hash()) != nil {
		t.Fatal("the overdrafting transaction must not be in the pool")
	}

	// Replacing an existing nonce only has to cover the delta, so it is fine.
	replacement := ubfTx(t, pool, acc, 1, big.NewInt(0))
	if err := pool.addRemoteSync(replacement); err != nil {
		t.Fatalf("expected a same-cost replacement to be accepted, got: %v", err)
	}
}

// TestUBF045_PricedListStalesRace exercises the stale counter and Reheap under
// concurrency. Run with -race.
// Upstream 067084fed (#23474) and 51ed39c09 (#23542).
func TestUBF045_PricedListStalesRace(t *testing.T) {
	const (
		goroutines = 8
		iterations = 1000
	)

	// The stale counter must not lose updates. The urgent heap is padded so the
	// 25% reheap threshold is never reached and Removed only touches `stales`.
	priced := newTxPricedList(newTxLookup())
	priced.urgent.list = make([]*types.Transaction, 100*goroutines*iterations)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				priced.Removed(1)
			}
		}()
	}
	wg.Wait()

	if have, want := atomic.LoadInt64(&priced.stales), int64(goroutines*iterations); have != want {
		t.Fatalf("stale counter lost updates: have %d, want %d", have, want)
	}

	// Reheap must be serialised by reheapMu; concurrent runs would otherwise
	// rebuild both heaps at the same time.
	reheaped := newTxPricedList(newTxLookup())
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				reheaped.Reheap()
			}
		}()
	}
	wg.Wait()

	if have := atomic.LoadInt64(&reheaped.stales); have != 0 {
		t.Fatalf("Reheap left a non-zero stale counter: %d", have)
	}
}

// TestUBF046_ThrottleReplacementsBetweenReorgs checks that the pool refuses to
// churn an unbounded number of transactions between two reorg runs, and that
// the budget is restored by a reorg.
// Upstream d705f5a55 (#23095).
func TestUBF046_ThrottleReplacementsBetweenReorgs(t *testing.T) {
	pool := newUBFPool(t)
	defer pool.Stop()

	fillUBFPool(t, pool, 8)

	// Pretend we already churned more than 25% of the executable slots.
	pool.mu.Lock()
	limit := int(pool.config.GlobalSlots / 4)
	pool.changesSinceReorg = limit + 1
	pool.mu.Unlock()

	// An immediately executable (non-gapped) transaction from a fresh account.
	// Without the throttle it would happily evict somebody else's transaction.
	newcomer := newUBFAccount(t, pool, new(big.Int).Mul(ubfBaseCost(), big.NewInt(100)))
	tx := ubfTx(t, pool, newcomer, 0, big.NewInt(0))

	if err := pool.addRemoteSync(tx); err != ErrTxPoolOverflow {
		t.Fatalf("expected ErrTxPoolOverflow from the replacement throttle, got: %v", err)
	}
	if pool.Get(tx.Hash()) != nil {
		t.Fatal("the throttled transaction must not be in the pool")
	}

	// addRemoteSync above waited for a reorg, which resets the budget.
	pool.mu.Lock()
	changes := pool.changesSinceReorg
	pool.mu.Unlock()
	if changes != 0 {
		t.Fatalf("runReorg did not reset changesSinceReorg: got %d", changes)
	}

	// With the budget restored, the very same transaction is accepted.
	retry := ubfTx(t, pool, newcomer, 0, big.NewInt(0))
	if err := pool.addRemoteSync(retry); err != nil {
		t.Fatalf("expected the transaction to be accepted after the reorg, got: %v", err)
	}
}

// TestUBF047_NoncerDoesNotCacheZero checks that the nonce cache does not pin a
// not-yet-existing account to nonce 0.
// Upstream ada603fab (#25603).
func TestUBF047_NoncerDoesNotCacheZero(t *testing.T) {
	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	noncer := newTxNoncer(statedb)

	t.Run("get", func(t *testing.T) {
		addr := common.BytesToAddress([]byte{0x01})
		if got := noncer.get(addr); got != 0 {
			t.Fatalf("unknown account should report nonce 0, got %d", got)
		}
		if _, cached := noncer.nonces[addr]; cached {
			t.Fatal("the zero nonce of a non-existent account must not be cached")
		}
		// The account is created later on; the noncer must pick that up.
		noncer.fallback.SetNonce(addr, 7)
		if got := noncer.get(addr); got != 7 {
			t.Fatalf("stale zero nonce served after account creation: got %d, want 7", got)
		}
	})

	t.Run("setIfLower", func(t *testing.T) {
		addr := common.BytesToAddress([]byte{0x02})
		noncer.setIfLower(addr, 3)
		if _, cached := noncer.nonces[addr]; cached {
			t.Fatal("the zero nonce of a non-existent account must not be cached")
		}
		noncer.fallback.SetNonce(addr, 9)
		noncer.setIfLower(addr, 5)
		if got := noncer.get(addr); got != 5 {
			t.Fatalf("setIfLower worked off a stale zero nonce: got %d, want 5", got)
		}
	})
}

// TestUBF048_QueueTruncationEvictsStalest checks that the global queue limit
// drops the least recently active accounts, not the most recently active ones.
// Upstream 2bfd9a28d (#24907).
func TestUBF048_QueueTruncationEvictsStalest(t *testing.T) {
	pool := newUBFPool(t)
	defer pool.Stop()

	balance := new(big.Int).Mul(ubfBaseCost(), big.NewInt(100))
	stale := newUBFAccount(t, pool, balance)
	fresh := newUBFAccount(t, pool, balance)

	// Gapped nonces, so both transactions stay in the future queue.
	for _, acc := range []*ubfAccount{stale, fresh} {
		if err := pool.addRemoteSync(ubfTx(t, pool, acc, 5, big.NewInt(0))); err != nil {
			t.Fatalf("failed to queue future tx: %v", err)
		}
	}
	if _, queued := pool.Stats(); queued != 2 {
		t.Fatalf("expected 2 queued transactions, got %d", queued)
	}

	pool.mu.Lock()
	pool.config.GlobalQueue = 1
	pool.beats[stale.addr] = time.Now().Add(-time.Hour)
	pool.beats[fresh.addr] = time.Now()
	pool.truncateQueue()
	_, staleLeft := pool.queue[stale.addr]
	_, freshLeft := pool.queue[fresh.addr]
	pool.mu.Unlock()

	if staleLeft {
		t.Error("queue truncation kept the stalest account")
	}
	if !freshLeft {
		t.Error("queue truncation evicted the most recently active account")
	}
}
