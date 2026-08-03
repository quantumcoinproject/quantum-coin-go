package proofofstake

import (
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/internal/ethapi"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking/stakingv3"
	"math/big"
)

// executeWithGas mirrors the shared execute() helper but also returns the gas consumed by the call,
// so tests can measure how much of the RPC gas cap a batched read uses.
func executeWithGas(tcc *TestChainContext, data []byte, from common.Address, state *state.StateDB, header *types.Header, value *big.Int) (hexutil.Bytes, uint64, error) {
	msgData := (hexutil.Bytes)(data)

	args := ethapi.TransactionArgs{
		From:  &from,
		To:    &ContractAddress,
		Data:  &msgData,
		Value: (*hexutil.Big)(value),
	}

	// A finite, generous gas budget with real metering (OverrideGasFailure off) so result.UsedGas
	// reflects the true execution gas, not just the intrinsic gas.
	msg, err := args.ToMessage(uint64(500000000))
	if err != nil {
		return nil, 0, err
	}

	vmConfig := &vm.Config{}
	txContext := core.NewEVMTxContext(msg)
	blockContext := core.NewEVMBlockContext(header, tcc, nil)
	evm := vm.NewEVM(blockContext, txContext, state, chainConfig, *vmConfig)

	gp := new(core.GasPool).AddGas(math.MaxUint64)
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, 0, err
	}
	if result == nil {
		return nil, 0, errors.New("result is nil")
	}
	if len(result.Revert()) > 0 {
		return nil, result.UsedGas, core.NewRevertError(result)
	}

	return result.Return(), result.UsedGas, result.Err
}

// Reports the actual gas used by a full 250-validator listValidatorStakingDetails window and its
// fraction of the 50M RPC gas cap.
func TestStakingV3_ListValidatorStakingDetailsGas(t *testing.T) {
	const valCount = 250
	state := newStakingStateDbV3()
	for i := 0; i < valCount; i++ {
		registerDepositorV3(t, state, common.RandomAddress(), common.RandomAddress())
	}

	method := staking.GetContract_Method_ListValidatorStakingDetails()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		t.Fatal(err)
	}
	data, err := encodeCall(&abiData, method, big.NewInt(0), big.NewInt(int64(valCount)))
	if err != nil {
		t.Fatal(err)
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))
	result, gasUsed, err := executeWithGas(tcc, data, ZERO_ADDRESS, state, header, new(big.Int))
	if err != nil {
		t.Fatal(err)
	}

	var out []ValidatorDetailsV2
	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		t.Fatal(err)
	}
	if len(out) != valCount {
		t.Fatalf("expected %d entries, got %d", valCount, len(out))
	}

	const rpcGasCap = uint64(50000000)
	log.Info("TestStakingV3_ListValidatorStakingDetailsGas",
		"validatorsInWindow", valCount,
		"gasUsed", gasUsed,
		"gasPerValidator", gasUsed/uint64(valCount),
		"rpcGasCap", rpcGasCap,
		"pctOfCap", float64(gasUsed)/float64(rpcGasCap)*100.0)
}

// GetValidatorListLengthV3State reads the raw _validatorList length via the V3 getValidatorListLength().
func GetValidatorListLengthV3State(state *state.StateDB) (*big.Int, error) {
	method := staking.GetContract_Method_GetValidatorListLength()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("GetValidatorListLengthV3State abi error", "err", err)
		return nil, err
	}
	data, err := encodeCall(&abiData, method)
	if err != nil {
		log.Error("Unable to pack getValidatorListLength", "error", err)
		return nil, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, ZERO_ADDRESS, state, header, new(big.Int))
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("getValidatorListLength result is 0")
	}

	var out *big.Int
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err)
		return nil, err
	}
	return out, nil
}

// ListValidatorStakingDetailsV3State reads a single paginated window via the V3 listValidatorStakingDetails().
func ListValidatorStakingDetailsV3State(state *state.StateDB, start uint64, count uint64) ([]ValidatorDetailsV2, error) {
	method := staking.GetContract_Method_ListValidatorStakingDetails()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("ListValidatorStakingDetailsV3State abi error", "err", err)
		return nil, err
	}
	data, err := encodeCall(&abiData, method, new(big.Int).SetUint64(start), new(big.Int).SetUint64(count))
	if err != nil {
		log.Error("Unable to pack listValidatorStakingDetails", "error", err)
		return nil, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, ZERO_ADDRESS, state, header, new(big.Int))
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}

	var out []ValidatorDetailsV2
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err)
		return nil, err
	}
	return out, nil
}

// listAllValidatorStakingDetailsV3State pages through every validator in batches of batchSize, mirroring the
// production listValidatorStakingDetailsV3 helper but driven against a raw state DB in tests.
func listAllValidatorStakingDetailsV3State(state *state.StateDB, batchSize uint64) ([]*ValidatorDetailsV2, error) {
	total, err := GetValidatorListLengthV3State(state)
	if err != nil {
		return nil, err
	}

	totalCount := total.Uint64()
	out := make([]*ValidatorDetailsV2, 0, totalCount)
	for start := uint64(0); start < totalCount; start += batchSize {
		page, err := ListValidatorStakingDetailsV3State(state, start, batchSize)
		if err != nil {
			return nil, err
		}
		for i := range page {
			out = append(out, &page[i])
		}
	}
	return out, nil
}

// listValidatorsAsMapLegacyV3State mirrors the legacy per-validator ListValidatorsAsMap path (listValidators +
// per-validator getDepositorOfValidator + getStakingDetails), keyed by validator and excluding zero-depositor.
func listValidatorsAsMapLegacyV3State(state *state.StateDB) (map[common.Address]*ValidatorDetailsV2, error) {
	validators, err := ListValidators(state)
	if err != nil {
		return nil, err
	}

	out := make(map[common.Address]*ValidatorDetailsV2)
	for _, v := range validators {
		depositor, err := GetDepositorOfValidator(state, v)
		if err != nil {
			return nil, err
		}
		if depositor.IsEqualTo(ZERO_ADDRESS) {
			continue
		}
		details, err := GetStakingDetails(state, v)
		if err != nil {
			return nil, err
		}
		out[v] = details
	}
	return out, nil
}

func validatorDetailsEqual(a, b *ValidatorDetailsV2) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !a.Depositor.IsEqualTo(b.Depositor) || !a.Validator.IsEqualTo(b.Validator) {
		return false
	}
	if a.IsValidationPaused != b.IsValidationPaused {
		return false
	}
	bigFieldsA := []*big.Int{a.Balance, a.NetBalance, a.BlockRewards, a.Slashings, a.WithdrawalBlock, a.WithdrawalAmount, a.LastNiLBlock, a.NilBlockCount}
	bigFieldsB := []*big.Int{b.Balance, b.NetBalance, b.BlockRewards, b.Slashings, b.WithdrawalBlock, b.WithdrawalAmount, b.LastNiLBlock, b.NilBlockCount}
	for i := range bigFieldsA {
		if bigFieldsA[i] == nil || bigFieldsB[i] == nil {
			if bigFieldsA[i] != bigFieldsB[i] {
				return false
			}
			continue
		}
		if bigFieldsA[i].Cmp(bigFieldsB[i]) != 0 {
			return false
		}
	}
	return true
}

// Unit test: the paginated method returns correct windows, stitches into the full set, matches
// getValidatorListLength(), excludes rotated-away (stale) validators, and handles the edge cases
// (start >= total returns empty, partial last page).
func TestStakingV3_ListValidatorStakingDetails_Windows(t *testing.T) {
	state := newStakingStateDbV3()

	const activeCount = 7
	depositors := make([]common.Address, 0)
	validators := make([]common.Address, 0)
	for i := 0; i < activeCount; i++ {
		depositor := common.RandomAddress()
		validator := common.RandomAddress()
		registerDepositorV3(t, state, depositor, validator)
		depositors = append(depositors, depositor)
		validators = append(validators, validator)
	}

	// Rotate the first validator to a new one; the old entry stays in _validatorList but becomes stale.
	rotatedDepositor := depositors[0]
	newValidator := common.RandomAddress()
	if err := BondDepositorForRotationV3(state, newValidator, rotatedDepositor); err != nil {
		t.Fatal(err)
	}
	if err := ChangeValidatorV3(state, rotatedDepositor, newValidator); err != nil {
		t.Fatal(err)
	}
	validators[0] = newValidator

	// Raw list length includes the stale rotated-away entry: activeCount original deposits + 1 rotation push.
	rawLen, err := GetValidatorListLengthV3State(state)
	if err != nil {
		t.Fatal(err)
	}
	if rawLen.Uint64() != uint64(activeCount+1) {
		t.Fatalf("raw list length = %d, want %d", rawLen.Uint64(), activeCount+1)
	}

	// Full set via small batches stitches to exactly the live validators.
	all, err := listAllValidatorStakingDetailsV3State(state, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != activeCount {
		t.Fatalf("live validator count = %d, want %d", len(all), activeCount)
	}

	liveSet := make(map[common.Address]bool)
	for _, v := range validators {
		liveSet[v] = true
	}
	for _, d := range all {
		if !liveSet[d.Validator] {
			t.Fatalf("unexpected validator %s in batch result", d.Validator.String())
		}
		if d.Depositor.IsEqualTo(ZERO_ADDRESS) {
			t.Fatal("batch result should not contain zero depositor")
		}
	}

	// start >= total returns empty.
	empty, err := ListValidatorStakingDetailsV3State(state, rawLen.Uint64(), 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("start >= total should return empty, got %d", len(empty))
	}

	// A partial last page: with rawLen=8 and count=3, the last window [6,8) has 2 raw entries.
	lastPage, err := ListValidatorStakingDetailsV3State(state, 6, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(lastPage) > 2 {
		t.Fatalf("partial last page returned %d entries, want <= 2", len(lastPage))
	}
}

// Equivalence test: a single mixed fixture (active, paused, withdrawing, rewarded/slashed, rotated) yields the
// same map from the legacy per-validator path and the new paginated batch path.
func TestStakingV3_ListValidatorStakingDetails_Equivalence(t *testing.T) {
	state := newStakingStateDbV3()

	// Active validators.
	for i := 0; i < 5; i++ {
		registerDepositorV3(t, state, common.RandomAddress(), common.RandomAddress())
	}

	// Paused validator.
	pausedDepositor := common.RandomAddress()
	registerDepositorV3(t, state, pausedDepositor, common.RandomAddress())
	if err := PauseValidation(state, pausedDepositor); err != nil {
		t.Fatal(err)
	}

	// Rewarded + slashed validator.
	rewardedDepositor := common.RandomAddress()
	registerDepositorV3(t, state, rewardedDepositor, common.RandomAddress())
	if err := AddDepositorRewardV3(state, ZERO_ADDRESS, rewardedDepositor, params.EtherToWei(big.NewInt(123))); err != nil {
		t.Fatal(err)
	}
	if err := AddDepositorSlashingV3(state, ZERO_ADDRESS, rewardedDepositor, params.EtherToWei(big.NewInt(45))); err != nil {
		t.Fatal(err)
	}

	// Withdrawing validator.
	withdrawingDepositor := common.RandomAddress()
	registerDepositorV3(t, state, withdrawingDepositor, common.RandomAddress())
	withdrawAmount := new(big.Int).Sub(MIN_VALIDATOR_DEPOSIT, params.EtherToWei(big.NewInt(2000)))
	if err := InitiatePartialWithdrawalV3(state, withdrawingDepositor, withdrawAmount, 10); err != nil {
		t.Fatal(err)
	}

	// Rotated validator.
	rotatedDepositor := common.RandomAddress()
	registerDepositorV3(t, state, rotatedDepositor, common.RandomAddress())
	newValidator := common.RandomAddress()
	if err := BondDepositorForRotationV3(state, newValidator, rotatedDepositor); err != nil {
		t.Fatal(err)
	}
	if err := ChangeValidatorV3(state, rotatedDepositor, newValidator); err != nil {
		t.Fatal(err)
	}

	legacyMap, err := listValidatorsAsMapLegacyV3State(state)
	if err != nil {
		t.Fatal(err)
	}
	batchList, err := listAllValidatorStakingDetailsV3State(state, validatorStakingDetailsBatchSize)
	if err != nil {
		t.Fatal(err)
	}

	if len(batchList) != len(legacyMap) {
		t.Fatalf("row count mismatch: batch=%d legacy=%d", len(batchList), len(legacyMap))
	}
	for _, d := range batchList {
		legacy, ok := legacyMap[d.Validator]
		if !ok {
			t.Fatalf("validator %s present in batch but missing in legacy", d.Validator.String())
		}
		if !validatorDetailsEqual(d, legacy) {
			t.Fatalf("validator %s details mismatch between batch and legacy", d.Validator.String())
		}
	}
}

// Perf + deep-equivalence test: with 1000 validators, the legacy per-validator path and the new paginated path
// (~4 page lookups of 250) must produce the same number of rows and identical data for every row.
func TestStakingV3_ListValidatorStakingDetailsPerf(t *testing.T) {
	const valCount = 1000
	state := newStakingStateDbV3()

	for i := 0; i < valCount; i++ {
		registerDepositorV3(t, state, common.RandomAddress(), common.RandomAddress())
	}

	startLegacy := time.Now()
	legacyMap, err := listValidatorsAsMapLegacyV3State(state)
	if err != nil {
		t.Fatal(err)
	}
	legacyDuration := time.Since(startLegacy)

	startBatch := time.Now()
	batchList, err := listAllValidatorStakingDetailsV3State(state, validatorStakingDetailsBatchSize)
	if err != nil {
		t.Fatal(err)
	}
	batchDuration := time.Since(startBatch)

	log.Info("TestStakingV3_ListValidatorStakingDetailsPerf",
		"valCount", valCount,
		"legacy (1+2N calls)", legacyDuration,
		"batched (~4 page calls)", batchDuration)

	// Same number of rows.
	if len(legacyMap) != valCount {
		t.Fatalf("legacy row count = %d, want %d", len(legacyMap), valCount)
	}
	if len(batchList) != len(legacyMap) {
		t.Fatalf("row count mismatch: batch=%d legacy=%d", len(batchList), len(legacyMap))
	}

	// Same data for each row (deep check), keyed by validator so ordering differences don't matter.
	batchMap := make(map[common.Address]*ValidatorDetailsV2, len(batchList))
	for _, d := range batchList {
		if _, dup := batchMap[d.Validator]; dup {
			t.Fatalf("duplicate validator %s in batch result", d.Validator.String())
		}
		batchMap[d.Validator] = d
	}

	for v, legacy := range legacyMap {
		batched, ok := batchMap[v]
		if !ok {
			t.Fatalf("validator %s present in legacy but missing in batch", v.String())
		}
		if !validatorDetailsEqual(batched, legacy) {
			t.Fatalf("validator %s details mismatch between batch and legacy", v.String())
		}
		if !reflect.DeepEqual(batched, legacy) {
			t.Fatalf("validator %s details not deeply equal between batch and legacy", v.String())
		}
	}
}

// newStakingStateDbV3OnDisk mirrors newStakingStateDbV3 but backs the state with a real on-disk
// LevelDB (the same key-value backend the actual node uses), created under the test's temp dir.
// The LevelDB handle is closed on test cleanup before the temp dir is removed (required on Windows).
func newStakingStateDbV3OnDisk(t *testing.T) (*state.StateDB, state.Database, ethdb.Database) {
	t.Helper()

	dir := t.TempDir()
	// cache (MB) / handles roughly match a node's chaindata open; exact values are not important
	// here, only that reads/writes go through an on-disk LevelDB rather than an in-memory map.
	diskdb, err := rawdb.NewLevelDBDatabase(filepath.Join(dir, "chaindata"), 256, 1024, "", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { diskdb.Close() })

	sdb := state.NewDatabase(diskdb)
	statedb, err := state.New(common.Hash{}, sdb, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	statedb.CreateAccount(ContractAddress)
	statedb.SetCode(ContractAddress, common.FromHex(stakingv3.STAKING_RUNTIME_BIN))
	statedb.Finalise(true) // Push the state into the "original" slot

	return statedb, sdb, diskdb
}

// On-disk counterpart of TestStakingV3_ListValidatorStakingDetailsPerf: identical 1000-validator
// fixture and legacy-vs-batch deep-equivalence checks, but the state is backed by a real LevelDB on
// disk. After the fixture is built it is committed to disk and reopened with cold trie-node caches,
// so the legacy and batch read paths actually load nodes from LevelDB (as on a live node) instead of
// from an in-memory database.
func TestStakingV3_ListValidatorStakingDetailsPerfOnDisk(t *testing.T) {
	const valCount = 1000

	statedb, sdb, diskdb := newStakingStateDbV3OnDisk(t)
	for i := 0; i < valCount; i++ {
		registerDepositorV3(t, statedb, common.RandomAddress(), common.RandomAddress())
	}

	// Persist the fixture to the on-disk LevelDB: Commit flushes dirty objects/code and produces the
	// state root, then TrieDB().Commit pushes the cached trie nodes down to the disk key-value store.
	root, err := statedb.Commit(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := sdb.TrieDB().Commit(root, false, nil); err != nil {
		t.Fatal(err)
	}

	// Reopen the committed state through a fresh state.Database (cold trie-node cache) over the same
	// on-disk LevelDB, so subsequent reads are served from disk rather than warm memory.
	diskState, err := state.New(root, state.NewDatabase(diskdb), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	startLegacy := time.Now()
	legacyMap, err := listValidatorsAsMapLegacyV3State(diskState)
	if err != nil {
		t.Fatal(err)
	}
	legacyDuration := time.Since(startLegacy)

	startBatch := time.Now()
	batchList, err := listAllValidatorStakingDetailsV3State(diskState, validatorStakingDetailsBatchSize)
	if err != nil {
		t.Fatal(err)
	}
	batchDuration := time.Since(startBatch)

	log.Info("TestStakingV3_ListValidatorStakingDetailsPerfOnDisk",
		"valCount", valCount,
		"backend", "leveldb (on-disk)",
		"legacy (1+2N calls)", legacyDuration,
		"batched (~4 page calls)", batchDuration)

	// Same number of rows.
	if len(legacyMap) != valCount {
		t.Fatalf("legacy row count = %d, want %d", len(legacyMap), valCount)
	}
	if len(batchList) != len(legacyMap) {
		t.Fatalf("row count mismatch: batch=%d legacy=%d", len(batchList), len(legacyMap))
	}

	// Same data for each row (deep check), keyed by validator so ordering differences don't matter.
	batchMap := make(map[common.Address]*ValidatorDetailsV2, len(batchList))
	for _, d := range batchList {
		if _, dup := batchMap[d.Validator]; dup {
			t.Fatalf("duplicate validator %s in batch result", d.Validator.String())
		}
		batchMap[d.Validator] = d
	}

	for v, legacy := range legacyMap {
		batched, ok := batchMap[v]
		if !ok {
			t.Fatalf("validator %s present in legacy but missing in batch", v.String())
		}
		if !validatorDetailsEqual(batched, legacy) {
			t.Fatalf("validator %s details mismatch between batch and legacy", v.String())
		}
		if !reflect.DeepEqual(batched, legacy) {
			t.Fatalf("validator %s details not deeply equal between batch and legacy", v.String())
		}
	}
}
