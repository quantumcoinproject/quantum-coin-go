package proofofstake

import (
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"math/big"
	"strconv"
	"testing"
)

func canValidateTest(lastNilBlock int64, nilBlockCount int64, currentBlock uint64, expected bool) bool {
	valDetails := &ValidatorDetailsV2{
		LastNiLBlock:  big.NewInt(lastNilBlock),
		NilBlockCount: big.NewInt(nilBlockCount),
	}

	result, _ := canValidate(valDetails, currentBlock)
	if result != expected {
		return false
	}

	return true
}

func TestPacketHandler_canValidate(t *testing.T) {
	if canValidateTest(0, 0, 100, true) == false {
		t.Fatalf("failed1")
	}
	if canValidateTest(0, 10, 100, true) == false {
		t.Fatalf("failed2")
	}
	if canValidateTest(int64(OfflineValidatorDeferStartBlock+1000), 127, uint64(OfflineValidatorDeferStartBlock+100), true) == false {
		t.Fatalf("failed3")
	}
	if canValidateTest(int64(OfflineValidatorDeferStartBlock+1000), 128, uint64(OfflineValidatorDeferStartBlock+100), false) == false {
		t.Fatalf("failed4")
	}
}

func canProposeTest(lastNilBlock int64, nilBlockCount int64, currentBlock uint64, expected bool) bool {
	valDetails := &ValidatorDetailsV2{
		LastNiLBlock:  big.NewInt(lastNilBlock),
		NilBlockCount: big.NewInt(nilBlockCount),
	}

	result, _ := canPropose(valDetails, currentBlock)
	if result != expected {
		return false
	}

	return true
}

func TestPacketHandler_canPropose(t *testing.T) {
	if canProposeTest(1744781, 128000, 1744781, false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(0, 0, 100, true) == false {
		t.Fatalf("failed")
	}
	if canProposeTest(0, 10, 100, true) == false {
		t.Fatalf("failed")
	}
	if canProposeTest(1, 1, 2, true) == false {
		t.Fatalf("failed")
	}
	if canProposeTest(1, 1, 3, true) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(50, 1, 51, true) == false {
		t.Fatalf("failed")
	}

	for i := uint64(1); i < 16; i++ {
		if canProposeTest(50, int64(i*BLOCK_PROPOSER_OFFLINE_NIL_BLOCK_MULTIPLIER), 51, false) == false {
			t.Fatalf("failed")
		}

		if canProposeTest(int64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+50), int64(i*BLOCK_PROPOSER_OFFLINE_NIL_BLOCK_MULTIPLIER), BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK, false) == false {
			t.Fatalf("failed")
		}

		if canProposeTest(int64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+50), int64(i*BLOCK_PROPOSER_OFFLINE_NIL_BLOCK_MULTIPLIER),
			uint64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2+50), true) == false {
			t.Fatalf("failed")
		}
	}

	if canProposeTest(int64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK), 1024,
		uint64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2-1), false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+1), 1024,
		uint64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2+1), true) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+1), 28,
		uint64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2), false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+1), 27,
		uint64(BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2), true) == false {
		t.Fatalf("failed")
	}
}

func testGetBlockProposerV2(validatorMap *map[common.Address]*ValidatorDetailsV2, expected common.Address, blockNumber uint64) bool {
	parentHash := common.BytesToHash([]byte(strconv.FormatInt(int64(blockNumber), 10)))
	proposer, err := getBlockProposerV2(parentHash, validatorMap, 1, blockNumber)
	if err != nil {
		fmt.Println("err", err)
		return false
	}

	fmt.Println("proposer", proposer, "expected", expected)

	return proposer.IsEqualTo(expected)
}

func TestPacketHandler_getBlockProposerV2(t *testing.T) {
	validatorMap := make(map[common.Address]*ValidatorDetailsV2)

	for i := 0; i < 100; i++ {
		v := &ValidatorDetailsV2{
			Validator:     common.BytesToAddress([]byte(string(rune(i)))),
			LastNiLBlock:  new(big.Int),
			NilBlockCount: new(big.Int),
		}
		validatorMap[v.Validator] = v
	}

	for i := 101; i < 128; i++ {
		v := &ValidatorDetailsV2{
			Validator:     common.BytesToAddress([]byte(string(rune(i)))),
			LastNiLBlock:  big.NewInt(50),
			NilBlockCount: big.NewInt(10),
		}
		validatorMap[v.Validator] = v
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000059"), 81) == false {
		t.Fatalf("failed")
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000056"), 85) == false {
		t.Fatalf("failed")
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x000000000000000000000000000000000000000000000000000000000000005a"), 50) == false {
		t.Fatalf("failed")
	}

	validatorMap = make(map[common.Address]*ValidatorDetailsV2)
	for i := 0; i < MIN_VALIDATORS; i++ {
		if i == 0 {
			v := &ValidatorDetailsV2{
				Validator:     common.BytesToAddress([]byte(string(rune(i)))),
				LastNiLBlock:  big.NewInt(20),
				NilBlockCount: big.NewInt(100),
			}
			validatorMap[v.Validator] = v
		} else {
			v := &ValidatorDetailsV2{
				Validator:     common.BytesToAddress([]byte(string(rune(i)))),
				LastNiLBlock:  new(big.Int),
				NilBlockCount: new(big.Int),
			}
			validatorMap[v.Validator] = v
		}
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001"), 50) == false {
		t.Fatalf("failed")
	}

	validatorMap = make(map[common.Address]*ValidatorDetailsV2)
	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001"), 50) == true {
		t.Fatalf("failed")
	}
}

func TestPacketHandler_getBlockProposerV3(t *testing.T) {
	validatorMap := make(map[common.Address]*ValidatorDetailsV2)

	for i := 0; i < 100; i++ {
		v := &ValidatorDetailsV2{
			Validator:     common.BytesToAddress([]byte(string(rune(i)))),
			LastNiLBlock:  new(big.Int),
			NilBlockCount: new(big.Int),
		}
		validatorMap[v.Validator] = v
	}

	for i := 101; i < 128; i++ {
		v := &ValidatorDetailsV2{
			Validator:     common.BytesToAddress([]byte(string(rune(i)))),
			LastNiLBlock:  big.NewInt(50),
			NilBlockCount: big.NewInt(10),
		}
		validatorMap[v.Validator] = v
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000052"), 500000) == false {
		t.Fatalf("failed")
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x000000000000000000000000000000000000000000000000000000000000003F"), 500001) == false {
		t.Fatalf("failed")
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000035"), 500002) == false {
		t.Fatalf("failed")
	}

	validatorMap = make(map[common.Address]*ValidatorDetailsV2)
	for i := 0; i < MIN_VALIDATORS; i++ {
		if i == 0 {
			v := &ValidatorDetailsV2{
				Validator:     common.BytesToAddress([]byte(string(rune(i)))),
				LastNiLBlock:  big.NewInt(20),
				NilBlockCount: big.NewInt(100),
			}
			validatorMap[v.Validator] = v
		} else {
			v := &ValidatorDetailsV2{
				Validator:     common.BytesToAddress([]byte(string(rune(i)))),
				LastNiLBlock:  new(big.Int),
				NilBlockCount: new(big.Int),
			}
			validatorMap[v.Validator] = v
		}
	}

	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000"), 500003) == false {
		t.Fatalf("failed")
	}

	validatorMap = make(map[common.Address]*ValidatorDetailsV2)
	if testGetBlockProposerV2(&validatorMap, common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001"), 500004) == true {
		t.Fatalf("failed")
	}
}

func TestPacketHandler_canPropose_v3_positive(t *testing.T) {
	lastNilBlock := int64(OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2083437)
	if canProposeTest(lastNilBlock, 17, currentBlock, true) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}

func TestPacketHandler_canPropose_v3_positive_max_block_delay_equal(t *testing.T) {
	lastNilBlock := int64(OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2148717)
	if canProposeTest(lastNilBlock, 32, currentBlock, true) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}

func TestPacketHandler_canPropose_v3_positive_max_block_delay_greater(t *testing.T) {
	lastNilBlock := int64(OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2148717)
	if canProposeTest(lastNilBlock, 33, currentBlock, true) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}

func TestPacketHandler_canPropose_v3_negative_max_block_delay_greater(t *testing.T) {
	lastNilBlock := int64(OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2148717 - 1)
	if canProposeTest(lastNilBlock, 33, currentBlock, false) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}
