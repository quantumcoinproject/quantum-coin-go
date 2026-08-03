// Copyright 2015 The go-ethereum Authors
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

package abi

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
)

// TestUBF108_InvalidFieldNamePanic — upstream af02e9792.
// A tuple component whose name camel-cases into something that is not a legal Go
// identifier used to reach reflect.StructOf, which panics. Untrusted ABI JSON
// could therefore crash the process.
func TestUBF108_InvalidFieldNamePanic(t *testing.T) {
	// "0" camel-cases to "0", which is not a valid struct field name.
	for _, rawName := range []string{"0", "1x", "a.b", "a-b", "0abc"} {
		_, err := NewType("tuple", "", []ArgumentMarshaling{
			{Name: rawName, Type: "uint256"},
		})
		if err == nil {
			t.Fatalf("expected an error for field name %q, got none", rawName)
		}
	}
	// A valid name must still work.
	if _, err := NewType("tuple", "", []ArgumentMarshaling{
		{Name: "amount", Type: "uint256"},
	}); err != nil {
		t.Fatalf("valid tuple rejected: %v", err)
	}
}

// TestUBF108_InvalidFieldNameFromJSON exercises the same guard through the JSON
// entry point that an untrusted ABI would arrive on.
func TestUBF108_InvalidFieldNameFromJSON(t *testing.T) {
	const def = `[{"inputs":[{"components":[{"internalType":"uint256","name":"0","type":"uint256"}],` +
		`"internalType":"struct S","name":"s","type":"tuple"}],"name":"f","outputs":[],"type":"function"}]`
	if _, err := JSON(strings.NewReader(def)); err == nil {
		t.Fatal("expected an error for an ABI with an invalid tuple field name")
	}
}

// TestUBF111_FixedBytesTooLarge — upstream 8578eb2fe.
func TestUBF111_FixedBytesTooLarge(t *testing.T) {
	for _, typ := range []string{"bytes33", "bytes64", "bytes4096"} {
		if _, err := NewType(typ, "", nil); err == nil {
			t.Errorf("%s: fixed bytes with size over 32 is not spec'd, expected an error", typ)
		}
	}
	// The boundary must still be accepted.
	if _, err := NewType("bytes32", "", nil); err != nil {
		t.Fatalf("bytes32 rejected: %v", err)
	}
}

// TestUBF110_StringentIntegerDecode — upstream cefc0fa00.
// A word wider than the declared type used to be silently truncated to its low
// bytes; it must now be rejected.
func TestUBF110_StringentIntegerDecode(t *testing.T) {
	mkWord := func(hex string) []byte {
		return common.Hex2Bytes(hex)
	}
	tests := []struct {
		typ     string
		word    string
		wantErr bool
		want    interface{}
	}{
		// 0x0101 does not fit uint8: previously decoded as 0x01.
		{"uint8", "0000000000000000000000000000000000000000000000000000000000000101", true, nil},
		{"uint8", "00000000000000000000000000000000000000000000000000000000000000ff", false, byte(0xff)},
		{"uint16", "0000000000000000000000000000000000000000000000000000000000010001", true, nil},
		{"uint32", "0000000000000000000000000000000000000000000000000000000100000001", true, nil},
		{"uint64", "0000000000000000000000000000000000000000000000010000000000000001", true, nil},
		// -1 as int256 does not fit int8 either.
		{"int8", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00", true, nil},
		{"int8", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", false, int8(-1)},
	}
	for _, tt := range tests {
		typ, err := NewType(tt.typ, "", nil)
		if err != nil {
			t.Fatalf("%s: %v", tt.typ, err)
		}
		got, err := ReadInteger(typ, mkWord(tt.word))
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s/%s: expected an error, got value %v", tt.typ, tt.word, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s/%s: unexpected error: %v", tt.typ, tt.word, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s/%s: got %v (%T), want %v (%T)", tt.typ, tt.word, got, got, tt.want, tt.want)
		}
	}
}

// TestUBF110_StringentIntegerDecodeThroughUnpack makes sure the error is threaded
// all the way out of Arguments.UnpackValues rather than swallowed.
func TestUBF110_StringentIntegerDecodeThroughUnpack(t *testing.T) {
	abi, err := JSON(strings.NewReader(`[{"name":"f","type":"function","outputs":[{"type":"uint8"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	data := common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000101")
	if _, err := abi.Unpack("f", data); err == nil {
		t.Fatal("expected an error unpacking an over-wide uint8")
	}
}

// TestUBF112_NegativeBigIntTopic — upstream d0edc5af4 + 3c754e2a0.
// rule.Bytes() drops the sign, so a negative int256 filter produced the topic of
// the absolute value instead of the two's-complement encoding.
func TestUBF112_NegativeBigIntTopic(t *testing.T) {
	rule := big.NewInt(-1)
	orig := new(big.Int).Set(rule)

	topics, err := MakeTopics([]interface{}{rule})
	if err != nil {
		t.Fatal(err)
	}
	want := common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	if topics[0][0] != want {
		t.Errorf("negative topic: got %x, want %x", topics[0][0], want)
	}
	// 3c754e2a0: math.U256Bytes mutates its argument, the caller's value must be
	// left alone.
	if rule.Cmp(orig) != 0 {
		t.Errorf("MakeTopics mutated its input: got %v, want %v", rule, orig)
	}

	// A small negative value keeps the sign extension.
	topics, err = MakeTopics([]interface{}{big.NewInt(-129)})
	if err != nil {
		t.Fatal(err)
	}
	want = common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7f")
	if topics[0][0] != want {
		t.Errorf("negative topic: got %x, want %x", topics[0][0], want)
	}

	// Positive values must be unchanged.
	topics, err = MakeTopics([]interface{}{big.NewInt(128)})
	if err != nil {
		t.Fatal(err)
	}
	want = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000080")
	if topics[0][0] != want {
		t.Errorf("positive topic: got %x, want %x", topics[0][0], want)
	}
}

// TestUBF113_AllIndexedEventEmptyData — upstream 345b1fb82.
// An event whose arguments are all indexed carries no data section, which is not
// an error.
func TestUBF113_AllIndexedEventEmptyData(t *testing.T) {
	def := `[{"type":"bool","indexed":true},{"type":"uint64","indexed":true}]`
	var args Arguments
	if err := json.Unmarshal([]byte(def), &args); err != nil {
		t.Fatal(err)
	}
	if _, err := args.Unpack(nil); err != nil {
		t.Errorf("Unpack: unexpected error for an all-indexed event: %v", err)
	}
	m := make(map[string]interface{})
	if err := args.UnpackIntoMap(m, nil); err != nil {
		t.Errorf("UnpackIntoMap: unexpected error for an all-indexed event: %v", err)
	}
	var out struct{}
	if err := args.Copy(&out, nil); err != nil {
		t.Errorf("Copy: unexpected error for an all-indexed event: %v", err)
	}

	// A partially indexed event must still reject empty data.
	def = `[{"type":"bytes32","indexed":true},{"type":"uint256","indexed":false}]`
	args = nil
	if err := json.Unmarshal([]byte(def), &args); err != nil {
		t.Fatal(err)
	}
	if _, err := args.Unpack(nil); err == nil {
		t.Error("Unpack: expected an error when a non-indexed argument is expected")
	}
}

// TestUBF114_ToGoTypeErrorCheckedFirst — upstream c2918c2f4.
// The toGoType error was checked only after the virtual-argument arithmetic ran,
// so malformed input panicked before the error could be returned.
func TestUBF114_ToGoTypeErrorCheckedFirst(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UnpackValues panicked instead of returning the toGoType error: %v", r)
		}
	}()
	// toGoType rejects this output as too short. The old code then went on to
	// evaluate isDynamicType/getTypeSize on the array type, dereferencing an Elem
	// it had never validated.
	args := Arguments{{Name: "a", Type: Type{T: ArrayTy, Size: 2}}}
	if _, err := args.UnpackValues([]byte{}); err == nil {
		t.Fatal("expected an error for truncated output")
	}
	// Same for the tuple path in forTupleUnpack.
	tuple := Type{T: TupleTy, TupleElems: []*Type{{T: ArrayTy, Size: 2}}, TupleRawNames: []string{"a"}}
	tuple.TupleType = reflect.StructOf([]reflect.StructField{
		{Name: "A", Type: reflect.ArrayOf(2, reflect.TypeOf(common.Hash{}))},
	})
	if _, err := forTupleUnpack(tuple, []byte{}); err == nil {
		t.Fatal("expected an error for truncated tuple output")
	}

	// A well-formed ABI must still round-trip errors normally.
	abi, err := JSON(strings.NewReader(`[{"name":"f","type":"function","outputs":[{"type":"uint256[2]"},{"type":"uint256"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := abi.Unpack("f", make([]byte, 16)); err == nil {
		t.Fatal("expected an error for truncated output")
	}
}

// TestUBF115_SetAddressableInterface — upstream 086588062.
// set() recursed into the interface's dynamic value even when that value was not
// a pointer and not settable, which fails for e.g. []interface{}{"", ...}.
func TestUBF115_SetAddressableInterface(t *testing.T) {
	abi, err := JSON(strings.NewReader(`[{"name":"f","type":"function","outputs":[{"name":"i","type":"uint256"},{"name":"s","type":"string"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	// int = 1, string = "hello"
	data := common.Hex2Bytes(
		"0000000000000000000000000000000000000000000000000000000000000001" +
			"0000000000000000000000000000000000000000000000000000000000000040" +
			"0000000000000000000000000000000000000000000000000000000000000005" +
			"68656c6c6f000000000000000000000000000000000000000000000000000000")

	bigint := new(big.Int)
	// The second slot holds a plain (non-pointer, non-addressable) string. Before
	// the fix this failed with "cannot unmarshal string in to string".
	out := &[]interface{}{&bigint, ""}
	if err := abi.UnpackIntoInterface(out, "f", data); err != nil {
		t.Fatalf("unpack into a non-indirected slice element: %v", err)
	}
	if got := (*out)[1].(string); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if bigint.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("got %v, want 1", bigint)
	}
}

// TestUBF116_CopyAtomicSingleStruct — upstream 57a3fab8a.
// A single tuple argument decodes to a struct, and must still be stored in the
// destination's first field.
func TestUBF116_CopyAtomicSingleStruct(t *testing.T) {
	const def = `[{"name":"e","type":"event","anonymous":false,"inputs":[{"components":[` +
		`{"internalType":"uint256","name":"a","type":"uint256"},` +
		`{"internalType":"uint256","name":"b","type":"uint256"}],` +
		`"indexed":false,"internalType":"struct MyStruct","name":"s","type":"tuple"}]}]`
	abi, err := JSON(strings.NewReader(def))
	if err != nil {
		t.Fatal(err)
	}
	data := common.Hex2Bytes(
		"0000000000000000000000000000000000000000000000000000000000000001" +
			"0000000000000000000000000000000000000000000000000000000000000002")

	type myStruct struct {
		A *big.Int
		B *big.Int
	}
	var out struct {
		S myStruct
	}
	if err := abi.UnpackIntoInterface(&out, "e", data); err != nil {
		t.Fatalf("unpack single struct argument: %v", err)
	}
	if out.S.A == nil || out.S.A.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("A: got %v, want 1", out.S.A)
	}
	if out.S.B == nil || out.S.B.Cmp(big.NewInt(2)) != 0 {
		t.Errorf("B: got %v, want 2", out.S.B)
	}
}

// TestUBF117_ErrorQuotesUntrustedData — upstream 111ed1af1.
// getArguments must not splice raw attacker-controlled bytes into its error
// message. Here the payload is only described by its length, which is strictly
// stronger than upstream's %q.
func TestUBF117_ErrorQuotesUntrustedData(t *testing.T) {
	abi, err := JSON(strings.NewReader(`[{"name":"f","type":"function","outputs":[{"type":"uint256"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	// Not a multiple of 32, so getArguments errors out. The payload contains
	// terminal escape sequences and a NUL byte.
	payload := []byte("\x1b[31mBOOM\x00\n")
	_, err = abi.Unpack("f", payload)
	if err == nil {
		t.Fatal("expected an error for a misaligned output")
	}
	if strings.Contains(err.Error(), string(payload)) {
		t.Errorf("error message embeds the raw untrusted payload: %q", err.Error())
	}
	for _, c := range []string{"\x1b", "\x00"} {
		if strings.Contains(err.Error(), c) {
			t.Errorf("error message contains an unescaped control byte %q: %q", c, err.Error())
		}
	}
}
