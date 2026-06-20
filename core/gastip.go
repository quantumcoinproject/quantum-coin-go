package core

// This file is the pure, self-contained home for the gas-tip / priority-fee selection and
// pool-split logic that activates at GasTipStartBlock. These functions are deterministic and
// free of state/engine/config coupling so they can be unit-tested in isolation. They live in
// package core (rather than proofofstake) because the two-pass executor in state_processor.go
// needs them and proofofstake already imports core, which would otherwise create an import cycle.
//
// Gas tip -- rationale
//
// From GasTipStartBlock, the block gas limit is split into two equal pools:
//   - a "basic" pool (50% of the gas limit) reserved for basic coin-transfer transactions
//     (EOA-to-EOA value transfers with no calldata), and
//   - a "general" pool (the remaining 50%) for all transactions including smart-contract calls
//     and contract creations. The general pool is always exactly 50% and is never enlarged even
//     when the basic pool is underused; basic transactions that do not fit the basic pool may
//     spill into the general pool.
//
// The effective tip (min(gasTipCap, gasFeeCap - baseFee)) only influences which transactions are
// selected into a capacity-limited block: higher-tip transactions are preferred. It does not
// change execution order (which stays hash-sorted) and it is not the authoritative enforcer of
// the pools -- ProcessTransactions re-derives and enforces the two-pass split during execution.
// Because selection runs only on the proposer (consensus agrees on the proposer's chosen set),
// the selection heuristic does not need cross-node determinism; it is nonetheless deterministic
// here for testability.

import (
	"bytes"
	"container/heap"
	"errors"
	"math/big"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
)

// IsBasicTransfer reports whether tx is a basic coin transfer: it has a recipient, carries no
// calldata, and the recipient has no contract code (EOA-to-EOA). codeSizeFn returns the code
// size of the recipient against the relevant state; a nil codeSizeFn treats the recipient as a
// plain account (used by unit tests).
func IsBasicTransfer(tx *types.Transaction, codeSizeFn func(common.Address) int) bool {
	to := tx.To()
	if to == nil { // contract creation
		return false
	}
	if len(tx.Data()) != 0 {
		return false
	}
	if codeSizeFn != nil && codeSizeFn(*to) != 0 {
		return false
	}
	return true
}

// ValidateGasFeeCaps enforces the gas tip / priority fee field rules for a transaction against the
// given base fee. It is the single source of truth shared by the transaction pool (validateTx) and
// consensus execution (ApplyTransaction) so both accept exactly the same set of transactions.
//
// Only dynamic-fee transactions carry tip/feecap fields; for every other type it is a no-op. The
// rules mirror the pool's opt-out semantics:
//   - gasFeeCap and gasTipCap must be non-negative;
//   - gasFeeCap == 0 means the sender opted out of tips (legacy/default), which requires gasTipCap == 0;
//   - otherwise gasFeeCap must cover the base fee and gasTipCap must not exceed gasFeeCap.
//
// This must only be called on the gas-tip-active path; before GasTipStartBlock the legacy
// "caps must be zero" rules continue to apply unchanged.
func ValidateGasFeeCaps(tx *types.Transaction, baseFee *big.Int) error {
	if tx.Type() != types.DynamicFeeTxType {
		return nil
	}
	feeCap := tx.GasFeeCap()
	tipCap := tx.GasTipCap()
	if feeCap.Sign() < 0 || tipCap.Sign() < 0 {
		return errors.New("negative gasFeeCap or gasTipCap")
	}
	if feeCap.Sign() == 0 {
		if tipCap.Sign() != 0 {
			return errors.New("gasTipCap set without gasFeeCap")
		}
		return nil
	}
	if baseFee != nil && feeCap.Cmp(baseFee) < 0 {
		return errors.New("gasFeeCap less than base fee")
	}
	if tipCap.Cmp(feeCap) > 0 {
		return errors.New("gasTipCap greater than gasFeeCap")
	}
	return nil
}

// SplitGasPools splits a block gas limit into the basic pool (50%, floored) and the general
// pool (the remainder, which receives the odd unit). basic + general always equals gasLimit.
func SplitGasPools(gasLimit uint64) (basicBudget, generalBudget uint64) {
	basicBudget = gasLimit / 2
	generalBudget = gasLimit - basicBudget
	return basicBudget, generalBudget
}

// tipCandidate is the next not-yet-selected transaction of an account, with its effective tip
// and hash precomputed for ordering.
type tipCandidate struct {
	addr common.Address
	idx  int // index into the account's nonce-sorted transaction list
	tip  *big.Int
	hash common.Hash
}

// tipHeap orders candidates by effective tip descending, breaking ties by ascending tx hash so
// selection is deterministic.
type tipHeap []*tipCandidate

func (h tipHeap) Len() int { return len(h) }
func (h tipHeap) Less(i, j int) bool {
	c := h[i].tip.Cmp(h[j].tip)
	if c != 0 {
		return c > 0 // higher tip first
	}
	return bytes.Compare(h[i].hash.Bytes(), h[j].hash.Bytes()) < 0
}
func (h tipHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *tipHeap) Push(x interface{}) {
	*h = append(*h, x.(*tipCandidate))
}
func (h *tipHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// SelectByEffectiveTip selects transactions from txnMap (per-account, nonce-sorted, contiguous)
// for inclusion in a block, prioritizing higher effective tip while preserving per-account nonce
// order, and simulating the two-pass packing of ProcessTransactions:
//   - basic transfers fill the basic pool first and may spill into the general pool;
//   - all other transactions compete only for the general pool;
//   - a transaction's gas limit is charged against the pool (mirroring execution's reservation).
//
// Once an account's next transaction cannot be placed (or is invalid), that account's remaining
// (higher-nonce) transactions are skipped to keep nonce order intact. Selection stops at maxCount
// transactions (maxCount <= 0 means unbounded). The returned map preserves per-account nonce order.
func SelectByEffectiveTip(txnMap map[common.Address]types.Transactions, basicBudget, generalBudget uint64,
	codeSizeFn func(common.Address) int, maxCount int) map[common.Address]types.Transactions {
	selected := make(map[common.Address]types.Transactions)
	if len(txnMap) == 0 {
		return selected
	}

	basicRemaining := basicBudget
	generalRemaining := generalBudget

	h := &tipHeap{}
	heap.Init(h)

	pushCandidate := func(addr common.Address, idx int) {
		txns := txnMap[addr]
		if idx >= len(txns) {
			return
		}
		tx := txns[idx]
		tip, err := tx.EffectiveGasTip(tx.BaseFee())
		if err != nil {
			// Invalid fee cap: exclude this tx and (by not pushing) the account's later nonces.
			return
		}
		heap.Push(h, &tipCandidate{addr: addr, idx: idx, tip: tip, hash: tx.Hash()})
	}

	for addr := range txnMap {
		pushCandidate(addr, 0)
	}

	count := 0
	for h.Len() > 0 {
		if maxCount > 0 && count >= maxCount {
			break
		}
		cand := heap.Pop(h).(*tipCandidate)
		tx := txnMap[cand.addr][cand.idx]
		gas := tx.Gas()

		placed := false
		if IsBasicTransfer(tx, codeSizeFn) {
			if basicRemaining >= gas {
				basicRemaining -= gas
				placed = true
			} else if generalRemaining >= gas {
				generalRemaining -= gas
				placed = true
			}
		} else if generalRemaining >= gas {
			generalRemaining -= gas
			placed = true
		}

		if placed == false {
			// Cannot fit this account's next transaction; skip its remaining nonces.
			continue
		}

		selected[cand.addr] = append(selected[cand.addr], tx)
		count++
		pushCandidate(cand.addr, cand.idx+1)
	}

	return selected
}
