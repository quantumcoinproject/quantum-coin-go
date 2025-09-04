package qcreadapi

import (
	"context"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/cachemanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	relay "github.com/quantumcoinproject/quantum-coin-go/relayv2"
	"net/http"
	"time"
)

// GetTokenDetails - Get account token details
func (s *ReadApiAPIService) GetTokenDetails(ctx context.Context, contractAddress string) (ImplResponse, error) {
	startTime := time.Now()

	log.Info(relay.InfoTitleAccountDetails, relay.MsgDial, s.DpUrl)

	if !common.IsHexAddressDeep(contractAddress) {
		log.Error(relay.MsgContractAddress, relay.MsgContractAddress, contractAddress, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), relay.ErrInvalidAddress
	}

	tokenDetailsResponse, err := s.cacheManager.GetTokenDetails(contractAddress)
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleTokenDetails, relay.MsgAddress, contractAddress, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

	return Response(http.StatusOK, tokenDetailsResponse), nil
}
