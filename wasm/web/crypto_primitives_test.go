//go:build !js
// +build !js

package main

// Deterministic known-answer tests for the List-B crypto primitives added to
// the WASM surface (Sha256, Sha512, Ripemd160, ComputeHmac, Pbkdf2) and the
// parameterized Scrypt. main.go is a (js && wasm)-only build, so - following
// the existing ...ForTest convention in main_test.go - the pure helper logic is
// mirrored here and exercised against fixed vectors. The real WASM globals are
// additionally verified end-to-end by the quantum-coin-js-sdk Node + browser
// test suites.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"testing"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/scrypt"
)

// ---- helper replicas (mirror main.go) ----

func hashConstructorForTest(alg string) (func() hash.Hash, error) {
	switch alg {
	case "sha256":
		return sha256.New, nil
	case "sha512":
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %q", alg)
	}
}

func sha256BytesForTest(data []byte) []byte { s := sha256.Sum256(data); return s[:] }
func sha512BytesForTest(data []byte) []byte { s := sha512.Sum512(data); return s[:] }

func ripemd160BytesForTest(data []byte) []byte {
	h := ripemd160.New()
	h.Write(data)
	return h.Sum(nil)
}

func hmacBytesForTest(alg string, key, data []byte) ([]byte, error) {
	nh, err := hashConstructorForTest(alg)
	if err != nil {
		return nil, err
	}
	m := hmac.New(nh, key)
	m.Write(data)
	return m.Sum(nil), nil
}

func pbkdf2BytesForTest(password, salt []byte, iter, keyLen int, alg string) ([]byte, error) {
	if iter <= 0 {
		return nil, errors.New("pbkdf2: iterations must be positive")
	}
	if keyLen <= 0 {
		return nil, errors.New("pbkdf2: keyLen must be positive")
	}
	nh, err := hashConstructorForTest(alg)
	if err != nil {
		return nil, err
	}
	return pbkdf2.Key(password, salt, iter, keyLen, nh), nil
}

const maxScryptNForTest = 1 << 21

func scryptBytesForTest(secret, salt []byte, N, r, p, dkLen int) ([]byte, error) {
	if N <= 0 || r <= 0 || p <= 0 || dkLen <= 0 {
		return nil, errors.New("scrypt: N, r, p and dkLen must be positive")
	}
	if N > maxScryptNForTest {
		return nil, fmt.Errorf("scrypt: N too large (max %d)", maxScryptNForTest)
	}
	return scrypt.Key(secret, salt, N, r, p, dkLen)
}

func mustHex(t *testing.T, h string) []byte {
	t.Helper()
	b, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("bad hex vector %q: %v", h, err)
	}
	return b
}

// ---- SHA-256 / SHA-512 / RIPEMD-160 ----

func TestSha256KnownAnswers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}
	for _, c := range cases {
		got := sha256BytesForTest([]byte(c.in))
		if !bytes.Equal(got, mustHex(t, c.want)) {
			t.Fatalf("sha256(%q) = %x, want %s", c.in, got, c.want)
		}
		// determinism / repeatability
		if !bytes.Equal(got, sha256BytesForTest([]byte(c.in))) {
			t.Fatalf("sha256(%q) not deterministic", c.in)
		}
	}
}

func TestSha512KnownAnswers(t *testing.T) {
	got := sha512BytesForTest([]byte("abc"))
	want := "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
		"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"
	if !bytes.Equal(got, mustHex(t, want)) {
		t.Fatalf("sha512(abc) = %x, want %s", got, want)
	}
	if !bytes.Equal(got, sha512BytesForTest([]byte("abc"))) {
		t.Fatal("sha512 not deterministic")
	}
}

func TestRipemd160KnownAnswers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "9c1185a5c5e9fc54612808977ee8f548b2258d31"},
		{"abc", "8eb208f7e05d987a9b044a8e98c6b087f15a0bfc"},
	}
	for _, c := range cases {
		got := ripemd160BytesForTest([]byte(c.in))
		if !bytes.Equal(got, mustHex(t, c.want)) {
			t.Fatalf("ripemd160(%q) = %x, want %s", c.in, got, c.want)
		}
	}
}

// ---- HMAC (RFC 4231 test case 2) ----

func TestComputeHmacRFC4231(t *testing.T) {
	key := []byte("Jefe")
	data := []byte("what do ya want for nothing?")

	sha256Want := "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"
	got256, err := hmacBytesForTest("sha256", key, data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got256, mustHex(t, sha256Want)) {
		t.Fatalf("hmac-sha256 = %x, want %s", got256, sha256Want)
	}

	sha512Want := "164b7a7bfcf819e2e395fbe73b56e0a387bd64222e831fd610270cd7ea250554" +
		"9758bf75c05a994a6d034f65f8f0e6fdcaeab1a34d4a6b4b636e070a38bce737"
	got512, err := hmacBytesForTest("sha512", key, data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got512, mustHex(t, sha512Want)) {
		t.Fatalf("hmac-sha512 = %x, want %s", got512, sha512Want)
	}
}

func TestComputeHmacUnsupportedAlg(t *testing.T) {
	if _, err := hmacBytesForTest("md5", []byte("k"), []byte("d")); err == nil {
		t.Fatal("expected error for unsupported HMAC alg")
	}
}

// ---- PBKDF2-HMAC-SHA256 (published vector) ----

func TestPbkdf2KnownAnswer(t *testing.T) {
	got, err := pbkdf2BytesForTest([]byte("password"), []byte("salt"), 1, 32, "sha256")
	if err != nil {
		t.Fatal(err)
	}
	want := "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
	if !bytes.Equal(got, mustHex(t, want)) {
		t.Fatalf("pbkdf2 = %x, want %s", got, want)
	}
	if !bytes.Equal(got, mustPbkdf2(t)) {
		t.Fatal("pbkdf2 not deterministic")
	}
}

func mustPbkdf2(t *testing.T) []byte {
	t.Helper()
	b, err := pbkdf2BytesForTest([]byte("password"), []byte("salt"), 1, 32, "sha256")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPbkdf2NegativeParams(t *testing.T) {
	if _, err := pbkdf2BytesForTest([]byte("p"), []byte("s"), 0, 32, "sha256"); err == nil {
		t.Fatal("expected error for iterations <= 0")
	}
	if _, err := pbkdf2BytesForTest([]byte("p"), []byte("s"), 1, 0, "sha256"); err == nil {
		t.Fatal("expected error for keyLen <= 0")
	}
	if _, err := pbkdf2BytesForTest([]byte("p"), []byte("s"), 1, 32, "md5"); err == nil {
		t.Fatal("expected error for unsupported alg")
	}
}

// ---- Scrypt (parameterized) ----

func TestScryptRFC7914Vector(t *testing.T) {
	// RFC 7914 section 12: scrypt("", "", 16, 1, 1, 64)
	got, err := scryptBytesForTest([]byte(""), []byte(""), 16, 1, 1, 64)
	if err != nil {
		t.Fatal(err)
	}
	want := "77d6576238657b203b19ca42c18a0497f16b4844e3074ae8dfdffa3fede21442" +
		"fcd0069ded0948f8326a753a0fc81f17e8d3e0fb2e0d3628cf35e20c38d18906"
	if !bytes.Equal(got, mustHex(t, want)) {
		t.Fatalf("scrypt rfc7914 = %x, want %s", got, want)
	}
}

func TestScryptParityVector(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping expensive N=262144 scrypt vector in -short mode")
	}
	// Parity with quantum-coin-js-sdk / Node crypto: N=262144, r=8, p=1, dkLen=32.
	got, err := scryptBytesForTest([]byte("password"), []byte("salt"), 262144, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	want := "d36e883d93698af49daa529419bb1d97da262bbaa225c12fcf05651268659f42"
	if !bytes.Equal(got, mustHex(t, want)) {
		t.Fatalf("scrypt parity = %x, want %s", got, want)
	}
}

func TestScryptDeterministicAndParameterized(t *testing.T) {
	a, err := scryptBytesForTest([]byte("pw"), []byte("salt"), 16, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	b, err := scryptBytesForTest([]byte("pw"), []byte("salt"), 16, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("scrypt not deterministic for identical inputs")
	}
	// A different (valid) param set must produce a different key -> proves params are honored.
	c, err := scryptBytesForTest([]byte("pw"), []byte("salt"), 32, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, c) {
		t.Fatal("changing N should change the derived key")
	}
	if len(a) != 32 {
		t.Fatalf("dkLen not honored: got %d bytes", len(a))
	}
}

func TestScryptNegativeParams(t *testing.T) {
	if _, err := scryptBytesForTest([]byte("p"), []byte("s"), 0, 8, 1, 32); err == nil {
		t.Fatal("expected error for N <= 0")
	}
	if _, err := scryptBytesForTest([]byte("p"), []byte("s"), 3, 8, 1, 32); err == nil {
		t.Fatal("expected error for N not a power of two")
	}
	if _, err := scryptBytesForTest([]byte("p"), []byte("s"), (1<<21)+2, 8, 1, 32); err == nil {
		t.Fatal("expected error for N above clamp")
	}
}
