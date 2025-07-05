package qcreadapi

import (
	"errors"
	"github.com/gorilla/mux"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	relay "github.com/quantumcoinproject/quantum-coin-go/relayv2"
	"net/http"
)

// GetTokenDetails - Get account details
func (c *ReadApiAPIController) GetTokenDetails(w http.ResponseWriter, r *http.Request) {
	requestId := ""
	if r.Header != nil {
		requestId = r.Header.Get(REQUEST_ID_HEADER_NAME)
	}
	if len(requestId) > 0 {
		log.Info("GetTokenDetails", "requestId", requestId)
	}

	c.setupCORS(&w, r)
	if (*r).Method == "OPTIONS" {
		log.Info("GetTokenDetails OPTIONS", "requestId", requestId)
		return
	}

	if c.authorize(r) == false {
		result := Response(http.StatusUnauthorized, nil)
		// If no error, encode the body and the result code
		_ = EncodeJSONResponse(result.Body, &result.Code, w)

		log.Error("GetTokenDetails", "requestId", requestId, "error", UNAUTHORIZED_ERROR_MSG)

		c.errorHandler(w, r, errors.New(UNAUTHORIZED_ERROR_MSG), &result)
		return
	}

	params := mux.Vars(r)
	contractAddressParam := params["contractAddress"]
	if contractAddressParam == "" {
		c.errorHandler(w, r, &RequiredError{"contractAddress"}, nil)
		log.Error("GetTokenDetails", "requestId", requestId, "error", "contractAddress is empty")
		return
	}

	if !common.IsHexAddressDeep(contractAddressParam) {
		log.Error(relay.MsgContractAddress, relay.MsgContractAddress, contractAddressParam, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest, "requestId", requestId)
		c.errorHandler(w, r, &ParsingError{"contractAddress", errors.New("Invalid contractAddress")}, nil)
		return
	}

	log.Info("GetTokenDetails", "requestId", requestId, "contractAddressParam", contractAddressParam)
	result, err := c.service.GetTokenDetails(r.Context(), contractAddressParam)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		log.Error("GetTokenDetails", "requestId", requestId, "error", err)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)

	log.Info("GetTokenDetails ok", "requestId", requestId)
}
