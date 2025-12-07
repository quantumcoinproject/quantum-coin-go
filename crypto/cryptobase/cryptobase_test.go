package cryptobase

import (
	"fmt"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

func testSigAlg(t *testing.T, blockNumber uint64, expected crypto.SignatureAlgorithmType) {
	sigAlg := GetSigAlgForValidation(blockNumber)
	if GetSigAlgForValidation(blockNumber).GetSigAlgType() != expected {
		fmt.Println("testSigAlg", "expected", expected, "got", sigAlg.GetSigAlgType(), "blockNumber", blockNumber)
		t.Fatalf("failed")
	}
}

// excludes fullSign
func TestValidationAlg(t *testing.T) {
	testSigAlg(t, defaults.DefaultConfig.PosConfig.SigAlgSwitchBlock-1, crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID)
	testSigAlg(t, defaults.DefaultConfig.PosConfig.SigAlgSwitchBlock, crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID)
	testSigAlg(t, defaults.DefaultConfig.PosConfig.SigAlgSwitchBlock+1, crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID)
	testSigAlg(t, defaults.DefaultConfig.PosConfig.SigAlgSwitchBlock+1, crypto.MLDSA_ED25519_SLHDSA_5_ID)
	block := uint64(10000000)

	testSigAlg(t, block, crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID)
	err := defaults.SetCryptoBreakGlassBlock(block)
	if err != nil {
		t.Fatal(err)
	}

	testSigAlg(t, block-1, crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID)
	testSigAlg(t, block, crypto.MLDSA_ED25519_SLHDSA_FULL_ID)
	testSigAlg(t, block+1, crypto.MLDSA_ED25519_SLHDSA_FULL_ID)
	testSigAlg(t, block+1, crypto.MLDSA_ED25519_SLHDSA_5_ID)
}
