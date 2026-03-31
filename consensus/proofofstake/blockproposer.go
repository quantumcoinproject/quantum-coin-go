package proofofstake

import (
	"bytes"
	"errors"
	"math/big"
	"sort"

	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

func canValidate(valDetails *ValidatorDetailsV2, currentBlockNumber uint64) (bool, uint64) {
	if valDetails.LastNiLBlock.Cmp(new(big.Int)) == 0 {
		return true, currentBlockNumber
	}
	if valDetails.NilBlockCount.Uint64() < OFFLINE_VALIDATOR_DEFER_THRESHOLD {
		return true, currentBlockNumber
	}

	nextValidationBlock := valDetails.LastNiLBlock.Uint64() + OFFLINE_VALIDATOR_DEFER_COUNT
	result := currentBlockNumber >= nextValidationBlock

	log.Debug("canValidate", "validator", valDetails.Validator, "result", result, "currentBlockNumber", currentBlockNumber, "LastNiLBlock", valDetails.LastNiLBlock,
		"NilBlockCount", valDetails.NilBlockCount, "nextValidationBlock", nextValidationBlock)

	return result, nextValidationBlock
}

func canPropose(valDetails *ValidatorDetailsV2, currentBlockNumber uint64) (bool, uint64) {
	if valDetails.LastNiLBlock.Cmp(new(big.Int)) == 0 {
		log.Debug("canPropose no nil block", "currentBlockNumber", currentBlockNumber, "canPropose", true, "validator", valDetails.Validator)
		return true, currentBlockNumber
	}

	var maxBlockDelay uint64
	if currentBlockNumber >= defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock {
		maxBlockDelay = BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V3
	} else if currentBlockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK {
		maxBlockDelay = BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2
	} else {
		maxBlockDelay = BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT
	}

	slotsMissed := valDetails.NilBlockCount.Uint64() / BLOCK_PROPOSER_OFFLINE_NIL_BLOCK_MULTIPLIER
	if slotsMissed >= 16 { //to avoid overflow errors
		slotsMissed = 16
	}
	blockDelay := uint64(1) << slotsMissed
	if blockDelay > maxBlockDelay {
		blockDelay = maxBlockDelay
	}

	if valDetails.NilBlockCount.Uint64() > 1 && currentBlockNumber >= defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock {
		blockDelay = blockDelay + defaults.DefaultConfig.PosConfig.MinOfflineProposerBlockDelay
	}

	nextProposalBlock := valDetails.LastNiLBlock.Uint64() + blockDelay
	result := currentBlockNumber >= nextProposalBlock
	log.Debug("canPropose", "LastNiLBlock", valDetails.LastNiLBlock, "NilBlockCount", valDetails.NilBlockCount,
		"slotsMissed", slotsMissed, "blockDelay", blockDelay, "nextProposalBlock", nextProposalBlock, "maxBlockDelay", maxBlockDelay,
		"currentBlockNumber", currentBlockNumber, "canPropose", result, "validator", valDetails.Validator)
	return result, nextProposalBlock
}

func getBlockProposer(parentHash common.Hash, filteredValidatorDepositMap *map[common.Address]*big.Int, round byte,
	validatorDetailsMap *map[common.Address]*ValidatorDetailsV2, blockNumber uint64, contextHash common.Hash) (common.Address, *backupmanager.BlockProposerV2RoundTrace, error) {
	var proposer common.Address
	if blockNumber >= defaults.DefaultConfig.PosConfig.CONTEXT_BASED_START_BLOCK {
		return getBlockProposerV2(contextHash, validatorDetailsMap, round, blockNumber, backupmanager.BlockProposerV2TraceInput{
			ParentHash: parentHash, UsedConsensusContext: true,
		})
	}

	if blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK {
		return getBlockProposerV2(parentHash, validatorDetailsMap, round, blockNumber, backupmanager.BlockProposerV2TraceInput{
			ParentHash: parentHash, UsedConsensusContext: false,
		})
	}

	if len(*filteredValidatorDepositMap) < MIN_VALIDATORS {
		return proposer, nil, errors.New("min validators not found")
	}

	validators := make([]common.Address, len(*filteredValidatorDepositMap))
	i := 0
	for k := range *filteredValidatorDepositMap {
		validators[i].CopyFrom(k)
		log.Debug("getBlockProposer validator", "v", validators[i], "i", i)
		i = i + 1
	}

	sort.SliceStable(validators, func(i, j int) bool {
		vi := crypto.Keccak256Hash(parentHash.Bytes(), validators[i].Bytes(), []byte{round}).Bytes()
		vj := crypto.Keccak256Hash(parentHash.Bytes(), validators[j].Bytes(), []byte{round}).Bytes()
		return bytes.Compare(vi, vj) == -1
	})

	proposer = validators[0]
	log.Debug("getBlockProposer", "proposer", proposer, "round", round)

	return proposer, nil, nil
}

func getBlockProposerV2(contextHash common.Hash, validatorMap *map[common.Address]*ValidatorDetailsV2, round byte, blockNumber uint64, traceIn backupmanager.BlockProposerV2TraceInput) (common.Address, *backupmanager.BlockProposerV2RoundTrace, error) {
	var proposer common.Address
	trace := &backupmanager.BlockProposerV2RoundTrace{
		Round:                                  round,
		BlockNumber:                            blockNumber,
		ParentHash:                             traceIn.ParentHash,
		SelectionHash:                          contextHash,
		UsedConsensusContext:                   traceIn.UsedConsensusContext,
		MinValidatorsRequired:                  MIN_VALIDATORS,
		InputValidatorCount:                    len(*validatorMap),
		SortUsesBlockNumberInHash:              blockNumber >= defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock,
		ConfigOfflineValidatorDeferStartBlock:  defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock,
		ConfigBlockProposerOfflineV2StartBlock: defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK,
		ConfigOfflineValidatorV4StartBlock:     defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock,
		ConfigMinOfflineProposerBlockDelay:     defaults.DefaultConfig.PosConfig.MinOfflineProposerBlockDelay,
		ConfigMaxBlockDelayV1:                  BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT,
		ConfigMaxBlockDelayV2:                  BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2,
		ConfigMaxBlockDelayV3:                  BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V3,
	}

	if len(*validatorMap) < MIN_VALIDATORS {
		return proposer, trace, errors.New("getBlockProposerV2 min validators not found")
	}

	selectedValMap := make(map[common.Address]*ValidatorDetailsV2)
	for valAddr, valDetails := range *validatorMap {
		canProp, _ := canPropose(valDetails, blockNumber)
		log.Debug("getBlockProposerV2", "valAddr", valAddr, "canPropose", canProp)
		if canProp == false {
			continue
		}
		selectedValMap[valAddr] = valDetails
	}

	trace.AfterFilterCount = len(selectedValMap)

	//If fewer proposers than MIN_VALIDATORS, then select everyone, something is wrong
	if len(selectedValMap) < MIN_VALIDATORS {
		trace.FallbackExpandedToAll = true
		for valAddr, valDetails := range *validatorMap {
			selectedValMap[valAddr] = valDetails
		}
	}

	trace.ValidatorEvaluations = serializeSelectedValidatorEvaluations(selectedValMap, blockNumber)

	validators := make([]common.Address, len(selectedValMap))
	j := 0
	for valAddr := range selectedValMap {
		validators[j] = valAddr
		j = j + 1
	}
	blockBytes := common.Uint64ToBytes(blockNumber)

	var sortComparisons []backupmanager.BlockProposerV2SortComparison
	sort.SliceStable(validators, func(i, j int) bool {
		var vi, vj common.Hash
		var cmpResult int
		if blockNumber < defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock {
			vi = crypto.Keccak256Hash(contextHash.Bytes(), validators[i].Bytes(), []byte{round})
			vj = crypto.Keccak256Hash(contextHash.Bytes(), validators[j].Bytes(), []byte{round})
			cmpResult = bytes.Compare(vi.Bytes(), vj.Bytes())
		} else {
			vi = crypto.Keccak256Hash(contextHash.Bytes(), validators[i].Bytes(), []byte{round}, blockBytes)
			vj = crypto.Keccak256Hash(contextHash.Bytes(), validators[j].Bytes(), []byte{round}, blockBytes)
			cmpResult = bytes.Compare(vi.Bytes(), vj.Bytes())
		}
		sortComparisons = append(sortComparisons, backupmanager.BlockProposerV2SortComparison{
			IndexI:      i,
			IndexJ:      j,
			ValidatorI:  validators[i],
			ValidatorJ:  validators[j],
			Vi:          vi,
			Vj:          vj,
			CmpResult:   cmpResult,
			ContextHash: contextHash,
			Round:       round,
			BlockBytes:  hexutil.Bytes(blockBytes),
		})
		return cmpResult == -1
	})
	trace.SortComparisons = sortComparisons

	proposer = validators[0]
	trace.SelectedProposer = proposer

	log.Debug("getBlockProposerV2 final", "proposer", proposer, "round", round, "contextHash", contextHash, "valCount selected", len(validators), "valcount before", len(*validatorMap),
		"OfflineValidatorV4StartBlock", defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, "blockBytes", blockBytes)

	return proposer, trace, nil
}
