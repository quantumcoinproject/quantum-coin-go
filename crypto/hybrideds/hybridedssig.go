// Package hybrideds implements hybrid post-quantum (PQC) signatures for QuantumCoin.
// It combines classical Ed25519 with NIST-standardized PQC (Dilithium + SPHINCS+) in hybrid mode,
// providing security against both classical and quantum adversaries. Compact signature variant.
// Dilithium/SPHINCS+ are part of the NIST PQC standardization track (standardized as ML-DSA/SLH-DSA).
package hybrideds

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"

	"github.com/quantumcoinproject/circl/sign/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybridedsfull"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereds"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

// CRYPTO_SIGNATURE_BYTES is the size of the hybrid PQC compact signature (Ed25519 + NIST PQC Dilithium/SPHINCS+).
const CRYPTO_SIGNATURE_BYTES = hybrideds.CompactSigLength

const PreExpansionSeedSize = hybrideds.BaseSeedSize

// HybridedsSig implements hybrid post-quantum signatures: classical Ed25519 + NIST PQC in hybrid mode.
type HybridedsSig struct {
	sigName                      string
	publicKeyLength              int
	privateKeyLength             int
	signatureLength              int
	signatureWithPublicKeyLength int
	fullSigAlg                   *hybridedsfull.HybridedsfullSig
}

func CreateHybridedsSig() HybridedsSig {
	fullSigAlg := hybridedsfull.CreateHybridedsfullSig()

	return HybridedsSig{sigName: "hybrideds",
		publicKeyLength:              hybrideds.PublicKeySize,
		privateKeyLength:             hybrideds.PrivateKeySize,
		signatureLength:              hybrideds.CompactSigLength,
		signatureWithPublicKeyLength: hybrideds.PublicKeySize + hybrideds.CompactSigLength + common.LengthByteSize + common.LengthByteSize,
		fullSigAlg:                   &fullSigAlg,
	}
}

func (s HybridedsSig) SignatureName() string {
	return s.sigName
}

func (s HybridedsSig) GetSigAlgType() crypto.SignatureAlgorithmType {
	return crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID
}

func (s HybridedsSig) PublicKeyLength() int {
	return s.publicKeyLength
}

func (s HybridedsSig) PrivateKeyLength() int {
	return s.privateKeyLength
}

func (s HybridedsSig) SignatureLength() int {
	return s.signatureLength
}

func (s HybridedsSig) SignatureWithPublicKeyLength() int {
	return s.signatureWithPublicKeyLength
}

func (s HybridedsSig) GenerateKey() (*signaturealgorithm.PrivateKey, error) {
	pubKey, priKey, err := pqchelpereds.GenerateKey()
	if err != nil {
		return nil, err
	}

	if len(pubKey) != s.publicKeyLength || len(priKey) != s.privateKeyLength {
		panic("keygen basic check failed")
	}

	privy := new(signaturealgorithm.PrivateKey)
	privy.PriData = make([]byte, len(priKey))
	copy(privy.PriData, priKey)

	privy.PublicKey.PubData = make([]byte, len(pubKey))
	copy(privy.PublicKey.PubData, pubKey)

	return privy, nil
}

func (s HybridedsSig) GenerateKeyWithReader(rand io.Reader) (*signaturealgorithm.PrivateKey, error) {
	// first step is to create a slice of bytes with the desired length
	seedBuf := make([]byte, hybrideds.SeedSize)
	// then we can call rand.Read.
	n, err := rand.Read(seedBuf)
	if err != nil {
		return nil, err
	}
	if n < hybrideds.SeedSize {
		return nil, errors.New("n less than SEED_SIZE")
	}
	return s.GenerateKeyWithSeed(seedBuf[:])
}

func (s HybridedsSig) GetRequiredSeedLength() uint {
	return hybrideds.SeedSize
}

func (s HybridedsSig) PreExpansionSeedSize() int {
	return PreExpansionSeedSize
}

func (s HybridedsSig) GenerateKeyFromPreExpansionSeed(preExpansionSeed []byte) (*signaturealgorithm.PrivateKey, error) {
	pubKey, priKey, err := pqchelpereds.GenerateKeyFromPreExpansionSeed(preExpansionSeed)
	if err != nil {
		return nil, err
	}

	privy := new(signaturealgorithm.PrivateKey)
	privy.PriData = make([]byte, len(priKey))
	copy(privy.PriData, priKey)

	privy.PublicKey.PubData = make([]byte, len(pubKey))
	copy(privy.PublicKey.PubData, pubKey)

	return privy, nil
}

func (s HybridedsSig) GenerateKeyWithSeed(seed []byte) (*signaturealgorithm.PrivateKey, error) {
	if len(seed) != hybrideds.SeedSize {
		return nil, errors.New("invalid seed size")
	}
	pubKey, priKey, err := pqchelpereds.GenerateKeyWithSeed(seed)
	if err != nil {
		return nil, err
	}

	if len(pubKey) != s.publicKeyLength || len(priKey) != s.privateKeyLength {
		panic("keygen basic check failed")
	}

	privy := new(signaturealgorithm.PrivateKey)
	privy.PriData = make([]byte, len(priKey))
	copy(privy.PriData, priKey)

	privy.PublicKey.PubData = make([]byte, len(pubKey))
	copy(privy.PublicKey.PubData, pubKey)

	return privy, nil
}

func (s HybridedsSig) SerializePrivateKey(priv *signaturealgorithm.PrivateKey) ([]byte, error) {
	priBytes, err := s.exportPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	return priBytes, err
}

func (s HybridedsSig) DeserializePrivateKey(priv []byte) (*signaturealgorithm.PrivateKey, error) {

	privKeyBytes, pubKeyBytes, err := pqchelpereds.PrivateAndPublicFromPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	privKey, err := s.convertBytesToPrivate(privKeyBytes)
	if err != nil {
		return nil, err
	}

	pubkey, err := s.convertBytesToPublic(pubKeyBytes)
	if err != nil {
		return nil, err
	}

	privKey.PublicKey = *pubkey

	return privKey, err
}

func (s HybridedsSig) SerializePublicKey(pub *signaturealgorithm.PublicKey) ([]byte, error) {
	return s.exportPublicKey(pub)
}

func (s HybridedsSig) DeserializePublicKey(pub []byte) (*signaturealgorithm.PublicKey, error) {
	pubKey, error := s.convertBytesToPublic(pub)
	return pubKey, error
}

func (s HybridedsSig) HexToPrivateKey(hexkey string) (*signaturealgorithm.PrivateKey, error) {
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return nil, err
	}

	if byteErr, ok := err.(hex.InvalidByteError); ok {
		return nil, fmt.Errorf("invalid hex character %q in private key", byte(byteErr))
	} else if err != nil {
		return nil, errors.New("invalid hex data for private key")
	}
	return s.DeserializePrivateKey(b)
}

func (s HybridedsSig) HexToPrivateKeyNoError(hexkey string) *signaturealgorithm.PrivateKey {
	p, err := s.HexToPrivateKey(hexkey)
	if err != nil {
		panic("HexToPrivateKey")
	}
	return p
}

func (s HybridedsSig) PrivateKeyToHex(priv *signaturealgorithm.PrivateKey) (string, error) {
	data, err := s.SerializePrivateKey(priv)
	if err != nil {
		return "", err
	}
	k := hex.EncodeToString(data)
	return k, nil
}

func (s HybridedsSig) PublicKeyToHex(pub *signaturealgorithm.PublicKey) (string, error) {
	data, err := s.SerializePublicKey(pub)
	if err != nil {
		return "", err
	}
	k := hex.EncodeToString(data)
	return k, nil
}

func (s HybridedsSig) HexToPublicKey(hexkey string) (*signaturealgorithm.PublicKey, error) {
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return nil, err
	}

	if byteErr, ok := err.(hex.InvalidByteError); ok {
		return nil, fmt.Errorf("invalid hex character %q in private key", byte(byteErr))
	} else if err != nil {
		return nil, errors.New("invalid hex data for private key")
	}
	return s.DeserializePublicKey(b)
}

func (s HybridedsSig) LoadPrivateKeyFromFile(file string) (*signaturealgorithm.PrivateKey, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	r := bufio.NewReader(fd)
	buf := make([]byte, (s.privateKeyLength)*2)
	n, err := readASCII(buf, r)
	if err != nil {
		return nil, err
	} else if n != len(buf) {
		return nil, fmt.Errorf("key file too short, want oqs hex character")
	}
	if err := checkKeyFileEnd(r); err != nil {
		return nil, err
	}
	return s.HexToPrivateKey(string(buf))
}

func (s HybridedsSig) SavePrivateKeyToFile(file string, key *signaturealgorithm.PrivateKey) error {
	k, err := s.PrivateKeyToHex(key)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, []byte(k), 0600)
}

func (s HybridedsSig) PublicKeyToAddress(p *signaturealgorithm.PublicKey) (common.Address, error) {
	pubBytes, err := s.SerializePublicKey(p)
	tempAddr := common.Address{}
	if err != nil {
		return tempAddr, err
	}
	return crypto.PublicKeyBytesToAddress(pubBytes), nil
}

func (s HybridedsSig) PublicKeyToAddressNoError(p *signaturealgorithm.PublicKey) common.Address {
	addr, err := s.PublicKeyToAddress(p)
	if err != nil {
		panic("PublicKeyToAddress failed")
	}
	return addr
}

func (s HybridedsSig) Sign(digestHash []byte, prv *signaturealgorithm.PrivateKey) (sig []byte, err error) {
	seckey, err := s.exportPrivateKey(prv)
	if err != nil {
		return nil, err
	}

	sigBytes, err := pqchelpereds.SignCompact(seckey, digestHash)
	if err != nil {
		return nil, err
	}

	pubBytes, err := s.SerializePublicKey(&prv.PublicKey)
	if err != nil {
		return nil, err
	}

	combinedSignature := common.CombineTwoParts(sigBytes, pubBytes)

	if !s.Verify(pubBytes, digestHash, combinedSignature) {
		return nil, errors.New("Verify failed after signing")
	}

	return combinedSignature, nil
}

func (s HybridedsSig) SignWithContext(digestHash []byte, prv *signaturealgorithm.PrivateKey, context []byte) (sig []byte, err error) {
	return s.fullSigAlg.SignWithContext(digestHash, prv, context)
}

func (s HybridedsSig) VerifyWithContext(pubKey []byte, digestHash []byte, signature []byte, context []byte) bool {
	return s.fullSigAlg.VerifyWithContext(pubKey, digestHash, signature, context)
}

func (s HybridedsSig) Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	sigBytes, pubKeyBytes, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false
	}

	if !bytes.Equal(pubKey, pubKeyBytes) {
		return false
	}
	return pqchelpereds.VerifyCompact(pubKey, digestHash, sigBytes)
}

func (s HybridedsSig) PublicKeyAndSignatureFromCombinedSignature(digestHash []byte, sig []byte) (signature []byte, pubKey []byte, err error) {
	signature, pubKey, err = common.ExtractTwoParts(sig)
	if err != nil {
		return nil, nil, err
	}

	ok := pqchelpereds.VerifyCompact(pubKey, digestHash, signature)

	if ok == false {
		return nil, nil, pqchelpereds.ErrVerifyFailed
	}

	return signature, pubKey, nil
}

func (s HybridedsSig) PublicKeyAndSignatureFromCombinedSignatureWithContext(digestHash []byte, sig []byte, context []byte) (signature []byte, pubKey []byte, digest []byte, err error) {
	return s.fullSigAlg.PublicKeyAndSignatureFromCombinedSignatureWithContext(digestHash, sig, context)
}

func (s HybridedsSig) CombinePublicKeySignature(sigBytes []byte, pubKeyBytes []byte) (combinedSignature []byte, err error) {
	if len(sigBytes) < s.signatureLength {
		log.Debug("HybridedsSig CombinePublicKeySignature", "sigbytes len", len(sigBytes), "signatureLength", s.signatureLength)
		return nil, pqchelpereds.ErrInvalidSignatureLen
	}

	if len(pubKeyBytes) != s.publicKeyLength {
		return nil, errors.New("invalid public key length")
	}

	return common.CombineTwoParts(sigBytes, pubKeyBytes), nil
}

func (s HybridedsSig) PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
	return pqchelpereds.PublicKeyBytesFromSignatureCompact(digestHash, sig)
}

func (s HybridedsSig) PublicKeyFromSignature(digestHash []byte, sig []byte) (*signaturealgorithm.PublicKey, error) {
	b, err := s.PublicKeyBytesFromSignature(digestHash, sig)
	if err != nil {
		return nil, err
	}
	return s.DeserializePublicKey(b)
}

func (s HybridedsSig) GetAddress(digestHash []byte, sig []byte) (common.Address, error) {
	pubKeyBytes, err := s.PublicKeyBytesFromSignature(digestHash[:], sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(pubKeyBytes) != 0 && len(pubKeyBytes) != s.PublicKeyLength() {
		return common.Address{}, errors.New("invalid public key")
	}
	var addr common.Address
	addr.CopyFrom(crypto.PublicKeyBytesToAddress(pubKeyBytes[:]))
	return addr, nil
}

func (s HybridedsSig) PublicKeyFromSignatureWithContext(digestHash []byte, sig []byte, context []byte) (*signaturealgorithm.PublicKey, error) {
	return s.fullSigAlg.PublicKeyFromSignatureWithContext(digestHash, sig, context)
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func (osig HybridedsSig) ValidateSignatureValues(digestHash []byte, v byte, r, s *big.Int) (isOk bool, pub []byte, sig []byte) {
	if v != 1 {
		return false, nil, nil
	}
	pubKey, signature := r.Bytes(), s.Bytes()

	if len(pubKey) != osig.PublicKeyLength() {
		if len(pubKey) > osig.PublicKeyLength() {
			return false, nil, nil
		}
		zeroBuff := make([]byte, osig.PublicKeyLength()-len(pubKey))
		pubKey = append(zeroBuff, pubKey...)
	}

	if len(signature) < osig.SignatureLength() {
		return false, nil, nil
	}

	combinedSignature := common.CombineTwoParts(signature, pubKey)
	if !osig.Verify(pubKey, digestHash, combinedSignature) {
		return false, nil, nil
	}

	return true, pubKey, signature
}

func (s HybridedsSig) PublicKeyStartValue() byte {
	return 0x00 + 9
}

func (s HybridedsSig) SignatureStartValue() byte {
	return 0x30 + 9
}

func (s HybridedsSig) Zeroize(prv *signaturealgorithm.PrivateKey) {
	b := prv.PriData
	for i := range b {
		b[i] = 0
	}
}

func (s HybridedsSig) EncodePublicKey(pubKey *signaturealgorithm.PublicKey) []byte {
	encoded := make([]byte, s.publicKeyLength)
	copy(encoded, pubKey.PubData)
	return encoded
}

func (s HybridedsSig) DecodePublicKey(encoded []byte) (*signaturealgorithm.PublicKey, error) {
	if len(encoded) != s.publicKeyLength {
		return nil, errors.New("wrong size public key data")
	}
	p := &signaturealgorithm.PublicKey{}
	p.PubData = make([]byte, s.publicKeyLength)
	copy(p.PubData, encoded)
	return p, nil
}

// readASCII reads into 'buf', stopping when the buffer is full or
// when a non-printable control character is encountered.
func readASCII(buf []byte, r *bufio.Reader) (n int, err error) {
	for ; n < len(buf); n++ {
		buf[n], err = r.ReadByte()
		switch {
		case err == io.EOF || buf[n] < '!':
			return n, nil
		case err != nil:
			return n, err
		}
	}
	return n, nil
}

// checkKeyFileEnd skips over additional newlines at the end of a key file.
func checkKeyFileEnd(r *bufio.Reader) error {
	for i := 0; ; i++ {
		b, err := r.ReadByte()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case b != '\n' && b != '\r':
			return fmt.Errorf("invalid character %q at end of key file", b)
		case i >= 2:
			return errors.New("key file too long, want 64 hex characters")
		}
	}
}

// convertBytesToPrivate exports the corresponding secret key from the sig receiver.
func (s HybridedsSig) convertBytesToPrivate(privy []byte) (*signaturealgorithm.PrivateKey, error) {
	if len(privy) != s.privateKeyLength {
		return nil, pqchelpereds.ErrInvalidPrivateKeyLen
	}
	privKey := new(signaturealgorithm.PrivateKey)
	privKey.PriData = make([]byte, s.privateKeyLength)
	copy(privKey.PriData, privy)

	return privKey, nil
}

// convertBytesToPublic exports the corresponding secret key from the sig receiver.
func (s HybridedsSig) convertBytesToPublic(pub []byte) (*signaturealgorithm.PublicKey, error) {
	if len(pub) != s.publicKeyLength {
		return nil, pqchelpereds.ErrInvalidPublicKeyLen
	}
	pubKey := new(signaturealgorithm.PublicKey)
	pubKey.PubData = make([]byte, s.publicKeyLength)
	copy(pubKey.PubData, pub)
	return pubKey, nil
}

// exportPrivateKey exports a private key into a binary dump.
func (s HybridedsSig) exportPrivateKey(privy *signaturealgorithm.PrivateKey) ([]byte, error) {
	if len(privy.PriData) != s.privateKeyLength {
		return nil, pqchelpereds.ErrInvalidPrivateKeyLen
	}

	buf := make([]byte, s.privateKeyLength)
	copy(buf, privy.PriData)
	return buf, nil
}

// exportPublicKey exports a public key into a binary dump.
func (s HybridedsSig) exportPublicKey(pub *signaturealgorithm.PublicKey) ([]byte, error) {
	if len(pub.PubData) != s.publicKeyLength {
		return nil, pqchelpereds.ErrInvalidPublicKeyLen
	}
	buf := make([]byte, s.publicKeyLength)
	copy(buf, pub.PubData)
	return buf, nil
}
