package proofofstake

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// validatorCountV2FixedContext is returned by the getBlockConsensusContext stub so the only variable
// feeding consensusContext is the validator count.
var validatorCountV2FixedContext = func() [32]byte {
	var b [32]byte
	copy(b[:], []byte("validatorCountV2ContextStub"))
	return b
}()

func validatorCountV2ContextStub(key string, blockHash common.Hash) ([32]byte, error) {
	return validatorCountV2FixedContext, nil
}

// buildValidatorCountV2Fixtures builds an active deposit map (paused excluded, like GetValidators) and a
// full details map (paused included, like ListValidatorsAsMap) for totalCount validators where the first
// pausedCount validators are paused.
func buildValidatorCountV2Fixtures(totalCount, pausedCount int) (GetValidatorsFn, ListValidatorsAsMapFn) {
	list := hardcodedValidatorListFilter(totalCount)
	deposit := params.EtherToWei(big.NewInt(500000000000))

	activeMap := make(map[common.Address]*big.Int)
	detailsMap := make(map[common.Address]*ValidatorDetailsV2)
	for i, addr := range list {
		paused := i < pausedCount
		detailsMap[addr] = &ValidatorDetailsV2{
			Validator:          addr,
			Balance:            new(big.Int).Set(deposit),
			NetBalance:         new(big.Int).Set(deposit),
			BlockRewards:       big.NewInt(0),
			Slashings:          big.NewInt(0),
			IsValidationPaused: paused,
			WithdrawalBlock:    big.NewInt(0),
			WithdrawalAmount:   big.NewInt(0),
			LastNiLBlock:       big.NewInt(0),
			NilBlockCount:      big.NewInt(0),
		}
		if !paused {
			activeMap[addr] = new(big.Int).Set(deposit)
		}
	}

	getValidatorsFn := func(common.Hash) (map[common.Address]*big.Int, error) {
		out := make(map[common.Address]*big.Int, len(activeMap))
		for k, v := range activeMap {
			out[k] = new(big.Int).Set(v)
		}
		return out, nil
	}
	listValidatorsFn := func(common.Hash) (map[common.Address]*ValidatorDetailsV2, error) {
		out := make(map[common.Address]*ValidatorDetailsV2, len(detailsMap))
		for k, v := range detailsMap {
			out[k] = copyValidatorDetailsV2(v)
		}
		return out, nil
	}
	return getValidatorsFn, listValidatorsFn
}

func prepareValidatorCountV2Ctx(t *testing.T, blockNumber uint64, totalCount, pausedCount int) (common.Hash, int) {
	t.Helper()
	getValidatorsFn, listValidatorsFn := buildValidatorCountV2Fixtures(totalCount, pausedCount)
	prepared, err := PrepareConsensusData(randHash(), blockNumber, getValidatorsFn, validatorCountV2ContextStub, listValidatorsFn, common.Hash{})
	if err != nil {
		t.Fatalf("PrepareConsensusData(block=%d total=%d paused=%d): %v", blockNumber, totalCount, pausedCount, err)
	}
	return prepared.ConsensusContext, prepared.PreFilterValidatorCount
}

// TestPrepareConsensusData_ValidatorCountV2PausedAccrual verifies the ValidatorCountV2StartBlock gate:
// from that block onward paused validators accrue to the validator count that seeds consensusContext,
// so toggling pause/unpause cannot shift block-proposer selection. Before the gate, the legacy behavior
// (paused validators excluded from the count) is preserved.
func TestPrepareConsensusData_ValidatorCountV2PausedAccrual(t *testing.T) {
	origMaxRound := MAX_ROUND
	MAX_ROUND = byte(2)
	defer func() { MAX_ROUND = origMaxRound }()

	forkBlock := defaults.DefaultConfig.PosConfig.ValidatorCountV2StartBlock
	const totalCount = 6
	const pausedCount = 2

	expectedContext := func(count int) common.Hash {
		return crypto.Keccak256Hash(validatorCountV2FixedContext[:], []byte(strconv.Itoa(count)))
	}

	// At/after the fork: count = active + paused = totalCount in both scenarios, so the same total set
	// yields the same consensusContext regardless of which validators are paused.
	ctxAllActive, preCountAllActive := prepareValidatorCountV2Ctx(t, forkBlock, totalCount, 0)
	ctxSomePaused, preCountSomePaused := prepareValidatorCountV2Ctx(t, forkBlock, totalCount, pausedCount)
	if ctxAllActive != ctxSomePaused {
		t.Fatalf("at fork: expected consensusContext to be stable across pause/unpause; allActive=%x somePaused=%x", ctxAllActive, ctxSomePaused)
	}
	if ctxSomePaused != expectedContext(totalCount) {
		t.Fatalf("at fork: expected consensusContext to use count=%d; got %x want %x", totalCount, ctxSomePaused, expectedContext(totalCount))
	}
	if preCountAllActive != totalCount {
		t.Fatalf("at fork: PreFilterValidatorCount allActive: got %d want %d", preCountAllActive, totalCount)
	}
	if preCountSomePaused != totalCount-pausedCount {
		t.Fatalf("at fork: PreFilterValidatorCount somePaused: got %d want %d", preCountSomePaused, totalCount-pausedCount)
	}

	// Just below the fork: legacy behavior, paused validators are excluded from the count, so differing
	// pause counts produce differing consensusContext.
	beforeBlock := forkBlock - 1
	ctxAllActiveBefore, _ := prepareValidatorCountV2Ctx(t, beforeBlock, totalCount, 0)
	ctxSomePausedBefore, preCountSomePausedBefore := prepareValidatorCountV2Ctx(t, beforeBlock, totalCount, pausedCount)
	if ctxAllActiveBefore == ctxSomePausedBefore {
		t.Fatalf("before fork: expected consensusContext to differ when paused count changes; both=%x", ctxAllActiveBefore)
	}
	if ctxSomePausedBefore != expectedContext(totalCount-pausedCount) {
		t.Fatalf("before fork: expected consensusContext to use active count=%d; got %x want %x", totalCount-pausedCount, ctxSomePausedBefore, expectedContext(totalCount-pausedCount))
	}
	if preCountSomePausedBefore != totalCount-pausedCount {
		t.Fatalf("before fork: PreFilterValidatorCount somePaused: got %d want %d", preCountSomePausedBefore, totalCount-pausedCount)
	}
}
