// Copyright 2024 The go-ethereum Authors
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
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"sync"
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
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// txnHookBlockChain is a minimal blockChain implementation that additionally
// implements txCommitChecker so tests can control exactly which transactions
// are considered committed on chain.
type txnHookBlockChain struct {
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed

	mu          sync.Mutex
	committed   map[common.Hash]bool
	blockNumber uint64
}

func (bc *txnHookBlockChain) CurrentBlock() *types.Block {
	bc.mu.Lock()
	number := bc.blockNumber
	bc.mu.Unlock()
	return types.NewBlock(&types.Header{
		Number:   new(big.Int).SetUint64(number),
		GasLimit: bc.gasLimit,
	}, nil, nil, trie.NewStackTrie(nil))
}

// setBlockNumber updates the reported chain head number.
func (bc *txnHookBlockChain) setBlockNumber(number uint64) {
	bc.mu.Lock()
	bc.blockNumber = number
	bc.mu.Unlock()
}

func (bc *txnHookBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return bc.CurrentBlock()
}

func (bc *txnHookBlockChain) StateAt(common.Hash, *big.Int) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *txnHookBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

// DoesTransactionExist satisfies txCommitChecker.
func (bc *txnHookBlockChain) DoesTransactionExist(hash common.Hash) bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.committed[hash]
}

// markCommitted marks the given transactions as committed on chain.
func (bc *txnHookBlockChain) markCommitted(txs ...*types.Transaction) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	for _, tx := range txs {
		bc.committed[tx.Hash()] = true
	}
}

// setupTxnHookPool builds a TxPool backed by a txnHookBlockChain with a single
// generously funded sender, returning the pool, the sender key, and the chain.
func setupTxnHookPool(t *testing.T) (*TxPool, *signaturealgorithm.PrivateKey, *txnHookBlockChain) {
	t.Helper()

	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	statedb.AddBalance(addr, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1_000_000)))

	bc := &txnHookBlockChain{
		statedb:       statedb,
		gasLimit:      300000000,
		chainHeadFeed: new(event.Feed),
		committed:     make(map[common.Hash]bool),
	}

	config := DefaultTxPoolConfig
	config.Journal = "" // avoid touching disk

	pool := NewTxPool(config, txPoolVTestChainConfig, bc)
	return pool, key, bc
}

// signedTxWithNonce returns a valid, signed DefaultFeeTx with the given nonce.
func signedTxWithNonce(t *testing.T, pool *TxPool, key *signaturealgorithm.PrivateKey, nonce uint64) *types.Transaction {
	t.Helper()

	to := common.BytesToAddress([]byte{0x11})
	tx := types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      nonce,
		To:         &to,
		Value:      big.NewInt(0),
		Gas:        params.TxGas,
		MaxGasTier: types.GAS_TIER_DEFAULT,
	})

	signed, err := types.SignTx(tx, pool.signer, key)
	if err != nil {
		t.Fatalf("failed to sign tx (nonce %d): %v", nonce, err)
	}
	return signed
}

// waitFor polls cond until it returns true or the timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return cond()
}

// TestTxnTestHookSubmitsBatchesInOrder verifies that the hook submits each batch
// only after the previous batch has been committed, in ascending batch order.
func TestTxnTestHookSubmitsBatchesInOrder(t *testing.T) {
	pool, key, bc := setupTxnHookPool(t)
	defer pool.Stop()

	// 3 batches of 2 transactions each, sequential nonces 0..5.
	txs := make([]*types.Transaction, 6)
	for i := range txs {
		txs[i] = signedTxWithNonce(t, pool, key, uint64(i))
	}
	batches := [][]*types.Transaction{
		{txs[0], txs[1]},
		{txs[2], txs[3]},
		{txs[4], txs[5]},
	}

	hook := &txnTestHook{
		pool:         pool,
		batchNumbers: []int64{0, 1, 2},
		batches:      batches,
		pollInterval: 5 * time.Millisecond,
		batchTimeout: 5 * time.Second,
		quit:         make(chan struct{}),
		progress:     make(chan txnTestBatchResult, 1),
	}
	pool.txnHook = hook
	go hook.run()

	for i, batch := range batches {
		// The current batch must get submitted into the pool.
		if !waitFor(t, 2*time.Second, func() bool { return pool.Get(batch[0].Hash()) != nil }) {
			t.Fatalf("batch %d was not submitted to the pool", i)
		}

		// The next batch must NOT be submitted until this one is committed.
		if i+1 < len(batches) {
			next := batches[i+1]
			if pool.Get(next[0].Hash()) != nil {
				t.Fatalf("batch %d was submitted before batch %d committed", i+1, i)
			}
		}

		// Commit the current batch and observe the reported result.
		bc.markCommitted(batch...)

		select {
		case res := <-hook.progress:
			if res.BatchNumber != int64(i) {
				t.Fatalf("expected progress for batch %d, got %d", i, res.BatchNumber)
			}
			if !res.Committed {
				t.Fatalf("batch %d reported as not committed", i)
			}
			if res.Count != len(batch) {
				t.Fatalf("batch %d reported count %d, want %d", i, res.Count, len(batch))
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for batch %d progress", i)
		}
	}

	// After the last batch, run should finish and close the progress channel.
	select {
	case _, ok := <-hook.progress:
		if ok {
			t.Fatalf("expected progress channel to be closed after final batch")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for hook to finish")
	}
}

// TestTxnTestHookWaitsForCommit verifies that the hook blocks on an uncommitted
// batch and does not advance to the next batch until commit occurs.
func TestTxnTestHookWaitsForCommit(t *testing.T) {
	pool, key, bc := setupTxnHookPool(t)
	defer pool.Stop()

	tx0 := signedTxWithNonce(t, pool, key, 0)
	tx1 := signedTxWithNonce(t, pool, key, 1)
	batches := [][]*types.Transaction{{tx0}, {tx1}}

	hook := &txnTestHook{
		pool:         pool,
		batchNumbers: []int64{0, 1},
		batches:      batches,
		pollInterval: 5 * time.Millisecond,
		batchTimeout: 5 * time.Second,
		quit:         make(chan struct{}),
		progress:     make(chan txnTestBatchResult, 2),
	}
	pool.txnHook = hook
	go hook.run()

	if !waitFor(t, 2*time.Second, func() bool { return pool.Get(tx0.Hash()) != nil }) {
		t.Fatalf("first batch was not submitted")
	}

	// While batch 0 is uncommitted, batch 1 must stay unsubmitted.
	time.Sleep(150 * time.Millisecond)
	if pool.Get(tx1.Hash()) != nil {
		t.Fatalf("second batch submitted before first batch committed")
	}

	// Commit batch 0; batch 1 must now be submitted.
	bc.markCommitted(tx0)
	if !waitFor(t, 2*time.Second, func() bool { return pool.Get(tx1.Hash()) != nil }) {
		t.Fatalf("second batch was not submitted after first batch committed")
	}

	bc.markCommitted(tx1)
}

// TestTxnTestHookWaitsForStartBlock verifies the hook does not submit any
// transactions until the chain head reaches the configured start block number.
func TestTxnTestHookWaitsForStartBlock(t *testing.T) {
	pool, key, bc := setupTxnHookPool(t)
	defer pool.Stop()

	bc.setBlockNumber(5)

	tx0 := signedTxWithNonce(t, pool, key, 0)

	hook := &txnTestHook{
		pool:             pool,
		startBlockNumber: 10,
		batchNumbers:     []int64{0},
		batches:          [][]*types.Transaction{{tx0}},
		pollInterval:     5 * time.Millisecond,
		batchTimeout:     5 * time.Second,
		quit:             make(chan struct{}),
		progress:         make(chan txnTestBatchResult, 1),
	}
	pool.txnHook = hook
	go hook.run()

	// Below the start block: nothing should be submitted.
	time.Sleep(150 * time.Millisecond)
	if pool.Get(tx0.Hash()) != nil {
		t.Fatalf("transaction submitted before reaching start block number")
	}

	// Reach the start block: the batch should now be submitted and commit.
	bc.setBlockNumber(10)
	if !waitFor(t, 2*time.Second, func() bool { return pool.Get(tx0.Hash()) != nil }) {
		t.Fatalf("transaction not submitted after reaching start block number")
	}
	bc.markCommitted(tx0)

	select {
	case res := <-hook.progress:
		if !res.Committed {
			t.Fatalf("batch reported as not committed")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for batch progress")
	}
}

// TestTxnTestHookThrottlesLookups verifies that consecutive transaction status
// lookups are rate limited to the configured interval.
func TestTxnTestHookThrottlesLookups(t *testing.T) {
	pool, key, bc := setupTxnHookPool(t)
	defer pool.Stop()

	// Four committed transactions; allCommitted must iterate over all of them.
	txs := make([]*types.Transaction, 4)
	for i := range txs {
		txs[i] = signedTxWithNonce(t, pool, key, uint64(i))
	}
	bc.markCommitted(txs...)

	interval := 50 * time.Millisecond
	hook := &txnTestHook{
		pool:           pool,
		lookupInterval: interval,
		quit:           make(chan struct{}),
	}

	start := time.Now()
	if !hook.allCommitted(txs) {
		t.Fatalf("expected all transactions to be reported committed")
	}
	elapsed := time.Since(start)

	// The first lookup is immediate; the remaining three are each throttled by
	// at least one interval.
	min := 3 * interval
	if elapsed < min {
		t.Fatalf("lookups not throttled: elapsed %v, want >= %v", elapsed, min)
	}
}

// TestLoadAndDecodeTxnTransactions verifies the JSON load and hex-decode path
// round-trips transactions and groups them by batch in ascending order.
func TestLoadAndDecodeTxnTransactions(t *testing.T) {
	pool, key, _ := setupTxnHookPool(t)
	defer pool.Stop()

	// Build transactions out of batch order to confirm sorting and grouping.
	tx0 := signedTxWithNonce(t, pool, key, 0)
	tx1 := signedTxWithNonce(t, pool, key, 1)
	tx2 := signedTxWithNonce(t, pool, key, 2)

	encode := func(tx *types.Transaction) string {
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatalf("failed to marshal tx: %v", err)
		}
		return "0x" + hex.EncodeToString(raw)
	}

	input := &TxnTestTransactions{
		StartBlockNumber: 42,
		Transactions: []TxnTestTransaction{
			{BatchNumber: 1, TxnHex: encode(tx2)},
			{BatchNumber: 0, TxnHex: encode(tx0)},
			{BatchNumber: 0, TxnHex: encode(tx1)},
		},
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input json: %v", err)
	}
	path := filepath.Join(t.TempDir(), "txns.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	loaded, err := loadTxnTestTransactions(path)
	if err != nil {
		t.Fatalf("failed to load transactions: %v", err)
	}
	if loaded.StartBlockNumber != 42 {
		t.Fatalf("unexpected start block number: got %d, want 42", loaded.StartBlockNumber)
	}

	batchNumbers, batches, err := buildBatches(loaded)
	if err != nil {
		t.Fatalf("failed to build batches: %v", err)
	}

	if len(batchNumbers) != 2 || batchNumbers[0] != 0 || batchNumbers[1] != 1 {
		t.Fatalf("unexpected batch ordering: %v", batchNumbers)
	}
	if len(batches[0]) != 2 || len(batches[1]) != 1 {
		t.Fatalf("unexpected batch sizes: %d, %d", len(batches[0]), len(batches[1]))
	}

	// Batch 0 preserves input order (tx0 then tx1); batch 1 holds tx2.
	if batches[0][0].Hash() != tx0.Hash() || batches[0][1].Hash() != tx1.Hash() {
		t.Fatalf("batch 0 transactions decoded incorrectly")
	}
	if batches[1][0].Hash() != tx2.Hash() {
		t.Fatalf("batch 1 transaction decoded incorrectly")
	}
}

// TestTxnTestHookEnvActivation verifies the hook is started by NewTxPool when
// TXN_HOOK_FILE is set, and is a no-op when it is unset.
func TestTxnTestHookEnvActivation(t *testing.T) {
	// Disabled case: no env var, no hook.
	os.Unsetenv("TXN_HOOK_FILE")
	disabledPool, key, _ := setupTxnHookPool(t)
	if disabledPool.txnHook != nil {
		t.Fatalf("hook should not be started when TXN_HOOK_FILE is unset")
	}
	disabledPool.Stop()

	// Enabled case: write a file, set the env var, and build a new pool. The
	// transactions are pre-marked committed so the hook completes promptly.
	tx0 := signedTxWithNonce(t, disabledPool, key, 0)
	raw, err := tx0.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}
	input := &TxnTestTransactions{
		Transactions: []TxnTestTransaction{
			{BatchNumber: 0, TxnHex: "0x" + hex.EncodeToString(raw)},
		},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input json: %v", err)
	}
	path := filepath.Join(t.TempDir(), "txns.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	os.Setenv("TXN_HOOK_FILE", path)
	defer os.Unsetenv("TXN_HOOK_FILE")

	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	statedb.AddBalance(addr, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1_000_000)))
	bc := &txnHookBlockChain{
		statedb:       statedb,
		gasLimit:      300000000,
		chainHeadFeed: new(event.Feed),
		committed:     make(map[common.Hash]bool),
	}
	bc.markCommitted(tx0) // ensure the hook can complete quickly

	config := DefaultTxPoolConfig
	config.Journal = ""
	pool := NewTxPool(config, txPoolVTestChainConfig, bc)
	defer pool.Stop()

	if pool.txnHook == nil {
		t.Fatalf("hook should be started when TXN_HOOK_FILE is set")
	}
	if len(pool.txnHook.batches) != 1 || len(pool.txnHook.batches[0]) != 1 {
		t.Fatalf("hook loaded unexpected batches: %v", pool.txnHook.batches)
	}
	if pool.txnHook.batches[0][0].Hash() != tx0.Hash() {
		t.Fatalf("hook decoded transaction incorrectly")
	}
}

// TestTxnTestHookParallelSubmit verifies that a batch is submitted concurrently
// across multiple goroutines (parallelism > 1) with every transaction landing
// in the pool and the batch reported as committed.
func TestTxnTestHookParallelSubmit(t *testing.T) {
	pool, key, bc := setupTxnHookPool(t)
	defer pool.Stop()

	// A single batch of 8 transactions submitted with parallelism 4.
	txs := make([]*types.Transaction, 8)
	for i := range txs {
		txs[i] = signedTxWithNonce(t, pool, key, uint64(i))
	}

	hook := &txnTestHook{
		pool:         pool,
		parallelism:  4,
		batchNumbers: []int64{0},
		batches:      [][]*types.Transaction{txs},
		pollInterval: 5 * time.Millisecond,
		batchTimeout: 5 * time.Second,
		quit:         make(chan struct{}),
		progress:     make(chan txnTestBatchResult, 1),
	}
	pool.txnHook = hook
	go hook.run()

	// All transactions in the batch must reach the pool.
	for i, tx := range txs {
		txHash := tx.Hash()
		if !waitFor(t, 2*time.Second, func() bool { return pool.Get(txHash) != nil }) {
			t.Fatalf("transaction %d was not submitted to the pool", i)
		}
	}

	bc.markCommitted(txs...)

	select {
	case res := <-hook.progress:
		if !res.Committed {
			t.Fatalf("batch reported as not committed")
		}
		if res.Count != len(txs) {
			t.Fatalf("batch reported count %d, want %d", res.Count, len(txs))
		}
		if res.Accepted != len(txs) {
			t.Fatalf("batch reported %d accepted, want %d", res.Accepted, len(txs))
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for batch progress")
	}
}
