package hybrideds

import (
	"bytes"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybridpqc"
	"math/rand"
	"testing"
)

var (
	testmsg1 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626262")
	testmsg2 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626261")
)

func TestHybrideds_Basic(t *testing.T) {
	if CRYPTO_SIGNATURE_BYTES != 2+64+2420+40+CRYPTO_MESSAGE_LEN {
		t.Fatal("incorrect sig size")
	}
	pubKey, priKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	priBytes, pubBytes, err := hybridpqc.PrivateAndPublicFromPrivateKey(priKey)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(priKey, priBytes) != 0 {
		t.Fatal("PrivateAndPublicFromPrivateKey private compare failed")
	}

	if bytes.Compare(pubKey, pubBytes) != 0 {
		t.Fatal("PrivateAndPublicFromPrivateKey public compare failed")
	}

	digestHash1 := []byte(testmsg1)

	signature, err := Sign(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}

	sigAlg := CreateHybridedsSig(true)

	sig := common.CombineTwoParts(signature, pubKey)

	ok := sigAlg.Verify(pubKey, digestHash1, sig)
	if ok == false {
		t.Fatal("verify failed")
	}

	digestHash1[0] = digestHash1[0] + 1
	sig = common.CombineTwoParts(signature, pubKey)
	ok = sigAlg.Verify(pubKey, digestHash1, signature)
	if ok == true {
		t.Fatal("verify passed unexpectedly")
	}

	signature2, err := Sign(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}
	sig2 := common.CombineTwoParts(signature2, pubKey)
	ok = sigAlg.Verify(pubKey, digestHash1, sig2)
	if ok == false {
		t.Fatal("verify failed")
	}

}

func TestHybrideds_Random(t *testing.T) {

	var keyMap map[string]bool
	keyMap = make(map[string]bool)

	for i := 1; i < 100; i++ {
		pubKey, priKey, err := GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		pubKeyText := string(pubKey[:])
		if keyMap[pubKeyText] == true {
			t.Fatal("same key")
		}

		keyMap[pubKeyText] = true

		digestHash1 := make([]byte, 32)
		rand.Read(digestHash1)

		signature1, err := Sign(priKey, digestHash1)
		if err != nil {
			t.Fatal(err)
		}

		sigAlg := CreateHybridedsSig(true)
		sig1 := common.CombineTwoParts(signature1, pubKey)

		ok := sigAlg.Verify(pubKey, digestHash1, sig1)
		if ok == false {
			t.Fatal("verify failed")
		}

		digestHash2 := make([]byte, 32)
		rand.Read(digestHash2)

		signature2, err := Sign(priKey, digestHash2)
		if err != nil {
			t.Fatal(err)
		}

		sig2 := common.CombineTwoParts(signature2, pubKey)
		ok = sigAlg.Verify(pubKey, digestHash2, sig2)
		if ok == false {
			t.Fatal("verify failed")
		}

		sig2a := common.CombineTwoParts(signature1, pubKey)
		ok = sigAlg.Verify(pubKey, digestHash2, sig2a)
		if ok == true {
			t.Fatal("verify passed while it should have failed")
		}

		sig2b := common.CombineTwoParts(signature2, pubKey)
		ok = sigAlg.Verify(pubKey, digestHash1, sig2b)
		if ok == true {
			t.Fatal("verify passed while it should have failed")
		}
	}

}
