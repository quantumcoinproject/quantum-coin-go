package cryptobase

import (
	"errors"
	"math/big"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideddsamldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideddsamldsaslhdsafull"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybridedsfull"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

var SigAlg = hybrideds.CreateHybridedsSig()

var SigAlgHybridEds = hybrideds.CreateHybridedsSig()
var SigAlgHybridEdsFull = hybridedsfull.CreateHybridedsfullSig()
var SigAlgHybridMlDsaEddsaSlhDsaFull = hybrideddsamldsaslhdsafull.CreateHybridEddsaMldsaSlhdsaFullSig()
var SigAlgHybridMlDsaEddsaSlhDsaCompact = hybrideddsamldsaslhdsa.CreateHybridEddsaMldsaSlhdsaSig()

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
	}
	return nil, errors.New("invalid sign mode")
}

func (ds DynamicSigner) SignWithContext(digestHash []byte, prv *signaturealgorithm.PrivateKey, context []byte) (sig []byte, err error) {
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
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaCompact.CombinePublicKeySignature(sigBytes, pubKeyBytes)
	} else if algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
		return SigAlgHybridMlDsaEddsaSlhDsaFull.CombinePublicKeySignature(sigBytes, pubKeyBytes)
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
		return nil, nil, err
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
	} else {
		return nil, nil, errors.New("PublicKeyAndSignatureFromCombinedSignature invalid signature type")
	}
}

func (dv DynamicVerifier) PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error) {
	sigBytes, _, err := common.ExtractTwoParts(sig)
	if err != nil {
		return nil, err
	}
	if len(sigBytes) < 1 {
		return nil, err
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
		return common.Address{}, err
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
	} else {
		return false, nil, nil
	}
}

func (dv DynamicVerifier) IsSignatureTypeAllowedForTxn(blockNumber uint64, signature []byte) (bool, error) {
	algType := crypto.SignatureAlgorithmType(signature[0])

	isBreakglassBlock := defaults.IsCryptoBreakglassMode(blockNumber)
	if isBreakglassBlock == false {
		if defaults.IsSigAlgSwitchMode(blockNumber) {
			return algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID || algType == crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID, nil
		} else {
			return algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID, nil
		}
	} else {
		return algType == crypto.MLDSA_ED25519_SLHDSA_FULL_ID, nil
	}
}

func GetSigAlgForValidation(blockNumber uint64) signaturealgorithm.SignatureAlgorithm {
	if defaults.IsCryptoBreakglassMode(blockNumber) {
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridMlDsaEddsaSlhDsaFull)
	} else if defaults.IsSigAlgSwitchMode(blockNumber) {
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridMlDsaEddsaSlhDsaCompact)
	} else {
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridEds)
	}
}
