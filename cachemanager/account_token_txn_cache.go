package cachemanager

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"strings"
)

const AccountTokenTxnCountKey = "account-token-txn-count-%s"          //%s is account address
const AccountTokenTransactionPageKey = "account-token-txn-list-%s-%d" //%s is account address,%d is page number

func getAccountTokenTxnCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTokenTxnCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTokenTxnCount(address string) (uint64, error) {
	accountTokenTxnCountKey, keyBlob := getAccountTokenTxnCountKey(address)
	accountTokenTxnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTokenTxnCount not found", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTokenTxnCount cacheDb.Get address", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "error", err)
			return 0, err
		}
	} else {
		tokenTxnCount := common.BytesToUint64(accountTokenTxnCountBlob)
		log.Info("getAccountTokenTxnCount", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "tokenTxnCount", tokenTxnCount)
		return tokenTxnCount, nil
	}
}

func (c *CacheManager) putAccountTokenTxnCount(address string, tokenTxnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTokenTxnCountKey, keyBlob := getAccountTokenTxnCountKey(address)
	log.Info("putAccountTokenTxnCount", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "tokenTxnCount", tokenTxnCount)

	blob := common.Uint64ToBytes(tokenTxnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTokenTxnCount address", "error", err, "address", address, "tokenTxnCount", tokenTxnCount)
		return err
	}

	return nil
}

func (c *CacheManager) ListTokenTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTokenTransactionsResponse, error) {
	return ListAccountTokenTransactionsResponse{}, nil
}
