package keyestablishmentalgorithm

import (
	"github.com/quantumcoinproject/circl/kem"
	"github.com/quantumcoinproject/circl/kem/hybrid"
	"github.com/quantumcoinproject/circl/kem/kyber/kyber512"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"math/big"
	"time"
)

func GetScheme() kem.Scheme {
	if time.Now().UTC().Unix() < defaults.DefaultConfig.KemSwitchTime && schemeOverride == false {
		return kyber512.Scheme()
	} else {
		return hybrid.X25519MLKEM768()
	}
}

var schemeOverride = false

type KeyEncap struct {
	PriKey *PrivateKey
}

func SetSchemeHybrid() { //test hook
	schemeOverride = true
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
	//todo: race condition during GetScheme switch
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
	scheme := GetScheme()
	pub, err := scheme.UnmarshalBinaryPublicKey(publicKey)
	if err != nil {
		return nil, nil, err
	}
	return scheme.Encapsulate(pub)
}

func (kem *KeyEncap) DecapsulateSecret(ciphertext []byte) ([]byte, error) {
	scheme := GetScheme()
	pri, err := scheme.UnmarshalBinaryPrivateKey(kem.PriKey.D.Bytes())
	if err != nil {
		return nil, err
	}
	return scheme.Decapsulate(pri, ciphertext)
}

func (kem *KeyEncap) Clean() {
	kem.PriKey = nil
	//no op
}
