package defaults

import (
	"errors"
	"math/big"
	"os"

	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

const BASIC_TXN_GAS = uint64(21000)

// MIN_DYNAMIC_GAS_LIMIT is the floor block gas limit (~100 basic txns) used by the
// dynamic gas-limit scheme that activates at GasV2StartBlock.
const MIN_DYNAMIC_GAS_LIMIT = uint64(2100000)

// BreakglassDefaultGasLimit is the reduced maximum block gas limit enforced while
// breakglass mode is active.
const BreakglassDefaultGasLimit = uint64(30000000)

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
	return int(DefaultConfig.DefaultGasLimit / BASIC_TXN_GAS)
}

// GetMaxGasLimit returns the maximum allowed block gas limit for the dynamic gas-limit
// scheme: the normal default, or a reduced cap when breakglass mode is active.
func GetMaxGasLimit(blockNumber uint64) uint64 {
	if IsCryptoBreakglassMode(blockNumber) {
		return BreakglassDefaultGasLimit
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
}

type Config struct {
	PosConfig               *ProofOfStakeConfig
	DeepCheckStartBlock     uint64
	GasPriceStartBlock      uint64
	DefaultGasLimit         uint64
	ValidateSigPubStartTime int64
	TxnStartAllowedTime     int64
	ConversionTxnLastTime   int64
	KemSwitchTime           int64
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
	PACKET_PROTOCOL_START_BLOCK:   uint64(65),

	PROPOSAL_TIME_HASH_START_BLOCK:        uint64(64),
	BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK: uint64(64),

	//Note: both of the below should add upto 100
	TxnFeeRewardsPercentage: int64(50),

	SixtyVoteStartBlock: uint64(64),

	SlashV2StartBlock:               uint64(90),
	OfflineValidatorDeferStartBlock: 100,

	SixtySevenVoteStartBlock: uint64(110),

	OfflineValidatorV4StartBlock: 120,

	SigAlgSwitchBlock: 122,

	MinOfflineProposerBlockDelay: 3600,

	DynamicFeeTxStartBlock:     132,
	SkipProposerStartBlock:     0,
	SkipProposerEndBlock:       0,
	ExtraDataV3StartBlock:      142,
	Normalizationv2StartBlock:  152,
	ValidatorCountV2StartBlock: 162,
	GasV2StartBlock:            172,
}

var MainnetConfig = &Config{
	PosConfig:               &mainnetPosConfig,
	DeepCheckStartBlock:     uint64(3426264),
	GasPriceStartBlock:      uint64(3426265),
	DefaultGasLimit:         300000000,
	ValidateSigPubStartTime: int64(1769904000), //Feb 1, 2026 12:00:00 AM
	TxnStartAllowedTime:     int64(1713052800), //April 14th, 2024
	ConversionTxnLastTime:   int64(1744675199), //April 14th, 2025, 11:59:59 PM UTC
	KemSwitchTime:           int64(1799229600), //Jan 06, 2027 10:00:00 AM UTC
}

var DevnetConfig = &Config{
	PosConfig:               &devnetPosConfig,
	DeepCheckStartBlock:     uint64(130),
	GasPriceStartBlock:      uint64(131),
	DefaultGasLimit:         300000000,
	ValidateSigPubStartTime: int64(1769904000), //Feb 1, 2026 12:00:00 AM
	TxnStartAllowedTime:     int64(1713052800), //April 14th, 2024
	ConversionTxnLastTime:   int64(1744675199), //April 14th, 2025, 11:59:59 PM UTC
	KemSwitchTime:           int64(1713052800), //April 14th, 2025, 11:59:59 PM UTC
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
