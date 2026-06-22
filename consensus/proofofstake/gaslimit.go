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
// ... -> 8F) as the event ages out of the observation window.
//
// Signal -- round-1 nil blocks (proposer-availability indicator): a round-1 nil block occurs when
// the selected proposer is offline or unresponsive. Isolated occurrences are benign; a high
// density within the window suggests systemic overload or network degradation, so the scheme
// applies a superlinear penalty (drop proportional to nilScore^2) -- sparse failures barely
// register while correlated clusters reduce throughput rapidly.
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

	// GAS_RAMP_R2_WEIGHT is how much a round-2 nil counts relative to a round-1 nil in the
	// superlinear ramp's weighted nil score.
	GAS_RAMP_R2_WEIGHT = 2

	// GAS_RAMP_FULL_DROP_SCORE is the weighted nil score at which the ramp reaches the floor
	// (full drop). Reaching the floor at this many round-1 nils makes the ramp aggressive.
	GAS_RAMP_FULL_DROP_SCORE = 16

	// GAS_DROP_SCALE is the fixed-point resolution of the drop fraction. The drop is a value
	// in [0, 1], but consensus math must be integer-only and deterministic (no floats), so it
	// is represented in permille (parts per thousand); GAS_DROP_SCALE == full drop.
	GAS_DROP_SCALE = 1000
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
// Two effects combine, and the smaller one wins (round-1 nils can only push gas lower than
// the round-2 cap):
//   - Round-2 cap: the nearest round-2 nil sets an upper cap by its distance band of width
//     GAS_TIER_BAND_WIDTH -- distance 1-4 -> 1*minGas, 5-8 -> 2*minGas, 9-12 -> 3*minGas,
//     ... 29-32 -> 8*minGas. No round-2 nil in the window -> cap = maxGas.
//   - Superlinear ramp: nilScore = r1Count + GAS_RAMP_R2_WEIGHT*r2Count over the window,
//     dropPermille = min(GAS_DROP_SCALE, nilScore^2 * GAS_DROP_SCALE / GAS_RAMP_FULL_DROP_SCORE^2),
//     rampGas = maxGas - (maxGas-minGas)*dropPermille/GAS_DROP_SCALE.
//
// Result = clamp(min(rampGas, cap), minGas, maxGas).
func ComputeBlockGasLimit(status [GAS_LIMIT_WINDOW]byte, blockNumber, maxGas, minGas uint64) uint64 {
	if maxGas <= minGas {
		return maxGas
	}

	// Round-2 tiered cap: the nearest round-2 nil wins (smallest distance -> lowest cap).
	cap := maxGas
	for i := uint64(1); i <= GAS_LIMIT_WINDOW && i <= blockNumber; i++ {
		if status[(blockNumber-i)%GAS_LIMIT_WINDOW] == GasStatusNilRound2 {
			band := (i-1)/GAS_TIER_BAND_WIDTH + 1 // 1..8
			cap = band * minGas                   // 1*F .. 8*F
			break                                 // nearest is the most aggressive band
		}
	}

	// Superlinear ramp over the full window (round-1 + weighted round-2).
	var r1Count, r2Count uint64
	for i := uint64(1); i <= GAS_LIMIT_WINDOW && i <= blockNumber; i++ {
		switch status[(blockNumber-i)%GAS_LIMIT_WINDOW] {
		case GasStatusNilRound1:
			r1Count++
		case GasStatusNilRound2:
			r2Count++
		}
	}

	nilScore := r1Count + GAS_RAMP_R2_WEIGHT*r2Count
	dropPermille := nilScore * nilScore * GAS_DROP_SCALE / (GAS_RAMP_FULL_DROP_SCORE * GAS_RAMP_FULL_DROP_SCORE)
	if dropPermille > GAS_DROP_SCALE {
		dropPermille = GAS_DROP_SCALE
	}
	gas := maxGas - (maxGas-minGas)*dropPermille/GAS_DROP_SCALE

	// The round-1/round-2 ramp can only push below the round-2 cap.
	if cap < gas {
		gas = cap
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
