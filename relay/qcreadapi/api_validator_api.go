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

// ListValidators - List blocks
func (c *ReadApiAPIController) ListValidators(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListValidators", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListValidators", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
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
			log.Error("ListValidators", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListValidators(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListValidators", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListValidators ok", "requestId", requestId)
}

// GetValidatorDetails - Get Validator Details
func (c *ReadApiAPIController) GetValidatorDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetValidatorDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetValidatorDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetValidatorDetails OPTIONS", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	hashParam := params["hash"]
	if hashParam == "" {
		c.errorHandler(w, r, &RequiredError{"hash"}, nil)
		log.Error("GetValidatorDetails hashParam is empty", "requestId", requestId)
		return
	}

	if !common.IsHexAddressDeep(hashParam) {
		log.Error(relay.MsgAddress, relay.MsgHash, hashParam, relay.MsgError, relay.ErrInvalidHash, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"hash", errors.New("Invalid hash")}, nil)
		return
	}

	result, err := c.service.GetValidatorDetails(r.Context(), hashParam)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("GetValidatorDetails", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetValidatorDetailsDetails ok", "requestId", requestId)
}

// ListValidatorReport - List validator report
func (c *ReadApiAPIController) ListValidatorReport(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListValidatorReport", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListValidatorReport", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
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
			log.Error("ListValidatorReport", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListValidatorReport(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListValidatorReport", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListValidatorReport ok", "requestId", requestId)
}

// ListSpecificValidatorReport - List account transactions
func (c *ReadApiAPIController) ListSpecificValidatorReport(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListSpecificValidatorReport", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListSpecificValidatorReport", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	addressParam := params["address"]
	if addressParam == "" {
		c.errorHandler(w, r, &RequiredError{"address"}, nil)
		log.Error("ListSpecificValidatorReport address is empty", "requestId", requestId)
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
			log.Error("ListSpecificValidatorReport", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListSpecificValidatorReport(r.Context(), addressParam, pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListSpecificValidatorReport", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListSpecificValidatorReport ok", "requestId", requestId)
}
