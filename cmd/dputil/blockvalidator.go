package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
)

// BlockCmd fetches eth_getBlockByNumber (full tx objects) and proofofstake_getBlockConsensusData,
// writes them to BLOCK_NUMBER.json in the current directory, and prints the block JSON plus the file path.
func BlockCmd() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage: dputil block BLOCK_NUMBER")
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
	blockHex := hexutil.EncodeBig(new(big.Int).SetUint64(blockNum))

	var blockRaw json.RawMessage
	if err := client.GetRpcClient().CallContext(ctx, &blockRaw, "eth_getBlockByNumber", blockHex, true); err != nil {
		return fmt.Errorf("eth_getBlockByNumber: %w", err)
	}
	if len(blockRaw) == 0 || string(blockRaw) == "null" {
		return fmt.Errorf("eth_getBlockByNumber: block %d not found", blockNum)
	}

	var consensusRaw json.RawMessage
	if err := client.GetRpcClient().CallContext(ctx, &consensusRaw, "proofofstake_getBlockConsensusData", blockHex); err != nil {
		return fmt.Errorf("proofofstake_getBlockConsensusData: %w", err)
	}

	out := map[string]json.RawMessage{
		"Block":              blockRaw,
		"BlockConsensusData": consensusRaw,
	}

	// GetBlockValidatorDetailsByBlock for both contexts; do not bail if one fails.
	details1, err1 := client.GetBlockValidatorDetailsByBlock(ctx, blockNum, backupmanager.BlockValidatorContextValidator)
	if err1 != nil {
		out["blockValidatorDetails_1"] = json.RawMessage("null")
	} else {
		b1, err := json.Marshal(details1)
		if err != nil {
			out["blockValidatorDetails_1"] = json.RawMessage("null")
		} else {
			out["blockValidatorDetails_1"] = json.RawMessage(b1)
		}
	}
	details2, err2 := client.GetBlockValidatorDetailsByBlock(ctx, blockNum, backupmanager.BlockValidatorContextBlockVerify)
	if err2 != nil {
		out["blockValidatorDetails_2"] = json.RawMessage("null")
	} else {
		b2, err := json.Marshal(details2)
		if err != nil {
			out["blockValidatorDetails_2"] = json.RawMessage("null")
		} else {
			out["blockValidatorDetails_2"] = json.RawMessage(b2)
		}
	}

	filename := blockNumStr + ".json"
	merged, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, merged, 0644); err != nil {
		return err
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	var blockPretty bytes.Buffer
	if err := json.Indent(&blockPretty, blockRaw, "", "  "); err != nil {
		return err
	}
	fmt.Println(blockPretty.String())
	fmt.Println("Additional block details written to file: ", absPath)

	return nil
}

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
		fmt.Println("GetBlockValidatorDetails (Validator context) failed:", err, "context", backupmanager.BlockValidatorContextValidator)
	} else {
		printBlockValidatorDetailsDetail("Validator", detailsValidator)
		if err := writeBlockValidatorDetailsFile(blockNumStr, backupmanager.BlockValidatorContextValidator, detailsValidator); err != nil {
			fmt.Println("Write JSON (Validator context) failed:", err)
		}
	}

	fmt.Println()
	fmt.Println("========== GetBlockValidatorDetails (context: BlockValidatorContextBlockVerify / \"2\") ==========")
	detailsBlockVerify, err := client.GetBlockValidatorDetailsByBlock(ctx, blockNum, backupmanager.BlockValidatorContextBlockVerify)
	if err != nil {
		fmt.Println("GetBlockValidatorDetails (BlockVerify context) failed:", err, "context", backupmanager.BlockValidatorContextBlockVerify)
		return err
	} else {
		printBlockValidatorDetailsDetail("BlockVerify", detailsBlockVerify)
		if err := writeBlockValidatorDetailsFile(blockNumStr, backupmanager.BlockValidatorContextBlockVerify, detailsBlockVerify); err != nil {
			fmt.Println("Write JSON (BlockVerify context) failed:", err)
		}
	}

	fmt.Println()
	fmt.Println("========== Comparison: differences between Validator and BlockVerify ==========")
	compareBlockValidatorDetails(detailsValidator, detailsBlockVerify)

	return nil
}

func writeBlockValidatorDetailsFile(blockNumStr, context string, d *backupmanager.BlockValidatorDetails) error {
	if d == nil {
		return nil
	}
	// Copy and sort by validator address so saved JSON is deterministic (same order as compareBlockValidatorDetails).
	copyDetails := *d
	copyDetails.FilteredValidatorDepositList = make([]backupmanager.ValidatorDeposit, len(d.FilteredValidatorDepositList))
	copy(copyDetails.FilteredValidatorDepositList, d.FilteredValidatorDepositList)
	sort.Slice(copyDetails.FilteredValidatorDepositList, func(i, j int) bool {
		return bytes.Compare(copyDetails.FilteredValidatorDepositList[i].ValidatorAddress[:], copyDetails.FilteredValidatorDepositList[j].ValidatorAddress[:]) < 0
	})
	copyDetails.ValidatorDetailsList = make([]backupmanager.ValidatorDetailsV2, len(d.ValidatorDetailsList))
	copy(copyDetails.ValidatorDetailsList, d.ValidatorDetailsList)
	sort.Slice(copyDetails.ValidatorDetailsList, func(i, j int) bool {
		return bytes.Compare(copyDetails.ValidatorDetailsList[i].Validator[:], copyDetails.ValidatorDetailsList[j].Validator[:]) < 0
	})
	filename := fmt.Sprintf("block-validator-%s-%s.json", blockNumStr, context)
	b, err := json.MarshalIndent(&copyDetails, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, b, 0644)
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
		return bytes.Compare(sortedFilteredA[i].ValidatorAddress[:], sortedFilteredA[j].ValidatorAddress[:]) < 0
	})
	sortedFilteredB := make([]backupmanager.ValidatorDeposit, len(b.FilteredValidatorDepositList))
	copy(sortedFilteredB, b.FilteredValidatorDepositList)
	sort.Slice(sortedFilteredB, func(i, j int) bool {
		return bytes.Compare(sortedFilteredB[i].ValidatorAddress[:], sortedFilteredB[j].ValidatorAddress[:]) < 0
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
		return bytes.Compare(sortedDetailsA[i].Validator[:], sortedDetailsA[j].Validator[:]) < 0
	})
	sortedDetailsB := make([]backupmanager.ValidatorDetailsV2, len(b.ValidatorDetailsList))
	copy(sortedDetailsB, b.ValidatorDetailsList)
	sort.Slice(sortedDetailsB, func(i, j int) bool {
		return bytes.Compare(sortedDetailsB[i].Validator[:], sortedDetailsB[j].Validator[:]) < 0
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

	if !preparedConsensusStateEqual(a.PreparedConsensusState, b.PreparedConsensusState) {
		fmt.Printf("  PreparedConsensusState: DIFFERENT\n")
		diff = true
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

func preparedConsensusStateEqual(a, b *backupmanager.PreparedConsensusState) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if cmpBigInt(a.TotalBlockDepositValue, b.TotalBlockDepositValue) {
		return false
	}
	if cmpBigInt(a.MinDepositRequired, b.MinDepositRequired) {
		return false
	}
	if !addrBigIntMapEqual(a.FilteredValidatorsDepositMap, b.FilteredValidatorsDepositMap) {
		return false
	}
	if !addrValidatorDetailsMapEqual(a.ValidatorDetailsMap, b.ValidatorDetailsMap) {
		return false
	}
	if !byteAddrMapEqual(a.RoundProposers, b.RoundProposers) {
		return false
	}
	if !byteHashMapEqual(a.NilVoteProposalHashes, b.NilVoteProposalHashes) {
		return false
	}
	if !byteHashMapEqual(a.NilVotePrecommitHashes, b.NilVotePrecommitHashes) {
		return false
	}
	return true
}

func addrBigIntMapEqual(am, bm map[common.Address]*big.Int) bool {
	if len(am) != len(bm) {
		return false
	}
	for k, av := range am {
		bv, ok := bm[k]
		if !ok || cmpBigInt(av, bv) {
			return false
		}
	}
	return true
}

func addrValidatorDetailsMapEqual(am, bm map[common.Address]backupmanager.ValidatorDetailsV2) bool {
	if len(am) != len(bm) {
		return false
	}
	for k, av := range am {
		bv, ok := bm[k]
		if !ok {
			return false
		}
		if av.Depositor != bv.Depositor || av.Validator != bv.Validator {
			return false
		}
		if cmpBigInt(av.Balance, bv.Balance) || cmpBigInt(av.NetBalance, bv.NetBalance) {
			return false
		}
		if cmpBigInt(av.BlockRewards, bv.BlockRewards) || cmpBigInt(av.Slashings, bv.Slashings) {
			return false
		}
		if av.IsValidationPaused != bv.IsValidationPaused {
			return false
		}
		if cmpBigInt(av.WithdrawalBlock, bv.WithdrawalBlock) || cmpBigInt(av.WithdrawalAmount, bv.WithdrawalAmount) {
			return false
		}
		if cmpBigInt(av.LastNiLBlock, bv.LastNiLBlock) || cmpBigInt(av.NilBlockCount, bv.NilBlockCount) {
			return false
		}
	}
	return true
}

func byteAddrMapEqual(am, bm map[byte]common.Address) bool {
	if len(am) != len(bm) {
		return false
	}
	for k, av := range am {
		bv, ok := bm[k]
		if !ok || av != bv {
			return false
		}
	}
	return true
}

func byteHashMapEqual(am, bm map[byte]common.Hash) bool {
	if len(am) != len(bm) {
		return false
	}
	for k, av := range am {
		bv, ok := bm[k]
		if !ok || av != bv {
			return false
		}
	}
	return true
}
