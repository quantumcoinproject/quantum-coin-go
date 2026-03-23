package proofofstake

import (
	"errors"
	"math/big"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

type PreparedConsensusState struct {
	FilteredValidatorsDepositMap map[common.Address]*big.Int
	ValidatorDetailsMap          map[common.Address]*ValidatorDetailsV2
	TotalBlockDepositValue       *big.Int
	MinDepositRequired           *big.Int
}

func PrepareConsensusState(
	consensusContext common.Hash,
	validatorDepositMap map[common.Address]*big.Int,
	validatorDetailsMap map[common.Address]*ValidatorDetailsV2,
	blockNumber uint64,
) (*PreparedConsensusState, error) {
	depMapCopy := make(map[common.Address]*big.Int, len(validatorDepositMap))
	for addr, dep := range validatorDepositMap {
		if dep != nil {
			depMapCopy[addr] = new(big.Int).Set(dep)
		}
	}

	var detailsMapCopy map[common.Address]*ValidatorDetailsV2
	if validatorDetailsMap != nil {
		detailsMapCopy = make(map[common.Address]*ValidatorDetailsV2, len(validatorDetailsMap))
		for addr, vd := range validatorDetailsMap {
			detailsMapCopy[addr] = copyValidatorDetailsV2(vd)
		}
	}

	filteredValidators, totalBlockDepositValue, minDepositRequired, err := filterValidators(consensusContext, &depMapCopy, blockNumber, &detailsMapCopy)
	if err != nil {
		return nil, err
	}

	if MIN_BLOCK_DEPOSIT.Cmp(minDepositRequired) > 0 {
		log.Warn("PrepareConsensusState minDepositRequired not met", "minDepositRequired", minDepositRequired)
		return nil, errors.New("min deposit required error")
	}

	if len(filteredValidators) < MIN_VALIDATORS {
		return nil, errors.New("filteredValidators MIN_VALIDATORS")
	}

	if len(filteredValidators) > MAX_VALIDATORS {
		return nil, errors.New("filteredValidators MAX_VALIDATORS")
	}

	filteredValidatorDepositMap := make(map[common.Address]*big.Int, len(filteredValidators))
	for v := range filteredValidators {
		filteredValidatorDepositMap[v] = depMapCopy[v]
		log.Debug("PrepareConsensusState", "validator", v, "deposit value after filtering", depMapCopy[v], "blockNumber", blockNumber)
	}

	if blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK {
		var toRemove []common.Address
		for valAddr, valDetails := range detailsMapCopy {
			if valDetails.IsValidationPaused {
				toRemove = append(toRemove, valAddr)
				log.Debug("PrepareConsensusState ValidationPaused remove", "validator", valAddr, "blockNumber", blockNumber)
				continue
			}
			_, ok := filteredValidatorDepositMap[valAddr]
			if ok == false {
				log.Debug("PrepareConsensusState filteredValidatorDepositMap remove", "validator", valAddr, "blockNumber", blockNumber)
				toRemove = append(toRemove, valAddr)
			}
		}
		for _, valAddr := range toRemove {
			delete(detailsMapCopy, valAddr)
		}

		log.Debug("PrepareConsensusState before getBlockProposer", "len(filteredValidatorDepositMap)",
			len(filteredValidatorDepositMap), "len(detailsMapCopy)", len(detailsMapCopy), "blockNumber", blockNumber, "consensusContext", consensusContext)
	}

	return &PreparedConsensusState{
		FilteredValidatorsDepositMap: filteredValidatorDepositMap,
		ValidatorDetailsMap:          detailsMapCopy,
		TotalBlockDepositValue:       totalBlockDepositValue,
		MinDepositRequired:           minDepositRequired,
	}, nil
}
