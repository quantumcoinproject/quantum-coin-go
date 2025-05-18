package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
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
const DailySpecificValidatorReportKey = "daily-specific-validator-report-%s-%s"

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

func (c *CacheManager) GetValidator(address string) (ValidatorDetails, error) {
	listResponse, err := c.ListValidators(1)
	if err != nil {
		return ValidatorDetails{}, err
	}

	for _, validator := range listResponse.Items {
		if validator.Validator == address || validator.Depositor == address {
			val := fromValidatorCompact(&validator)
			return *val, nil
		}
	}
	return ValidatorDetails{}, errors.New(NotFoundErrMsg)
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

func (c *CacheManager) GetDailyValidatorReport(reportTime time.Time) (*ValidatorReport, error) {
	key, keyBlob := getDailyValidatorReportKey(reportTime.Format("2006-02-01"))
	log.Debug("GetDailyValidatorReport", "key", key, "reportTime", reportTime)

	itemBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		log.Error("GetDailyValidatorReport cacheDb.Get", "error", err, "reportTime", reportTime)
		return nil, err
	}
	var item ValidatorReport
	err = json.Unmarshal(itemBlob, &item)
	if err != nil {
		log.Error("GetDailyValidatorReport json.Unmarshal", "error", err, "reportTime", reportTime)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putDailyValidatorDetailsInDb(item *ValidatorReport, reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	key, keyBlob := getDailyValidatorReportKey(reportTime.Format("2006-02-01"))
	log.Debug("putDailyValidatorDetailsInDb", "key", key)

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

func getDailySpecificValidatorReportKey(date string, address string) (key string, blob []byte) {
	key = fmt.Sprintf(DailySpecificValidatorReportKey, date, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) GetDailySpecificValidatorReport(reportTime time.Time, address string) (*SpecificValidatorReport, error) {
	key, keyBlob := getDailySpecificValidatorReportKey(reportTime.Format("2006-02-01"), address)
	log.Debug("GetDailySpecificValidatorReport", "key", key, "reportTime", reportTime, "address", address)

	itemBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("GetDailySpecificValidatorReport cacheDb.Get", "error", err, "reportTime", reportTime, "address", address)
		} else {
			log.Error("GetDailySpecificValidatorReport cacheDb.Get", "error", err, "reportTime", reportTime, "address", address)
		}
		return nil, err
	}
	var item SpecificValidatorReport
	err = json.Unmarshal(itemBlob, &item)
	if err != nil {
		log.Error("GetDailySpecificValidatorReport json.Unmarshal", "error", err, "reportTime", reportTime, "address", address)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) incrementDailySpecificValidatorDetailsInDb(block *Block, reportTime time.Time, batch *ethdb.Batch) error {
	txnBatch := *batch
	var item *SpecificValidatorReport
	var proposer string
	var err error

	if block.ConsensusDetails.VoteType == string(OK_VOTE) {
		proposer = block.ConsensusDetails.BlockProposer
	} else {
		if len(block.ConsensusDetails.Slashings) == 0 {
			return nil
		}
		proposer = block.ConsensusDetails.Slashings[0].SlashedAccount
	}

	item, err = c.GetDailySpecificValidatorReport(reportTime, proposer)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			item = &SpecificValidatorReport{
				TotalBlockRewardsCoins: "0x0",
				TotalSlashedCoins:      "0x0",
				ReportDate:             reportTime.Unix(),
			}
		} else {
			log.Error("putDailySpecificValidatorDetailsInDb GetDailySpecificValidatorReport", "error", err, "reportTime", reportTime)
			return err
		}
	}

	var voteType proofofstake.VoteType
	if block.ConsensusDetails.VoteType == string(OK_VOTE) {
		voteType = proofofstake.VOTE_TYPE_OK
		item.TotalOkBlocks = item.TotalOkBlocks + 1
	} else {
		voteType = proofofstake.VOTE_TYPE_NIL
		if block.ConsensusDetails.Rounds == 1 {
			item.TotalNilBlocksOfflineValidator = item.TotalNilBlocksOfflineValidator + 1
		} else {
			item.TotalNilBlocksOther = item.TotalNilBlocksOther + 1
		}
	}
	rewards, slashings := proofofstake.GetRewardsSlashingsByVote(big.NewInt(block.BlockNumber), voteType, block.ConsensusDetails.Rounds)
	currentRewards, err := hexutil.DecodeBig(item.TotalBlockRewardsCoins)
	if err != nil {
		return err
	}
	currentSlashings, err := hexutil.DecodeBig(item.TotalSlashedCoins)
	if err != nil {
		return err
	}
	totalRewards := currentRewards.Add(currentRewards, rewards)
	totalSlashings := currentSlashings.Add(currentSlashings, slashings)

	item.TotalBlockRewardsCoins = hexutil.EncodeBig(totalRewards)
	item.TotalSlashedCoins = hexutil.EncodeBig(totalSlashings)

	key, keyBlob := getDailySpecificValidatorReportKey(reportTime.Format("2006-02-01"), block.ConsensusDetails.BlockProposer)
	log.Info("putDailySpecificValidatorDetailsInDb", "key", key, "reportTime", reportTime)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putDailySpecificValidatorDetailsInDb", "error", err, "key", key, "reportTime", reportTime)
		return err
	}

	return nil
}
