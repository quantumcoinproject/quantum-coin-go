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

package core

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
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

// newPreCheckTransition builds the minimal StateTransition needed to exercise
// preCheck against the given state at the given block height.
func newPreCheckTransition(statedb *state.StateDB, blockNumber int64, msg Message) *StateTransition {
	blockCtx := vm.BlockContext{
		CanTransfer: func(vm.StateDB, common.Address, *big.Int) bool { return true },
		Transfer:    func(vm.StateDB, common.Address, common.Address, *big.Int) {},
		BlockNumber: big.NewInt(blockNumber),
	}
	evm := vm.NewEVM(blockCtx, vm.TxContext{}, statedb, params.AllEthashProtocolChanges, vm.Config{})
	return NewStateTransition(evm, msg, new(GasPool).AddGas(msg.Gas()))
}

// TestUBF002_EIP3607RejectSenderWithCode covers upstream 0658712f6 + d02c60536:
// EIP-3607 rejects a transaction whose sender account holds code.
func TestUBF002_EIP3607RejectSenderWithCode(t *testing.T) {
	setUpstreamFixesGate(t, 100)

	sender := common.BytesToAddress([]byte("contract-sender"))

	tests := []struct {
		name    string
		block   int64
		wantErr error
	}{
		{"pre-fork accepts a sender with code", 99, nil},
		{"post-fork rejects a sender with code", 100, ErrSenderNoEOA},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
			statedb.CreateAccount(sender)
			statedb.SetCode(sender, []byte{0x60, 0x00})
			statedb.AddBalance(sender, big.NewInt(1000000000))
			statedb.Finalise(true)

			msg := types.NewMessage(sender, &common.Address{}, 0, new(big.Int), params.TxGas, big.NewInt(1), nil, nil, true, 0)
			err := newPreCheckTransition(statedb, tt.block, msg).preCheck()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error mismatch: have %v, want %v", err, tt.wantErr)
			}
		})
	}

	// A codeless sender must stay acceptable on both sides of the fork.
	t.Run("post-fork accepts an EOA", func(t *testing.T) {
		eoa := common.BytesToAddress([]byte("eoa-sender"))
		statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
		statedb.CreateAccount(eoa)
		statedb.AddBalance(eoa, big.NewInt(1000000000))
		statedb.Finalise(true)

		msg := types.NewMessage(eoa, &common.Address{}, 0, new(big.Int), params.TxGas, big.NewInt(1), nil, nil, true, 0)
		if err := newPreCheckTransition(statedb, 100, msg).preCheck(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestUBF003_EIP2681NonceMax covers upstream f32feeb26: a sender already at nonce
// 2^64-1 may not send another transaction, since applying it would wrap the nonce.
func TestUBF003_EIP2681NonceMax(t *testing.T) {
	setUpstreamFixesGate(t, 100)

	sender := common.BytesToAddress([]byte("max-nonce-sender"))

	tests := []struct {
		name    string
		block   int64
		wantErr error
	}{
		{"pre-fork accepts the max nonce", 99, nil},
		{"post-fork rejects the max nonce", 100, ErrNonceMax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
			statedb.CreateAccount(sender)
			statedb.SetNonce(sender, math.MaxUint64)
			statedb.AddBalance(sender, big.NewInt(1000000000))
			statedb.Finalise(true)

			msg := types.NewMessage(sender, &common.Address{}, math.MaxUint64, new(big.Int), params.TxGas, big.NewInt(1), nil, nil, true, 0)
			err := newPreCheckTransition(statedb, tt.block, msg).preCheck()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error mismatch: have %v, want %v", err, tt.wantErr)
			}
		})
	}
}
