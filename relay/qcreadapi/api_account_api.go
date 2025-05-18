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

// GetAccountDetails - Get account details
func (c *ReadApiAPIController) GetAccountDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetAccountDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetAccountDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetAccountDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)

		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("GetAccountDetails", "requestId", requestId, "error", "address is empty")
		return
	}

	if !common.IsHexAddressDeep(addressParam) {
		log.Error(relay.MsgAddress, relay.MsgAddress, addressParam, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"address", errors.New("Invalid address")}, nil)
		return
	}

	log.Info("GetAccountDetails", "requestId", requestId, "addressParam", addressParam)
	result, err := c.service.GetAccountDetails(r.Context(), addressParam)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("GetAccountDetails", "requestId", requestId, "error", err)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetAccountDetails ok", "requestId", requestId)
}

// ListAccountTransactions - List account transactions
func (c *ReadApiAPIController) ListAccountTransactions(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListAccountTransactions", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListAccountTransactions", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("ListAccountTransactions address is empty", "requestId", requestId)
		return
	}

	if !common.IsHexAddressDeep(addressParam) {
		log.Error(relay.MsgAddress, relay.MsgAddress, addressParam, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"address", errors.New("Invalid address")}, nil)
		return
	}

	pageNumber := int64(-1)
	pageNumberParam := params["pageNumber"]
	var err error
	if len(pageNumberParam) > 0 {
		pageNumber, err = strconv.ParseInt(pageNumberParam, 10, 64)
		if err != nil {
			c.errorHandler(w, r, &ParsingError{"pageNumber", err}, nil)
			log.Error("ListAccountTransactions", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListAccountTransactions(r.Context(), addressParam, pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListAccountTransactions", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListAccountTransactions ok", "requestId", requestId)
}

// ListAccountPendingTransactions - List account pending transactions
func (c *ReadApiAPIController) ListAccountPendingTransactions(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListAccountPendingTransactions", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListAccountPendingTransactions", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("ListAccountPendingTransactions address is empty", "requestId", requestId)
		return
	}

	if !common.IsHexAddressDeep(addressParam) {
		log.Error(relay.MsgAddress, relay.MsgAddress, addressParam, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"address", errors.New("Invalid address")}, nil)
		return
	}

	pageNumber := int64(-1)
	pageNumberParam := params["pageNumber"]
	var err error
	if len(pageNumberParam) > 0 {
		pageNumber, err = strconv.ParseInt(pageNumberParam, 10, 64)
		if err != nil {
			c.errorHandler(w, r, &ParsingError{"pageNumber", err}, nil)
			log.Error("ListAccountPendingTransactions", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListAccountPendingTransactions(r.Context(), addressParam, pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListAccountPendingTransactions", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListAccountPendingTransactions ok", "requestId", requestId)
}
