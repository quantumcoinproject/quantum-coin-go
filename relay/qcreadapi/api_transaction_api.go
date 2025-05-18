package qcreadapi

import (
	"errors"
	"github.com/gorilla/mux"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/relay"
	"net/http"
	"strconv"
)

// ListTransactions - List transactions
func (c *ReadApiAPIController) ListTransactions(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListTransactions", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListTransactions", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
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
			log.Error("ListTransactions", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListTransactions(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListTransactions", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListTransactions ok", "requestId", requestId)
}

// GetTransaction - Get Transaction
func (c *ReadApiAPIController) GetTransactionDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetTransactionDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetTransactionDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetTransactionDetails OPTIONS", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	hashParam := params["hash"]
	if hashParam == "" {
		c.errorHandler(w, r, &RequiredError{"hash"}, nil)
		log.Error("GetTransactionDetails hashParam is empty", "requestId", requestId)
		return
	}

	if !common.IsHexAddressDeep(hashParam) {
		log.Error(relay.MsgAddress, relay.MsgHash, hashParam, relay.MsgError, relay.ErrInvalidHash, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"hash", errors.New("Invalid hash")}, nil)
		return
	}

	result, err := c.service.GetTransactionDetails(r.Context(), hashParam)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("GetTransactionDetails", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetTransactionDetails ok", "requestId", requestId)
}

// ListTransactionReport - List transaction report
func (c *ReadApiAPIController) ListTransactionReport(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListTransactionReport", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListTransactionReport", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
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
			log.Error("ListTransactionReport", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListTransactionReport(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListTransactionReport", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListTransactionReport ok", "requestId", requestId)
}
