package proofofstake

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

type PreparedConsensusState struct {
	FilteredValidatorsDepositMap map[common.Address]*big.Int
	ValidatorDetailsMap          map[common.Address]*ValidatorDetailsV2
	TotalBlockDepositValue       *big.Int
	MinDepositRequired           *big.Int
}

// PreparedConsensusData is the result of loading validator state from chain and running PrepareConsensusState.
type PreparedConsensusData struct {
	ConsensusContext          common.Hash
	PreFilterValidatorCount   int
	ValidatorsDepositMap      map[common.Address]*big.Int
	OrigValidatorDetailsMap   map[common.Address]*ValidatorDetailsV2
	Prepared                  *PreparedConsensusState
}

// PrepareConsensusData loads validators (and optionally validator details) via the given callbacks, derives
// consensusContext (using preContextConsensusContext when below CONTEXT_BASED_START_BLOCK), and runs
// PrepareConsensusState. This is the single place for getValidatorsFn / listValidatorsFn in this flow.
func PrepareConsensusData(
	parentHash common.Hash,
	blockNumber uint64,
	getValidatorsFn GetValidatorsFn,
	getBlockConsensusContext GetBlockConsensusContextFn,
	listValidatorsFn ListValidatorsAsMapFn,
	preContextConsensusContext common.Hash,
) (*PreparedConsensusData, error) {
	if getValidatorsFn == nil {
		return nil, errors.New("getValidatorsFn nil")
	}

	validators, err := getValidatorsFn(parentHash)
	if err != nil {
		return nil, err
	}
	preFilterValidatorCount := len(validators)

	var consensusContext common.Hash
	if blockNumber >= defaults.DefaultConfig.PosConfig.CONTEXT_BASED_START_BLOCK {
		contextKey, err := GetBlockConsensusContextKeyForBlock(blockNumber)
		if err != nil {
			return nil, err
		}
		blockContext, err := getBlockConsensusContext(contextKey, parentHash)
		if err != nil {
			return nil, err
		}
		consensusContext = crypto.Keccak256Hash(blockContext[:], []byte(strconv.Itoa(preFilterValidatorCount)))
		log.Debug("PrepareConsensusData", "blockContext", blockContext, "consensusContext", consensusContext,
			"preFilterValidatorCount", preFilterValidatorCount, "blockNumber", blockNumber)
	} else {
		consensusContext = preContextConsensusContext
	}

	var validatorDetailsMap map[common.Address]*ValidatorDetailsV2
	var origValidatorDetailsMap map[common.Address]*ValidatorDetailsV2
	if blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK {
		if listValidatorsFn == nil {
			return nil, errors.New("listValidatorsFn nil")
		}
		validatorDetailsMap, err = listValidatorsFn(parentHash)
		if err != nil {
			return nil, err
		}
		origValidatorDetailsMap = make(map[common.Address]*ValidatorDetailsV2, len(validatorDetailsMap))
		for addr, vd := range validatorDetailsMap {
			origValidatorDetailsMap[addr] = copyValidatorDetailsV2(vd)
		}
	}

	prepared, err := PrepareConsensusState(consensusContext, validators, validatorDetailsMap, blockNumber)
	if err != nil {
		return nil, err
	}

	return &PreparedConsensusData{
		ConsensusContext:        consensusContext,
		PreFilterValidatorCount:   preFilterValidatorCount,
		ValidatorsDepositMap:      validators,
		OrigValidatorDetailsMap:   origValidatorDetailsMap,
		Prepared:                  prepared,
	}, nil
}

func preparedConsensusStateFromBlockState(d *BlockStateDetails) *PreparedConsensusState {
	var vd map[common.Address]*ValidatorDetailsV2
	if d.validatorDetailsMap != nil {
		vd = *d.validatorDetailsMap
	}
	return &PreparedConsensusState{
		FilteredValidatorsDepositMap: d.filteredValidatorsDepositMap,
		ValidatorDetailsMap:          vd,
		TotalBlockDepositValue:       d.totalBlockDepositValue,
		MinDepositRequired:           d.blockMinWeightedProposalsRequired,
	}
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
