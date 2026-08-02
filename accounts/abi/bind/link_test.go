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

package bind

import (
	"regexp"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

// The 32-byte-address Solidity fork emits library link placeholders of the
// form __$<58 hex chars>$__ (64 characters total, filling a PUSH32 operand),
// where the hex chars are the first 58 hex digits of the keccak256 hash of
// the fully qualified library name. Upstream solc used 34 hex chars inside a
// PUSH20 operand. This fixture is genuine output of the forked solc 0.7.6
// (v32b) for a contract calling library "libtest.sol:Math".
const solcForkLinkedBytecode = `608060405234801561001057600080fd5b5061015a806100206000396000f3fe608060405234801561001057600080fd5b506004361061002b5760003560e01c8063771602f714610030575b600080fd5b6100666004803603604081101561004657600080fd5b81019080803590602001909291908035906020019092919050505061007c565b6040518082815260200191505060405180910390f35b60007f__$58a68790b4feeb3b540fa1fbf9d48edfb36eb2e731adff81a19d69f8e5$__63771602f784846040518363ffffffff1660e01b8152600401808381526020018281526020019250505060206040518083038186803b1580156100e157600080fd5b505af41580156100f5573d6000803e3d6000fd5b505050506040513d602081101561010b57600080fd5b810190808051906020019092919050505090509291505056fea2646970667358221220c416209c7b5676ddacd1605d1c3af82d8508700bc11a4e60977ecaa25a3b7c1864736f6c63430007060033`

// TestSolcForkLinkPlaceholder verifies that the abigen placeholder derivation
// (keccak256 of the fully qualified library name, hex chars [2:60]) matches
// what the forked solc actually embeds in unlinked bytecode.
func TestSolcForkLinkPlaceholder(t *testing.T) {
	pattern := crypto.Keccak256Hash([]byte("libtest.sol:Math")).String()[2:60]
	matched, err := regexp.MatchString("__\\$"+pattern+"\\$__", solcForkLinkedBytecode)
	if err != nil {
		t.Fatalf("regexp error: %v", err)
	}
	if !matched {
		t.Fatalf("derived placeholder __$%s$__ not found in solc fork bytecode", pattern)
	}
}

// Genuine forked-solc 0.7.6 output for the library "libtest.sol:Math" that
// solcForkLinkedBytecode links against, plus both contracts' ABIs.
const solcForkMathBytecode = `60d0610025600082828239805160001a607f1461001857fe5b30600152607f81538281f3fe7f00000000000000000000000000000000000000000000000000000000000000003014608060405260043610603f5760003560e01c8063771602f7146044575b600080fd5b607760048036036040811015605857600080fd5b810190808035906020019092919080359060200190929190505050608d565b6040518082815260200191505060405180910390f35b600081830190509291505056fea264697066735822122086ee6a1a61d7aaf1e64438c768bf05462939b146cf875f086c4251108696b19964736f6c63430007060033`
const solcForkMathABI = `[{"inputs":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256","name":"b","type":"uint256"}],"name":"add","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
const solcForkUseLibraryABI = `[{"inputs":[{"internalType":"uint256","name":"c","type":"uint256"},{"internalType":"uint256","name":"d","type":"uint256"}],"name":"add","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

// TestSolcForkLibraryBind runs the full binding generation the abigen CLI
// performs (58-hex pattern derivation + library detection + link-substitution
// codegen) against genuine forked-solc output, mirroring cmd/abigen.
func TestSolcForkLibraryBind(t *testing.T) {
	pattern := crypto.Keccak256Hash([]byte("libtest.sol:Math")).String()[2:60]
	libs := map[string]string{pattern: "Math"}
	code, err := Bind(
		[]string{"UseLibrary", "Math"},
		[]string{solcForkUseLibraryABI, solcForkMathABI},
		[]string{solcForkLinkedBytecode, solcForkMathBytecode},
		nil,
		"bindtest",
		LangGo,
		libs,
		nil,
	)
	if err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	// Math must have been detected as a library dependency of UseLibrary...
	if !strings.Contains(code, "DeployMath") {
		t.Fatal("generated code does not contain DeployMath — library not detected")
	}
	// ...and the deploy code must substitute the 64-char placeholder with the
	// 64 hex chars of the deployed library's 32-byte address.
	wantSubst := `strings.Replace(UseLibraryBin, "__$` + pattern + `$__", mathAddr.String()[2:], -1)`
	if !strings.Contains(code, wantSubst) {
		t.Fatalf("generated code does not contain link substitution %q", wantSubst)
	}
}

// TestSolcForkLinkSubstitution verifies that substituting a deployed library
// address for the placeholder (as the generated deploy code does) yields
// bytecode of unchanged length: the 64-char placeholder is replaced by the
// 64 hex chars of a 32-byte address inside a PUSH32 operand.
func TestSolcForkLinkSubstitution(t *testing.T) {
	pattern := crypto.Keccak256Hash([]byte("libtest.sol:Math")).String()[2:60]
	placeholder := "__$" + pattern + "$__"
	if len(placeholder) != 64 {
		t.Fatalf("placeholder length = %d, want 64", len(placeholder))
	}
	addr := common.HexToAddress("0x1122334455667788990011223344556677889900112233445566778899001122")
	linked := strings.Replace(solcForkLinkedBytecode, placeholder, addr.String()[2:], -1)
	if strings.Contains(linked, "$") {
		t.Fatal("placeholder not fully substituted")
	}
	if len(linked) != len(solcForkLinkedBytecode) {
		t.Fatalf("linked bytecode length changed: %d != %d", len(linked), len(solcForkLinkedBytecode))
	}
	// The placeholder must sit directly after a PUSH32 (0x7f) opcode.
	idx := strings.Index(solcForkLinkedBytecode, placeholder)
	if solcForkLinkedBytecode[idx-2:idx] != "7f" {
		t.Fatalf("placeholder not preceded by PUSH32, got %q", solcForkLinkedBytecode[idx-2:idx])
	}
}
