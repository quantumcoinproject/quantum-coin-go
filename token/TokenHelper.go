package token

import (
	"errors"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/crypto"
	"math/big"
	"strings"
)

var NotATokenError = errors.New("invalid erc20 token")
var contractAbi, _ = abi.JSON(strings.NewReader(string(TokenMetaData.ABI)))
var logTransferSig = []byte("Transfer(address,address,uint256)")
var LogApprovalSig = []byte("Approval(address,address,uint256)")
var logTransferSigHash = strings.ToLower(crypto.Keccak256Hash(logTransferSig).Hex())
var logApprovalSigHash = strings.ToLower(crypto.Keccak256Hash(LogApprovalSig).Hex())

var InvalidTokenLog = errors.New("invalid token log")

type TokenDetails struct {
	Name        string
	Symbol      string
	Owner       common.Address
	TotalSupply *big.Int
	Decimals    uint8
}

type LogTransfer struct {
	From   common.Address
	To     common.Address
	Tokens *big.Int
}

type LogApproval struct {
	TokenOwner common.Address
	Spender    common.Address
	Tokens     *big.Int
}

func GetAccountsInvolvedOfTransaction(txn *types.Transaction, receipt *types.Receipt) ([]*LogTransfer, []*LogApproval, error) {
	if txn == nil || receipt == nil || txn.To() == nil {
		return nil, nil, errors.New("not a token contract")
	}

	txHash := txn.Hash()
	if txHash.IsEqualTo(receipt.TxHash) == false {
		return nil, nil, errors.New("hash mismatch")
	}

	transfers := make([]*LogTransfer, 0)
	approvals := make([]*LogApproval, 0)

	for _, rLog := range receipt.Logs {
		switch strings.ToLower(rLog.Topics[0].Hex()) {

		case logTransferSigHash:
			var transferEvent LogTransfer

			err := contractAbi.UnpackIntoInterface(&transferEvent, "Transfer", rLog.Data)
			if err != nil {
				return nil, nil, InvalidTokenLog
			}

			transferEvent.From = common.HexToAddress(rLog.Topics[1].Hex())
			transferEvent.To = common.HexToAddress(rLog.Topics[2].Hex())

			transfers = append(transfers, &transferEvent)

		case logApprovalSigHash:
			var approvalEvent LogApproval

			err := contractAbi.UnpackIntoInterface(&approvalEvent, "Approval", rLog.Data)
			if err != nil {
				return nil, nil, InvalidTokenLog
			}

			approvalEvent.TokenOwner = common.HexToAddress(rLog.Topics[1].Hex())
			approvalEvent.Spender = common.HexToAddress(rLog.Topics[2].Hex())

			approvals = append(approvals, &approvalEvent)
		}
	}

	if len(transfers) > 0 || len(approvals) > 0 {
		panic("done")
	}

	return transfers, approvals, nil
}
