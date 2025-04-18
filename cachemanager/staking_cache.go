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

const DailyStakingKey = "staking-%s" //%s is epoch time at 11:59:59:00

func (c *CacheManager) refreshStakingDetails(blockNum *big.Int, batch *ethdb.Batch) (*big.Int, error) {
	stakingContractBalance, err := c.client.BalanceAt(context.Background(), staking.STAKING_CONTRACT_ADDRESS, blockNum)
	if err != nil {
		log.Error("CacheManager BalanceAt staking contract", "error", err)
		return nil, err
	}

	reportTime := time.Now().UTC()
	daily := &StakingDetails{
		TotalStakedCoins: stakingContractBalance.String(),
		ReportDate:       reportTime.Unix(),
	}
	err = c.putDailyStakingDetailsInDb(daily, reportTime, batch)

	return stakingContractBalance, nil
}

func getDailyStakingKey(date string) (key string, blob []byte) {
	key = fmt.Sprintf(DailyStakingKey, date)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) putDailyStakingDetailsInDb(item *StakingDetails, reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	key, keyBlob := getDailyStakingKey(reportTime.Format("2006-02-01"))
	log.Info("putDailyStakingDetailsInDb", "key", key)

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
