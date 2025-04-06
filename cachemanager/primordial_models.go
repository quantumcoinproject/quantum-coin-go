package cachemanager

import (
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/proofofstake"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"math/big"
	"strings"
)

type PrimordialAccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
	Code    []byte                `json:"code,omitempty"` //for contracts only
}

type AccessTuple struct {
	Address     string   `json:"address"        gencodec:"required"`
	StorageKeys []string `json:"storageKeys"    gencodec:"required"`
}

func fromNativeAccessTuple(accessTuple *types.AccessTuple) *AccessTuple {
	if accessTuple == nil {
		return nil
	}
	at := &AccessTuple{}
	at.Address = strings.ToLower(accessTuple.Address.Hex())
	if accessTuple.StorageKeys != nil {
		at.StorageKeys = make([]string, len(accessTuple.StorageKeys))
		for i, item := range accessTuple.StorageKeys {
			at.StorageKeys[i] = strings.ToLower(item.Hex())
		}
	}

	return at
}

type PrimordialTransaction struct {
	TxType     byte           `json:"txType"       genco_tedec:"required"`
	AccessList []*AccessTuple `json:"accessList"`
	Data       []byte         `json:"data"`
	Gas        uint64         `json:"gas"`
	GasPrice   *big.Int       `json:"gasPrice"`
	Value      *big.Int       `json:"value"`
	Nonce      uint64         `json:"nonce"`
	From       string
	To         *string `json:"to"`
	Remarks    []byte  `json:"remarks"`
	Hash       string  `json:"hash"`
}

func fromNativeTransaction(txn *types.Transaction) *PrimordialTransaction {
	t := &PrimordialTransaction{
		TxType:   txn.Type(),
		Gas:      txn.Gas(),
		GasPrice: txn.GasPrice(),
		Value:    txn.Value(),
		Nonce:    txn.Nonce(),
	}
	accessList := txn.AccessList()
	if accessList != nil {
		t.AccessList = make([]*AccessTuple, len(accessList))
		for i, item := range accessList {
			t.AccessList[i] = fromNativeAccessTuple(&item)
		}
	}
	if txn.Data() != nil {
		t.Data = make([]byte, len(txn.Data()))
		copy(t.Data, txn.Data())
	}

	if txn.Remarks() != nil {
		t.Remarks = make([]byte, len(txn.Remarks()))
		copy(t.Remarks, txn.Remarks())
	}
	t.Hash = strings.ToLower(txn.Hash().Hex())

	msg, err := txn.AsMessage(types.NewLondonSigner(chainID))
	if err != nil {
		log.Error("AsMessage", "error", err)
	} else {
		t.From = strings.ToLower(msg.From().Hex())
	}

	if txn.To() != nil {
		to := strings.ToLower(txn.To().Hex())
		t.To = &to
	}

	return t
}

type TransactionDetailsExpanded struct {
	InternalTransactions []*InternalTransactionDetail `json:"internalTransactions,omitempty"`
	Receipt              *PrimordialReceipt           `json:"receipt,omitempty"`
	RevertReason         string                       `json:"revertReason,omitempty"`
	Transaction          *PrimordialTransaction       `json:"transaction"`
}

type PrimordialBlock struct {
	Hash              string   `json:"hash"             genco_tedec:"required"`
	ParentHash        string   `json:"parentHash"       genco_tedec:"required"`
	StateRoot         string   `json:"stateRoot"        gencodec:"required"`
	TransactionsRoot  string   `json:"transactionsRoot" gencodec:"required"`
	ReceiptsRoot      string   `json:"receiptsRoot"     gencodec:"required"`
	Number            *big.Int `json:"number"           gencodec:"required"`
	GasLimit          uint64   `json:"gasLimit"         gencodec:"required"`
	GasUsed           uint64   `json:"gasUsed"          gencodec:"required"`
	Time              uint64   `json:"timestamp"        gencodec:"required"`
	MixDigest         string   `json:"mixHash"          gencodec:"required"`
	TransactionsCount uint     `json:"transactionsCount"          gencodec:"required"`
}

type PrimordialBlockData struct {
	Block                     *PrimordialBlock                 `json:"block,omitempty"`
	ConsensusData             *proofofstake.ConsensusData      `json:"consensusData,omitempty"`
	ZeroAddressBalance        *big.Int                         `json:"zeroAddressBalance,omitempty"`
	StakingContractBalance    *big.Int                         `json:"stakingContractBalance,omitempty"`
	ConversionContractBalance *big.Int                         `json:"conversionContractBalance,omitempty"`
	TransactionList           []*TransactionDetailsExpanded    `json:"transactionList,omitempty"`
	ValidatorList             []*proofofstake.ValidatorDetails `json:"validatorList,omitempty"`
}

func fromNativeBlock(block *types.Block) *PrimordialBlock {
	b := &PrimordialBlock{
		Number:   block.Number(),
		GasLimit: block.GasLimit(),
		GasUsed:  block.GasUsed(),
		Time:     block.Time(),
	}
	b.Hash = block.Header().Hash().HexLower()
	b.Hash = block.Header().Hash().HexLower()
	b.ParentHash = block.ParentHash().HexLower()
	b.StateRoot = block.Root().HexLower()
	b.TransactionsRoot = block.TxHash().HexLower()
	b.ReceiptsRoot = block.ReceiptHash().HexLower()
	b.MixDigest = block.MixDigest().HexLower()
	b.TransactionsCount = uint(len(block.Transactions()))

	return b
}

// Log represents a contract log event. These events are generated by the LOG opcode and
// stored/indexed by the node.
type Log struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address `json:"address" gencodec:"required"`
	// list of topics provided by the contract.
	Topics []common.Hash `json:"topics" gencodec:"required"`
	// supplied by the contract, usually ABI-encoded
	Data []byte `json:"data" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber uint64 `json:"blockNumber"`
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
	// index of the transaction in the block
	TxIndex uint `json:"transactionIndex"`
	// hash of the block in which the transaction was included
	BlockHash common.Hash `json:"blockHash"`
	// index of the log in the block
	Index uint `json:"logIndex"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed"`
}

func fromNativeLog(log *types.Log) *Log {
	if log == nil {
		return nil
	}
	l := &Log{
		BlockNumber: log.BlockNumber,
		TxIndex:     log.TxIndex,
		Index:       log.Index,
		Removed:     log.Removed,
	}

	l.Address.CopyFrom(log.Address)
	if l.Topics != nil {
		log.Topics = make([]common.Hash, len(log.Topics))
		for i, item := range log.Topics {
			l.Topics[i].CopyFrom(item)
		}
	}

	if log.Data != nil {
		l.Data = make([]byte, len(log.Data))
		copy(l.Data, log.Data)
	}
	l.TxHash.CopyFrom(log.TxHash)
	l.BlockHash.CopyFrom(log.BlockHash)

	return l
}

// PrimordialReceipt represents the results of a transaction.
type PrimordialReceipt struct {
	// Consensus fields: These fields are defined by the Yellow Paper
	Type              uint8  `json:"type,omitempty"`
	PostState         []byte `json:"root"`
	Status            uint64 `json:"status"`
	CumulativeGasUsed uint64 `json:"cumulativeGasUsed" gencodec:"required"`
	Logs              []*Log `json:"logs"              gencodec:"required"`

	// Implementation fields: These fields are added by geth when processing a transaction.
	// They are stored in the chain database.
	TxHash          string `json:"transactionHash" gencodec:"required"`
	ContractAddress string `json:"contractAddress"`
	GasUsed         uint64 `json:"gasUsed" gencodec:"required"`

	// Inclusion information: These fields provide information about the inclusion of the
	// transaction corresponding to this receipt.
	BlockHash        string   `json:"blockHash,omitempty"`
	BlockNumber      *big.Int `json:"blockNumber,omitempty"`
	TransactionIndex uint     `json:"transactionIndex"`
}

func fromNativeReceipt(receipt *types.Receipt) *PrimordialReceipt {
	if receipt == nil {
		return nil
	}
	r := &PrimordialReceipt{
		Type:              receipt.Type,
		Status:            receipt.Status,
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		GasUsed:           receipt.GasUsed,
		BlockNumber:       receipt.BlockNumber,
		TransactionIndex:  receipt.TransactionIndex,
	}

	if receipt.PostState != nil {
		r.PostState = make([]byte, len(receipt.PostState))
		copy(r.PostState, receipt.PostState)
	}

	if receipt.Logs != nil {
		r.Logs = make([]*Log, len(receipt.Logs))
		for i, item := range receipt.Logs {
			r.Logs[i] = fromNativeLog(item)
		}
	}

	r.TxHash = receipt.TxHash.HexLower()
	r.ContractAddress = receipt.ContractAddress.HexLower()
	r.BlockHash = receipt.BlockHash.HexLower()

	return r
}
