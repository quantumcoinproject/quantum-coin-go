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

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	ethereum "github.com/quantumcoinproject/quantum-coin-go"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/proofofstake"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"github.com/quantumcoinproject/quantum-coin-go/tokenv2"
)

const rpcCallTimeout = 30 * time.Second

// txKind classifies a generated transaction for the 50/50 split checks.
// Coin transfers are "basic" (core/gastip.go IsBasicTransfer); token transfers
// (calldata + contract recipient) and the deploy (nil To) are "general".
type txKind string

const (
	kindCoin   txKind = "coin"
	kindToken  txKind = "token"
	kindDeploy txKind = "deploy"
)

// vTx is one hook transaction plus everything learned about it.
type vTx struct {
	tx     *types.Transaction
	hash   common.Hash
	sender common.Address
	batch  int64
	kind   txKind
	effTip *big.Int // expected effective tip per gas (min(tipCap, feeCap-baseFee))

	// From the receipt:
	committed bool
	status    uint64
	block     uint64
	gasUsed   uint64
}

// blockRow is the per-block summary line of the final report.
type blockRow struct {
	number      uint64
	gasLimit    uint64
	gasUsed     uint64
	txCount     int
	basicCount  int
	generalCnt  int
	basicUsed   uint64
	generalUsed uint64
	basicBudget uint64
	genBudget   uint64
	baseFees    *big.Int
	tips        *big.Int
	proposer    string
	rpcRewards  *big.Int // blockRewardsInfo.blockProposerRewards from the RPC
	rpcTips     *big.Int // blockRewardsInfo.tipRewards from the RPC (nil if absent)
}

// verifier aggregates results while checks run.
type verifier struct {
	ec       *ethclient.Client
	failures []string
	warnings []string
	infos    []string
}

func (v *verifier) failf(format string, args ...interface{}) {
	v.failures = append(v.failures, fmt.Sprintf(format, args...))
}

func (v *verifier) warnf(format string, args ...interface{}) {
	v.warnings = append(v.warnings, fmt.Sprintf(format, args...))
}

func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), rpcCallTimeout)
}

// runVerify implements -verify: it loads the hook JSON written by generate
// mode, reads the chain back over RPC, and validates commitment, gas ceilings,
// the 50/50 basic-vs-general split, ordering, tip payment, and token balances.
// See the runbook in main.go for the full list and the consensus code each
// check mirrors.
func runVerify(rpcURL, hookPath string) error {
	cfgName := "mainnet"
	if defaults.DefaultConfig == defaults.DevnetConfig {
		cfgName = "devnet"
	}
	fmt.Printf("txnhookgen verify\n")
	fmt.Printf("  config:           %s (Q_DEFAULT_CONFIG selects devnet; must match the node)\n", cfgName)
	fmt.Printf("  hook file:        %s\n", hookPath)
	fmt.Printf("  rpc:              %s\n", rpcURL)
	fmt.Printf("  GasTipStartBlock: %d\n", defaults.DefaultConfig.PosConfig.GasTipStartBlock)
	fmt.Printf("  GasV3StartBlock:  %d\n", defaults.DefaultConfig.PosConfig.GasV3StartBlock)

	// ---- Load and decode the hook file ----
	raw, err := os.ReadFile(hookPath)
	if err != nil {
		return err
	}
	var hook core.TxnTestTransactions
	if err := json.Unmarshal(raw, &hook); err != nil {
		return fmt.Errorf("decode hook json: %w", err)
	}
	signer := types.LatestSignerForChainID(chainID)

	txs := make([]*vTx, 0, len(hook.Transactions))
	var contractAddr common.Address
	haveContract := false
	for i, ht := range hook.Transactions {
		txBytes, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ht.TxnHex), "0x"))
		if err != nil {
			return fmt.Errorf("tx %d: bad hex: %w", i, err)
		}
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(txBytes); err != nil {
			return fmt.Errorf("tx %d: unmarshal: %w", i, err)
		}
		sender, err := types.Sender(signer, tx)
		if err != nil {
			return fmt.Errorf("tx %d: recover sender: %w", i, err)
		}
		effTip, err := tx.EffectiveGasTip(tx.BaseFee())
		if err != nil {
			return fmt.Errorf("tx %d: effective tip: %w", i, err)
		}
		m := &vTx{tx: tx, hash: tx.Hash(), sender: sender, batch: ht.BatchNumber, effTip: effTip}
		if tx.To() == nil {
			m.kind = kindDeploy
			contractAddr = crypto.CreateAddress(sender, tx.Nonce())
			haveContract = true
		}
		txs = append(txs, m)
	}
	for _, m := range txs {
		if m.kind == kindDeploy {
			continue
		}
		if haveContract && m.tx.To() != nil && *m.tx.To() == contractAddr {
			m.kind = kindToken
		} else {
			m.kind = kindCoin
		}
	}
	fmt.Printf("  transactions:     %d (deploy=%v)\n", len(txs), haveContract)
	if haveContract {
		fmt.Printf("  token contract:   %s\n", contractAddr.Hex())
	}

	// ---- Connect ----
	ec, err := ethclient.Dial(rpcURL)
	if err != nil {
		return fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	v := &verifier{ec: ec}
	{
		ctx, cancel := ctxTimeout()
		gotChainID, err := ec.ChainID(ctx)
		cancel()
		if err != nil {
			return fmt.Errorf("chainId: %w", err)
		}
		if gotChainID.Cmp(chainID) != 0 {
			return fmt.Errorf("chain id mismatch: node %s, expected %s", gotChainID, chainID)
		}
	}

	// ---- Receipts / commitment ----
	blockSet := map[uint64]bool{}
	for i, m := range txs {
		ctx, cancel := ctxTimeout()
		rec, err := ec.TransactionReceipt(ctx, m.hash)
		cancel()
		if err != nil {
			v.failf("commitment: tx %d (%s, batch %d, %s) has no receipt: %v", i, m.hash.Hex(), m.batch, m.kind, err)
			continue
		}
		m.committed = true
		m.status = rec.Status
		m.block = rec.BlockNumber.Uint64()
		m.gasUsed = rec.GasUsed
		blockSet[m.block] = true
		if rec.Status != types.ReceiptStatusSuccessful {
			v.failf("commitment: tx %d (%s, batch %d, %s) failed with status %d in block %d", i, m.hash.Hex(), m.batch, m.kind, rec.Status, m.block)
		}
	}

	blockNums := make([]uint64, 0, len(blockSet))
	for n := range blockSet {
		blockNums = append(blockNums, n)
	}
	sort.Slice(blockNums, func(i, j int) bool { return blockNums[i] < blockNums[j] })
	if len(blockNums) == 0 {
		v.failf("commitment: no hook transaction is committed at all")
		return v.report(nil)
	}

	byHash := map[common.Hash]*vTx{}
	for _, m := range txs {
		byHash[m.hash] = m
	}

	// codeSizeFn mirrors the statedb GetCodeSize the node uses for
	// IsBasicTransfer. The token contract address is known; every other
	// recipient in the hook file is a freshly generated EOA. Anything else
	// (foreign traffic) is resolved via eth_getCode and cached.
	codeCache := map[common.Address]int{}
	codeSizeFn := func(a common.Address) int {
		if haveContract && a == contractAddr {
			return 1
		}
		if sz, ok := codeCache[a]; ok {
			return sz
		}
		ctx, cancel := ctxTimeout()
		code, err := ec.CodeAt(ctx, a, nil)
		cancel()
		if err != nil {
			v.warnf("codeAt %s failed (%v); treating as EOA", a.Hex(), err)
			code = nil
		}
		codeCache[a] = len(code)
		return len(code)
	}

	// ---- Per-block checks ----
	rows := make([]*blockRow, 0, len(blockNums))
	blockTxLists := map[uint64][]*vTx{}

	for _, bn := range blockNums {
		ctx, cancel := ctxTimeout()
		block, err := ec.BlockByNumber(ctx, new(big.Int).SetUint64(bn))
		cancel()
		if err != nil {
			v.failf("block %d: fetch failed: %v", bn, err)
			continue
		}
		header := block.Header()
		row := &blockRow{number: bn, gasLimit: header.GasLimit, gasUsed: header.GasUsed, txCount: len(block.Transactions())}

		// Gas ceiling checks: from GasV2StartBlock the limit is dynamic within
		// [MIN_DYNAMIC_GAS_LIMIT, GetMaxGasLimit]; GetMaxGasLimit drops to
		// DefaultGasLimitV2 from GasV3StartBlock (defaults/config.go).
		if bn >= defaults.DefaultConfig.PosConfig.GasV2StartBlock {
			maxGas := defaults.GetMaxGasLimit(bn)
			if header.GasLimit > maxGas {
				v.failf("gas ceiling: block %d gasLimit %d exceeds GetMaxGasLimit %d", bn, header.GasLimit, maxGas)
			}
			if header.GasLimit < defaults.MIN_DYNAMIC_GAS_LIMIT {
				v.failf("gas ceiling: block %d gasLimit %d below MIN_DYNAMIC_GAS_LIMIT %d", bn, header.GasLimit, defaults.MIN_DYNAMIC_GAS_LIMIT)
			}
		}
		if maxCount := defaults.GetMaxTransactionsForBlock(bn); len(block.Transactions()) > maxCount {
			v.failf("txn count: block %d has %d txns, max %d", bn, len(block.Transactions()), maxCount)
		}

		// Build the ordered execution list. Foreign transactions (not from the
		// hook file) still participate in pool accounting, so they are wrapped
		// into ad-hoc metas.
		ordered := make([]*vTx, 0, len(block.Transactions()))
		for _, btx := range block.Transactions() {
			if m, ok := byHash[btx.Hash()]; ok {
				ordered = append(ordered, m)
				continue
			}
			sender, err := types.Sender(signer, btx)
			if err != nil {
				v.failf("block %d: foreign tx %s: recover sender: %v", bn, btx.Hash().Hex(), err)
				continue
			}
			effTip, err := btx.EffectiveGasTip(btx.BaseFee())
			if err != nil {
				effTip = new(big.Int)
			}
			ctx, cancel := ctxTimeout()
			rec, err := ec.TransactionReceipt(ctx, btx.Hash())
			cancel()
			if err != nil {
				v.failf("block %d: foreign tx %s: receipt: %v", bn, btx.Hash().Hex(), err)
				continue
			}
			v.warnf("block %d contains foreign (non-hook) tx %s", bn, btx.Hash().Hex())
			ordered = append(ordered, &vTx{
				tx: btx, hash: btx.Hash(), sender: sender, batch: -1, kind: kindCoin,
				effTip: effTip, committed: true, status: rec.Status, block: bn, gasUsed: rec.GasUsed,
			})
		}
		blockTxLists[bn] = ordered

		// Replay the authoritative two-pass pool accounting of
		// core/state_processor.go ProcessTransactions (see replayTwoPass).
		replay := make([]replayTx, 0, len(ordered))
		for _, m := range ordered {
			isBasic := core.IsBasicTransfer(m.tx, codeSizeFn)
			if isBasic {
				row.basicCount++
			} else {
				row.generalCnt++
			}
			replay = append(replay, replayTx{sender: m.sender, gas: m.tx.Gas(), gasUsed: m.gasUsed, basic: isBasic, label: m.hash.Hex()})
		}
		basicBudget, generalBudget := core.SplitGasPools(header.GasLimit)
		row.basicBudget, row.genBudget = basicBudget, generalBudget
		var problems []string
		row.basicUsed, row.generalUsed, problems = replayTwoPass(replay, header.GasLimit)
		for _, p := range problems {
			v.failf("split: block %d: %s", bn, p)
		}
		if row.basicUsed > basicBudget {
			v.failf("split: block %d basic pool used %d exceeds budget %d", bn, row.basicUsed, basicBudget)
		}
		if row.generalUsed > generalBudget {
			v.failf("split: block %d general pool used %d exceeds budget %d", bn, row.generalUsed, generalBudget)
		}
		if row.basicUsed+row.generalUsed != header.GasUsed {
			v.failf("split: block %d replayed gas %d+%d != header.GasUsed %d", bn, row.basicUsed, row.generalUsed, header.GasUsed)
		}

		// Fee totals for the reward checks below.
		row.baseFees = new(big.Int)
		row.tips = new(big.Int)
		for _, m := range ordered {
			gasUsed := new(big.Int).SetUint64(m.gasUsed)
			row.baseFees.Add(row.baseFees, new(big.Int).Mul(m.tx.GasPrice(), gasUsed))
			row.tips.Add(row.tips, new(big.Int).Mul(m.effTip, gasUsed))
		}

		// Proposer identity via the consensus data RPC. header.Coinbase is
		// always the zero address in this chain, so this is the only way to
		// see who proposed (and whose depositor earns the rewards).
		{
			ctx, cancel := ctxTimeout()
			cd, err := ec.GetBlockConsensusData(ctx, new(big.Int).SetUint64(bn))
			cancel()
			if err != nil {
				v.warnf("block %d: proofofstake_getBlockConsensusData failed: %v", bn, err)
			} else {
				if cd.Data != nil {
					row.proposer = cd.Data.BlockProposer.Hex()
				}
				if cd.BlockRewardsInfo != nil {
					if r, err := hexutil.DecodeBig(cd.BlockRewardsInfo.BlockProposerRewards); err == nil {
						row.rpcRewards = r
					}
					if cd.BlockRewardsInfo.TipRewards != "" {
						if r, err := hexutil.DecodeBig(cd.BlockRewardsInfo.TipRewards); err == nil {
							row.rpcTips = r
						}
					}
				}
			}
		}
		rows = append(rows, row)
	}

	// ---- Ordering checks ----
	// Hard: per-sender nonces must be strictly sequential in commit order
	// (block asc, then in-block position). This is the invariant both the
	// selection heap and the execution cursor preserve.
	orderedAll := make([]*vTx, 0, len(txs))
	for _, bn := range blockNums {
		orderedAll = append(orderedAll, blockTxLists[bn]...)
	}
	lastNonce := map[common.Address]uint64{}
	seen := map[common.Address]bool{}
	for _, m := range orderedAll {
		if seen[m.sender] && m.tx.Nonce() != lastNonce[m.sender]+1 {
			v.failf("ordering: sender %s nonce %d follows %d (must be sequential)", m.sender.Hex(), m.tx.Nonce(), lastNonce[m.sender])
		}
		lastNonce[m.sender] = m.tx.Nonce()
		seen[m.sender] = true
	}

	// Soft: within a batch, a higher effective tip should not land in a later
	// block than a lower one from a different sender. Selection is
	// tip-ordered (core/gastip.go SelectByEffectiveTip) but packing is
	// nonce-ordered, and single-block batches trivially satisfy this, so
	// inversions are reported as warnings with counts rather than failures.
	tipInversions, tipPairs := 0, 0
	byBatch := map[int64][]*vTx{}
	for _, m := range txs {
		if m.committed {
			byBatch[m.batch] = append(byBatch[m.batch], m)
		}
	}
	for _, list := range byBatch {
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				a, b := list[i], list[j]
				if a.sender == b.sender || a.effTip.Cmp(b.effTip) == 0 {
					continue
				}
				// Orient so a has the higher tip.
				if a.effTip.Cmp(b.effTip) < 0 {
					a, b = b, a
				}
				tipPairs++
				if a.block > b.block {
					tipInversions++
				}
			}
		}
	}
	if tipPairs > 0 {
		v.infos = append(v.infos, fmt.Sprintf("tip ordering: %d/%d cross-sender same-batch pairs inverted (higher tip landed later)", tipInversions, tipPairs))
		if tipInversions*5 > tipPairs { // >20% inversions is suspicious even for a soft check
			v.warnf("tip ordering: high inversion rate %d/%d", tipInversions, tipPairs)
		}
	}

	// Tip mix: the file must contain both tipped and tipless transactions of
	// each kind present (coin and general), and under contention (a batch+kind
	// spanning multiple blocks) the tipped ones should be included first while
	// tipless ones are excluded to later blocks by SelectByEffectiveTip.
	var mixTippedCoin, mixTiplessCoin, mixTippedGen, mixTiplessGen int
	for _, m := range txs {
		if !m.committed {
			continue
		}
		tipped := m.effTip.Sign() > 0
		if m.kind == kindCoin {
			if tipped {
				mixTippedCoin++
			} else {
				mixTiplessCoin++
			}
		} else {
			if tipped {
				mixTippedGen++
			} else {
				mixTiplessGen++
			}
		}
	}
	anyTipped := mixTippedCoin+mixTippedGen > 0
	// All-tipped with no tipless transactions is the -tipsascending signature:
	// there the schedule is strictly increasing and pairwise distinct, so
	// highest-tip-first inclusion is enforced STRICTLY below instead of the
	// soft mixed-mode reporting.
	allTipped := anyTipped && mixTiplessCoin == 0 && mixTiplessGen == 0
	if anyTipped {
		v.infos = append(v.infos, fmt.Sprintf("tip mix: coin %d tipped / %d tipless; general %d tipped / %d tipless",
			mixTippedCoin, mixTiplessCoin, mixTippedGen, mixTiplessGen))
		if allTipped {
			v.infos = append(v.infos, "all transactions tipped (ascending mode): enforcing strict highest-tip-first inclusion per batch+kind")
		} else {
			if mixTippedCoin == 0 || mixTiplessCoin == 0 {
				v.warnf("tip mix: coin transactions are not a tipped/tipless mix (%d/%d)", mixTippedCoin, mixTiplessCoin)
			}
			if haveContract && (mixTippedGen == 0 || mixTiplessGen == 0) {
				v.warnf("tip mix: general transactions are not a tipped/tipless mix (%d/%d)", mixTippedGen, mixTiplessGen)
			}
		}

		// Exclusion under contention, per batch and pool kind.
		contended := false
		for _, batch := range sortedBatchKeys(byBatch) {
			list := byBatch[batch]
			for _, wantCoin := range []bool{true, false} {
				var members []*vTx
				minBlock, maxBlock := uint64(1<<62), uint64(0)
				for _, m := range list {
					if (m.kind == kindCoin) != wantCoin {
						continue
					}
					members = append(members, m)
					if m.block < minBlock {
						minBlock = m.block
					}
					if m.block > maxBlock {
						maxBlock = m.block
					}
				}
				if len(members) == 0 {
					continue
				}
				kindName := "coin"
				if !wantCoin {
					kindName = "general"
				}
				// Strict ascending check runs even for single-block groups (a
				// no-op there); the reporting below only concerns spans.
				if allTipped {
					if violations, example := ascendingExclusionViolations(members); violations > 0 {
						v.failf("ascending tips: batch %d %s: %d cross-sender pairs where a HIGHER tip landed in a LATER block (e.g. %s) — selection did not include highest tips first",
							batch, kindName, violations, example)
					}
				}
				if minBlock == maxBlock {
					continue
				}
				contended = true
				if allTipped {
					// Report the boundary: the lowest tip that made the first
					// block vs the highest tip that was excluded to later blocks.
					minFirst, maxLater := new(big.Int), new(big.Int)
					included, excluded := 0, 0
					for _, m := range members {
						if m.block == minBlock {
							included++
							if minFirst.Sign() == 0 || m.effTip.Cmp(minFirst) < 0 {
								minFirst.Set(m.effTip)
							}
						} else {
							excluded++
							if m.effTip.Cmp(maxLater) > 0 {
								maxLater.Set(m.effTip)
							}
						}
					}
					v.infos = append(v.infos, fmt.Sprintf(
						"ascending exclusion: batch %d %s spans blocks %d-%d: first block included %d txns (lowest tip %s); %d lower-tip txns excluded to later blocks (highest excluded tip %s)",
						batch, kindName, minBlock, maxBlock, included, minFirst, excluded, maxLater))
					continue
				}
				var firstTipped, firstTipless, laterTipped, laterTipless, excluded int
				for _, m := range members {
					tipped := m.effTip.Sign() > 0
					if m.block == minBlock {
						if tipped {
							firstTipped++
						} else {
							firstTipless++
						}
					} else {
						if tipped {
							laterTipped++
						} else {
							laterTipless++
							excluded++
						}
					}
				}
				v.infos = append(v.infos, fmt.Sprintf(
					"exclusion: batch %d %s spans blocks %d-%d: first block took %d tipped / %d tipless; later blocks took %d tipped / %d tipless (%d tipless excluded from the first block)",
					batch, kindName, minBlock, maxBlock, firstTipped, firstTipless, laterTipped, laterTipless, excluded))
			}
		}
		if !contended {
			v.warnf("tip exclusion not exercised: no batch+kind spanned multiple blocks (no pool contention) — raise -levels so selection has to exclude transactions")
		}
	}

	// ---- Tip payment / burn accounting (consensus/proofofstake Finalize) ----
	// Expected per-block staking contract balance delta:
	//   GetReward(N) + TxnFeeRewardsPercentage% of base fees + 100% of tips.
	// Burn (ZERO_ADDRESS) delta: the remaining base-fee share. The tip goes to
	// the proposer's depositor inside the staking contract; the proposer's own
	// account balance never changes.
	stakingAddr := common.HexToAddress(staking.GetStakingContract_Address_String())
	feePct := big.NewInt(defaults.DefaultConfig.PosConfig.TxnFeeRewardsPercentage)
	balanceAt := func(addr common.Address, bn uint64) *big.Int {
		ctx, cancel := ctxTimeout()
		defer cancel()
		bal, err := ec.BalanceAt(ctx, addr, new(big.Int).SetUint64(bn))
		if err != nil {
			v.warnf("balanceAt %s @%d failed: %v (state pruned? verify promptly after the run)", addr.Hex(), bn, err)
			return nil
		}
		return bal
	}
	// Below defaults.Config.TxnFeeCutoffBlock the fee-split branch in Finalize
	// is skipped entirely and state_transition credits the whole charge —
	// including the effective tip — to the coinbase, which is always the zero
	// address on this chain. Devnet activates the split from the start
	// (TxnFeeCutoffBlock 2) so the tip-payment path is testable; mainnet keeps
	// the historical 1607600. The expectations below model both regimes.
	txnFeeCutoff := defaults.DefaultConfig.TxnFeeCutoffBlock
	feeSplitActive := blockNums[0] >= txnFeeCutoff
	for _, row := range rows {
		prevStaking := balanceAt(stakingAddr, row.number-1)
		curStaking := balanceAt(stakingAddr, row.number)
		prevZero := balanceAt(common.ZERO_ADDRESS, row.number-1)
		curZero := balanceAt(common.ZERO_ADDRESS, row.number)
		if prevStaking == nil || curStaking == nil || prevZero == nil || curZero == nil {
			continue
		}
		feeShare := common.SafeRelativePercentageBigInt(row.baseFees, feePct)
		expectedStaking := proofofstake.GetReward(new(big.Int).SetUint64(row.number))
		expectedBurn := new(big.Int)
		if row.number >= txnFeeCutoff {
			expectedStaking = new(big.Int).Add(expectedStaking, feeShare)
			expectedBurn = new(big.Int).Sub(row.baseFees, feeShare)
			if defaults.IsGasTipActive(row.number) {
				expectedStaking.Add(expectedStaking, row.tips)
			}
		} else {
			// Pre-cutoff: state_transition credits (baseFee + effTip) * gasUsed
			// to the zero-address coinbase; Finalize adds no fee/tip rewards.
			expectedBurn = new(big.Int).Add(row.baseFees, row.tips)
		}
		actualStaking := new(big.Int).Sub(curStaking, prevStaking)
		if actualStaking.Cmp(expectedStaking) != 0 {
			v.failf("tip payment: block %d staking delta %s != expected %s (reward + %s%% fees %s + tips %s, feeSplitActive=%v)",
				row.number, actualStaking, expectedStaking, feePct, feeShare, row.tips, row.number >= txnFeeCutoff)
		}
		actualBurn := new(big.Int).Sub(curZero, prevZero)
		if actualBurn.Cmp(expectedBurn) != 0 {
			v.failf("burn: block %d ZERO_ADDRESS delta %s != expected %s", row.number, actualBurn, expectedBurn)
		}

		// Cross-check the RPC-reported rewards (ParseRewardsInfo) against the
		// same consensus expectation: blockProposerRewards must include the
		// base reward, the fee share, and — from GasTipStartBlock — the tip
		// total, with tipRewards broken out when non-zero.
		if row.number >= txnFeeCutoff {
			if row.rpcRewards != nil && row.rpcRewards.Cmp(expectedStaking) != 0 {
				v.failf("rpc rewards: block %d blockProposerRewards %s != expected %s (must include tips)", row.number, row.rpcRewards, expectedStaking)
			}
			if defaults.IsGasTipActive(row.number) && row.tips.Sign() > 0 {
				if row.rpcTips == nil {
					v.failf("rpc rewards: block %d tipRewards missing from blockRewardsInfo (expected %s)", row.number, row.tips)
				} else if row.rpcTips.Cmp(row.tips) != 0 {
					v.failf("rpc rewards: block %d tipRewards %s != expected %s", row.number, row.rpcTips, row.tips)
				}
			}
		}
	}
	if !feeSplitActive {
		v.warnf("blocks are below defaults TxnFeeCutoffBlock (%d) — the fee split AND the gas tip are credited to the zero-address coinbase instead of the proposer's depositor, so tip payment is not exercised in this range", txnFeeCutoff)
	}

	// ---- Token balance conservation ----
	if haveContract {
		v.verifyTokens(txs, contractAddr)
	}

	return v.report(rows)
}

// ascendingExclusionViolations counts cross-sender pairs within one batch+kind
// where a strictly higher effective tip landed in a strictly later block than a
// lower one. When SelectByEffectiveTip drives inclusion and every transaction
// is tipped with pairwise distinct, per-sender-ascending tips, no such pair can
// occur: the selection heap always takes the highest-tip account head, and a
// deferred higher-tip transaction implies pool exhaustion that would also have
// deferred every lower-tip candidate of the same pool. Returns the count and
// one example pair for the failure message.
func ascendingExclusionViolations(members []*vTx) (int, string) {
	violations := 0
	example := ""
	for i := 0; i < len(members); i++ {
		for j := i + 1; j < len(members); j++ {
			a, b := members[i], members[j]
			if a.sender == b.sender {
				continue
			}
			if a.effTip.Cmp(b.effTip) < 0 {
				a, b = b, a
			}
			if a.effTip.Cmp(b.effTip) != 0 && a.block > b.block {
				violations++
				if example == "" {
					example = fmt.Sprintf("tip %s landed in block %d while lower tip %s landed in block %d", a.effTip, a.block, b.effTip, b.block)
				}
			}
		}
	}
	return violations, example
}

// sortedBatchKeys returns the batch numbers of the map in ascending order so
// report lines are deterministic.
func sortedBatchKeys(m map[int64][]*vTx) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// replayTx is one block transaction as seen by the two-pass replay.
type replayTx struct {
	sender  common.Address
	gas     uint64 // tx gas limit (a pool must hold this much up front)
	gasUsed uint64 // net pool consumption (limit minus refund)
	basic   bool   // core/gastip.go IsBasicTransfer
	label   string // for problem messages
}

// replayTwoPass mirrors the two-pass pool accounting of
// core/state_processor.go ProcessTransactions over an already-committed block:
// pass 1 runs basic transfers against the basic pool (gl/2 via
// core.SplitGasPools), deferring on account-block or pool exhaustion (SubGas
// requires the full gas limit up front; only gasUsed stays consumed after the
// refund); pass 2 runs the deferred list against the general pool. A non-basic
// transaction blocks its account so higher nonces stay ordered behind it.
// Since every input transaction actually committed, a general-pool exhaustion
// here means the replay disagrees with the node and is reported as a problem.
func replayTwoPass(txs []replayTx, blockGasLimit uint64) (basicUsed, generalUsed uint64, problems []string) {
	basicBudget, generalBudget := core.SplitGasPools(blockGasLimit)
	basicRemaining, generalRemaining := basicBudget, generalBudget
	blocked := map[common.Address]bool{}
	var deferred []replayTx
	for _, t := range txs {
		if blocked[t.sender] || !t.basic {
			blocked[t.sender] = true
			deferred = append(deferred, t)
			continue
		}
		if basicRemaining < t.gas {
			// Pool exhaustion defers the tx and blocks the account.
			blocked[t.sender] = true
			deferred = append(deferred, t)
			continue
		}
		basicRemaining -= t.gasUsed
		basicUsed += t.gasUsed
	}
	for _, t := range deferred {
		if generalRemaining < t.gas {
			problems = append(problems, fmt.Sprintf("tx %s needs %d gas in the general pool but only %d remains — replay disagrees with the node", t.label, t.gas, generalRemaining))
			continue
		}
		generalRemaining -= t.gasUsed
		generalUsed += t.gasUsed
	}
	return basicUsed, generalUsed, problems
}

// verifyTokens checks tokenv2 balances: each leaf recipient (token transfers of
// the highest batch) holds exactly the transferred amount, and the sum of leaf
// amounts equals totalSupply (root and intermediates net to zero by
// construction: each receives needTok[k] and forwards 2*needTok[k+1] == needTok[k]).
func (v *verifier) verifyTokens(txs []*vTx, contractAddr common.Address) {
	parsedABI, err := tokenv2.Tokenv2MetaData.GetAbi()
	if err != nil {
		v.failf("token: ABI: %v", err)
		return
	}

	maxBatch := int64(-1)
	for _, m := range txs {
		if m.kind == kindToken && m.batch > maxBatch {
			maxBatch = m.batch
		}
	}
	if maxBatch < 0 {
		v.warnf("token: contract deployed but no token transfers found")
		return
	}

	call := func(data []byte) ([]byte, error) {
		ctx, cancel := ctxTimeout()
		defer cancel()
		return v.ec.CallContract(ctx, ethereum.CallMsg{To: &contractAddr, Data: data}, nil)
	}

	leafTotal := new(big.Int)
	checked := 0
	for _, m := range txs {
		if m.kind != kindToken || m.batch != maxBatch || !m.committed {
			continue
		}
		data := m.tx.Data()
		if len(data) < 4 {
			v.failf("token: leaf transfer %s has short calldata", m.hash.Hex())
			continue
		}
		method, err := parsedABI.MethodById(data[:4])
		if err != nil || method.Name != "transfer" {
			v.failf("token: tx %s calldata is not transfer(...)", m.hash.Hex())
			continue
		}
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil || len(args) != 2 {
			v.failf("token: tx %s: unpack transfer args: %v", m.hash.Hex(), err)
			continue
		}
		recipient := args[0].(common.Address)
		amount := args[1].(*big.Int)
		leafTotal.Add(leafTotal, amount)

		// Spot-check the first leaves' balances via eth_call.
		if checked < 32 {
			packed, err := parsedABI.Pack("balanceOf", recipient)
			if err != nil {
				v.failf("token: pack balanceOf: %v", err)
				continue
			}
			ret, err := call(packed)
			if err != nil {
				v.warnf("token: balanceOf(%s) call failed: %v", recipient.Hex(), err)
				continue
			}
			out, err := parsedABI.Unpack("balanceOf", ret)
			if err != nil || len(out) != 1 {
				v.failf("token: unpack balanceOf: %v", err)
				continue
			}
			bal := out[0].(*big.Int)
			if bal.Cmp(amount) != 0 {
				v.failf("token: leaf %s balance %s != transferred %s", recipient.Hex(), bal, amount)
			}
			checked++
		}
	}

	packed, err := parsedABI.Pack("totalSupply")
	if err != nil {
		v.failf("token: pack totalSupply: %v", err)
		return
	}
	ret, err := call(packed)
	if err != nil {
		v.warnf("token: totalSupply call failed: %v", err)
		return
	}
	out, err := parsedABI.Unpack("totalSupply", ret)
	if err != nil || len(out) != 1 {
		v.failf("token: unpack totalSupply: %v", err)
		return
	}
	totalSupply := out[0].(*big.Int)
	if totalSupply.Cmp(leafTotal) != 0 {
		v.failf("token: conservation: sum of leaf transfers %s != totalSupply %s", leafTotal, totalSupply)
	} else {
		v.infos = append(v.infos, fmt.Sprintf("token conservation: %d leaf balances checked; leaf total == totalSupply == %s", checked, totalSupply))
	}
}

// report prints the final PASS/FAIL report and returns an error when any hard
// check failed (so the process exits non-zero).
func (v *verifier) report(rows []*blockRow) error {
	fmt.Println()
	if len(rows) > 0 {
		fmt.Println("Per-block summary:")
		fmt.Printf("  %-8s %-11s %-11s %-5s %-6s %-8s %-22s %-22s %-14s %s\n",
			"block", "gasLimit", "gasUsed", "txns", "basic", "general", "basicUsed/budget", "generalUsed/budget", "tips (wei)", "proposer")
		for _, r := range rows {
			fmt.Printf("  %-8d %-11d %-11d %-5d %-6d %-8d %-22s %-22s %-14s %s\n",
				r.number, r.gasLimit, r.gasUsed, r.txCount, r.basicCount, r.generalCnt,
				fmt.Sprintf("%d/%d", r.basicUsed, r.basicBudget),
				fmt.Sprintf("%d/%d", r.generalUsed, r.genBudget),
				r.tips.String(), r.proposer)
		}
		fmt.Println()
	}
	for _, s := range v.infos {
		fmt.Printf("INFO: %s\n", s)
	}
	for _, s := range v.warnings {
		fmt.Printf("WARN: %s\n", s)
	}
	for _, s := range v.failures {
		fmt.Printf("FAIL: %s\n", s)
	}
	if len(v.failures) == 0 {
		fmt.Println("RESULT: PASS (all hard checks green)")
		return nil
	}
	fmt.Printf("RESULT: FAIL (%d failures)\n", len(v.failures))
	return fmt.Errorf("%d verification failures", len(v.failures))
}
