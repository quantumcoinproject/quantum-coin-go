package qcreadapi

import (
	"context"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/cachemanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	relay "github.com/quantumcoinproject/quantum-coin-go/relayv2"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
	"math/big"
	"net/http"
	"time"
)

type AccountDetailsResponse struct {
	Result *cachemanager.AccountDetails `json:"result,omitempty"`
}

// GetAccountDetails - Get account details
func (s *ReadApiAPIService) GetAccountDetails(ctx context.Context, address string) (ImplResponse, error) {

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

	var balance *hexutil.Big
	err = client.CallContext(ctx, &balance, "eth_getBalance", common.HexToAddress(address), "latest")
	if err != nil {
		log.Error(relay.MsgBalance, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	var nonce *hexutil.Big
	err = client.CallContext(ctx, &nonce, "eth_getTransactionCount", common.HexToAddress(address), "latest")
	if err != nil {
		log.Error(relay.MsgNonce, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	var blockNumber *hexutil.Uint64
	err = client.CallContext(ctx, &blockNumber, "eth_blockNumber")
	if err != nil {
		log.Error(relay.MsgBlockNumber, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
		return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
	}

	accountBalance, err := hexutil.DecodeBig(balance.String())
	if err != nil {
		log.Error(relay.MsgBalance, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}

	accountNonce, err := hexutil.DecodeBig(nonce.String())
	if err != nil {
		log.Error(relay.MsgNonce, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}

	latestBlockNumber, err := hexutil.DecodeBig(blockNumber.String())
	if err != nil {
		log.Error(relay.MsgBlockNumber, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}

	accDetailsCache, err := s.cacheManager.GetAccountDetails(address)
	if err != nil {
		log.Error(relay.MsgBlockNumber, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), errors.New(err.Error())
	}

	accDetailsCache.Balance = accountBalance.String()
	accDetailsCache.Nonce = uint64(accountNonce.Int64())
	accDetailsCache.BlockNumber = latestBlockNumber.Int64()
	accDetailsResponse := AccountDetailsResponse{
		Result: accDetailsCache,
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleAccountDetails, relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

	return Response(http.StatusOK, accDetailsResponse), nil
}

// ListAccountTransactions - List account transactions
func (s *ReadApiAPIService) ListAccountTransactions(ctx context.Context, address string, pageNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleListAccountTransactions)

	if common.IsHexAddressDeep(address) == false {
		return Response(http.StatusInternalServerError, nil), relay.ErrInvalidAddress
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleListAccountTransactions, relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	listResponse, err := s.cacheManager.ListTransactionsByAccount(common.HexToAddress(address), pageNumber)
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), InternalError
	}

	return Response(http.StatusOK, listResponse), nil
}

// ListAccountPendingTransactions - List account pending transactions
func (s *ReadApiAPIService) ListAccountPendingTransactions(ctx context.Context, address string, pageNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleListAccountPendingTransactions)

	if common.IsHexAddressDeep(address) == false {
		return Response(http.StatusInternalServerError, nil), relay.ErrInvalidAddress
	}

	duration := time.Now().Sub(startTime)

	log.Info(relay.InfoTitleListAccountPendingTransactions, relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	listResponse, err := s.cacheManager.ListPendingTransactionsByAccount(common.HexToAddress(address), pageNumber)
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), InternalError
	}

	return Response(http.StatusOK, listResponse), nil
}

// GetAccountTokenDetails - Get account token details
func (s *ReadApiAPIService) GetAccountTokenDetails(ctx context.Context, address string, contractAddress string) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleAccountDetails, relay.MsgDial, s.DpUrl)

	if !common.IsHexAddressDeep(address) {
		log.Error(relay.MsgAddress, relay.MsgAddress, address, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), relay.ErrInvalidAddress
	}

	if !common.IsHexAddressDeep(contractAddress) {
		log.Error(relay.MsgContractAddress, relay.MsgContractAddress, contractAddress, relay.MsgError, relay.ErrInvalidAddress, relay.MsgStatus, http.StatusBadRequest)
		return Response(http.StatusBadRequest, nil), relay.ErrInvalidAddress
	}

	var accountTokenDetails AccountTokenDetails
	var accountTokenDetailsResponse AccountTokenDetailsResponse

	if s.enableExtendedApis == true {
		details, err := s.cacheManager.GetAccountTokenDetails(address, contractAddress)
		if err != nil {
			log.Error(relay.MsgContractAddress, relay.MsgContractAddress, contractAddress, relay.MsgError, err.Error(), relay.MsgStatus, http.StatusBadRequest)
			if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
				return Response(http.StatusNotFound, nil), NotFoundError
			}
			return Response(http.StatusInternalServerError, nil), InternalError
		}
		accountTokenDetails = AccountTokenDetails{
			TokenBalance:    &details.TokenBalance,
			ContractAddress: &contractAddress,
		}
	} else { //always get balance from blockchain node
		var client *ethclient.Client

		client, err := ethclient.Dial(s.DpUrl)
		if err != nil {
			log.Error(relay.MsgDial, relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
			return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
		}
		defer client.Close()

		var balance *big.Int
		balance, err = client.GetAccountTokenBalance(common.HexToAddress(contractAddress), common.HexToAddress(address))
		if err != nil {
			log.Error("GetTokenBalance", relay.MsgError, errors.New(err.Error()), relay.MsgStatus, http.StatusInternalServerError)
			return Response(http.StatusInternalServerError, nil), errors.New(err.Error())
		}
		hexBalance := hexutil.EncodeBig(balance)

		accountTokenDetails = AccountTokenDetails{
			TokenBalance:    &hexBalance,
			ContractAddress: &contractAddress,
		}
	}

	duration := time.Now().Sub(startTime)
	log.Info(relay.InfoTitleAccountTokenDetails, relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusOK)

	accountTokenDetailsResponse = AccountTokenDetailsResponse{
		Result: accountTokenDetails,
	}

	return Response(http.StatusOK, accountTokenDetailsResponse), nil
}

// ListAccountTokens - List account tokens
func (s *ReadApiAPIService) ListAccountTokens(ctx context.Context, address string, pageNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleListAccountTransactions)

	if common.IsHexAddressDeep(address) == false {
		return Response(http.StatusInternalServerError, nil), relay.ErrInvalidAddress
	}

	duration := time.Now().Sub(startTime)

	log.Info("ListAccountTokens", relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	listResponse, err := s.cacheManager.ListTokensByAccount(common.HexToAddress(address), pageNumber)
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), InternalError
	}

	return Response(http.StatusOK, listResponse), nil
}

// ListAccountTokenTransactions - List account token transactions
func (s *ReadApiAPIService) ListAccountTokenTransactions(ctx context.Context, address string, pageNumber int64) (ImplResponse, error) {

	startTime := time.Now()

	log.Info(relay.InfoTitleListAccountTransactions)

	if common.IsHexAddressDeep(address) == false {
		return Response(http.StatusInternalServerError, nil), relay.ErrInvalidAddress
	}

	duration := time.Now().Sub(startTime)

	log.Info("ListTokenTransactionsByAccount", relay.MsgAddress, address, relay.MsgTimeDuration, duration, relay.MsgStatus, http.StatusNoContent)

	listResponse, err := s.cacheManager.ListTokenTransactionsByAccount(common.HexToAddress(address), pageNumber)
	if err != nil {
		if err.Error() == cachemanager.LevelDbNoTFoundErrMsg {
			return Response(http.StatusNotFound, nil), NotFoundError
		}
		return Response(http.StatusInternalServerError, nil), InternalError
	}

	return Response(http.StatusOK, listResponse), nil
}
