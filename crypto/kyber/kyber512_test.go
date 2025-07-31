package kyber

import (
	"bytes"
	"crypto/rand"
	"io"
	"log"
	"sync"
	"testing"
)

// wgKEMCorrectness groups goroutines and blocks the caller until all goroutines finish.
var wgKEMCorrectness sync.WaitGroup

// wgKEMWrongCiphertext groups goroutines and blocks the caller until all goroutines finish.
var wgKEMWrongCiphertext sync.WaitGroup

const KemName = "Kyber512"

func EncapSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	kem := KeyEncapsulation{}
	defer kem.Clean() // clean up even in case of panic
	err = kem.Init(KemName, nil)
	if err != nil {
		return nil, nil, err
	}
	ciphertext, sharedSecret, err = kem.EncapsulateSecret(publicKey)
	return ciphertext, sharedSecret, err
}

func DecapSecret(seckey, ciphertext []byte) ([]byte, error) {
	kem := KeyEncapsulation{}
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

	log.Println("Correctness - ", KemName) // thread-safe
	if threading == true {
		defer wgKEMCorrectness.Done()
	}
	// ignore potential errors everywhere
	clientKey, err := GenerateKemKeyPair()
	if err != nil {
		t.Errorf(KemName + ": GenerateKemKeyPair failed")
	}

	ciphertext, sharedSecretServer, err := EncapSecret(clientKey.N.Bytes())
	if err != nil {
		t.Errorf(KemName + ": EncapSecret sharedSecretServer failed")
	}

	if bytes.Equal(clientKey.N.Bytes(), ciphertext) {
		// t.Errorf is thread-safe
		t.Errorf(KemName + ": publicKey ciphertext coincides")
	}

	ciphertext1, sharedSecretServer1, err := EncapSecret(clientKey.N.Bytes())
	if err != nil {
		t.Errorf(KemName + ": EncapSecret sharedSecretServer1 failed")
	}

	if bytes.Equal(ciphertext, ciphertext1) {
		// t.Errorf is thread-safe
		t.Errorf(KemName + ": ciphertext coincides")
	}

	sharedSecretClient, err := DecapSecret(clientKey.D.Bytes(), ciphertext)
	if err != nil {
		t.Errorf(KemName + ": DecapSecret sharedSecretClient failed")
	}

	if !bytes.Equal(sharedSecretClient, sharedSecretServer) {
		// t.Errorf is thread-safe
		t.Errorf(KemName + ": shared secrets do not coincide")
	}

	sharedSecretClient1, err := DecapSecret(clientKey.D.Bytes(), ciphertext1)
	if err != nil {
		t.Errorf(KemName + ": DecapSecret sharedSecretClient1 failed")
	}

	if !bytes.Equal(sharedSecretClient1, sharedSecretServer1) {
		// t.Errorf is thread-safe
		t.Errorf(KemName + ": shared secrets do not coincide")
	}
}

// testKEMWrongCiphertext tests the wrong ciphertext regime of a specific KEM.
func testKEMWrongCiphertext(threading bool, t *testing.T) {
	if threading == true {
		defer wgKEMWrongCiphertext.Done()
	}
	// ignore potential errors everywhere
	clientKey, err := GenerateKemKeyPair()
	if err != nil {
		t.Errorf(KemName + ": GenerateKemKeyPair failed")
	}

	ciphertext, sharedSecretServer, err := EncapSecret(clientKey.N.Bytes())
	if err != nil {
		t.Errorf(KemName + ": EncapSecret sharedSecretServer failed")
	}

	wrongCiphertext := csprngEntropy(len(ciphertext))
	sharedSecretClient, err := DecapSecret(clientKey.D.Bytes(), wrongCiphertext)
	if err != nil {
		t.Errorf(KemName + ": DecapSecret sharedSecretClient failed")
	}

	if bytes.Equal(sharedSecretClient, sharedSecretServer) {
		t.Errorf(KemName + ": shared secrets should not coincide")
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
