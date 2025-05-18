package qcreadapi

import (
	"context"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/cachemanager"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/relay"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
	"net/http"
	"time"
)

// GetLatestBlockDetails - Get latest block details
func (s *ReadApiAPIService) GetLatestBlockDetails(ctx context.Context) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleLatestBlockDetails, relay.MsgDial, s.DpUrl)

	client, err := rpc.Dial(s.DpUrl)
	if err != nil {
		log.Error(relay.MsgDial, relay.MsgError, "errors.New(err.Error())", relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}
	defer client.Close()

	var blockNumber *hexutil.Uint64
	err = client.CallContext(ctx, &blockNumber, "eth_blockNumber")
	if err != nil {
		log.Error(relay.MsgBlockNumber, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	latestBlockNumber, err := hexutil.DecodeBig(blockNumber.String())
	if err != nil {
		log.Error(relay.MsgBlockNumber, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleLatestBlockDetails, "blockNumber", latestBlockNumber.Int64(), relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)
	l := latestBlockNumber.Int64()
	blockDetails := cachemanager.Block{
		BlockNumber: int64(l),
	}
	return Response(http.StatusOK, BlockDetailsResponse{blockDetails}), nil
}

// GetBlockDetails - Get latest block details
func (s *ReadApiAPIService) GetBlockDetails(ctx context.Context, blockNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleLatestBlockDetails, relay.MsgDial, s.DpUrl)

	block, err := s.cacheManager.GetBlockDetails(uint64(blockNumber))
	if err != nil {
		log.Error(relay.MsgBlockNumber, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}
	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleLatestBlockDetails, "blockNumber", blockNumber, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)
	return Response(http.StatusOK, BlockDetailsResponse{*block}), nil
}
