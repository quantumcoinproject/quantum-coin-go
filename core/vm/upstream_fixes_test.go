// Copyright 2014 The go-ethereum Authors
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

package vm

import (
	"bytes"
	"math"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// setUpstreamFixesGate points the UpstreamConsensusFixesV1 gate at the given block
// and restores the previous value when the test ends.
func setUpstreamFixesGate(t *testing.T, block uint64) {
	t.Helper()
	old := defaults.DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock
	t.Cleanup(func() {
		defaults.DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock = old
	})
	defaults.DefaultConfig.PosConfig.UpstreamConsensusFixesV1StartBlock = block
}

func newTestBlockContext(blockNumber int64) BlockContext {
	return BlockContext{
		CanTransfer: func(StateDB, common.Address, *big.Int) bool { return true },
		Transfer:    func(StateDB, common.Address, common.Address, *big.Int) {},
		BlockNumber: big.NewInt(blockNumber),
	}
}

// TestUBF001_IdentityPrecompileOverlappingReturnData covers CVE-2021-39137 /
// upstream 1d9957319: the identity precompile (0x04) returns its input slice by
// reference, so writing the CALL result into a return region that overlaps the
// input region mutates the result in place. The return data captured afterwards
// for RETURNDATACOPY then reflects the rewritten memory instead of what the
// callee actually produced.
func TestUBF001_IdentityPrecompileOverlappingReturnData(t *testing.T) {
	setUpstreamFixesGate(t, 100)

	wordA := bytes.Repeat([]byte{0xaa}, 32)
	wordB := bytes.Repeat([]byte{0xbb}, 32)

	// mem[0:32] = A, mem[32:64] = B, then
	// CALL(0xffff, 0x04, 0, in=[0,64), ret=[32,64)) — the return region overlaps
	// the second half of the input region. Finally copy the whole 64-byte return
	// data to mem[128:192) and return it.
	var code []byte
	code = append(code, byte(PUSH32))
	code = append(code, wordA...)
	code = append(code, byte(PUSH1), 0x00, byte(MSTORE))
	code = append(code, byte(PUSH32))
	code = append(code, wordB...)
	code = append(code, byte(PUSH1), 0x20, byte(MSTORE))
	code = append(code,
		byte(PUSH1), 0x20, // retSize
		byte(PUSH1), 0x20, // retOffset
		byte(PUSH1), 0x40, // inSize
		byte(PUSH1), 0x00, // inOffset
		byte(PUSH1), 0x00, // value
		byte(PUSH1), 0x04, // address of the identity precompile
		byte(PUSH2), 0xff, 0xff, // gas
		byte(CALL),
		byte(POP),
		byte(PUSH1), 0x40, // length
		byte(PUSH1), 0x00, // data offset
		byte(PUSH1), 0x80, // memory offset
		byte(RETURNDATACOPY),
		byte(PUSH1), 0x40, // length
		byte(PUSH1), 0x80, // offset
		byte(RETURN),
	)

	tests := []struct {
		name  string
		block int64
		want  []byte
	}{
		// Before the fork the return data aliases memory, so the second word has
		// already been overwritten with the first by the time it is captured.
		{"pre-fork keeps the aliased return data", 99, append(append([]byte{}, wordA...), wordA...)},
		// After the fork the return data is snapshotted before the memory write.
		{"post-fork returns the callee output", 100, append(append([]byte{}, wordA...), wordB...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address := common.BytesToAddress([]byte("caller"))

			statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
			statedb.CreateAccount(address)
			statedb.SetCode(address, code)
			statedb.Finalise(true)

			vmenv := NewEVM(newTestBlockContext(tt.block), TxContext{}, statedb, params.AllEthashProtocolChanges, Config{})
			ret, _, err := vmenv.Call(AccountRef(common.Address{}), address, nil, 10000000, new(big.Int))
			if err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if !bytes.Equal(ret, tt.want) {
				t.Errorf("return data mismatch:\nhave %x\nwant %x", ret, tt.want)
			}
		})
	}
}

// TestUBF003_EIP2681CreateNonceOverflow covers upstream f32feeb26: CREATE must
// fail rather than wrap the creator's nonce past 2^64-1.
func TestUBF003_EIP2681CreateNonceOverflow(t *testing.T) {
	setUpstreamFixesGate(t, 100)

	tests := []struct {
		name    string
		block   int64
		wantErr error
	}{
		{"pre-fork wraps the nonce", 99, nil},
		{"post-fork rejects the creation", 100, ErrNonceUintOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := common.BytesToAddress([]byte("creator"))

			statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
			statedb.CreateAccount(caller)
			statedb.SetNonce(caller, math.MaxUint64)
			statedb.Finalise(true)

			vmenv := NewEVM(newTestBlockContext(tt.block), TxContext{}, statedb, params.AllEthashProtocolChanges, Config{})
			_, _, _, err := vmenv.Create(AccountRef(caller), []byte{byte(STOP)}, 1000000, new(big.Int))
			if err != tt.wantErr {
				t.Fatalf("error mismatch: have %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				// The un-gated path is unchanged: the nonce silently wraps to zero.
				if got := statedb.GetNonce(caller); got != 0 {
					t.Errorf("nonce mismatch: have %d, want 0", got)
				}
			} else if got := statedb.GetNonce(caller); got != uint64(math.MaxUint64) {
				t.Errorf("nonce should be untouched: have %d", got)
			}
		})
	}
}

// TestUBF007_JumpTableDeepCopyOnExtraEips covers upstream 7dc5e785a (#26137) and
// c55c56cf0 (#25131): enabling an extra EIP used to mutate the operations of the
// process-global instruction set, because a JumpTable copy only duplicates the
// *operation pointers.
func TestUBF007_JumpTableDeepCopyOnExtraEips(t *testing.T) {
	// EIP-1884 reprices SLOAD, BALANCE and EXTCODEHASH; under Berlin's EIP-2929 all
	// three are priced dynamically, so their constant gas must still read zero in
	// the shared table afterwards.
	repriced := []OpCode{SLOAD, BALANCE, EXTCODEHASH}
	before := make(map[OpCode]uint64, len(repriced))
	for _, op := range repriced {
		before[op] = londonInstructionSet[op].constantGas
	}

	evm := NewEVM(newTestBlockContext(1), TxContext{}, nil, params.AllEthashProtocolChanges, Config{ExtraEips: []int{1884}})
	if got := evm.interpreter.cfg.JumpTable[SLOAD].constantGas; got != params.SloadGasEIP1884 {
		t.Fatalf("EIP-1884 not applied to the interpreter table: SLOAD gas %d", got)
	}
	for _, op := range repriced {
		if got := londonInstructionSet[op].constantGas; got != before[op] {
			t.Errorf("global london instruction set was mutated: %v gas %d, want %d", op, got, before[op])
		}
	}

	// Two failing EIPs in a row: the old code spliced the slice it was ranging
	// over, so the second one was never removed.
	extra := []int{99998, 99999}
	evm = NewEVM(newTestBlockContext(1), TxContext{}, nil, params.AllEthashProtocolChanges, Config{ExtraEips: extra})
	if got := evm.interpreter.cfg.ExtraEips; len(got) != 0 {
		t.Errorf("failed EIPs reported as activated: %v", got)
	}
	if extra[0] != 99998 || extra[1] != 99999 {
		t.Errorf("caller's ExtraEips slice was mutated: %v", extra)
	}
}
