package cachemanager

import "math/big"

type TransactionType string

// List of TransactionType
const (
	COIN_TRANSFER      TransactionType = "CoinTransfer"
	NEW_TOKEN          TransactionType = "NewToken"
	TOKEN_TRANSFER     TransactionType = "TokenTransfer"
	NEW_SMART_CONTRACT TransactionType = "NewSmartContract"
	SMART_CONTRACT     TransactionType = "SmartContract"
)

type Slashing struct {
	SlashedAccount string `json:"slashedAccount"`
	SlashedAmount  string `json:"slashedAmount"     gencodec:"required"`
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

	Input string `json:"data,omitempty"`

	Nonce uint64 `json:"nonce,omitempty"`

	Value string `json:"value,omitempty"`

	Receipt TransactionReceipt `json:"receipt,omitempty"`

	CreatedAt string `json:"createdAt,omitempty"`

	TxnFee string `json:"txnFee,omitempty"`

	TransactionType string `json:"transactionType,omitempty"`

	TokenTransaction TokenTransactionCompact `json:"tokenTransaction,omitempty"`
}

type TransactionCompact struct {
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

func transactionCompactFromTransaction(txn *TransactionDetails) TransactionCompact {
	return TransactionCompact{
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

type TransactionList struct {
	Transactions []TransactionCompact `json:"transactions"`
}

type ListTransactionsResponse struct {
	PageCount uint64               `json:"pageCount"`
	Items     []TransactionCompact `json:"items"`
}

type TransactionReport struct {
	TotalTransactions uint64 `json:"totalTransactions,omitempty"`
	ReportDate        int64  `json:"reportDate,omitempty"`
}
