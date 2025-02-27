package proofofstake

import (
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/params"
	"math/big"
	"testing"
	"time"
)

var TestFilterValidatorsBlockNumber = SixtyVoteStartBlock

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
	TestFilterValidatorsBlockNumber = OfflineValidatorDeferStartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})
	val4 := common.BytesToAddress([]byte{4})

	validatorsDDetailsMap[val1] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
		LastNiLBlock:  big.NewInt(int64(OfflineValidatorDeferStartBlock) + int64(10)),
	}

	validatorsDDetailsMap[val2] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD) - 1),
		LastNiLBlock:  big.NewInt(int64(OfflineValidatorDeferStartBlock) - 10),
	}

	validatorsDDetailsMap[val3] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(1),
		LastNiLBlock:  big.NewInt(int64(OfflineValidatorDeferStartBlock) - 100),
	}

	validatorsDDetailsMap[val4] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(100000000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(200000000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(400000000000))
	validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(500000000000))

	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, OfflineValidatorDeferStartBlock, &validatorsDDetailsMap)
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
	TestFilterValidatorsBlockNumber = SixtyVoteStartBlock
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
	TestFilterValidatorsBlockNumber = SixtySevenVoteStartBlock
	consensusContext := common.BytesToHash([]byte{100})
	validatorsDepositMap := make(map[common.Address]*big.Int)

	validatorsDDetailsMap := make(map[common.Address]*ValidatorDetailsV2)

	val1 := common.BytesToAddress([]byte{1})
	val2 := common.BytesToAddress([]byte{2})
	val3 := common.BytesToAddress([]byte{3})
	val4 := common.BytesToAddress([]byte{4})

	validatorsDDetailsMap[val1] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD)),
		LastNiLBlock:  big.NewInt(int64(SixtySevenVoteStartBlock) + int64(10)),
	}

	validatorsDDetailsMap[val2] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(int64(OFFLINE_VALIDATOR_DEFER_THRESHOLD) - 1),
		LastNiLBlock:  big.NewInt(int64(SixtySevenVoteStartBlock) - 10),
	}

	validatorsDDetailsMap[val3] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(1),
		LastNiLBlock:  big.NewInt(int64(SixtySevenVoteStartBlock) - 100),
	}

	validatorsDDetailsMap[val4] = &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(0),
		LastNiLBlock:  big.NewInt(0),
	}

	validatorsDepositMap[val1] = params.EtherToWei(big.NewInt(100000000000))
	validatorsDepositMap[val2] = params.EtherToWei(big.NewInt(200000000000))
	validatorsDepositMap[val3] = params.EtherToWei(big.NewInt(400000000000))
	validatorsDepositMap[val4] = params.EtherToWei(big.NewInt(500000000000))

	resultMap, filteredDepositValue, _, err := filterValidators(consensusContext, &validatorsDepositMap, SixtySevenVoteStartBlock, &validatorsDDetailsMap)
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
	TestFilterValidatorsBlockNumber = SixtyVoteStartBlock
}
