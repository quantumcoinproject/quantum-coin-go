package tracetest

// The call tracer is what block explorers read to learn where coins moved
// inside a transaction. Every value-carrying opcode must therefore produce a
// frame, including the ones whose callee has no code: a plain account.
//
// Solidity 0.7.6 offers three ways to send coins to an address and they all
// compile to CALL, differing only in the gas they forward and how they report
// failure:
//
//	to.transfer(amount)         CALL with the 2300-gas stipend, reverts on failure
//	to.send(amount)             CALL with the 2300-gas stipend, returns bool
//	to.call{value: amount}("")  CALL forwarding the remaining gas
//
// and two more that move a balance without being a call at all:
//
//	selfdestruct(to)            SELFDESTRUCT, moves the whole balance
//	new C{value: amount}()      CREATE / CREATE2 with an endowment
//
// These tests assert a frame appears for each, carrying the right value. A
// missing frame is not a cosmetic problem: an indexer that reconstructs
// balances from traces silently loses the credit, and the recipient's balance
// drifts from consensus.

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/eth/tracers"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/tests"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
)

// recipientByte is the low byte of the plain account that receives the coins.
// It is funded with nothing and has no code, which is the case a tracer is most
// likely to skip.
const recipientByte = 0xff

// traceValueTransfer runs one contract body as a transaction and returns the
// resulting call trace.
func traceValueTransfer(t *testing.T, code []byte, contractBalance *big.Int) *callTrace {
	t.Helper()

	contract := common.BytesToAddress([]byte{1})
	privkey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer := types.NewLondonSignerDefaultChain()
	tx, err := types.SignNewTx(privkey, signer, &types.DefaultFeeTx{
		ChainID: big.NewInt(123123),
		Gas:     200000,
		To:      &contract,
		// verifyFields requires the default tier; without it the gas price
		// does not match GetDefaultGasPrice and signing is refused.
		MaxGasTier: types.GAS_TIER_DEFAULT,
	})
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	origin, _ := signer.Sender(tx)

	txContext := vm.TxContext{Origin: origin, GasPrice: types.GetDefaultGasPrice()}
	context := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    common.Address{},
		BlockNumber: new(big.Int).SetUint64(8000000),
		Time:        new(big.Int).SetUint64(5),
		Difficulty:  big.NewInt(0x30000),
		GasLimit:    uint64(6000000),
	}
	alloc := core.GenesisAlloc{
		contract: core.GenesisAccount{Nonce: 1, Code: code, Balance: contractBalance},
		// The default gas price is ~0.0476 Q per gas, so the origin needs
		// room for gas * price on top of any value sent.
		origin: core.GenesisAccount{Nonce: 0, Balance: new(big.Int).Mul(big.NewInt(1e18), big.NewInt(100000))},
	}
	_, statedb := tests.MakePreState(rawdb.NewMemoryDatabase(), alloc, false)

	tracer, err := tracers.New("callTracer", nil)
	if err != nil {
		t.Fatalf("new call tracer: %v", err)
	}
	evm := vm.NewEVM(context, txContext, statedb, params.MainnetChainConfig,
		vm.Config{Debug: true, Tracer: tracer})
	msg, err := tx.AsMessage(signer)
	if err != nil {
		t.Fatalf("as message: %v", err)
	}
	st := core.NewStateTransition(evm, msg, new(core.GasPool).AddGas(tx.Gas()))
	if _, err = st.TransitionDb(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	res, err := tracer.GetResult()
	if err != nil {
		t.Fatalf("trace result: %v", err)
	}
	trace := new(callTrace)
	if err := json.Unmarshal(res, trace); err != nil {
		t.Fatalf("unmarshal trace: %v", err)
	}
	return trace
}

// callWithValue builds a body performing CALL with the given value and gas.
// CALL pops gas, addr, value, argsOffset, argsLength, retOffset, retLength, so
// the pushes run in the reverse of that order.
func callWithValue(value byte, gasHi, gasLo byte) []byte {
	return []byte{
		byte(vm.PUSH1), 0x00, // retLength
		byte(vm.PUSH1), 0x00, // retOffset
		byte(vm.PUSH1), 0x00, // argsLength
		byte(vm.PUSH1), 0x00, // argsOffset
		byte(vm.PUSH1), value, // value
		byte(vm.PUSH1), recipientByte, // to
		byte(vm.PUSH2), gasHi, gasLo, // gas
		byte(vm.CALL),
		byte(vm.STOP),
	}
}

// assertOneValueFrame checks the trace has exactly one sub-frame, of the given
// type, moving the given value to the recipient.
func assertOneValueFrame(t *testing.T, trace *callTrace, wantType string, wantValue *big.Int) {
	t.Helper()
	if len(trace.Calls) != 1 {
		t.Fatalf("%s produced %d sub-frames, want exactly 1; the value transfer is invisible to the tracer",
			wantType, len(trace.Calls))
	}
	got := trace.Calls[0]
	if got.Type != wantType {
		t.Errorf("frame type = %q, want %q", got.Type, wantType)
	}
	if got.Value == nil || got.Value.ToInt().Cmp(wantValue) != 0 {
		t.Errorf("frame value = %v, want %v", got.Value, wantValue)
	}
	wantTo := common.BytesToAddress([]byte{recipientByte})
	if got.To != wantTo {
		t.Errorf("frame to = %v, want %v", got.To, wantTo)
	}
}

// to.transfer(x) and to.send(x) both emit CALL with the 2300-gas stipend and an
// empty payload. The callee is a plain account, so no code runs inside the
// frame -- the frame must still be reported, because the coins did move.
func TestCallTracerStipendTransferToPlainAccount(t *testing.T) {
	const stipend = 2300
	trace := traceValueTransfer(t, callWithValue(0x64, 0x08, 0xfc), big.NewInt(1000))
	assertOneValueFrame(t, trace, "CALL", big.NewInt(0x64))
	_ = stipend
}

// to.call{value: x}("") forwards the remaining gas rather than a stipend.
func TestCallTracerCallWithValueToPlainAccount(t *testing.T) {
	// 0xffff gas is more than the stipend and less than the block limit.
	trace := traceValueTransfer(t, callWithValue(0x2a, 0xff, 0xff), big.NewInt(1000))
	assertOneValueFrame(t, trace, "CALL", big.NewInt(0x2a))
}

// A zero-value call still enters a frame; the explorer relies on it to show
// the call happened even though no coins moved.
func TestCallTracerZeroValueCallStillFramed(t *testing.T) {
	trace := traceValueTransfer(t, callWithValue(0x00, 0xff, 0xff), big.NewInt(1000))
	assertOneValueFrame(t, trace, "CALL", big.NewInt(0))
}

// selfdestruct(to) moves the contract's whole balance without a call.
func TestCallTracerSelfdestructFramed(t *testing.T) {
	code := []byte{
		byte(vm.PUSH1), recipientByte,
		byte(vm.SELFDESTRUCT),
	}
	trace := traceValueTransfer(t, code, big.NewInt(777))
	if len(trace.Calls) != 1 {
		t.Fatalf("selfdestruct produced %d sub-frames, want 1", len(trace.Calls))
	}
	if got := trace.Calls[0].Type; got != "SELFDESTRUCT" {
		t.Errorf("frame type = %q, want SELFDESTRUCT", got)
	}
	if v := trace.Calls[0].Value; v == nil || v.ToInt().Cmp(big.NewInt(777)) != 0 {
		t.Errorf("selfdestruct value = %v, want 777 (the whole balance)", v)
	}
}

// new C{value: x}() endows the created contract. CREATE pops value, offset,
// size, so the pushes are size, offset, value.
func TestCallTracerCreateWithValueFramed(t *testing.T) {
	code := []byte{
		byte(vm.PUSH1), 0x00, // size (empty init code)
		byte(vm.PUSH1), 0x00, // offset
		byte(vm.PUSH1), 0x39, // value
		byte(vm.CREATE),
		byte(vm.STOP),
	}
	trace := traceValueTransfer(t, code, big.NewInt(1000))
	if len(trace.Calls) != 1 {
		t.Fatalf("create produced %d sub-frames, want 1", len(trace.Calls))
	}
	if got := trace.Calls[0].Type; got != "CREATE" {
		t.Errorf("frame type = %q, want CREATE", got)
	}
	if v := trace.Calls[0].Value; v == nil || v.ToInt().Cmp(big.NewInt(0x39)) != 0 {
		t.Errorf("create value = %v, want 0x39", v)
	}
}
