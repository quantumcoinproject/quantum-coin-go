package kyber

import "github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"

type KeyEncapsulation struct {
}

func (kem *KeyEncapsulation) Init(algName string, secretKey []byte) error {
	return nil
}

func (kem *KeyEncapsulation) Details() keyestablishmentalgorithm.KeyEncapsulationDetails {
	return keyestablishmentalgorithm.KeyEncapsulationDetails{}
}

func (kem *KeyEncapsulation) GenerateKemKeyPair() (*keyestablishmentalgorithm.PrivateKey, error) {
	return nil, nil
}

func (kem *KeyEncapsulation) EncapsulateSecret(publicKey []byte) (ciphertext,
	sharedSecret []byte, err error) {
	return nil, nil, err
}

func (kem *KeyEncapsulation) DecapsulateSecret(ciphertext []byte) ([]byte, error) {
	return nil, nil
}
