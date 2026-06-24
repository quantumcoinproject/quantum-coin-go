package proofofstake

// This file is the single, self-contained home for the dynamic block gas-limit logic.
// Layer A holds the pure, deterministic calculation (no state / engine / config coupling)
// so it can be unit-tested in isolation. Layer B holds thin engine glue that reads/writes
// the round-robin nil-block status from the consensus context and calls into Layer A.
// No gas-limit logic lives anywhere else; other files only call into this module.
//
// Dynamic TPS -- rationale
//
// This mechanism dynamically modulates throughput (transactions per second, governed by the
// block gas limit) as a function of observed consensus health.
//
// Why liveness is prioritized over throughput: a Byzantine-fault-tolerant (BFT) state-machine-
// replication protocol has two fundamental correctness properties -- safety (no two conflicting
// blocks are finalized) and liveness (the chain continues to make progress). Safety is
// non-negotiable and is preserved unconditionally by the consensus protocol regardless of the
// gas limit. Throughput is not a correctness property; it is a performance parameter. A chain
// that loses liveness is unavailable to all participants and processes zero transactions,
// whereas a chain that temporarily lowers its throughput remains fully available and continues
// to finalize blocks. Consequently the marginal cost of a liveness failure strictly dominates
// the marginal benefit of additional throughput, and the gas limit -- which bounds the
// computation and bandwidth required to produce, propagate, and verify each block -- is the
// natural control variable for trading throughput to recover liveness margin. Two properties of
// this network sharpen the asymmetry: (i) post-quantum cryptographic (PQC) signatures are
// substantially larger than classical signatures, raising per-block size,
// propagation latency, and verification cost; and (ii) the 3+1 round BFT protocol incurs
// additional communication rounds, widening the window in which network degradation can prevent
// timely consensus. Under these conditions, conservatively sacrificing throughput to preserve
// liveness headroom is the rational design choice.
//
// Signal -- round-2 nil blocks (network-stress indicator): a round-2 nil block is produced only
// when validators fail to reach consensus in the first round, which correlates with network-level
// stress (latency, partition, bandwidth saturation) rather than a single faulty node. A recent
// round-2 nil is therefore treated as a high-severity signal and applies a hard, distance-banded
// cap that collapses the gas limit toward the floor, relaxing it only gradually (F -> 2F -> 3F ->
// ... -> 8F) as the event ages out of the observation window (Pass1). Additional round-2 nils
// beyond the nearest one each apply a flat absolute gas penalty (GAS_PENALTY_R2_UNITS *
// GAS_PENALTY_UNIT per block) on top of the cap (Pass2). The single nearest round-2 is deliberately
// excluded from the Pass2 penalty because it is already fully accounted for by the Pass1 cap;
// charging it in both passes would double-penalize the same event.
//
// Signal -- round-1 nil blocks (proposer-availability indicator): a round-1 nil block occurs when
// the selected proposer is offline or unresponsive. Isolated occurrences are benign, so round-1
// nils are ignored entirely until at least GAS_PENALTY_MIN_R1_COUNT of them accumulate within the
// window. This threshold (rather than penalizing from the very first nil) serves two purposes: a
// single offline validator -- which can produce at most a few nils as it cycles through proposer
// selection -- must not be able to move the gas limit on its own; and crossing the threshold of
// 5+ round-1 nils within the 32-block window is itself a meaningful signal of a broader problem
// (e.g. validators unable to keep up with transaction speed, or sustained proposer unavailability),
// which is precisely the condition under which throttling throughput to protect liveness is
// warranted. Once the threshold is met, each round-1 nil applies a flat absolute gas penalty
// (GAS_PENALTY_R1_UNITS * GAS_PENALTY_UNIT per block).
//
// Deliberate exclusion of stake weight: the function does not use validator staking percentage.
// Block-proposer selection is already weighted by staked coins (voting weight), so a validator's
// expected contribution to the nil-block statistics is proportional to its stake; low-stake
// validators are selected rarely and have negligible aggregate influence, while any coalition
// large enough to move the limit materially would necessarily control significant stake.
// Excluding stake weight keeps the control law simple, deterministic, and proposer-agnostic; the
// residual influence of low-stake validators is bounded and acceptable given the overriding
// priority of liveness. This is a conscious, documented tradeoff.

import (
	"errors"
	"math"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/internal/ethapi"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/consensuscontext"
)

// Tunable parameters of the dynamic gas-limit scheme.
const (
	// GAS_LIMIT_WINDOW is the number of recent blocks whose nil-block status influences
	// the gas limit. It also sizes the round-robin status array stored on-chain.
	GAS_LIMIT_WINDOW = 32

	// GAS_TIER_BAND_WIDTH is the width (in blocks) of each round-2 cap band. With a 32-block
	// window this yields 8 bands; the nearest round-2 nil's band sets the upper cap:
	// distance 1-4 -> 1*floor, 5-8 -> 2*floor, 9-12 -> 3*floor, ... 29-32 -> 8*floor.
	GAS_TIER_BAND_WIDTH = 4

	// GAS_PENALTY_UNIT is the base gas unit for nil-block penalties (the standard 21000 basic
	// transaction gas). Penalties are expressed as integer multiples of this unit so the math
	// stays integer-only and deterministic.
	GAS_PENALTY_UNIT = 21000

	// GAS_PENALTY_R1_UNITS / GAS_PENALTY_R2_UNITS are the flat penalty units charged per
	// round-1 / round-2 nil block in the window (multiplied by GAS_PENALTY_UNIT). Round-2 is
	// weighted heavier because it signals network-level stress rather than a single offline
	// proposer.
	GAS_PENALTY_R1_UNITS = 10
	GAS_PENALTY_R2_UNITS = 20

	// GAS_PENALTY_MIN_R1_COUNT is the minimum number of round-1 nils in the window before any
	// round-1 penalty applies. Below this threshold round-1 nils are ignored so that an
	// isolated or occasional offline proposer cannot move the gas limit; reaching this many
	// round-1 nils instead indicates a broader network problem worth throttling for. See the
	// rationale at the top of this file.
	GAS_PENALTY_MIN_R1_COUNT = 5
)

// Round-robin status byte values stored per block in the on-chain status array.
const (
	GasStatusUnwritten = byte(0) // warmup / never written; treated as ok
	GasStatusOk        = byte(1) // VOTE_TYPE_OK
	GasStatusNilRound1 = byte(2) // VOTE_TYPE_NIL, round 1
	GasStatusNilRound2 = byte(3) // VOTE_TYPE_NIL, round >= 2
)

// GAS_NIL_STATUS_KEY is the fixed consensus-context key holding the round-robin status array.
const GAS_NIL_STATUS_KEY = "gas-nil-status"

// ---------------------------------------------------------------------------
// Layer A: pure calculation (deterministic, integer-only, no external coupling)
// ---------------------------------------------------------------------------

// ComputeBlockGasLimit returns the block gas limit for blockNumber given the round-robin
// nil-block status array of the previous blocks. status[k] holds the status of the block
// whose number % GAS_LIMIT_WINDOW == k. It is the sole calculation entry point; it is
// integer-only and deterministic. See the Dynamic TPS rationale at the top of this file.
//
// Two effects combine, and the smaller one wins (the flat penalty can only push gas lower than
// the round-2 cap):
//   - Pass1 -- Round-2 cap: the nearest round-2 nil sets an upper cap by its distance band of
//     width GAS_TIER_BAND_WIDTH -- distance 1-4 -> 1*minGas, 5-8 -> 2*minGas, 9-12 -> 3*minGas,
//     ... 29-32 -> 8*minGas. No round-2 nil in the window -> cap = maxGas.
//   - Pass2 -- Flat count-based penalty: over the window, penalty = r1Pen + r2Pen, where
//     r2Pen = (r2Count-1)*GAS_PENALTY_R2_UNITS*GAS_PENALTY_UNIT when r2Count > 0 (the single
//     nearest round-2 is dropped because Pass1 already accounts for it), and
//     r1Pen = r1Count*GAS_PENALTY_R1_UNITS*GAS_PENALTY_UNIT only when r1Count >=
//     GAS_PENALTY_MIN_R1_COUNT (otherwise 0). penaltyGas = saturating(maxGas - penalty).
//
// Result = clamp(min(penaltyGas, cap), minGas, maxGas).
func ComputeBlockGasLimit(status [GAS_LIMIT_WINDOW]byte, blockNumber, maxGas, minGas uint64) uint64 {
	if maxGas <= minGas {
		return maxGas
	}

	// Pass1: Round-2 tiered cap: the nearest round-2 nil wins (smallest distance -> lowest cap).
	gasCap := maxGas
	for i := uint64(1); i <= GAS_LIMIT_WINDOW && i <= blockNumber; i++ {
		if status[(blockNumber-i)%GAS_LIMIT_WINDOW] == GasStatusNilRound2 {
			band := (i-1)/GAS_TIER_BAND_WIDTH + 1 // 1..8
			gasCap = band * minGas                // 1*F .. 8*F
			break                                 // nearest is the most aggressive band
		}
	}

	// Pass2: flat count-based penalty over the full window (round-1 + round-2).
	var r1Count, r2Count uint64
	for i := uint64(1); i <= GAS_LIMIT_WINDOW && i <= blockNumber; i++ {
		switch status[(blockNumber-i)%GAS_LIMIT_WINDOW] {
		case GasStatusNilRound1:
			r1Count++
		case GasStatusNilRound2:
			r2Count++
		}
	}

	var penalty uint64
	if r2Count > 0 {
		// Skip one round-2 nil: the nearest round-2 is already fully accounted for by the
		// Pass1 distance-banded hard cap (the result is min()-ed against gasCap), so counting
		// it here too would double-penalize the same event. The guard prevents uint64
		// underflow when there are no round-2 nils.
		penalty += (r2Count - 1) * GAS_PENALTY_R2_UNITS * GAS_PENALTY_UNIT
	}
	if r1Count >= GAS_PENALTY_MIN_R1_COUNT {
		// Below the threshold an isolated/occasional offline proposer is ignored so a single
		// validator cannot move the gas limit; at/above it the cluster signals a broader
		// network issue worth throttling for.
		penalty += r1Count * GAS_PENALTY_R1_UNITS * GAS_PENALTY_UNIT
	}

	gas := maxGas
	if penalty >= maxGas {
		gas = minGas // penalty would underflow maxGas; floor directly
	} else {
		gas = maxGas - penalty
	}

	// The flat penalty can only push below the round-2 cap.
	if gasCap < gas {
		gas = gasCap
	}
	if gas < minGas {
		gas = minGas
	}
	if gas > maxGas {
		gas = maxGas
	}
	return gas
}

// gasNilStatusFromVote maps a block's consensus vote type and round to a status byte.
func gasNilStatusFromVote(voteType byte, round byte) byte {
	if voteType == byte(VOTE_TYPE_OK) {
		return GasStatusOk
	}
	// VOTE_TYPE_NIL (or anything non-OK) is a nil block.
	if round <= 1 {
		return GasStatusNilRound1
	}
	return GasStatusNilRound2
}

// ---------------------------------------------------------------------------
// Layer B: thin engine glue (state I/O + interface method), calling into Layer A
// ---------------------------------------------------------------------------

// getGasNilStatusArray reads the round-robin nil-block status array from the consensus
// context against the in-progress state. A never-set context reads back as all zeros.
func (c *ProofOfStake) getGasNilStatusArray(state *state.StateDB, header *types.Header) ([GAS_LIMIT_WINDOW]byte, error) {
	var arr [GAS_LIMIT_WINDOW]byte

	method := consensuscontext.GET_CONTEXT_METHOD
	abiData, err := consensuscontext.GetConsensusContract_ABI()
	if err != nil {
		log.Error("getGasNilStatusArray abi error", "err", err)
		return arr, err
	}
	contractAddress := consensuscontext.CONSENSUS_CONTEXT_CONTRACT_ADDRESS

	data, err := abiData.Pack(method, GAS_NIL_STATUS_KEY)
	if err != nil {
		log.Error("Unable to pack getGasNilStatusArray", "error", err)
		return arr, err
	}

	msgData := (hexutil.Bytes)(data)
	var from common.Address
	from.CopyFrom(ZERO_ADDRESS)
	args := ethapi.TransactionArgs{
		From: &from,
		To:   &contractAddress,
		Data: &msgData,
	}

	msg, err := args.ToMessage(math.MaxUint64)
	if err != nil {
		return arr, err
	}

	// ExecuteNoGas runs a full EVM state transition (ApplyMessage), which mutates the
	// passed state (e.g. it bumps the ZERO_ADDRESS sender nonce). This is a pure read of
	// the status array, and GetGasLimit is called a different number of times on the miner
	// path (worker + ProcessTransactions + Finalize) than on the import/verify path. Running
	// it against the live block state would therefore produce a non-deterministic state root
	// across nodes. Execute against a copy so the read has no effect on the canonical state.
	result, err := c.blockchain.ExecuteNoGas(msg, state.Copy(), header)
	if err != nil {
		return arr, err
	}
	if len(result) == 0 {
		return arr, errors.New("getGasNilStatusArray result is 0")
	}

	if err := abiData.UnpackIntoInterface(&arr, method, result); err != nil {
		log.Debug("UnpackIntoInterface getGasNilStatusArray", "err", err)
		return arr, err
	}

	return arr, nil
}

// writeGasNilStatus records the given block's nil-block status into the round-robin array,
// overwriting the slot of the block GAS_LIMIT_WINDOW positions earlier. Called from Finalize.
func (c *ProofOfStake) writeGasNilStatus(state *state.StateDB, header *types.Header, blockConsensusData *BlockConsensusData) error {
	arr, err := c.getGasNilStatusArray(state, header)
	if err != nil {
		return err
	}

	blockNumber := header.Number.Uint64()
	arr[blockNumber%GAS_LIMIT_WINDOW] = gasNilStatusFromVote(byte(blockConsensusData.VoteType), blockConsensusData.Round)

	var context [32]byte
	copy(context[:], arr[:])
	return c.SetConsensusContext(GAS_NIL_STATUS_KEY, context, state, header)
}

// GetGasLimit implements consensus.Engine. Below GasV2StartBlock it returns the legacy
// fixed gas limit; from GasV2StartBlock onward it computes the dynamic value from the
// round-robin nil-block status stored in the (parent) state.
func (c *ProofOfStake) GetGasLimit(header *types.Header, statedb *state.StateDB) (uint64, error) {
	number := header.Number.Uint64()
	if number < defaults.DefaultConfig.PosConfig.GasV2StartBlock {
		return defaults.GetGasLimit(number), nil
	}

	arr, err := c.getGasNilStatusArray(statedb, header)
	if err != nil {
		return 0, err
	}

	return ComputeBlockGasLimit(arr, number, defaults.GetMaxGasLimit(number), defaults.MIN_DYNAMIC_GAS_LIMIT), nil
}
