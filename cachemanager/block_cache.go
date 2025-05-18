package cachemanager

import (
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"time"
)

const DailyBlockReportKey = "daily-block-report-%s"

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

func (c *CacheManager) GetBlockDetails(blockNumber uint64) (*Block, error) {
	key, keyBlob := getBlockKey(blockNumber)
	blob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("GetBlockDetails not found", "blockNumber", blockNumber, "key", key)
			return nil, nil
		} else {
			log.Error("GetBlockDetails", "blockNumber", blockNumber, "key", key, "error", err)
			return nil, err
		}
	}

	item := Block{}
	err = json.Unmarshal(blob, &item)
	if err != nil {
		log.Error("GetBlockDetails", "error", err, "blockNumber", blockNumber, "key", key, "error", err)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putBlockInDb(item *Block, batch *ethdb.Batch) error {
	txnBatch := *batch
	key, keyBlob := getBlockKey(uint64(item.BlockNumber))
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

func getDailyBlockReportKey(date string) (key string, blob []byte) {
	key = fmt.Sprintf(DailyBlockReportKey, date)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getDailyBlockReportKey(reportTime time.Time) (*BlockReport, error) {
	key, keyBlob := getDailyBlockReportKey(reportTime.Format("2006-02-01"))
	log.Debug("getDailyBlockReport", "key", key, "reportTime", reportTime)

	itemBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		log.Error("getDailyBlockReport cacheDb.Get", "error", err, "reportTime", reportTime)
		return nil, err
	}
	var item BlockReport
	err = json.Unmarshal(itemBlob, &item)
	if err != nil {
		log.Error("getDailyBlockReport json.Unmarshal", "error", err, "reportTime", reportTime)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) GetDailyBlockReport(reportTime time.Time) (*BlockReport, error) {
	key, keyBlob := getDailyBlockReportKey(reportTime.Format("2006-02-01"))
	log.Debug("GetDailyBlockReport", "key", key, "reportTime", reportTime)

	itemBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		log.Error("GetDailyBlockReport cacheDb.Get", "error", err, "reportTime", reportTime)
		return nil, err
	}
	var item BlockReport
	err = json.Unmarshal(itemBlob, &item)
	if err != nil {
		log.Error("GetDailyBlockReport json.Unmarshal", "error", err, "reportTime", reportTime)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) incrementDailyBlockDetailsInDb(reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	var item *BlockReport
	var err error
	item, err = c.GetDailyBlockReport(reportTime)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			item = &BlockReport{
				TotalBlocks: 1,
				ReportDate:  reportTime.Unix(),
			}
		} else {
			log.Error("putDailyTransactionDetailsInDb getDailyTransactionReport", "error", err, "reportTime", reportTime)
			return err
		}
	} else {
		item.TotalBlocks = item.TotalBlocks + 1
	}

	key, keyBlob := getDailyBlockReportKey(reportTime.Format("2006-02-01"))
	log.Info("getDailyBlockReportKey", "key", key, "reportTime", reportTime)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putDailyBlockDetailsInDb", "error", err, "key", key, "reportTime", reportTime)
		return err
	}

	return nil
}
