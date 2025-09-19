package pqchelpereds

import (
	"bytes"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"math/rand"
	"testing"
)

var (
	testmsg1 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626262")
	testmsg2 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626261")
)

func TestHybrideds_Basic_compact(t *testing.T) {
	if CRYPTO_COMPACT_SIGNATURE_BYTES != 2+64+2420+40+CRYPTO_MESSAGE_LEN {
		t.Fatal("incorrect sig size")
	}
	pubKey, priKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	priBytes, pubBytes, err := PrivateAndPublicFromPrivateKey(priKey)
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

	signature, err := SignCompact(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}

	ok := VerifyCompact(pubKey, digestHash1, signature)
	if ok == false {
		t.Fatal("verify failed")
	}

	digestHash1[0] = digestHash1[0] + 1
	ok = VerifyCompact(pubKey, digestHash1, signature)
	if ok == true {
		t.Fatal("verify passed unexpectedly")
	}

	signature2, err := SignCompact(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}

	ok = VerifyCompact(pubKey, digestHash1, signature2)
	if ok == false {
		t.Fatal("verify failed")
	}

}

func TestHybrideds_Random_compact(t *testing.T) {

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

		signature1, err := SignCompact(priKey, digestHash1)
		if err != nil {
			t.Fatal(err)
		}

		ok := VerifyCompact(pubKey, digestHash1, signature1)
		if ok == false {
			t.Fatal("verify failed")
		}

		digestHash2 := make([]byte, 32)
		rand.Read(digestHash2)

		signature2, err := SignCompact(priKey, digestHash2)
		if err != nil {
			t.Fatal(err)
		}

		ok = VerifyCompact(pubKey, digestHash2, signature2)
		if ok == false {
			t.Fatal("verify failed")
		}

		ok = VerifyCompact(pubKey, digestHash2, signature1)
		if ok == true {
			t.Fatal("verify passed while it should have failed")
		}

		ok = VerifyCompact(pubKey, digestHash1, signature2)
		if ok == true {
			t.Fatal("verify passed while it should have failed")
		}
	}

}
