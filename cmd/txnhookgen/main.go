// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Command txnhookgen generates a TXN_HOOK_FILE JSON of pre-signed DynamicFeeTx
// transactions for TPS testing. Starting from a funded root wallet keystore, it
// hierarchically funds newly created wallets in a doubling binary tree: each
// batch's funded wallets become the senders of the next batch.
//
// Usage:
//
//	txnhookgen <input-wallet-path> <output-path> [wallet-password]
//
// If the wallet password is omitted, it is prompted for interactively. All
// other parameters (chain id, amounts, gas, batch pattern, start block) are
// hardcoded below.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/accounts/keystore"
	"github.com/quantumcoinproject/quantum-coin-go/console/prompt"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

const (
	// leafCoins is the amount (in coins) each leaf wallet ends up funded with.
	leafCoins = 1000

	// gasBudgetCoins is the per-transaction gas budget reserved when computing
	// funding amounts. It matches the base fee of a default DynamicFeeTx
	// (~100 coins for a 21000-gas transfer); the actual fee is slightly less,
	// leaving each sender a tiny surplus.
	gasBudgetCoins = 100
)

// chainID is the network chain id used for signing.
var chainID = big.NewInt(types.DEFAULT_CHAIN_ID)

// batchSpec is one entry of the hardcoded funding pattern: the BatchNumber that
// will be written into the hook file and the number of wallets funded in that
// batch (which equals the number of transactions in the batch).
type batchSpec struct {
	batchNumber int64
	funded      int
}

// batchPattern is the hardcoded doubling schedule. Each funded count must equal
// twice the previous level's size (root is level 0 with size 1).
var batchPattern = []batchSpec{
	{1, 2},
	{2, 4},
	{3, 8},
	{4, 16},
	{5, 32},
	{10, 64},
	{11, 128},
	{12, 256},
	{13, 512},
	{14, 1024},
	{15, 2048},
}

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "usage: %s <input-wallet-path> <output-path> [wallet-password]\n", os.Args[0])
		os.Exit(2)
	}
	inputWalletPath := os.Args[1]
	outputPath := os.Args[2]

	// Accept the wallet password from the command line; otherwise prompt for it
	// interactively (same approach as cmd/dputil).
	var password string
	if len(os.Args) == 4 {
		password = os.Args[3]
	} else {
		pwd, err := prompt.Stdin.PromptPassword("Enter the wallet password : ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to read password: %v\n", err)
			os.Exit(1)
		}
		password = pwd
	}

	if err := run(inputWalletPath, outputPath, password); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(inputWalletPath, outputPath, password string) error {
	// Select the network config (devnet via Q_DEFAULT_CONFIG=1) so the start
	// block reflects the target network.
	defaults.LoadDefaultConfig()
	startBlock := defaults.DefaultConfig.PosConfig.SystemContractV3StartBlock + 10

	logf("Loading root wallet from %s", inputWalletPath)
	rootKey, err := loadRootKey(inputWalletPath, password)
	if err != nil {
		return fmt.Errorf("failed to load root wallet: %w", err)
	}
	rootAddr := cryptobase.SigAlg.PublicKeyToAddressNoError(&rootKey.PublicKey)
	logf("Root wallet loaded: %s", rootAddr.Hex())

	if err := validatePattern(); err != nil {
		return err
	}

	// Compute per-level funding amounts bottom-up. need[i] is the amount a
	// wallet at level i must receive so it can fund its two children plus gas.
	// Levels are 1..len(batchPattern); level len is the leaf level.
	levels := len(batchPattern)
	gasWei := coins(gasBudgetCoins)
	twoGasWei := new(big.Int).Mul(gasWei, big.NewInt(2))

	need := make([]*big.Int, levels+1) // 1-indexed by level
	need[levels] = coins(leafCoins)
	for i := levels - 1; i >= 1; i-- {
		// need[i] = 2*need[i+1] + 2*gas
		need[i] = new(big.Int).Add(new(big.Int).Mul(need[i+1], big.NewInt(2)), twoGasWei)
	}

	signer := types.LatestSignerForChainID(chainID)
	signingCtx := cryptobase.GetSigningContext()

	totalTxns := 0
	for _, spec := range batchPattern {
		totalTxns += spec.funded
	}
	logf("Generating %d transactions across %d batches (startBlockNumber=%d)", totalTxns, len(batchPattern), startBlock)

	txns := make([]core.TxnTestTransaction, 0, totalTxns)
	signedSoFar := 0

	// level 0 senders is just the root.
	senders := []*signaturealgorithm.PrivateKey{rootKey}
	for idx, spec := range batchPattern {
		level := idx + 1
		valuePerChild := need[level]
		logf("Batch %d/%d (batchNumber=%d): generating %d transactions, %s coins each",
			level, len(batchPattern), spec.batchNumber, spec.funded, weiToCoins(valuePerChild))

		newWallets := make([]*signaturealgorithm.PrivateKey, 0, spec.funded)
		// Log intra-batch progress roughly every ~10% for large batches.
		progressStep := spec.funded / 10
		for _, sender := range senders {
			// Each sender funds exactly two children, using nonces 0 and 1.
			for nonce := uint64(0); nonce < 2; nonce++ {
				childKey, err := cryptobase.SigAlg.GenerateKey()
				if err != nil {
					return fmt.Errorf("failed to generate wallet: %w", err)
				}
				childAddr := cryptobase.SigAlg.PublicKeyToAddressNoError(&childKey.PublicKey)

				tx := types.NewDynamicFeeTransaction(chainID, nonce, &childAddr, valuePerChild, params.TxGas, signingCtx, nil, nil)
				signed, err := types.SignTx(tx, signer, sender)
				if err != nil {
					return fmt.Errorf("failed to sign tx (batch %d): %w", spec.batchNumber, err)
				}
				raw, err := signed.MarshalBinary()
				if err != nil {
					return fmt.Errorf("failed to encode tx (batch %d): %w", spec.batchNumber, err)
				}

				txns = append(txns, core.TxnTestTransaction{
					BatchNumber: spec.batchNumber,
					TxnHex:      "0x" + hex.EncodeToString(raw),
				})
				newWallets = append(newWallets, childKey)
				signedSoFar++

				if progressStep > 0 && len(newWallets)%progressStep == 0 && len(newWallets) != spec.funded {
					logf("  batch %d: %d/%d signed (overall %d/%d)", spec.batchNumber, len(newWallets), spec.funded, signedSoFar, totalTxns)
				}
			}
		}

		if len(newWallets) != spec.funded {
			return fmt.Errorf("internal error: batch %d produced %d wallets, expected %d", spec.batchNumber, len(newWallets), spec.funded)
		}
		logf("  batch %d complete: %d transactions (overall %d/%d)", spec.batchNumber, spec.funded, signedSoFar, totalTxns)
		senders = newWallets
	}

	logf("Signing complete: %d transactions. Writing %s ...", len(txns), outputPath)

	out := core.TxnTestTransactions{
		StartBlockNumber: int64(startBlock),
		Transactions:     txns,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputPath, err)
	}

	printSummary(outputPath, startBlock, need, len(txns))
	return nil
}

// loadRootKey decrypts the root wallet keystore at path using the password.
func loadRootKey(path, password string) (*signaturealgorithm.PrivateKey, error) {
	keyjson, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := keystore.DecryptKey(keyjson, password)
	if err != nil {
		return nil, err
	}
	return key.PrivateKey, nil
}

// validatePattern ensures the funded counts follow the doubling rule.
func validatePattern() error {
	prev := 1 // root level size
	for _, spec := range batchPattern {
		if spec.funded != prev*2 {
			return fmt.Errorf("invalid batch pattern: batch %d funds %d wallets, expected %d (2x previous level)", spec.batchNumber, spec.funded, prev*2)
		}
		prev = spec.funded
	}
	return nil
}

// logf writes a timestamped progress line to stderr.
func logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

// coins converts a whole number of coins to wei (1 coin == 1e18 wei).
func coins(n int64) *big.Int {
	return params.EtherToWei(big.NewInt(n))
}

// weiToCoins renders a wei amount as a decimal coin count for display. The
// amounts here are always whole multiples of a coin minus tiny gas dust, so we
// report the integer coin portion.
func weiToCoins(wei *big.Int) string {
	c := new(big.Int).Div(wei, big.NewInt(params.Ether))
	return c.String()
}

func printSummary(outputPath string, startBlock uint64, need []*big.Int, totalTxns int) {
	fmt.Printf("Wrote %s\n", outputPath)
	fmt.Printf("  startBlockNumber: %d\n", startBlock)
	fmt.Printf("  chainID:          %s\n", chainID.String())
	fmt.Printf("  totalTransactions:%d\n", totalTxns)
	fmt.Printf("  leaf amount:      %d coins\n", int64(leafCoins))
	fmt.Printf("  gas budget/txn:   %d coins\n", int64(gasBudgetCoins))

	fmt.Println("  per-batch funding (value sent to each funded wallet):")
	for idx, spec := range batchPattern {
		level := idx + 1
		fmt.Printf("    batch %-2d: %5d txns, %s coins each\n", spec.batchNumber, spec.funded, weiToCoins(need[level]))
	}

	// Required root balance: root sends two transfers of need[1] plus gas.
	rootOut := new(big.Int).Add(new(big.Int).Mul(need[1], big.NewInt(2)), new(big.Int).Mul(coins(gasBudgetCoins), big.NewInt(2)))
	fmt.Printf("  required root balance (approx): %s coins\n", weiToCoins(rootOut))
}
