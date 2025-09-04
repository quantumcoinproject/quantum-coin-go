package cachemanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"math/big"
	"time"
)

const DailyStakingReportKey = "daily-staking-report-%s" //%s is epoch time at 11:59:59:00

func (c *CacheManager) refreshStakingDetails(blockNum *big.Int, batch *ethdb.Batch) (*big.Int, error) {
	stakingContractBalance, err := c.client.BalanceAt(context.Background(), staking.STAKING_CONTRACT_ADDRESS, blockNum)
	if err != nil {
		log.Error("CacheManager BalanceAt staking contract", "error", err)
		return nil, err
	}

	reportTime := time.Now().UTC()
	daily := &StakingReport{
		TotalStakedCoins: stakingContractBalance.String(),
		ReportDate:       reportTime.Unix(),
	}
	err = c.putDailyStakingDetailsInDb(daily, reportTime, batch)

	return stakingContractBalance, nil
}

func getDailyStakingReportKey(date string) (key string, blob []byte) {
	key = fmt.Sprintf(DailyStakingReportKey, date)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) GetDailyStakingReport(reportTime time.Time) (*StakingReport, error) {
	key, keyBlob := getDailyStakingReportKey(reportTime.Format("2006-02-01"))
	log.Debug("getDailyStakingReportKey", "key", key, "reportTime", reportTime)

	itemBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		log.Error("GetDailyStakingReport cacheDb.Get", "error", err, "reportTime", reportTime)
		return nil, err
	}
	var item StakingReport
	err = json.Unmarshal(itemBlob, &item)
	if err != nil {
		log.Error("GetDailyStakingReport", "error", err, "reportTime", reportTime)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putDailyStakingDetailsInDb(item *StakingReport, reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	key, keyBlob := getDailyStakingReportKey(reportTime.Format("2006-02-01"))
	log.Debug("putDailyStakingDetailsInDb", "key", key)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putDailyStakingDetailsInDb", "error", err, "key", key)
		return err
	}

	return nil
}
