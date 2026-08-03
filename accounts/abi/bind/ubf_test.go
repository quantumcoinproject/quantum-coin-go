// Copyright 2019 The go-ethereum Authors
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

package bind_test

import (
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi/bind"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

// TestUBF118_UnpackLogChecksEventSignature — upstream 92c5d104d.
// UnpackLog used to decode whatever log it was handed, so a log emitted by a
// completely different event was silently mis-decoded into the caller's struct.
func TestUBF118_UnpackLogChecksEventSignature(t *testing.T) {
	const abiString = `[{"anonymous":false,"inputs":[{"indexed":true,"name":"content","type":"bytes"},` +
		`{"indexed":false,"name":"sender","type":"address"},{"indexed":false,"name":"amount","type":"uint256"},` +
		`{"indexed":false,"name":"memo","type":"bytes"}],"name":"received","type":"event"}]`
	parsedAbi, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		t.Fatal(err)
	}
	bc := bind.NewBoundContract(common.HexToAddress("0x0"), parsedAbi, nil, nil, nil)

	correct := crypto.Keccak256Hash([]byte("received(bytes,address,uint256,bytes)"))
	payload := common.Hex2Bytes(
		"000000000000000000000000376c47978271565f56deb45495afa69e59c16ab2" +
			"0000000000000000000000000000000000000000000000000000000000000001" +
			"0000000000000000000000000000000000000000000000000000000000000060" +
			"0000000000000000000000000000000000000000000000000000000000000001" +
			"5800000000000000000000000000000000000000000000000000000000000000")

	mkLog := func(topics []common.Hash) types.Log {
		return types.Log{
			Address: common.HexToAddress("0x0"),
			Topics:  topics,
			Data:    payload,
		}
	}

	// A log from a different event must be rejected outright.
	wrong := crypto.Keccak256Hash([]byte("somethingElse(bytes)"))
	out := make(map[string]interface{})
	if err := bc.UnpackLogIntoMap(out, "received", mkLog([]common.Hash{wrong, {}})); err == nil {
		t.Error("UnpackLogIntoMap accepted a log with the wrong event signature")
	}
	var outStruct struct {
		Content common.Hash
		Sender  common.Address
		Amount  interface{}
		Memo    []byte
	}
	if err := bc.UnpackLog(&outStruct, "received", mkLog([]common.Hash{wrong, {}})); err == nil {
		t.Error("UnpackLog accepted a log with the wrong event signature")
	}

	// Our extra guard: an anonymous event carries no signature topic at all.
	// Upstream's unguarded log.Topics[0] panics here.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("UnpackLogIntoMap panicked on a topic-less log: %v", r)
			}
		}()
		if err := bc.UnpackLogIntoMap(make(map[string]interface{}), "received", mkLog(nil)); err == nil {
			t.Error("UnpackLogIntoMap accepted a log without any topics")
		}
		if err := bc.UnpackLog(&outStruct, "received", mkLog(nil)); err == nil {
			t.Error("UnpackLog accepted a log without any topics")
		}
	}()

	// The matching log must still decode.
	out = make(map[string]interface{})
	if err := bc.UnpackLogIntoMap(out, "received", mkLog([]common.Hash{correct, {}})); err != nil {
		t.Errorf("UnpackLogIntoMap rejected a matching log: %v", err)
	}
	if len(out) != 4 {
		t.Errorf("unpacked map length %d, want 4", len(out))
	}
}
