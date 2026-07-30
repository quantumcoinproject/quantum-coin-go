package ethapi

// A transaction's public key is carried in its `r` field as a big.Int, so any
// leading zero byte of the key is lost on the way back out: Bytes() renders a
// magnitude, not a fixed-width buffer. eth_getTransactionSignature has to put
// those bytes back before parsing, or every key that happens to begin with 0x00
// is rejected as the wrong length even though the chain holds it correctly.
//
// This was not hypothetical -- roughly 2% of transactions on a devnet failed
// this way, all with "packed public key must be of 1408 bytes".

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/circl/sign/hybridedmldsaslhdsa"
	"github.com/quantumcoinproject/circl/sign/hybridedmldsaslhdsa5"
	"github.com/quantumcoinproject/circl/sign/hybrideds"
)

func TestHybridPublicKeySize(t *testing.T) {
	cases := map[byte]int{
		hybrideds.DILITHIUM_ED25519_SPHINCS_COMPACT_ID:      hybrideds.PublicKeySize,
		hybrideds.DILITHIUM_ED25519_SPHINCS_FULL_ID:         hybrideds.PublicKeySize,
		hybridedmldsaslhdsa.ED25519_MLDSA_SLHDSA_COMPACT_ID: hybridedmldsaslhdsa.PublicKeySize,
		hybridedmldsaslhdsa.ED25519_MLDSA_SLHDSA_FULL_ID:    hybridedmldsaslhdsa.PublicKeySize,
		hybridedmldsaslhdsa5.ED25519_MLDSA5_SLHDSA5_FULL_ID: hybridedmldsaslhdsa5.PublicKeySize,
	}
	for id, want := range cases {
		if got := hybridPublicKeySize(id); got != want {
			t.Errorf("hybridPublicKeySize(%d) = %d, want %d", id, got, want)
		}
	}
	// An unknown scheme must report 0 so the key is passed through untouched
	// and ParseHybrid still answers ErrNotHybrid rather than being handed a
	// key this code silently reshaped.
	for _, id := range []byte{0, 6, 200, 255} {
		if got := hybridPublicKeySize(id); got != 0 {
			t.Errorf("hybridPublicKeySize(%d) = %d, want 0 for an unknown scheme", id, got)
		}
	}
}

// The regression itself: a key with leading zero bytes must survive the big.Int
// round trip that RawSignatureValues performs.
func TestPadHybridPublicKeyRestoresLeadingZeros(t *testing.T) {
	schemeID := hybrideds.DILITHIUM_ED25519_SPHINCS_COMPACT_ID
	signature := []byte{schemeID, 0x00}

	original := make([]byte, hybrideds.PublicKeySize)
	for i := range original {
		original[i] = byte(i%251) + 1 // no zeros except the ones set below
	}
	original[0], original[1] = 0x00, 0x00 // the case that broke

	// This is exactly what the transaction does to it.
	roundTripped := new(big.Int).SetBytes(original).Bytes()
	if len(roundTripped) != len(original)-2 {
		t.Fatalf("expected big.Int to drop 2 leading zeros, got %d bytes from %d",
			len(roundTripped), len(original))
	}

	restored := padHybridPublicKey(roundTripped, signature)
	if len(restored) != hybrideds.PublicKeySize {
		t.Fatalf("padded length = %d, want %d", len(restored), hybrideds.PublicKeySize)
	}
	if !bytes.Equal(restored, original) {
		t.Error("padded key does not match the original key")
	}
}

// A key that is already the right length must be handed through byte for byte.
func TestPadHybridPublicKeyLeavesExactLengthAlone(t *testing.T) {
	signature := []byte{hybridedmldsaslhdsa.ED25519_MLDSA_SLHDSA_COMPACT_ID, 0x00}
	key := bytes.Repeat([]byte{0x7f}, hybridedmldsaslhdsa.PublicKeySize)
	got := padHybridPublicKey(key, signature)
	if !bytes.Equal(got, key) {
		t.Error("an exact-length key was modified")
	}
}

// An over-long key is left for the parser to reject. Truncating it here would
// hide a genuine problem behind a key this code invented.
func TestPadHybridPublicKeyDoesNotTruncate(t *testing.T) {
	signature := []byte{hybrideds.DILITHIUM_ED25519_SPHINCS_COMPACT_ID, 0x00}
	key := bytes.Repeat([]byte{0x01}, hybrideds.PublicKeySize+16)
	got := padHybridPublicKey(key, signature)
	if len(got) != len(key) {
		t.Errorf("over-long key was resized to %d; it must be left for the parser", len(got))
	}
}

// An unknown scheme, or no signature at all, must not reshape anything.
func TestPadHybridPublicKeyPassesThroughUnknown(t *testing.T) {
	key := []byte{0x01, 0x02, 0x03}
	if got := padHybridPublicKey(key, []byte{0xff}); !bytes.Equal(got, key) {
		t.Error("unknown scheme id should pass the key through untouched")
	}
	if got := padHybridPublicKey(key, nil); !bytes.Equal(got, key) {
		t.Error("empty signature should pass the key through untouched")
	}
}
