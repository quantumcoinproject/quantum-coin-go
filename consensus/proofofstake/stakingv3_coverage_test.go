package proofofstake

import (
	"math/big"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// This file adds exhaustive positive/negative coverage for every require/assert in the v3
// StakingContract. Negative tests pin the EXACT revert reason (execute() surfaces
// "execution reverted: <reason>" via core.NewRevertError), so they prove which require fired.
//
// Cross-version guards (the *EverExisted flags and the v1-only _depositorWithdrawalRequests
// mapping) cannot be produced by v2/v3 code, but they ARE reachable over storage migrated from a
// v1 deployment. We simulate that migrated storage by writing the exact storage slots with
// state.SetState. The contract has no inheritance storage (the interface declares none), so slots
// follow declaration order in StakingContract.sol; the v3 vars are append-only.

const (
	slotDepositorBalances              uint64 = 1
	slotTotalDepositedBalance          uint64 = 2
	slotValidatorExists                uint64 = 4
	slotDepositorExists                uint64 = 5
	slotValidatorEverExisted           uint64 = 6
	slotDepositorEverExisted           uint64 = 7
	slotDepositorSlashings             uint64 = 10
	slotDepositorRewards               uint64 = 11
	slotDepositorWithdrawalRequests    uint64 = 12
	slotPartialWithdrawalBlockMapping  uint64 = 14
	slotPartialWithdrawalAmountMapping uint64 = 15
	slotDepositorBond                  uint64 = 19
	slotValidatorBonded                uint64 = 20
)

// mustRevert fails unless err is a revert whose reason contains want. This is the core assertion
// that pins each require to its message.
func mustRevert(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected revert %q, got nil error", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected revert containing %q, got %q", want, err.Error())
	}
}

// mapSlot returns the storage slot of mapping(key => _)[key] declared at baseSlot, per Solidity's
// keccak256(leftPad32(key) . leftPad32(baseSlot)) rule.
func mapSlot(key common.Address, baseSlot uint64) common.Hash {
	k := common.LeftPadBytes(key.Bytes(), 32)
	s := common.LeftPadBytes(new(big.Int).SetUint64(baseSlot).Bytes(), 32)
	return common.BytesToHash(crypto.Keccak256(k, s))
}

// nestedMapSlot returns the slot of mapping(k1 => mapping(k2 => _))[k1][k2] at baseSlot.
func nestedMapSlot(k1, k2 common.Address, baseSlot uint64) common.Hash {
	outer := mapSlot(k1, baseSlot)
	k := common.LeftPadBytes(k2.Bytes(), 32)
	return common.BytesToHash(crypto.Keccak256(k, outer.Bytes()))
}

func setBoolFlag(stateDb *state.StateDB, baseSlot uint64, key common.Address, v bool) {
	var val common.Hash
	if v {
		val = common.BytesToHash([]byte{1})
	}
	stateDb.SetState(ContractAddress, mapSlot(key, baseSlot), val)
	stateDb.Finalise(true)
}

func setUintSlot(stateDb *state.StateDB, baseSlot uint64, key common.Address, value *big.Int) {
	stateDb.SetState(ContractAddress, mapSlot(key, baseSlot), common.BigToHash(value))
	stateDb.Finalise(true)
}

func setBondFlag(stateDb *state.StateDB, depositor, validator common.Address, v bool) {
	var val common.Hash
	if v {
		val = common.BytesToHash([]byte{1})
	}
	stateDb.SetState(ContractAddress, nestedMapSlot(depositor, validator, slotDepositorBond), val)
	stateDb.Finalise(true)
}

// plantRevertingCode installs minimal runtime bytecode (PUSH1 0; PUSH1 0; REVERT) so any value-call
// to addr reverts. Used to force the require(success) failure paths.
func plantRevertingCode(stateDb *state.StateDB, addr common.Address) {
	stateDb.SetCode(addr, []byte{0x60, 0x00, 0x60, 0x00, 0xfd})
	stateDb.Finalise(true)
}

// reentrancyAttackerCode builds runtime bytecode that, when invoked (e.g. by the withdrawal
// value-call), re-enters completePartialWithdrawal() on the staking contract and then STOPs
// (swallowing the inner revert). It is used to prove the reentrancy guard plus
// checks-effects-interactions ordering prevent any double withdrawal.
func reentrancyAttackerCode() []byte {
	sel := crypto.Keccak256([]byte("completePartialWithdrawal()"))[:4]
	code := []byte{
		0x63, sel[0], sel[1], sel[2], sel[3], // PUSH4 selector
		0x60, 0xE0, // PUSH1 0xE0
		0x1b,       // SHL    -> selector << 224 (left-aligned)
		0x60, 0x00, // PUSH1 0x00
		0x52,       // MSTORE -> mem[0:32] = selector word
		0x60, 0x00, // PUSH1 0x00 retSize
		0x60, 0x00, // PUSH1 0x00 retOffset
		0x60, 0x04, // PUSH1 0x04 argsSize
		0x60, 0x00, // PUSH1 0x00 argsOffset
		0x60, 0x00, // PUSH1 0x00 value
		0x73, // PUSH20 ContractAddress
	}
	code = append(code, ContractAddress.Bytes()...)
	code = append(code, 0x5a, 0xf1, 0x50, 0x00) // GAS; CALL; POP; STOP
	return code
}

// TestStakingV3_SeedingInfraSanity cross-checks the slot-seeding helpers against the contract's own
// view of state before any seeded test relies on them.
func TestStakingV3_SeedingInfraSanity(t *testing.T) {
	stateDb := newStakingStateDbV3()
	addr := common.RandomAddress()

	exists, err := DoesDepositorExist(stateDb, addr)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("fresh address should not be a depositor")
	}

	setBoolFlag(stateDb, slotDepositorExists, addr, true)

	exists, err = DoesDepositorExist(stateDb, addr)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("seeding slotDepositorExists should make DoesDepositorExist return true")
	}

	// Uint slot seeding cross-check via getDepositorRewards.
	other := common.RandomAddress()
	setUintSlot(stateDb, slotDepositorRewards, other, params.EtherToWei(big.NewInt(7)))
	rewards, err := GetDepositorRewards(stateDb, other)
	if err != nil {
		t.Fatal(err)
	}
	if rewards.Cmp(params.EtherToWei(big.NewInt(7))) != 0 {
		t.Fatalf("seeded rewards mismatch: got %s", rewards.String())
	}

	// Nested bond slot cross-check via isDepositorBonded.
	dep := common.RandomAddress()
	val := common.RandomAddress()
	setBondFlag(stateDb, dep, val, true)
	bonded, err := IsDepositorBondedV3(stateDb, dep, val)
	if err != nil {
		t.Fatal(err)
	}
	if !bonded {
		t.Fatal("seeding the nested bond slot should make isDepositorBonded return true")
	}
}

// makeRetiredValidatorV3 registers depositor/validator then rotates to newValidator, leaving
// `validator` retired: _validatorExists == false but _validatorEverExisted == true.
func makeRetiredValidatorV3(t *testing.T, stateDb *state.StateDB, depositor, validator, newValidator common.Address) {
	t.Helper()
	registerDepositorV3(t, stateDb, depositor, validator)
	if err := BondDepositorForRotationV3(stateDb, newValidator, depositor); err != nil {
		t.Fatal("rotation bond failed", err)
	}
	if err := ChangeValidatorV3(stateDb, depositor, newValidator); err != nil {
		t.Fatal("changeValidator failed", err)
	}
}

// migratedV2ValidatorV3 deploys v2, makes a normal v2 deposit (validator exists, NOT bonded), then
// upgrades to v3, returning a depositor/validator whose validator has _validatorExists==true and
// _validatorBonded==false (a state v3 code never produces on its own).
func migratedV2ValidatorV3(t *testing.T, stateDb *state.StateDB, depositor, validator common.Address) {
	t.Helper()
	stateDb.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))
	if err := NewDeposit(stateDb, depositor, validator, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal("v2 deposit failed", err)
	}
	UpgradeV2ToV3(stateDb)
}

// --- bondDepositor negatives (each pins its exact require) ---

func TestStakingV3_BondDepositor_SenderIsExistingDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	// msg.sender is an existing depositor -> rejected as a would-be validator.
	err := BondDepositorV3(stateDb, depositor, common.RandomAddress())
	mustRevert(t, err, "Depositor already exists as new validator once")
}

func TestStakingV3_BondDepositor_DepositorArgIsLiveValidator(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	// Trying to bond a live validator address as a depositor.
	err := BondDepositorV3(stateDb, common.RandomAddress(), validator)
	mustRevert(t, err, "Validator already exists as new depositor")
}

func TestStakingV3_BondDepositor_DepositorArgIsRetiredValidator(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	newValidator := common.RandomAddress()
	makeRetiredValidatorV3(t, stateDb, depositor, validator, newValidator)

	// `validator` is now retired (everExisted && !exists); bonding it as a depositor is rejected.
	err := BondDepositorV3(stateDb, common.RandomAddress(), validator)
	mustRevert(t, err, "Validator existed once as new depositor")
}

func TestStakingV3_BondDepositor_SenderIsMigratedV2Validator(t *testing.T) {
	stateDb := newStakingStateDb() // start on v2 bytecode, then upgrade
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	migratedV2ValidatorV3(t, stateDb, depositor, validator)

	// The migrated validator exists but was never bonded (v2 had no bond map); bondDepositor as it
	// passes the bond gate and hits the validator-exists guard.
	err := BondDepositorV3(stateDb, validator, common.RandomAddress())
	mustRevert(t, err, "Validator already exists")
}

func TestStakingV3_BondDepositor_SeededValidatorEverExisted(t *testing.T) {
	stateDb := newStakingStateDbV3()
	sender := common.RandomAddress()
	setBoolFlag(stateDb, slotValidatorEverExisted, sender, true)

	err := BondDepositorV3(stateDb, sender, common.RandomAddress())
	mustRevert(t, err, "Validator existed once")
}

func TestStakingV3_BondDepositor_SeededDepositorEverExistedSender(t *testing.T) {
	stateDb := newStakingStateDbV3()
	sender := common.RandomAddress()
	setBoolFlag(stateDb, slotDepositorEverExisted, sender, true)

	err := BondDepositorV3(stateDb, sender, common.RandomAddress())
	mustRevert(t, err, "Depositor existed once as new validator")
}

func TestStakingV3_BondDepositor_SeededDepositorEverExistedArg(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositorArg := common.RandomAddress()
	setBoolFlag(stateDb, slotDepositorEverExisted, depositorArg, true)

	err := BondDepositorV3(stateDb, common.RandomAddress(), depositorArg)
	mustRevert(t, err, "Depositor existed once")
}

func TestStakingV3_BondDepositor_SeededValidatorEverExistedArg(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositorArg := common.RandomAddress()
	setBoolFlag(stateDb, slotValidatorEverExisted, depositorArg, true)

	err := BondDepositorV3(stateDb, common.RandomAddress(), depositorArg)
	mustRevert(t, err, "Validator existed once as new depositor")
}

// --- bondDepositorForRotation negatives ---

func TestStakingV3_BondRotation_SenderIsExistingDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := BondDepositorForRotationV3(stateDb, depositor, common.RandomAddress())
	mustRevert(t, err, "Depositor already exists as new validator once")
}

func TestStakingV3_BondRotation_SenderIsMigratedV2Validator(t *testing.T) {
	stateDb := newStakingStateDb() // start on v2 bytecode, then upgrade
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	migratedV2ValidatorV3(t, stateDb, depositor, validator)

	err := BondDepositorForRotationV3(stateDb, validator, common.RandomAddress())
	mustRevert(t, err, "Validator already exists")
}

func TestStakingV3_BondRotation_SeededValidatorEverExisted(t *testing.T) {
	stateDb := newStakingStateDbV3()
	sender := common.RandomAddress()
	setBoolFlag(stateDb, slotValidatorEverExisted, sender, true)

	err := BondDepositorForRotationV3(stateDb, sender, common.RandomAddress())
	mustRevert(t, err, "Validator existed once")
}

func TestStakingV3_BondRotation_SeededDepositorEverExisted(t *testing.T) {
	stateDb := newStakingStateDbV3()
	sender := common.RandomAddress()
	setBoolFlag(stateDb, slotDepositorEverExisted, sender, true)

	err := BondDepositorForRotationV3(stateDb, sender, common.RandomAddress())
	mustRevert(t, err, "Depositor existed once as new validator")
}

// --- newDeposit negatives ---

func TestStakingV3_Deposit_BelowMinimum(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	stateDb.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))
	if err := BondDepositorV3(stateDb, validator, depositor); err != nil {
		t.Fatal(err)
	}

	// The minimum-deposit check precedes the bond gate.
	err := NewDepositV3(stateDb, depositor, validator, params.EtherToWei(big.NewInt(1000000)))
	mustRevert(t, err, "Deposit amount below minimum deposit amount")
}

func TestStakingV3_Deposit_SelfAsValidator(t *testing.T) {
	stateDb := newStakingStateDbV3()
	self := common.RandomAddress()
	stateDb.SetBalance(self, params.EtherToWei(big.NewInt(100000000)))

	// Sufficient value to clear the minimum, so the self-address guard fires next.
	err := NewDepositV3(stateDb, self, self, MIN_VALIDATOR_DEPOSIT)
	mustRevert(t, err, "Depositor address cannot be same as Validator address")
}

func TestStakingV3_Deposit_ZeroValidatorReason(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	stateDb.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))

	err := NewDepositV3(stateDb, depositor, ZERO_ADDRESS, MIN_VALIDATOR_DEPOSIT)
	mustRevert(t, err, "Invalid validator")
}

func TestStakingV3_Deposit_UnbondedReason(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	stateDb.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))

	err := NewDepositV3(stateDb, depositor, validator, MIN_VALIDATOR_DEPOSIT)
	mustRevert(t, err, "Validator has not bonded depositor")
}

func TestStakingV3_Deposit_SecondDepositReason(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator1 := common.RandomAddress()
	validator2 := common.RandomAddress()
	stateDb.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))

	// Two validators bond the same depositor BEFORE it exists, so the second bond is recorded;
	// after the first deposit the depositor exists and the second deposit hits the depositor guard.
	if err := BondDepositorV3(stateDb, validator1, depositor); err != nil {
		t.Fatal(err)
	}
	if err := BondDepositorV3(stateDb, validator2, depositor); err != nil {
		t.Fatal(err)
	}
	if err := NewDepositV3(stateDb, depositor, validator1, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal(err)
	}
	err := NewDepositV3(stateDb, depositor, validator2, MIN_VALIDATOR_DEPOSIT)
	mustRevert(t, err, "Depositor already exists")
}

// seededDepositRevert opens the bond gate by seeding _depositorBond[D][V], seeds one conflicting
// flag, and asserts the corresponding downstream require fires.
func seededDepositRevert(t *testing.T, seed func(stateDb *state.StateDB, d, v common.Address), want string) {
	t.Helper()
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	stateDb.SetBalance(depositor, params.EtherToWei(big.NewInt(100000000)))
	setBondFlag(stateDb, depositor, validator, true)
	seed(stateDb, depositor, validator)

	err := NewDepositV3(stateDb, depositor, validator, MIN_VALIDATOR_DEPOSIT)
	mustRevert(t, err, want)
}

func TestStakingV3_Deposit_SeededDownstreamDuplicates(t *testing.T) {
	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotValidatorExists, v, true)
	}, "Validator already exists")

	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotValidatorEverExisted, v, true)
	}, "Validator existed once")

	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotValidatorExists, d, true)
	}, "Validator already exists as new depositor")

	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotValidatorEverExisted, d, true)
	}, "Validator existed once as new depositor")

	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotDepositorEverExisted, d, true)
	}, "Depositor existed once")

	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotDepositorExists, v, true)
	}, "Depositor already exists as new validator once")

	seededDepositRevert(t, func(s *state.StateDB, d, v common.Address) {
		setBoolFlag(s, slotDepositorEverExisted, v, true)
	}, "Depositor existed once as new validator")
}

// --- pauseValidation / resumeValidation ---

func TestStakingV3_Pause_NonDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	err := PauseValidation(stateDb, common.RandomAddress())
	mustRevert(t, err, "Depositor does not exist")
}

func TestStakingV3_Pause_AlreadyPaused(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	if err := PauseValidation(stateDb, depositor); err != nil {
		t.Fatal("first pause should succeed", err)
	}
	err := PauseValidation(stateDb, depositor)
	mustRevert(t, err, "Validation is already paused")
}

func TestStakingV3_Resume_NonDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	err := ResumeValidation(stateDb, common.RandomAddress())
	mustRevert(t, err, "Depositor does not exist")
}

func TestStakingV3_Resume_NotPaused(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := ResumeValidation(stateDb, depositor)
	mustRevert(t, err, "Validation is not paused")
}

func TestStakingV3_PauseResume_Positive(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	if err := PauseValidation(stateDb, depositor); err != nil {
		t.Fatal(err)
	}
	paused, err := IsValidationPaused(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if !paused {
		t.Fatal("validation should be paused")
	}

	if err := ResumeValidation(stateDb, depositor); err != nil {
		t.Fatal(err)
	}
	paused, err = IsValidationPaused(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if paused {
		t.Fatal("validation should be resumed")
	}
}

// --- addDepositorReward / addDepositorSlashing ---

func TestStakingV3_Reward_NonVMReverts(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := AddDepositorRewardV3(stateDb, depositor, depositor, params.EtherToWei(big.NewInt(1)))
	mustRevert(t, err, "Only VM calls are allowed")
}

func TestStakingV3_Slashing_NonVMReverts(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := AddDepositorSlashingV3(stateDb, depositor, depositor, params.EtherToWei(big.NewInt(1)))
	mustRevert(t, err, "Only VM calls are allowed")
}

func TestStakingV3_RewardAndSlashing_VMPositiveState(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	reward := params.EtherToWei(big.NewInt(25))
	if err := AddDepositorRewardV3(stateDb, ZERO_ADDRESS, depositor, reward); err != nil {
		t.Fatal(err)
	}
	gotReward, err := GetDepositorRewards(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if gotReward.Cmp(reward) != 0 {
		t.Fatalf("rewards mismatch: want %s got %s", reward, gotReward)
	}

	slash := params.EtherToWei(big.NewInt(10))
	if err := AddDepositorSlashingV3(stateDb, ZERO_ADDRESS, depositor, slash); err != nil {
		t.Fatal(err)
	}
	gotSlash, err := GetDepositorSlashings(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if gotSlash.Cmp(slash) != 0 {
		t.Fatalf("slashings mismatch: want %s got %s", slash, gotSlash)
	}
}

// --- changeValidator negatives ---

func TestStakingV3_ChangeValidator_ToLiveValidator(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	depositor2 := common.RandomAddress()
	validator2 := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor2, validator2)

	err := ChangeValidatorV3(stateDb, depositor, validator2)
	mustRevert(t, err, "Validator already exists")
}

func TestStakingV3_ChangeValidator_ToCurrentDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	depositor2 := common.RandomAddress()
	validator2 := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor2, validator2)

	err := ChangeValidatorV3(stateDb, depositor, depositor2)
	mustRevert(t, err, "Validator is a depositor")
}

func TestStakingV3_ChangeValidator_ToRetiredValidator(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	newValidator := common.RandomAddress()
	makeRetiredValidatorV3(t, stateDb, depositor, validator, newValidator)

	// `validator` is retired (everExisted && !exists); rotating back to it is rejected.
	err := ChangeValidatorV3(stateDb, depositor, validator)
	mustRevert(t, err, "Validator already existed")
}

func TestStakingV3_ChangeValidator_SeededDepositorEverExisted(t *testing.T) {
	stateDb := newStakingStateDbV3()
	newValidator := common.RandomAddress()
	setBoolFlag(stateDb, slotDepositorEverExisted, newValidator, true)

	err := ChangeValidatorV3(stateDb, common.RandomAddress(), newValidator)
	mustRevert(t, err, "Depositor already existed")
}

func TestStakingV3_ChangeValidator_ToZero(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := ChangeValidatorV3(stateDb, depositor, ZERO_ADDRESS)
	mustRevert(t, err, "Invalid validator")
}

func TestStakingV3_ChangeValidator_NonDepositorCaller(t *testing.T) {
	stateDb := newStakingStateDbV3()
	err := ChangeValidatorV3(stateDb, common.RandomAddress(), common.RandomAddress())
	mustRevert(t, err, "Depositor does not exist")
}

func TestStakingV3_ChangeValidator_UnbondedReason(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := ChangeValidatorV3(stateDb, depositor, common.RandomAddress())
	mustRevert(t, err, "New validator has not bonded depositor")
}

func TestStakingV3_ChangeValidator_SeededWithdrawalPending(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	newValidator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	if err := BondDepositorForRotationV3(stateDb, newValidator, depositor); err != nil {
		t.Fatal(err)
	}
	// Simulate a v1-migrated legacy full-withdrawal request still pending.
	setUintSlot(stateDb, slotDepositorWithdrawalRequests, depositor, big.NewInt(123))

	err := ChangeValidatorV3(stateDb, depositor, newValidator)
	mustRevert(t, err, "Withdrawal is pending")
}

// --- increaseDeposit ---

func TestStakingV3_IncreaseDeposit_NonDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	addr := common.RandomAddress()
	stateDb.SetBalance(addr, params.EtherToWei(big.NewInt(100)))
	err := IncreaseDepositV3(stateDb, addr, params.EtherToWei(big.NewInt(1)))
	mustRevert(t, err, "Depositor does not exist")
}

func TestStakingV3_IncreaseDeposit_ZeroValue(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := IncreaseDepositV3(stateDb, depositor, big.NewInt(0))
	mustRevert(t, err, "Deposit amount is zero")
}

func TestStakingV3_IncreaseDeposit_SeededWithdrawalPending(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)
	setUintSlot(stateDb, slotDepositorWithdrawalRequests, depositor, big.NewInt(1))

	err := IncreaseDepositV3(stateDb, depositor, params.EtherToWei(big.NewInt(1)))
	mustRevert(t, err, "Depositor withdrawal request exists")
}

func TestStakingV3_IncreaseDeposit_Positive(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	add := params.EtherToWei(big.NewInt(7))
	if err := IncreaseDepositV3(stateDb, depositor, add); err != nil {
		t.Fatal(err)
	}
	bal, err := GetBalanceOfDepositorV3(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Int).Add(MIN_VALIDATOR_DEPOSIT, add)
	if bal.Cmp(want) != 0 {
		t.Fatalf("balance after increase mismatch: want %s got %s", want, bal)
	}
}

// --- initiatePartialWithdrawal ---

func TestStakingV3_InitiatePartial_NonDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	err := InitiatePartialWithdrawalV3(stateDb, common.RandomAddress(), params.EtherToWei(big.NewInt(1)), 10)
	mustRevert(t, err, "Depositor does not exist")
}

func TestStakingV3_InitiatePartial_DoubleInitiate(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	amount := params.EtherToWei(big.NewInt(1000000))
	if err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 10); err != nil {
		t.Fatal("first initiate should succeed", err)
	}
	err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 11)
	mustRevert(t, err, "Depositor partial withdrawal request exists")
}

func TestStakingV3_InitiatePartial_NetBalanceLow(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	// Amount exceeds the net balance, so the net-balance guard fires before the min-remaining one.
	tooMuch := new(big.Int).Add(MIN_VALIDATOR_DEPOSIT, params.EtherToWei(big.NewInt(1)))
	err := InitiatePartialWithdrawalV3(stateDb, depositor, tooMuch, 10)
	mustRevert(t, err, "Depositor net balance is low")
}

func TestStakingV3_InitiatePartial_SeededWithdrawalPending(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)
	setUintSlot(stateDb, slotDepositorWithdrawalRequests, depositor, big.NewInt(1))

	err := InitiatePartialWithdrawalV3(stateDb, depositor, params.EtherToWei(big.NewInt(1000000)), 10)
	mustRevert(t, err, "Depositor withdrawal request exists")
}

func TestStakingV3_InitiatePartial_RewardsBranch(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	reward := params.EtherToWei(big.NewInt(2000))
	if err := AddDepositorRewardV3(stateDb, ZERO_ADDRESS, depositor, reward); err != nil {
		t.Fatal(err)
	}

	// debit (1000) <= rewards (2000): the withdrawal is taken entirely from rewards; principal stays.
	amount := params.EtherToWei(big.NewInt(1000))
	if err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 10); err != nil {
		t.Fatal(err)
	}

	gotReward, err := GetDepositorRewards(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	wantReward := new(big.Int).Sub(reward, amount)
	if gotReward.Cmp(wantReward) != 0 {
		t.Fatalf("rewards should be reduced by the withdrawal: want %s got %s", wantReward, gotReward)
	}

	bal, err := GetBalanceOfDepositorV3(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if bal.Cmp(MIN_VALIDATOR_DEPOSIT) != 0 {
		t.Fatalf("principal balance should be unchanged in the rewards branch: got %s", bal)
	}

	// The queued amount is recorded.
	details, err := GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if details.WithdrawalAmount.Cmp(amount) != 0 {
		t.Fatalf("queued withdrawal amount mismatch: want %s got %s", amount, details.WithdrawalAmount)
	}
}

// --- completePartialWithdrawal ---

func TestStakingV3_CompletePartial_NonDepositor(t *testing.T) {
	stateDb := newStakingStateDbV3()
	err := CompletePartialWithdrawalV3(stateDb, common.RandomAddress(), 40000)
	mustRevert(t, err, "Depositor does not exist")
}

func TestStakingV3_CompletePartial_NoActiveRequest(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	err := CompletePartialWithdrawalV3(stateDb, depositor, 40000)
	mustRevert(t, err, "Depositor partial withdrawal request does not exist")
}

func TestStakingV3_CompletePartial_BeforeCutoff(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	if err := InitiatePartialWithdrawalV3(stateDb, depositor, params.EtherToWei(big.NewInt(1000000)), 10); err != nil {
		t.Fatal(err)
	}
	err := CompletePartialWithdrawalV3(stateDb, depositor, 100)
	mustRevert(t, err, "Depositor partial withdrawal request cutoff block not reached")
}

func TestStakingV3_CompletePartial_SeededWithdrawalPending(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	if err := InitiatePartialWithdrawalV3(stateDb, depositor, params.EtherToWei(big.NewInt(1000000)), 10); err != nil {
		t.Fatal(err)
	}
	setUintSlot(stateDb, slotDepositorWithdrawalRequests, depositor, big.NewInt(1))

	err := CompletePartialWithdrawalV3(stateDb, depositor, 40000)
	mustRevert(t, err, "Depositor withdrawal request exists")
}

func TestStakingV3_CompletePartial_Positive(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	amount := params.EtherToWei(big.NewInt(1000000))
	if err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 10); err != nil {
		t.Fatal(err)
	}

	preEOA := stateDb.GetBalance(depositor)
	if err := CompletePartialWithdrawalV3(stateDb, depositor, 40000); err != nil {
		t.Fatal("complete after the cutoff should succeed", err)
	}

	postEOA := stateDb.GetBalance(depositor)
	gain := new(big.Int).Sub(postEOA, preEOA)
	if gain.Cmp(amount) != 0 {
		t.Fatalf("depositor should receive the withdrawn amount: want %s got %s", amount, gain)
	}

	wb, err := GetWithdrawalBlock(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if wb.Sign() != 0 {
		t.Fatalf("withdrawal mappings should be cleared after completion, got block %s", wb)
	}
}

// --- setNilBlock / resetNilBlock ---

func TestStakingV3_SetNilBlock_NonVMReverts(t *testing.T) {
	stateDb := newStakingStateDbV3()
	validator := common.RandomAddress()
	err := SetNilBlock(stateDb, common.RandomAddress(), validator)
	mustRevert(t, err, "Only VM calls are allowed")
}

func TestStakingV3_ResetNilBlock_NonVMReverts(t *testing.T) {
	stateDb := newStakingStateDbV3()
	validator := common.RandomAddress()
	err := ResetNilBlock(stateDb, common.RandomAddress(), validator)
	mustRevert(t, err, "Only VM calls are allowed")
}

func TestStakingV3_NilBlock_VMPositive(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	if err := SetNilBlock(stateDb, ZERO_ADDRESS, validator); err != nil {
		t.Fatal(err)
	}
	details, err := GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if details.LastNiLBlock.Sign() == 0 {
		t.Fatal("LastNiLBlock should be set after setNilBlock")
	}
	if details.NilBlockCount.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("NilBlockCount should be 1, got %s", details.NilBlockCount)
	}

	if err := SetNilBlock(stateDb, ZERO_ADDRESS, validator); err != nil {
		t.Fatal(err)
	}
	details, err = GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if details.NilBlockCount.Cmp(big.NewInt(2)) != 0 {
		t.Fatalf("NilBlockCount should be 2 after a second set, got %s", details.NilBlockCount)
	}

	if err := ResetNilBlock(stateDb, ZERO_ADDRESS, validator); err != nil {
		t.Fatal(err)
	}
	details, err = GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if details.LastNiLBlock.Sign() != 0 || details.NilBlockCount.Sign() != 0 {
		t.Fatalf("reset should clear nil-block state, got last=%s count=%s", details.LastNiLBlock, details.NilBlockCount)
	}
}

// --- getStakingDetails ---

func TestStakingV3_GetStakingDetails_NonExistentValidator(t *testing.T) {
	stateDb := newStakingStateDbV3()
	_, err := GetStakingDetails(stateDb, common.RandomAddress())
	mustRevert(t, err, "Validator does not exist")
}

func TestStakingV3_GetStakingDetails_Positive(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	d, err := GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Depositor.IsEqualTo(depositor) || !d.Validator.IsEqualTo(validator) {
		t.Fatal("depositor/validator mismatch")
	}
	if d.Balance.Cmp(MIN_VALIDATOR_DEPOSIT) != 0 || d.NetBalance.Cmp(MIN_VALIDATOR_DEPOSIT) != 0 {
		t.Fatal("balance/netBalance should equal the deposit")
	}
	if d.BlockRewards.Sign() != 0 || d.Slashings.Sign() != 0 {
		t.Fatal("rewards/slashings should be zero")
	}
	if d.IsValidationPaused {
		t.Fatal("validation should not be paused")
	}
	if d.WithdrawalBlock.Sign() != 0 || d.WithdrawalAmount.Sign() != 0 {
		t.Fatal("withdrawal fields should be zero")
	}
	if d.LastNiLBlock.Sign() != 0 || d.NilBlockCount.Sign() != 0 {
		t.Fatal("nil-block fields should be zero")
	}
}

func TestStakingV3_GetStakingDetails_PartialWithdrawalBranch(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	amount := params.EtherToWei(big.NewInt(1000000))
	if err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 10); err != nil {
		t.Fatal(err)
	}

	d, err := GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	// Stored block is header number (10+1); the reported block adds WITHDRAWAL_BLOCK_DELAY (32000).
	wantBlock := big.NewInt(11 + 32000)
	if d.WithdrawalBlock.Cmp(wantBlock) != 0 {
		t.Fatalf("withdrawal block mismatch: want %s got %s", wantBlock, d.WithdrawalBlock)
	}
	if d.WithdrawalAmount.Cmp(amount) != 0 {
		t.Fatalf("withdrawal amount mismatch: want %s got %s", amount, d.WithdrawalAmount)
	}
}

func TestStakingV3_GetStakingDetails_SeededLegacyBranch(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	// Simulate a v1-migrated legacy full-withdrawal request (no partial request present).
	setUintSlot(stateDb, slotDepositorWithdrawalRequests, depositor, big.NewInt(777))

	d, err := GetStakingDetails(stateDb, validator)
	if err != nil {
		t.Fatal(err)
	}
	if d.WithdrawalBlock.Cmp(big.NewInt(777)) != 0 {
		t.Fatalf("legacy withdrawal block should be reported as-is: got %s", d.WithdrawalBlock)
	}
	// A pending legacy request forces the reported net balance (and queued amount) to zero.
	if d.NetBalance.Sign() != 0 || d.WithdrawalAmount.Sign() != 0 {
		t.Fatalf("legacy pending request should report zero net/withdrawal amount, got net=%s amt=%s", d.NetBalance, d.WithdrawalAmount)
	}
}

// --- getNetBalanceOfDepositor branches ---

func TestStakingV3_NetBalance_NonExistentIsZero(t *testing.T) {
	stateDb := newStakingStateDbV3()
	net, err := GetNetBalanceOfDepositorV3(stateDb, common.RandomAddress())
	if err != nil {
		t.Fatal(err)
	}
	if net.Sign() != 0 {
		t.Fatalf("net balance of a non-depositor should be 0, got %s", net)
	}
}

func TestStakingV3_NetBalance_SlashingsCoverBalanceIsZero(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	// Slash the entire principal so balance <= slashings -> net 0.
	if err := AddDepositorSlashingV3(stateDb, ZERO_ADDRESS, depositor, MIN_VALIDATOR_DEPOSIT); err != nil {
		t.Fatal(err)
	}
	net, err := GetNetBalanceOfDepositorV3(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if net.Sign() != 0 {
		t.Fatalf("net balance should be 0 when slashings cover balance, got %s", net)
	}
}

func TestStakingV3_NetBalance_NormalIsBalanceMinusSlashings(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	slash := params.EtherToWei(big.NewInt(10))
	if err := AddDepositorSlashingV3(stateDb, ZERO_ADDRESS, depositor, slash); err != nil {
		t.Fatal(err)
	}
	net, err := GetNetBalanceOfDepositorV3(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Int).Sub(MIN_VALIDATOR_DEPOSIT, slash)
	if net.Cmp(want) != 0 {
		t.Fatalf("net balance mismatch: want %s got %s", want, net)
	}
}

func TestStakingV3_NetBalance_SeededLegacyWithdrawalIsZero(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	setUintSlot(stateDb, slotDepositorWithdrawalRequests, depositor, big.NewInt(1))

	net, err := GetNetBalanceOfDepositorV3(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if net.Sign() != 0 {
		t.Fatalf("net balance should be 0 with a pending legacy request, got %s", net)
	}

	wb, err := GetWithdrawalBlock(stateDb, depositor)
	if err != nil {
		t.Fatal(err)
	}
	if wb.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("getWithdrawalBlock should report the legacy block, got %s", wb)
	}
}

// --- exotic require(success) / reentrancy paths ---

func TestStakingV3_Slashing_TransferToZeroFails(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	// Force the slashing burn-to-address(0) value-call to fail by planting reverting code there.
	plantRevertingCode(stateDb, ZERO_ADDRESS)

	err := AddDepositorSlashingV3(stateDb, ZERO_ADDRESS, depositor, params.EtherToWei(big.NewInt(10)))
	mustRevert(t, err, "transfer to zeroAddress failed")
}

func TestStakingV3_CompletePartial_RevertingRecipientFails(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	amount := params.EtherToWei(big.NewInt(1000000))
	if err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 10); err != nil {
		t.Fatal(err)
	}

	// The recipient rejects the funds, so the withdrawal value-call fails.
	plantRevertingCode(stateDb, depositor)

	err := CompletePartialWithdrawalV3(stateDb, depositor, 40000)
	mustRevert(t, err, "Withdraw failed")
}

// TestStakingV3_CompletePartial_ReentrancyNoDoubleSpend installs an attacker that re-enters
// completePartialWithdrawal during the withdrawal value-call. The reentrancy guard (plus
// checks-effects-interactions ordering) blocks the inner call, so the attacker is paid exactly
// once and the request is consumed. Note: the inner revert is swallowed by the low-level call, so
// the literal "ReentrancyGuard: reentrant call" string cannot surface here; the protection is
// verified behaviorally via the single, non-duplicated payout below.
func TestStakingV3_CompletePartial_ReentrancyNoDoubleSpend(t *testing.T) {
	stateDb := newStakingStateDbV3()
	depositor := common.RandomAddress()
	validator := common.RandomAddress()
	registerDepositorV3(t, stateDb, depositor, validator)

	amount := params.EtherToWei(big.NewInt(1000000))
	if err := InitiatePartialWithdrawalV3(stateDb, depositor, amount, 10); err != nil {
		t.Fatal(err)
	}

	stateDb.SetCode(depositor, reentrancyAttackerCode())
	stateDb.Finalise(true)

	preEOA := stateDb.GetBalance(depositor)
	if err := CompletePartialWithdrawalV3(stateDb, depositor, 40000); err != nil {
		t.Fatal("withdrawal should complete once despite the re-entrant callback", err)
	}

	gain := new(big.Int).Sub(stateDb.GetBalance(depositor), preEOA)
	if gain.Cmp(amount) != 0 {
		t.Fatalf("attacker must be paid exactly once: want %s got %s", amount, gain)
	}

	// The request is consumed; a second completion finds nothing (no double spend).
	err := CompletePartialWithdrawalV3(stateDb, depositor, 40001)
	mustRevert(t, err, "Depositor partial withdrawal request does not exist")
}

// Coverage notes: the two outcomes below are intentionally NOT asserted as distinct revert
// strings, because they cannot be surfaced as such through any reachable call path. Everything
// else in StakingContract.sol is exercised above, including the cross-version guards
// (*EverExisted and the v1-only _depositorWithdrawalRequests branches) reached via storage seeding.
//
//  1. nonReentrant -> require(_locked == false, "ReentrancyGuard: reentrant call").
//     The only re-entrant path is the withdrawal value-call to the depositor. A re-entrant inner
//     call reverts on this require, but that revert is swallowed by the low-level `.call`, which
//     returns success=false and re-surfaces as the outer "Withdraw failed" (or, if the attacker
//     swallows the failure, as a single non-duplicated payout). Both behaviors are covered by
//     TestStakingV3_CompletePartial_ReentrancyNoDoubleSpend and
//     TestStakingV3_CompletePartial_RevertingRecipientFails; the positive lock-reset is covered by
//     TestStakingV3_ReentrancyLockResets. The literal guard string is therefore unobservable here.
//
//  2. SafeMath assert(...) overflow/underflow (mul/sub/add/ceil).
//     Each call site is protected by a preceding require with valid inputs - e.g.
//     initiatePartialWithdrawal does require(netBalance >= amount) before netBalance.sub(amount),
//     and getNetBalanceOfDepositor checks balance <= slashings before subtracting. Reaching an
//     assert would require seeding balances near 2^256, which would test the SafeMath library
//     itself rather than this contract's logic, so it is documented rather than executed.
