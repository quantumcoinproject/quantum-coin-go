package qcreadapi

import (
	"errors"
	"github.com/gorilla/mux"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"net/http"
	"strconv"
)

// ListStakingReport - List staking report
func (c *ReadApiAPIController) ListStakingReport(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListStakingReport", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListStakingReport", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)

	pageNumber := int64(-1)
	pageNumberParam := params["pageNumber"]
	var err error
	if len(pageNumberParam) > 0 {
		pageNumber, err = strconv.ParseInt(pageNumberParam, 10, 64)
		if err != nil {
			c.errorHandler(w, r, &ParsingError{"pageNumber", err}, nil)
			log.Error("ListStakingReport", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListStakingReport(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListStakingReport", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListStakingReport ok", "requestId", requestId)
}
