// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

/*
 * QC Write API
 *
 * No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)
 *
 * API version: v1
 */

package qcwriteapi

import (
	"context"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/relay"
	"github.com/QuantumCoinProject/qc/rpc"
	"net/http"
	"errors"
	"github.com/mattn/go-colorable"
	"strings"
	"time"
)

// WriteApiAPIService is a service that implements the logic for the WriteApiAPIServicer
// This service should implement the business logic for every endpoint for the WriteApiAPI API.
// Include any external packages or services that will be required by this service.
type WriteApiAPIService struct {
	DpUrl string
}

// NewWriteApiAPIService creates a default api service
func NewWriteApiAPIService(dpUrl string) *WriteApiAPIService {
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(3), log.StreamHandler(colorable.NewColorableStderr(), log.TerminalFormat(true))))
	return &WriteApiAPIService{DpUrl: dpUrl}
}

// SendTransaction - Send Transaction
func (s *WriteApiAPIService) SendTransaction(ctx context.Context, sendTransactionRequest SendTransactionRequest) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleSendTransaction, relay.MsgDial, s.DpUrl)

	client, err := rpc.Dial(s.DpUrl)
	if err != nil {
		log.Error(relay.MsgDial, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}
	defer client.Close()

	rawTxHex := sendTransactionRequest.TxnData

	if(len(strings.TrimSpace(rawTxHex)) == 0) {
		log.Error(relay.MsgRawRawTxHex, relay.MsgError, relay.ErrEmptyRawTxHex, relay.MsgStatus, http.StatusBadRequest)
		return  Response(http.StatusBadRequest, nil), relay.ErrEmptyRawTxHex
	}

	var txHash *common.Hash
	err = client.CallContext(ctx, &txHash, "eth_sendRawTransaction", rawTxHex)

	if err != nil {
		log.Error(relay.MsgSend + " " + relay.MsgTransaction, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.MsgSend + " " + relay.MsgTransaction, relay.MsgHash, txHash.String(), relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

	return Response(http.StatusOK, txHash.String()), nil
}
