package proofofstake

import (
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"math/big"
)

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
}

var MainnetConfig = ProofOfStakeConfig{
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
}

var DevnetConfig = ProofOfStakeConfig{
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
}

var DefaultConfig = MainnetConfig
