package cryptobase

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybridedsfull"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"math/big"
)

var SigAlg = hybrideds.CreateHybridedsSig(true)

var SigAlgHybridEds = hybrideds.CreateHybridedsSig(true)
var SigAlgHybridEdsFull = hybridedsfull.CreateHybridedsfullSig()

type DynamicVerifier struct {
}

var DynamicSigVerifier DynamicVerifier = DynamicVerifier{}

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

func (dv DynamicVerifier) ValidateSignatureValues(digestHash []byte, v byte, r, s *big.Int) bool {
	sigBytes := s.Bytes()
	if len(sigBytes) < 1 {
		return false
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	if algType == crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID {
		return SigAlgHybridEds.ValidateSignatureValues(digestHash, v, r, s)
	} else if algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID {
		return SigAlgHybridEdsFull.ValidateSignatureValues(digestHash, v, r, s)
	} else {
		return false
	}
}

func (dv DynamicVerifier) IsBreakglassCompatible(blockNumber uint64, signature []byte) (bool, error) {
	isBreakglassBlock := defaults.IsCryptoBreakglassMode(blockNumber)
	if isBreakglassBlock == false {
		return false, nil

	}

	sigBytes, _, err := common.ExtractTwoParts(signature)
	if err != nil {
		return false, errors.New("invalid signature")
	}
	algType := crypto.SignatureAlgorithmType(sigBytes[0])
	return algType == crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID, nil
}
