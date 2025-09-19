package proofofstake

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"math/big"
	"strconv"
	"testing"
)

func canValidateTest(lastNilBlock int64, nilBlockCount int64, currentBlock uint64, expected bool) bool {
	valDetails := &ValidatorDetailsV2{
		LastNiLBlock:  big.NewInt(lastNilBlock),
		NilBlockCount: big.NewInt(nilBlockCount),
	}

	result, nextValidationBlock := canValidate(valDetails, currentBlock)
	fmt.Println("lastNilBlock", lastNilBlock, "nilBlockCount", nilBlockCount, "currentBlock", currentBlock, "nextValidationBlock", nextValidationBlock, "blocks remaining", nextValidationBlock-currentBlock)
	if result != expected {
		return false
	}
	return true
}

func TestPacketHandler_canValidate_single(t *testing.T) {
	if canValidateTest(int64(2543878), 37, uint64(2548263), true) == false {
		t.Fatalf("failed4")
	}
}

func TestPacketHandler_canValidate(t *testing.T) {
	if canValidateTest(0, 0, 100, true) == false {
		t.Fatalf("failed1")
	}
	if canValidateTest(0, 10, 100, true) == false {
		t.Fatalf("failed2")
	}
	if canValidateTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock+1), 32, uint64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock+32+1), true) == false {
		t.Fatalf("failed3")
	}
	if canValidateTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock+1), 127, uint64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock+127+1), true) == false {
		t.Fatalf("failed3")
	}
	if canValidateTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock+1), 128, uint64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock+128+1), false) == false {
		t.Fatalf("failed4")
	}
}

func testOfflineValidatorDepositAfterPenalty(nilBlockCount int64, currentBlock uint64, depositValue *big.Int, expectedDepositValue *big.Int) bool {
	valDetails := &ValidatorDetailsV2{
		NilBlockCount: big.NewInt(nilBlockCount),
	}
	result := getOfflineValidatorDepositAfterPenalty(valDetails, currentBlock, depositValue)
	fmt.Println("depositValue", depositValue, "result", result, "expectedDepositValue", expectedDepositValue)
	if result.Cmp(expectedDepositValue) == 0 {
		return true
	}
	return false
}

func TestConsensusHandler_offlineValidatorDepositAfterPenalty(t *testing.T) {
	if (testOfflineValidatorDepositAfterPenalty(0, 1, big.NewInt(100000000000), big.NewInt(100000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(10000, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock-1, big.NewInt(100000000000), big.NewInt(100000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(0, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(100000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(1, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(100000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(2, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(100000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(3, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(94000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(12, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(76000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(32, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(36000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(49, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(2000000000))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(49, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000001), big.NewInt(2000000001))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(49, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000002), big.NewInt(2000000001))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(50, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(0))) == false {
		t.Fatalf("failed")
	}
	if (testOfflineValidatorDepositAfterPenalty(10000, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock, big.NewInt(100000000000), big.NewInt(0))) == false {
		t.Fatalf("failed")
	}
}

func TestProposeValidate(t *testing.T) {
	lastNiLBlock := int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock)
	lastNiLBlockStart := lastNiLBlock
	totalSteps := 128
	step := 1
	nilBlockCount := int64(0)
	currentBlock := uint64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock) + 1
	validatorCount := 128
	for {
		valDetails := &ValidatorDetailsV2{
			LastNiLBlock:  big.NewInt(lastNiLBlock),
			NilBlockCount: big.NewInt(nilBlockCount),
		}

		canP, nextProposalBlock := canPropose(valDetails, currentBlock)
		canV, nextValidationBlock := canValidate(valDetails, currentBlock)
		blocksSinceStart := currentBlock - uint64(lastNiLBlockStart)
		blocksPerDayExample := uint64(4000)
		days := blocksSinceStart / blocksPerDayExample
		newDepositValue := getOfflineValidatorDepositAfterPenalty(valDetails, currentBlock, big.NewInt(100000000000))

		fmt.Println("currentBlock", currentBlock, "NilBlockCount", valDetails.NilBlockCount, "canPropose", canP, "canValidate", canV,
			"lastNiLBlock", lastNiLBlock, "nextProposalBlock", nextProposalBlock, "nextValidationBlock", nextValidationBlock,
			"diff", nextProposalBlock-valDetails.LastNiLBlock.Uint64(), "blocksSinceStart", blocksSinceStart, "days", days, "newDepositValue", newDepositValue)
		lastNiLBlock = int64(currentBlock)
		nilBlockCount = nilBlockCount + 1
		if step >= totalSteps {
			break
		}
		step = step + 1
		currentBlock = nextProposalBlock + uint64(validatorCount)
	}
}

func canProposeTest(lastNilBlock int64, nilBlockCount int64, currentBlock uint64, expected bool) bool {
	valDetails := &ValidatorDetailsV2{
		LastNiLBlock:  big.NewInt(lastNilBlock),
		NilBlockCount: big.NewInt(nilBlockCount),
	}

	result, nextProposalBlock := canPropose(valDetails, currentBlock)
	fmt.Println("currentBlock", currentBlock, "NilBlockCount", valDetails.NilBlockCount, "canPropose", result,
		"lastNiLBlock", valDetails.LastNiLBlock, "nextProposalBlock", nextProposalBlock)
	if result != expected {
		return false
	}

	return true
}

func TestCanPropose_v4(t *testing.T) {
	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock), 0, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock+1, true) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock), 1, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock+1, true) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock), 2, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock+1, false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock), 2, defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock+defaults.DefaultConfig.PosConfig.MinOfflineProposerBlockDelay+1, false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock), 2, 3003632+600000, true) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock), 32, 3069166+600000, true) == false {
		t.Fatalf("failed")
	}
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

		if canProposeTest(int64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+50), int64(i*BLOCK_PROPOSER_OFFLINE_NIL_BLOCK_MULTIPLIER), defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK, false) == false {
			t.Fatalf("failed")
		}

		if canProposeTest(int64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+50), int64(i*BLOCK_PROPOSER_OFFLINE_NIL_BLOCK_MULTIPLIER),
			uint64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2+50), true) == false {
			t.Fatalf("failed")
		}
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK), 1024,
		uint64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2-1), false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+1), 1024,
		uint64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2+1), true) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+1), 28,
		uint64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2), false) == false {
		t.Fatalf("failed")
	}

	if canProposeTest(int64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+1), 27,
		uint64(defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_OFFLINE_V2_START_BLOCK+BLOCK_PROPOSER_OFFLINE_MAX_DELAY_BLOCK_COUNT_V2), true) == false {
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
	lastNilBlock := int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2083437)
	if canProposeTest(lastNilBlock, 17, currentBlock, true) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}

func TestPacketHandler_canPropose_v3_positive_max_block_delay_equal(t *testing.T) {
	lastNilBlock := int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2148717)
	if canProposeTest(lastNilBlock, 32, currentBlock, true) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}

func TestPacketHandler_canPropose_v3_positive_max_block_delay_greater(t *testing.T) {
	lastNilBlock := int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2148717)
	if canProposeTest(lastNilBlock, 33, currentBlock, true) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}

func TestPacketHandler_canPropose_v3_negative_max_block_delay_greater(t *testing.T) {
	lastNilBlock := int64(defaults.DefaultConfig.PosConfig.OfflineValidatorDeferStartBlock + 1000)
	currentBlock := uint64(2148717 - 1)
	if canProposeTest(lastNilBlock, 33, currentBlock, false) == false {
		t.Fatalf("failed")
	}
	fmt.Println("canPropose_v3_positive", "diff", currentBlock-uint64(lastNilBlock))
}
