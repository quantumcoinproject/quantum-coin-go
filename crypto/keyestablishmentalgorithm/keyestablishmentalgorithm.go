package keyestablishmentalgorithm

type KeyEncapsulationDetails struct {
	Name               string
	LengthCiphertext   int
	LengthPublicKey    int
	LengthSecretKey    int
	LengthSharedSecret int
}

type PublicKey struct {
	N []byte // public key bytes
}

type PrivateKey struct {
	PublicKey        // public part.
	D        []byte // private key bytes
}

type KeyEncapsulation interface {
	Init(algName string, secretKey []byte) error
	Details() KeyEncapsulationDetails
	GenerateKemKeyPair() (*PrivateKey, error)
	EncapsulateSecret(publicKey []byte) (ciphertext, sharedSecret []byte, err error)
	DecapsulateSecret(ciphertext []byte) ([]byte, error)
	Clean()
}
