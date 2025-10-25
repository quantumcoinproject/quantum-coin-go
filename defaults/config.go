package defaults

import (
	"errors"
	"math/big"

	"github.com/quantumcoinproject/quantum-coin-go/params"
)

var DEFAULT_PRICE = int64(47619047619047600)
var cryptoBreakglassBlock uint64 = 0
var signingMode byte = 1 //crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID)

func GetGasLimit(blockNumber uint64) uint64 {
	if blockNumber < DefaultConfig.GasPriceStartBlock {
		return DefaultConfig.DefaultGasLimit
	} else {
		return DefaultConfig.DefaultGasLimit
	}
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

	VALIDATOR_NIL_BLOCK_START_BLOCK:      uint64(421888) + 1,
	BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK: uint64(421888) + 1 + 16,

	CONTEXT_BASED_START_BLOCK:     uint64(536000),
	CONTEXT_BASED_BLOCK_THRESHOLD: uint64(64000),
	BLOCK_TIME_ORIG_START_BLOCK:   uint64(536000 + 1),
	PACKET_PROTOCOL_START_BLOCK:   uint64(536000 + 1 + 32),

	PROPOSAL_TIME_HASH_START_BLOCK:        uint64(1507600),
	BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK: uint64(1597600),

	//Note: both of the below should add upto 100
	TxnFeeRewardsPercentage: int64(50),

	SixtyVoteStartBlock: uint64(1386825),

	SlashV2StartBlock:               uint64(2082171),
	OfflineValidatorDeferStartBlock: 2082171 + 10,

	SixtySevenVoteStartBlock: uint64(2082171 + 10 + 10),

	OfflineValidatorV4StartBlock: 3600030,

	SigAlgSwitchBlock: 3600030 + 2,

	MinOfflineProposerBlockDelay: 3600,
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

	VALIDATOR_NIL_BLOCK_START_BLOCK:      uint64(8) + 1,
	BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK: uint64(8+1) + 16,

	CONTEXT_BASED_START_BLOCK:     uint64(32),
	CONTEXT_BASED_BLOCK_THRESHOLD: uint64(4),
	BLOCK_TIME_ORIG_START_BLOCK:   uint64(32 + 1),
	PACKET_PROTOCOL_START_BLOCK:   uint64(32 + 1 + 32),

	PROPOSAL_TIME_HASH_START_BLOCK:        uint64(64),
	BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK: uint64(64),

	//Note: both of the below should add upto 100
	TxnFeeRewardsPercentage: int64(50),

	SixtyVoteStartBlock: uint64(64),

	SlashV2StartBlock:               uint64(90),
	OfflineValidatorDeferStartBlock: 90 + 10,

	SixtySevenVoteStartBlock: uint64(90 + 10 + 10),

	OfflineValidatorV4StartBlock: 90 + 10 + 10 + 10,

	SigAlgSwitchBlock: 90 + 10 + 10 + 10 + 2,

	MinOfflineProposerBlockDelay: 3600,
}

var MainnetConfig = &Config{
	PosConfig:               &mainnetPosConfig,
	DeepCheckStartBlock:     uint64(4000000),
	GasPriceStartBlock:      uint64(4000001),
	DefaultGasLimit:         300000000,
	ValidateSigPubStartTime: int64(1767225600000), //Thursday, January 1, 2026 12:00:00 AM
	TxnStartAllowedTime:     int64(1713052800),    //April 14th, 2024
	ConversionTxnLastTime:   int64(1744675199),    //April 14th, 2025, 11:59:59 PM UTC
	KemSwitchTime:           int64(1767225600000), //Thursday, January 1, 2026 12:00:00 AM
}

var DevnetConfig = &Config{
	PosConfig:               &devnetPosConfig,
	DeepCheckStartBlock:     uint64(256),
	GasPriceStartBlock:      uint64(257),
	DefaultGasLimit:         300000000,
	ValidateSigPubStartTime: int64(1767225600000), //Thursday, January 1, 2026 12:00:00 AM
	TxnStartAllowedTime:     int64(1713052800),    //April 14th, 2024
	ConversionTxnLastTime:   int64(1744675199),    //April 14th, 2025, 11:59:59 PM UTC
	KemSwitchTime:           int64(1767225600000), //Thursday, January 1, 2026 12:00:00 AM
}

var DefaultConfig = MainnetConfig
