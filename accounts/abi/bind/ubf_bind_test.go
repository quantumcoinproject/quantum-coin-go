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

package bind

import (
	"strings"
	"testing"
)

// TestUBF119_BindConstructorOnly — upstream b45931cc4.
// The structs map was only populated from evmABI.Methods, which excludes the
// constructor. A struct used solely as a constructor parameter was therefore
// missing when the template rendered it, causing a nil dereference.
func TestUBF119_BindConstructorOnly(t *testing.T) {
	const abiJSON = `[{"inputs":[{"components":[` +
		`{"internalType":"uint256","name":"field","type":"uint256"}],` +
		`"internalType":"struct ConstructorOnly.S","name":"s","type":"tuple"}],` +
		`"stateMutability":"nonpayable","type":"constructor"}]`

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Bind panicked on a constructor-only contract: %v", r)
		}
	}()
	code, err := Bind(
		[]string{"ConstructorOnly"},
		[]string{abiJSON},
		[]string{"0x00"},
		nil,
		"bindtest",
		LangGo,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	// The struct used only by the constructor must be emitted.
	if !strings.Contains(code, "type ConstructorOnlyS struct {") {
		t.Errorf("generated binding is missing the constructor-only struct type:\n%s", code)
	}
}
