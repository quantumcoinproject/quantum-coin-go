package qcreadapi

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"net/http"
)

func (c *ReadApiAPIController) GetBlockchainDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetBlockchainDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetBlockchainDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetBlockchainDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	result, err := c.service.GetBlockchainDetails(r.Context())
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("GetBlockchainDetails", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetBlockchainDetails ok", "requestId", requestId)
}

func (c *ReadApiAPIController) QueryDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("QueryDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("QueryDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)
		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	log.Info("api", "url", r.URL, "q", r.URL.Query().Get("q"))
	queryTerm := r.URL.Query().Get("q")

	result, err := c.service.QueryDetails(r.Context(), queryTerm)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("QueryDetails", "requestId", requestId, "error", err)
		return
	}

	// If no error, encode the body and the result code
	_ = EncodeTextResponse(result.Body, &result.Code, w)

	log.Info("QueryDetails ok", "requestId", requestId)
}
