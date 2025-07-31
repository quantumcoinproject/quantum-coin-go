package keyestablishmentalgorithm

import (
	"math/big"
)

type KeyEncapsulationDetails struct {
	//ClaimedNISTLevel   int
	//IsINDCCA           bool
	Name               string
	LengthCiphertext   int
	LengthPublicKey    int
	LengthSecretKey    int
	LengthSharedSecret int
	//Version            string
}

type PublicKey struct {
	N *big.Int // public key bytes
}

type PrivateKey struct {
	PublicKey          // public part.
	D         *big.Int // private key bytes
}

type KeyEncapsulation interface {
	Init(algName string, secretKey []byte) error
	Details() KeyEncapsulationDetails
	GenerateKemKeyPair() (*PrivateKey, error)
	EncapsulateSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error)
	DecapsulateSecret(ciphertext []byte) ([]byte, error)
	Clean()
}
