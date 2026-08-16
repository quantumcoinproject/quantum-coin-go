// ============================================================================
// STAKING DEVNET RUNBOOK — how to re-validate the stakingv3 bond flows on a
// live devnet end to end (proven 2026-08; follow verbatim next time).
// ============================================================================
//
// Bootstrap: use the full devnet runbook in cmd/txnhookgen/main.go (build
// dp.exe + dputil.exe, delete <dir>\data, `dp init` with
// consensus\proofofstake\genesis\devnet\genesis.json, copy both keystores from
// resources\devnet\ — password QuantumCoinExample123!, funded root
// 0x1a84...aec6 — then start the single-validator node with Q_DEFAULT_CONFIG=1,
// MIN_VALIDATORS=1, SKIP_STARTUP_DELAY=1, BLOCK_EXTENDED_SAVE=1, DP_ACC_PWD,
// --mine --http). No TXN_HOOK_FILE is needed for staking tests. If another
// devnet node is running, use --port 30310 --http.port 8546 --ipcpath
// geth-hooktest.ipc instead of killing it.
//
// Staking-specific facts (all hit for real in the Aug 2026 run):
//   - The v3 bytecode swaps in at PosConfig.SystemContractV3StartBlock
//     (devnet 74; defaults/config.go). The last devnet fork is GasV3 at 82 —
//     wait until head >= 85 before sending any staking transaction.
//   - Do NOT send ANY transaction (even plain transfers) before devnet block 52
//     (PosConfig.SigAlgSwitchBlock): dputil signs with MLDSA (mode 3), which the
//     txpool rejects below that fork ("txpool tx signature type is not allowed").
//   - Gas is expensive: DEFAULT_PRICE ~0.0476 coins/gas, so a bond costs ~8.3k
//     coins and a deposit/rotation ~19k upfront (mostly refunded). Fund
//     depositors 6,050,000+ and validators ~20,000 from the root wallet with
//     `dputil transfercoins` (DP_ACC_PWD + SHOULD_CONFIRM=no make it prompt-free).
//     Give the negative-case depositor the full 6M too, so its failure is
//     provably the bond gate and not insufficient balance.
//   - MINIMUM_DEPOSIT is 5,000,000 coins; use 6,000,000 in tests.
//   - Create fresh wallets with `dp.exe account new --password <file>`
//     (Q_DEFAULT_CONFIG=1, same datadir keystore). dputil env: DP_RAW_URL,
//     DP_KEY_FILE_DIR=<datadir>\keystore.
//   - dputil prompts on stdin; pipe answers with LF-only line endings (bash
//     `printf '...\n' | ./dputil.exe ...`). PowerShell piping appends CRLF and
//     the \r corrupts the password ("could not decrypt key").
//     Prompt order — bonddepositor/bonddepositorforrotation: validator pwd.
//     stakingdeposit: depositor pwd, y, validator pwd, y.
//     changevalidator: depositor pwd, validator pwd, y, y (and it requires the
//     NEW validator's key file in DP_KEY_FILE_DIR, so negatives against an
//     address without a local key file must use a freshly created wallet).
//   - Expected-failure txns are accepted into the pool and revert ON-CHAIN:
//     evidence is the status-0 receipt (eth_getTransactionReceipt), optionally
//     tracer_traceTransaction for the revert point.
//
// Onboarding sequence (mirrors TestStakingV3_AntiFrontRunAttackerFirst below):
//  1. dputil bonddepositor X D    — attacker validator X bonds depositor D it
//     does NOT own the key of. Succeeds: anyone may ATTEMPT to bond.
//  2. dputil bonddepositor V D    — owner's validator V bonds the SAME D.
//     Succeeds: pair-keyed, the attacker bond neither overwrites nor blocks.
//  3. dputil stakingdeposit AD X 6000000 — attacker's own depositor AD naming
//     X. FAILS ("Validator has not bonded depositor"): X's bond is keyed to D.
//  4. dputil stakingdeposit D V 6000000  — succeeds despite the competing bond.
//     Verify: getstakingdetails V shows depositor D, balance 6,000,000.
//
// NIL-block verification (no node runs the new validator's key, so its slots
// go nil while the chain keeps progressing; each nil round takes ~60s, so the
// chain visibly slows while the offline validator is in the set): scan recent
// heights via proofofstake_getBlockConsensusData (or `dputil block N`). In a
// NIL block the consensus data has voteType == 2 (VOTE_TYPE_NIL) and
// blockProposer ZEROED — the timed-out proposer is listed in
// nilvotedBlockProposers instead. Cross-check `dputil getstakingdetails V`:
// Nil Block Count > 0, recent Last NiL Block, and nil slashing of 100
// coins/block accruing against the depositor's net balance.
//
// Rotation sequence (mirrors TestStakingV3_RotationAntiFrontRunAttackerFirst
// and TestStakingV3_RotationNegatives below):
//  5. dputil bonddepositorforrotation X2 <fresh-addr> — FAILS ("Depositor does
//     not exist"): rotation bonds require an existing depositor.
//  6. dputil bonddepositorforrotation X2 D — attacker rotation-bond FIRST;
//     succeeds (attempt allowed). Then `dputil changevalidator D <unbonded
//     fresh addr>` FAILS ("New validator has not bonded depositor").
//  7. dputil bonddepositorforrotation N D — owner's new validator; succeeds.
//  8. dputil changevalidator D N — succeeds; getstakingdetails N shows D and
//     the old validator V no longer resolves. The NIL verification for N is
//     the MIGRATED state on N's staking details (Last NiL Block / Nil Block
//     Count / slashing carried over from V) — do NOT wait for a fresh nil
//     block by N: with NilBlockCount > 1 the proposer selection defers the
//     offline validator for MinOfflineProposerBlockDelay (devnet 3600) blocks
//     past LastNiLBlock (blockproposer.go canPropose), so post-rotation blocks
//     show zero nil rounds. That absence, plus the migrated counters, is the
//     expected evidence.
//
// Record every expected failure with its error text or status-0 receipt hash
// (dputil txn TXN_HASH) — never assert a negative without evidence.
//
// dputil gas-limit gotcha (fixed Aug 2026): the v3 contract needs MORE gas than
// the old hardcoded dputil limits — measured newDeposit ~256k (was capped 250k)
// and changeValidator ~227k (was capped 175k); both out-of-gas reverted with a
// status-0 receipt that LOOKS like a require() failure. cmd/dputil/util.go now
// uses 400k for both. When a positive case unexpectedly reverts, run
// eth_estimateGas with the txn's exact from/to/value/data before suspecting the
// contract.
// ============================================================================

package proofofstake

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking/stakingv3"
	"math/big"
	"testing"
)

// This file REUSES the shared helpers/constants declared in stakingv2_test.go (same package proofofstake):
// ContractAddress, tcc, execute, encodeCall, ZERO_HASH, ZERO_ADDRESS, MIN_VALIDATOR_DEPOSIT, NewDeposit,
// IncreaseDeposit, InitiatePartialWithdrawal, PauseValidation, ResumeValidation, ChangeValidator,
// GetBalanceOfDepositor, GetNetBalanceOfDepositor, GetTotalDepositedBalance, GetValidatorOfDepositor,
// ListValidators, GetStakingDetails, AddDepositorReward, AddDepositorSlashing, newStakingStateDb, etc.
// It declares ONLY new *V3-suffixed symbols that wire the v3 bytecode + GetStakingContractV3_ABI.

func newStakingStateDbV3() *state.StateDB {
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	statedb.CreateAccount(ContractAddress)
	statedb.SetCode(ContractAddress, common.FromHex(stakingv3.STAKING_RUNTIME_BIN))
	statedb.Finalise(true) // Push the state into the "original" slot

	return statedb
}

// UpgradeV2ToV3 swaps the live v2 bytecode for v3 over the SAME storage, mirroring proofofstake.go SetCode.
func UpgradeV2ToV3(state *state.StateDB) {
	state.SetCode(ContractAddress, common.FromHex(stakingv3.STAKING_RUNTIME_BIN))
	state.Finalise(true)
}

func NewDepositV3(state *state.StateDB, depositor common.Address, validator common.Address, amount *big.Int) error {
	method := staking.GetContract_Method_NewDeposit()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("NewDepositV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, validator)
	if err != nil {
		log.Error("Unable to pack NewDepositV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, depositor, state, header, amount)
	if err != nil {
		return err
	}

	return nil
}

// BondDepositorV3 has the validator (msg.sender) bond a depositor (onboarding flow).
func BondDepositorV3(state *state.StateDB, validator common.Address, depositor common.Address) error {
	method := staking.GetContract_Method_BondDepositor()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("BondDepositorV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, depositor)
	if err != nil {
		log.Error("Unable to pack BondDepositorV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, validator, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

// BondDepositorForRotationV3 has the NEW validator (msg.sender) bond an existing depositor for rotation.
func BondDepositorForRotationV3(state *state.StateDB, newValidator common.Address, depositor common.Address) error {
	method := staking.GetContract_Method_BondDepositorForRotation()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("BondDepositorForRotationV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, depositor)
	if err != nil {
		log.Error("Unable to pack BondDepositorForRotationV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, newValidator, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

// IsDepositorBondedV3 reads the pair-keyed bond (depositor => validator => bonded). Serves both flows.
func IsDepositorBondedV3(state *state.StateDB, depositor common.Address, validator common.Address) (bool, error) {
	var out bool

	method := staking.GetContract_Method_IsDepositorBonded()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("IsDepositorBondedV3 abi error", "err", err)
		return out, err
	}
	data, err := encodeCall(&abiData, method, depositor, validator)
	if err != nil {
		log.Error("Unable to pack IsDepositorBondedV3", "error", err)
		return out, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, ZERO_ADDRESS, state, header, new(big.Int))
	if err != nil {
		return out, err
	}

	if len(result) == 0 {
		return out, errors.New("IsDepositorBondedV3 result is 0")
	}
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err)
		return out, err
	}

	return out, nil
}

func ChangeValidatorV3(state *state.StateDB, depositor common.Address, newValidatorAddress common.Address) error {
	method := staking.GetContract_Method_ChangeValidator()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("ChangeValidatorV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, newValidatorAddress)
	if err != nil {
		log.Error("Unable to pack ChangeValidatorV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, depositor, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

func GetValidatorOfDepositorV3(state *state.StateDB, depositor common.Address) (common.Address, error) {
	method := staking.GetContract_Method_GetValidatorOfDepositor()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("GetValidatorOfDepositorV3 abi error", "err", err)
		return ZERO_ADDRESS, err
	}
	data, err := encodeCall(&abiData, method, depositor)
	if err != nil {
		log.Error("Unable to pack GetValidatorOfDepositorV3", "error", err)
		return ZERO_ADDRESS, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, depositor, state, header, new(big.Int))
	if err != nil {
		return ZERO_ADDRESS, err
	}

	if len(result) == 0 {
		return ZERO_ADDRESS, errors.New("GetValidatorOfDepositorV3 result is 0")
	}

	out := new(common.Address)
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err, "depositor", depositor)
		return ZERO_ADDRESS, err
	}

	return *out, nil
}

func GetBalanceOfDepositorV3(state *state.StateDB, depositor common.Address) (*big.Int, error) {
	method := staking.GetContract_Method_GetBalanceOfDepositor()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("GetBalanceOfDepositorV3 abi error", "err", err)
		return nil, err
	}
	data, err := encodeCall(&abiData, method, depositor)
	if err != nil {
		log.Error("Unable to pack GetBalanceOfDepositorV3", "error", err)
		return nil, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, depositor, state, header, new(big.Int))
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("GetBalanceOfDepositorV3 result is 0")
	}

	var out *big.Int
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func GetNetBalanceOfDepositorV3(state *state.StateDB, depositor common.Address) (*big.Int, error) {
	method := staking.GetContract_Method_GetNetBalanceOfDepositor()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("GetNetBalanceOfDepositorV3 abi error", "err", err)
		return nil, err
	}
	data, err := encodeCall(&abiData, method, depositor)
	if err != nil {
		log.Error("Unable to pack GetNetBalanceOfDepositorV3", "error", err)
		return nil, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, depositor, state, header, new(big.Int))
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("GetNetBalanceOfDepositorV3 result is 0")
	}

	var out *big.Int
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func GetTotalDepositedBalanceV3(state *state.StateDB) (*big.Int, error) {
	method := staking.GetContract_Method_GetTotalDepositedBalance()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("GetTotalDepositedBalanceV3 abi error", "err", err)
		return nil, err
	}
	data, err := encodeCall(&abiData, method)
	if err != nil {
		log.Error("Unable to pack GetTotalDepositedBalanceV3", "error", err)
		return nil, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, ZERO_ADDRESS, state, header, new(big.Int))
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("GetTotalDepositedBalanceV3 result is 0")
	}

	var out *big.Int
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err)
		return nil, err
	}

	return out, nil
}

func DoesValidatorExistV3(state *state.StateDB, validator common.Address) (bool, error) {
	var out bool

	method := staking.GetContract_Method_DoesValidatorExist()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("DoesValidatorExistV3 abi error", "err", err)
		return out, err
	}
	data, err := encodeCall(&abiData, method, validator)
	if err != nil {
		log.Error("Unable to pack DoesValidatorExistV3", "error", err)
		return out, err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	result, err := execute(tcc, data, validator, state, header, new(big.Int))
	if err != nil {
		return out, err
	}

	if len(result) == 0 {
		return out, errors.New("DoesValidatorExistV3 result is 0")
	}
	if err = abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err, "validator", validator)
		return out, err
	}

	return out, nil
}

// AddDepositorRewardV3 is a VM-only call (msg.sender must be address(0)); it takes (depositor, amount).
func AddDepositorRewardV3(state *state.StateDB, from common.Address, depositor common.Address, amount *big.Int) error {
	method := staking.GetContract_Method_AddDepositorReward()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("AddDepositorRewardV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, depositor, amount)
	if err != nil {
		log.Error("Unable to pack AddDepositorRewardV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, from, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

// IncreaseDepositV3 sends the additional deposit as msg.value (increaseDeposit is payable with no args).
func IncreaseDepositV3(state *state.StateDB, depositor common.Address, amount *big.Int) error {
	method := staking.GetContract_Method_IncreaseDeposit()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("IncreaseDepositV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method)
	if err != nil {
		log.Error("Unable to pack IncreaseDepositV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, depositor, state, header, amount)
	if err != nil {
		return err
	}

	return nil
}

// CompletePartialWithdrawalV3 takes an explicit block number so the WITHDRAWAL_BLOCK_DELAY cutoff can be reached.
func CompletePartialWithdrawalV3(state *state.StateDB, depositor common.Address, currentBlockNumber uint64) error {
	method := staking.GetContract_Method_CompletePartialWithdrawal()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("CompletePartialWithdrawalV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method)
	if err != nil {
		log.Error("Unable to pack CompletePartialWithdrawalV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, currentBlockNumber)

	_, err = execute(tcc, data, depositor, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

// registerDepositorV3 funds the depositor, bonds the pair, and performs newDeposit (the onboarding happy path).
func registerDepositorV3(t *testing.T, state *state.StateDB, depositor common.Address, validator common.Address) {
	t.Helper()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))
	if err := BondDepositorV3(state, validator, depositor); err != nil {
		t.Fatal("bondDepositor failed", err)
	}
	if err := NewDepositV3(state, depositor, validator, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("newDeposit failed", err)
	}
}

// Positive baseline: validator bonds depositor -> newDeposit succeeds; the bond is NOT cleared after deposit.
func TestStakingV3_Basic(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state := newStakingStateDbV3()

	state.SetBalance(depositor, params.EtherToWei(big.NewInt(10000000)))

	if err := BondDepositorV3(state, validator, depositor); err != nil {
		t.Fatal(err)
	}

	bonded, err := IsDepositorBondedV3(state, depositor, validator)
	if err != nil {
		t.Fatal(err)
	}
	if bonded == false {
		t.Fatal("pair should be bonded after bondDepositor")
	}

	depositAmount := MIN_VALIDATOR_DEPOSIT
	if err := NewDepositV3(state, depositor, validator, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal(err)
	}

	// Bond pair is intentionally retained after a successful deposit.
	bonded, err = IsDepositorBondedV3(state, depositor, validator)
	if err != nil {
		t.Fatal(err)
	}
	if bonded == false {
		t.Fatal("bond pair should be retained after deposit")
	}

	stakingBalance, err := GetBalanceOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if stakingBalance.Cmp(depositAmount) != 0 {
		t.Fatalf("balance compare failed")
	}

	stakingNetBalance, err := GetNetBalanceOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if stakingNetBalance.Cmp(depositAmount) != 0 {
		t.Fatalf("net balance compare failed")
	}

	totalDepositedBalance, err := GetTotalDepositedBalanceV3(state)
	if err != nil {
		t.Fatal(err)
	}
	if totalDepositedBalance.Cmp(depositAmount) != 0 {
		t.Fatalf("totalDepositedBalance compare failed")
	}

	doesValExist, err := DoesValidatorExistV3(state, validator)
	if err != nil {
		t.Fatal(err)
	}
	if doesValExist == false {
		t.Fatal("validator should exist after deposit")
	}
}

// F1/F3: newDeposit without any bond must revert.
func TestStakingV3_DepositRequiresBond(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state := newStakingStateDbV3()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(10000000)))

	if err := NewDepositV3(state, depositor, validator, MIN_VALIDATOR_DEPOSIT); err == nil {
		t.Fatal("newDeposit should fail without a bond")
	}
}

// F1: an unauthorized newDeposit(address(0)) must revert (retained non-zero require + false bond gate).
func TestStakingV3_DepositZeroValidatorReverts(t *testing.T) {
	depositor := common.RandomAddress()
	state := newStakingStateDbV3()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(10000000)))

	if err := NewDepositV3(state, depositor, ZERO_ADDRESS, MIN_VALIDATOR_DEPOSIT); err == nil {
		t.Fatal("newDeposit(address(0)) should revert")
	}
}

// F3: validator A bonds depositor, depositor deposits naming validator B (never bonded) -> revert.
func TestStakingV3_DepositWithDifferentValidatorReverts(t *testing.T) {
	depositor := common.RandomAddress()
	validatorA := common.RandomAddress()
	validatorB := common.RandomAddress()
	state := newStakingStateDbV3()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(10000000)))

	if err := BondDepositorV3(state, validatorA, depositor); err != nil {
		t.Fatal(err)
	}

	if err := NewDepositV3(state, depositor, validatorB, MIN_VALIDATOR_DEPOSIT); err == nil {
		t.Fatal("newDeposit should fail when depositing for a non-bonded validator")
	}
}

// F3 anti-front-run: A bonds depositor; attacker B bonds the SAME depositor; newDeposit(A) STILL succeeds.
func TestStakingV3_AntiFrontRun(t *testing.T) {
	depositor := common.RandomAddress()
	validatorA := common.RandomAddress()
	validatorB := common.RandomAddress()
	state := newStakingStateDbV3()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(10000000)))

	if err := BondDepositorV3(state, validatorA, depositor); err != nil {
		t.Fatal("legit bond failed", err)
	}
	// A competing bond for a DIFFERENT validator (depositor side is non-exclusive/pair-keyed).
	if err := BondDepositorV3(state, validatorB, depositor); err != nil {
		t.Fatal("competing bond should succeed (pair-keyed)", err)
	}

	bondedA, _ := IsDepositorBondedV3(state, depositor, validatorA)
	bondedB, _ := IsDepositorBondedV3(state, depositor, validatorB)
	if bondedA == false || bondedB == false {
		t.Fatal("both pairs should be bonded")
	}

	// The legit pair's deposit is unaffected by the competing bond.
	if err := NewDepositV3(state, depositor, validatorA, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("deposit for the legit bonded validator should succeed despite a competing bond", err)
	}
}

// F3 anti-front-run, attacker-FIRST ordering: attacker validator X bonds a depositor address it does
// not own BEFORE the legit validator; the legit bond and deposit are unaffected, and the attacker's
// bond cannot be exercised from any other depositor address (pair-keyed, msg.sender-gated).
func TestStakingV3_AntiFrontRunAttackerFirst(t *testing.T) {
	depositor := common.RandomAddress()
	attackerValidator := common.RandomAddress()
	attackerDepositor := common.RandomAddress()
	legitValidator := common.RandomAddress()
	state := newStakingStateDbV3()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(10000000)))
	state.SetBalance(attackerDepositor, params.EtherToWei(big.NewInt(10000000)))

	// Attacker bonds the victim's depositor address FIRST (anyone may attempt to bond any address).
	if err := BondDepositorV3(state, attackerValidator, depositor); err != nil {
		t.Fatal("attacker bond attempt should succeed", err)
	}

	// The legit validator's bond on the SAME depositor must still succeed (no overwrite/block).
	if err := BondDepositorV3(state, legitValidator, depositor); err != nil {
		t.Fatal("legit bond after attacker bond should succeed", err)
	}

	bondedAttacker, _ := IsDepositorBondedV3(state, depositor, attackerValidator)
	bondedLegit, _ := IsDepositorBondedV3(state, depositor, legitValidator)
	if bondedAttacker == false || bondedLegit == false {
		t.Fatal("both pairs should be bonded")
	}

	// The attacker cannot exercise its bond from its own depositor address (bond is keyed to the victim).
	if err := NewDepositV3(state, attackerDepositor, attackerValidator, MIN_VALIDATOR_DEPOSIT); err == nil {
		t.Fatal("attacker deposit from a different depositor address should revert")
	}

	// The legit depositor's deposit is unaffected by the pre-existing attacker bond.
	if err := NewDepositV3(state, depositor, legitValidator, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("legit deposit should succeed despite the attacker's earlier bond", err)
	}
}

// F13: a validator may bond AT MOST ONE depositor; re-bonding (different or same depositor) reverts.
func TestStakingV3_ValidatorExclusivity(t *testing.T) {
	validator := common.RandomAddress()
	depositor1 := common.RandomAddress()
	depositor2 := common.RandomAddress()
	state := newStakingStateDbV3()

	if err := BondDepositorV3(state, validator, depositor1); err != nil {
		t.Fatal(err)
	}

	// A second, different depositor must be rejected.
	if err := BondDepositorV3(state, validator, depositor2); err == nil {
		t.Fatal("validator bonding a second depositor should revert")
	}
	bonded2, _ := IsDepositorBondedV3(state, depositor2, validator)
	if bonded2 == true {
		t.Fatal("the rejected second bond must not be recorded")
	}

	// Re-bonding the SAME depositor must also be rejected (one-shot).
	if err := BondDepositorV3(state, validator, depositor1); err == nil {
		t.Fatal("re-bonding the same depositor should revert")
	}
}

// F13: multiple validators bond the same depositor (all succeed), but only ONE newDeposit can win.
func TestStakingV3_MultipleValidatorsOneWinner(t *testing.T) {
	depositor := common.RandomAddress()
	validator1 := common.RandomAddress()
	validator2 := common.RandomAddress()
	state := newStakingStateDbV3()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))

	if err := BondDepositorV3(state, validator1, depositor); err != nil {
		t.Fatal(err)
	}
	if err := BondDepositorV3(state, validator2, depositor); err != nil {
		t.Fatal(err)
	}

	b1, _ := IsDepositorBondedV3(state, depositor, validator1)
	b2, _ := IsDepositorBondedV3(state, depositor, validator2)
	if b1 == false || b2 == false {
		t.Fatal("both validators should be bonded to the depositor")
	}

	if err := NewDepositV3(state, depositor, validator1, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("first deposit should succeed", err)
	}

	// The second deposit fails because the depositor now already exists.
	if err := NewDepositV3(state, depositor, validator2, MIN_VALIDATOR_DEPOSIT); err == nil {
		t.Fatal("second deposit for the same depositor should revert")
	}
}

// bondDepositor input validation: self-bond, address(0), and already-existing parties must revert.
func TestStakingV3_BondDepositorValidation(t *testing.T) {
	state := newStakingStateDbV3()

	self := common.RandomAddress()
	if err := BondDepositorV3(state, self, self); err == nil {
		t.Fatal("self bond should revert")
	}

	validator := common.RandomAddress()
	if err := BondDepositorV3(state, validator, ZERO_ADDRESS); err == nil {
		t.Fatal("bondDepositor(address(0)) should revert")
	}

	// Register a pair, then a fresh validator cannot bond the already-existing depositor.
	depositor := common.RandomAddress()
	existingValidator := common.RandomAddress()
	registerDepositorV3(t, state, depositor, existingValidator)

	freshValidator := common.RandomAddress()
	if err := BondDepositorV3(state, freshValidator, depositor); err == nil {
		t.Fatal("bonding an already-existing depositor should revert")
	}
}

// F7 reentrancy guard sanity: the lock resets between transactions, so two sequential nonReentrant
// newDeposit calls in the same state both succeed.
func TestStakingV3_ReentrancyLockResets(t *testing.T) {
	state := newStakingStateDbV3()

	depositor1 := common.RandomAddress()
	validator1 := common.RandomAddress()
	depositor2 := common.RandomAddress()
	validator2 := common.RandomAddress()

	registerDepositorV3(t, state, depositor1, validator1)
	registerDepositorV3(t, state, depositor2, validator2)

	total, err := GetTotalDepositedBalanceV3(state)
	if err != nil {
		t.Fatal(err)
	}
	expected := new(big.Int).Mul(MIN_VALIDATOR_DEPOSIT, big.NewInt(2))
	if total.Cmp(expected) != 0 {
		t.Fatal("total deposited balance mismatch after two deposits")
	}
}

// F15 positive: an existing depositor rotates to a new validator that bonded it via bondDepositorForRotation.
func TestStakingV3_RotationBondPositive(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	newValidator := common.RandomAddress()
	state := newStakingStateDbV3()

	registerDepositorV3(t, state, depositor, validator)

	if err := BondDepositorForRotationV3(state, newValidator, depositor); err != nil {
		t.Fatal("bondDepositorForRotation failed", err)
	}

	bonded, _ := IsDepositorBondedV3(state, depositor, newValidator)
	if bonded == false {
		t.Fatal("rotation bond should be recorded")
	}

	if err := ChangeValidatorV3(state, depositor, newValidator); err != nil {
		t.Fatal("changeValidator should succeed after rotation bond", err)
	}

	got, err := GetValidatorOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsEqualTo(newValidator) == false {
		t.Fatal("validator of depositor should be the new validator")
	}

	// The rotation bond pair is intentionally retained.
	bonded, _ = IsDepositorBondedV3(state, depositor, newValidator)
	if bonded == false {
		t.Fatal("rotation bond pair should be retained after changeValidator")
	}
}

// F15 anti-front-run: two new validators bond the same depositor; rotation to the chosen one still succeeds.
func TestStakingV3_RotationAntiFrontRun(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	newValidator1 := common.RandomAddress()
	newValidator2 := common.RandomAddress()
	state := newStakingStateDbV3()

	registerDepositorV3(t, state, depositor, validator)

	if err := BondDepositorForRotationV3(state, newValidator1, depositor); err != nil {
		t.Fatal(err)
	}
	if err := BondDepositorForRotationV3(state, newValidator2, depositor); err != nil {
		t.Fatal("competing rotation bond should succeed (pair-keyed)", err)
	}

	if err := ChangeValidatorV3(state, depositor, newValidator1); err != nil {
		t.Fatal("rotation to the chosen validator should succeed despite a competing bond", err)
	}
}

// F15 anti-front-run, attacker-FIRST ordering: an attacker rotation-bonds an existing depositor
// BEFORE the owner's chosen new validator; the legit rotation bond and changeValidator are unaffected.
func TestStakingV3_RotationAntiFrontRunAttackerFirst(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	attackerValidator := common.RandomAddress()
	newValidator := common.RandomAddress()
	state := newStakingStateDbV3()

	registerDepositorV3(t, state, depositor, validator)

	// Attacker rotation-bonds the depositor FIRST (anyone may attempt).
	if err := BondDepositorForRotationV3(state, attackerValidator, depositor); err != nil {
		t.Fatal("attacker rotation bond attempt should succeed", err)
	}

	// The owner's chosen new validator can still rotation-bond the SAME depositor (no overwrite/block).
	if err := BondDepositorForRotationV3(state, newValidator, depositor); err != nil {
		t.Fatal("legit rotation bond after attacker bond should succeed", err)
	}

	if err := ChangeValidatorV3(state, depositor, newValidator); err != nil {
		t.Fatal("rotation to the owner's chosen validator should succeed despite the attacker's earlier bond", err)
	}

	got, err := GetValidatorOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsEqualTo(newValidator) == false {
		t.Fatal("depositor should be mapped to the owner's chosen validator")
	}
}

// F15 negatives for changeValidator and bondDepositorForRotation.
func TestStakingV3_RotationNegatives(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state := newStakingStateDbV3()
	registerDepositorV3(t, state, depositor, validator)

	// changeValidator to a NOT-yet-bonded validator reverts.
	unbonded := common.RandomAddress()
	if err := ChangeValidatorV3(state, depositor, unbonded); err == nil {
		t.Fatal("changeValidator without a rotation bond should revert")
	}

	// bondDepositorForRotation for a depositor that does NOT exist reverts.
	freshDepositor := common.RandomAddress()
	freshValidator := common.RandomAddress()
	if err := BondDepositorForRotationV3(state, freshValidator, freshDepositor); err == nil {
		t.Fatal("bondDepositorForRotation for a non-existent depositor should revert")
	}

	// bondDepositorForRotation(address(0)) and self-bond revert.
	if err := BondDepositorForRotationV3(state, freshValidator, ZERO_ADDRESS); err == nil {
		t.Fatal("bondDepositorForRotation(address(0)) should revert")
	}
	if err := BondDepositorForRotationV3(state, freshValidator, freshValidator); err == nil {
		t.Fatal("self rotation bond should revert")
	}
}

// F13 one-shot across both bond functions: a validator used once cannot bond again via either function.
func TestStakingV3_RotationOneShot(t *testing.T) {
	depositor1 := common.RandomAddress()
	validator1 := common.RandomAddress()
	depositor2 := common.RandomAddress()
	validator2 := common.RandomAddress()
	state := newStakingStateDbV3()

	// Two registered depositors to rotate.
	registerDepositorV3(t, state, depositor1, validator1)
	registerDepositorV3(t, state, depositor2, validator2)

	newValidator := common.RandomAddress()
	if err := BondDepositorForRotationV3(state, newValidator, depositor1); err != nil {
		t.Fatal(err)
	}
	// The same new validator cannot bond a second depositor for rotation.
	if err := BondDepositorForRotationV3(state, newValidator, depositor2); err == nil {
		t.Fatal("rotation bond one-shot should revert on the second bond")
	}

	// A validator already used via bondDepositor (onboarding) cannot later bondDepositorForRotation.
	usedValidator := common.RandomAddress()
	freshOnboardDepositor := common.RandomAddress()
	if err := BondDepositorV3(state, usedValidator, freshOnboardDepositor); err != nil {
		t.Fatal(err)
	}
	if err := BondDepositorForRotationV3(state, usedValidator, depositor1); err == nil {
		t.Fatal("a validator already bonded via bondDepositor cannot bondDepositorForRotation")
	}
}

// F14 no-regression: participants registered under v2 keep full functionality after the v3 bytecode swap.
func TestStakingV3_ExistingParticipantsUnaffected(t *testing.T) {
	state := newStakingStateDb() // deploys v2 bytecode

	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))

	// Legacy onboarding path (no bond) under v2.
	if err := NewDeposit(state, depositor, validator, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("legacy v2 deposit failed", err)
	}

	// Snapshot pre-upgrade views.
	preBalance, err := GetBalanceOfDepositor(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	preTotal, err := GetTotalDepositedBalance(state)
	if err != nil {
		t.Fatal(err)
	}
	preValidator, err := GetValidatorOfDepositor(state, depositor)
	if err != nil {
		t.Fatal(err)
	}

	// Swap to v3 bytecode over the SAME storage.
	UpgradeV2ToV3(state)

	// State preserved.
	postBalance, err := GetBalanceOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if postBalance.Cmp(preBalance) != 0 {
		t.Fatal("balance changed across upgrade")
	}
	postTotal, err := GetTotalDepositedBalanceV3(state)
	if err != nil {
		t.Fatal(err)
	}
	if postTotal.Cmp(preTotal) != 0 {
		t.Fatal("total changed across upgrade")
	}
	postValidator, err := GetValidatorOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if postValidator.IsEqualTo(preValidator) == false {
		t.Fatal("validator-of-depositor changed across upgrade")
	}

	// Bond map is empty but harmless for the legacy pair.
	bonded, err := IsDepositorBondedV3(state, depositor, validator)
	if err != nil {
		t.Fatal(err)
	}
	if bonded == true {
		t.Fatal("legacy pair should not be bonded")
	}

	// Lifecycle still works for the never-bonded legacy pair.
	if err := IncreaseDepositV3(state, depositor, big.NewInt(1000)); err != nil {
		t.Fatal("increaseDeposit failed after upgrade", err)
	}

	if err := InitiatePartialWithdrawal(state, depositor, big.NewInt(1000), 10); err != nil {
		t.Fatal("initiatePartialWithdrawal failed after upgrade", err)
	}
	if err := CompletePartialWithdrawalV3(state, depositor, 40000); err != nil {
		t.Fatal("completePartialWithdrawal failed after upgrade", err)
	}

	if err := PauseValidation(state, depositor); err != nil {
		t.Fatal("pauseValidation failed after upgrade", err)
	}
	if err := ResumeValidation(state, depositor); err != nil {
		t.Fatal("resumeValidation failed after upgrade", err)
	}

	// Rotation now requires a bond (F15): unbonded rotation reverts; bonded rotation succeeds.
	unbondedValidator := common.RandomAddress()
	if err := ChangeValidatorV3(state, depositor, unbondedValidator); err == nil {
		t.Fatal("rotation to an unbonded validator should revert after upgrade")
	}

	freshValidator := common.RandomAddress()
	if err := BondDepositorForRotationV3(state, freshValidator, depositor); err != nil {
		t.Fatal("rotation bond failed", err)
	}
	if err := ChangeValidatorV3(state, depositor, freshValidator); err != nil {
		t.Fatal("bonded rotation should succeed after upgrade", err)
	}
	got, err := GetValidatorOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsEqualTo(freshValidator) == false {
		t.Fatal("validator should be updated after rotation")
	}

	// VM-only calls (from ZERO_ADDRESS) are unaffected (unguarded per F7).
	if err := AddDepositorRewardV3(state, ZERO_ADDRESS, depositor, big.NewInt(1000)); err != nil {
		t.Fatal("addDepositorReward failed after upgrade", err)
	}
}

// F14 boundary: after the upgrade, a legacy depositor calling newDeposit again still reverts.
func TestStakingV3_LegacyDepositorReDepositStillBlocked(t *testing.T) {
	state := newStakingStateDb()

	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))

	if err := NewDeposit(state, depositor, validator, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("legacy v2 deposit failed", err)
	}

	UpgradeV2ToV3(state)

	if err := NewDepositV3(state, depositor, validator, MIN_VALIDATOR_DEPOSIT); err == nil {
		t.Fatal("legacy depositor re-deposit should still revert after upgrade")
	}
}

// InitiatePartialWithdrawalV3 mirrors the v2 helper but wires the v3 ABI (same selector, new min-remaining guard).
func InitiatePartialWithdrawalV3(state *state.StateDB, depositor common.Address, amount *big.Int, currentBlockNumber uint64) error {
	method := staking.GetContract_Method_InitiatePartialWithdrawal()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("InitiatePartialWithdrawalV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, amount)
	if err != nil {
		log.Error("Unable to pack InitiatePartialWithdrawalV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, currentBlockNumber)

	_, err = execute(tcc, data, depositor, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

// AddDepositorSlashingV3 is a VM-only call (msg.sender must be address(0)); it takes (depositor, amount).
func AddDepositorSlashingV3(state *state.StateDB, from common.Address, depositor common.Address, amount *big.Int) error {
	method := staking.GetContract_Method_AddDepositorSlashing()
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("AddDepositorSlashingV3 abi error", "err", err)
		return err
	}
	data, err := encodeCall(&abiData, method, depositor, amount)
	if err != nil {
		log.Error("Unable to pack AddDepositorSlashingV3", "error", err)
		return err
	}

	header := tcc.GetHeader(ZERO_HASH, uint64(1))

	_, err = execute(tcc, data, from, state, header, new(big.Int))
	if err != nil {
		return err
	}

	return nil
}

// A partial withdrawal must always leave at least the minimum remaining balance; a full exit is not possible.
func TestStakingV3_PartialWithdrawalMinimumRemaining(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state := newStakingStateDbV3()
	registerDepositorV3(t, state, depositor, validator)

	// Net balance starts at the minimum deposit (5,000,000 coins); the retained floor is 1000 coins.
	floor := params.EtherToWei(big.NewInt(1000))

	// Withdrawing the entire balance (remaining 0) must revert: there is no full-exit path in v3.
	if err := InitiatePartialWithdrawalV3(state, depositor, MIN_VALIDATOR_DEPOSIT, 10); err == nil {
		t.Fatal("withdrawing the entire balance should revert (must retain minimum remaining)")
	}

	// Leaving less than the floor (500 coins remaining) must revert.
	leaveBelowFloor := new(big.Int).Sub(MIN_VALIDATOR_DEPOSIT, params.EtherToWei(big.NewInt(500)))
	if err := InitiatePartialWithdrawalV3(state, depositor, leaveBelowFloor, 10); err == nil {
		t.Fatal("leaving less than the minimum remaining balance should revert")
	}

	// Leaving exactly the floor (1000 coins remaining) must succeed (require is >=).
	leaveExactlyFloor := new(big.Int).Sub(MIN_VALIDATOR_DEPOSIT, floor)
	if err := InitiatePartialWithdrawalV3(state, depositor, leaveExactlyFloor, 10); err != nil {
		t.Fatal("leaving exactly the minimum remaining balance should succeed", err)
	}

	// The retained principal equals the floor.
	bal, err := GetBalanceOfDepositorV3(state, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if bal.Cmp(floor) != 0 {
		t.Fatal("remaining balance after withdrawal should equal the minimum floor")
	}
}

// The VM(0) system-call paths must keep working with the added nonReentrant guard, and stay VM-only.
func TestStakingV3_SystemCallsWorkWithReentrancyGuard(t *testing.T) {
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	state := newStakingStateDbV3()
	registerDepositorV3(t, state, depositor, validator)

	if err := AddDepositorRewardV3(state, ZERO_ADDRESS, depositor, params.EtherToWei(big.NewInt(10))); err != nil {
		t.Fatal("addDepositorReward (system call) should succeed with the reentrancy guard", err)
	}
	if err := AddDepositorSlashingV3(state, ZERO_ADDRESS, depositor, params.EtherToWei(big.NewInt(10))); err != nil {
		t.Fatal("addDepositorSlashing (system call) should succeed with the reentrancy guard", err)
	}

	// A non-VM sender must still be rejected.
	if err := AddDepositorSlashingV3(state, depositor, depositor, params.EtherToWei(big.NewInt(10))); err == nil {
		t.Fatal("addDepositorSlashing from a non-VM sender should revert")
	}
}

// The legacy completeWithdrawal entry point must be gone from v3, while partial withdrawal remains.
func TestStakingV3_CompleteWithdrawalRemoved(t *testing.T) {
	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := abiData.Methods["completeWithdrawal"]; ok {
		t.Fatal("completeWithdrawal must not exist in the v3 ABI")
	}

	if _, ok := abiData.Methods["initiatePartialWithdrawal"]; !ok {
		t.Fatal("initiatePartialWithdrawal should still exist in the v3 ABI")
	}
	if _, ok := abiData.Methods["completePartialWithdrawal"]; !ok {
		t.Fatal("completePartialWithdrawal should still exist in the v3 ABI")
	}
}
