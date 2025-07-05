package qcreadapi

import (
	"context"
	"github.com/quantumcoinproject/quantum-coin-go/cachemanager"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	relay "github.com/quantumcoinproject/quantum-coin-go/relayv2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GetBlockchainDetails - Get blockchain details
func (s *ReadApiAPIService) GetBlockchainDetails(ctx context.Context) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleGetBlockchainDetails)

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleGetBlockchainDetails, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	if s.enableExtendedApis == false {
		return Response(http.StatusNotFound, nil), NotFoundError
	}

	getResponse, err := s.cacheManager.GetBlockchainDetails()
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), InternalError
	}

	return Response(http.StatusOK, getResponse), nil
}

// QueryDetails - Query details
func (s *ReadApiAPIService) QueryDetails(ctx context.Context, queryTerm string) (ImplResponse, error) {
	queryTerm = strings.ToLower(queryTerm)
	startTime := time.Now()

	log.Info(relay.InfoTitleQueryDetails)

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleQueryDetails, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	if s.enableExtendedApis == false {
		return Response(http.StatusNotFound, nil), NotFoundError
	}

	getResponse, err := s.cacheManager.GetBlockchainDetails()
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), InternalError
	}

	result := ""
	if queryTerm == "totalcoins" {
		val, err := hexutil.DecodeBig(getResponse.Result.TotalSupply)
		if err != nil {
			return Response(http.StatusInternalServerError, nil), InternalError
		}
		result = strconv.FormatUint(params.WeiToEther(val).Uint64(), 10)
	} else if queryTerm == "circulating" {
		val, err := hexutil.DecodeBig(getResponse.Result.CirculatingSupply)
		if err != nil {
			return Response(http.StatusInternalServerError, nil), InternalError
		}
		result = strconv.FormatUint(params.WeiToEther(val).Uint64(), 10)
	}

	return Response(http.StatusOK, result), nil
}
