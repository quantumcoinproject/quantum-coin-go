package qcreadapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/relay"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
	"net/http"
	"time"
)

// GetTransactionDetails - Get transaction Details
func (s *ReadApiAPIService) GetTransactionDetails(ctx context.Context, hash string) (ImplResponse, error) {

	startTime := time.Now()
	isDiscarded := false
	discardReason := ""
	log.Info(relay.InfoTitleTransaction, relay.MsgDial, s.DpUrl)

	client, err := rpc.Dial(s.DpUrl)
	if err != nil {
		log.Error(relay.MsgDial, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}
	defer client.Close()

	if !common.IsHexAddressDeep(hash) {
		log.Error(relay.MsgHash, relay.MsgHash, hash, relay.MsgError, relay.ErrInvalidHash, relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), relay.ErrInvalidHash
	}

	var raw json.RawMessage
	err = client.CallContext(ctx, &raw, "eth_getTransactionByHash", common.HexToHash(hash))
	if err != nil {
		log.Error(relay.MsgTransaction, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	if raw != nil {

		var rpcTxn *RPCTransaction

		err = json.Unmarshal(raw, &rpcTxn)
		if err != nil {
			log.Error(relay.MsgNonce, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
			return Response(http.StatusBadRequest, nil), errors.New(err.Error())
		}

		var blochHash string
		var blockNumber int64
		var from, gas, gasPrice, txnHash, input, to string

		if rpcTxn.BlockHash != nil {
			blochHash = rpcTxn.BlockHash.String()
		}
		if rpcTxn.BlockNumber != nil {
			b := rpcTxn.BlockNumber.ToInt()
			blockNumber = b.Int64()
		}

		from = rpcTxn.From.String()
		gas = rpcTxn.Gas.String()
		gasPrice = rpcTxn.GasPrice.String()
		txnHash = rpcTxn.Hash.String()
		input = rpcTxn.Input.String()

		if rpcTxn.To != nil {
			to = rpcTxn.To.String()
		}

		transNonce := rpcTxn.Nonce
		n, err := hexutil.DecodeBig(transNonce.String())
		if err != nil {
			log.Error(relay.MsgNonce, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
			return Response(http.StatusBadRequest, nil), errors.New(err.Error())
		}

		nonce := n.Int64()

		value := rpcTxn.Value.String()

		var receipt map[string]interface{}
		err = client.CallContext(ctx, &receipt, "eth_getTransactionReceipt", common.HexToHash(hash))
		if err != nil {
			log.Error(relay.MsgTransactionReceipt, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusServiceUnavailable)
			return Response(http.StatusServiceUnavailable, nil), errors.New(err.Error())
		}

		var transactionReceipt TransactionReceipt
		if receipt != nil {
			cumulativeGasUsed := receipt["cumulativeGasUsed"].(string)
			effectiveGasPrice := receipt["effectiveGasPrice"].(string)
			gasUsed := receipt["gasUsed"].(string)
			status := receipt["status"].(string)
			txnReceiptHash := receipt["transactionHash"].(string)
			t := receipt["type"].(string)
			contractAddress := ""
			if receipt["contractAddress"] != nil {
				contractAddress = receipt["contractAddress"].(string)
			}
			transactionReceipt = TransactionReceipt{
				cumulativeGasUsed, effectiveGasPrice, gasUsed,
				status, txnReceiptHash, t, contractAddress}
		} else {
			duration := time.Now().Sub(startTime)

			log.Info(relay.InfoTitleTransaction, relay.MsgHash, hash, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

			txnDetails := TransactionDetails{
				&blochHash, &blockNumber, from, gas, gasPrice, txnHash,
				input, &isDiscarded, &discardReason, nonce, &to, value,
				transactionReceipt}

			Dump(txnDetails)

			return Response(http.StatusOK, TransactionResponse{txnDetails}), nil
		}
		duration := time.Now().Sub(startTime)

		log.Info(relay.InfoTitleTransaction, relay.MsgHash, hash, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

		txnDetails := TransactionDetails{
			&blochHash, &blockNumber, from, gas, gasPrice, txnHash,
			input, &isDiscarded, &discardReason, nonce, &to, value,
			transactionReceipt}

		Dump(txnDetails)

		return Response(http.StatusOK, TransactionResponse{txnDetails}), nil
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleTransaction, relay.MsgHash, hash, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	return Response(http.StatusNotFound, nil), nil
}
