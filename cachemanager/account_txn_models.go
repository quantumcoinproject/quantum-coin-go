package cachemanager

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
