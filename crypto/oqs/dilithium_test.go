package oqs

import (
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"testing"
)

func TestDilithiumSig_Basic(t *testing.T) {
	InitOqs()

	var sig signaturealgorithm.SignatureAlgorithm
	sig = InitDilithium()

	signaturealgorithm.SignatureAlgorithmTest(t, sig)
}
