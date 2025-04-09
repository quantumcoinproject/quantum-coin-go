package cachemanager

import (
	"encoding/json"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
)

const SummaryKey = "summary"

func (c *CacheManager) getSummaryFromDb() (*BlockchainDetails, error) {
	db := c.cacheDb
	summaryBlob, err := db.Get([]byte(SummaryKey))
	if err != nil {
		return nil, err
	}

	var summary BlockchainDetails
	err = json.Unmarshal(summaryBlob, &summary)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

func (c *CacheManager) putSummary(summary *BlockchainDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	keyBlob := []byte(SummaryKey)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) GetBlockchainDetails() (GetBlockchainDetailsResponse, error) {
	if c.enableExtendedApis == false {
		return GetBlockchainDetailsResponse{}, errors.New("enableExtendedApis is false")
	}
	getResponse := GetBlockchainDetailsResponse{}
	details, err := c.getSummaryFromDb()
	if err != nil {
		return getResponse, err
	}

	getResponse.Result = *details

	return getResponse, nil
}
