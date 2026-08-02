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

package abigen

import (
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

// Genuine output of the 32-byte-address solc fork (0.7.6) for:
//
//	library Math { function add(uint a, uint b) public view returns (uint) { return a + b; } }
//	contract UseLibrary { function add(uint c, uint d) public view returns (uint) { return Math.add(c, d); } }
//
// compiled from a file named "libtest.sol". The UseLibrary bytecode contains
// the unlinked 58-hex placeholder __$<keccak256("libtest.sol:Math")[0:58]>$__
// inside a PUSH32 operand.
const (
	linkTestUseLibraryBin = `608060405234801561001057600080fd5b5061015a806100206000396000f3fe608060405234801561001057600080fd5b506004361061002b5760003560e01c8063771602f714610030575b600080fd5b6100666004803603604081101561004657600080fd5b81019080803590602001909291908035906020019092919050505061007c565b6040518082815260200191505060405180910390f35b60007f__$58a68790b4feeb3b540fa1fbf9d48edfb36eb2e731adff81a19d69f8e5$__63771602f784846040518363ffffffff1660e01b8152600401808381526020018281526020019250505060206040518083038186803b1580156100e157600080fd5b505af41580156100f5573d6000803e3d6000fd5b505050506040513d602081101561010b57600080fd5b810190808051906020019092919050505090509291505056fea2646970667358221220c416209c7b5676ddacd1605d1c3af82d8508700bc11a4e60977ecaa25a3b7c1864736f6c63430007060033`
	linkTestMathBin       = `60d0610025600082828239805160001a607f1461001857fe5b30600152607f81538281f3fe7f00000000000000000000000000000000000000000000000000000000000000003014608060405260043610603f5760003560e01c8063771602f7146044575b600080fd5b607760048036036040811015605857600080fd5b810190808035906020019092919080359060200190929190505050608d565b6040518082815260200191505060405180910390f35b600081830190509291505056fea264697066735822122086ee6a1a61d7aaf1e64438c768bf05462939b146cf875f086c4251108696b19964736f6c63430007060033`
	linkTestMathABI       = `[{"inputs":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256","name":"b","type":"uint256"}],"name":"add","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
	linkTestUseLibraryABI = `[{"inputs":[{"internalType":"uint256","name":"c","type":"uint256"},{"internalType":"uint256","name":"d","type":"uint256"}],"name":"add","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
)

// linkTestPattern derives the placeholder the same way cmd/abigen2 does:
// first 58 hex chars of keccak256 of the fully qualified library name.
func linkTestPattern() string {
	return crypto.Keccak256Hash([]byte("libtest.sol:Math")).String()[2:60]
}

// TestSolcForkLibraryBind exercises the v1 binder with the 58-hex placeholder
// pattern the CLI derives, against genuine forked-solc bytecode.
func TestSolcForkLibraryBind(t *testing.T) {
	pattern := linkTestPattern()
	libs := map[string]string{pattern: "Math"}
	code, err := Bind(
		[]string{"UseLibrary", "Math"},
		[]string{linkTestUseLibraryABI, linkTestMathABI},
		[]string{linkTestUseLibraryBin, linkTestMathBin},
		nil,
		"bindtest",
		libs,
		nil,
	)
	if err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
	if !strings.Contains(code, "DeployMath") {
		t.Fatal("generated code does not contain DeployMath — library not detected from the 58-hex placeholder")
	}
	wantSubst := `strings.ReplaceAll(UseLibraryBin, "__$` + pattern + `$__", mathAddr.String()[2:])`
	if !strings.Contains(code, wantSubst) {
		t.Fatalf("generated code does not contain link substitution %q", wantSubst)
	}
}

// TestSolcForkLibraryBindV2 exercises the v2 binder: the library dependency
// must be discovered from the unlinked bytecode (parseLibraryDeps) and wired
// into the generated metadata.
func TestSolcForkLibraryBindV2(t *testing.T) {
	pattern := linkTestPattern()
	libs := map[string]string{pattern: "Math"}
	code, err := BindV2(
		[]string{"UseLibrary", "Math"},
		[]string{linkTestUseLibraryABI, linkTestMathABI},
		[]string{linkTestUseLibraryBin, linkTestMathBin},
		"bindtest",
		libs,
		nil,
	)
	if err != nil {
		t.Fatalf("BindV2 failed: %v", err)
	}
	// The library's metadata carries its link pattern as ID (gofmt may align
	// the struct literal, so match the pattern string itself)...
	if !strings.Contains(code, `"`+pattern+`"`) {
		t.Fatalf("generated code does not carry the library pattern %q", pattern)
	}
	// ...and the consumer contract must list the library as a dependency.
	if !strings.Contains(code, "&MathMetaData") {
		t.Fatal("generated code does not list &MathMetaData as a dependency of UseLibrary")
	}
}
