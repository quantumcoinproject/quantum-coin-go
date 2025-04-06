package cryptobase

import (
	"github.com/quantumcoinproject/quantum-coin-go/crypto/drng/ChaCha20"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideds"
)

var SigAlg = hybrideds.CreateHybridedsSig(true)

var DRNG = &ChaCha20.ChaCha20DRNGInitializer{}
