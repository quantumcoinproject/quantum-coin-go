package keyestablishmentalgorithm

import (
	"github.com/quantumcoinproject/circl/kem/kyber/kyber512"
	"github.com/quantumcoinproject/circl/kem/mlkem/mlkem512"
	"math/big"
)

var Scheme = kyber512.Scheme()

type KeyEncap struct {
	PriKey *PrivateKey
}

func NewKeyEncap() (*KeyEncap, error) {
	return &KeyEncap{}, nil
}

func (kem *KeyEncap) Init(algName string, secretKey []byte) error {
	kem.PriKey = &PrivateKey{
		D: new(big.Int).SetBytes(secretKey),
	}

	return nil
}

func (kem *KeyEncap) Details() KeyEncapsulationDetails {
	return KeyEncapsulationDetails{
		Name:               Scheme.Name(),
		LengthPublicKey:    mlkem512.PublicKeySize,
		LengthSecretKey:    mlkem512.PrivateKeySize,
		LengthCiphertext:   mlkem512.CiphertextSize,
		LengthSharedSecret: mlkem512.SharedKeySize,
	}
}

func GenerateKemKeyPair() (*PrivateKey, error) {
	pub, pri, err := Scheme.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		return nil, err
	}
	priBytes, err := pri.MarshalBinary()
	if err != nil {
		return nil, err
	}

	privy := new(PrivateKey)
	privy.D = new(big.Int).SetBytes(priBytes)
	privy.PublicKey.N = new(big.Int).SetBytes(pubBytes)

	return privy, nil
}

func (kem *KeyEncap) GenerateKemKeyPair() (*PrivateKey, error) {
	k, err := GenerateKemKeyPair()
	if err != nil {
		return nil, err
	}
	kem.PriKey = k
	return k, nil
}

func (kem *KeyEncap) EncapsulateSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	pub, err := Scheme.UnmarshalBinaryPublicKey(publicKey)
	if err != nil {
		return nil, nil, err
	}
	return Scheme.Encapsulate(pub)
}

func (kem *KeyEncap) DecapsulateSecret(ciphertext []byte) ([]byte, error) {
	pri, err := Scheme.UnmarshalBinaryPrivateKey(kem.PriKey.D.Bytes())
	if err != nil {
		return nil, err
	}
	return Scheme.Decapsulate(pri, ciphertext)
}

func (kem *KeyEncap) Clean() {
	kem.PriKey = nil
	//no op
}
