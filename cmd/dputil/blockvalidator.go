package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
)

func GetBlockValidatorDetailsCmd() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage: dputil getblockvalidatordetails BLOCK_NUMBER")
	}

	blockNumStr := os.Args[2]
	blockNum, err := strconv.ParseUint(blockNumStr, 10, 64)
	if err != nil {
		fmt.Println("Enter block number correctly: ", blockNumStr)
		return err
	}

	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
	}

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		fmt.Println("Dial failed, ensure DP_RAW_URL is set correctly", err)
		return err
	}
	defer client.Close()

	ctx := context.Background()

	// Call with BlockValidatorContextValidator ("1")
	fmt.Println("========== GetBlockValidatorDetails (context: BlockValidatorContextValidator / \"1\") ==========")
	detailsValidator, err := client.GetBlockValidatorDetailsByBlock(ctx, blockNum, backupmanager.BlockValidatorContextValidator)
	if err != nil {
		fmt.Println("GetBlockValidatorDetails (Validator context) failed:", err)
		return err
	}
	printBlockValidatorDetailsDetail("Validator", detailsValidator)

	fmt.Println()
	fmt.Println("========== GetBlockValidatorDetails (context: BlockValidatorContextBlockVerify / \"2\") ==========")
	detailsBlockVerify, err := client.GetBlockValidatorDetailsByBlock(ctx, blockNum, backupmanager.BlockValidatorContextBlockVerify)
	if err != nil {
		fmt.Println("GetBlockValidatorDetails (BlockVerify context) failed:", err)
		return err
	}
	printBlockValidatorDetailsDetail("BlockVerify", detailsBlockVerify)

	fmt.Println()
	fmt.Println("========== Comparison: differences between Validator and BlockVerify ==========")
	compareBlockValidatorDetails(detailsValidator, detailsBlockVerify)

	return nil
}

func printBlockValidatorDetailsDetail(label string, d *backupmanager.BlockValidatorDetails) {
	if d == nil {
		fmt.Printf("[%s] BlockValidatorDetails is nil\n", label)
		return
	}
	fmt.Printf("[%s] BlockNumber: %s\n", label, bigIntStr(d.BlockNumber))
	fmt.Printf("[%s] ParentHash: %s\n", label, d.ParentHash.Hex())
	fmt.Printf("[%s] PreFilterValidatorCount: %s\n", label, bigIntStr(d.PreFilterValidatorCount))
	fmt.Printf("[%s] ConsensusContext: %s\n", label, d.ConsensusContext.Hex())
	fmt.Printf("[%s] FilteredValidatorDepositList (len=%d):\n", label, len(d.FilteredValidatorDepositList))
	for i, v := range d.FilteredValidatorDepositList {
		fmt.Printf("  [%s]   [%d] ValidatorAddress=%s PostFilterDeposit=%s\n", label, i, v.ValidatorAddress.Hex(), bigIntStr(v.PostFilterDeposit))
	}
	fmt.Printf("[%s] ValidatorDetailsList (len=%d):\n", label, len(d.ValidatorDetailsList))
	for i, v := range d.ValidatorDetailsList {
		fmt.Printf("  [%s]   [%d] Depositor=%s Validator=%s Balance=%s NetBalance=%s BlockRewards=%s Slashings=%s IsValidationPaused=%v WithdrawalBlock=%s WithdrawalAmount=%s LastNiLBlock=%s NilBlockCount=%s\n",
			label, i, v.Depositor.Hex(), v.Validator.Hex(), bigIntStr(v.Balance), bigIntStr(v.NetBalance), bigIntStr(v.BlockRewards), bigIntStr(v.Slashings),
			v.IsValidationPaused, bigIntStr(v.WithdrawalBlock), bigIntStr(v.WithdrawalAmount), bigIntStr(v.LastNiLBlock), bigIntStr(v.NilBlockCount))
	}
	// JSON summary for full detail
	jsonBytes, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		fmt.Printf("[%s] JSON marshal error: %v\n", label, err)
	} else {
		fmt.Printf("[%s] JSON:\n%s\n", label, string(jsonBytes))
	}
}

func bigIntStr(b *big.Int) string {
	if b == nil {
		return "<nil>"
	}
	return b.String()
}

func compareBlockValidatorDetails(a, b *backupmanager.BlockValidatorDetails) {
	if a == nil && b == nil {
		fmt.Println("Both are nil; no differences.")
		return
	}
	if a == nil {
		fmt.Println("Validator context returned nil; BlockVerify context returned non-nil. Full difference.")
		return
	}
	if b == nil {
		fmt.Println("BlockVerify context returned nil; Validator context returned non-nil. Full difference.")
		return
	}

	diff := false

	if cmpBigInt(a.BlockNumber, b.BlockNumber) {
		fmt.Printf("  BlockNumber: DIFFERENT  Validator=%s  BlockVerify=%s\n", bigIntStr(a.BlockNumber), bigIntStr(b.BlockNumber))
		diff = true
	}
	if a.ParentHash != b.ParentHash {
		fmt.Printf("  ParentHash: DIFFERENT  Validator=%s  BlockVerify=%s\n", a.ParentHash.Hex(), b.ParentHash.Hex())
		diff = true
	}
	if cmpBigInt(a.PreFilterValidatorCount, b.PreFilterValidatorCount) {
		fmt.Printf("  PreFilterValidatorCount: DIFFERENT  Validator=%s  BlockVerify=%s\n", bigIntStr(a.PreFilterValidatorCount), bigIntStr(b.PreFilterValidatorCount))
		diff = true
	}
	if a.ConsensusContext != b.ConsensusContext {
		fmt.Printf("  ConsensusContext: DIFFERENT  Validator=%s  BlockVerify=%s\n", a.ConsensusContext.Hex(), b.ConsensusContext.Hex())
		diff = true
	}

	// Sort both lists by validator address before comparing
	sortedFilteredA := make([]backupmanager.ValidatorDeposit, len(a.FilteredValidatorDepositList))
	copy(sortedFilteredA, a.FilteredValidatorDepositList)
	sort.Slice(sortedFilteredA, func(i, j int) bool {
		return strings.Compare(sortedFilteredA[i].ValidatorAddress.Hex(), sortedFilteredA[j].ValidatorAddress.Hex()) < 0
	})
	sortedFilteredB := make([]backupmanager.ValidatorDeposit, len(b.FilteredValidatorDepositList))
	copy(sortedFilteredB, b.FilteredValidatorDepositList)
	sort.Slice(sortedFilteredB, func(i, j int) bool {
		return strings.Compare(sortedFilteredB[i].ValidatorAddress.Hex(), sortedFilteredB[j].ValidatorAddress.Hex()) < 0
	})

	if len(sortedFilteredA) != len(sortedFilteredB) {
		fmt.Printf("  FilteredValidatorDepositList length: DIFFERENT  Validator=%d  BlockVerify=%d\n", len(sortedFilteredA), len(sortedFilteredB))
		diff = true
	} else {
		for i := range sortedFilteredA {
			av, bv := sortedFilteredA[i], sortedFilteredB[i]
			if av.ValidatorAddress != bv.ValidatorAddress {
				fmt.Printf("  FilteredValidatorDepositList[%d].ValidatorAddress: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, av.ValidatorAddress.Hex(), bv.ValidatorAddress.Hex())
				diff = true
			}
			if cmpBigInt(av.PostFilterDeposit, bv.PostFilterDeposit) {
				fmt.Printf("  FilteredValidatorDepositList[%d].PostFilterDeposit: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.PostFilterDeposit), bigIntStr(bv.PostFilterDeposit))
				diff = true
			}
		}
	}

	// Sort both lists by validator address before comparing
	sortedDetailsA := make([]backupmanager.ValidatorDetailsV2, len(a.ValidatorDetailsList))
	copy(sortedDetailsA, a.ValidatorDetailsList)
	sort.Slice(sortedDetailsA, func(i, j int) bool {
		return strings.Compare(sortedDetailsA[i].Validator.Hex(), sortedDetailsA[j].Validator.Hex()) < 0
	})
	sortedDetailsB := make([]backupmanager.ValidatorDetailsV2, len(b.ValidatorDetailsList))
	copy(sortedDetailsB, b.ValidatorDetailsList)
	sort.Slice(sortedDetailsB, func(i, j int) bool {
		return strings.Compare(sortedDetailsB[i].Validator.Hex(), sortedDetailsB[j].Validator.Hex()) < 0
	})

	if len(sortedDetailsA) != len(sortedDetailsB) {
		fmt.Printf("  ValidatorDetailsList length: DIFFERENT  Validator=%d  BlockVerify=%d\n", len(sortedDetailsA), len(sortedDetailsB))
		diff = true
	} else {
		for i := range sortedDetailsA {
			av, bv := sortedDetailsA[i], sortedDetailsB[i]
			if av.Depositor != bv.Depositor {
				fmt.Printf("  ValidatorDetailsList[%d].Depositor: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, av.Depositor.Hex(), bv.Depositor.Hex())
				diff = true
			}
			if av.Validator != bv.Validator {
				fmt.Printf("  ValidatorDetailsList[%d].Validator: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, av.Validator.Hex(), bv.Validator.Hex())
				diff = true
			}
			if cmpBigInt(av.Balance, bv.Balance) {
				fmt.Printf("  ValidatorDetailsList[%d].Balance: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.Balance), bigIntStr(bv.Balance))
				diff = true
			}
			if cmpBigInt(av.NetBalance, bv.NetBalance) {
				fmt.Printf("  ValidatorDetailsList[%d].NetBalance: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.NetBalance), bigIntStr(bv.NetBalance))
				diff = true
			}
			if cmpBigInt(av.BlockRewards, bv.BlockRewards) {
				fmt.Printf("  ValidatorDetailsList[%d].BlockRewards: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.BlockRewards), bigIntStr(bv.BlockRewards))
				diff = true
			}
			if cmpBigInt(av.Slashings, bv.Slashings) {
				fmt.Printf("  ValidatorDetailsList[%d].Slashings: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.Slashings), bigIntStr(bv.Slashings))
				diff = true
			}
			if av.IsValidationPaused != bv.IsValidationPaused {
				fmt.Printf("  ValidatorDetailsList[%d].IsValidationPaused: DIFFERENT  Validator=%v  BlockVerify=%v\n", i, av.IsValidationPaused, bv.IsValidationPaused)
				diff = true
			}
			if cmpBigInt(av.WithdrawalBlock, bv.WithdrawalBlock) {
				fmt.Printf("  ValidatorDetailsList[%d].WithdrawalBlock: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.WithdrawalBlock), bigIntStr(bv.WithdrawalBlock))
				diff = true
			}
			if cmpBigInt(av.WithdrawalAmount, bv.WithdrawalAmount) {
				fmt.Printf("  ValidatorDetailsList[%d].WithdrawalAmount: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.WithdrawalAmount), bigIntStr(bv.WithdrawalAmount))
				diff = true
			}
			if cmpBigInt(av.LastNiLBlock, bv.LastNiLBlock) {
				fmt.Printf("  ValidatorDetailsList[%d].LastNiLBlock: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.LastNiLBlock), bigIntStr(bv.LastNiLBlock))
				diff = true
			}
			if cmpBigInt(av.NilBlockCount, bv.NilBlockCount) {
				fmt.Printf("  ValidatorDetailsList[%d].NilBlockCount: DIFFERENT  Validator=%s  BlockVerify=%s\n", i, bigIntStr(av.NilBlockCount), bigIntStr(bv.NilBlockCount))
				diff = true
			}
		}
	}

	if !diff {
		fmt.Println("  No differences; both contexts returned identical data.")
	}
}

func cmpBigInt(x, y *big.Int) bool {
	if x == nil && y == nil {
		return false
	}
	if x == nil || y == nil {
		return true
	}
	return x.Cmp(y) != 0
}
