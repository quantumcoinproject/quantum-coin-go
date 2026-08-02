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

package main

import "testing"

// The 32-byte-address solc fork embeds library link placeholders of the form
// __$<58 hex>$__ (64 chars, filling a PUSH32 operand), where the hex chars
// are the first 58 hex digits of keccak256 of the fully qualified library
// name. This placeholder was taken verbatim from forked-solc 0.7.6 output
// for a library named "Math" in "libtest.sol".
const solcForkMathPlaceholder = "58a68790b4feeb3b540fa1fbf9d48edfb36eb2e731adff81a19d69f8e5"

func TestLibPatternMatchesSolcFork(t *testing.T) {
	got := libPattern("libtest.sol:Math")
	if len(got) != 58 {
		t.Fatalf("libPattern length = %d, want 58 (solc fork emits 58-hex placeholders)", len(got))
	}
	if got != solcForkMathPlaceholder {
		t.Fatalf("libPattern(\"libtest.sol:Math\") = %s, want %s (real solc fork placeholder)", got, solcForkMathPlaceholder)
	}
	// The full placeholder token must be exactly 64 characters, so that
	// substituting the 64 hex chars of a 32-byte address preserves length.
	if token := "__$" + got + "$__"; len(token) != 64 {
		t.Fatalf("placeholder token length = %d, want 64", len(token))
	}
}
