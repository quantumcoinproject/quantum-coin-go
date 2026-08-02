// Copyright 2026 The go-ethereum Authors
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

package runtime

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

// Deployment bytecode compiled with the 32-byte-address solc fork
// (0.7.6, ContractType storage size fixed to 32 bytes) from:
//
//	contract B { function get() public pure returns (uint) { return 42; } }
//	contract X {
//	    address a; uint96 b; address payable p;
//	    B c; bool f;
//	    function setB(B _c) public { c = _c; }
//	    function callB() public view returns (uint) { return c.get(); }
//	}
//
// The contract-typed state variable `c` must occupy a full 32-byte storage
// slot (slot 3). Compilers that still treat contract types as 20 bytes mask
// the stored address and the cross-contract call below reverts.
const bin32ByteB = "6080604052348015600f57600080fd5b5060888061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c80636d4ce63c14602d575b600080fd5b60336049565b6040518082815260200191505060405180910390f35b6000602a90509056fea26469706673582212203b02d0be3020ed7d51a8621784757182a82e8954072918ff08ee5d0b8902bdff64736f6c63430007060033"
const bin32ByteX = "608060405234801561001057600080fd5b5061012f806100206000396000f3fe6080604052348015600f57600080fd5b506004361060325760003560e01c806344fd4fa01460375780637004be72146053575b600080fd5b603d607e565b6040518082815260200191505060405180910390f35b607c60048036036020811015606757600080fd5b810190808035906020019092919050505060ef565b005b6000600354636d4ce63c6040518163ffffffff1660e01b815260040160206040518083038186803b15801560b157600080fd5b505afa15801560c4573d6000803e3d6000fd5b505050506040513d602081101560d957600080fd5b8101908080519060200190929190505050905090565b806003819055505056fea2646970667358221220cf0c809f6495a25811803f1f2e96585d388e228f31a294230807b147e4a3c5c064736f6c63430007060033"

// TestCrossContractCallThroughStoredReference deploys B and X, stores B's
// 32-byte address (which has non-zero high bytes) in X's contract-typed state
// variable, and calls through it. This exercises the full 32-byte address
// path: CREATE address derivation, ABI address argument, SSTORE/SLOAD of an
// unmasked address word and the CALL through it.
func TestCrossContractCallThroughStoredReference(t *testing.T) {
	mustHex := func(s string) []byte {
		b, err := hex.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	cfg := &Config{
		State:    statedb,
		GasLimit: 10_000_000,
		Value:    big.NewInt(0),
	}

	_, bAddr, _, err := Create(mustHex(bin32ByteB), cfg)
	if err != nil {
		t.Fatalf("deploy B failed: %v", err)
	}
	_, xAddr, _, err := Create(mustHex(bin32ByteX), cfg)
	if err != nil {
		t.Fatalf("deploy X failed: %v", err)
	}
	if bAddr[0] == 0 {
		t.Logf("note: B address high byte is zero; truncation coverage weakened: %x", bAddr)
	}

	setB := append(crypto.Keccak256([]byte("setB(address)"))[:4], common.LeftPadBytes(bAddr.Bytes(), 32)...)
	if _, _, err := Call(xAddr, setB, cfg); err != nil {
		t.Fatalf("setB failed: %v", err)
	}

	// The contract-typed variable owns slot 3 entirely; the full unmasked
	// 32-byte address must be stored.
	slot3 := statedb.GetState(xAddr, common.BigToHash(big.NewInt(3)))
	if slot3 != bAddr.Hash() {
		t.Fatalf("stored contract reference corrupted: slot3 = %x, want %x", slot3, bAddr)
	}

	callB := crypto.Keccak256([]byte("callB()"))[:4]
	ret, _, err := Call(xAddr, callB, cfg)
	if err != nil {
		t.Fatalf("callB failed: %v", err)
	}
	if got := new(big.Int).SetBytes(ret); got.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("callB returned %v, want 42", got)
	}
}
