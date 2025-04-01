package cachemanager

import (
	"encoding/json"
	"github.com/QuantumCoinProject/qc/consensus/proofofstake"
	"github.com/QuantumCoinProject/qc/ethdb"
)

const ConversionKey = "conversion"

func (c *CacheManager) getConversionDetailsFromDb() (*proofofstake.ConversionDetails, error) {
	db := c.cacheDb
	keyBlob, err := db.Get([]byte(ConversionKey))
	if err != nil {
		return nil, err
	}

	var item proofofstake.ConversionDetails
	err = json.Unmarshal(keyBlob, &item)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putConversionDetailsInDb(item *proofofstake.ConversionDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}
	keyBlob := []byte(ConversionKey)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) updateConversionDetails(batch *ethdb.Batch) error {
	var conversionDetails proofofstake.ConversionDetails
	var err error

	err = c.putConversionDetailsInDb(&conversionDetails, batch)
	if err != nil {
		return err
	}

	return nil
}
