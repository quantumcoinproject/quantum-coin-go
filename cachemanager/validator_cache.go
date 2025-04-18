package cachemanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/proofofstake"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"math/big"
	"time"
)

const ValidatorCountKey = "validator-count"
const ValidatorPageKey = "validator-list-%d"
const ValidatorPageSize uint64 = 10000
const DailyValidatorReportKey = "daily-validator-report-%s"

func getValidatorPageKey(pageCount uint64) []byte {
	pageKey := fmt.Sprintf(ValidatorPageKey, pageCount)
	return []byte(pageKey)
}

func getValidatorCountKey() (key string, blob []byte) {
	key = ValidatorCountKey
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) refreshValidators(blockNumber *big.Int, batch *ethdb.Batch) error {
	log.Debug("processValidators", "blockNumber", blockNumber.String())
	validatorList, err := c.client.ListValidators(context.Background(), blockNumber)
	if err != nil {
		return err
	}

	err = c.processValidators(validatorList, batch)
	if err != nil {
		return err
	}

	validatorCount := uint64(0)
	for _, validator := range validatorList {
		if validator.IsValidationPaused == false {
			balance, err := hexutil.DecodeBig(validator.NetBalance)
			if err != nil {
				log.Error("refreshValidators DecodeBig", "error", err)
				return err
			}
			if balance.Cmp(proofofstake.MIN_VALIDATOR_DEPOSIT) >= 0 {
				validatorCount = validatorCount + 1
			}
		}
	}

	reportTime := time.Now().UTC()
	daily := &ValidatorReport{
		TotalValidators: validatorCount,
		ReportDate:      reportTime.Unix(),
	}
	err = c.putDailyValidatorDetailsInDb(daily, reportTime, batch)

	return nil
}

func (c *CacheManager) getValidatorCount() (uint64, error) {
	validatorCountKey, keyBlob := getValidatorCountKey()
	validatorCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getValidatorCount not found", "validatorCountKey", validatorCountKey)
			return 0, nil
		} else {
			log.Error("getValidatorCount cacheDb.Get", "validatorCountKey", validatorCountKey, "error", err)
			return 0, err
		}
	} else {
		validatorCount := common.BytesToUint64(validatorCountBlob)
		log.Info("getValidatorCount", "validatorCountKey", validatorCountKey, "validatorCount", validatorCount)
		return validatorCount, nil
	}
}

func (c *CacheManager) putValidatorCount(validatorCount uint64, batch *ethdb.Batch) error {
	validatorBatch := *batch
	validatorCountKey, keyBlob := getValidatorCountKey()
	log.Info("putValidatorCount", "validatorCountKey", validatorCountKey, "validatorCount", validatorCount)

	blob := common.Uint64ToBytes(validatorCount)
	err := validatorBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putValidatorCount", "error", err, "validatorCount", validatorCount)
		return err
	}

	return nil
}

func (c *CacheManager) processValidators(validators []*proofofstake.ValidatorDetails, batch *ethdb.Batch) error {
	log.Debug("CacheManager processValidators")
	validatorBatch := *batch
	var err error

	validatorList := make([]ValidatorCompact, len(validators))
	for i, validator := range validators {
		v := fromValidatorDetails(validator)
		validatorList[i] = *v
	}

	var validatorPageCount uint64
	var validatorPageKey []byte
	var validatorListBlob []byte

	log.Debug("CacheManager processValidators", "validatorCount", len(validators))

	validatorPageKey = getValidatorPageKey(1) //todo: dynamic

	validatorListBlob, err = json.Marshal(validatorList)
	if err != nil {
		log.Error("CacheManager json.Marshal ValidatorListBlob", "error", err)
		return err
	}
	err = validatorBatch.Put(validatorPageKey, validatorListBlob)
	if err != nil {
		log.Error("CacheManager validatorBatch.Put ValidatorListBlob", "error", err)
		return err
	}

	err = c.putValidatorCount(uint64(len(validators)), batch)
	if err != nil {
		log.Error("CacheManager validatorBatch.Put ValidatorListBlob", "error", err)
		return err
	}

	log.Info("CacheManager validatorBatch.Put", "validatorPageCount", validatorPageCount)

	return nil
}

func (c *CacheManager) ListValidators(pageNumberInput int64) (ListValidatorsResponse, error) {
	listResponse := ListValidatorsResponse{}

	var pageCount uint64
	ValidatorCount, err := c.getValidatorCount()
	if err != nil {
		return ListValidatorsResponse{}, err
	}
	if ValidatorCount%ValidatorPageSize == 0 {
		pageCount = ValidatorCount / ValidatorPageSize
	} else {
		pageCount = (ValidatorCount / ValidatorPageSize) + 1
	}

	if pageCount == 0 {
		return ListValidatorsResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListValidatorBy", "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "ValidatorCount", ValidatorCount)
	if pageNumber > pageCount {
		return ListValidatorsResponse{PageCount: pageCount}, nil
	}

	pageKey := ValidatorPageKey
	validatorPageKey := []byte(pageKey)
	log.Info("cache get", "key", pageKey)

	validatorListBlob, err := c.cacheDb.Get(validatorPageKey)
	if err != nil {
		log.Error("ListValidatorBy cacheDb.Get fromValidatorPageKey", "error", err)
		return ListValidatorsResponse{}, err
	}
	var validatorList ValidatorList
	err = json.Unmarshal(validatorListBlob, &validatorList)
	if err != nil {
		log.Error("ListValidatorBy json.Unmarshal validatorListBlob", "error", err)
		return ListValidatorsResponse{}, err
	}

	listResponse.Items = validatorList.Validators
	listResponse.PageCount = pageCount

	return listResponse, nil
}

func getDailyValidatorReportKey(date string) (key string, blob []byte) {
	key = fmt.Sprintf(DailyValidatorReportKey, date)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) putDailyValidatorDetailsInDb(item *ValidatorReport, reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	key, keyBlob := getDailyValidatorReportKey(reportTime.Format("2006-02-01"))
	log.Info("putDailyValidatorDetailsInDb", "key", key)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putDailyValidatorDetailsInDb", "error", err, "key", key)
		return err
	}

	return nil
}
