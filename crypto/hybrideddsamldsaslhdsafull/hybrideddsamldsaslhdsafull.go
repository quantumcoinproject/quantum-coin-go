package hybrideddsamldsaslhdsafull

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/quantumcoinproject/circl/sign/hybridedmldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"time"
)

type HybridEddsaMldsaSlhdsaFullSig struct {
	sigName                      string
	publicKeyLength              int
	privateKeyLength             int
	signatureLength              int
	signatureWithPublicKeyLength int
}

func CreateHybridEddsaMldsaSlhdsaFullSig() HybridEddsaMldsaSlhdsaFullSig {
	return HybridEddsaMldsaSlhdsaFullSig{sigName: "hybrideddsamldsaslhdsafull",
		publicKeyLength:              hybridedmldsaslhdsa.PublicKeySize,
		privateKeyLength:             hybridedmldsaslhdsa.PrivateKeySize,
		signatureLength:              hybridedmldsaslhdsa.SigLength,
		signatureWithPublicKeyLength: hybridedmldsaslhdsa.PublicKeySize + hybridedmldsaslhdsa.SigLength + common.LengthByteSize + common.LengthByteSize,
	}
}

func (s HybridEddsaMldsaSlhdsaFullSig) SignatureName() string {
	return s.sigName
}

func (s HybridEddsaMldsaSlhdsaFullSig) GetSigAlgType() crypto.SignatureAlgorithmType {
	return crypto.MLDSA_ED25519_SLHDSA_FULL_ID
}

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyLength() int {
	return s.publicKeyLength
}

func (s HybridEddsaMldsaSlhdsaFullSig) PrivateKeyLength() int {
	return s.privateKeyLength
}

func (s HybridEddsaMldsaSlhdsaFullSig) SignatureLength() int {
	return s.signatureLength
}

func (s HybridEddsaMldsaSlhdsaFullSig) SignatureWithPublicKeyLength() int {
	return s.signatureWithPublicKeyLength
}

func (s HybridEddsaMldsaSlhdsaFullSig) GenerateKey() (*signaturealgorithm.PrivateKey, error) {
	pubKey, priKey, err := pqchelpereddsamldsaslhdsa.GenerateKey()
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

func (s HybridEddsaMldsaSlhdsaFullSig) GenerateKeyWithReader(rand io.Reader) (*signaturealgorithm.PrivateKey, error) {
	// first step is to create a slice of bytes with the desired length
	seedBuf := make([]byte, hybridedmldsaslhdsa.SeedSize)
	// then we can call rand.Read.
	n, err := rand.Read(seedBuf)
	if err != nil {
		return nil, err
	}
	if n < hybridedmldsaslhdsa.SeedSize {
		return nil, errors.New("n less than SEED_SIZE")
	}
	return s.GenerateKeyWithSeed(seedBuf[:])
}

func (s HybridEddsaMldsaSlhdsaFullSig) GetRequiredSeedLength() uint {
	return hybridedmldsaslhdsa.SeedSize
}

func (s HybridEddsaMldsaSlhdsaFullSig) GenerateKeyWithSeed(seed []byte) (*signaturealgorithm.PrivateKey, error) {
	if len(seed) != hybridedmldsaslhdsa.SeedSize {
		return nil, errors.New("invalid seed size")
	}
	pubKey, priKey, err := pqchelpereddsamldsaslhdsa.GenerateKeyWithSeed(seed)
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

func (s HybridEddsaMldsaSlhdsaFullSig) SerializePrivateKey(priv *signaturealgorithm.PrivateKey) ([]byte, error) {
	priBytes, err := s.exportPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	return priBytes, err
}

func (s HybridEddsaMldsaSlhdsaFullSig) DeserializePrivateKey(priv []byte) (*signaturealgorithm.PrivateKey, error) {

	privKeyBytes, pubKeyBytes, err := pqchelpereddsamldsaslhdsa.PrivateAndPublicFromPrivateKey(priv)
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

func (s HybridEddsaMldsaSlhdsaFullSig) SerializePublicKey(pub *signaturealgorithm.PublicKey) ([]byte, error) {
	return s.exportPublicKey(pub)
}

func (s HybridEddsaMldsaSlhdsaFullSig) DeserializePublicKey(pub []byte) (*signaturealgorithm.PublicKey, error) {
	pubKey, error := s.convertBytesToPublic(pub)
	return pubKey, error
}

func (s HybridEddsaMldsaSlhdsaFullSig) HexToPrivateKey(hexkey string) (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridEddsaMldsaSlhdsaFullSig) HexToPrivateKeyNoError(hexkey string) *signaturealgorithm.PrivateKey {
	p, err := s.HexToPrivateKey(hexkey)
	if err != nil {
		panic("HexToPrivateKey")
	}
	return p
}

func (s HybridEddsaMldsaSlhdsaFullSig) PrivateKeyToHex(priv *signaturealgorithm.PrivateKey) (string, error) {
	data, err := s.SerializePrivateKey(priv)
	if err != nil {
		return "", err
	}
	k := hex.EncodeToString(data)
	return k, nil
}

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyToHex(pub *signaturealgorithm.PublicKey) (string, error) {
	data, err := s.SerializePublicKey(pub)
	if err != nil {
		return "", err
	}
	k := hex.EncodeToString(data)
	return k, nil
}

func (s HybridEddsaMldsaSlhdsaFullSig) HexToPublicKey(hexkey string) (*signaturealgorithm.PublicKey, error) {
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

func (s HybridEddsaMldsaSlhdsaFullSig) LoadPrivateKeyFromFile(file string) (*signaturealgorithm.PrivateKey, error) {
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

func (s HybridEddsaMldsaSlhdsaFullSig) SavePrivateKeyToFile(file string, key *signaturealgorithm.PrivateKey) error {
	k, err := s.PrivateKeyToHex(key)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, []byte(k), 0600)
}

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyToAddress(p *signaturealgorithm.PublicKey) (common.Address, error) {
	pubBytes, err := s.SerializePublicKey(p)
	tempAddr := common.Address{}
	if err != nil {
		return tempAddr, err
	}
	return crypto.PublicKeyBytesToAddress(pubBytes), nil
}

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyToAddressNoError(p *signaturealgorithm.PublicKey) common.Address {
	addr, err := s.PublicKeyToAddress(p)
	if err != nil {
		panic("PublicKeyToAddress failed")
	}
	return addr
}

func (s HybridEddsaMldsaSlhdsaFullSig) Sign(digestHash []byte, prv *signaturealgorithm.PrivateKey) (sig []byte, err error) {
	seckey, err := s.exportPrivateKey(prv)
	if err != nil {
		return nil, err
	}

	sigBytes, err := pqchelpereddsamldsaslhdsa.Sign(seckey, digestHash)
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

func (s HybridEddsaMldsaSlhdsaFullSig) SignWithContext(digestHash []byte, prv *signaturealgorithm.PrivateKey, context []byte) (sig []byte, err error) {
	if context == nil || len(context) < 1 {
		return nil, errors.New("SignWithContext failed context")
	}

	if context[0] == byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
		newDigestHash := crypto.Keccak256(digestHash, context)
		return s.Sign(newDigestHash, prv)
	}

	return nil, errors.New("SignWithContext failed invalid context")
}

func (s HybridEddsaMldsaSlhdsaFullSig) Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	sigBytes, pubKeyBytes, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false
	}

	if !bytes.Equal(pubKey, pubKeyBytes) {
		return false
	}

	return pqchelpereddsamldsaslhdsa.Verify(pubKey, digestHash, sigBytes)
}

func (s HybridEddsaMldsaSlhdsaFullSig) VerifyWithContext(pubKey []byte, digestHash []byte, signature []byte, context []byte) bool {
	if context == nil || len(context) < 1 {
		return false
	}

	if context[0] == byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
		newDigestHash := crypto.Keccak256(digestHash, context)
		return s.Verify(pubKey, newDigestHash, signature)
	}

	return false
}

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyAndSignatureFromCombinedSignature(digestHash []byte, sig []byte) (signature []byte, pubKey []byte, err error) {
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

func (s HybridEddsaMldsaSlhdsaFullSig) CombinePublicKeySignature(sigBytes []byte, pubKeyBytes []byte) (combinedSignature []byte, err error) {
	if len(sigBytes) < s.signatureLength {
		log.Debug("HybridEddsaMldsaSlhdsaFullSig CombinePublicKeySignature", "sigbytes len", len(sigBytes), "signatureLength", s.signatureLength)
		return nil, pqchelpereddsamldsaslhdsa.ErrInvalidSignatureLen
	}

	if len(pubKeyBytes) != s.publicKeyLength {
		return nil, errors.New("invalid public key length")
	}

	return common.CombineTwoParts(sigBytes, pubKeyBytes), nil
}

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
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

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyFromSignature(digestHash []byte, sig []byte) (*signaturealgorithm.PublicKey, error) {
	b, err := s.PublicKeyBytesFromSignature(digestHash, sig)
	if err != nil {
		return nil, err
	}
	return s.DeserializePublicKey(b)
}

func (s HybridEddsaMldsaSlhdsaFullSig) GetAddress(digestHash []byte, sig []byte) (common.Address, error) {
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

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyFromSignatureWithContext(digestHash []byte, sig []byte, context []byte) (*signaturealgorithm.PublicKey, error) {
	if context[0] != byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
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
func (osig HybridEddsaMldsaSlhdsaFullSig) ValidateSignatureValues(digestHash []byte, v byte, r, s *big.Int) (isOk bool, pub []byte, sig []byte) {
	if v != 1 {
		return false, nil, nil
	}
	pubKey, signature := r.Bytes(), s.Bytes()

	if len(pubKey) != osig.PublicKeyLength() {
		if time.Now().UTC().Unix() < defaults.DefaultConfig.ValidateSigPubStartTime { //remove check after time has elapsed
			return false, nil, nil
		}

		if len(pubKey) > osig.PublicKeyLength() {
			return false, nil, nil
		}
		//conversion issues since big.Int setBytes stores only positive integers. pad with zero's
		log.Debug("ValidateSignatureValues padding zero", "pubKey len", len(pubKey), "expected len", osig.PublicKeyLength())
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

func (s HybridEddsaMldsaSlhdsaFullSig) PublicKeyStartValue() byte {
	return 0x00 + 9
}

func (s HybridEddsaMldsaSlhdsaFullSig) SignatureStartValue() byte {
	return 0x30 + 9
}

func (s HybridEddsaMldsaSlhdsaFullSig) Zeroize(prv *signaturealgorithm.PrivateKey) {
	b := prv.PriData
	for i := range b {
		b[i] = 0
	}
}

func (s HybridEddsaMldsaSlhdsaFullSig) EncodePublicKey(pubKey *signaturealgorithm.PublicKey) []byte {
	encoded := make([]byte, s.publicKeyLength)
	copy(encoded, pubKey.PubData)
	return encoded
}

func (s HybridEddsaMldsaSlhdsaFullSig) DecodePublicKey(encoded []byte) (*signaturealgorithm.PublicKey, error) {
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
func (s HybridEddsaMldsaSlhdsaFullSig) convertBytesToPrivate(privy []byte) (*signaturealgorithm.PrivateKey, error) {
	if len(privy) != s.privateKeyLength {
		return nil, pqchelpereddsamldsaslhdsa.ErrInvalidPrivateKeyLen
	}
	privKey := new(signaturealgorithm.PrivateKey)
	privKey.PriData = make([]byte, s.privateKeyLength)
	copy(privKey.PriData, privy)

	return privKey, nil
}

// convertBytesToPublic exports the corresponding secret key from the sig receiver.
func (s HybridEddsaMldsaSlhdsaFullSig) convertBytesToPublic(pub []byte) (*signaturealgorithm.PublicKey, error) {
	if len(pub) != s.publicKeyLength {
		return nil, pqchelpereddsamldsaslhdsa.ErrInvalidPublicKeyLen
	}
	pubKey := new(signaturealgorithm.PublicKey)
	pubKey.PubData = make([]byte, s.publicKeyLength)
	copy(pubKey.PubData, pub)
	return pubKey, nil
}

// exportPrivateKey exports a private key into a binary dump.
func (s HybridEddsaMldsaSlhdsaFullSig) exportPrivateKey(privy *signaturealgorithm.PrivateKey) ([]byte, error) {
	if len(privy.PriData) != s.privateKeyLength {
		return nil, pqchelpereddsamldsaslhdsa.ErrInvalidPrivateKeyLen
	}

	buf := make([]byte, s.privateKeyLength)
	copy(buf, privy.PriData)
	return buf, nil
}

// exportPublicKey exports a public key into a binary dump.
func (s HybridEddsaMldsaSlhdsaFullSig) exportPublicKey(pub *signaturealgorithm.PublicKey) ([]byte, error) {
	if len(pub.PubData) != s.publicKeyLength {
		return nil, pqchelpereddsamldsaslhdsa.ErrInvalidPublicKeyLen
	}
	buf := make([]byte, s.publicKeyLength)
	copy(buf, pub.PubData)
	return buf, nil
}
