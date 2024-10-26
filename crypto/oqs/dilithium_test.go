package oqs

import (
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"
	"testing"
)

func TestDilithiumSig_Basic(t *testing.T) {
	InitOqs()

	var sig signaturealgorithm.SignatureAlgorithm
	sig = InitDilithium()

	signaturealgorithm.SignatureAlgorithmTest(t, sig)
}
