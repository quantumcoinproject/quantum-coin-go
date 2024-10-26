package cryptobase

import (
	"github.com/QuantumCoinProject/qc/crypto/drng/ChaCha20"
	"github.com/QuantumCoinProject/qc/crypto/hybrideds"
)

var SigAlg = hybrideds.CreateHybridedsSig(true)

var DRNG = &ChaCha20.ChaCha20DRNGInitializer{}

//var SigAlg = mocksignaturealgorithm.CreateMockSig()
