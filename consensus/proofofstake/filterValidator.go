package proofofstake

import (
	"bytes"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"math/big"
	"sort"
)

func filterValidators(consensusContext common.Hash, valDepMap *map[common.Address]*big.Int, blockNumber uint64, validatorDetailsMap *map[common.Address]*ValidatorDetailsV2) (filteredValidators map[common.Address]bool,
	filteredDepositValue *big.Int, blockMinWeightedProposalsRequired *big.Int, err error) {

	validatorsDepositMap := *valDepMap

	totalDepositValue := big.NewInt(0)
	valCount := 0
	for val, depositValue := range validatorsDepositMap {
		if depositValue.Cmp(MIN_VALIDATOR_DEPOSIT) == -1 {
			log.Trace("Skipping validator with low balance", "val", val, "depositValue", depositValue)
			delete(validatorsDepositMap, val)
			continue
		}
		if blockNumber >= defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock {
			valDetailsMap := *validatorDetailsMap
			canVal, _ := canValidate(valDetailsMap[val], blockNumber)
			if canVal == false {
				log.Trace("Skipping offline validator", "val", val, "depositValue", depositValue)
				delete(validatorsDepositMap, val)
				continue
			}
			if blockNumber >= defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock {
				origDepositValue := depositValue
				depositValue = getOfflineValidatorDepositAfterPenalty(valDetailsMap[val], blockNumber, depositValue)
				if depositValue.Cmp(MIN_VALIDATOR_DEPOSIT) == -1 {
					log.Debug("Skipping validator with low balance AfterPenalty", "val", val, "afterPenalty depositValue", depositValue, "origDepositValue", origDepositValue)
					delete(validatorsDepositMap, val)
					continue
				}
				validatorsDepositMap[val] = depositValue
			}
		}
		totalDepositValue = common.SafeAddBigInt(totalDepositValue, depositValue)
		valCount = valCount + 1
	}

	if valCount < MIN_VALIDATORS {
		log.Warn("Validator count", "count", valCount, "MIN_VALIDATORS", MIN_VALIDATORS)
		return nil, nil, nil, errors.New("number of validators less than minimum")
	}

	if totalDepositValue.Cmp(MIN_BLOCK_DEPOSIT) == -1 {
		return nil, nil, nil, errors.New("min block deposit not met")
	}

	filteredValidators = make(map[common.Address]bool)

	if len(validatorsDepositMap) <= MAX_VALIDATORS {
		for validator := range validatorsDepositMap {
			filteredValidators[validator] = true
		}
	} else {
		filteredValidatorsRet, err := getMaxFilteredValidators(consensusContext, totalDepositValue, valDepMap)
		if err != nil {
			return nil, nil, nil, err
		}
		filteredValidators = *filteredValidatorsRet
	}

	filteredDepositValue = big.NewInt(0)
	for val, _ := range filteredValidators {
		depositValue := validatorsDepositMap[val]
		filteredDepositValue = common.SafeAddBigInt(filteredDepositValue, depositValue)
	}

	if filteredDepositValue.Cmp(MIN_BLOCK_DEPOSIT) == -1 {
		return nil, nil, nil, errors.New("min block deposit not met for filteredDepositValue")
	}

	var minPercentage *big.Int
	if blockNumber >= defaults.DefaultConfig.PosConfig.SixtySevenVoteStartBlock {
		minPercentage = MIN_BLOCK_TRANSACTION_WEIGHTED_PROPOSALS_PERCENTAGE_V3
	} else if blockNumber >= defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock {
		minPercentage = MIN_BLOCK_TRANSACTION_WEIGHTED_PROPOSALS_PERCENTAGE_V2
	} else {
		minPercentage = MIN_BLOCK_TRANSACTION_WEIGHTED_PROPOSALS_PERCENTAGE
	}

	blockMinWeightedProposalsRequired = common.SafeRelativePercentageBigInt(filteredDepositValue, minPercentage)

	return filteredValidators, filteredDepositValue, blockMinWeightedProposalsRequired, nil
}

/*
For security of the network, the goal is to select validators in a way that at-least 51% of staked coins are selected,
while at the same time allowing even validators will lesser amount of staked coins (relative to others), to get selected for validation.
It might not always be possible to select validators in a way that 51% of staked coins are selected, but the algorithm tries to establish a balance.
*/
func getMaxFilteredValidators(consensusContext common.Hash, totalDepositValue *big.Int, valDepMap *map[common.Address]*big.Int) (*map[common.Address]bool, error) {
	validatorsDepositMap := *valDepMap

	depositValueSoFar := big.NewInt(0)
	blockMinWeightedStake := common.SafeRelativePercentageBigInt(totalDepositValue, MAX_VALIDATOR_SELECTION_MIN_PERCENTAGE)
	log.Debug("getMaxFilteredValidators", "blockMinWeightedStake", blockMinWeightedStake, "totalDepositValue", totalDepositValue)
	filteredValidators := make(map[common.Address]bool)

	validatorList := make([]common.Address, len(validatorsDepositMap))
	ctr := 0
	for validator, _ := range validatorsDepositMap {
		validatorList[ctr] = validator
		ctr = ctr + 1
	}

	//First pass, fill weighted at-least blockMinWeightedStake percentage of coins
	sort.SliceStable(validatorList, func(i, j int) bool { //SliceStable needed since values can be equal
		valDepI := validatorsDepositMap[validatorList[i]]
		valDepJ := validatorsDepositMap[validatorList[j]]
		cmpResult := valDepI.Cmp(valDepJ)
		if cmpResult == 0 { //equal deposit, sort by consensus context + validator address combo
			vi := crypto.Keccak256Hash(consensusContext.Bytes(), validatorList[i].Bytes()).Bytes()
			vj := crypto.Keccak256Hash(consensusContext.Bytes(), validatorList[j].Bytes()).Bytes()
			return bytes.Compare(vi, vj) == -1
		}
		return cmpResult > 0
	})
	for _, validator := range validatorList {
		filteredValidators[validator] = true
		depositValue := validatorsDepositMap[validator]
		depositValueSoFar = common.SafeAddBigInt(depositValueSoFar, depositValue)
		if len(filteredValidators) == MAX_VALIDATORS_FIRST_PASS_VALIDATOR_SELECTION_CUTOFF || depositValueSoFar.Cmp(blockMinWeightedStake) > 0 {
			log.Trace("getMaxFilteredValidators first pass", "len(filteredValidators)", len(filteredValidators), "depositValueSoFar", depositValueSoFar, "blockMinWeightedStake", blockMinWeightedStake)
			break
		}
	}

	log.Trace("validator count after first pass", "filteredValidators", len(filteredValidators), "depositValueSoFar", depositValueSoFar, "blockMinWeightedStake", blockMinWeightedStake)

	//Second pass, fill based no weighted sort order, but randomness based on consensus context. This ensures those with higher stake have a greater probability of being selected for validation.
	for _, validator := range validatorList {
		_, ok := filteredValidators[validator]
		if ok == true {
			continue
		}
		//Note, we do Keccak256Hash to reduce risk from generation of validator address that are more likely to be lower than just comparing with consensus-context
		leftHash := crypto.Keccak256Hash(consensusContext.Bytes(), validator.Bytes()).Bytes()
		rightHash := crypto.Keccak256Hash(validator.Bytes(), consensusContext.Bytes()).Bytes()
		if bytes.Compare(leftHash, rightHash) > 0 {
			filteredValidators[validator] = true
			if len(filteredValidators) == MAX_VALIDATORS_SECOND_PASS_VALIDATOR_SELECTION_CUTOFF {
				break
			}
		} else {
			log.Trace("validator skip second pass", "validator", validator)
		}
	}

	log.Trace("validator count after second pass", "filteredValidators", len(filteredValidators), "depositValueSoFar", depositValueSoFar, "blockMinWeightedStake", blockMinWeightedStake)

	//Third pass, fill by consensus context sort order, if the buffer is not full even after second pass. This is to ensure fairness even for validators with lower number of staked coins
	//Sort based on consensus context
	sort.Slice(validatorList, func(i, j int) bool {
		vi := crypto.Keccak256Hash(consensusContext.Bytes(), validatorList[i].Bytes()).Bytes()
		vj := crypto.Keccak256Hash(consensusContext.Bytes(), validatorList[j].Bytes()).Bytes()
		return bytes.Compare(vi, vj) == -1
	})

	for _, validator := range validatorList {
		_, ok := filteredValidators[validator]
		if ok == true {
			continue
		}
		filteredValidators[validator] = true
		if len(filteredValidators) == MAX_VALIDATORS {
			break
		}
	}

	return &filteredValidators, nil
}
