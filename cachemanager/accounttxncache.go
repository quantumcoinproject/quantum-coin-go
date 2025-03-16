package cachemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"strings"
)

var AccountTxnCountKey = "account-txn-count-%s"                  //%s is account address
var AccountTransactionPageKey = "account-transaction-list-%s-%d" //%s is account address, %d is page number

func getAccountTransactionPageKey(address string, pageCount uint64) []byte {
	pageKey := fmt.Sprintf(AccountTransactionPageKey, strings.ToLower(address), pageCount)
	return []byte(pageKey)
}

func getAccountTxnCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTxnCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTxnCount(address string) (uint64, error) {
	accountTxnCountKey, keyBlob := getAccountTxnCountKey(address)
	accountTxnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTxnCount not found", "address", address, "accountTxnCountKey", accountTxnCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTxnCount cacheDb.Get address", "address", address, "accountTxnCountKey", accountTxnCountKey, "error", err)
			return 0, err
		}
	} else {
		txnCount := common.BytesToUint64(accountTxnCountBlob)
		log.Info("getAccountTxnCount", "address", address, "accountTxnCountKey", accountTxnCountKey, "txnCount", txnCount)
		return txnCount, nil
	}
}

func (c *CacheManager) putAccountTxnCount(address string, txnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTxnCountKey, keyBlob := getAccountTxnCountKey(address)
	log.Info("putAccountTxnCount", "address", address, "accountTxnCountKey", accountTxnCountKey, "txnCount", txnCount)

	blob := common.Uint64ToBytes(txnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTxnCount address", "error", err, "address", address, "txnCount", txnCount)
		return err
	}

	return nil
}

func (c *CacheManager) ListTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTransactionsResponse, error) {
	listResponse := ListAccountTransactionsResponse{}
	address := accountAddress.HexLower()

	var pageCount uint64
	accountTxnCount, err := c.getAccountTxnCount(address)
	if err != nil {
		return ListAccountTransactionsResponse{}, err
	}
	if accountTxnCount%PageSize == 0 {
		pageCount = accountTxnCount / PageSize
	} else {
		pageCount = (accountTxnCount / PageSize) + 1
	}

	if pageCount == 0 {
		return ListAccountTransactionsResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListTransactionByAccount", "address", address, "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "accountTxnCount", accountTxnCount)
	if pageNumber > pageCount {
		return ListAccountTransactionsResponse{PageCount: pageCount}, nil
	}

	pageKey := fmt.Sprintf(AccountTransactionPageKey, address, pageNumber)
	accountTxnPageKey := []byte(pageKey)
	log.Info("cache get", "key", pageKey)

	accountTransactionListBlob, err := c.cacheDb.Get(accountTxnPageKey)
	if err != nil {
		log.Error("ListTransactionByAccount cacheDb.Get fromAccountTxnPageKey", "error", err)
		return ListAccountTransactionsResponse{}, err
	}
	var accountTransactionList AccountTransactionList
	err = json.Unmarshal(accountTransactionListBlob, &accountTransactionList)
	if err != nil {
		log.Error("ListTransactionByAccount json.Unmarshal accountTransactionListBlob", "error", err)
		return ListAccountTransactionsResponse{}, err
	}

	if strings.ToLower(accountTransactionList.Address) != address {
		log.Error("unexpected address accountTransactionList.Address", "address", address, "accountTransactionList.Address", accountTransactionList.Address)
		return ListAccountTransactionsResponse{}, errors.New("unexpected address accountTransactionList.Address")
	}

	listResponse.Items = accountTransactionList.Transactions
	listResponse.PageCount = pageCount

	return listResponse, nil
}
