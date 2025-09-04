package qcreadapi

import (
	"errors"
	"github.com/gorilla/mux"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	relay "github.com/quantumcoinproject/quantum-coin-go/relayv2"
	"net/http"
	"strconv"
)

// GetAccountTokenDetails - Get account details
func (c *ReadApiAPIController) GetAccountTokenDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetAccountTokenDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetAccountTokenDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetAccountTokenDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)

		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("GetAccountTokenDetails", "requestId", requestId, "error", "address is empty")
		return
	}

	if !common.IsHexAddressDeep(addressParam) {
		log.Error(relay.MsgAddress, relay.MsgAddress, addressParam, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"address", errors.New("Invalid address")}, nil)
		return
	}

	contractAddressParam := params["contractAddress"]
	if contractAddressParam == "" {
		c.errorHandler(w, r, &RequiredError{"contractAddress"}, nil)
		log.Error("GetAccountTokenDetails", "requestId", requestId, "error", "contractAddress is empty")
		return
	}

	if !common.IsHexAddressDeep(contractAddressParam) {
		log.Error(relay.MsgContractAddress, relay.MsgContractAddress, contractAddressParam, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"contractAddress", errors.New("Invalid contractAddress")}, nil)
		return
	}

	log.Info("GetAccountTokenDetails", "requestId", requestId, "addressParam", addressParam, "contractAddressParam", contractAddressParam)
	result, err := c.service.GetAccountTokenDetails(r.Context(), addressParam, contractAddressParam)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("GetAccountTokenDetails", "requestId", requestId, "error", err)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetAccountTokenDetails ok", "requestId", requestId)
}

// func (c *ReadApiAPIController) ListAccountTokens(w http.ResponseWriter, r *http.Request) { - List account transactions
func (c *ReadApiAPIController) ListAccountTokens(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListAccountTokens", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListAccountTokens", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("ListAccountTokens address is empty", "requestId", requestId)
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
			log.Error("ListAccountTokens", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListAccountTokens(r.Context(), addressParam, pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListAcountTokens", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListAccountTokens ok", "requestId", requestId)
}

// ListAccountTokenTransactions - List account transactions
func (c *ReadApiAPIController) ListAccountTokenTransactions(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListAccountTokenTransactions", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListAccountTokenTransactions", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("ListAccountTokenTransactions address is empty", "requestId", requestId)
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
			log.Error("ListAccountTokenTransactions", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListAccountTokenTransactions(r.Context(), addressParam, pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListAccountTokenTransactions", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListAccountTokenTransactions ok", "requestId", requestId)
}
