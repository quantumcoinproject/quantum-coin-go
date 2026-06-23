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
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

// The transaction test hook is a devnet-only utility used to measure real TPS.
// When the TXN_HOOK_FILE environment variable points to a JSON file of signed
// transactions grouped into batches, the hook submits each batch into the live
// TxPool, waits for the batch to be committed (mined) on chain, and only then
// submits the next batch. It is started from NewTxPool and is a no-op unless the
// environment variable is set.

const (
	// defaultTxnHookPollInterval is how often the hook checks whether the
	// currently submitted batch has been committed.
	defaultTxnHookPollInterval = 500 * time.Millisecond

	// defaultTxnHookBatchTimeout is the maximum time the hook waits for a single
	// batch to commit before giving up.
	defaultTxnHookBatchTimeout = 10 * time.Minute
)

// TxnTestTransaction is a single signed transaction entry in the hook input
// file. TxnHex is the canonical signed-transaction encoding (the output of
// (*types.Transaction).MarshalBinary), optionally prefixed with "0x".
type TxnTestTransaction struct {
	BatchNumber int64  `json:"batchNumber"`
	TxnHex      string `json:"txnHex"`
}

// TxnTestTransactions is the top-level structure of the hook input file.
// StartBlockNumber is the chain block number at or after which the hook begins
// submitting transactions; the hook waits until the chain head reaches it.
type TxnTestTransactions struct {
	StartBlockNumber int64                `json:"startBlockNumber"`
	Transactions     []TxnTestTransaction `json:"transactions"`
}

// txnTestBatchResult reports the outcome of submitting and committing a single
// batch. It is only emitted when the hook has a progress channel configured
// (used by tests); production runs leave the channel nil.
type txnTestBatchResult struct {
	BatchNumber int64
	Count       int
	Accepted    int
	Elapsed     time.Duration
	TPS         float64
	Committed   bool
}

// txCommitChecker is implemented by the canonical *BlockChain and lets the hook
// determine whether a transaction has been included on chain. When the chain
// backing the pool does not implement it (for example in unit tests using a
// minimal blockChain), the hook falls back to pool absence.
type txCommitChecker interface {
	DoesTransactionExist(hash common.Hash) bool
}

// txnTestHook drives batch-by-batch submission of pre-signed transactions into
// the pool for TPS measurement.
type txnTestHook struct {
	pool *TxPool

	startBlockNumber uint64

	batchNumbers []int64
	batches      [][]*types.Transaction

	pollInterval time.Duration
	batchTimeout time.Duration

	quit chan struct{}

	// progress, when non-nil, receives one result per processed batch and is
	// closed when run returns. Used by tests to observe ordering.
	progress chan txnTestBatchResult
}

// maybeStartTxnTestHook starts the transaction test hook in a background
// goroutine if the TXN_HOOK_FILE environment variable is set. All failures are
// logged and never affect normal pool operation.
func maybeStartTxnTestHook(pool *TxPool) {
	path := defaults.GetTxnHookFile()
	if len(path) == 0 {
		return
	}

	log.Warn("Transaction test hook enabled", "file", path)

	txns, err := loadTxnTestTransactions(path)
	if err != nil {
		log.Error("Transaction test hook failed to load file", "file", path, "err", err)
		return
	}

	batchNumbers, batches, err := buildBatches(txns)
	if err != nil {
		log.Error("Transaction test hook failed to decode transactions", "file", path, "err", err)
		return
	}
	if len(batches) == 0 {
		log.Warn("Transaction test hook has no transactions to submit", "file", path)
		return
	}

	var startBlockNumber uint64
	if txns.StartBlockNumber > 0 {
		startBlockNumber = uint64(txns.StartBlockNumber)
	}

	hook := &txnTestHook{
		pool:             pool,
		startBlockNumber: startBlockNumber,
		batchNumbers:     batchNumbers,
		batches:          batches,
		pollInterval:     defaultTxnHookPollInterval,
		batchTimeout:     defaultTxnHookBatchTimeout,
		quit:             make(chan struct{}),
	}
	pool.txnHook = hook
	go hook.run()
}

// stop signals the hook goroutine to exit. It is safe to call multiple times.
func (h *txnTestHook) stop() {
	select {
	case <-h.quit:
	default:
		close(h.quit)
	}
}

// run submits each batch in order, waiting for commit between batches. It exits
// on completion, on stop (pool shutdown), on Ctrl+C / system termination
// signals, or when a batch fails to commit within batchTimeout.
func (h *txnTestHook) run() {
	if h.progress != nil {
		defer close(h.progress)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	totalBatches := len(h.batches)
	log.Info("Transaction test hook started", "batches", totalBatches, "startBlockNumber", h.startBlockNumber)

	// Hold off submitting until the chain head reaches the configured start
	// block number.
	if !h.waitForStartBlock(sigCh) {
		return
	}

	startAll := time.Now()
	committedTxns := 0

	for i, batch := range h.batches {
		select {
		case <-h.quit:
			log.Info("Transaction test hook stopping", "reason", "quit")
			return
		case <-sigCh:
			log.Info("Transaction test hook stopping", "reason", "signal")
			return
		default:
		}

		log.Info("Transaction test hook submitting batch",
			"batchNumber", h.batchNumbers[i],
			"batch", i+1,
			"totalBatches", totalBatches,
			"txCount", len(batch))

		batchStart := time.Now()
		errs := h.pool.AddLocals(batch)
		accepted := 0
		for _, err := range errs {
			if err == nil {
				accepted++
			} else {
				log.Warn("Transaction test hook transaction rejected", "batchNumber", h.batchNumbers[i], "err", err)
			}
		}

		committed := h.waitForBatchCommit(batch, sigCh)
		elapsed := time.Since(batchStart)
		tps := 0.0
		if secs := elapsed.Seconds(); secs > 0 {
			tps = float64(len(batch)) / secs
		}

		if committed {
			committedTxns += len(batch)
			log.Info("Transaction test hook batch committed",
				"batchNumber", h.batchNumbers[i],
				"txCount", len(batch),
				"accepted", accepted,
				"elapsed", elapsed,
				"tps", tps)
		} else {
			log.Warn("Transaction test hook batch not committed",
				"batchNumber", h.batchNumbers[i],
				"txCount", len(batch),
				"accepted", accepted,
				"elapsed", elapsed)
		}

		if h.progress != nil {
			h.progress <- txnTestBatchResult{
				BatchNumber: h.batchNumbers[i],
				Count:       len(batch),
				Accepted:    accepted,
				Elapsed:     elapsed,
				TPS:         tps,
				Committed:   committed,
			}
		}

		if !committed {
			// Stop processing further batches on timeout, quit, or signal.
			return
		}
	}

	totalElapsed := time.Since(startAll)
	overallTPS := 0.0
	if secs := totalElapsed.Seconds(); secs > 0 {
		overallTPS = float64(committedTxns) / secs
	}
	log.Info("Transaction test hook completed",
		"batches", totalBatches,
		"txCount", committedTxns,
		"elapsed", totalElapsed,
		"tps", overallTPS)
}

// waitForStartBlock blocks until the chain head reaches the configured start
// block number. It returns false if it should stop early (quit or signal).
func (h *txnTestHook) waitForStartBlock(sigCh <-chan os.Signal) bool {
	if h.startBlockNumber == 0 {
		return true
	}

	if h.pool.chain.CurrentBlock().NumberU64() >= h.startBlockNumber {
		return true
	}

	log.Info("Transaction test hook waiting for start block",
		"startBlockNumber", h.startBlockNumber,
		"currentBlock", h.pool.chain.CurrentBlock().NumberU64())

	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if h.pool.chain.CurrentBlock().NumberU64() >= h.startBlockNumber {
				log.Info("Transaction test hook reached start block", "startBlockNumber", h.startBlockNumber)
				return true
			}
		case <-h.quit:
			log.Info("Transaction test hook stopping", "reason", "quit")
			return false
		case <-sigCh:
			log.Info("Transaction test hook stopping", "reason", "signal")
			return false
		}
	}
}

// waitForBatchCommit blocks until every transaction in txs is committed, the
// batch timeout elapses, or a stop/signal is received. It returns true only if
// all transactions committed.
func (h *txnTestHook) waitForBatchCommit(txs []*types.Transaction, sigCh <-chan os.Signal) bool {
	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	var timeout <-chan time.Time
	if h.batchTimeout > 0 {
		t := time.NewTimer(h.batchTimeout)
		defer t.Stop()
		timeout = t.C
	}

	for {
		if h.allCommitted(txs) {
			return true
		}

		select {
		case <-ticker.C:
		case <-h.quit:
			return false
		case <-sigCh:
			return false
		case <-timeout:
			log.Warn("Transaction test hook batch commit timed out", "txCount", len(txs), "timeout", h.batchTimeout)
			return false
		}
	}
}

// allCommitted reports whether every transaction in txs has been committed.
func (h *txnTestHook) allCommitted(txs []*types.Transaction) bool {
	for _, tx := range txs {
		if !h.isCommitted(tx.Hash()) {
			return false
		}
	}
	return true
}

// isCommitted reports whether a single transaction is committed on chain. It
// uses the chain's DoesTransactionExist when available, otherwise treats a
// transaction that is no longer in the pool as committed.
func (h *txnTestHook) isCommitted(hash common.Hash) bool {
	if checker, ok := h.pool.chain.(txCommitChecker); ok {
		return checker.DoesTransactionExist(hash)
	}
	return h.pool.Get(hash) == nil
}

// loadTxnTestTransactions reads and parses the hook input file.
func loadTxnTestTransactions(path string) (*TxnTestTransactions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	txns := new(TxnTestTransactions)
	if err := json.Unmarshal(data, txns); err != nil {
		return nil, err
	}
	return txns, nil
}

// decodeTxnTestTx decodes a single hex-encoded signed transaction.
func decodeTxnTestTx(txnHex string) (*types.Transaction, error) {
	s := strings.TrimSpace(txnHex)
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")

	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return tx, nil
}

// buildBatches decodes all transactions and groups them by batch number,
// returning the batch numbers and batches sorted in ascending batch order.
func buildBatches(txns *TxnTestTransactions) ([]int64, [][]*types.Transaction, error) {
	if txns == nil {
		return nil, nil, nil
	}

	grouped := make(map[int64][]*types.Transaction)
	var order []int64

	for i, entry := range txns.Transactions {
		tx, err := decodeTxnTestTx(entry.TxnHex)
		if err != nil {
			return nil, nil, fmt.Errorf("transaction index %d (batch %d): %w", i, entry.BatchNumber, err)
		}
		if _, ok := grouped[entry.BatchNumber]; !ok {
			order = append(order, entry.BatchNumber)
		}
		grouped[entry.BatchNumber] = append(grouped[entry.BatchNumber], tx)
	}

	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	batches := make([][]*types.Transaction, len(order))
	for i, bn := range order {
		batches[i] = grouped[bn]
	}
	return order, batches, nil
}
