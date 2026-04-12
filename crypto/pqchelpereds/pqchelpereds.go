// Package pqchelpereds provides helpers for hybrid post-quantum (PQC) signatures used by QuantumCoin.
// Wraps hybrid Ed25519 + NIST Dilithium + SPHINCS+ (compact) for key generation, signing, and verification.
// PQC is used in hybrid mode: classical + NIST-standardized post-quantum for quantum resistance.
package pqchelpereds

import (
	"crypto/rand"
	"errors"
	"github.com/quantumcoinproject/circl/sign/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const (
	CRYPTO_MESSAGE_LEN             = 32
	// CRYPTO_COMPACT_SIGNATURE_BYTES: hybrid PQC compact sig size (Ed25519 + NIST Dilithium + SPHINCS+).
	CRYPTO_COMPACT_SIGNATURE_BYTES = 2 + 64 + 2420 + 40 + CRYPTO_MESSAGE_LEN //2558
)

var (
	ErrInvalidSignatureLen  = errors.New("invalid signature length")
	ErrInvalidPublicKeyLen  = errors.New("invalid public key length")
	ErrInvalidPrivateKeyLen = errors.New("invalid private key length")
	ErrVerifyFailed         = errors.New("verify failed")
	ErrInvalidSeed          = errors.New("invalid seed length")
)

func GenerateKeyFromPreExpansionSeed(preExpansionSeed []byte) (publicKey []byte, secretKey []byte, err error) {
	if len(preExpansionSeed) != hybrideds.BaseSeedSize {
		return nil, nil, ErrInvalidSeed
	}
	var baseSeed [hybrideds.BaseSeedSize]byte
	copy(baseSeed[:], preExpansionSeed)

	expandedSeed, err := hybrideds.ExpandSeed(baseSeed)
	if err != nil {
		return nil, nil, err
	}
	return GenerateKeyWithSeed(expandedSeed[:])
}

func GenerateKeyWithSeed(seed []byte) (publicKey []byte, secretKey []byte, err error) {
	if len(seed) != hybrideds.SeedSize {
		return nil, nil, ErrInvalidSeed
	}
	var seedAlt [hybrideds.SeedSize]byte
	copy(seedAlt[:], seed)

	pub, pri, err := hybrideds.NewKeyFromSeed(&seedAlt)
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
	pub, pri, err := hybrideds.GenerateKey(rand.Reader)
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
	pri, err := hybrideds.UnmarshalPrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	signature, err := hybrideds.Sign(pri, rand.Reader, message)
	if err != nil {
		return nil, err
	}

	return signature[:], nil
}

func Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	pub, err := hybrideds.UnmarshalPublicKey(pubKey)
	if err != nil {
		return false
	}
	return hybrideds.Verify(pub, digestHash, signature)
}

func SignCompact(secretKey []byte, message []byte) ([]byte, error) {
	pri, err := hybrideds.UnmarshalPrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	signature, err := hybrideds.SignCompact(pri, rand.Reader, message)
	if err != nil {
		return nil, err
	}

	return signature[:], nil
}

func VerifyCompact(pubKey []byte, digestHash []byte, signature []byte) bool {
	pub, err := hybrideds.UnmarshalPublicKey(pubKey)
	if err != nil {
		return false
	}
	return hybrideds.VerifyCompact(pub, digestHash, signature)
}

func PublicKeyBytesFromSignatureCompact(digestHash []byte, sig []byte) ([]byte, error) {
	signature, pubKeyBytes, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}
	if len(signature) != CRYPTO_COMPACT_SIGNATURE_BYTES {
		log.Debug("PublicKeyBytesFromSignatureCompact", "len signature", len(signature), "CRYPTO_COMPACT_SIGNATURE_BYTES", CRYPTO_COMPACT_SIGNATURE_BYTES)
		return nil, ErrInvalidSignatureLen
	}

	ok := VerifyCompact(pubKeyBytes, digestHash, signature)
	if ok == false {
		return nil, ErrVerifyFailed
	}

	return pubKeyBytes, nil
}

func PrivateAndPublicFromPrivateKey(compositePrivateKey []byte) (privateBytes []byte, publicBytes []byte, err error) {
	pri, err := hybrideds.UnmarshalPrivateKey(compositePrivateKey)
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
	if len(sigBytes) < CRYPTO_COMPACT_SIGNATURE_BYTES {
		return nil, errors.New("invalid signature length")
	}

	if len(pubKeyBytes) != hybrideds.PublicKeySize {
		return nil, errors.New("invalid public key length")
	}

	return common.CombineTwoParts(sigBytes, pubKeyBytes), nil
}
