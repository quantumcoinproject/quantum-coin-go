package defaults

import (
	"errors"
	"math"
	"math/big"
	"os"

	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

const BASIC_TXN_GAS = uint64(21000)

// NotScheduled marks a fork activation height that has not been agreed yet. A gate
// set to this value never activates, so the fix is compiled in but dormant until a
// real height is chosen.
const NotScheduled = uint64(math.MaxUint64)

// MIN_DYNAMIC_GAS_LIMIT is the floor block gas limit (~100 basic txns) used by the
// dynamic gas-limit scheme that activates at GasV2StartBlock.
const MIN_DYNAMIC_GAS_LIMIT = uint64(2100000)

var DEFAULT_PRICE = int64(47619047619047600)
var SigningContextLevel1Multiplier = int64(20)
var SigningContextLevel2Multiplier = int64(30)
var cryptoBreakglassBlock uint64 = 0
var signingMode byte = 1 //crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID)
var okConfig bool = LoadDefaultConfig()

func GetGasLimit(blockNumber uint64) uint64 {
	if blockNumber < DefaultConfig.GasPriceStartBlock {
		return DefaultConfig.DefaultGasLimit
	} else {
		return DefaultConfig.DefaultGasLimit
	}
}

func GetMaxTransactionsForBlock(blockNumber uint64) int {
	if IsGasV3(blockNumber) {
		return int(DefaultConfig.DefaultGasLimitV2 / BASIC_TXN_GAS)
	}
	return int(DefaultConfig.DefaultGasLimit / BASIC_TXN_GAS)
}

// GetMaxGasLimit returns the maximum allowed block gas limit for the dynamic gas-limit
// scheme: the normal default, or a reduced cap when breakglass mode is active. Both
// ceilings drop once GasV3StartBlock is active.
func GetMaxGasLimit(blockNumber uint64) uint64 {
	if IsCryptoBreakglassMode(blockNumber) {
		if IsGasV3(blockNumber) {
			return DefaultConfig.BreakglassDefaultGasLimitV2
		}
		return DefaultConfig.BreakglassDefaultGasLimit
	}
	if IsGasV3(blockNumber) {
		return DefaultConfig.DefaultGasLimitV2
	}
	return DefaultConfig.DefaultGasLimit
}

func SetCryptoBreakGlassBlock(blockNumber uint64) error {
	if cryptoBreakglassBlock > 0 && blockNumber != 0 {
		return errors.New("SetCryptoBreakGlassBlock already set")
	}
	cryptoBreakglassBlock = blockNumber
	return nil
}

func IsCryptoBreakglassMode(blockNumber uint64) bool {
	return cryptoBreakglassBlock != 0 && blockNumber >= cryptoBreakglassBlock
}

// IsGasTipActive reports whether gas tip / priority fee support is active at the
// given block number (from GasTipStartBlock onward).
func IsGasTipActive(blockNumber uint64) bool {
	return blockNumber >= DefaultConfig.PosConfig.GasTipStartBlock
}

// IsConsensusMalleabilityV1 reports whether the consensus/header malleability
// hardening (see ConsensusMalleabilityV1StartBlock) is active at the given block
// number. All consensus-affecting malleability checks are gated behind this so they
// only apply from a finalized activation height and never retroactively reject
// historical blocks.
func IsConsensusMalleabilityV1(blockNumber uint64) bool {
	return blockNumber >= DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock
}

// IsUpstreamConsensusFixesV1 reports whether the upstream-derived consensus fix
// bundle (see UpstreamConsensusFixesV1StartBlock) is active at the given block
// number. Every fix behind this gate changes the state transition, so it must
// only apply from a finalized activation height and never retroactively.
func IsUpstreamConsensusFixesV1(blockNumber uint64) bool {
	return blockNumber >= DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock
}

// IsUpstreamConsensusFixesV1Big is the *big.Int form for EVM call sites, which
// carry the block number as a *big.Int. A nil block number is treated as
// pre-activation so that callers without block context never activate the fork.
func IsUpstreamConsensusFixesV1Big(blockNumber *big.Int) bool {
	if blockNumber == nil {
		return false
	}
	if !blockNumber.IsUint64() {
		// Beyond uint64 range: past every schedulable height, but still inactive
		// when the fork is unscheduled.
		return DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock != NotScheduled
	}
	return IsUpstreamConsensusFixesV1(blockNumber.Uint64())
}

// IsGasV3 reports whether the reduced gas-limit ceilings (see GasV3StartBlock) are
// active at the given block number. Consensus-affecting: it changes the valid
// header.GasLimit range, the dynamic gas-limit computation, and the
// max-transactions-per-block bound, so it only applies from a scheduled activation
// height and never retroactively.
func IsGasV3(blockNumber uint64) bool {
	return blockNumber >= DefaultConfig.PosConfig.GasV3StartBlock
}

// IsBlockTimeBindingV1 reports whether header.Time must equal the value derived
// from the consensus-agreed BlockTime (see BlockTimeBindingV1StartBlock). This is
// consensus-affecting, so it only applies from a finalized activation height and
// never retroactively.
func IsBlockTimeBindingV1(blockNumber uint64) bool {
	return blockNumber >= DefaultConfig.PosConfig.BlockTimeBindingV1StartBlock
}

func IsSigAlgSwitchMode(blockNumber uint64) bool {
	if blockNumber >= DefaultConfig.PosConfig.SigAlgSwitchBlock {
		if cryptoBreakglassBlock != 0 && blockNumber >= cryptoBreakglassBlock {
			return false
		}
		return true
	}
	return false
}

func SetCryptoSigningMode(signMode byte) {
	signingMode = signMode
}

func GetSigningMode() byte {
	return signingMode
}

type ProofOfStakeConfig struct {
	SLASH_AMOUNT    *big.Int
	SLASH_AMOUNT_V2 *big.Int

	RewardStartBlockNumber uint64
	SlashStartBlockNumber  uint64

	FULL_SIGN_PROPOSAL_CUTOFF_BLOCK     uint64
	FULL_SIGN_PROPOSAL_FREQUENCY_BLOCKS uint64

	STAKING_CONTRACT_V2_CUTOFF_BLOCK  uint64
	CONSENSUS_CONTEXT_START_BLOCK     uint64
	CONSENSUS_CONTEXT_MAX_BLOCK_COUNT uint64

	SystemContractV3StartBlock uint64

	// GranularBlockTimeStartBlock activates granular proposal-time alignment
	// (BlockTimeGranularity seconds, was 60s). Consensus-affecting: GetProposalTime,
	// VerifyBlockProposalTime, and VerifyBlockProposalTimeConsensus all change
	// behavior at this height.
	GranularBlockTimeStartBlock uint64

	// BlockTimeGranularity is the proposal-time alignment in seconds once
	// GranularBlockTimeStartBlock is active (before that, 60s applies).
	// Consensus-affecting: must only change together with a scheduled fork.
	BlockTimeGranularity int64

	VALIDATOR_NIL_BLOCK_START_BLOCK      uint64
	BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK uint64

	CONTEXT_BASED_START_BLOCK     uint64
	CONTEXT_BASED_BLOCK_THRESHOLD uint64
	BLOCK_TIME_ORIG_START_BLOCK   uint64
	PACKET_PROTOCOL_START_BLOCK   uint64

	PROPOSAL_TIME_HASH_START_BLOCK        uint64
	BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK uint64

	//Note: both of the below should add upto 100
	TxnFeeRewardsPercentage int64

	SixtyVoteStartBlock uint64

	SlashV2StartBlock               uint64
	OfflineValidatorDeferStartBlock uint64

	SixtySevenVoteStartBlock uint64

	OfflineValidatorV4StartBlock uint64

	SigAlgSwitchBlock uint64

	MinOfflineProposerBlockDelay uint64

	DynamicFeeTxStartBlock     uint64
	SkipProposerStartBlock     uint64
	SkipProposerEndBlock       uint64
	ExtraDataV3StartBlock      uint64
	Normalizationv2StartBlock  uint64
	ValidatorCountV2StartBlock uint64
	GasV2StartBlock            uint64

	// GasTipStartBlock activates gas tip / priority fee support. From this block,
	// transactions are selected by effective tip subject to a 50/50 basic-vs-general
	// gas split, the tip is paid to the block proposer, and ProcessTransactions
	// enforces the split via two-pass execution. It is GasV2StartBlock + 10.
	GasTipStartBlock uint64

	// ConsensusMalleabilityV1StartBlock activates the consensus/header malleability
	// hardening: preservation and cross-check of the deciding-round proposal packet's
	// BlockTime/Txns against BlockConsensusData, strict consensus-protocol-version
	// matching (== current version), header.Time monotonicity vs parent, an InitTime
	// upper bound, and live-handler equivocation detection. Every one of these checks
	// is consensus-affecting and can reject blocks that previously verified, so this
	// MUST be replayed against full chain history before being scheduled. Mainnet is
	// intentionally set to "never" (max uint64) until a concrete activation height has
	// been finalized; only then should this be lowered to that height.
	ConsensusMalleabilityV1StartBlock uint64

	// UpstreamConsensusFixesV1StartBlock activates the consensus fixes ported from
	// upstream go-ethereum (see docs/upstream-bugfix-audit-2026-08.md). It gates:
	//
	//   - CVE-2021-39137: the identity precompile (0x04) returns its input slice by
	//     reference, so a CALL whose return region overlaps caller memory can
	//     retroactively mutate already-consumed return data. Upstream 1d9957319
	//     (v1.10.8); split Ethereum mainnet at block 13107518.
	//   - EIP-3607: reject transactions sent from an account that has code.
	//     Upstream 0658712f6 + d02c60536 (v1.10.9).
	//   - EIP-2681: reject a transaction at nonce 2^64-1, and fail CREATE/CREATE2
	//     when the creator's nonce would wrap. Upstream f32feeb26.
	//
	// Every one of these changes the state transition or block validity: blocks that
	// verify today can become invalid, and EVM output changes for contract-reachable
	// input. Activating retroactively would fork the chain, so mainnet is set to
	// NotScheduled until a concrete activation height has been agreed; only then
	// should this be lowered to that height.
	UpstreamConsensusFixesV1StartBlock uint64

	BlockTimeBindingV1StartBlock uint64

	// GasV3StartBlock reduces the maximum block gas limit from DefaultGasLimit (300M)
	// to DefaultGasLimitV2 (45M), and the breakglass maximum from
	// BreakglassDefaultGasLimit (30M) to BreakglassDefaultGasLimitV2 (9M). The dynamic
	// gas-limit scheme from GasV2StartBlock keeps operating; only its ceilings change.
	// The max-transactions-per-block bound drops proportionally. Consensus-affecting:
	// must only activate at a scheduled height, never retroactively.
	GasV3StartBlock uint64
}

type Config struct {
	PosConfig           *ProofOfStakeConfig
	DeepCheckStartBlock uint64
	GasPriceStartBlock  uint64
	DefaultGasLimit     uint64
	// DefaultGasLimitV2 is the reduced maximum block gas limit enforced once
	// GasV3StartBlock is active.
	DefaultGasLimitV2 uint64
	// BreakglassDefaultGasLimit is the reduced maximum block gas limit enforced while
	// breakglass mode is active.
	BreakglassDefaultGasLimit uint64
	// BreakglassDefaultGasLimitV2 is the maximum block gas limit enforced while
	// breakglass mode is active once GasV3StartBlock is active.
	BreakglassDefaultGasLimitV2 uint64
	ValidateSigPubStartTime     int64
	TxnStartAllowedTime         int64
	ConversionTxnLastTime       int64
	KemSwitchTime               int64
}

var mainnetPosConfig = ProofOfStakeConfig{
	SLASH_AMOUNT: params.EtherToWei(big.NewInt(10)),

	SLASH_AMOUNT_V2: params.EtherToWei(big.NewInt(100)),

	RewardStartBlockNumber: uint64(277204),
	SlashStartBlockNumber:  uint64(1497600),

	FULL_SIGN_PROPOSAL_CUTOFF_BLOCK:     uint64(421888),
	FULL_SIGN_PROPOSAL_FREQUENCY_BLOCKS: uint64(4096),

	STAKING_CONTRACT_V2_CUTOFF_BLOCK:  uint64(421888),
	CONSENSUS_CONTEXT_START_BLOCK:     uint64(421888),
	CONSENSUS_CONTEXT_MAX_BLOCK_COUNT: uint64(512000),

	VALIDATOR_NIL_BLOCK_START_BLOCK:      uint64(421889),
	BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK: uint64(421905),

	CONTEXT_BASED_START_BLOCK:     uint64(536000),
	CONTEXT_BASED_BLOCK_THRESHOLD: uint64(64000),
	BLOCK_TIME_ORIG_START_BLOCK:   uint64(536001),
	PACKET_PROTOCOL_START_BLOCK:   uint64(536033),

	PROPOSAL_TIME_HASH_START_BLOCK:        uint64(1507600),
	BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK: uint64(1597600),

	//Note: both of the below should add upto 100
	TxnFeeRewardsPercentage: int64(50),

	SixtyVoteStartBlock: uint64(1386825),

	SlashV2StartBlock:               uint64(2082171),
	OfflineValidatorDeferStartBlock: 2082181,

	SixtySevenVoteStartBlock: uint64(2082191),

	OfflineValidatorV4StartBlock: 3426261,

	SigAlgSwitchBlock: 3426263,

	MinOfflineProposerBlockDelay: 3600,

	DynamicFeeTxStartBlock:     3426273,
	SkipProposerStartBlock:     3426273,
	SkipProposerEndBlock:       3790264,
	ExtraDataV3StartBlock:      5319208,
	Normalizationv2StartBlock:  5319218,
	ValidatorCountV2StartBlock: 5319228,
	GasV2StartBlock:            5319238,
	GasTipStartBlock:           5319248,

	SystemContractV3StartBlock: 5319258,

	GranularBlockTimeStartBlock: 5319268,
	BlockTimeGranularity:        6,

	ConsensusMalleabilityV1StartBlock: 0,

	UpstreamConsensusFixesV1StartBlock: 5319269,

	BlockTimeBindingV1StartBlock: 5319270,

	GasV3StartBlock: 5319280,
}

var devnetPosConfig = ProofOfStakeConfig{
	SLASH_AMOUNT: params.EtherToWei(big.NewInt(10)),

	SLASH_AMOUNT_V2: params.EtherToWei(big.NewInt(100)),

	RewardStartBlockNumber: uint64(2),
	SlashStartBlockNumber:  uint64(4),

	FULL_SIGN_PROPOSAL_CUTOFF_BLOCK:     uint64(8),
	FULL_SIGN_PROPOSAL_FREQUENCY_BLOCKS: uint64(32),

	STAKING_CONTRACT_V2_CUTOFF_BLOCK:  uint64(8),
	CONSENSUS_CONTEXT_START_BLOCK:     uint64(8),
	CONSENSUS_CONTEXT_MAX_BLOCK_COUNT: uint64(128),

	VALIDATOR_NIL_BLOCK_START_BLOCK:      uint64(9),
	BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK: uint64(25),

	CONTEXT_BASED_START_BLOCK:     uint64(32),
	CONTEXT_BASED_BLOCK_THRESHOLD: uint64(4),
	BLOCK_TIME_ORIG_START_BLOCK:   uint64(33),
	PACKET_PROTOCOL_START_BLOCK:   uint64(36),

	PROPOSAL_TIME_HASH_START_BLOCK:        uint64(38),
	BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK: uint64(38),

	//Note: both of the below should add upto 100
	TxnFeeRewardsPercentage: int64(40),

	SixtyVoteStartBlock: uint64(42),

	SlashV2StartBlock:               uint64(44),
	OfflineValidatorDeferStartBlock: 46,

	SixtySevenVoteStartBlock: uint64(48),

	OfflineValidatorV4StartBlock: 50,

	SigAlgSwitchBlock: 52,

	MinOfflineProposerBlockDelay: 3600,

	DynamicFeeTxStartBlock:     58,
	SkipProposerStartBlock:     0,
	SkipProposerEndBlock:       0,
	ExtraDataV3StartBlock:      64,
	Normalizationv2StartBlock:  66,
	ValidatorCountV2StartBlock: 68,
	GasV2StartBlock:            70,
	GasTipStartBlock:           72,

	SystemContractV3StartBlock: 74,

	// Devnet uses 1-second granularity post-fork so header times match the wall
	// clock exactly. Devnet chains must be reset when these change.
	GranularBlockTimeStartBlock: 76,
	BlockTimeGranularity:        1,

	ConsensusMalleabilityV1StartBlock: 0,

	// Devnet activates at the next height in the sequence so the fork path is
	// exercised end to end. Devnet chains must be reset when this changes.
	UpstreamConsensusFixesV1StartBlock: 78,

	// Devnet activates the header.Time binding so the fork path is exercised end to
	// end. Devnet chains must be reset when this changes.
	BlockTimeBindingV1StartBlock: 80,

	// Devnet activates the reduced gas-limit ceilings at the next height in the
	// sequence so the fork path is exercised end to end. Devnet chains must be reset
	// when this changes.
	GasV3StartBlock: 82,
}

var MainnetConfig = &Config{
	PosConfig:                   &mainnetPosConfig,
	DeepCheckStartBlock:         uint64(3426264),
	GasPriceStartBlock:          uint64(3426265),
	DefaultGasLimit:             300000000,
	DefaultGasLimitV2:           45000000,
	BreakglassDefaultGasLimit:   30000000,
	BreakglassDefaultGasLimitV2: 9000000,
	ValidateSigPubStartTime:     int64(1769904000), //Feb 1, 2026 12:00:00 AM
	TxnStartAllowedTime:         int64(1713052800), //April 14th, 2024
	ConversionTxnLastTime:       int64(1744675199), //April 14th, 2025, 11:59:59 PM UTC
	KemSwitchTime:               int64(1799229600), //Jan 06, 2027 10:00:00 AM UTC
}

var DevnetConfig = &Config{
	PosConfig: &devnetPosConfig,
	// DeepCheckStartBlock must stay <= PosConfig.ExtraDataV3StartBlock (and >= SigAlgSwitchBlock),
	// matching mainnet ordering, so the Extra-data produce gate (Finalize, >= DeepCheckStartBlock)
	// and verify gate (VerifyExtraData, >= ExtraDataV3StartBlock) stay aligned. Otherwise blocks in
	// [ExtraDataV3StartBlock, DeepCheckStartBlock) are sealed with empty Extra but verified as v3,
	// producing "DecodeBlockExtraData v3 error=EOF" BAD BLOCKs.
	DeepCheckStartBlock:         uint64(54),
	GasPriceStartBlock:          uint64(56),
	DefaultGasLimit:             300000000,
	DefaultGasLimitV2:           45000000,
	BreakglassDefaultGasLimit:   30000000,
	BreakglassDefaultGasLimitV2: 9000000,
	ValidateSigPubStartTime:     int64(1769904000), //Feb 1, 2026 12:00:00 AM
	TxnStartAllowedTime:         int64(1713052800), //April 14th, 2024
	ConversionTxnLastTime:       int64(1744675199), //April 14th, 2025, 11:59:59 PM UTC
	KemSwitchTime:               int64(1713052800), //April 14th, 2025, 11:59:59 PM UTC
}

var DefaultConfig = MainnetConfig

func LoadDefaultConfig() bool {
	config := os.Getenv("Q_DEFAULT_CONFIG")
	if config == "1" {
		DefaultConfig = DevnetConfig
		log.Warn("Setting default config to DevnetConfig. Q_DEFAULT_CONFIG is set.")
	} else {
		log.Info("Setting default config to MainnetConfig")
		DefaultConfig = MainnetConfig
	}
	return true
}
