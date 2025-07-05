package qcreadapi

import (
	"errors"
	"github.com/gorilla/mux"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"net/http"
	"strconv"
)

// GetLatestBlockDetails - Get latest block details
func (c *ReadApiAPIController) GetLatestBlockDetails(w http.ResponseWriter, r *http.Request) {
	log.Info("GetLatestBlockDetails")
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetLatestBlockDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetLatestBlockDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetLatestBlockDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	result, err := c.service.GetLatestBlockDetails(r.Context())
	// If an error occurred, encode the error with the status code
	if err != nil {
		log.Error("GetLatestBlockDetails", "requestId", requestId, "error", err)
		c.errorHandler(w, r, err, &result)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetLatestBlockDetails ok", "requestId", requestId)
}

// GetBlockDetails - Get block details
func (c *ReadApiAPIController) GetBlockDetails(w http.ResponseWriter, r *http.Request) {
	log.Info("GetBlockDetails")
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetBlockDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetBlockDetails OPTIONS", "requestId", requestId)
		return
	}

	blockNumber := int64(-1)
	params := mux.Vars(r)
	blockNumberParam := params["blockNumber"]
	var err error
	if len(blockNumberParam) > 0 {
		blockNumber, err = strconv.ParseInt(blockNumberParam, 10, 64)
		if err != nil {
			c.errorHandler(w, r, &ParsingError{"blockNumber", err}, nil)
			log.Error("GetBlockDetails", "requestId", requestId, "error", "invalid blockNumber")
			return
		}
		if blockNumber <= 0 {
			c.errorHandler(w, r, &ParsingError{"blockNumber", err}, nil)
			log.Error("GetBlockDetails", "requestId", requestId, "error", "invalid blockNumber value")
			return
		}
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetBlockDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	result, err := c.service.GetBlockDetails(r.Context(), blockNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		log.Error("GetBlockDetails", "requestId", requestId, "error", err)
		c.errorHandler(w, r, err, &result)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetBlockDetails ok", "requestId", requestId)
}

// ListBlocks - List blocks
func (c *ReadApiAPIController) ListBlocks(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListBlocks", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListBlocks", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
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
			log.Error("ListBlocks", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListBlocks(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListBlocks", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListBlocks ok", "requestId", requestId)
}

// ListBlockReport - List block report
func (c *ReadApiAPIController) ListBlockReport(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("ListBlockReport", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("ListBlockReport", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
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
			log.Error("ListBlockReport", "requestId", requestId, "error", "invalid pageNumber")
			return
		}
		if pageNumber <= 0 {
			pageNumber = -1
		}
	}

	result, err := c.service.ListBlockReport(r.Context(), pageNumber)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("ListBlockReport", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("ListBlockReport ok", "requestId", requestId)
}
