package cachemanager

import (
	"encoding/json"
	"fmt"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"strings"
)

const TokenDetailsKey = "erc20-%s"

func (c *CacheManager) getTokenDetailsInternal(contractAddress string) (*TokenDetails, error) {
	contractAddress = strings.ToLower(contractAddress)
	tokenDetails, ok := c.tokenMap[contractAddress]
	if ok {
		return tokenDetails, nil
	}

	key := fmt.Sprintf(TokenDetailsKey, contractAddress)

	db := c.cacheDb
	tokenBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	tokenDetails = &TokenDetails{}
	err = json.Unmarshal(tokenBlob, tokenDetails)
	if err != nil {
		return nil, err
	}

	c.tokenMap[contractAddress] = tokenDetails

	return tokenDetails, nil
}

func (c *CacheManager) GetTokenDetails(contractAddress string) (*GetTokenDetailsResponse, error) {
	contractAddress = strings.ToLower(contractAddress)

	key := fmt.Sprintf(TokenDetailsKey, contractAddress)

	db := c.cacheDb
	tokenBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	var tokenDetails TokenDetails
	err = json.Unmarshal(tokenBlob, &tokenDetails)
	if err != nil {
		return nil, err
	}

	return &GetTokenDetailsResponse{
		Result: tokenDetails,
	}, nil
}

func (c *CacheManager) putTokenInDb(tokenDetails *TokenDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(tokenDetails)
	if err != nil {
		return err
	}

	contractAddress := strings.ToLower(tokenDetails.ContractAddress)
	key := fmt.Sprintf(TokenDetailsKey, contractAddress)
	keyBlob := []byte(key)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	c.tokenMap[contractAddress] = tokenDetails

	log.Info("putTokenInDb", "contractAddress", contractAddress)

	return nil
}
