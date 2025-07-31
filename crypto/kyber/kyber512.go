package kyber

import (
	"crypto/rand"
	"github.com/quantumcoinproject/circl/kem/kyber/kyber512"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"math/big"
)

type KeyEncapsulation struct {
	PriKey *keyestablishmentalgorithm.PrivateKey
}

func NewKeyEncapsulation() (*KeyEncapsulation, error) {
	pri, err := GenerateKemKeyPair()
	if err != nil {
		return nil, err
	}

	return &KeyEncapsulation{
		PriKey: pri,
	}, nil
}

func (kem *KeyEncapsulation) Init(algName string, secretKey []byte) error {

	kem.PriKey = &keyestablishmentalgorithm.PrivateKey{
		D: new(big.Int).SetBytes(secretKey),
	}

	return nil
}

func (kem *KeyEncapsulation) Details() keyestablishmentalgorithm.KeyEncapsulationDetails {
	return keyestablishmentalgorithm.KeyEncapsulationDetails{
		Name:               "Kyber512",
		LengthPublicKey:    800,
		LengthSecretKey:    1632,
		LengthCiphertext:   768,
		LengthSharedSecret: 32,
	}
}

func GenerateKemKeyPair() (*keyestablishmentalgorithm.PrivateKey, error) {
	pub, pri, err := kyber512.GenerateKeyPair(rand.Reader)
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

	privy := new(keyestablishmentalgorithm.PrivateKey)
	privy.D = new(big.Int).SetBytes(priBytes)
	privy.PublicKey.N = new(big.Int).SetBytes(pubBytes)

	return privy, nil
}

func (kem *KeyEncapsulation) GenerateKemKeyPair() (*keyestablishmentalgorithm.PrivateKey, error) {
	return GenerateKemKeyPair()
}

func (kem *KeyEncapsulation) EncapsulateSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error) {
	pub, err := kyber512.Scheme().UnmarshalBinaryPublicKey(publicKey)
	if err != nil {
		return nil, nil, err
	}
	return kyber512.Scheme().Encapsulate(pub)
}

func (kem *KeyEncapsulation) DecapsulateSecret(ciphertext []byte) ([]byte, error) {
	pri, err := kyber512.Scheme().UnmarshalBinaryPrivateKey(kem.PriKey.D.Bytes())
	if err != nil {
		return nil, err
	}
	return kyber512.Scheme().Decapsulate(pri, ciphertext)
}

func (kem *KeyEncapsulation) Clean() {
	//no op
}
