package proofofstake

import (
	"bytes"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"math/big"
	"sort"
	"testing"
	"time"
)

var TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock

func testFilterValidatorsTest(t *testing.T, consensusContext common.Hash, validatorsDepositMap map[common.Address]*big.Int, shouldPass bool) *big.Int {
	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, TestFilterValidatorsBlockNumber, nil)
	if err == nil {
		if shouldPass == false {
			t.Fatalf("failed")
		}
	} else {
		fmt.Println("filterValidators error", err)
		if shouldPass == true {
			t.Fatalf("filterValidators failed")
		}
		return nil
	}

	if MIN_BLOCK_DEPOSIT.Cmp(filteredDepositValue) > 0 {
		t.Fatalf("failed")
	}

	fmt.Println("selected validator count", len(resultMap), "total validators", len(validatorsDepositMap))
	if len(resultMap) < MIN_VALIDATORS {
		t.Fatalf("failed")
	}

	if len(resultMap) > MAX_VALIDATORS {
		t.Fatalf("failed")
	}

	if len(validatorsDepositMap) <= MAX_VALIDATORS && len(resultMap) != len(validatorsDepositMap) {
		t.Fatalf("failed")
	}

	if len(validatorsDepositMap) > MAX_VALIDATORS && len(resultMap) != MAX_VALIDATORS {
		t.Fatalf("failed")
	}

	totalDeposit := big.NewInt(0)
	for val, _ := range validatorsDepositMap {
		depositValue, ok := validatorsDepositMap[val]
		if ok == false {
			t.Fatalf("unexpected validator")
		}

		totalDeposit = common.SafeAddBigInt(totalDeposit, depositValue)
	}
	fmt.Println("filteredDepositValue", filteredDepositValue, "totalDeposit", totalDeposit, "filteredValidatorCount", len(resultMap))

	if totalDeposit.Cmp(filteredDepositValue) < 0 {
		t.Fatalf("failed")
	}

	if MIN_BLOCK_DEPOSIT.Cmp(totalDeposit) > 0 {
		t.Fatalf("failed")
	}

	return filteredDepositValue
}

func TestFilterValidators_negative(t *testing.T) {
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, false)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})

	validatorsDepositMap[val1] = big.NewInt(1000000)
	validatorsDepositMap[val2] = big.NewInt(2000000)
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, false)

	validatorsDepositMap[val1] = big.NewInt(10000)
	validatorsDepositMap[val2] = big.NewInt(20000)
	validatorsDepositMap[val3] = big.NewInt(30000)
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, false)

	b := byte(0)
	for i := 0; i < MAX_VALIDATORS*2; i++ {
		val := common.BytesToAddress([]byte{b})
		validatorsDepositMap[val] = big.NewInt(1000)
		b = b + 1
	}
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, false)
}

func TestFilterValidators_positive(t *testing.T) {
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(100000000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(200000000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(400000000000))
	fmt.Println("Test1")
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, true)

	b := byte(0)
	for i := 0; i < MAX_VALIDATORS/2; i++ {
		val := common.BytesToAddress([]byte{b})
		validatorsDepositMap[val] = params.EtherToWei(big.NewInt(10000000000))
		b = b + 1
	}
	fmt.Println("Test2")
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, true)

	b = byte(0)
	for i := 0; i < MAX_VALIDATORS; i++ {
		val := common.BytesToAddress([]byte{b})
		validatorsDepositMap[val] = params.EtherToWei(big.NewInt(5000000000))
		b = b + 1
	}
	fmt.Println("Test3")
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, true)

	b = byte(0)
	for i := 0; i < MAX_VALIDATORS+1; i++ {
		val := common.BytesToAddress([]byte{b})
		validatorsDepositMap[val] = params.EtherToWei(big.NewInt(5000000000))
		b = b + 1
	}
	fmt.Println("Test4")
	testFilterValidatorsTest(t, consensusContext, validatorsDepositMap, true)
}

func TestFilterValidators_offline_validator(t *testing.T) {
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})
	val4 := common.BytesToAddress([]byte{4})

	validatorsDDetailsMap[val1] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock) + int64(10)),
	}

	validatorsDDetailsMap[val2] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD) - 1),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock) - 10),
	}

	validatorsDDetailsMap[val3] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(1),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock) - 100),
	}

	validatorsDDetailsMap[val4] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(100000000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(200000000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(400000000000))
	validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(500000000000))

	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed1")
	}

	_, ok := resultMap[val1]
	if ok == true {
		t.Fatalf("failed2")
	}

	if len(resultMap) != 3 {
		t.Fatalf("failed3")
	}

	if filteredDepositValue.Cmp(params.EtherToWei(big.NewInt(1100000000000))) != 0 {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue)
		t.Fatalf("failed4")
	}
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock
}

func TestFilterValidators_positive_Extended(t *testing.T) {
	parentHash := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	b := byte(0)
	for i := 0; i < MAX_VALIDATORS+1; i++ {
		val := common.BytesToAddress([]byte{b})
		validatorsDepositMap[val] = params.EtherToWei(big.NewInt(5000000000))
		b = b + 1
	}
	testFilterValidatorsTest(t, parentHash, validatorsDepositMap, true)
}

func TestFilterValidators_positive_second_pass(t *testing.T) {
	for test := 0; test < 2; test++ {
		validatorsDepositMap := make(map[common.Address]*big.Int)

		b := byte(0)
		for i := 1; i < 110; i++ {
			val := common.BytesToAddress([]byte{b})
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(int64(5000000 * i)))
			b = b + 1
		}

		for i := 1; i < 90; i++ {
			val := common.BytesToAddress([]byte{b, b})
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(20000000000))
			b = b + 1
		}

		consensusContext1 := common.BytesToHash([]byte{100})
		totalDeposit := testFilterValidatorsTest(t, consensusContext1, validatorsDepositMap, true)
		expected := params.EtherToWei(big.NewInt(1496770000000))
		if totalDeposit.Cmp(expected) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit), "expected", params.WeiToEther(expected))
			t.Fatalf("failed a")
		}

		consensusContext2 := common.BytesToHash([]byte{200})
		totalDeposit = testFilterValidatorsTest(t, consensusContext2, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(1595535000000))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed b")
		}

		consensusContext3 := common.BytesToHash([]byte{255})
		totalDeposit = testFilterValidatorsTest(t, consensusContext3, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(1592665000000))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed c")
		}
	}
}

func TestFilterValidators_positive_third_pass(t *testing.T) {
	for test := 0; test < 2; test++ {
		validatorsDepositMap := make(map[common.Address]*big.Int)

		b := byte(0)
		for i := 1; i < 256; i++ {
			val := common.BytesToAddress([]byte{b})
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(1000000000 + int64(i)))
			b = b + 1
		}

		for i := 1; i < 256; i++ {
			val := common.BytesToAddress([]byte{b, b})
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(20000000000 + int64(i)))
			b = b + 1
		}

		consensusContext1 := common.BytesToHash([]byte{100})
		totalDeposit := testFilterValidatorsTest(t, consensusContext1, validatorsDepositMap, true)
		expected := params.EtherToWei(big.NewInt(2123000022057))
		if totalDeposit.Cmp(expected) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit), "expected", params.WeiToEther(expected))
			t.Fatalf("failed a")
		}

		consensusContext2 := common.BytesToHash([]byte{200})
		totalDeposit = testFilterValidatorsTest(t, consensusContext2, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(2180000022119))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed b")
		}

		consensusContext3 := common.BytesToHash([]byte{255})
		totalDeposit = testFilterValidatorsTest(t, consensusContext3, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(2142000022027))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed c")
		}
	}
}

func TestFilterValidators_positive_real(t *testing.T) {
	for test := 0; test < 2; test++ {
		validatorsDepositMap := make(map[common.Address]*big.Int)

		b := byte(0)
		for i := 1; i < 10; i++ {
			val := common.BytesToAddress([]byte{b})
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(185000000000))
			b = b + 1
		}

		for i := 1; i < 256; i++ {
			val := common.BytesToAddress([]byte{b, b})
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(5000000 + int64(i)))
			b = b + 1
		}

		consensusContext1 := common.BytesToHash([]byte{100})
		totalDeposit := testFilterValidatorsTest(t, consensusContext1, validatorsDepositMap, true)
		expected := params.EtherToWei(big.NewInt(1480600018701))
		if totalDeposit.Cmp(expected) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit), "expected", params.WeiToEther(expected))
			t.Fatalf("failed a")
		}

		consensusContext2 := common.BytesToHash([]byte{200})
		totalDeposit = testFilterValidatorsTest(t, consensusContext2, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(1480600019514))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed b")
		}

		consensusContext3 := common.BytesToHash([]byte{255})
		totalDeposit = testFilterValidatorsTest(t, consensusContext3, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(1480600019249))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed c")
		}
	}
}

func TestFilterValidators_positive_low_balance(t *testing.T) {
	for test := 0; test < 2; test++ {
		validatorsDepositMap := make(map[common.Address]*big.Int)

		val1 := common.BytesToAddress([]byte{1})
		validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(1000))

		val2 := common.BytesToAddress([]byte{2})
		validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(900000000000))

		val3 := common.BytesToAddress([]byte{3})
		validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(10000000))

		val4 := common.BytesToAddress([]byte{4})
		validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(5000000))

		parentHash1 := common.BytesToHash([]byte{100})
		totalDeposit := testFilterValidatorsTest(t, parentHash1, validatorsDepositMap, true)
		if totalDeposit.Cmp(params.EtherToWei(big.NewInt(900015000000))) != 0 {
			fmt.Println("dep", params.WeiToEther(totalDeposit))
			t.Fatalf("failed")
		}
	}
}

func TestFilterValidators_positive_low_balance_negative_total(t *testing.T) {
	for test := 0; test < 2; test++ {
		validatorsDepositMap := make(map[common.Address]*big.Int)

		val1 := common.BytesToAddress([]byte{1})
		validatorsDepositMap[val1] = big.NewInt(1000)

		val2 := common.BytesToAddress([]byte{2})
		validatorsDepositMap[val2] = big.NewInt(100000)

		val3 := common.BytesToAddress([]byte{3})
		validatorsDepositMap[val3] = big.NewInt(200000)

		val4 := common.BytesToAddress([]byte{4})
		validatorsDepositMap[val4] = big.NewInt(300000)

		parentHash1 := common.BytesToHash([]byte{100})
		testFilterValidatorsTest(t, parentHash1, validatorsDepositMap, false)
	}
}

func TestFilterValidators_positive_low_balance_negative(t *testing.T) {
	for test := 0; test < 2; test++ {
		validatorsDepositMap := make(map[common.Address]*big.Int)

		b := byte(0)
		for i := 1; i < 255; i++ {
			val := common.BytesToAddress([]byte{b})
			validatorsDepositMap[val] = big.NewInt(1000)
			b = b + 1
		}

		val2 := common.BytesToAddress([]byte{1, 2})
		validatorsDepositMap[val2] = big.NewInt(100000)

		val3 := common.BytesToAddress([]byte{1, 3})
		validatorsDepositMap[val3] = big.NewInt(1000000)

		parentHash1 := common.BytesToHash([]byte{100})
		testFilterValidatorsTest(t, parentHash1, validatorsDepositMap, false)
	}
}

func testLargeValidator(t *testing.T, valCount uint64) {
	validatorsDepositMap := make(map[common.Address]*big.Int)

	for i := uint64(1); i < valCount; i++ {
		val := common.BytesToAddress(common.Uint64ToBytes(i))
		validatorsDepositMap[val] = params.EtherToWei(big.NewInt(50000000000 + int64(i)))
	}
	parentHash := common.BytesToHash([]byte{100})
	startTime := time.Now()
	testFilterValidatorsTest(t, parentHash, validatorsDepositMap, true)
	log.Info("large filter validator", "valCount", valCount, "time taken", time.Since(startTime))
}

func TestFilterValidators_positive_large(t *testing.T) {
	for i := uint64(32); i <= 65536; i = i * 2 {
		testLargeValidator(t, i)
	}
	testLargeValidator(t, 1000000)
}

func TestFilterValidators_offline_validator_sixty_seven(t *testing.T) {
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtySevenVoteStartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})
	val4 := common.BytesToAddress([]byte{4})

	validatorsDDetailsMap[val1] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.SixtySevenVoteStartBlock) + int64(10)),
	}

	validatorsDDetailsMap[val2] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD) - 1),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.SixtySevenVoteStartBlock) - 10),
	}

	validatorsDDetailsMap[val3] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(1),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.SixtySevenVoteStartBlock) - 100),
	}

	validatorsDDetailsMap[val4] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(100000000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(200000000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(400000000000))
	validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(500000000000))

	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.SixtySevenVoteStartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed1")
	}

	_, ok := resultMap[val1]
	if ok == true {
		t.Fatalf("failed2")
	}

	if len(resultMap) != 3 {
		t.Fatalf("failed3")
	}

	if filteredDepositValue.Cmp(params.EtherToWei(big.NewInt(1100000000000))) != 0 {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue)
		t.Fatalf("failed4")
	}
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock
}

func TestFilterValidators_offline_validator_penalty_error(t *testing.T) {
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})
	val4 := common.BytesToAddress([]byte{4})

	validatorsDDetailsMap[val1] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock) + int64(10)),
	}

	validatorsDDetailsMap[val2] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD) - 1),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock) - 10),
	}

	validatorsDDetailsMap[val3] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(1),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock) - 100),
	}

	validatorsDDetailsMap[val4] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(100000000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(200000000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(400000000000))
	validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(500000000000))

	_, _, _, err := filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, &validatorsDDetailsMap)
	if err == nil {
		log.Error("error nil when it should have failed")
		t.Fatalf("failed1")
	}
}

func TestFilterValidators_offline_validator_penalty(t *testing.T) {
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})
	val4 := common.BytesToAddress([]byte{4})
	val5 := common.BytesToAddress([]byte{5})

	validatorsDDetailsMap[val1] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(50),
		LastNiLBlock:  big.NewInt(50),
	}

	validatorsDDetailsMap[val2] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDDetailsMap[val3] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDDetailsMap[val4] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDDetailsMap[val5] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(4),
		LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock)),
	}

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(5000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(5000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(5000000))
	validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(5000000))
	validatorsDepositMap[val5] = params.EtherToWei(big.NewInt(900000000000))

	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed1")
	}

	_, ok := resultMap[val1]
	if ok == true {
		t.Fatalf("failed2")
	}

	if len(resultMap) != 4 {
		t.Fatalf("failed3")
	}

	if filteredDepositValue.String() != "828015000000000000000000000000" {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue.String())
		t.Fatalf("failed4")
	}
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock
}

func testNormalizeDeposit(items []int64, offline []bool, expected []int64) bool {
	valDepMap := make(map[common.Address]*big.Int)
	validatorDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	for i := 0; i < len(items); i++ {
		val := common.BytesToAddress([]byte{byte(i)})
		valDepMap[val] = big.NewInt(items[i])
		valDetails := &ValidatorDetailsV2{
			NilBlockCount: big.NewInt(0),
		}
		if offline[i] == true {
			valDetails.NilBlockCount = big.NewInt(1)
		}
		validatorDetailsMap[val] = valDetails
	}

	normalizeDeposit(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, &valDepMap, &validatorDetailsMap)
	for i := 0; i < len(items); i++ {
		val := common.BytesToAddress([]byte{byte(i)})
		if valDepMap[val].Cmp(big.NewInt(expected[i])) != 0 {
			fmt.Println("failed val", val, "actual", valDepMap[val], "want", expected[i])
			return false
		}
	}

	return true
}

func TestNormalizeDeposit(t *testing.T) {
	items := []int64{250, 130, 120, 110, 100, 100, 100, 50, 10, 10, 10, 5, 5}
	offline := []bool{false, true, false, false, false, false, false, true, true, true, true, false, false}
	expected := []int64{134, 100, 134, 134, 134, 134, 134, 50, 10, 10, 10, 6, 6}

	if testNormalizeDeposit(items, offline, expected) == false {
		t.Fatalf("failed")
	}
}

func TestFilterValidators_normalizedeposit(t *testing.T) {
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	for i := byte(0); i < 128; i++ {
		val := common.BytesToAddress([]byte{i})

		if i < 3 {
			validatorsDDetailsMap[val] = &ValidatorDetailsV2{
				NilBlockCount: big.NewInt(1),
				LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock) + int64(10)),
			}
		} else if i == 4 {
			validatorsDDetailsMap[val] = &ValidatorDetailsV2{
				NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
				LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock) + int64(10)),
			}
		} else {
			validatorsDDetailsMap[val] = &ValidatorDetailsV2{
				NilBlockCount: big.NewInt(0),
				LastNiLBlock:  big.NewInt(0),
			}
		}
		if i == 0 {
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(500000000000))
		} else if i >= 5 && i <= 9 {
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(100000000000))
		} else {
			validatorsDepositMap[val] = params.EtherToWei(big.NewInt(6000000000))
		}
	}

	//First test before OfflineValidatorV4StartBlock start block
	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed1")
	}

	if len(resultMap) != 127 {
		t.Fatalf("failed2")
	}

	log.Trace("filteredDepositValue", "value", filteredDepositValue)

	if filteredDepositValue.Cmp(params.EtherToWei(big.NewInt(1726000000000))) != 0 {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue)
		t.Fatalf("failed4")
	}

	//Now test after OfflineValidatorV4StartBlock start block
	resultMap, filteredDepositValue, _, err = filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed5")
	}

	if len(resultMap) != 127 {
		t.Fatalf("failed6")
	}

	log.Trace("filteredDepositValue", "value", filteredDepositValue)

	if filteredDepositValue.String() != "1725999999999999999999999999953" {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue.String())
		t.Fatalf("failed7")
	}

	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock
}

func TestFilterValidators_normalizedeposit_nochanges(t *testing.T) {
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	for i := byte(0); i < 128; i++ {
		val := common.BytesToAddress([]byte{i})

		if i < 3 {
			validatorsDDetailsMap[val] = &ValidatorDetailsV2{
				NilBlockCount: big.NewInt(1),
				LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock) + int64(10)),
			}
		} else if i == 4 {
			validatorsDDetailsMap[val] = &ValidatorDetailsV2{
				NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
				LastNiLBlock:  big.NewInt(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock) + int64(10)),
			}
		} else {
			validatorsDDetailsMap[val] = &ValidatorDetailsV2{
				NilBlockCount: big.NewInt(0),
				LastNiLBlock:  big.NewInt(0),
			}
		}
		validatorsDepositMap[val] = params.EtherToWei(big.NewInt(6000000000))
	}

	//First test before OfflineValidatorV4StartBlock start block
	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed1")
	}

	if len(resultMap) != 127 {
		t.Fatalf("failed2")
	}

	log.Trace("filteredDepositValue", "value", filteredDepositValue)

	if filteredDepositValue.String() != "762000000000000000000000000000" {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue)
		t.Fatalf("failed4")
	}

	//Now test after OfflineValidatorV4StartBlock start block
	resultMap, filteredDepositValue, _, err = filterValidators(consensusContext, &validatorsDepositMap, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, &validatorsDDetailsMap)
	if err != nil {
		log.Error("error", "msg", err)
		t.Fatalf("failed5")
	}

	if len(resultMap) != 127 {
		t.Fatalf("failed6")
	}

	log.Trace("filteredDepositValue", "value", filteredDepositValue)

	if filteredDepositValue.String() != "762000000000000000000000000000" {
		log.Info("filteredDepositValue", "filteredDepositValue", filteredDepositValue.String())
		t.Fatalf("failed7")
	}

	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.SixtyVoteStartBlock
}

// hardcodedValidatorListFilter returns a deterministic list of n validator addresses (Keccak256-based) for filter tests.
func hardcodedValidatorListFilter(n int) []common.Address {
	seed := []byte("filterValidatorsDeterminismTest")
	list := make([]common.Address, n)
	for i := 0; i < n; i++ {
		h := crypto.Keccak256Hash(seed, common.Uint64ToBytes(uint64(i)))
		list[i] = common.BytesToAddress(h.Bytes()[12:32])
	}
	return list
}

// buildFilterValidatorsInput_1000Validators builds deposit map and details map for 1000 validators in 10 groups of same deposit.
// Returns (validatorsDepositMap, validatorsDDetailsMap). Caller must copy deposit map before each filterValidators call (it mutates).
func buildFilterValidatorsInput_1000Validators(validatorList []common.Address, iter int) (map[common.Address]*big.Int, map[common.Address]*ValidatorDetailsV2) {
	const numValidators = 1000
	const numGroups = 10
	const perGroup = 100
	validatorsDepositMap := make(map[common.Address]*big.Int)
	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
	// 10 sets of validators with same deposit: group 0 = 500B, groups 1-9 = 10B each (so normalizeDeposit can run and we hit all passes).
	depositsPerGroup := [numGroups]*big.Int{
		params.EtherToWei(big.NewInt(500000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
		params.EtherToWei(big.NewInt(10000000000)),
	}
	// Indices 50-59 in group 0 have NilBlockCount=1 so normalizeDeposit second round hits "if NilBlockCount > 0 { continue }" (skip offline for increase).
	const nilBlockCountStart, nilBlockCountEnd = 50, 60
	for i := 0; i < numValidators; i++ {
		idx := (i + iter) % numValidators
		addr := validatorList[idx]
		group := idx / perGroup
		validatorsDepositMap[addr] = new(big.Int).Set(depositsPerGroup[group])
		nilBlockCount := new(big.Int)
		if idx >= nilBlockCountStart && idx < nilBlockCountEnd {
			nilBlockCount = big.NewInt(1)
		}
		validatorsDDetailsMap[addr] = &ValidatorDetailsV2{
			Validator:     addr,
			LastNiLBlock:  new(big.Int),
			NilBlockCount: nilBlockCount,
		}
	}
	return validatorsDepositMap, validatorsDDetailsMap
}

func sortedSelectedAddressesHex(resultMap map[common.Address]bool) []string {
	out := make([]string, 0, len(resultMap))
	for addr := range resultMap {
		out = append(out, addr.Hex())
	}
	sort.Strings(out)
	return out
}

// Hardcoded expected output for TestFilterValidatorsDeterminism_1000Validators (block OfflineValidatorV4StartBlock, 10 groups same deposit).
const (
	expectedFilteredDepositValue_1000Validators  = "42440000000000000000000000000000"
	expectedSelectedAddressesHash_1000Validators = "0xb7a2299326e3b776544c010cf0b538040937631ed37b57328dfb1d370c62ae6b"
)

// Branch coverage for filterValidators and normalizeDeposit in TestFilterValidatorsDeterminism_1000Validators:
//
// filterValidators:
//   - depositValue < MIN_VALIDATOR_DEPOSIT → toRemove: NOT HIT (all deposits >= 10B)
//   - blockNumber >= OfflineValidatorDeferStartBlock: HIT
//   - canValidate == false → toRemove: NOT HIT (all canValidate true; NilBlockCount 0 or 1 < threshold)
//   - blockNumber >= OfflineValidatorV4StartBlock: HIT
//   - after penalty depositValue < MIN → toRemove: NOT HIT (no penalty below min)
//   - valCount < MIN_VALIDATORS: NOT HIT
//   - totalDepositValue < MIN_BLOCK_DEPOSIT: NOT HIT
//   - len <= MAX_VALIDATORS (take all): NOT HIT (we have 1000 > 128)
//   - else getMaxFilteredValidators: HIT
//   - filteredDepositValue < MIN_BLOCK_DEPOSIT after normalize: NOT HIT here; see TestFilterValidators_filteredDepositValueBelowMin_afterNormalize
//   - minPercentage: SixtySevenVoteStartBlock branch HIT; SixtyVote/else NOT HIT (block is past SixtySeven)
//
// normalizeDeposit:
//   - blockNumber < OfflineValidatorV4StartBlock → return: NOT HIT
//   - len < MIN_VALIDATORS_NORMALIZATION → return: NOT HIT (1000 >= 12)
//   - amt > maxCoins (first round reduction): HIT
//   - NilBlockCount == 0 → add to nonOfflineCoinsAfterReduction: HIT (most validators)
//   - hasChanges == false → return: NOT HIT
//   - Second round NilBlockCount > 0 → continue (skip offline): HIT (we add 10 validators with NilBlockCount=1)
func TestFilterValidatorsDeterminism_1000Validators(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	const numValidators = 1000
	blockNumber := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	consensusContext := common.BytesToHash([]byte("filterDeterminismConsensusContext"))
	validatorList := hardcodedValidatorListFilter(numValidators)

	for iter := 0; iter < 1000; iter++ {
		validatorsDepositMap, validatorsDDetailsMap := buildFilterValidatorsInput_1000Validators(validatorList, iter)
		resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, blockNumber, &validatorsDDetailsMap)
		if err != nil {
			t.Fatalf("iteration %d: filterValidators failed: %v", iter, err)
		}
		depositStr := filteredDepositValue.String()
		sortedHex := sortedSelectedAddressesHex(resultMap)
		addressesHash := crypto.Keccak256Hash([]byte(fmt.Sprint(sortedHex))).Hex()

		if depositStr != expectedFilteredDepositValue_1000Validators {
			t.Fatalf("iteration %d: filteredDepositValue %s != expected %s", iter, depositStr, expectedFilteredDepositValue_1000Validators)
		}
		if addressesHash != expectedSelectedAddressesHash_1000Validators {
			t.Fatalf("iteration %d: selected addresses hash %s != expected %s", iter, addressesHash, expectedSelectedAddressesHash_1000Validators)
		}
	}
}

// Scenario A: 1,000 validators, widely distributed stake. Pass 1 hits the 42-slot cap.
// Expected: Pass 1 validators (~42) get proposer rate ~42/128, Pass 2 (~42) get ~42/128,
// Pass 3 (~44) get ~44/128 of total proposer selections.
func TestFilterValidators_ProposerProbability_ScenarioA(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	const numValidators = 1000
	const numBlocks = 10000
	const round = byte(1)
	baseBlockNumber := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock

	validatorList := hardcodedValidatorListFilter(numValidators)
	proposerCount := make(map[common.Address]int)
	committeeCount := make(map[common.Address]int)

	for block := uint64(0); block < numBlocks; block++ {
		blockNumber := baseBlockNumber + block
		consensusContext := crypto.Keccak256Hash([]byte("scenarioA"), common.Uint64ToBytes(blockNumber))

		validatorsDepositMap := make(map[common.Address]*big.Int)
		validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
		for i := 0; i < numValidators; i++ {
			addr := validatorList[i]
			validatorsDepositMap[addr] = params.EtherToWei(big.NewInt(int64(5000000000 + i)))
			validatorsDDetailsMap[addr] = &ValidatorDetailsV2{
				Validator:     addr,
				NilBlockCount: big.NewInt(0),
				LastNiLBlock:  big.NewInt(0),
			}
		}

		resultMap, _, _, err := filterValidators(consensusContext, &validatorsDepositMap, blockNumber, &validatorsDDetailsMap)
		if err != nil {
			t.Fatalf("block %d: filterValidators failed: %v", block, err)
		}
		if len(resultMap) != MAX_VALIDATORS {
			t.Fatalf("block %d: expected %d validators, got %d", block, MAX_VALIDATORS, len(resultMap))
		}

		for addr := range resultMap {
			committeeCount[addr]++
		}

		committeeDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
		for addr := range resultMap {
			committeeDetailsMap[addr] = validatorsDDetailsMap[addr]
		}
		proposer, _, err := getBlockProposer(common.Hash{}, nil, round, &committeeDetailsMap, blockNumber, consensusContext)
		if err != nil {
			t.Fatalf("block %d: getBlockProposer failed: %v", block, err)
		}
		proposerCount[proposer]++
	}

	pass1Total := 0
	pass2Total := 0
	pass3Total := 0
	for i := 0; i < numValidators; i++ {
		rate := float64(committeeCount[validatorList[i]]) / float64(numBlocks)
		cnt := proposerCount[validatorList[i]]
		if rate > 0.95 {
			pass1Total += cnt
		} else if rate > 0.35 {
			pass2Total += cnt
		} else {
			pass3Total += cnt
		}
	}

	pass1Rate := float64(pass1Total) / float64(numBlocks)
	pass2Rate := float64(pass2Total) / float64(numBlocks)
	pass3Rate := float64(pass3Total) / float64(numBlocks)

	expectedPass1 := float64(42) / float64(128)
	expectedPass2 := float64(42) / float64(128)
	expectedPass3 := float64(44) / float64(128)

	fmt.Printf("Scenario A (1,000 validators, widely distributed stake):\n")
	fmt.Printf("  Pass 1 proposer rate: %.4f (expected ~%.4f)\n", pass1Rate, expectedPass1)
	fmt.Printf("  Pass 2 proposer rate: %.4f (expected ~%.4f)\n", pass2Rate, expectedPass2)
	fmt.Printf("  Pass 3 proposer rate: %.4f (expected ~%.4f)\n", pass3Rate, expectedPass3)

	tolerance := 0.08
	if pass1Rate < expectedPass1-tolerance || pass1Rate > expectedPass1+tolerance {
		t.Fatalf("Scenario A: Pass 1 proposer rate %.4f outside tolerance of expected %.4f", pass1Rate, expectedPass1)
	}
	if pass2Rate < expectedPass2-tolerance || pass2Rate > expectedPass2+tolerance {
		t.Fatalf("Scenario A: Pass 2 proposer rate %.4f outside tolerance of expected %.4f", pass2Rate, expectedPass2)
	}
	if pass3Rate < expectedPass3-tolerance || pass3Rate > expectedPass3+tolerance {
		t.Fatalf("Scenario A: Pass 3 proposer rate %.4f outside tolerance of expected %.4f", pass3Rate, expectedPass3)
	}
}

// Scenario B: 1,000 validators, top 18 hold >85% of total deposit. Pass 1 stops at deposit threshold (18).
// Pass 2 fills up to 66 slots (84-18), Pass 3 fills 44. Proposer rate per pass: 18/128, 66/128, 44/128.
func TestFilterValidators_ProposerProbability_ScenarioB(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	const numValidators = 1000
	const numBlocks = 10000
	const round = byte(1)
	const topCount = 18
	baseBlockNumber := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock

	validatorList := hardcodedValidatorListFilter(numValidators)
	proposerCount := make(map[common.Address]int)
	committeeCount := make(map[common.Address]int)

	for block := uint64(0); block < numBlocks; block++ {
		blockNumber := baseBlockNumber + block
		consensusContext := crypto.Keccak256Hash([]byte("scenarioB"), common.Uint64ToBytes(blockNumber))

		validatorsDepositMap := make(map[common.Address]*big.Int)
		validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
		for i := 0; i < numValidators; i++ {
			addr := validatorList[i]
			var deposit *big.Int
			if i < topCount {
				deposit = params.EtherToWei(big.NewInt(100000000000))
			} else {
				deposit = params.EtherToWei(big.NewInt(5000000 + int64(i)))
			}
			validatorsDepositMap[addr] = deposit
			validatorsDDetailsMap[addr] = &ValidatorDetailsV2{
				Validator:     addr,
				NilBlockCount: big.NewInt(0),
				LastNiLBlock:  big.NewInt(0),
			}
		}

		resultMap, _, _, err := filterValidators(consensusContext, &validatorsDepositMap, blockNumber, &validatorsDDetailsMap)
		if err != nil {
			t.Fatalf("block %d: filterValidators failed: %v", block, err)
		}
		if len(resultMap) != MAX_VALIDATORS {
			t.Fatalf("block %d: expected %d validators, got %d", block, MAX_VALIDATORS, len(resultMap))
		}

		for addr := range resultMap {
			committeeCount[addr]++
		}

		committeeDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
		for addr := range resultMap {
			committeeDetailsMap[addr] = validatorsDDetailsMap[addr]
		}
		proposer, _, err := getBlockProposer(common.Hash{}, nil, round, &committeeDetailsMap, blockNumber, consensusContext)
		if err != nil {
			t.Fatalf("block %d: getBlockProposer failed: %v", block, err)
		}
		proposerCount[proposer]++
	}

	pass1Total := 0
	pass1CommitteeTotal := 0
	for i := 0; i < topCount; i++ {
		pass1Total += proposerCount[validatorList[i]]
		pass1CommitteeTotal += committeeCount[validatorList[i]]
	}

	pass2Total := 0
	pass3Total := 0
	for i := topCount; i < numValidators; i++ {
		rate := float64(committeeCount[validatorList[i]]) / float64(numBlocks)
		cnt := proposerCount[validatorList[i]]
		if rate > 0.35 {
			pass2Total += cnt
		} else {
			pass3Total += cnt
		}
	}

	pass1Rate := float64(pass1Total) / float64(numBlocks)
	pass2Rate := float64(pass2Total) / float64(numBlocks)
	pass3Rate := float64(pass3Total) / float64(numBlocks)
	pass1CommitteeRate := float64(pass1CommitteeTotal) / float64(topCount) / float64(numBlocks)

	pass2Slots := 84 - topCount
	expectedPass1 := float64(topCount) / float64(128)
	expectedPass2 := float64(pass2Slots) / float64(128)
	expectedPass3 := float64(128-84) / float64(128)

	fmt.Printf("Scenario B (1,000 validators, top %d hold >85%% deposit):\n", topCount)
	fmt.Printf("  Pass 1: %d validators, committee rate: %.4f, proposer rate: %.4f (expected ~%.4f)\n",
		topCount, pass1CommitteeRate, pass1Rate, expectedPass1)
	fmt.Printf("  Pass 2: proposer rate: %.4f (expected ~%.4f)\n", pass2Rate, expectedPass2)
	fmt.Printf("  Pass 3: proposer rate: %.4f (expected ~%.4f)\n", pass3Rate, expectedPass3)

	if pass1CommitteeRate < 0.95 {
		t.Fatalf("Scenario B: Pass 1 validators should be selected ~100%%, got %.4f", pass1CommitteeRate)
	}
	tolerance := 0.08
	if pass1Rate < expectedPass1-tolerance || pass1Rate > expectedPass1+tolerance {
		t.Fatalf("Scenario B: Pass 1 proposer rate %.4f outside tolerance of expected %.4f", pass1Rate, expectedPass1)
	}
	if pass2Rate < expectedPass2-tolerance || pass2Rate > expectedPass2+tolerance {
		t.Fatalf("Scenario B: Pass 2 proposer rate %.4f outside tolerance of expected %.4f", pass2Rate, expectedPass2)
	}
	if pass3Rate < expectedPass3-tolerance || pass3Rate > expectedPass3+tolerance {
		t.Fatalf("Scenario B: Pass 3 proposer rate %.4f outside tolerance of expected %.4f", pass3Rate, expectedPass3)
	}
}

// Scenario C: 100,000 validators, widely distributed stake.
// Pass 3 validators have very low individual selection probability (~44/99874 = ~0.04%).
// Uses fewer blocks due to O(N log N) sorting cost per block with 100K validators.
func TestFilterValidators_ProposerProbability_ScenarioC(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	const numValidators = 100000
	const numBlocks = 50
	const round = byte(1)
	baseBlockNumber := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock

	validatorList := hardcodedValidatorListFilter(numValidators)

	depositMap := make(map[common.Address]*big.Int, numValidators)
	detailsMap := make(map[common.Address]*ValidatorDetailsV2, numValidators)
	for i := 0; i < numValidators; i++ {
		addr := validatorList[i]
		depositMap[addr] = params.EtherToWei(big.NewInt(int64(5000000000 + i)))
		detailsMap[addr] = &ValidatorDetailsV2{
			Validator:     addr,
			NilBlockCount: big.NewInt(0),
			LastNiLBlock:  big.NewInt(0),
		}
	}

	proposerCount := make(map[common.Address]int)
	committeeCount := make(map[common.Address]int)

	for block := uint64(0); block < numBlocks; block++ {
		blockNumber := baseBlockNumber + block
		consensusContext := crypto.Keccak256Hash([]byte("scenarioC"), common.Uint64ToBytes(blockNumber))

		depositCopy := make(map[common.Address]*big.Int, numValidators)
		for k, v := range depositMap {
			depositCopy[k] = new(big.Int).Set(v)
		}
		detailsCopy := make(map[common.Address]*ValidatorDetailsV2, numValidators)
		for k, v := range detailsMap {
			detailsCopy[k] = v
		}

		resultMap, _, _, err := filterValidators(consensusContext, &depositCopy, blockNumber, &detailsCopy)
		if err != nil {
			t.Fatalf("block %d: filterValidators failed: %v", block, err)
		}
		if len(resultMap) != MAX_VALIDATORS {
			t.Fatalf("block %d: expected %d validators, got %d", block, MAX_VALIDATORS, len(resultMap))
		}

		for addr := range resultMap {
			committeeCount[addr]++
		}

		committeeDetailsMap := make(map[common.Address]*ValidatorDetailsV2, len(resultMap))
		for addr := range resultMap {
			committeeDetailsMap[addr] = detailsMap[addr]
		}
		proposer, _, err := getBlockProposer(common.Hash{}, nil, round, &committeeDetailsMap, blockNumber, consensusContext)
		if err != nil {
			t.Fatalf("block %d: getBlockProposer failed: %v", block, err)
		}
		proposerCount[proposer]++
	}

	pass1Proposers := 0
	pass2Proposers := 0
	pass3Proposers := 0
	pass1Count := 0
	pass2Count := 0
	pass3Count := 0
	uniqueCommitteeMembers := 0

	for i := 0; i < numValidators; i++ {
		cc := committeeCount[validatorList[i]]
		pc := proposerCount[validatorList[i]]
		if cc == 0 {
			continue
		}
		uniqueCommitteeMembers++
		rate := float64(cc) / float64(numBlocks)
		// With 50 blocks: Pass 1 validators appear in all 50 (rate 1.0),
		// Pass 2 validators appear ~50% of the time (rate ~0.5),
		// Pass 3 validators appear rarely (rate < 0.10 for 100K pool).
		if rate > 0.90 {
			pass1Count++
			pass1Proposers += pc
		} else if rate > 0.10 {
			pass2Count++
			pass2Proposers += pc
		} else {
			pass3Count++
			pass3Proposers += pc
		}
	}

	pass1Rate := float64(pass1Proposers) / float64(numBlocks)
	pass2Rate := float64(pass2Proposers) / float64(numBlocks)
	pass3Rate := float64(pass3Proposers) / float64(numBlocks)

	expectedPass1 := float64(42) / float64(128)
	expectedPass2 := float64(42) / float64(128)
	expectedPass3 := float64(44) / float64(128)

	fmt.Printf("Scenario C (100,000 validators, widely distributed stake, %d blocks):\n", numBlocks)
	fmt.Printf("  Pass 1: %d validators, proposer rate: %.4f (expected ~%.4f)\n", pass1Count, pass1Rate, expectedPass1)
	fmt.Printf("  Pass 2: %d validators, proposer rate: %.4f (expected ~%.4f)\n", pass2Count, pass2Rate, expectedPass2)
	fmt.Printf("  Pass 3: %d validators, proposer rate: %.4f (expected ~%.4f)\n", pass3Count, pass3Rate, expectedPass3)
	fmt.Printf("  Unique validators seen in committee across %d blocks: %d / %d\n",
		numBlocks, uniqueCommitteeMembers, numValidators)

	// Wider tolerance due to low block count (50 blocks = each proposer event is 2% of total).
	tolerance := 0.20
	if pass1Rate < expectedPass1-tolerance || pass1Rate > expectedPass1+tolerance {
		t.Fatalf("Scenario C: Pass 1 proposer rate %.4f outside tolerance of expected %.4f", pass1Rate, expectedPass1)
	}
	if pass2Rate < expectedPass2-tolerance || pass2Rate > expectedPass2+tolerance {
		t.Fatalf("Scenario C: Pass 2 proposer rate %.4f outside tolerance of expected %.4f", pass2Rate, expectedPass2)
	}
	if pass3Rate < expectedPass3-tolerance || pass3Rate > expectedPass3+tolerance {
		t.Fatalf("Scenario C: Pass 3 proposer rate %.4f outside tolerance of expected %.4f", pass3Rate, expectedPass3)
	}
	if pass1Count < 35 {
		t.Fatalf("Scenario C: Expected ~42 Pass 1 validators, got %d", pass1Count)
	}
	if uniqueCommitteeMembers < 128 {
		t.Fatalf("Scenario C: Too few unique committee members: %d", uniqueCommitteeMembers)
	}
}

// Scenario D: 100 validators (<=128). All are selected every block. Equal proposer probability 1/N.
func TestFilterValidators_ProposerProbability_ScenarioD(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	const numValidators = 100
	const numBlocks = 10000
	const round = byte(1)
	baseBlockNumber := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock

	validatorList := hardcodedValidatorListFilter(numValidators)
	proposerCount := make(map[common.Address]int)
	committeeCount := make(map[common.Address]int)

	for block := uint64(0); block < numBlocks; block++ {
		blockNumber := baseBlockNumber + block
		consensusContext := crypto.Keccak256Hash([]byte("scenarioD"), common.Uint64ToBytes(blockNumber))

		validatorsDepositMap := make(map[common.Address]*big.Int)
		validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
		for i := 0; i < numValidators; i++ {
			addr := validatorList[i]
			validatorsDepositMap[addr] = params.EtherToWei(big.NewInt(int64(10000000000 + i)))
			validatorsDDetailsMap[addr] = &ValidatorDetailsV2{
				Validator:     addr,
				NilBlockCount: big.NewInt(0),
				LastNiLBlock:  big.NewInt(0),
			}
		}

		resultMap, _, _, err := filterValidators(consensusContext, &validatorsDepositMap, blockNumber, &validatorsDDetailsMap)
		if err != nil {
			t.Fatalf("block %d: filterValidators failed: %v", block, err)
		}
		if len(resultMap) != numValidators {
			t.Fatalf("block %d: expected all %d validators, got %d", block, numValidators, len(resultMap))
		}

		for addr := range resultMap {
			committeeCount[addr]++
		}

		committeeDetailsMap := make(map[common.Address]*ValidatorDetailsV2)
		for addr := range resultMap {
			committeeDetailsMap[addr] = validatorsDDetailsMap[addr]
		}
		proposer, _, err := getBlockProposer(common.Hash{}, nil, round, &committeeDetailsMap, blockNumber, consensusContext)
		if err != nil {
			t.Fatalf("block %d: getBlockProposer failed: %v", block, err)
		}
		proposerCount[proposer]++
	}

	for i := 0; i < numValidators; i++ {
		if committeeCount[validatorList[i]] != numBlocks {
			t.Fatalf("Scenario D: validator %d not selected every block (got %d/%d)", i, committeeCount[validatorList[i]], numBlocks)
		}
	}

	expectedRate := 1.0 / float64(numValidators)
	tolerance := 0.03
	for i := 0; i < numValidators; i++ {
		rate := float64(proposerCount[validatorList[i]]) / float64(numBlocks)
		if rate < expectedRate-tolerance || rate > expectedRate+tolerance {
			t.Fatalf("Scenario D: validator %d proposer rate %.4f outside tolerance of expected %.4f", i, rate, expectedRate)
		}
	}

	fmt.Printf("Scenario D (%d validators, all selected every block):\n", numValidators)
	fmt.Printf("  All validators in committee every block: true\n")
	fmt.Printf("  Expected proposer rate: %.4f, tolerance: %.4f\n", expectedRate, tolerance)
}

// TestFilterValidators_filteredDepositValueBelowMin_afterNormalize hits the branch where
// filteredDepositValue < MIN_BLOCK_DEPOSIT after normalizeDeposit (returns error).
// Setup: 100001 validators — 100000 with 5M ether and NilBlockCount 0, 1 with 5M and NilBlockCount 50.
// The NilBlockCount 50 validator gets penalty to 0 and is removed. Remaining 100000 × 5M = 500B (totalDepositValue >= MIN_BLOCK_DEPOSIT).
// Selected 128 all have 5M each → filteredDepositValue = 128×5M = 640M < MIN_BLOCK_DEPOSIT (500B).
func TestFilterValidators_filteredDepositValueBelowMin_afterNormalize(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	const (
		numValidatorsSameDeposit = 100000
		totalValidators          = numValidatorsSameDeposit + 1
	)
	deposit5M := params.EtherToWei(big.NewInt(5000000))
	blockNumber := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	consensusContext := common.BytesToHash([]byte("filteredDepositBelowMinTest"))

	validatorList := hardcodedValidatorListFilter(totalValidators)
	validatorsDepositMap := make(map[common.Address]*big.Int)
	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	for i := 0; i < totalValidators; i++ {
		addr := validatorList[i]
		validatorsDepositMap[addr] = new(big.Int).Set(deposit5M)
		nilCount := new(big.Int)
		if i == totalValidators-1 {
			nilCount = big.NewInt(50)
		}
		validatorsDDetailsMap[addr] = &ValidatorDetailsV2{
			Validator:     addr,
			LastNiLBlock:  new(big.Int),
			NilBlockCount: nilCount,
		}
	}

	_, _, _, err := filterValidators(consensusContext, &validatorsDepositMap, blockNumber, &validatorsDDetailsMap)
	if err == nil {
		t.Fatalf("expected filterValidators to fail with filteredDepositValue < MIN_BLOCK_DEPOSIT after normalize")
	}
	if err.Error() != "min block deposit not met for filteredDepositValue" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFilterValidators_normalizationv2 verifies the Normalizationv2StartBlock gate:
// after the cutoff, normalizeDeposit runs only over the selected committee instead of all validators,
// which changes filteredDepositValue when there are more than MAX_VALIDATORS validators and a whale
// exceeds the MAX_DEPOSIT_PERCENTAGE cap. Below MAX_VALIDATORS (committee == full set) the result is
// unchanged across the boundary.
//
// The non-whale validators all hold an identical deposit, so the committee's pre-normalize total is the
// same regardless of which equal validators selection picks for each block. That isolates the difference
// to the normalization scope rather than committee membership.
func TestFilterValidators_normalizationv2(t *testing.T) {
	origBlockNumber := TestFilterValidatorsBlockNumber
	TestFilterValidatorsBlockNumber = defaults.DefaultConfig.PosConfig.Normalizationv2StartBlock
	defer func() { TestFilterValidatorsBlockNumber = origBlockNumber }()

	consensusContext := common.BytesToHash([]byte("normalizationv2GateTest"))
	nb := defaults.DefaultConfig.PosConfig.Normalizationv2StartBlock

	buildInput := func(n int, whaleCoins, equalCoins int64) (map[common.Address]*big.Int, map[common.Address]*ValidatorDetailsV2) {
		list := hardcodedValidatorListFilter(n)
		dep := make(map[common.Address]*big.Int)
		det := make(map[common.Address]*ValidatorDetailsV2)
		for i := 0; i < n; i++ {
			addr := list[i]
			coins := equalCoins
			if i == 0 {
				coins = whaleCoins
			}
			dep[addr] = params.EtherToWei(big.NewInt(coins))
			det[addr] = &ValidatorDetailsV2{
				Validator:     addr,
				NilBlockCount: big.NewInt(0),
				LastNiLBlock:  big.NewInt(0),
			}
		}
		return dep, det
	}

	// Whale and equal-deposit amounts are sized so the total and committee deposits clear MIN_BLOCK_DEPOSIT
	// while the whale stays above the MAX_DEPOSIT_PERCENTAGE (10%) cap.
	const whaleCoins = int64(250000000000)
	const equalCoins = int64(5000000000)

	// Case 1: 300 validators (> MAX_VALIDATORS) with a whale above the 10% cap. The result must differ
	// across the Normalizationv2StartBlock boundary (normalize-all vs normalize-committee).
	depA, detA := buildInput(300, whaleCoins, equalCoins)
	_, fdvBefore, _, err := filterValidators(consensusContext, &depA, nb-1, &detA)
	if err != nil {
		t.Fatalf("case1 before boundary: %v", err)
	}
	depB, detB := buildInput(300, whaleCoins, equalCoins)
	_, fdvAfter, _, err := filterValidators(consensusContext, &depB, nb, &detB)
	if err != nil {
		t.Fatalf("case1 after boundary: %v", err)
	}
	if fdvBefore.Cmp(fdvAfter) == 0 {
		t.Fatalf("case1: expected filteredDepositValue to differ across Normalizationv2StartBlock boundary; both = %s", fdvBefore)
	}

	// Case 2: 100 validators (<= MAX_VALIDATORS) => committee == full set => normalization scope is the
	// same on both sides of the boundary => filteredDepositValue unchanged.
	depC, detC := buildInput(100, whaleCoins, equalCoins)
	_, fdvSmallBefore, _, err := filterValidators(consensusContext, &depC, nb-1, &detC)
	if err != nil {
		t.Fatalf("case2 before boundary: %v", err)
	}
	depD, detD := buildInput(100, whaleCoins, equalCoins)
	_, fdvSmallAfter, _, err := filterValidators(consensusContext, &depD, nb, &detD)
	if err != nil {
		t.Fatalf("case2 after boundary: %v", err)
	}
	if fdvSmallBefore.Cmp(fdvSmallAfter) != 0 {
		t.Fatalf("case2: expected filteredDepositValue to be unchanged across boundary for <= MAX_VALIDATORS; before=%s after=%s", fdvSmallBefore, fdvSmallAfter)
	}
}

// block4203208PageDepositsWholeCoins holds the "Staking Net Balance after Decay" (whole-coin amounts) of
// the 299 active validators from the quantumscan validator-page snapshot taken around block 4,203,208.
// Sub-coin decimals are dropped (this is a printonly demonstration). Real validator addresses are NOT
// used; fresh deterministic test addresses are generated instead.
var block4203208PageDepositsWholeCoins = []int64{
	278821098145, 272407360869, 267626587075, 266013974808, 215918264280, 215624454907, 214970404460, 214741281562,
	65203750710, 64286830136, 62093450149, 61699747029, 61507121339, 60195890476, 57525489153, 49580848107,
	28991998369, 27901915195, 23333864289, 22301708060, 19153530983, 16114134277, 12886083630, 12002984042,
	11851380808, 11445897772, 10390239040, 10005783657, 9204865494, 9117603545, 9117114617, 9090556864,
	8937990692, 8908924119, 8805745928, 8673650910, 8568436552, 8210355292, 8078005988, 7529214040,
	7448764847, 6765047427, 6684483212, 6430318553, 6210746733, 6174100294, 6066849239, 5974339715,
	5953676299, 5927842642, 5758358254, 5703075826, 5563568541, 5541019098, 5485342613, 5468480905,
	5460551458, 5426513743, 5420046271, 5398027379, 5365943246, 5355714081, 5341302443, 5338926064,
	5210739088, 5189148079, 5126421831, 5096121750, 5085937306, 5070395438, 4990459656, 4950573399,
	4947307918, 4908700623, 4880838204, 4849131705, 4824174254, 4797381505, 4693460208, 4691546018,
	4678618453, 4669644068, 4639146367, 4626129484, 4512015436, 4500929528, 4458575430, 4433506109,
	4412843044, 4376924546, 4365421210, 4348633832, 4340612506, 4338235814, 4337399311, 4321884768,
	4307996921, 4297435221, 4289183861, 4286306618, 4270718927, 4261544720, 4248946336, 4217229372,
	4202400375, 4199630428, 4190277589, 4184376443, 4179166021, 4176782182, 4167977157, 4160483908,
	4129729426, 4088207130, 4077871320, 4076310897, 4051684733, 4046270836, 4040693483, 4017947041,
	4008804675, 3994048173, 3977331625, 3973379092, 3965646258, 3940178907, 3926341991, 3888596708,
	3829343760, 3806625010, 3774405691, 3708915520, 3663568463, 3615207069, 3535944682, 3491846591,
	3468912108, 3444877045, 3408655588, 3108035647, 3045159254, 2957450704, 2953498059, 2927910384,
	2861080678, 2838277761, 2721809082, 2694754790, 2671419373, 2642180170, 2602681981, 2579535561,
	2540059150, 2512396942, 2459256846, 2433357536, 2242234344, 2191744858, 2131438893, 2098447095,
	1975509830, 1858601403, 1850689761, 1743175760, 1694773131, 1622571489, 1539405665, 1537308993,
	1445875524, 1376244841, 1251840032, 1229065020, 1193143598, 1190504688, 1178645072, 1158286587,
	1158245642, 1156812819, 1099166849, 1098698727, 1096134160, 1043361943, 1001850399, 962208184,
	932226397, 925104533, 893309967, 887996341, 876772962, 875058307, 871822223, 867933498,
	857792560, 854421752, 852584608, 849060468, 841802113, 832648158, 825961908, 802296810,
	799335043, 795712411, 791406881, 768485865, 765777052, 750999586, 717704219, 702229408,
	699743089, 696790270, 688205607, 684670890, 681589950, 677470525, 674402594, 674144258,
	668239475, 656929272, 656819150, 653967568, 633042085, 631128620, 608634811, 604219544,
	596891502, 587853531, 583570603, 566202686, 550281302, 544566173, 533144462, 521383623,
	515077180, 513167493, 467516099, 432320599, 420880635, 411485305, 364691585, 361600268,
	354462395, 352901534, 328517729, 327258479, 323320265, 320188582, 318185526, 317152025,
	299665565, 286479021, 286298692, 284136422, 273498870, 257285677, 256335795, 233883463,
	226717091, 210639227, 201685931, 200592699, 200425110, 198305084, 197887899, 197027720,
	192759230, 190974965, 177709249, 176475219, 170268960, 170145428, 166359326, 163270907,
	149436126, 144498636, 142755220, 139470593, 122313691, 121823062, 110453546, 108815682,
	101866994, 85716100, 80200223, 78987795, 75545725, 72985687, 66360188, 62562630,
	60288594, 60022524, 57551024, 55407486, 54316966, 42343588, 39468908, 38932360,
	25118634, 8782332, 5255709,
}

// TestNormalizeDeposit_Block4203208_CurrentVsNew demonstrates the Normalizationv2 behavior change on a
// realistic 304-validator universe modeled on block 4,203,208: the 299 active validators from the
// validator page, 4 small filler validators (to reach the page's reported 303 total), plus 1 synthetic
// "whale" validator holding 20% of the universe deposit.
//
// The 128-member committee is the whale plus the 127 largest validators. It prints, per committee
// validator, the deposit share under the current behavior (normalize over all 304) versus the new
// behavior (normalize over the committee only). All validators are treated as online (NilBlockCount 0).
func TestNormalizeDeposit_Block4203208_CurrentVsNew(t *testing.T) {
	blockNumber := defaults.DefaultConfig.PosConfig.Normalizationv2StartBlock

	// 299 real page deposits + 4 small fillers => 303 page-side validators.
	deposits := append([]int64{}, block4203208PageDepositsWholeCoins...)
	deposits = append(deposits, 5000000, 4000000, 3000000, 2000000)

	universe := make(map[common.Address]*big.Int, len(deposits)+1)
	details := make(map[common.Address]*ValidatorDetailsV2, len(deposits)+1)
	addrs := make([]common.Address, 0, len(deposits)+1)
	seed := []byte("normalizationv2Block4203208Test")
	sumNonWhale := big.NewInt(0)
	for i, coins := range deposits {
		h := crypto.Keccak256Hash(seed, common.Uint64ToBytes(uint64(i)))
		addr := common.BytesToAddress(h.Bytes()[12:32])
		dep := params.EtherToWei(big.NewInt(coins))
		universe[addr] = dep
		details[addr] = &ValidatorDetailsV2{Validator: addr, NilBlockCount: big.NewInt(0), LastNiLBlock: big.NewInt(0)}
		addrs = append(addrs, addr)
		sumNonWhale = common.SafeAddBigInt(sumNonWhale, dep)
	}

	// Whale holds 20% of the universe total: whale = sumNonWhale / 4 (since (S/4)/(S + S/4) = 20%).
	whaleHash := crypto.Keccak256Hash(seed, []byte("whale"))
	whaleAddr := common.BytesToAddress(whaleHash.Bytes()[12:32])
	whaleDep := common.SafeDivBigInt(sumNonWhale, big.NewInt(4))
	universe[whaleAddr] = whaleDep
	details[whaleAddr] = &ValidatorDetailsV2{Validator: whaleAddr, NilBlockCount: big.NewInt(0), LastNiLBlock: big.NewInt(0)}
	addrs = append(addrs, whaleAddr)

	// committee = top MAX_VALIDATORS by deposit (whale + 127 largest), tie-broken by address.
	sortByDepositDesc := func(list []common.Address, depMap map[common.Address]*big.Int) {
		sort.Slice(list, func(i, j int) bool {
			c := depMap[list[i]].Cmp(depMap[list[j]])
			if c == 0 {
				return bytes.Compare(list[i].Bytes(), list[j].Bytes()) < 0
			}
			return c > 0
		})
	}
	sortByDepositDesc(addrs, universe)
	committee := make(map[common.Address]bool, MAX_VALIDATORS)
	for i := 0; i < MAX_VALIDATORS && i < len(addrs); i++ {
		committee[addrs[i]] = true
	}
	if !committee[whaleAddr] {
		t.Fatalf("whale expected to be in committee")
	}

	preFilter := make(map[common.Address]*big.Int, len(committee))
	for a := range committee {
		preFilter[a] = new(big.Int).Set(universe[a])
	}

	// current behavior: normalize over the whole 304-validator universe.
	currentMap := make(map[common.Address]*big.Int, len(universe))
	for a, d := range universe {
		currentMap[a] = new(big.Int).Set(d)
	}
	currentSumBefore := big.NewInt(0)
	for _, d := range currentMap {
		currentSumBefore = common.SafeAddBigInt(currentSumBefore, d)
	}
	normalizeDeposit(blockNumber, &currentMap, &details)

	// new behavior: normalize over the committee only.
	newMap := make(map[common.Address]*big.Int, len(committee))
	for a := range committee {
		newMap[a] = new(big.Int).Set(universe[a])
	}
	newSumBefore := big.NewInt(0)
	for _, d := range newMap {
		newSumBefore = common.SafeAddBigInt(newSumBefore, d)
	}
	normalizeDeposit(blockNumber, &newMap, &details)

	// committee totals (denominators for the deposit-share percentages).
	rawCommitteeTotal := big.NewInt(0)
	curCommitteeTotal := big.NewInt(0)
	newCommitteeTotal := big.NewInt(0)
	for a := range committee {
		rawCommitteeTotal = common.SafeAddBigInt(rawCommitteeTotal, preFilter[a])
		curCommitteeTotal = common.SafeAddBigInt(curCommitteeTotal, currentMap[a])
		newCommitteeTotal = common.SafeAddBigInt(newCommitteeTotal, newMap[a])
	}

	pct := func(v, total *big.Int) float64 {
		if total.Sign() == 0 {
			return 0
		}
		f := new(big.Float).Quo(new(big.Float).SetInt(v), new(big.Float).SetInt(total))
		r, _ := f.Float64()
		return r * 100
	}

	committeeAddrs := make([]common.Address, 0, len(committee))
	for a := range committee {
		committeeAddrs = append(committeeAddrs, a)
	}
	sortByDepositDesc(committeeAddrs, preFilter)

	fmt.Printf("Normalizationv2 deposit-share: raw (no-normalize) vs current (normalize-all) vs new (normalize-committee)\n")
	fmt.Printf("block=%d committee=%d universe=%d whale holds 20%% of universe deposit\n", blockNumber, len(committee), len(universe))
	fmt.Printf("%-44s %12s %12s %12s\n", "Validator", "Raw %", "Before %", "After %")
	for _, a := range committeeAddrs {
		label := a.Hex()
		if a == whaleAddr {
			label = a.Hex() + " (whale)"
		}
		fmt.Printf("%-44s %11.4f%% %11.4f%% %11.4f%%\n", label, pct(preFilter[a], rawCommitteeTotal), pct(currentMap[a], curCommitteeTotal), pct(newMap[a], newCommitteeTotal))
	}
	fmt.Printf("%-44s %11.4f%% %11.4f%% %11.4f%%\n", "TOTAL", pct(rawCommitteeTotal, rawCommitteeTotal), pct(curCommitteeTotal, curCommitteeTotal), pct(newCommitteeTotal, newCommitteeTotal))

	// Invariants (printonly demonstration; no equality to the live page numbers).
	// 1) Total deposit conserved in each mode (within integer-division rounding of the redistribution).
	curSumAfter := big.NewInt(0)
	for _, d := range currentMap {
		curSumAfter = common.SafeAddBigInt(curSumAfter, d)
	}
	newSumAfter := big.NewInt(0)
	for _, d := range newMap {
		newSumAfter = common.SafeAddBigInt(newSumAfter, d)
	}
	roundingTolerance := big.NewInt(int64(len(universe)) + 10)
	if diff := common.SafeSubBigInt(currentSumBefore, curSumAfter); diff.Sign() < 0 || diff.Cmp(roundingTolerance) > 0 {
		t.Fatalf("current mode total not conserved: before=%s after=%s diff=%s", currentSumBefore, curSumAfter, diff)
	}
	if diff := common.SafeSubBigInt(newSumBefore, newSumAfter); diff.Sign() < 0 || diff.Cmp(roundingTolerance) > 0 {
		t.Fatalf("new mode total not conserved: before=%s after=%s diff=%s", newSumBefore, newSumAfter, diff)
	}

	// 2) The whale is capped below its pre-filter deposit in both modes.
	if currentMap[whaleAddr].Cmp(preFilter[whaleAddr]) >= 0 {
		t.Fatalf("whale not capped in current mode: pre=%s post=%s", preFilter[whaleAddr], currentMap[whaleAddr])
	}
	if newMap[whaleAddr].Cmp(preFilter[whaleAddr]) >= 0 {
		t.Fatalf("whale not capped in new mode: pre=%s post=%s", preFilter[whaleAddr], newMap[whaleAddr])
	}

	// 3) The whale's normalized deposit differs between the two modes (the scope change has an effect).
	if currentMap[whaleAddr].Cmp(newMap[whaleAddr]) == 0 {
		t.Fatalf("expected whale current vs new normalized deposit to differ; both = %s", currentMap[whaleAddr])
	}
}
