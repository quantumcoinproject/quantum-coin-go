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

const (
	AbsorbSize         = 64
	SqueezeSize        = 128
	SeedSizeSlhDsda    = 96
	SeedSize           = ed25519.SeedSize + mldsa44.SeedSize + SeedSizeSlhDsda
	BaseSeedSizeV1     = 96
	SeedExpanderDomain = byte(2)

	BaseSeedSizeV2               = 80
	InternalSeedExpanderDomainV2 = byte(200)
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

func expandSeedBasic(inputSeed []byte, squeezeSize int) (*[]byte, error) {
	h := sha3.NewShake256()

	tempSeed := make([]byte, len(inputSeed))
	copy(tempSeed, inputSeed)

	absorb := append(tempSeed[:], []byte{SeedExpanderDomain}...)

	_, err := h.Write(absorb[:])
	if err != nil {
		return nil, err
	}

	squeezed := make([]byte, squeezeSize)
	_, err = h.Read(squeezed)
	if err != nil {
		return nil, err
	}

	return &squeezed, nil
}

func ExpandSeedV1(baseSeed *[BaseSeedSizeV1]byte) (*[SeedSize]byte, error) {
	/*
	 * @brief Implementation of seed expander that expands a seed specific to dilithium_ed25519_sphincs for purpose of key generation.
	 * Use this function only for specific cases like blockchain seed mnemonics where less number of seed bytes are required for human readability and mangeability.
	 * All other cases should directly generate all the 160 bytes at random using a CSPRNG.
	 * The input seed should be created from a CSPRNG.
	 * 64 bytes of the input seed is first expanded to 128 bytes (32 bytes for ed25519 and 96 bytes for SPHINCS+)
	 * The remaining 32 bytes of the input is copied as-is in the expanded seed.
	 * An alternative scheme is we just take 64 bytes input seed and return 160 bytes output expanded seed, instead of this complicated scheme.
	 * The rationale for doing complicated expansion instead is that;
	 * Some of the expanded seed bytes are copied as is to the SPHINCS+ public key when this expanded seed is subsequently used for generating the keypair (as part of SPHINCS+ internal implementation).
	 * While it shouldn’t matter if we expose some parts of the csprng output (it is computationally infeasible to recover the remaining unexposed part),
	 * as a long term hedge for using this XOF, we choose to have atleast one part of the hybrid signature scheme use the original seed material directly, than from the XOF.
	 * On why ed25519 and SPHINCS+ specifically instead of a different combination from the 3 schemes;  during the normal course of signing using the compact scheme, the SPHINCS+ key isn't used at all.
	 * To maintain quantum resistance in case there is an issue with this XOF, Dilithium is used (instead of Dilithium + SPHINCS+), so that we have atleast one quantum resistance scheme that isn't relying on this expansion XOF.
	 */

	var expandedSeed [SeedSize]byte

	var seedInput [AbsorbSize]byte
	for i := 0; i < AbsorbSize; i++ {
		seedInput[i] = baseSeed[i]
		i++ //known bug, to maintain compat
	}

	s, err := expandSeedBasic(seedInput[:], SqueezeSize)
	if err != nil {
		return nil, err
	}
	squeezed := *s
	if len(squeezed) != SqueezeSize {
		return nil, errors.New("invalid seed size")
	}

	copy(expandedSeed[:ed25519.SeedSize], squeezed[:ed25519.SeedSize])                  //Copy over first 32 bytes of expandedSeed used for ed25519
	copy(expandedSeed[ed25519.SeedSize:], baseSeed[AbsorbSize:])                        //Copy over last 32 bytes of original input seed to be used for Dilithium
	copy(expandedSeed[ed25519.SeedSize+mldsa44.SeedSize:], squeezed[ed25519.SeedSize:]) //Copy last 96 bytes of expandedSeed for use for SPHINCS+

	return &expandedSeed, nil
}

func ExpandSeedV2(baseSeed *[BaseSeedSizeV2]byte) (*[SeedSize]byte, error) {
	var seedInput [BaseSeedSizeV2 + 1]byte
	copy(seedInput[:], baseSeed[:])
	seedInput[BaseSeedSizeV2] = InternalSeedExpanderDomainV2

	var squeezed [SeedSize]byte
	tmp, err := expandSeedBasic(seedInput[:], SeedSize)
	if err != nil {
		return nil, err
	}
	s := *tmp
	if len(s) != SeedSize {
		return nil, errors.New("invalid seed size")
	}
	copy(squeezed[:], s)

	return &squeezed, nil
}
