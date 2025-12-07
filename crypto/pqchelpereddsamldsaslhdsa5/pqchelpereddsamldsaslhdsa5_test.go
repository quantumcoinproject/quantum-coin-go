package pqchelpereddsamldsaslhdsa5

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
)

var (
	testmsg1 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626262")
	testmsg2 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626261")
)

func TestHybrideds_Basic(t *testing.T) {
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

	signature, err := Sign(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}

	ok := Verify(pubKey, digestHash1, signature)
	if ok == false {
		t.Fatal("verify failed")
	}

	digestHash1[0] = digestHash1[0] + 1
	ok = Verify(pubKey, digestHash1, signature)
	if ok == true {
		t.Fatal("verify passed unexpectedly")
	}

	signature2, err := Sign(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}

	ok = Verify(pubKey, digestHash1, signature2)
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

		ok := Verify(pubKey, digestHash1, signature1)
		if ok == false {
			t.Fatal("verify failed")
		}

		digestHash2 := make([]byte, 32)
		rand.Read(digestHash2)

		signature2, err := Sign(priKey, digestHash2)
		if err != nil {
			t.Fatal(err)
		}

		ok = Verify(pubKey, digestHash2, signature2)
		if ok == false {
			t.Fatal("verify failed")
		}

		ok = Verify(pubKey, digestHash2, signature1)
		if ok == true {
			t.Fatal("verify passed while it should have failed")
		}

		ok = Verify(pubKey, digestHash1, signature2)
		if ok == true {
			t.Fatal("verify passed while it should have failed")
		}
	}

}
