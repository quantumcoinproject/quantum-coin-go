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

package core

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/common/math"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/signer/core/apitypes"
)

// mailTypedData is a Mail/Person message that, unlike the flat EIP-712 example,
// has an *array of structs* as one of its members. That is the shape that
// upstream 5a0d487c3 was hashing incorrectly.
func mailTypedData(t *testing.T) TypedData {
	t.Helper()
	return TypedData{
		Types: Types{
			"EIP712Domain": []Type{
				{Name: "name", Type: "string"},
			},
			"Person": []Type{
				{Name: "name", Type: "string"},
				{Name: "wallet", Type: "address"},
			},
			"Mail": []Type{
				{Name: "from", Type: "Person"},
				{Name: "to", Type: "Person[]"},
				{Name: "contents", Type: "string"},
			},
		},
		PrimaryType: "Mail",
		Domain: TypedDataDomain{
			Name: "Ether Mail",
		},
		Message: TypedDataMessage{
			"from": map[string]interface{}{
				"name":   "Cow",
				"wallet": "0xCD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826CD2a3d9F938E13CD947Ec05A",
			},
			"to": []interface{}{
				map[string]interface{}{
					"name":   "Bob",
					"wallet": "0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbBbBbBBbBBbbBbBBbBBbBbbBbB",
				},
				map[string]interface{}{
					"name":   "Alice",
					"wallet": "0xaAaAaAaaAaAaAaaAaAAAAAAAAaaaAaAaAaaAaaAaAaAaAaAaAaAaAaAaAaAaAaAa",
				},
			},
			"contents": "Hello, Bob!",
		},
	}
}

// encodeAddress mirrors how a 32-byte address occupies a single ABI word here:
// the whole address fills the word, there is no 12-byte left pad.
func encodeAddress(hex string) []byte {
	word := make([]byte, 32)
	copy(word, common.HexToAddress(hex).Bytes())
	return word
}

// TestUBF109_EIP712ArrayOfStructHash — upstream 5a0d487c3.
//
// Two independent defects made clef sign the wrong digest for any typed data
// carrying an array of structs:
//   - Dependencies() did not strip the "[]" suffix, so the referenced struct was
//     missing from the encodeType string, and
//   - the array member encoding wrote enc(item) instead of hashStruct(item).
//
// The expectation below is derived straight from the EIP-712 specification
// rather than from the implementation.
func TestUBF109_EIP712ArrayOfStructHash(t *testing.T) {
	typedData := mailTypedData(t)

	// Per the spec: encodeType(Mail) is the primary type followed by its
	// referenced types sorted by name — "Person" is referenced through "Person[]".
	wantEncodeType := "Mail(Person from,Person[] to,string contents)Person(string name,address wallet)"
	if got := string(typedData.EncodeType("Mail")); got != wantEncodeType {
		t.Errorf("encodeType(Mail):\n got %q\nwant %q", got, wantEncodeType)
	}

	personTypeHash := crypto.Keccak256([]byte("Person(string name,address wallet)"))
	hashPerson := func(name, wallet string) []byte {
		var buf bytes.Buffer
		buf.Write(personTypeHash)
		buf.Write(crypto.Keccak256([]byte(name)))
		buf.Write(encodeAddress(wallet))
		return crypto.Keccak256(buf.Bytes())
	}

	// hashStruct(Mail) = keccak(typeHash ‖ hashStruct(from) ‖ keccak(‖ᵢ hashStruct(toᵢ)) ‖ keccak(contents))
	var toBuf bytes.Buffer
	toBuf.Write(hashPerson("Bob", "0xbBbBBBBbbBBBbbbBbbBbbbbBBbBbbbbBbBbbBBbBbBbBBbBBbbBbBBbBBbBbbBbB"))
	toBuf.Write(hashPerson("Alice", "0xaAaAaAaaAaAaAaaAaAAAAAAAAaaaAaAaAaaAaaAaAaAaAaAaAaAaAaAaAaAaAaAa"))

	var mailBuf bytes.Buffer
	mailBuf.Write(crypto.Keccak256([]byte(wantEncodeType)))
	mailBuf.Write(hashPerson("Cow", "0xCD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826CD2a3d9F938E13CD947Ec05A"))
	mailBuf.Write(crypto.Keccak256(toBuf.Bytes()))
	mailBuf.Write(crypto.Keccak256([]byte("Hello, Bob!")))
	want := crypto.Keccak256(mailBuf.Bytes())

	got, err := typedData.HashStruct("Mail", typedData.Message)
	if err != nil {
		t.Fatalf("HashStruct: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("hashStruct(Mail): got %x, want %x", got, want)
	}
}

// TestUBF120_EIP712ArrayValueTypes — upstream e76813eb1.
// Array members were only accepted when they arrived as []interface{}, which is
// what encoding/json produces; a caller passing a native Go slice was rejected.
func TestUBF120_EIP712ArrayValueTypes(t *testing.T) {
	typedData := TypedData{
		Types: Types{
			"EIP712Domain": []Type{{Name: "name", Type: "string"}},
			"Foo":          []Type{{Name: "list", Type: "string[]"}},
		},
		PrimaryType: "Foo",
		Domain:      TypedDataDomain{Name: "test"},
		Message:     TypedDataMessage{"list": []interface{}{"a", "b"}},
	}
	want, err := typedData.HashStruct("Foo", typedData.Message)
	if err != nil {
		t.Fatalf("baseline HashStruct: %v", err)
	}

	// The same value expressed as a native []string must hash identically.
	typedData.Message = TypedDataMessage{"list": []string{"a", "b"}}
	got, err := typedData.HashStruct("Foo", typedData.Message)
	if err != nil {
		t.Fatalf("HashStruct with a []string member: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("[]string member: got %x, want %x", got, want)
	}

	// Formatting for the UI must treat both representations the same way. (It
	// cannot render a string[] at all — a separate, pre-existing limitation of
	// formatPrimitiveValue — but it must not diverge between the two forms.)
	_, errIface := typedData.formatData("Foo", TypedDataMessage{"list": []interface{}{"a", "b"}})
	_, errNative := typedData.formatData("Foo", TypedDataMessage{"list": []string{"a", "b"}})
	if (errIface == nil) != (errNative == nil) {
		t.Errorf("formatData diverges: []interface{} -> %v, []string -> %v", errIface, errNative)
	}

	// A non-slice value is still a mismatch.
	typedData.Message = TypedDataMessage{"list": "notaslice"}
	if _, err := typedData.HashStruct("Foo", typedData.Message); err == nil {
		t.Error("expected an error for a non-slice array member")
	}
}

// TestUBF121_EIP712LowercaseReferenceType — upstream 4f4a25d79.
// A reference type was identified by a leading uppercase rune, so a perfectly
// legal lowercase struct name was treated as an unknown primitive.
func TestUBF121_EIP712LowercaseReferenceType(t *testing.T) {
	types := Types{
		"EIP712Domain": []Type{{Name: "name", Type: "string"}},
		"mail":         []Type{{Name: "from", Type: "person"}},
		"person":       []Type{{Name: "name", Type: "string"}},
	}
	if err := types.validate(); err != nil {
		t.Errorf("lowercase reference type rejected: %v", err)
	}

	// An undefined reference type must still be rejected, whatever its case.
	types = Types{
		"EIP712Domain": []Type{{Name: "name", Type: "string"}},
		"mail":         []Type{{Name: "from", Type: "nosuchtype"}},
	}
	if err := types.validate(); err == nil {
		t.Error("expected an error for an undefined reference type")
	}

	// And the digest of a lowercase-named struct must still be computable.
	typedData := TypedData{
		Types: Types{
			"EIP712Domain": []Type{{Name: "name", Type: "string"}},
			"mail":         []Type{{Name: "from", Type: "person"}},
			"person":       []Type{{Name: "name", Type: "string"}},
		},
		PrimaryType: "mail",
		Domain:      TypedDataDomain{Name: "test"},
		Message: TypedDataMessage{
			"from": map[string]interface{}{"name": "Cow"},
		},
	}
	if _, err := typedData.HashStruct("mail", typedData.Message); err != nil {
		t.Errorf("HashStruct on a lowercase struct: %v", err)
	}
}

// TestUBF122_EIP712GoInputTypes — upstream 6d5590834.
// Values handed in as native Go types (a *big.Int, a [N]byte, an address as raw
// bytes) were rejected; only the JSON string forms were accepted.
func TestUBF122_EIP712GoInputTypes(t *testing.T) {
	typedData := TypedData{
		Types:       Types{"EIP712Domain": []Type{{Name: "name", Type: "string"}}},
		PrimaryType: "EIP712Domain",
		Domain:      TypedDataDomain{Name: "test"},
	}
	const addrHex = "0xCD2a3d9F938E13CD947Ec05AbC7FE734Df8DD826CD2a3d9F938E13CD947Ec05A"
	addr := common.HexToAddress(addrHex)

	// 32-byte note: the whole address fills the word, no [12:] offset.
	wantAddr := make([]byte, common.AddressLength)
	copy(wantAddr, addr.Bytes())

	// A [32]byte array value.
	var arrayAddr [common.AddressLength]byte
	copy(arrayAddr[:], addr.Bytes())
	got, err := typedData.EncodePrimitiveValue("address", arrayAddr, 1)
	if err != nil {
		t.Fatalf("address as [%d]byte: %v", common.AddressLength, err)
	}
	if !bytes.Equal(got, wantAddr) {
		t.Errorf("address as array: got %x, want %x", got, wantAddr)
	}

	// A []byte value of the right length.
	got, err = typedData.EncodePrimitiveValue("address", addr.Bytes(), 1)
	if err != nil {
		t.Fatalf("address as []byte: %v", err)
	}
	if !bytes.Equal(got, wantAddr) {
		t.Errorf("address as slice: got %x, want %x", got, wantAddr)
	}

	// The hex string form must be unchanged.
	got, err = typedData.EncodePrimitiveValue("address", addrHex, 1)
	if err != nil {
		t.Fatalf("address as string: %v", err)
	}
	if !bytes.Equal(got, wantAddr) {
		t.Errorf("address as string: got %x, want %x", got, wantAddr)
	}

	// A wrongly sized byte slice is still a mismatch.
	if _, err := typedData.EncodePrimitiveValue("address", []byte{1, 2, 3}, 1); err == nil {
		t.Error("expected an error for a short address slice")
	}

	// Integers supplied as a plain *big.Int.
	got, err = typedData.EncodePrimitiveValue("uint256", big.NewInt(5), 1)
	if err != nil {
		t.Fatalf("uint256 as *big.Int: %v", err)
	}
	if want := math.U256Bytes(big.NewInt(5)); !bytes.Equal(got, want) {
		t.Errorf("uint256 as *big.Int: got %x, want %x", got, want)
	}
	if _, err := typedData.EncodePrimitiveValue("int32", big.NewInt(-2), 1); err != nil {
		t.Errorf("int32 as *big.Int: %v", err)
	}

	// Fixed bytes supplied as a Go array.
	var b4 [4]byte
	copy(b4[:], []byte{1, 2, 3, 4})
	got, err = typedData.EncodePrimitiveValue("bytes4", b4, 1)
	if err != nil {
		t.Fatalf("bytes4 as [4]byte: %v", err)
	}
	want := make([]byte, 32)
	copy(want, b4[:])
	if !bytes.Equal(got, want) {
		t.Errorf("bytes4 as [4]byte: got %x, want %x", got, want)
	}
}

// TestUBF123_EIP712Int96 — upstream 55f914a1d.
func TestUBF123_EIP712Int96(t *testing.T) {
	for _, typ := range []string{"int96", "int96[]", "uint96", "uint96[]"} {
		if !isPrimitiveTypeValid(typ) {
			t.Errorf("%s should be a valid primitive type", typ)
		}
	}
	types := Types{
		"EIP712Domain": []Type{{Name: "name", Type: "string"}},
		"Foo":          []Type{{Name: "a", Type: "uint96"}, {Name: "b", Type: "int96"}},
	}
	if err := types.validate(); err != nil {
		t.Errorf("int96/uint96 rejected: %v", err)
	}
	// Nonsense widths must still be rejected.
	if isPrimitiveTypeValid("uint97") {
		t.Error("uint97 should not be a valid primitive type")
	}
}

// deniedUI approves nothing. Only the methods SignGnosisSafeTx reaches are
// implemented; the embedded interface makes the rest explicit nil panics.
type deniedUI struct {
	UIClientAPI
	sawSignRequest bool
}

func (ui *deniedUI) ApproveSignData(*SignDataRequest) (SignDataResponse, error) {
	ui.sawSignRequest = true
	return SignDataResponse{Approved: false}, nil
}

func (ui *deniedUI) ShowError(string) {}

type noopValidator struct{}

func (noopValidator) ValidateTransaction(*string, *apitypes.SendTxArgs) (*apitypes.ValidationMessages, error) {
	return &apitypes.ValidationMessages{}, nil
}

// TestUBF124_GnosisSafeTxHashVerified — upstream 502fa829a (with the ChainId
// precursor 7dec26db2, which this repo was missing and which is ported alongside).
//
// Clef used to sign whatever safeTxHash it computed, never comparing it against
// the 'safeTxHash' the relayer said it wanted. A tampered payload was therefore
// signed silently.
func TestUBF124_GnosisSafeTxHashVerified(t *testing.T) {
	newTx := func() GnosisSafeTx {
		safe, _ := common.NewMixedcaseAddressFromString("0x899FcB1437DE65DC6315f5a69C017dd3F2837557899FcB1437DE65DC6315f5a6")
		to, _ := common.NewMixedcaseAddressFromString("0xD3Ed2b8756b942c98c851722F3bd507a17B4745FD3Ed2b8756b942c98c851722")
		data := hexutil.Bytes{0x0d, 0x58, 0x2f, 0x13}
		return GnosisSafeTx{
			Safe: *safe,
			To:   *to,
			Data: &data,
		}
	}
	newAPI := func(ui UIClientAPI) *SignerAPI {
		return &SignerAPI{
			chainID:   big.NewInt(1337),
			UI:        ui,
			validator: noopValidator{},
		}
	}

	// 1. A safeTxHash that does not match what we computed must be refused, and
	//    the user must never even be asked to sign.
	ui := &deniedUI{}
	api := newAPI(ui)
	tx := newTx()
	tx.InputExpHash = common.HexToHash("0xdeadbeef")
	if _, err := api.SignGnosisSafeTx(context.Background(), tx.Safe, tx, nil); err == nil {
		t.Error("a mismatched safeTxHash was accepted")
	} else if !strings.Contains(err.Error(), "mismatched safeTxHash") {
		t.Errorf("unexpected error: %v", err)
	}
	if ui.sawSignRequest {
		t.Error("clef asked the user to sign a tx whose safeTxHash did not match")
	}

	// 2. The matching hash gets through the check; signing then stops at the
	//    (denied) approval, which proves the verification passed.
	tx = newTx()
	sighash, _, err := TypedDataAndHash(tx.ToTypedData())
	if err != nil {
		t.Fatal(err)
	}
	tx.InputExpHash = common.BytesToHash(sighash)
	ui = &deniedUI{}
	api = newAPI(ui)
	if _, err := api.SignGnosisSafeTx(context.Background(), tx.Safe, tx, nil); !errors.Is(err, ErrRequestDenied) {
		t.Errorf("matching safeTxHash: got %v, want %v", err, ErrRequestDenied)
	}
	if !ui.sawSignRequest {
		t.Error("a matching safeTxHash never reached the signing step")
	}

	// 3. A payload whose json omits the chain id: the hash only matches once the
	//    configured chain id is folded into the domain.
	tx = newTx()
	withChain := newTx()
	withChain.ChainId = (*math.HexOrDecimal256)(big.NewInt(1337))
	sighash, _, err = TypedDataAndHash(withChain.ToTypedData())
	if err != nil {
		t.Fatal(err)
	}
	tx.InputExpHash = common.BytesToHash(sighash)
	ui = &deniedUI{}
	api = newAPI(ui)
	if _, err := api.SignGnosisSafeTx(context.Background(), tx.Safe, tx, nil); !errors.Is(err, ErrRequestDenied) {
		t.Errorf("chain-id retry: got %v, want %v", err, ErrRequestDenied)
	}
	if !ui.sawSignRequest {
		t.Error("the chain-id retry never reached the signing step")
	}

	// 4. A payload that already carries a chain id but a tampered safeTxHash
	//    must be refused too — the chain-id retry branch must not double as a
	//    bypass of the verification.
	tx = newTx()
	tx.ChainId = (*math.HexOrDecimal256)(big.NewInt(1337))
	tx.InputExpHash = common.HexToHash("0xdeadbeef")
	ui = &deniedUI{}
	api = newAPI(ui)
	if _, err := api.SignGnosisSafeTx(context.Background(), tx.Safe, tx, nil); err == nil {
		t.Error("a mismatched safeTxHash with an explicit chain id was accepted")
	} else if !strings.Contains(err.Error(), "mismatched safeTxHash") {
		t.Errorf("mismatch with explicit chain id: unexpected error: %v", err)
	}
	if ui.sawSignRequest {
		t.Error("clef asked the user to sign a tx whose safeTxHash did not match (chain id present)")
	}
}
