package cachemanager

import (
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/proofofstake"
)

const BlockDetailsKey = "block-%d" //%d is block number
const LastBlockKey = "last-block"

type VoteType string

const (
	OK_VOTE  VoteType = "OK"
	NIL_VOTE VoteType = "NIL"
)

type BlockConsensusDetails struct {
	BlockProposer        string     `json:"blockProposer"`
	VoteType             VoteType   `json:"voteType"`
	ProposalHash         string     `json:"proposalHash"`
	PrecommitHash        string     `json:"precommitHash"`
	SlashedValidators    []Slashing `json:"slashedValidators,omitempty"`
	Rounds               byte
	SelectedTransactions []string `json:"selectedTransactions,omitempty"` //this will be a super-set of transactions that actually got executed
	BlockTime            uint64   `json:"blockTime"`                      //as voted

	BlockProposerRewards     string `json:"blockProposerRewards,omitempty"`     //total rewards, blockRewards + txnFeeRewards
	BaseBlockProposerRewards string `json:"baseBlockProposerRewards,omitempty"` //block rewards excluding txn free rewards
	TxnFeeRewards            string `json:"txnFeeRewards,omitempty"`
	BurntTxnFee              string `json:"burntTxnFee,omitempty"`
	SlashAmount              string `json:"slashAmount,omitempty"` //total slash amount
}

type Block struct {
	Hash             string                `json:"hash,omitempty"`
	ParentHash       string                `json:"parentHash,omitempty"`
	StateRoot        string                `json:"stateRoot,omitempty"`
	TransactionsRoot string                `json:"transactionsRoot,omitempty"`
	ReceiptsRoot     string                `json:"receiptsRoot,omitempty"`
	Number           uint64                `json:"number,omitempty"`
	GasLimit         string                `json:"gasLimit,omitempty"`
	GasUsed          string                `json:"gasUsed,omitempty"`
	Time             uint64                `json:"timestamp,omitempty"` //will be different from ConsensusDetails blockTime for NIL blocks
	MixDigest        string                `json:"mixHash,omitempty"`
	TransactionCount uint                  `json:"transactionCount,omitempty"`
	ConsensusDetails BlockConsensusDetails `json:"consensusDetails,omitempty"`
}

func fromPrimordialBlockData(blockData *PrimordialBlockData) *Block {
	var block Block
	block.Hash = blockData.Block.Hash
	block.ParentHash = blockData.Block.ParentHash
	block.StateRoot = blockData.Block.StateRoot
	block.TransactionsRoot = blockData.Block.TransactionsRoot
	block.ReceiptsRoot = blockData.Block.ReceiptsRoot
	block.Number = blockData.Block.Number.Uint64()
	block.GasLimit = common.UintToHex(blockData.Block.GasLimit)
	block.GasUsed = common.UintToHex(blockData.Block.GasUsed)
	block.Time = blockData.Block.Time
	block.MixDigest = blockData.Block.MixDigest
	block.TransactionCount = blockData.Block.TransactionsCount
	block.StateRoot = blockData.Block.StateRoot

	block.ConsensusDetails.BlockProposer = blockData.ConsensusData.Data.BlockProposer.HexLower()
	if blockData.ConsensusData.Data.VoteType == proofofstake.VOTE_TYPE_OK {
		block.ConsensusDetails.VoteType = OK_VOTE
	} else {
		block.ConsensusDetails.VoteType = NIL_VOTE
	}
	block.ConsensusDetails.ProposalHash = blockData.ConsensusData.Data.ProposalHash.HexLower()
	block.ConsensusDetails.PrecommitHash = blockData.ConsensusData.Data.PrecommitHash.HexLower()
	if blockData.ConsensusData.BlockRewardsInfo.SlashedValidators != nil {
		block.ConsensusDetails.SlashedValidators = make([]Slashing, len(blockData.ConsensusData.BlockRewardsInfo.SlashedValidators))
		for i, v := range blockData.ConsensusData.BlockRewardsInfo.SlashedValidators {
			block.ConsensusDetails.SlashedValidators[i] = Slashing{
				SlashedAccount: v.SlashedValidator.HexLower(),
				SlashedAmount:  v.SlashedAmount,
			}
		}
	}
	block.ConsensusDetails.Rounds = blockData.ConsensusData.Data.Round
	if blockData.ConsensusData.Data.SelectedTransactions != nil {
		block.ConsensusDetails.SelectedTransactions = make([]string, len(blockData.ConsensusData.Data.SelectedTransactions))
		for i, v := range blockData.ConsensusData.Data.SelectedTransactions {
			block.ConsensusDetails.SelectedTransactions[i] = v.HexLower()
		}
	}
	block.ConsensusDetails.BlockTime = blockData.ConsensusData.Data.BlockTime
	block.ConsensusDetails.BlockProposerRewards = blockData.ConsensusData.BlockRewardsInfo.BlockProposerRewards
	block.ConsensusDetails.BaseBlockProposerRewards = blockData.ConsensusData.BlockRewardsInfo.BaseBlockProposerRewards
	block.ConsensusDetails.TxnFeeRewards = blockData.ConsensusData.BlockRewardsInfo.TxnFeeRewards
	block.ConsensusDetails.BurntTxnFee = blockData.ConsensusData.BlockRewardsInfo.BurntTxnFee
	block.ConsensusDetails.SlashAmount = blockData.ConsensusData.BlockRewardsInfo.SlashAmount

	return &block
}

type BlockCompact struct {
	Hash             string `json:"hash,omitempty"`
	Number           uint64 `json:"number,omitempty"`
	BlockProposer    string `json:"blockProposer"`
	TransactionCount uint   `json:"transactionCount,omitempty"`
	Time             uint64 `json:"timestamp,omitempty"`
}

func fromBlock(block *Block) *BlockCompact {
	return &BlockCompact{
		Hash:             block.Hash,
		Number:           block.Number,
		BlockProposer:    block.ConsensusDetails.BlockProposer,
		TransactionCount: block.TransactionCount,
		Time:             block.Time,
	}
}

type BlockList struct {
	Blocks []BlockCompact `json:"blocks"`
}

type ListBlocksResponse struct {
	PageCount uint64         `json:"pageCount"`
	Items     []BlockCompact `json:"items"`
}
