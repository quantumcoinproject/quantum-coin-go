package token

import (
	"errors"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/crypto"
	"github.com/QuantumCoinProject/qc/log"
	"math/big"
	"strings"
)

var NotATokenError = errors.New("invalid erc20 token")
var contractAbi, _ = abi.JSON(strings.NewReader(string(TokenMetaData.ABI)))
var logTransferSig = []byte("Transfer(address,address,uint256)")
var LogApprovalSig = []byte("Approval(address,address,uint256)")
var logTransferSigHash = strings.ToLower(crypto.Keccak256Hash(logTransferSig).Hex())
var logApprovalSigHash = strings.ToLower(crypto.Keccak256Hash(LogApprovalSig).Hex())

type TokenDetails struct {
	Name        string
	Symbol      string
	Owner       common.Address
	TotalSupply *big.Int
	Decimals    uint8
}

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

func ParseTokenTransaction(txn *types.Transaction, receipt *types.Receipt) ([]*LogTransfer, []*LogApproval, error) {
	if txn == nil || receipt == nil {
		return nil, nil, errors.New("txn or receipt is nil")
	}

	txHash := txn.Hash()
	if txHash.IsEqualTo(receipt.TxHash) == false {
		return nil, nil, errors.New("hash mismatch between txn and receipt")
	}

	transfers := make([]*LogTransfer, 0)
	approvals := make([]*LogApproval, 0)

	for _, rLog := range receipt.Logs {

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
