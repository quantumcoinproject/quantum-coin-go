package keyestablishmentalgorithm

type KeyEncapsulationDetails struct {
	ClaimedNISTLevel   int
	IsINDCCA           bool
	LengthCiphertext   int
	LengthPublicKey    int
	LengthSecretKey    int
	LengthSharedSecret int
	Name               string
	Version            string
}

type PublicKey struct {
	Key []byte
}
type PrivateKey struct {
	Public *PublicKey
	Key    []byte
}

type KeyEncapsulation interface {
	Init(algName string, secretKey []byte) error
	Details() KeyEncapsulationDetails
	EncapSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error)
	DecapSecret(seckey, ciphertext []byte) ([]byte, error)
	GenerateKemKeyPair() (*PrivateKey, error)
	EncapsulateSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error)
	DecapsulateSecret(ciphertext []byte) ([]byte, error)
}
