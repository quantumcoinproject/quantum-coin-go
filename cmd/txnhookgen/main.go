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
// transactions for TPS / consensus testing, and can verify the outcome of a run
// against a live node over RPC.
//
// ============================================================================
// RUNBOOK — everything needed to reproduce a devnet test run end to end.
// ============================================================================
//
// # Modes
//
// Generate (default):
//
//	txnhookgen -wallet <keystore> -out <hook.json> [-password <pwd>] [-levels N]
//	           [-startNonce N] [-parallelism N] [-startBlock N] [-tokens] [-tips]
//
// Verify (after the node has consumed the hook file):
//
//	txnhookgen -verify http://127.0.0.1:8545 -out <hook.json>
//
// All flags accept either "-flag value" or "-flag=value".
//
// # Flags
//
//   - -wallet:      path to the funded root wallet keystore (generate mode only).
//   - -out:         hook JSON path — written in generate mode, read in verify mode.
//   - -password:    root wallet password; prompted interactively if omitted.
//   - -levels:      number of doubling batches; batch i funds 2^i wallets (default 16).
//   - -startNonce:  root wallet's starting nonce (use its pending nonce when reusing).
//   - -parallelism: concurrent submitters the hook uses per batch (default 4).
//   - -startBlock:  block height the node-side hook waits for before submitting
//     (default 100). See the tip gate below before changing it.
//   - -tokens:      also deploy a tokenv2 ERC20 and mirror the coin funding tree
//     with token transfers (coin txns are "basic", token txns are "general" for
//     the 50/50 gas split — see core/gastip.go IsBasicTransfer).
//   - -tips:        build transactions with GasTipCap/GasFeeCap so the block
//     proposer earns a tip. Tips use a deterministic varied schedule (see
//     tipForIndex in generate.go) so effective-tip ordering is observable, and
//     every third transaction is intentionally TIPLESS (nil caps) — rotating
//     across both coin and token transactions — so blocks carry a
//     tipped/tipless mix and selection preference is testable. Generation
//     fails if the mix would be degenerate (all-tipped or all-tipless per kind).
//   - -tipsascending: tip EVERY transaction, with the tip strictly increasing
//     in generation order (pairwise distinct; see ascendingTipForIndex). Under
//     pool contention the proposer must then include the highest-tip
//     transactions first and exclude the lowest-tip ones to later blocks;
//     verify detects the all-tipped-distinct pattern and enforces this
//     STRICTLY (any cross-sender higher-tip-later-block pair is a failure).
//     Mutually exclusive with -tips; same GasV3StartBlock gate.
//   - -verify:      RPC endpoint; switches to verify mode (reads -out, no signing).
//
// # The tip gate (IMPORTANT)
//
// Tips are only embedded when -startBlock >= the loaded config's
// PosConfig.GasV3StartBlock (the fork where DefaultGasLimitV2 activates;
// devnet 82). Below that height the -tips flag is ignored with a warning.
// Fork heights and gas ceilings always come from defaults/config.go at runtime
// (never hardcode them): set Q_DEFAULT_CONFIG=1 to load the devnet config —
// the generator, the node, and verify mode must all agree on it, otherwise
// signing contexts and fork gates diverge and every transaction is rejected.
//
// # Devnet bootstrap from a clean repo checkout (Windows; adapt paths for sh)
//
//	go build -o dp.exe ./cmd/dp
//	go build -o txnhookgen.exe ./cmd/txnhookgen
//	$env:Q_DEFAULT_CONFIG="1"
//	.\dp.exe --datadir <dir>\data init consensus\proofofstake\genesis\devnet\genesis.json
//	# copy both keystores (validator + funded root) into the node keystore:
//	copy resources\devnet\45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71 <dir>\data\keystore\
//	copy resources\devnet\1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6 <dir>\data\keystore\
//	# keystore password for both accounts: QuantumCoinExample123!
//	# funded root account (~8B coins in genesis): 0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6
//	# validator / block proposer:                 0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71
//
// Generate the hook file (startBlock past the last devnet fork; see the tip gate):
//
//	$env:Q_DEFAULT_CONFIG="1"
//	.\txnhookgen.exe -wallet resources\devnet\1a846abe... -password QuantumCoinExample123! `
//	    -tokens -tips -levels 5 -startBlock 85 -out <dir>\hook.json
//
// Run the single-validator devnet node with the hook enabled:
//
//	$env:Q_DEFAULT_CONFIG="1"       # devnet fork heights + signing config (mandatory)
//	$env:MIN_VALIDATORS="1"         # single-validator consensus
//	$env:SKIP_STARTUP_DELAY="1"     # skip the ~120s startup wait
//	$env:BLOCK_EXTENDED_SAVE="1"    # enables proofofstake_getBlockExtendedDetails
//	$env:DP_ACC_PWD="QuantumCoinExample123!"
//	$env:TXN_HOOK_FILE="<dir>\hook.json"
//	.\dp.exe --datadir <dir>\data --networkid 123123 --syncmode full --gcmode full `
//	    --freezermode skipappend `
//	    --unlock 0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71 `
//	    --miner.etherbase 0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71 `
//	    --mine --http --http.port 8545 `
//	    --http.api eth,net,web3,personal,proofofstake,txpool,tracer --allow-insecure-unlock
//
// Notes: dp logs to stderr (capture with *> node.log). Only one dp instance may
// hold the datadir. Poll eth_blockNumber until it advances rather than sleeping.
// Blocks before GranularBlockTimeStartBlock align proposal times to 60s, so early
// heights can pace slowly; from that fork devnet uses 1s granularity.
//
// # How the node consumes the hook (core/tx_pool_testhook.go)
//
// When TXN_HOOK_FILE is set, NewTxPool starts a hook that: waits until the head
// reaches startBlockNumber, then submits each batch (ascending batchNumber) via
// AddRemotes with `parallelism` goroutines, and waits for every transaction of
// the batch to be committed before moving on (20 min timeout per batch; a failed
// batch aborts the remaining ones). Watch the node log for "Transaction test
// hook" lines: "reached start block", per-batch "committed", and a final
// "Transaction test hook completed" with overall TPS.
//
// # What -verify checks (see verify.go)
//
//  1. Every generated transaction is committed with receipt status 1.
//  2. Block gas limits respect defaults.GetMaxGasLimit at each height
//     (GasV3 ceiling from GasV3StartBlock onward) and the per-block
//     transaction-count bound (defaults.GetMaxTransactionsForBlock).
//  3. The 50/50 basic-vs-general gas split: replays the two-pass accounting of
//     core/state_processor.go ProcessTransactions over each block's receipts and
//     asserts per-pool budgets and exact header.GasUsed.
//  4. Ordering: per-sender nonces strictly sequential across blocks (hard);
//     effective-tip ordering across blocks within a batch (soft — selection is
//     tip-ordered but packing is nonce-ordered; see core/gastip.go).
//  5. Tip payment: per-block staking contract balance delta equals
//     proofofstake.GetReward + TxnFeeRewardsPercentage of base fees + 100% of
//     tips, and the ZERO_ADDRESS "burn" delta equals the remaining base-fee
//     share (consensus/proofofstake/proofofstake.go Finalize). Note: the tip is
//     paid to the proposer's *depositor* via the staking contract — the proposer
//     address balance never changes, and header.Coinbase is always the zero
//     address. The fee split + tip credit only run from
//     defaults.Config.TxnFeeCutoffBlock (mainnet 1607600 — the historical
//     core.TXN_FEE_CUTTOFF_BLOCK; devnet 2, so the path is testable) — below
//     it base fees AND tips are credited to the zero-address coinbase instead;
//     the verifier models both regimes. It also reports the tipped/tipless mix
//     per kind and, when a batch+kind spans multiple blocks (pool contention),
//     how many tipless transactions were excluded from the first block.
//  6. Token conservation (-tokens runs): leaf balanceOf == transferred amount,
//     intermediate wallets net to zero, and total distribution matches totalSupply.
//
// The report prints one PASS/FAIL section per category plus a per-block table;
// the process exits non-zero if any hard check fails.
//
// ============================================================================
// END-TO-END TEST SCENARIOS — replay ALL of these to re-validate the stack
// (miner tips, 50/50 gas split, ordering, exclusion, GasV3 ceiling, tokens).
// Each scenario is: reset chain -> generate hook -> start node -> wait for the
// hook to complete -> run -verify -> expect RESULT: PASS. Full run takes about
// 10-15 minutes per scenario (~5 min for the chain to reach block 85 at ~3s
// per block, then the hook batches).
// ============================================================================
//
// # Common steps for every scenario (see the bootstrap section above for detail)
//
//  1. Build dp.exe and txnhookgen.exe; ALWAYS reset the chain between
//     scenarios: delete <dir>\data, re-run `dp init` with the devnet genesis,
//     re-copy both keystores from resources\devnet\.
//  2. Generate the hook file for the scenario (all use -startBlock 85, which is
//     past the last devnet fork, GasV3StartBlock 82).
//  3. Start the node with the env block above plus TXN_HOOK_FILE=<hook.json>.
//     If another devnet node is already running on this machine (e.g. the
//     downloadable package under a path like C:\github\qc-devnet-pkg), do NOT
//     kill it — add "--port 30310 --http.port 8546 --ipcpath geth-hooktest.ipc"
//     instead; the default \\.\pipe\geth.ipc IPC pipe collision is a hard
//     Fatal, and the HTTP/P2P ports clash too. Then verify against :8546.
//  4. Watch the node log for "Transaction test hook completed", then run
//     -verify PROMPTLY (gcmode full keeps only ~128 recent block states; the
//     balance-delta checks warn and skip if the state is already pruned).
//  5. The run is healthy when every batch logs "accepted=<txCount>" and the
//     verify output ends with "RESULT: PASS (all hard checks green)".
//
// # Scenario 1 — mixed tips + tokens (tip payment, 50/50 split, tipless exclusion)
//
//	txnhookgen -wallet <keystore> -password QuantumCoinExample123! -tokens -tips \
//	    -levels 8 -startBlock 85 -out <dir>\hook.json
//
// 1021 txns (~2/3 tipped, ~1/3 tipless, rotating across coin AND token txns).
// Levels 8 makes batches 7-8 overflow the general pool (105 token txns per
// block: 105 x 100k gas limit == the 10.5M half budget), so tipless exclusion
// is observable. Expect in the verify report: tip payment/burn/rpc-rewards
// checks green (staking delta == GetReward + 40% base fees + 100% tips); "tip
// mix" INFO with all four buckets non-zero; "exclusion" INFO lines showing
// tipless txns excluded from the first block of each contended batch; token
// conservation green; every block's gasLimit == 21M (devnet DefaultGasLimitV2).
//
// # Scenario 2 — ascending tips + tokens (STRICT highest-tip-first inclusion)
//
//	txnhookgen -wallet <keystore> -password QuantumCoinExample123! -tokens \
//	    -tipsascending -levels 9 -startBlock 85 -out <dir>\hook.json
//
// 2045 txns, every one tipped, tips strictly increasing with generation order.
// Levels 9 contends BOTH pools in batch 9: coin fills the basic pool exactly
// (500 x 21000 == 10.5M, the ~12 lowest-tip coin txns excluded to the next
// block) and general spans ~5 blocks (~400 lowest-tip token txns excluded).
// Expect: "all transactions tipped (ascending mode)" INFO, ZERO
// "ascending tips: ... violations" failures, and "ascending exclusion" INFO
// lines whose excluded-tip boundary sits at/below the included-tip boundary
// (an overlap of exactly one tip unit is the same sender's next nonce — exempt).
//
// # Scenario 3 — legacy coins only (no tips, no tokens; optional regression)
//
//	txnhookgen -wallet <keystore> -password QuantumCoinExample123! \
//	    -levels 5 -startBlock 85 -out <dir>\hook.json
//
// Pre-tip behavior: all DynamicFeeTx with nil caps, zero tips everywhere;
// verify still checks commitment, the split replay, ordering, rewards (tips
// all zero), and the GasV3 ceiling.
//
// # Unit tests that accompany these scenarios
//
//	go test ./cmd/txnhookgen/ ./defaults/ ./core/
//	go test ./consensus/proofofstake/ -run 'TestTxnFee|TestCalculateTxnTipTotal'
//
// Consensus knobs these scenarios depend on (all in defaults/config.go — read
// them at runtime, never hardcode): GasTipStartBlock (devnet 72),
// GasV3StartBlock (devnet 82) + DefaultGasLimitV2, TxnFeeCutoffBlock (devnet 2;
// mainnet 1607600 — below it fees AND tips are burned to the zero address and
// tip payment cannot be exercised), TxnFeeRewardsPercentage (devnet 40).
package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/quantumcoinproject/quantum-coin-go/console/prompt"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

const (
	// leafCoins is the amount (in coins) each leaf wallet ends up funded with.
	leafCoins = 1000

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

func main() {
	inputWalletPath := flag.String("wallet", "", "path to the funded root wallet keystore file (required in generate mode)")
	outputPath := flag.String("out", "", "hook JSON path: written in generate mode, read in verify mode (required)")
	password := flag.String("password", "", "root wallet password (prompted interactively if omitted)")
	passwordSet := false
	levels := flag.Int("levels", defaultLevels, "number of doubling batches (each sender funds 2 children per level)")
	startNonce := flag.Uint64("startNonce", 0, "starting nonce for the root wallet's batch-1 transactions (other wallets are fresh and start at 0)")
	parallelism := flag.Int("parallelism", defaultParallelism, "number of concurrent submitters the hook uses per batch")
	startBlock := flag.Uint64("startBlock", defaultStartBlock, "block number written as startBlockNumber; the hook waits for this height before submitting")
	tokens := flag.Bool("tokens", false, "also deploy a tokenv2 ERC20 and mirror the funding tree with token transfers")
	tips := flag.Bool("tips", false, "include miner gas tips with a varied schedule plus tipless slots (only honored when startBlock >= GasV3StartBlock of the loaded config)")
	tipsAscending := flag.Bool("tipsascending", false, "tip EVERY transaction with a strictly increasing tip (ascending with generation order); verify then enforces that only the highest-tip transactions were included first under contention (same GasV3StartBlock gate as -tips)")
	verifyRPC := flag.String("verify", "", "RPC endpoint (e.g. http://127.0.0.1:8545); switches to verify mode, reading the hook JSON at -out")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage (generate): %s -wallet <path> -out <path> [-password <pwd>] [-levels N] [-startNonce N] [-parallelism N] [-startBlock N] [-tokens] [-tips|-tipsascending]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "usage (verify):   %s -verify <rpc-url> -out <path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "all flags accept either -flag value or -flag=value\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "password" {
			passwordSet = true
		}
	})

	// Select the network config (devnet via Q_DEFAULT_CONFIG=1) so signing,
	// fork gates, and gas ceilings reflect the target network in both modes.
	defaults.LoadDefaultConfig()

	if *verifyRPC != "" {
		if *outputPath == "" {
			flag.Usage()
			os.Exit(2)
		}
		if err := runVerify(*verifyRPC, *outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "verify error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *inputWalletPath == "" || *outputPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	if *levels < 1 || *levels > maxLevels {
		fmt.Fprintf(os.Stderr, "error: levels must be between 1 and %d\n", maxLevels)
		os.Exit(2)
	}

	if *tips && *tipsAscending {
		fmt.Fprintf(os.Stderr, "error: -tips and -tipsascending are mutually exclusive\n")
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

	cfg := genConfig{
		levels:        *levels,
		startNonce:    *startNonce,
		parallelism:   *parallelism,
		startBlock:    *startBlock,
		tokens:        *tokens,
		tips:          *tips,
		tipsAscending: *tipsAscending,
	}
	if err := run(*inputWalletPath, *outputPath, pwd, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
