// Package hybridedsfull implements full-size hybrid post-quantum (PQC) signatures for QuantumCoin.
// Combines classical Ed25519 with NIST-standardized PQC (Dilithium + SPHINCS+) in hybrid mode.
// Full variant; use for contexts requiring maximum PQC assurance and NIST standard compliance.
// Dilithium/SPHINCS+ are part of the NIST PQC standardization track (standardized as ML-DSA/SLH-DSA).
package hybridedsfull

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
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereds"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

type HybridedsfullSig struct {
	sigName                      string
	publicKeyLength              int
	privateKeyLength             int
	signatureLength              int
	signatureWithPublicKeyLength int
}

func CreateHybridedsfullSig() HybridedsfullSig {
	return HybridedsfullSig{sigName: "hybridedsfull",
		publicKeyLength:              hybrideds.PublicKeySize,
		privateKeyLength:             hybrideds.PrivateKeySize,
		signatureLength:              hybrideds.SigLength,
		signatureWithPublicKeyLength: hybrideds.PublicKeySize + hybrideds.SigLength + common.LengthByteSize + common.LengthByteSize,
	}
}

func (s HybridedsfullSig) SignatureName() string {
	return s.sigName
}

func (s HybridedsfullSig) GetSigAlgType() crypto.SignatureAlgorithmType {
	return crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID
}

func (s HybridedsfullSig) PublicKeyLength() int {
	return s.publicKeyLength
}

func (s HybridedsfullSig) PrivateKeyLength() int {
	return s.privateKeyLength
}

func (s HybridedsfullSig) SignatureLength() int {
	return s.signatureLength
}

func (s HybridedsfullSig) SignatureWithPublicKeyLength() int {
	return s.signatureWithPublicKeyLength
}

func (s HybridedsfullSig) GenerateKey() (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridedsfullSig) GenerateKeyWithReader(rand io.Reader) (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridedsfullSig) GetRequiredSeedLength() uint {
	return hybrideds.SeedSize
}

func (s HybridedsfullSig) GenerateKeyWithSeed(seed []byte) (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridedsfullSig) SerializePrivateKey(priv *signaturealgorithm.PrivateKey) ([]byte, error) {
	priBytes, err := s.exportPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	return priBytes, err
}

func (s HybridedsfullSig) DeserializePrivateKey(priv []byte) (*signaturealgorithm.PrivateKey, error) {

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

func (s HybridedsfullSig) SerializePublicKey(pub *signaturealgorithm.PublicKey) ([]byte, error) {
	return s.exportPublicKey(pub)
}

func (s HybridedsfullSig) DeserializePublicKey(pub []byte) (*signaturealgorithm.PublicKey, error) {
	pubKey, error := s.convertBytesToPublic(pub)
	return pubKey, error
}

func (s HybridedsfullSig) HexToPrivateKey(hexkey string) (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridedsfullSig) HexToPrivateKeyNoError(hexkey string) *signaturealgorithm.PrivateKey {
	p, err := s.HexToPrivateKey(hexkey)
	if err != nil {
		panic("HexToPrivateKey")
	}
	return p
}

func (s HybridedsfullSig) PrivateKeyToHex(priv *signaturealgorithm.PrivateKey) (string, error) {
	data, err := s.SerializePrivateKey(priv)
	if err != nil {
		return "", err
	}
	k := hex.EncodeToString(data)
	return k, nil
}

func (s HybridedsfullSig) PublicKeyToHex(pub *signaturealgorithm.PublicKey) (string, error) {
	data, err := s.SerializePublicKey(pub)
	if err != nil {
		return "", err
	}
	k := hex.EncodeToString(data)
	return k, nil
}

func (s HybridedsfullSig) HexToPublicKey(hexkey string) (*signaturealgorithm.PublicKey, error) {
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

func (s HybridedsfullSig) LoadPrivateKeyFromFile(file string) (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridedsfullSig) SavePrivateKeyToFile(file string, key *signaturealgorithm.PrivateKey) error {
	k, err := s.PrivateKeyToHex(key)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, []byte(k), 0600)
}

func (s HybridedsfullSig) PublicKeyToAddress(p *signaturealgorithm.PublicKey) (common.Address, error) {
	pubBytes, err := s.SerializePublicKey(p)
	tempAddr := common.Address{}
	if err != nil {
		return tempAddr, err
	}
	return crypto.PublicKeyBytesToAddress(pubBytes), nil
}

func (s HybridedsfullSig) PublicKeyToAddressNoError(p *signaturealgorithm.PublicKey) common.Address {
	addr, err := s.PublicKeyToAddress(p)
	if err != nil {
		panic("PublicKeyToAddress failed")
	}
	return addr
}

func (s HybridedsfullSig) Sign(digestHash []byte, prv *signaturealgorithm.PrivateKey) (sig []byte, err error) {
	seckey, err := s.exportPrivateKey(prv)
	if err != nil {
		return nil, err
	}

	sigBytes, err := pqchelpereds.Sign(seckey, digestHash)
	if err != nil {
		return nil, err
	}

	pubBytes, err := s.SerializePublicKey(&prv.PublicKey)
	if err != nil {
		return nil, err
	}

	combinedSignature := common.CombineTwoParts(sigBytes, pubBytes)

	if !s.Verify(pubBytes, digestHash, combinedSignature) {
		return nil, errors.New("VerifyCompact failed after signing")
	}

	return combinedSignature, nil
}

func (s HybridedsfullSig) SignWithContext(digestHash []byte, prv *signaturealgorithm.PrivateKey, context []byte) (sig []byte, err error) {
	if context == nil || len(context) < 1 {
		return nil, errors.New("SignWithContext failed context")
	}

	if context[0] == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		newDigestHash := crypto.Keccak256(digestHash, context)
		return s.Sign(newDigestHash, prv)
	}

	return nil, errors.New("SignWithContext failed invalid context")
}

func (s HybridedsfullSig) Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	sigBytes, pubKeyBytes, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false
	}

	if !bytes.Equal(pubKey, pubKeyBytes) {
		return false
	}

	return pqchelpereds.Verify(pubKey, digestHash, sigBytes)
}

func (s HybridedsfullSig) VerifyWithContext(pubKey []byte, digestHash []byte, signature []byte, context []byte) bool {
	if context == nil || len(context) < 1 {
		return false
	}

	if context[0] == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		newDigestHash := crypto.Keccak256(digestHash, context)
		return s.Verify(pubKey, newDigestHash, signature)
	}

	return false
}

func (s HybridedsfullSig) PublicKeyAndSignatureFromCombinedSignature(digestHash []byte, sig []byte) (signature []byte, pubKey []byte, err error) {
	signature, pubKey, err = common.ExtractTwoParts(sig)
	if err != nil {
		return nil, nil, err
	}

	ok := s.Verify(pubKey, digestHash, sig)
	if ok == false {
		return nil, nil, errors.New("VerifyCompact failed")
	}

	return signature, pubKey, nil
}

func (s HybridedsfullSig) PublicKeyAndSignatureFromCombinedSignatureWithContext(digestHash []byte, sig []byte, context []byte) (signature []byte, pubKey []byte, digest []byte, err error) {
	signature, pubKey, err = common.ExtractTwoParts(sig)
	if err != nil {
		return nil, nil, nil, err
	}

	if context[0] == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		newDigestHash := crypto.Keccak256(digestHash, context)
		ok := s.Verify(pubKey, newDigestHash, sig)
		if ok == false {
			return nil, nil, nil, errors.New("Verify failed")
		}
		return signature, pubKey, newDigestHash, nil
	} else {
		ok := s.Verify(pubKey, digestHash, sig)
		if ok == false {
			return nil, nil, nil, errors.New("Verify failed")
		}
		return signature, pubKey, digestHash, nil
	}
}

func (s HybridedsfullSig) CombinePublicKeySignature(sigBytes []byte, pubKeyBytes []byte) (combinedSignature []byte, err error) {
	if len(sigBytes) < s.signatureLength {
		log.Debug("HybridedsfullSig CombinePublicKeySignature", "sigbytes len", len(sigBytes), "signatureLength", s.signatureLength)
		return nil, pqchelpereds.ErrInvalidSignatureLen
	}

	if len(pubKeyBytes) != s.publicKeyLength {
		return nil, errors.New("invalid public key length")
	}

	return common.CombineTwoParts(sigBytes, pubKeyBytes), nil
}

func (s HybridedsfullSig) PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
	_, pubKeyBytes, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}

	ok := s.Verify(pubKeyBytes, digestHash, sig)
	if ok == false {
		return nil, errors.New("verify failed")
	}

	return pubKeyBytes, nil
}

func (s HybridedsfullSig) PublicKeyFromSignature(digestHash []byte, sig []byte) (*signaturealgorithm.PublicKey, error) {
	b, err := s.PublicKeyBytesFromSignature(digestHash, sig)
	if err != nil {
		return nil, err
	}
	return s.DeserializePublicKey(b)
}

func (s HybridedsfullSig) GetAddress(digestHash []byte, sig []byte) (common.Address, error) {
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

func (s HybridedsfullSig) PublicKeyFromSignatureWithContext(digestHash []byte, sig []byte, context []byte) (*signaturealgorithm.PublicKey, error) {
	if len(context) < 1 {
		return nil, errors.New("invalid context")
	}
	if context[0] != byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		return nil, errors.New("invalid context")
	}

	_, pubKeyBytes, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}

	newDigestHash := crypto.Keccak256(digestHash, context)
	ok := s.Verify(pubKeyBytes, newDigestHash, sig)
	if ok == false {
		return nil, errors.New("verify failed")
	}

	return s.DeserializePublicKey(pubKeyBytes)
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func (osig HybridedsfullSig) ValidateSignatureValues(digestHash []byte, v byte, r, s *big.Int) (isOk bool, pub []byte, sig []byte) {
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

func (s HybridedsfullSig) PublicKeyStartValue() byte {
	return 0x00 + 9
}

func (s HybridedsfullSig) SignatureStartValue() byte {
	return 0x30 + 9
}

func (s HybridedsfullSig) Zeroize(prv *signaturealgorithm.PrivateKey) {
	b := prv.PriData
	for i := range b {
		b[i] = 0
	}
}

func (s HybridedsfullSig) EncodePublicKey(pubKey *signaturealgorithm.PublicKey) []byte {
	encoded := make([]byte, s.publicKeyLength)
	copy(encoded, pubKey.PubData)
	return encoded
}

func (s HybridedsfullSig) DecodePublicKey(encoded []byte) (*signaturealgorithm.PublicKey, error) {
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
func (s HybridedsfullSig) convertBytesToPrivate(privy []byte) (*signaturealgorithm.PrivateKey, error) {
	if len(privy) != s.privateKeyLength {
		return nil, pqchelpereds.ErrInvalidPrivateKeyLen
	}
	privKey := new(signaturealgorithm.PrivateKey)
	privKey.PriData = make([]byte, s.privateKeyLength)
	copy(privKey.PriData, privy)

	return privKey, nil
}

// convertBytesToPublic exports the corresponding secret key from the sig receiver.
func (s HybridedsfullSig) convertBytesToPublic(pub []byte) (*signaturealgorithm.PublicKey, error) {
	if len(pub) != s.publicKeyLength {
		return nil, pqchelpereds.ErrInvalidPublicKeyLen
	}
	pubKey := new(signaturealgorithm.PublicKey)
	pubKey.PubData = make([]byte, s.publicKeyLength)
	copy(pubKey.PubData, pub)
	return pubKey, nil
}

func (s HybridedsfullSig) PreExpansionSeedSize() int {
	return 0
}

func (s HybridedsfullSig) GenerateKeyFromPreExpansionSeed(preExpansionSeed []byte) (*signaturealgorithm.PrivateKey, error) {
	return nil, errors.New("GenerateKeyFromPreExpansionSeed not implemented")
}

// exportPrivateKey exports a private key into a binary dump.
func (s HybridedsfullSig) exportPrivateKey(privy *signaturealgorithm.PrivateKey) ([]byte, error) {
	if len(privy.PriData) != s.privateKeyLength {
		return nil, pqchelpereds.ErrInvalidPrivateKeyLen
	}

	buf := make([]byte, s.privateKeyLength)
	copy(buf, privy.PriData)
	return buf, nil
}

// exportPublicKey exports a public key into a binary dump.
func (s HybridedsfullSig) exportPublicKey(pub *signaturealgorithm.PublicKey) ([]byte, error) {
	if len(pub.PubData) != s.publicKeyLength {
		return nil, pqchelpereds.ErrInvalidPublicKeyLen
	}
	buf := make([]byte, s.publicKeyLength)
	copy(buf, pub.PubData)
	return buf, nil
}
