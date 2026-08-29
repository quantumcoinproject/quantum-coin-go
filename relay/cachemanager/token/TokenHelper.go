package token

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/token"
	"math/big"
	"strings"
)

var NotATokenError = errors.New("invalid erc20 token")
var contractAbi, _ = abi.JSON(strings.NewReader(string(token.TokenMetaData.ABI)))
var logTransferSig = []byte("Transfer(address,address,uint256)")
var LogApprovalSig = []byte("Approval(address,address,uint256)")
var logTransferSigHash = strings.ToLower(crypto.Keccak256Hash(logTransferSig).Hex())
var logApprovalSigHash = strings.ToLower(crypto.Keccak256Hash(LogApprovalSig).Hex())

// Wrapped-Q (WETH9-style) balance events. deposit() mints WQ to the caller and
// emits Deposit(dst, wad); withdraw() burns and emits Withdrawal(src, wad).
// Neither emits Transfer, so without these the account's WQ holding never
// enters the index and its quantity never moves on wrap/unwrap.
var logDepositSig = []byte("Deposit(address,uint256)")
var logWithdrawalSig = []byte("Withdrawal(address,uint256)")
var logDepositSigHash = strings.ToLower(crypto.Keccak256Hash(logDepositSig).Hex())
var logWithdrawalSigHash = strings.ToLower(crypto.Keccak256Hash(logWithdrawalSig).Hex())

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

// Is main transaction a transfer?
func IsMainTransactionTokenTransfer(txn *types.Transaction, receipt *types.Receipt) (bool, error) {
	txHash := txn.Hash()
	if txHash.IsEqualTo(receipt.TxHash) == false {
		return false, errors.New("hash mismatch between txn and receipt")
	}

	if len(receipt.Logs) == 0 || len(receipt.Logs[0].Topics) == 0 {
		return false, nil
	}

	return strings.ToLower(receipt.Logs[0].Topics[0].Hex()) == logTransferSigHash, nil
}

func ParseTokenTransaction(txn *types.Transaction, receipt *types.Receipt) ([]*LogTransfer, []*LogApproval, error) {
	txHash := txn.Hash()
	if txHash.IsEqualTo(receipt.TxHash) == false {
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

		case logDepositSigHash, logWithdrawalSigHash:
			// A wrap is a mint of WQ to the account and an unwrap a burn, so
			// they are surfaced as the equivalent Transfer legs from/to the zero
			// address: the balance refresh downstream then re-reads balanceOf
			// for the account exactly as it does for a real transfer. Both
			// events have one indexed arg (the account); the amount is the
			// first data word. The token ABI carries neither event, hence the
			// manual decode. Emitters that are not tokens are dropped
			// downstream, so a same-named event from an unrelated contract
			// costs at most one node check.
			if len(rLog.Topics) < 2 || len(rLog.Data) < 32 {
				continue
			}
			leg := LogTransfer{
				ContractAddress: rLog.Address,
				Tokens:          new(big.Int).SetBytes(rLog.Data[:32]),
			}
			account := common.HexToAddress(rLog.Topics[1].Hex())
			if strings.ToLower(rLog.Topics[0].Hex()) == logDepositSigHash {
				leg.To = account // mint: zero -> account
			} else {
				leg.From = account // burn: account -> zero
			}
			transfers = append(transfers, &leg)
		}
	}

	return transfers, approvals, nil
}
