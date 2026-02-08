package tls

import (
	"bytes"
	"crypto"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
)

func TestHybridSigner_GenerateAndSign(t *testing.T) {
	pub, secret, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secret)
	if err != nil {
		t.Fatal(err)
	}
	// Public() should match generated public key
	pk := signer.Public().(*PublicKey)
	if !bytes.Equal(pk.Bytes, pub) {
		t.Error("Public() bytes != GenerateKey public")
	}
	// Use 32-byte message (e.g. SHA-256 digest) as expected by the hybrid scheme in practice
	msg := make([]byte, 32)
	for i := range msg {
		msg[i] = byte(i)
	}
	sig, err := signer.Sign(nil, msg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != SignatureSize() {
		t.Errorf("signature length = %d, want %d", len(sig), SignatureSize())
	}
	if !Verify(pk, msg, sig) {
		t.Error("Verify failed after Sign")
	}
	// Wrong message
	if Verify(pk, []byte("other"), sig) {
		t.Error("Verify should fail for wrong message")
	}
}

func TestHybridSigner_ImplementsSigner(t *testing.T) {
	_, secret, _ := pqchelpereddsamldsaslhdsa.GenerateKey()
	signer, _ := NewHybridSigner(secret)
	var _ crypto.Signer = signer
}

func TestPublicPrivateKeySizes(t *testing.T) {
	pub, secret, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != PublicKeySize() {
		t.Errorf("public key size = %d, want %d", len(pub), PublicKeySize())
	}
	if len(secret) != PrivateKeySize() {
		t.Errorf("private key size = %d, want %d", len(secret), PrivateKeySize())
	}
}
