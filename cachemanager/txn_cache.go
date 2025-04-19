package cachemanager

import (
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"strings"
	"time"
)

const TxnKey = "txn-%s" //%s is txn hash
const DailyTransactionReportKey = "daily-transaction-report-%s"

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
		log.Error("getTransactionFromDb", "error", err, "hash", hash, "error", err)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putTransactionInDb(item *TransactionDetails, batch *ethdb.Batch) error {
	txnBatch := *batch
	keyBlob := getTransactionKey(item.Hash)
	log.Info("putTransactionInDb", "Hash", item.Hash)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putTransactionInDb", "error", err, "Hash", item.Hash)
		return err
	}

	return nil
}

func getDailyTransactionReportKey(date string) (key string, blob []byte) {
	key = fmt.Sprintf(DailyTransactionReportKey, date)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getDailyTransactionReport(reportTime time.Time) (*TransactionReport, error) {
	key, keyBlob := getDailyTransactionReportKey(reportTime.Format("2006-02-01"))
	log.Debug("getDailyTransactionReport", "key", key, "reportTime", reportTime)

	itemBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		log.Error("getDailyTransactionReport cacheDb.Get", "error", err, "reportTime", reportTime)
		return nil, err
	}
	var item TransactionReport
	err = json.Unmarshal(itemBlob, &item)
	if err != nil {
		log.Error("getDailyTransactionReport json.Unmarshal", "error", err, "reportTime", reportTime)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) incrementDailyTransactionDetailsInDb(blockTxnCount uint64, reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	var item *TransactionReport
	var err error
	item, err = c.getDailyTransactionReport(reportTime)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			item = &TransactionReport{
				TotalTransactions: blockTxnCount,
				ReportDate:        reportTime.Unix(),
			}
		} else {
			log.Error("putDailyTransactionDetailsInDb getDailyTransactionReport", "error", err, "reportTime", reportTime)
			return err
		}
	} else {
		item.TotalTransactions = item.TotalTransactions + blockTxnCount
	}

	key, keyBlob := getDailyTransactionReportKey(reportTime.Format("2006-02-01"))
	log.Info("putDailyTransactionDetailsInDb", "key", key, "reportTime", reportTime)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putDailyTransactionDetailsInDb", "error", err, "key", key, "reportTime", reportTime)
		return err
	}

	return nil
}
