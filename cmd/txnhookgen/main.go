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
//	txnhookgen -wallet <path> -out <path> [-password <pwd>] [-levels N] [-startNonce N] [-parallelism N] [-startBlock N]
//
// All flags accept either "-flag value" or "-flag=value". -wallet and -out are
// required. -levels is the number of doubling batches (default 16). -startNonce
// is the starting nonce for the root wallet's batch-1 transactions (default 0);
// set it to the root account's current pending nonce when reusing a funded
// wallet. -parallelism is the per-batch concurrent submitter count the hook uses
// (default 4). -startBlock is the block height written as startBlockNumber that
// the hook waits for before submitting (default 100). If -password is omitted,
// it is prompted for interactively. All other parameters (chain id, amounts,
// gas) are hardcoded below.
package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
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

	// defaultLevels is the number of doubling batches generated when the
	// -levels flag is not provided.
	defaultLevels = 16

	// defaultParallelism is the per-batch concurrent submitter count written
	// into the hook file when the -parallelism flag is not provided.
	defaultParallelism = 4

	// defaultStartBlock is the startBlockNumber written into the hook file when
	// the -startBlock flag is not provided.
	defaultStartBlock = 100

	// maxLevels guards against absurd sizes / integer overflow (2^level).
	maxLevels = 30
)

// chainID is the network chain id used for signing.
var chainID = big.NewInt(types.DEFAULT_CHAIN_ID)

// batchSpec is one entry of the funding pattern: the BatchNumber written into
// the hook file and the number of wallets funded in that batch (which equals
// the number of transactions in the batch).
type batchSpec struct {
	batchNumber int64
	funded      int
}

// buildBatchPattern generates a doubling schedule for the given number of
// levels: batch i (1-indexed) funds 2^i wallets, each sender funding two
// children. The batch number equals the level.
func buildBatchPattern(levels int) []batchSpec {
	pattern := make([]batchSpec, levels)
	for i := 1; i <= levels; i++ {
		pattern[i-1] = batchSpec{batchNumber: int64(i), funded: 1 << uint(i)}
	}
	return pattern
}

func main() {
	inputWalletPath := flag.String("wallet", "", "path to the funded root wallet keystore file (required)")
	outputPath := flag.String("out", "", "path to write the generated hook JSON file (required)")
	password := flag.String("password", "", "root wallet password (prompted interactively if omitted)")
	passwordSet := false
	levels := flag.Int("levels", defaultLevels, "number of doubling batches (each sender funds 2 children per level)")
	startNonce := flag.Uint64("startNonce", 0, "starting nonce for the root wallet's batch-1 transactions (other wallets are fresh and start at 0)")
	parallelism := flag.Int("parallelism", defaultParallelism, "number of concurrent submitters the hook uses per batch")
	startBlock := flag.Uint64("startBlock", defaultStartBlock, "block number written as startBlockNumber; the hook waits for this height before submitting")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s -wallet <path> -out <path> [-password <pwd>] [-levels N] [-startNonce N] [-parallelism N] [-startBlock N]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "all flags accept either -flag value or -flag=value\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "password" {
			passwordSet = true
		}
	})

	if *inputWalletPath == "" || *outputPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	if *levels < 1 || *levels > maxLevels {
		fmt.Fprintf(os.Stderr, "error: levels must be between 1 and %d\n", maxLevels)
		os.Exit(2)
	}

	if *parallelism < 1 {
		fmt.Fprintf(os.Stderr, "error: parallelism must be at least 1\n")
		os.Exit(2)
	}

	// Use the password flag when provided; otherwise prompt for it
	// interactively (same approach as cmd/dputil).
	pwd := *password
	if !passwordSet {
		entered, err := prompt.Stdin.PromptPassword("Enter the wallet password : ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to read password: %v\n", err)
			os.Exit(1)
		}
		pwd = entered
	}

	if err := run(*inputWalletPath, *outputPath, pwd, *levels, *startNonce, *parallelism, *startBlock); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(inputWalletPath, outputPath, password string, levels int, startNonce uint64, parallelism int, startBlock uint64) error {
	// Select the network config (devnet via Q_DEFAULT_CONFIG=1) so signing and
	// related parameters reflect the target network.
	defaults.LoadDefaultConfig()

	logf("Loading root wallet from %s", inputWalletPath)
	rootKey, err := loadRootKey(inputWalletPath, password)
	if err != nil {
		return fmt.Errorf("failed to load root wallet: %w", err)
	}
	rootAddr := cryptobase.SigAlg.PublicKeyToAddressNoError(&rootKey.PublicKey)
	logf("Root wallet loaded: %s", rootAddr.Hex())

	batchPattern := buildBatchPattern(levels)

	// Compute per-level funding amounts bottom-up. need[i] is the amount a
	// wallet at level i must receive so it can fund its two children plus gas.
	// Levels are 1..levels; the last level is the leaf level.
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
		// The root is the only sender of batch 1 (level 1) and is a reused,
		// pre-funded account, so honor startNonce. Every other sender is a
		// freshly created wallet whose on-chain nonce is 0.
		baseNonce := uint64(0)
		if idx == 0 {
			baseNonce = startNonce
		}
		for _, sender := range senders {
			// Each sender funds exactly two children, using consecutive nonces.
			for j := uint64(0); j < 2; j++ {
				nonce := baseNonce + j
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
		Parallelism:      parallelism,
		Transactions:     txns,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputPath, err)
	}

	printSummary(outputPath, startBlock, startNonce, parallelism, batchPattern, need, len(txns))
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

func printSummary(outputPath string, startBlock uint64, startNonce uint64, parallelism int, batchPattern []batchSpec, need []*big.Int, totalTxns int) {
	fmt.Printf("Wrote %s\n", outputPath)
	fmt.Printf("  startBlockNumber: %d\n", startBlock)
	fmt.Printf("  chainID:          %s\n", chainID.String())
	fmt.Printf("  rootStartNonce:   %d\n", startNonce)
	fmt.Printf("  parallelism:      %d\n", parallelism)
	fmt.Printf("  levels:           %d\n", len(batchPattern))
	fmt.Printf("  totalTransactions:%d\n", totalTxns)
	fmt.Printf("  leaf amount:      %d coins\n", int64(leafCoins))
	fmt.Printf("  gas budget/txn:   %d coins\n", int64(gasBudgetCoins))

	fmt.Println("  per-batch funding (value sent to each funded wallet):")
	for idx, spec := range batchPattern {
		level := idx + 1
		fmt.Printf("    batch %-3d: %8d txns, %s coins each\n", spec.batchNumber, spec.funded, weiToCoins(need[level]))
	}

	// Required root balance: root sends two transfers of need[1] plus gas.
	rootOut := new(big.Int).Add(new(big.Int).Mul(need[1], big.NewInt(2)), new(big.Int).Mul(coins(gasBudgetCoins), big.NewInt(2)))
	fmt.Printf("  required root balance (approx): %s coins\n", weiToCoins(rootOut))
}
