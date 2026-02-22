package keyestablishmentalgorithm

import (
	"github.com/quantumcoinproject/circl/kem"
	"github.com/quantumcoinproject/circl/kem/hybrid"
)

func GetScheme() kem.Scheme {
	return hybrid.X25519MLKEM768()
}

type KeyEncap struct {
	PriKey *PrivateKey
}

func NewKeyEncap() (*KeyEncap, error) {
	return &KeyEncap{}, nil
}

func (kem *KeyEncap) Init(algName string, secretKey []byte) error {
	kem.PriKey = &PrivateKey{
		D: append([]byte(nil), secretKey...),
	}

	return nil
}

func (kem *KeyEncap) Details() KeyEncapsulationDetails {
	scheme := GetScheme()
	return KeyEncapsulationDetails{
		Name:               scheme.Name(),
		LengthPublicKey:    scheme.PublicKeySize(),
		LengthSecretKey:    scheme.PrivateKeySize(),
		LengthCiphertext:   scheme.CiphertextSize(),
		LengthSharedSecret: scheme.SharedKeySize(),
	}
}

func GenerateKemKeyPair() (*PrivateKey, error) {
	scheme := GetScheme()
	pub, pri, err := scheme.GenerateKeyPair()
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
	privy.D = make([]byte, len(priBytes))
	copy(privy.D, priBytes)
	privy.PublicKey.N = make([]byte, len(pubBytes))
	copy(privy.PublicKey.N, pubBytes)

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
	scheme := GetScheme()
	pub, err := scheme.UnmarshalBinaryPublicKey(publicKey)
	if err != nil {
		return nil, nil, err
	}
	return scheme.Encapsulate(pub)
}

func (kem *KeyEncap) DecapsulateSecret(ciphertext []byte) ([]byte, error) {
	scheme := GetScheme()
	pri, err := scheme.UnmarshalBinaryPrivateKey(kem.PriKey.D)
	if err != nil {
		return nil, err
	}
	return scheme.Decapsulate(pri, ciphertext)
}

func (kem *KeyEncap) Clean() {
	if kem.PriKey != nil {
		for i := range kem.PriKey.D {
			kem.PriKey.D[i] = 0
		}
		kem.PriKey = nil
	}
}
