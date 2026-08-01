package ethapi

// The combined signature size reported by eth_getTransactionSignature must be
// the size of the CombineTwoParts wire form -- [totalLen:2][sigLen:2][sig][pk]
// -- because that framing is what lets a verifier extract the public key from
// the signature blob. For any scheme, that is publicKey + signature + two
// uint16 length prefixes, which the crypto packages expose as
// SignatureWithPublicKeyLength(). A drift between the two would mean the API
// is describing a framing the chain does not use.

import (
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
)

func TestCombinedSignatureSizeMatchesScheme(t *testing.T) {
	pkLen := cryptobase.SigAlg.PublicKeyLength()
	sigLen := cryptobase.SigAlg.SignatureLength()
	combined := cryptobase.SigAlg.SignatureWithPublicKeyLength()

	if want := pkLen + sigLen + 2*common.LengthByteSize; combined != want {
		t.Fatalf("SignatureWithPublicKeyLength() = %d, want pk(%d)+sig(%d)+4 = %d",
			combined, pkLen, sigLen, want)
	}

	// The API computes it from the actual byte slices; same arithmetic.
	signature := make([]byte, sigLen)
	publicKey := make([]byte, pkLen)
	if got := len(signature) + len(publicKey) + 2*common.LengthByteSize; got != combined {
		t.Errorf("API formula = %d, scheme constant = %d", got, combined)
	}

	// And CombineTwoParts really produces a blob of exactly that length.
	signature[0] = 1 // CombineTwoParts panics on empty parts only; content free
	blob := common.CombineTwoParts(signature, publicKey)
	if len(blob) != combined {
		t.Errorf("CombineTwoParts produced %d bytes, want %d", len(blob), combined)
	}
}
