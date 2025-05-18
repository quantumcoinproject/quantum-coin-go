package qcreadapi

import (
	"context"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/cachemanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/relay"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
	"net/http"
	"time"
)

type ListValidatorReportResponse struct {
	PageCount int64                           `json:"pageCount,omitempty"`
	Items     []*cachemanager.ValidatorReport `json:"items,omitempty"`
}

type ValidatorDetailsResponse struct {
	Result *cachemanager.ValidatorDetails `json:"result,omitempty"`
}

// GetValidatorDetails - Get account details
func (s *ReadApiAPIService) GetValidatorDetails(ctx context.Context, address string) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleAccountDetails, relay.MsgDial, s.DpUrl)

	client, err := rpc.Dial(s.DpUrl)
	if err != nil {
		log.Error(relay.MsgDial, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}
	defer client.Close()

	if !common.IsHexAddressDeep(address) {
		log.Error(relay.MsgAddress, relay.MsgAddress, address, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), relay.ErrInvalidAddress
	}

	val, err := s.cacheManager.GetValidator(address)
	if err != nil {
		log.Error(relay.MsgDial, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleAccountDetails, relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

	valDetails := ValidatorDetailsResponse{
		Result: &val,
	}

	return Response(http.StatusOK, valDetails), nil
}

// ListValidatorReport - List validator report
func (s *ReadApiAPIService) ListValidatorReport(ctx context.Context, pageNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleListAccountTransactions)

	duration := time.Now().Sub(startTime)

	log.Info("ListValidatorReport", relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent, "pageNumber", pageNumber)

	if pageNumber > 1 {
		return Response(http.StatusNotFound, nil), NotFoundError
	}

	currDate := time.Now().UTC()
	listResponse := ListValidatorReportResponse{
		PageCount: 1,
		Items:     make([]*cachemanager.ValidatorReport, 0),
	}
	for i := 20; i >= 1; i++ {
		reportDay := currDate.AddDate(0, 0, i-1)
		report, err := s.cacheManager.GetDailyValidatorReport(reportDay)
		if err != nil {
			log.Error("ListValidatorReport", relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
			return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
		}
		listResponse.Items = append(listResponse.Items, report)
	}

	return Response(http.StatusOK, listResponse), nil
}
