// Package cryptobase provides QuantumCoin's unified crypto backend.
//
// QuantumCoin uses post-quantum cryptography (PQC) in hybrid mode: every signature
// combines a classical algorithm (e.g. Ed25519) with NIST-standardized PQC algorithms.
// This project implements the NIST PQC standards for quantum-resistant security.
// Supported PQC schemes include:
//   - Dilithium + SPHINCS+ (NIST PQC standardization)
//   - ML-DSA (FIPS 204) + SLH-DSA (FIPS 205) — NIST-standardized post-quantum signatures
//
// Hybrid PQC ensures security against both classical and quantum adversaries while
// retaining compatibility and defense-in-depth. All signature algorithm IDs and
// dynamic signer/verifier logic route through this package.
package cryptobase

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideddsamldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideddsamldsaslhdsa5"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideddsamldsaslhdsafull"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybridedsfull"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa5"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

// Default and named hybrid PQC signature algorithms (NIST-standardized post-quantum in hybrid mode).
// Each algorithm pairs Ed25519 with NIST PQC signatures (Dilithium/SPHINCS+ or ML-DSA/SLH-DSA).
// These are the concrete implementations used by dynamic signing/verification paths below.
var SigAlg = hybrideds.CreateHybridedsSig()
var SigAlgHybridEds = hybrideds.CreateHybridedsSig()
var SigAlgHybridEdsFull = hybridedsfull.CreateHybridedsfullSig()
var SigAlgHybridMlDsaEddsaSlhDsaFull = hybrideddsamldsaslhdsafull.CreateHybridEddsaMldsaSlhdsaFullSig()
var SigAlgHybridMlDsaEddsaSlhDsaCompact = hybrideddsamldsaslhdsa.CreateHybridEddsaMldsaSlhdsaSig()
var SigAlgHybridMlDsaEddsaSlhDsa5 = hybrideddsamldsaslhdsa5.CreateHybridEddsaMldsaSlhdsaSig5()

type DynamicSigner struct {
}

var DynamicSign DynamicSigner = DynamicSigner{}

func (ds DynamicSigner) SignSigAlg(digestHash []byte, prv *signaturealgorithm.PrivateKey, sigAlg byte) (sig []byte, err error) {
	if sigAlg == byte(crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID) {
		return SigAlg.Sign(digestHash, prv)
	} else if sigAlg == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		return SigAlgHybridEdsFull.Sign(digestHash, prv)
	} else if sigAlg == byte(crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.Sign(digestHash, prv)
	} else if sigAlg == byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.Sign(digestHash, prv)
	} else if sigAlg == byte(crypto.MLDSA_ED25519_SLHDSA_5_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsa5.Sign(digestHash, prv)
	}
	return nil, errors.New("invalid sign mode")
}

func (ds DynamicSigner) Sign(digestHash []byte, prv *signaturealgorithm.PrivateKey) (sig []byte, err error) {
	signMode := defaults.GetSigningMode()
	if signMode == byte(crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID) {
		return SigAlg.Sign(digestHash, prv)
	} else if signMode == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		return SigAlgHybridEdsFull.Sign(digestHash, prv)
	} else if signMode == byte(crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.Sign(digestHash, prv)
	} else if signMode == byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.Sign(digestHash, prv)
	} else if signMode == byte(crypto.MLDSA_ED25519_SLHDSA_5_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsa5.Sign(digestHash, prv)
	}
	return nil, errors.New("invalid sign mode")
}

func (ds DynamicSigner) SignWithContext(digestHash []byte, prv *signaturealgorithm.PrivateKey, context []byte) (sig []byte, err error) {
	if len(context) < 1 {
		return nil, errors.New("invalid context")
	}
	if context[0] == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		return SigAlgHybridEdsFull.SignWithContext(digestHash, prv, context)
	} else if context[0] == byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.SignWithContext(digestHash, prv, context)
	}

	return nil, errors.New("invalid sign mode")
}

type DynamicVerifier struct {
}

var DynamicSigVerifier DynamicVerifier = DynamicVerifier{}

func (dv DynamicVerifier) CombinePublicKeySignature(sigBytes []byte, pubKeyBytes []byte) (combinedSignature []byte, err error) {
	if len(sigBytes) < 1 {
		return nil, errors.New("invalid signature")
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else {
		return nil, errors.New("CombinePublicKeySignature invalid signature type")
	}
}

func (dv DynamicVerifier) PublicKeyAndSignatureFromCombinedSignature(digestHash []byte, sig []byte) (signature []byte, pubKey []byte, err error) {
	sigBytes, _, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, nil, err
	}
	if len(sigBytes) < 1 {
		return nil, nil, errors.New("invalid signature")
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.PublicKeyAndSignatureFromCombinedSignature(digestHash, sig)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.PublicKeyAndSignatureFromCombinedSignature(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.PublicKeyAndSignatureFromCombinedSignature(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.PublicKeyAndSignatureFromCombinedSignature(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.PublicKeyAndSignatureFromCombinedSignature(digestHash, sig)
	} else {
		return nil, nil, errors.New("PublicKeyAndSignatureFromCombinedSignature invalid signature type")
	}
}

func (dv DynamicVerifier) PrivateAndPublicFromPrivateKey(privateKey []byte) (priv []byte, pub []byte, err error) {
	if privateKey == nil {
		return nil, nil, errors.New("nil private key")
	}
	if len(privateKey) == SigAlgHybridMlDsaEddsaSlhDsaCompact.PrivateKeyLength() {
		return pqchelpereddsamldsaslhdsa.PrivateAndPublicFromPrivateKey(privateKey)
	} else if len(privateKey) == SigAlgHybridMlDsaEddsaSlhDsa5.PrivateKeyLength() {
		return pqchelpereddsamldsaslhdsa5.PrivateAndPublicFromPrivateKey(privateKey)
	}

	return nil, nil, errors.New("invalid private key length")
}

func (dv DynamicVerifier) PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
	sigBytes, _, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}
	if len(sigBytes) < 1 {
		return nil, errors.New("invalid signature")
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.PublicKeyBytesFromSignature(digestHash, sig)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.PublicKeyBytesFromSignature(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.PublicKeyBytesFromSignature(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.PublicKeyBytesFromSignature(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.PublicKeyBytesFromSignature(digestHash, sig)
	} else {
		return nil, errors.New("PublicKeyBytesFromSignature invalid signature type")
	}
}

func (dv DynamicVerifier) GetAddress(digestHash []byte, sig []byte) (common.Address, error) {
	sigBytes, _, err := common.ExtractTwoParts(sig)
	if err != nil {
		return common.Address{}, err
	}
	if len(sigBytes) < 1 {
		return common.Address{}, errors.New("invalid signature")
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.GetAddress(digestHash, sig)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.GetAddress(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.GetAddress(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.GetAddress(digestHash, sig)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.GetAddress(digestHash, sig)
	} else {
		return common.Address{}, errors.New("GetAddress invalid signature type")
	}
}

func (dv DynamicVerifier) Verify(pubKey []byte, digestHash []byte, signature []byte) bool {
	sigBytes, _, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false
	}
	if len(sigBytes) < 1 {
		return false
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.Verify(pubKey, digestHash, signature)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.Verify(pubKey, digestHash, signature)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.Verify(pubKey, digestHash, signature)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.Verify(pubKey, digestHash, signature)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.Verify(pubKey, digestHash, signature)
	} else {
		return false
	}
}

func (dv DynamicVerifier) VerifyWithContext(pubKey []byte, digestHash []byte, signature []byte, context []byte) bool {
	sigBytes, _, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false
	}
	if len(sigBytes) < 1 {
		return false
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.VerifyWithContext(pubKey, digestHash, signature, context)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.VerifyWithContext(pubKey, digestHash, signature, context)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.VerifyWithContext(pubKey, digestHash, signature, context)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.VerifyWithContext(pubKey, digestHash, signature, context)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.VerifyWithContext(pubKey, digestHash, signature, context)
	} else {
		return false
	}
}

func (dv DynamicVerifier) ValidateSignatureValues(digestHash []byte, v byte, r, s *big.Int) (isOk bool, pubKey []byte, signature []byte) {
	sigBytes := s.Bytes()
	if len(sigBytes) < 1 {
		return false, nil, nil
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.ValidateSignatureValues(digestHash, v, r, s)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.ValidateSignatureValues(digestHash, v, r, s)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.ValidateSignatureValues(digestHash, v, r, s)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.ValidateSignatureValues(digestHash, v, r, s)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		return SigAlgHybridMlDsaEddsaSlhDsa5.ValidateSignatureValues(digestHash, v, r, s)
	} else {
		return false, nil, nil
	}
}

func (dv DynamicVerifier) IsSignatureTypeAllowedForTxn(blockNumber uint64, signature []byte) (bool, error) {
	if len(signature) < 1 {
		return false, errors.New("invalid signature length")
	}
	algType := crypto.SignatureAlgorithmType(signature[0])

	isBreakglassBlock := defaults.IsCryptoBreakglassMode(blockNumber)
	if isBreakglassBlock == false {
		if defaults.IsSigAlgSwitchMode(blockNumber) {
			return algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID || algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID ||
				algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID || algType == crypto.MLDSA_ED25519_SLHDSA_5_ID, nil
		} else {
			return algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID, nil
		}
	} else {
		return algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID || algType == crypto.MLDSA_ED25519_SLHDSA_5_ID, nil
	}
}

// excludes fullSign blocks
func GetSigAlgForValidation(blockNumber uint64) signaturealgorithm.SignatureAlgorithm {
	if defaults.IsCryptoBreakglassMode(blockNumber) {
		log.Debug("GetSigAlgForValidation breakglass", "blockNumber", blockNumber)
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridMlDsaEddsaSlhDsaFull)
	} else if defaults.IsSigAlgSwitchMode(blockNumber) {
		log.Debug("GetSigAlgForValidation SigAlgSwitchMode", "blockNumber", blockNumber)
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridMlDsaEddsaSlhDsaCompact)
	} else {
		log.Debug("GetSigAlgForValidation default", "blockNumber", blockNumber)
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridEds)
	}
}

func GetSigningContext() crypto.SigningContext {
	signMode := defaults.GetSigningMode()
	if signMode == byte(crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID) {
		return crypto.SigningContextDefault
	} else if signMode == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		return crypto.SigningContextLevel1
	} else if signMode == byte(crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID) {
		return crypto.SigningContextDefault
	} else if signMode == byte(crypto.MLDSA_ED25519_SLHDSA_FULL_ID) {
		return crypto.SigningContextLevel2
	} else if signMode == byte(crypto.MLDSA_ED25519_SLHDSA_5_ID) {
		return crypto.SigningContextLevel1
	} else {
		return crypto.SigningContextDefault
	}
}

func GetSigAlgForPrivateKeyHex(privateKeyHex string) (*signaturealgorithm.SignatureAlgorithm, error) {
	privateKey, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, err
	}

	length := len(privateKey)
	if length == SigAlg.PrivateKeyLength() {
		var s signaturealgorithm.SignatureAlgorithm
		s = &SigAlgHybridEds
		return &s, nil
	} else if len(privateKey) == SigAlgHybridMlDsaEddsaSlhDsa5.PrivateKeyLength() {
		var s signaturealgorithm.SignatureAlgorithm
		s = &SigAlgHybridMlDsaEddsaSlhDsa5
		return &s, nil
	} else {
		return nil, errors.New("invalid private key length")
	}
}

func GetSigAlgForPrivateKey(privateKey []byte) (*signaturealgorithm.SignatureAlgorithm, error) {
	length := len(privateKey)
	if length == SigAlg.PrivateKeyLength() {
		var s signaturealgorithm.SignatureAlgorithm
		s = &SigAlgHybridEds
		return &s, nil
	} else if len(privateKey) == SigAlgHybridMlDsaEddsaSlhDsa5.PrivateKeyLength() {
		var s signaturealgorithm.SignatureAlgorithm
		s = &SigAlgHybridMlDsaEddsaSlhDsa5
		return &s, nil
	} else {
		return nil, errors.New("invalid private key length")
	}
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
			return errors.New("key file too long")
		}
	}
}

func LoadPrivateKeyFromFile(file string) (*signaturealgorithm.PrivateKey, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	r := bufio.NewReader(fd)
	buf := make([]byte, 7680*2) //max size possible based on hybridedmldsaslhdsa5.PrivateKeySize
	n, err := readASCII(buf, r)
	if err != nil {
		return nil, err
	} else if n != len(buf) {
		return nil, fmt.Errorf("key file too short, want oqs hex character")
	}
	if err := checkKeyFileEnd(r); err != nil {
		return nil, err
	}
	b, err := hex.DecodeString(string(buf))
	if err != nil {
		return nil, err
	}
	sigAlgPtr, err := GetSigAlgForPrivateKey(b)
	if err != nil {
		return nil, err
	}
	sigAlg := *sigAlgPtr

	return sigAlg.HexToPrivateKey(string(buf))
}

func SigAlgFromSignature(digestHash []byte, sig []byte) (algorithm *signaturealgorithm.SignatureAlgorithm, err error) {
	sigBytes, _, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}
	if len(sigBytes) < 1 {
		return nil, errors.New("invalid signature")
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	var s signaturealgorithm.SignatureAlgorithm
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		s = &SigAlgHybridEds
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		s = &SigAlgHybridEdsFull
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		s = &SigAlgHybridMlDsaEddsaSlhDsaCompact
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		s = &SigAlgHybridMlDsaEddsaSlhDsaFull
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_5_ID {
		s = &SigAlgHybridMlDsaEddsaSlhDsa5
	} else {
		return nil, errors.New("CombinePublicKeySignature invalid signature type")
	}
	return &s, nil
}

func GetSigAlgForKeyType(keyType int) (*signaturealgorithm.SignatureAlgorithm, error) {
	var s signaturealgorithm.SignatureAlgorithm
	if keyType >= 0 && keyType <= 4 {
		s = &SigAlgHybridMlDsaEddsaSlhDsaCompact
	} else if keyType == 5 {
		s = &SigAlgHybridMlDsaEddsaSlhDsa5
	} else {
		return nil, errors.New("invalid key type")
	}

	return &s, nil
}
