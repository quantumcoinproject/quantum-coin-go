package cachemanager

import (
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/ethclient"
	"math/big"
)

// List of TransactionType
const (
	COIN_TRANSFER      TransactionType = "CoinTransfer"
	NEW_TOKEN          TransactionType = "NewToken"
	TOKEN_TRANSFER     TransactionType = "TokenTransfer"
	NEW_SMART_CONTRACT TransactionType = "NewSmartContract"
	SMART_CONTRACT     TransactionType = "SmartContract"
)

type AccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
}

type TokenTransfers struct {
	ContractAddress common.Address
	From            common.Address
	To              common.Address
	Tokens          *big.Int
}

type TokenApprovals struct {
	ContractAddress common.Address
	TokenOwner      common.Address
	Spender         common.Address
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
