package cachemanager

import (
	"errors"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/crypto"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/token"
	"math/big"
	"strings"
)

var contractAbi, _ = abi.JSON(strings.NewReader(string(token.TokenMetaData.ABI)))
var logTransferSig = []byte("Transfer(address,address,uint256)")
var LogApprovalSig = []byte("Approval(address,address,uint256)")
var logTransferSigHash = strings.ToLower(crypto.Keccak256Hash(logTransferSig).Hex())
var logApprovalSigHash = strings.ToLower(crypto.Keccak256Hash(LogApprovalSig).Hex())

type LogTransferValue struct {
	Value *big.Int
}

type LogTransfer struct {
	ContractAddress common.Address
	From            common.Address
	To              common.Address
	Tokens          *big.Int
}

type LogApproval struct {
	ContractAddress common.Address
	TokenOwner      common.Address
	Spender         common.Address
	Tokens          *big.Int
}

// Is main transaction a transfer?
func IsMainTransactionTokenTransfer(txn *PrimordialTransaction, receipt *PrimordialReceipt) (bool, error) {
	txHash := txn.Hash
	if (txHash == receipt.TxHash) == false {
		return false, errors.New("hash mismatch between txn and receipt")
	}

	if len(receipt.Logs) == 0 || len(receipt.Logs[0].Topics) == 0 {
		return false, nil
	}

	return strings.ToLower(receipt.Logs[0].Topics[0].Hex()) == logTransferSigHash, nil
}

func ParseTokenTransaction(txn *PrimordialTransaction, receipt *PrimordialReceipt) ([]*LogTransfer, []*LogApproval, error) {
	txHash := txn.Hash
	if (txHash == receipt.TxHash) == false {
		return nil, nil, errors.New("hash mismatch between txn and receipt")
	}

	transfers := make([]*LogTransfer, 0)
	approvals := make([]*LogApproval, 0)

	for _, rLog := range receipt.Logs {

		if len(rLog.Topics) == 0 {
			continue
		}

		switch strings.ToLower(rLog.Topics[0].Hex()) {

		case logTransferSigHash:
			var transferEvent LogTransfer
			var logTransferValue LogTransferValue

			err := contractAbi.UnpackIntoInterface(&logTransferValue, "Transfer", rLog.Data)
			if err != nil {
				log.Debug("ParseTokenTransaction Transfer", "error", err)
				//do not return from error, since from/to address have to be updated still for balance update
			} else {
				transferEvent.Tokens = logTransferValue.Value
			}

			transferEvent.From = common.HexToAddress(rLog.Topics[1].Hex())
			transferEvent.To = common.HexToAddress(rLog.Topics[2].Hex())

			transferEvent.ContractAddress = rLog.Address

			transfers = append(transfers, &transferEvent)

		case logApprovalSigHash:
			var approvalEvent LogApproval
			var logTransferValue LogTransferValue

			err := contractAbi.UnpackIntoInterface(&logTransferValue, "Approval", rLog.Data)
			if err != nil {
				log.Debug("ParseTokenTransaction Approval", "error", err)
				//do not return from error, since from/to address have to be updated still for balance update
			} else {
				approvalEvent.Tokens = logTransferValue.Value
			}

			approvalEvent.ContractAddress = rLog.Address
			approvalEvent.TokenOwner = common.HexToAddress(rLog.Topics[1].Hex())
			approvalEvent.Spender = common.HexToAddress(rLog.Topics[2].Hex())

			approvals = append(approvals, &approvalEvent)
		}
	}

	return transfers, approvals, nil
}
