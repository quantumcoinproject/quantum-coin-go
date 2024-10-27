package qcreadapi

type TransactionReceiptInternal struct {
	Status int64 `json:"status,omitempty"`
}

type AccountTransactionCompactInternal struct {
	TxnHash string `json:"txnHash,omitempty"`

	BlockNumber int64 `json:"blockNumber,omitempty"`

	CreatedAt string `json:"createdAt,omitempty"`

	FromAddress *string `json:"fromAddress,omitempty"`

	ToAddress *string `json:"toAddress,omitempty"`

	Value *string `json:"value,omitempty"`

	TxnFee *string `json:"txnFee,omitempty"`

	TransactionType int64 `json:"transactionType,omitempty"`

	Receipt TransactionReceiptInternal `json:"receipt,omitempty"`

	ErrorReason *string `json:"errorReason,omitempty"`
}

type ListAccountTransactionsResponseInternal struct {
	PageCount int64 `json:"pageCount,omitempty"`

	Items []AccountTransactionCompactInternal `json:"result,omitempty"`
}
