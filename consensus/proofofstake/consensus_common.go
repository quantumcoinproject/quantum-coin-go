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
	// RoundProposers holds getBlockProposer results for rounds 1..MAX_ROUND (same inputs as filtering).
	RoundProposers map[byte]common.Address
	// NilVoteProposalHashes and NilVotePrecommitHashes are keyed by round 1..MAX_ROUND (same as getNilVote*).
	NilVoteProposalHashes  map[byte]common.Hash
	NilVotePrecommitHashes map[byte]common.Hash
}

func validatePreparedDerivedMaps(roundProposers map[byte]common.Address, nilProp, nilPre map[byte]common.Hash) error {
	want := int(MAX_ROUND)
	if len(roundProposers) != want || len(nilProp) != want || len(nilPre) != want {
		return errors.New("PrepareConsensusState invalid derived map lengths")
	}
	for r := byte(1); r <= MAX_ROUND; r++ {
		if _, ok := roundProposers[r]; !ok {
			return errors.New("PrepareConsensusState missing round proposer")
		}
		if _, ok := nilProp[r]; !ok {
			return errors.New("PrepareConsensusState missing nil vote proposal hash")
		}
		if _, ok := nilPre[r]; !ok {
			return errors.New("PrepareConsensusState missing nil vote precommit hash")
		}
	}
	return nil
}

func lookupRoundProposer(m map[byte]common.Address, round byte) (common.Address, error) {
	p, ok := m[round]
	if !ok {
		return common.Address{}, errors.New("round proposer not precomputed")
	}
	return p, nil
}

func lookupNilVoteProposalHash(m map[byte]common.Hash, round byte) (common.Hash, error) {
	h, ok := m[round]
	if !ok {
		return common.Hash{}, errors.New("nil vote proposal hash not precomputed")
	}
	return h, nil
}

func lookupNilVotePrecommitHash(m map[byte]common.Hash, round byte) (common.Hash, error) {
	h, ok := m[round]
	if !ok {
		return common.Hash{}, errors.New("nil vote precommit hash not precomputed")
	}
	return h, nil
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

	prepared, err := PrepareConsensusState(parentHash, consensusContext, validators, validatorDetailsMap, blockNumber)
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
	var rp map[byte]common.Address
	if len(d.roundProposers) > 0 {
		rp = make(map[byte]common.Address, len(d.roundProposers))
		for k, v := range d.roundProposers {
			rp[k] = v
		}
	}
	var np, npc map[byte]common.Hash
	if len(d.nilVoteProposalHashes) > 0 {
		np = make(map[byte]common.Hash, len(d.nilVoteProposalHashes))
		for k, v := range d.nilVoteProposalHashes {
			np[k] = v
		}
	}
	if len(d.nilVotePrecommitHashes) > 0 {
		npc = make(map[byte]common.Hash, len(d.nilVotePrecommitHashes))
		for k, v := range d.nilVotePrecommitHashes {
			npc[k] = v
		}
	}
	return &PreparedConsensusState{
		FilteredValidatorsDepositMap: d.filteredValidatorsDepositMap,
		ValidatorDetailsMap:          vd,
		TotalBlockDepositValue:       d.totalBlockDepositValue,
		MinDepositRequired:           d.blockMinWeightedProposalsRequired,
		RoundProposers:               rp,
		NilVoteProposalHashes:        np,
		NilVotePrecommitHashes:       npc,
	}
}

func PrepareConsensusState(
	parentHash common.Hash,
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

	roundProposers := make(map[byte]common.Address, MAX_ROUND)
	nilVoteProposalHashes := make(map[byte]common.Hash, MAX_ROUND)
	nilVotePrecommitHashes := make(map[byte]common.Hash, MAX_ROUND)
	for r := byte(1); r <= MAX_ROUND; r++ {
		prop, err := getBlockProposer(parentHash, &filteredValidatorDepositMap, r, &detailsMapCopy, blockNumber, consensusContext)
		if err != nil {
			return nil, err
		}
		roundProposers[r] = prop
		nilVoteProposalHashes[r] = getNilVoteProposalHash(parentHash, r)
		nilVotePrecommitHashes[r] = getNilVotePreCommitHash(parentHash, r)
	}

	if err := validatePreparedDerivedMaps(roundProposers, nilVoteProposalHashes, nilVotePrecommitHashes); err != nil {
		return nil, err
	}

	return &PreparedConsensusState{
		FilteredValidatorsDepositMap: filteredValidatorDepositMap,
		ValidatorDetailsMap:          detailsMapCopy,
		TotalBlockDepositValue:       totalBlockDepositValue,
		MinDepositRequired:           minDepositRequired,
		RoundProposers:               roundProposers,
		NilVoteProposalHashes:        nilVoteProposalHashes,
		NilVotePrecommitHashes:       nilVotePrecommitHashes,
	}, nil
}
