package cachemanager

import (
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

func (c *CacheManager) putLastBlockNumberInDb(blockNumber uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	blockKey := []byte(LastBlockKey)
	err := txnBatch.Put(blockKey, common.Uint64ToBytes(blockNumber))
	if err != nil {
		log.Error("putLastBlockNumberInDb txnBatch.Put", "error", err, "blockKey", blockKey)
		return err
	}
	return nil
}

func (c *CacheManager) getLastBlockNumberFromDb() (uint64, error) {
	db := c.cacheDb
	mySlice, err := db.Get([]byte(LastBlockKey))
	if err != nil {
		return uint64(0), err
	}

	blockNumber := common.BytesToUint64(mySlice)

	return blockNumber, nil
}

func getBlockKey(blockNumber uint64) (key string, blob []byte) {
	key = fmt.Sprintf(BlockDetailsKey, blockNumber)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getBlockFromDb(blockNumber uint64) (*Block, error) {
	key, keyBlob := getBlockKey(blockNumber)
	blob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getBlockFromDb not found", "blockNumber", blockNumber, "key", key)
			return nil, nil
		} else {
			log.Error("getBlockFromDb", "blockNumber", blockNumber, "key", key, "error", err)
			return nil, err
		}
	}

	item := Block{}
	err = json.Unmarshal(blob, &item)
	if err != nil {
		log.Error("getBlockFromDb", "error", err, "blockNumber", blockNumber, "key", key, "error", err)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putBlockInDb(item *Block, batch *ethdb.Batch) error {
	txnBatch := *batch
	key, keyBlob := getBlockKey(item.Number)
	log.Info("putBlockInDb", "key", key)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putBlockInDb", "error", err, "key", key)
		return err
	}

	return nil
}
