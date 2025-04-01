package cachemanager

import (
	"github.com/QuantumCoinProject/qc/ethclient"
	"strings"
)

type InternalTransactionDetailWithLevel struct {
	txn   *ethclient.InternalTransactionDetails
	level byte
}

type InternalTransactionDetail struct {
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Value   string `json:"value,omitempty"`
	Type    string `json:"type,omitempty"`
	Gas     string `json:"gas,omitempty"`
	GasUsed string `json:"gasUsed,omitempty"`
	Input   string `json:"input,omitempty"`
	Output  string `json:"output,omitempty"`
	Level   byte   `json:"level,omitempty"`
}

type Stack struct { //not thread safe
	internalTxnDetails []*InternalTransactionDetailWithLevel
	count              int
}

func newStack() *Stack {
	s := Stack{
		internalTxnDetails: make([]*InternalTransactionDetailWithLevel, 0),
	}
	return &s
}

func (s *Stack) Size() int {
	return len(s.internalTxnDetails)
}

func (s *Stack) IsEmpty() bool {
	return len(s.internalTxnDetails) == 0
}

func (s *Stack) Push(v *InternalTransactionDetailWithLevel) {
	s.internalTxnDetails = append(s.internalTxnDetails, v)
	s.count = len(s.internalTxnDetails)
}

func (s *Stack) Pop() *InternalTransactionDetailWithLevel {
	count := len(s.internalTxnDetails)
	last := s.internalTxnDetails[count-1]
	s.internalTxnDetails = s.internalTxnDetails[:count-1]

	return last
}

func flattenInternalTransactionDetails(details *ethclient.InternalTransactionDetails) []*InternalTransactionDetail {
	txnStack := newStack()
	txnList := make([]*InternalTransactionDetail, 0)

	detailsWithLevel := &InternalTransactionDetailWithLevel{
		txn:   details,
		level: byte(0),
	}
	txnStack.Push(detailsWithLevel)

	for txnStack.IsEmpty() == false {
		txnWithLevel := txnStack.Pop()
		txn := txnWithLevel.txn
		txnDetail := InternalTransactionDetail{
			From:    strings.ToLower(txn.From),
			To:      strings.ToLower(txn.To),
			Value:   txn.Value,
			Type:    txn.Type,
			Gas:     txn.Gas,
			GasUsed: txn.GasUsed,
			Input:   txn.Input,
			Output:  txn.Output,
			Level:   txnWithLevel.level,
		}
		txnList = append(txnList, &txnDetail)
		if txn.Calls != nil {
			for _, t := range txn.Calls {
				detailsWithLevel = &InternalTransactionDetailWithLevel{
					txn:   &t,
					level: txnWithLevel.level + 1,
				}
				txnStack.Push(detailsWithLevel)
			}
		}
	}
	return txnList
}
