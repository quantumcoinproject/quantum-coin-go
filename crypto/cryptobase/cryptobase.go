package cryptobase

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybridedsfull"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"math/big"
)

var SigAlg = hybrideds.CreateHybridedsSig()

var SigAlgHybridEds = hybrideds.CreateHybridedsSig()
var SigAlgHybridEdsFull = hybridedsfull.CreateHybridedsfullSig()

type DynamicSigner struct {
}

var DynamicSign DynamicSigner = DynamicSigner{}

func (ds DynamicSigner) Sign(digestHash []byte, prv *signaturealgorithm.PrivateKey) (sig []byte, err error) {
	signMode := defaults.GetSigningMode()
	if signMode == byte(crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID) {
		return SigAlg.Sign(digestHash, prv)
	} else if signMode == byte(crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID) {
		return SigAlgHybridEdsFull.Sign(digestHash, prv)
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
	} else {
		return false, nil, nil
	}
}

func (dv DynamicVerifier) IsSignatureTypeAllowed(blockNumber uint64, signature []byte) (bool, error) {
	algType := crypto.SignatureAlgorithmType(signature[0])

	isBreakglassBlock := defaults.IsCryptoBreakglassMode(blockNumber)
	if isBreakglassBlock == false {
		return algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID, nil
	} else {
		return algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID, nil
	}
}

func GetSigAlg(blockNumber uint64) signaturealgorithm.SignatureAlgorithm {
	if defaults.IsCryptoBreakglassMode(blockNumber) {
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridEdsFull)
	} else {
		return signaturealgorithm.SignatureAlgorithm(SigAlgHybridEds)
	}
}
