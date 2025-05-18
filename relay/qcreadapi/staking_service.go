package qcreadapi

import (
	"context"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/cachemanager"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/relay"
	"net/http"
	"time"
)

type ListStakingReportResponse struct {
	PageCount int64                         `json:"pageCount,omitempty"`
	Items     []*cachemanager.StakingReport `json:"items,omitempty"`
}

// ListStakingReport - List validator report
func (s *ReadApiAPIService) ListStakingReport(ctx context.Context, pageNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleListAccountTransactions)

	duration := time.Now().Sub(startTime)

	log.Info("ListStakingReport", relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent, "pageNumber", pageNumber)

	if pageNumber > 1 {
		return Response(http.StatusNotFound, nil), NotFoundError
	}

	currDate := time.Now().UTC()
	listResponse := ListStakingReportResponse{
		PageCount: 1,
		Items:     make([]*cachemanager.StakingReport, 0),
	}
	for i := 20; i >= 1; i++ {
		reportDay := currDate.AddDate(0, 0, i-1)
		report, err := s.cacheManager.GetDailyStakingReport(reportDay)
		if err != nil {
			log.Error("ListStakingReport", relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
			return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
		}
		listResponse.Items = append(listResponse.Items, report)
	}

	return Response(http.StatusOK, listResponse), nil
}
