// Package tls provides TLS 1.3 support with hybrid PQC (Ed25519+ML-DSA+SLH-DSA) certificates.
// It implements crypto.Signer for use with X.509 and extends TLS to support PQC certificate authentication.
package tls

import (
	"crypto"
	"io"

	"github.com/quantumcoinproject/circl/sign/hybridedmldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
)

// PublicKey is the public key for the hybrid PQC scheme (Ed25519 + ML-DSA + SLH-DSA).
// It implements crypto.PublicKey and holds the raw public key bytes.
type PublicKey struct {
	Bytes []byte
}

// HybridSigner implements crypto.Signer using pqchelpereddsamldsaslhdsa (hybrid Ed25519+ML-DSA+SLH-DSA).
// It can be used to sign X.509 certificates and TLS CertificateVerify messages.
type HybridSigner struct {
	secretKey []byte
	publicKey  *PublicKey
}

// NewHybridSigner creates a HybridSigner from raw secret key bytes (from pqchelpereddsamldsaslhdsa.GenerateKey).
// The secretKey is retained by the signer; the caller must not modify it.
func NewHybridSigner(secretKey []byte) (*HybridSigner, error) {
	_, pubBytes, err := pqchelpereddsamldsaslhdsa.PrivateAndPublicFromPrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	return &HybridSigner{
		secretKey: append([]byte(nil), secretKey...),
		publicKey: &PublicKey{Bytes: pubBytes},
	}, nil
}

// Public implements crypto.Signer.
func (s *HybridSigner) Public() crypto.PublicKey {
	return s.publicKey
}

// Sign implements crypto.Signer. It signs digest with the hybrid PQC key (compact format).
// opts are ignored; the hybrid scheme signs the message directly (no hash wrapper).
func (s *HybridSigner) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) (signature []byte, err error) {
	return pqchelpereddsamldsaslhdsa.SignCompact(s.secretKey, digest)
}

// Verify verifies a compact signature over digest using the given hybrid public key.
func Verify(pub *PublicKey, digest, signature []byte) bool {
	return pqchelpereddsamldsaslhdsa.VerifyCompact(pub.Bytes, digest, signature)
}

// PublicKeySize returns the length in bytes of a hybrid PQC public key.
func PublicKeySize() int {
	return hybridedmldsaslhdsa.PublicKeySize
}

// PrivateKeySize returns the length in bytes of a hybrid PQC private key.
func PrivateKeySize() int {
	return hybridedmldsaslhdsa.PrivateKeySize
}

// SignatureSize returns the length in bytes of a hybrid PQC compact signature.
func SignatureSize() int {
	return pqchelpereddsamldsaslhdsa.CRYPTO_COMPACT_SIGNATURE_BYTES
}
