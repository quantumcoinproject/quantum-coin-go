package proofofstake

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/defaults"
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
