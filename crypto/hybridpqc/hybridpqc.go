package hybridpqc

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"github.com/quantumcoinproject/circl/sign/mldsa/mldsa44"
	"github.com/quantumcoinproject/circl/sign/slhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"golang.org/x/crypto/sha3"
)

const CRYPTO_ED25519_PUBLICKEY_BYTES = 32
const CRYPTO_ED25519_SIGNATURE_BYTES = 64

const CRYPTO_DILITHIUM_PUBLICKEY_BYTES = 1312
const CRYPTO_DILITHIUM_SIGNATURE_BYTES = 2420

const CRYPTO_SPHINCS_PUBLICKEY_BYTES = 64
const NONCE_SIZE = 40

const CRYPTO_HYBRID_SIGNATURE_BYTES = 2 + 64 + 2420 + 40 //+MESSAGE_LEN
const SIGNATURE_ID = 1

const CRYPTO_SECRETKEY_BYTES = 64 + 2560 + 1312 + 128
const CRYPTO_PUBLICKEY_BYTES = 32 + 1312 + 64

var (
	ErrVerifyFailed         = errors.New("verify failed")
	ErrInvalidPrivateKeyLen = errors.New("invalid private key length")
)

func VerifyDilithium(digestHash []byte, signature []byte, publicKey []byte) error {

	var pubKey mldsa44.PublicKey

	var buf2 [CRYPTO_DILITHIUM_PUBLICKEY_BYTES]byte
	copy(buf2[:], publicKey)

	pubKey.Unpack(&buf2)

	if mldsa44.VerifyNoContext(&pubKey, digestHash, signature) == false {
		return errors.New("verify failed")
	}

	return nil
}

func VerifySphincs(digestHash []byte, signature []byte, publicKey []byte) error {

	var pubKey slhdsa.PublicKey
	pubKey.ID = slhdsa.SHAKE_256f

	var buf2 [CRYPTO_DILITHIUM_PUBLICKEY_BYTES]byte
	copy(buf2[:], publicKey)

	err := pubKey.UnmarshalBinary(publicKey)
	if err != nil {
		return err
	}

	if slhdsa.VerifyNoContext(&pubKey, digestHash, signature) == false {
		return errors.New("verify failed")
	}

	return nil
}

func VerifyHybridEds(pubKey []byte, digestHash []byte, signature []byte) bool {
	sigBytes, pubKeyBytes, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false
	}

	if !bytes.Equal(pubKey, pubKeyBytes) {
		return false
	}

	msgLen := len(digestHash)
	if msgLen <= 0 || msgLen > 255 {
		return false
	}

	if len(sigBytes) != CRYPTO_HYBRID_SIGNATURE_BYTES+msgLen {
		return false
	}

	if sigBytes[0] != SIGNATURE_ID {
		return false
	}

	if int(sigBytes[1]) != msgLen {
		return false
	}

	//Form the hybrid signature
	var hybridMsg [40 + 64 + 64]byte

	//Copy the nonce
	for i := 0; i < NONCE_SIZE; i++ {
		hybridMsg[i] = sigBytes[2+CRYPTO_ED25519_SIGNATURE_BYTES+CRYPTO_DILITHIUM_SIGNATURE_BYTES+i]
	}

	//Copy the original message
	for i := 0; i < msgLen; i++ {
		//This is an important check
		if sigBytes[2+CRYPTO_ED25519_SIGNATURE_BYTES+CRYPTO_DILITHIUM_SIGNATURE_BYTES+NONCE_SIZE+i] != digestHash[i] {
			return false
		}
		hybridMsg[NONCE_SIZE+i] = digestHash[i]
	}

	//Copy the SPHINCS public key
	for i := 0; i < CRYPTO_SPHINCS_PUBLICKEY_BYTES; i++ {
		hybridMsg[NONCE_SIZE+msgLen+i] = pubKey[CRYPTO_ED25519_PUBLICKEY_BYTES+CRYPTO_DILITHIUM_PUBLICKEY_BYTES+i]
	}

	//Hash the hybrid message
	hasher := sha3.New512()
	hasher.Write(hybridMsg[:NONCE_SIZE+msgLen+CRYPTO_SPHINCS_PUBLICKEY_BYTES])
	hybridDigest := hasher.Sum(nil)

	ed25519Signature := sigBytes[2 : 2+CRYPTO_ED25519_SIGNATURE_BYTES]
	ed25519PubKey := pubKey[:CRYPTO_ED25519_PUBLICKEY_BYTES]

	ok := ed25519.Verify(ed25519PubKey, hybridDigest, ed25519Signature)
	if ok == false {
		return false
	}

	dilithiumSignature := sigBytes[2+CRYPTO_ED25519_SIGNATURE_BYTES : 2+CRYPTO_ED25519_SIGNATURE_BYTES+CRYPTO_DILITHIUM_SIGNATURE_BYTES]
	dilithiumPubKey := pubKey[CRYPTO_ED25519_PUBLICKEY_BYTES : CRYPTO_ED25519_PUBLICKEY_BYTES+CRYPTO_DILITHIUM_PUBLICKEY_BYTES]

	err = VerifyDilithium(hybridDigest, dilithiumSignature, dilithiumPubKey)
	if err != nil {
		return false
	}

	return true
}

func PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
	_, pubKeyBytes, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}

	ok := VerifyHybridEds(pubKeyBytes, digestHash, sig)
	if ok == false {
		return nil, ErrVerifyFailed
	}

	return pubKeyBytes, nil
}

func PrivateAndPublicFromPrivateKey(compositePrivateKey []byte) (privateBytes []byte, publicBytes []byte, err error) {
	if len(compositePrivateKey) != CRYPTO_SECRETKEY_BYTES {
		return nil, nil, ErrInvalidPrivateKeyLen
	}

	pub1 := make([]byte, len(compositePrivateKey[32:64]))
	copy(pub1, compositePrivateKey[32:64])

	pub2 := make([]byte, len(compositePrivateKey[64+2560:64+2560+1312]))
	copy(pub2, compositePrivateKey[64+2560:64+2560+1312])

	pub3 := make([]byte, len(compositePrivateKey[64+2560+1312+64:]))
	copy(pub3, compositePrivateKey[64+2560+1312+64:])

	pubKeyBytes := make([]byte, CRYPTO_PUBLICKEY_BYTES)
	pubKeyBytes = append(pub1, pub2...)
	pubKeyBytes = append(pubKeyBytes, pub3...)

	return compositePrivateKey, pubKeyBytes, nil
}
