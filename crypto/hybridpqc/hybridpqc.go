package hybridpqc

import (
	"errors"
	"github.com/quantumcoinproject/circl/sign/mldsa/mldsa44"
	"github.com/quantumcoinproject/circl/sign/slhdsa"
)

const CRYPTO_DILITHIUM_PUBLICKEY_BYTES = 1312

func VerifyDilithium(digestHash []byte, signature []byte, publicKey []byte) error {

	var pubKey mldsa44.PublicKey

	var buf2 [CRYPTO_DILITHIUM_PUBLICKEY_BYTES]byte
	copy(buf2[:], publicKey)

	pubKey.Unpack(&buf2)

	if mldsa44.VerifyNoContext(&pubKey, digestHash, signature) == false {
		return errors.New("verify failed")
	}

	return nil
}

func VerifySphincs(digestHash []byte, signature []byte, publicKey []byte) error {

	var pubKey slhdsa.PublicKey
	pubKey.ID = slhdsa.SHAKE_256f

	var buf2 [CRYPTO_DILITHIUM_PUBLICKEY_BYTES]byte
	copy(buf2[:], publicKey)

	err := pubKey.UnmarshalBinary(publicKey)
	if err != nil {
		return err
	}

	if slhdsa.VerifyNoContext(&pubKey, digestHash, signature) == false {
		return errors.New("verify failed")
	}

	return nil
}
