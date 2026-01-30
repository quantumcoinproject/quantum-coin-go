// Package pqchelpereddsamldsaslhdsa5 provides helpers for NIST ML-DSA (FIPS 204) + SLH-DSA (FIPS 205) hybrid PQC.
// Post-quantum cryptography in hybrid mode; level-5 parameter set.
package pqchelpereddsamldsaslhdsa5

import (
	"crypto/rand"
	"errors"

	"github.com/quantumcoinproject/circl/sign/hybridedmldsaslhdsa5"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const (
	CRYPTO_MESSAGE_LEN     = 32
	CRYPTO_SIGNATURE_BYTES = hybridedmldsaslhdsa5.SigLength
)

var (
	ErrInvalidSignatureLen  = errors.New("invalid signature length")
	ErrInvalidPublicKeyLen  = errors.New("invalid public key length")
	ErrInvalidPrivateKeyLen = errors.New("invalid private key length")
	ErrVerifyFailed         = errors.New("verify failed")
	ErrInvalidSeed          = errors.New("invalid seed length")
)

func GenerateKeyWithSeed(seed []byte) (publicKey []byte, secretKey []byte, err error) {
	if len(seed) != hybridedmldsaslhdsa5.SeedSize {
		return nil, nil, ErrInvalidSeed
	}
	var seedAlt [hybridedmldsaslhdsa5.SeedSize]byte
	copy(seedAlt[:], seed)

	pub, pri, err := hybridedmldsaslhdsa5.NewKeyFromSeed(&seedAlt)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err = pub.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	secretKey, err = pri.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	return publicKey, secretKey, nil
}

func GenerateKey() (publicKey []byte, secretKey []byte, err error) {
	pub, pri, err := hybridedmldsaslhdsa5.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err = pub.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	secretKey, err = pri.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	return publicKey, secretKey, nil
}

func Sign(secretKey []byte, message []byte) ([]byte, error) {
	pri, err := hybridedmldsaslhdsa5.UnmarshalPrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	signature, err := hybridedmldsaslhdsa5.Sign(pri, rand.Reader, message)
	if err != nil {
		return nil, err
	}

	return signature[:], nil
}

func Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	pub, err := hybridedmldsaslhdsa5.UnmarshalPublicKey(pubKey)
	if err != nil {
		return false
	}
	return hybridedmldsaslhdsa5.Verify(pub, digestHash, signature)
}

func PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
	signature, pubKeyBytes, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}
	if len(signature) != CRYPTO_SIGNATURE_BYTES {
		log.Debug("PublicKeyBytesFromSignature", "len signature", len(signature), "CRYPTO_SIGNATURE_BYTES", CRYPTO_SIGNATURE_BYTES)
		return nil, ErrInvalidSignatureLen
	}

	ok := Verify(pubKeyBytes, digestHash, signature)
	if ok == false {
		return nil, ErrVerifyFailed
	}

	return pubKeyBytes, nil
}

func PrivateAndPublicFromPrivateKey(compositePrivateKey []byte) (privateBytes []byte, publicBytes []byte, err error) {
	pri, err := hybridedmldsaslhdsa5.UnmarshalPrivateKey(compositePrivateKey)
	if err != nil {
		return nil, nil, err
	}

	privateBytes, err = pri.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	pub, err := pri.GetPublicKey()
	if err != nil {
		return nil, nil, err
	}

	publicBytes, err = pub.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	return privateBytes, publicBytes, nil
}

func CombinePublicKeySignature(sigBytes []byte, pubKeyBytes []byte) (combinedSignature []byte, err error) {
	if len(sigBytes) != hybridedmldsaslhdsa5.SigLength {
		return nil, errors.New("invalid signature length")
	}

	if len(pubKeyBytes) != hybridedmldsaslhdsa5.PublicKeySize {
		return nil, errors.New("invalid public key length")
	}

	return common.CombineTwoParts(sigBytes, pubKeyBytes), nil
}
