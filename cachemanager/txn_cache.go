package cachemanager

import (
	"encoding/json"
	"fmt"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"strings"
)

const TxnKey = "txn-%s" //%s is txn hash

func getTransactionKey(hash string) []byte {
	pageKey := fmt.Sprintf(TxnKey, strings.ToLower(hash))
	return []byte(pageKey)
}

func (c *CacheManager) getTransactionFromDb(hash string) (*TransactionDetails, error) {
	keyBlob := getTransactionKey(hash)
	blob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getTransactionFromDb not found", "hash", hash)
			return nil, nil
		} else {
			log.Error("getTransactionFromDb", "hash", hash, "error", err)
			return nil, err
		}
	}

	item := TransactionDetails{}
	err = json.Unmarshal(blob, &item)
	if err != nil {
		log.Error("getBlockFromDb", "error", err, "hash", hash, "error", err)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putTransactionInDb(item *TransactionDetails, batch *ethdb.Batch) error {
	txnBatch := *batch
	keyBlob := getTransactionKey(item.Hash)
	log.Info("putBlockInDb", "Hash", item.Hash)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putBlockInDb", "error", err, "Hash", item.Hash)
		return err
	}

	return nil
}
