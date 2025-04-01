package cachemanager

import (
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/consensus/proofofstake"
	"github.com/QuantumCoinProject/qc/ethclient"
	"math/big"
)

type TransactionType string

// List of TransactionType
const (
	COIN_TRANSFER      TransactionType = "CoinTransfer"
	NEW_TOKEN          TransactionType = "NewToken"
	TOKEN_TRANSFER     TransactionType = "TokenTransfer"
	NEW_SMART_CONTRACT TransactionType = "NewSmartContract"
	SMART_CONTRACT     TransactionType = "SmartContract"
)

const (
	OK_VOTE  VoteType = "OK"
	NIL_VOTE VoteType = "NIL"
)

type VoteType string

type Slashing struct {
	SlashedAccount string `json:"slashedAccount"`
	SlashedAmount  string `json:"slashedAmount"     gencodec:"required"`
}

type BlockConsensusDetails struct {
	BlockProposer        string     `json:"blockProposer"`
	VoteType             VoteType   `json:"voteType"`
	ProposalHash         string     `json:"proposalHash"`
	PrecommitHash        string     `json:"precommitHash"`
	SlashedValidators    []Slashing `json:"slashedValidators,omitempty"`
	Rounds               byte
	SelectedTransactions []string `json:"selectedTransactions,omitempty"` //this will be a super-set of transactions that actually got executed
	BlockTime            uint64   `json:"blockTime"`

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
	Time             uint64                `json:"timestamp,omitempty"`
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

type AccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
}

type TokenTransfers struct {
	ContractAddress string
	From            string
	To              string
	Tokens          *big.Int
}

type TokenApprovals struct {
	ContractAddress string
	TokenOwner      string
	Spender         string
	Tokens          *big.Int
}

type TransactionReceipt struct {
	CumulativeGasUsed string `json:"cumulativeGasUsed,omitempty"`

	EffectiveGasPrice string `json:"effectiveGasPrice,omitempty"`

	GasUsed string `json:"gasUsed,omitempty"`

	Status string `json:"status,omitempty"`

	Type string `json:"type,omitempty"`

	ContractAddress string `json:"contractAddress,omitempty"`

	Hash string `json:"hash,omitempty"`
}

type TokenTransactionCompact struct {
	TokenFromAddress string `json:"tokenFromAddress,omitempty"`

	TokenToAddress string `json:"tokenToAddress,omitempty"`

	ContractAddress string `json:"contractAddress,omitempty"`

	TokenCount string `json:"tokenCount,omitempty"`

	TokenSymbol string `json:"tokenSymbol,omitempty"`

	TokenName string `json:"tokenName,omitempty"`
}

type TransactionDetails struct {
	Hash string `json:"hash,omitempty"`

	BlockHash string `json:"blockHash,omitempty"`

	BlockNumber uint64 `json:"blockNumber,omitempty"`

	Origin string `json:"origin,omitempty"`

	From string `json:"from,omitempty"`

	To string `json:"to,omitempty"`

	Gas string `json:"gas,omitempty"`

	GasPrice string `json:"gasPrice,omitempty"`

	Data []byte `json:"data,omitempty"`

	Nonce uint64 `json:"nonce,omitempty"`

	Value string `json:"value,omitempty"`

	Receipt TransactionReceipt `json:"receipt,omitempty"`

	CreatedAt string `json:"createdAt,omitempty"`

	TxnFee string `json:"txnFee,omitempty"`

	TransactionType string `json:"transactionType,omitempty"`

	TokenTransaction TokenTransactionCompact `json:"tokenTransaction,omitempty"`
}

type AccountTransactionCompact struct {
	Hash string `json:"hash,omitempty"`

	BlockNumber uint64 `json:"blockNumber,omitempty"`

	CreatedAt string `json:"createdAt,omitempty"`

	From string `json:"from,omitempty"`

	To string `json:"to,omitempty"`

	Value string `json:"value,omitempty"`

	TxnFee string `json:"txnFee,omitempty"`

	Status string `json:"status,omitempty"`

	TransactionType string `json:"transactionType,omitempty"`

	TokenTransaction TokenTransactionCompact `json:"tokenTransaction,omitempty"`
}

func accountTransactionCompactFromTransaction(txn *TransactionDetails) AccountTransactionCompact {
	return AccountTransactionCompact{
		Hash:             txn.Hash,
		BlockNumber:      txn.BlockNumber,
		CreatedAt:        txn.CreatedAt,
		From:             txn.From,
		To:               txn.To,
		Value:            txn.Value,
		TxnFee:           txn.TxnFee,
		Status:           txn.Receipt.Status,
		TransactionType:  txn.TransactionType,
		TokenTransaction: txn.TokenTransaction,
	}
}

type AccountTransactionList struct {
	Address      string                      `json:"address"`
	Transactions []AccountTransactionCompact `json:"transactions"`
}

type ListAccountTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount"`
	Items     []AccountTransactionCompact `json:"items"`
}

type AccountPendingTransactionCompact struct {
	Hash string `json:"hash,omitempty"`

	From string `json:"from,omitempty"`

	To string `json:"to,omitempty"`

	Value string `json:"value,omitempty"`

	Nonce uint64 `json:"nonce,omitempty"`
}

type ListAccountPendingTransactionsResponse struct {
	Items     []AccountPendingTransactionCompact `json:"items"`
	PageCount uint64                             `json:"pageCount"`
}

type BlockchainDetails struct {
	BlockNumber           uint64 `json:"blockNumber" gencodec:"required"`
	MaxSupply             string `json:"maxSupply" gencodec:"required"`
	TotalSupply           string `json:"totalSupply" gencodec:"required"`
	CirculatingSupply     string `json:"circulatingSupply" gencodec:"required"`
	BurntCoins            string `json:"burntCoins" gencodec:"required"`
	BlockRewardsCoins     string `json:"blockRewardsCoins" gencodec:"required"` //baseBlockRewardsCoins + TxnFeeRewardsCoins
	BaseBlockRewardsCoins string `json:"baseBlockRewardsCoins" gencodec:"required"`
	TxnFeeRewardsCoins    string `json:"txnFeeRewardsCoins" gencodec:"required"`
	TxnFeeBurntCoins      string `json:"txnFeeBurntCoins" gencodec:"required"`
	SlashedCoins          string `json:"slashedCoins" gencodec:"required"`
}

type GetBlockchainDetailsResponse struct {
	Result BlockchainDetails `json:"result" gencodec:"required"`
}

type TokenDetails struct {
	ContractAddress        string `json:"contractAddress,omitempty"`
	CreatorAddress         string `json:"creatorAddress,omitempty"`
	CreatedBlockNumber     uint64 `json:"createdBlockNumber,omitempty"`
	CreatedTransactionHash string `json:"createdTransactionHash,omitempty"`
	Name                   string `json:"name,omitempty"`
	Symbol                 string `json:"symbol,omitempty"`
	TotalSupply            string `json:"totalSupply,omitempty"`
	Decimals               string `json:"decimals,omitempty"`
}

type GetTokenDetailsResponse struct {
	Result TokenDetails `json:"result,omitempty"`
}

type AccountTokenSummary struct {
	AccountAddress  string `json:"accountAddress,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"`
	Name            string `json:"name,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	TokenBalance    string `json:"tokenBalance,omitempty"`
}

type AccountTokenList struct {
	Address string                `json:"address"`
	Tokens  []AccountTokenSummary `json:"tokens"`
}

type ListAccountTokensResponse struct {
	PageCount      uint64                `json:"pageCount"`
	AccountAddress string                `json:"accountAddress"`
	Items          []AccountTokenSummary `json:"items"`
}

type AccountTokenTransactionList struct {
	Address      string                      `json:"address"`
	Transactions []AccountTransactionCompact `json:"transactions"`
}

type ListAccountTokenTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount"`
	Items     []AccountTransactionCompact `json:"items"`
}
