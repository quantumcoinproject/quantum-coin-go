package keyestablishmentalgorithm

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"sync"
	"testing"
)

// wgKEMCorrectness groups goroutines and blocks the caller until all goroutines finish.
var wgKEMCorrectness sync.WaitGroup

// wgKEMWrongCiphertext groups goroutines and blocks the caller until all goroutines finish.
var wgKEMWrongCiphertext sync.WaitGroup

func EncapSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	var scheme = GetScheme()
	var KemName = scheme.Name()

	kem := KeyEncap{}
	defer kem.Clean() // clean up even in case of panic
	err = kem.Init(KemName, nil)
	if err != nil {
		return nil, nil, err
	}
	ciphertext, sharedSecret, err = kem.EncapsulateSecret(publicKey)
	return ciphertext, sharedSecret, err
}

func DecapSecret(seckey, ciphertext []byte) ([]byte, error) {
	var scheme = GetScheme()
	var KemName = scheme.Name()

	kem := KeyEncap{}
	defer kem.Clean() // clean up even in case of panic
	err := kem.Init(KemName, seckey)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := kem.DecapsulateSecret(ciphertext)
	return sharedSecret, err
}

// testKEMCorrectness tests the correctness of a specific KEM.
func testKEMCorrectness(threading bool, t *testing.T) {
	var scheme = GetScheme()
	var KemName = scheme.Name()

	log.Println("Correctness - ", KemName) // thread-safe
	if threading == true {
		defer wgKEMCorrectness.Done()
	}
	// ignore potential errors everywhere
	clientKey, err := GenerateKemKeyPair()
	if err != nil {
		fmt.Println(KemName + ": GenerateKemKeyPair failed")
		t.Fatalf("failed")
	}

	ciphertext, sharedSecretServer, err := EncapSecret(clientKey.N)
	if err != nil {
		fmt.Println(KemName + ": EncapSecret sharedSecretServer failed")
	}

	if bytes.Equal(clientKey.N, ciphertext) {
		// t.Errorf is thread-safe
		fmt.Println(KemName + ": publicKey ciphertext coincides")
		t.Fatalf("failed")
	}

	ciphertext1, sharedSecretServer1, err := EncapSecret(clientKey.N)
	if err != nil {
		fmt.Println(KemName + ": EncapSecret sharedSecretServer1 failed")
	}

	if bytes.Equal(ciphertext, ciphertext1) {
		// t.Errorf is thread-safe
		fmt.Println(KemName + ": ciphertext coincides")
		t.Fatalf("failed")
	}

	sharedSecretClient, err := DecapSecret(clientKey.D, ciphertext)
	if err != nil {
		fmt.Println(KemName + ": DecapSecret sharedSecretClient failed")
		t.Fatalf("failed")
	}

	if !bytes.Equal(sharedSecretClient, sharedSecretServer) {
		// t.Errorf is thread-safe
		fmt.Println(KemName + ": shared secrets do not coincide")
		t.Fatalf("failed")
	}

	sharedSecretClient1, err := DecapSecret(clientKey.D, ciphertext1)
	if err != nil {
		fmt.Println(KemName + ": DecapSecret sharedSecretClient1 failed")
		t.Fatalf("failed")
	}

	if !bytes.Equal(sharedSecretClient1, sharedSecretServer1) {
		// t.Errorf is thread-safe
		fmt.Println(KemName + ": shared secrets do not coincide")
		t.Fatalf("failed")
	}
}

// testKEMWrongCiphertext tests the wrong ciphertext regime of a specific KEM.
func testKEMWrongCiphertext(threading bool, t *testing.T) {
	var scheme = GetScheme()
	var KemName = scheme.Name()

	if threading == true {
		defer wgKEMWrongCiphertext.Done()
	}
	// ignore potential errors everywhere
	clientKey, err := GenerateKemKeyPair()
	if err != nil {
		fmt.Println(KemName + ": GenerateKemKeyPair failed")
		t.Fatalf("failed")
	}

	ciphertext, sharedSecretServer, err := EncapSecret(clientKey.N)
	if err != nil {
		fmt.Println(KemName + ": EncapSecret sharedSecretServer failed")
		t.Fatalf("failed")
	}

	wrongCiphertext := csprngEntropy(len(ciphertext))
	sharedSecretClient, err := DecapSecret(clientKey.D, wrongCiphertext)
	if err != nil {
		fmt.Println(KemName + ": DecapSecret sharedSecretClient failed")
		t.Fatalf("failed")
	}

	if bytes.Equal(sharedSecretClient, sharedSecretServer) {
		fmt.Println(KemName + ": shared secrets should not coincide")
		t.Fatalf("failed")
	}
}

func TestEncapBasic(t *testing.T) {
	var scheme = GetScheme()

	pub1, pri1, err := scheme.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed")
	}

	_, pri2, err := scheme.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed")
	}

	cipher1, ss1, err := scheme.Encapsulate(pub1)
	if err != nil {
		t.Fatalf("failed")
	}
	ss2, err := scheme.Decapsulate(pri1, cipher1)
	if err != nil {
		t.Fatalf("failed")
	}

	if bytes.Equal(ss1, ss2) == false {
		t.Fatalf("failed")
	}

	ss3, err := scheme.Decapsulate(pri2, cipher1)
	if err != nil {
		t.Fatalf("failed")
	}
	if bytes.Equal(ss1, ss3) == true {
		t.Fatalf("failed")
	}
}

// TestKeyEncapsulationCorrectness tests the correctness of all enabled KEMs.
func TestKeyEncapsulationCorrectness(t *testing.T) {
	testKEMCorrectness(false, t)
	wgKEMCorrectness.Add(1)
	testKEMCorrectness(true, t)
	wgKEMCorrectness.Wait()
}

// TestKeyEncapsulationWrongCiphertext tests the wrong ciphertext regime of all enabled KEMs.
func TestKeyEncapsulationWrongCiphertext(t *testing.T) {
	testKEMWrongCiphertext(false, t)
	wgKEMWrongCiphertext.Add(1)
	testKEMWrongCiphertext(true, t)
	wgKEMWrongCiphertext.Wait()
}

func csprngEntropy(n int) []byte {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic("reading from crypto/rand failed: " + err.Error())
	}
	return buf
}
