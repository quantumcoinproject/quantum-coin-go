package cachemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const TxnCountKey = "txn-count"
const TransactionPageKey = "transaction-list-%d" //%d is page number

func getTransactionPageKey(pageCount uint64) []byte {
	pageKey := fmt.Sprintf(TransactionPageKey, pageCount)
	return []byte(pageKey)
}

func getTxnCountKey() (key string, blob []byte) {
	key = TxnCountKey
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getTxnCount() (uint64, error) {
	txnCountKey, keyBlob := getTxnCountKey()
	txnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getTxnCount not found", "txnCountKey", txnCountKey)
			return 0, nil
		} else {
			log.Error("getTxnCount cacheDb.Get", "txnCountKey", txnCountKey, "error", err)
			return 0, err
		}
	} else {
		txnCount := common.BytesToUint64(txnCountBlob)
		log.Info("getTxnCount", "txnCountKey", txnCountKey, "txnCount", txnCount)
		return txnCount, nil
	}
}

func (c *CacheManager) putTxnCount(txnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	txnCountKey, keyBlob := getTxnCountKey()
	log.Info("putTxnCount", "txnCountKey", txnCountKey, "txnCount", txnCount)

	blob := common.Uint64ToBytes(txnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putTxnCount", "error", err, "txnCount", txnCount)
		return err
	}

	return nil
}

func (c *CacheManager) processTransactions(txnList *[]*TransactionDetails, batch *ethdb.Batch) error {
	log.Debug("CacheManager processTransactions")
	txnBatch := *batch
	var txnCount uint64
	var err error

	txnCount, err = c.getTxnCount()
	if err != nil {
		return err
	}
	newTxnCount := txnCount + 1
	var transactionList TransactionList

	log.Debug("CacheManager processTransactions", "txnCount", txnCount, "transaction count in block", len(*txnList))

	if newTxnCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
		transactionList.Transactions = make([]TransactionCompact, 0)
		log.Debug("CacheManager processTransactions", "newTxnCount", newTxnCount)
	} else {
		//Load current state form the cache
		txnPageCount := getPageCount(newTxnCount)
		txnPageKey := getTransactionPageKey(txnPageCount)

		log.Debug("CacheManager processTransactions loading from cache", "newTxnCount", newTxnCount, "txnPageCount", txnPageCount)

		TransactionListBlob, err := c.cacheDb.Get(txnPageKey)
		if err != nil {
			log.Error("CacheManager cacheDb.Get TxnPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(TransactionListBlob, &transactionList)
		if err != nil {
			log.Error("CacheManager json.Unmarshal TransactionListBlob", "error", err)
			return err
		}

		if transactionList.Transactions == nil {
			return errors.New("unexpected transactions is nul")
		}

		if len(transactionList.Transactions) != int(txnCount%PageSize) {
			log.Error("CacheManager unexpected transactions count", "actual", len(transactionList.Transactions), "expected", int(txnCount%PageSize), "txnCount", txnCount)
			return errors.New("unexpected transactions count")
		}
	}

	for i, txn := range *txnList {
		log.Trace("CacheManager processTransactions", "txn", txn.Hash)
		err = c.putTransactionInDb(txn, batch)
		if err != nil {
			log.Error("putTransactionInDb", "error", err, "hash", txn.Hash)
			return err
		}

		atxn := transactionCompactFromTransaction(txn)
		transactionList.Transactions = append([]TransactionCompact{atxn}, transactionList.Transactions...) //prepend for backward compat

		if len(transactionList.Transactions) == int(PageSize) || i == len(*txnList)-1 {
			TransactionListBlob, err := json.Marshal(transactionList)
			if err != nil {
				log.Error("CacheManager json.Marshal TransactionListBlob", "error", err)
				return err
			}

			runningTxnCount := txnCount + uint64(i) + 1
			txnPageCount := getPageCount(runningTxnCount)
			txnPageKey := getTransactionPageKey(txnPageCount)

			err = txnBatch.Put(txnPageKey, TransactionListBlob)
			if err != nil {
				log.Error("CacheManager txnBatch.Put TransactionListBlob", "error", err)
				return err
			}
			log.Info("CacheManager txnBatch.Put", "runningTxnCount", runningTxnCount, "txnPageCount", txnPageCount)
			transactionList.Transactions = make([]TransactionCompact, 0) //reset
		}
	}

	log.Trace("CacheManager putTxnCount txnCount", "txnCount", txnCount)
	txnCount = txnCount + uint64(len(*txnList))
	err = c.putTxnCount(txnCount, batch)
	if err != nil {
		return err
	}

	log.Info("CacheManager inserted  txn list", "txnCount", txnCount, "txnPageCount", getPageCount(txnCount), "txnCountInBlock", len(*txnList))

	return nil
}

func (c *CacheManager) ListTransactions(pageNumberInput int64) (ListTransactionsResponse, error) {
	listResponse := ListTransactionsResponse{}

	var pageCount uint64
	TxnCount, err := c.getTxnCount()
	if err != nil {
		return ListTransactionsResponse{}, err
	}
	if TxnCount%PageSize == 0 {
		pageCount = TxnCount / PageSize
	} else {
		pageCount = (TxnCount / PageSize) + 1
	}

	if pageCount == 0 {
		return ListTransactionsResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListTransactionBy", "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "TxnCount", TxnCount)
	if pageNumber > pageCount {
		return ListTransactionsResponse{PageCount: pageCount}, nil
	}

	pageKey := TransactionPageKey
	TxnPageKey := []byte(pageKey)
	log.Info("cache get", "key", pageKey)

	TransactionListBlob, err := c.cacheDb.Get(TxnPageKey)
	if err != nil {
		log.Error("ListTransactionBy cacheDb.Get fromTxnPageKey", "error", err)
		return ListTransactionsResponse{}, err
	}
	var transactionList TransactionList
	err = json.Unmarshal(TransactionListBlob, &transactionList)
	if err != nil {
		log.Error("ListTransactionBy json.Unmarshal TransactionListBlob", "error", err)
		return ListTransactionsResponse{}, err
	}

	listResponse.Items = transactionList.Transactions
	listResponse.PageCount = pageCount

	return listResponse, nil
}
