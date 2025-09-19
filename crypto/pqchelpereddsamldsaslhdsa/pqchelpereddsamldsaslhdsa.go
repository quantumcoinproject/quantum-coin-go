package pqchelpereddsamldsaslhdsa

import (
	"crypto/rand"
	"errors"
	"github.com/quantumcoinproject/circl/sign/hybridedmldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const (
	CRYPTO_MESSAGE_LEN             = 32
	CRYPTO_COMPACT_SIGNATURE_BYTES = 2 + 64 + 2420 + CRYPTO_MESSAGE_LEN //2558
)

var (
	ErrInvalidSignatureLen  = errors.New("invalid signature length")
	ErrInvalidPublicKeyLen  = errors.New("invalid public key length")
	ErrInvalidPrivateKeyLen = errors.New("invalid private key length")
	ErrVerifyFailed         = errors.New("verify failed")
	ErrInvalidSeed          = errors.New("invalid seed length")
)

func GenerateKeyWithSeed(seed []byte) (publicKey []byte, secretKey []byte, err error) {
	if len(seed) != hybridedmldsaslhdsa.SeedSize {
		return nil, nil, ErrInvalidSeed
	}
	var seedAlt [hybridedmldsaslhdsa.SeedSize]byte
	copy(seedAlt[:], seed)

	pub, pri, err := hybridedmldsaslhdsa.NewKeyFromSeed(&seedAlt)
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
	pub, pri, err := hybridedmldsaslhdsa.GenerateKey(rand.Reader)
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
	pri, err := hybridedmldsaslhdsa.UnmarshalPrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	signature, err := hybridedmldsaslhdsa.Sign(pri, rand.Reader, message)
	if err != nil {
		return nil, err
	}

	return signature[:], nil
}

func Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	pub, err := hybridedmldsaslhdsa.UnmarshalPublicKey(pubKey)
	if err != nil {
		return false
	}
	return hybridedmldsaslhdsa.Verify(pub, digestHash, signature)
}

func SignCompact(secretKey []byte, message []byte) ([]byte, error) {
	pri, err := hybridedmldsaslhdsa.UnmarshalPrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	signature, err := hybridedmldsaslhdsa.SignCompact(pri, rand.Reader, message)
	if err != nil {
		return nil, err
	}

	return signature[:], nil
}

func VerifyCompact(pubKey []byte, digestHash []byte, signature []byte) bool {
	pub, err := hybridedmldsaslhdsa.UnmarshalPublicKey(pubKey)
	if err != nil {
		return false
	}
	return hybridedmldsaslhdsa.VerifyCompact(pub, digestHash, signature)
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
	pri, err := hybridedmldsaslhdsa.UnmarshalPrivateKey(compositePrivateKey)
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

	if len(pubKeyBytes) != hybridedmldsaslhdsa.PublicKeySize {
		return nil, errors.New("invalid public key length")
	}

	return common.CombineTwoParts(sigBytes, pubKeyBytes), nil
}
