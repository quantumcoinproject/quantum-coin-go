package rlpx

import (
	"bytes"
	crypto2 "crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"io"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/sha3"
	"time"
)

const (
	derivedLabelName                = "derived"
	clientHandshakeTrafficLabelName = "c hs traffic"
	serverHandshakeTrafficLabelName = "s hs traffic"
	secretKeyLabelName              = "key"
	secretIvLabelName               = "iv"

	clientApplicationTrafficLabelName = "c ap traffic"
	serverApplicationTrafficLabelName = "s ap traffic"
)

type SessionSecret struct {
	handshakeSecret []byte

	clientHandshakeTrafficSecret []byte
	serverHandshakeTrafficSecret []byte
	ClientHandshakeKey           []byte
	ServerHandshakeKey           []byte
	ClientHandshakeIv            []byte
	ServerHandshakeIv            []byte

	clientApplicationTrafficSecret []byte
	serverApplicationTrafficSecret []byte
	ClientApplicationKey           []byte
	ServerApplicationKey           []byte
	ClientApplicationIv            []byte
	ServerApplicationIv            []byte

	ClientHandshakeCipher cipher.AEAD
	ServerHandshakeCipher cipher.AEAD

	ClientApplicationCipher cipher.AEAD
	ServerApplicationCipher cipher.AEAD

	masterSecret   []byte
	TranscriptHash []byte
}

// NewSessionSecret derives handshake keys (legacy path: uses transcriptHash as salt
// in the initial HKDF-Extract).
func NewSessionSecret(transcriptHash []byte, sharedSecret []byte) (*SessionSecret, error) {
	zeroKey := bytes.Repeat([]byte{0}, common.HashLength)
	earlySecret := hkdf.Extract(sha3.New256, zeroKey, transcriptHash)
	return newSessionSecretImpl(earlySecret, transcriptHash, sharedSecret)
}

// NewSessionSecretV2 derives handshake keys (v2 path: uses nil salt in the
// initial HKDF-Extract, matching the TLS 1.3 key schedule).
func NewSessionSecretV2(transcriptHash []byte, sharedSecret []byte) (*SessionSecret, error) {
	zeroKey := bytes.Repeat([]byte{0}, common.HashLength)
	earlySecret := hkdf.Extract(sha3.New256, zeroKey, nil)
	return newSessionSecretImpl(earlySecret, transcriptHash, sharedSecret)
}

func newSessionSecretImpl(earlySecret []byte, transcriptHash []byte, sharedSecret []byte) (*SessionSecret, error) {
	defer zeroBytes(earlySecret)

	emptyHash := crypto2.SHA3_256.New().Sum(nil)

	derivedSecret, err := HkdfExpandLabel(
		earlySecret,
		derivedLabelName,
		emptyHash,
		shaLength)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(derivedSecret)

	handshakeSecret := hkdf.Extract(sha3.New256, sharedSecret, derivedSecret)

	clientHandshakeTrafficSecret, err := HkdfExpandLabel(
		handshakeSecret,
		clientHandshakeTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return nil, err
	}

	serverHandshakeTrafficSecret, err := HkdfExpandLabel(
		handshakeSecret,
		serverHandshakeTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return nil, err
	}

	clientHandshakeKey, err := HkdfExpandLabel(
		clientHandshakeTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return nil, err
	}

	serverHandshakeKey, err := HkdfExpandLabel(
		serverHandshakeTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return nil, err
	}

	clientHandshakeIv, err := HkdfExpandLabel(
		clientHandshakeTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return nil, err
	}

	serverHandshakeIv, err := HkdfExpandLabel(
		serverHandshakeTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return nil, err
	}

	secret := &SessionSecret{
		handshakeSecret:              handshakeSecret,
		clientHandshakeTrafficSecret: clientHandshakeTrafficSecret,
		serverHandshakeTrafficSecret: serverHandshakeTrafficSecret,
		ClientHandshakeKey:           clientHandshakeKey,
		ServerHandshakeKey:           serverHandshakeKey,
		ClientHandshakeIv:            clientHandshakeIv,
		ServerHandshakeIv:            serverHandshakeIv,
	}

	blockHandshakeClient, err := aes.NewCipher(clientHandshakeKey)
	if err != nil {
		return nil, err
	}

	secret.ClientHandshakeCipher, err = cipher.NewGCM(blockHandshakeClient)
	if err != nil {
		return nil, err
	}

	blockHandshakeServer, err := aes.NewCipher(serverHandshakeKey)
	if err != nil {
		return nil, err
	}

	secret.ServerHandshakeCipher, err = cipher.NewGCM(blockHandshakeServer)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (ss *SessionSecret) CreateApplicationSecrets(transcriptHash []byte) error {
	var hash crypto2.Hash
	hash = crypto2.SHA3_256
	emptyHash := hash.New().Sum(nil)

	derivedSecret, err := HkdfExpandLabel(
		ss.handshakeSecret,
		derivedLabelName,
		emptyHash,
		shaLength)
	if err != nil {
		return err
	}
	defer zeroBytes(derivedSecret)

	zeroKey := bytes.Repeat([]byte{0}, common.HashLength)
	masterSecret := hkdf.Extract(sha3.New256, zeroKey, derivedSecret)
	ss.masterSecret = masterSecret
	ss.TranscriptHash = transcriptHash

	clientApplicationTrafficSecret, err := HkdfExpandLabel(
		masterSecret,
		clientApplicationTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return err
	}
	ss.clientApplicationTrafficSecret = clientApplicationTrafficSecret

	serverApplicationTrafficSecret, err := HkdfExpandLabel(
		masterSecret,
		serverApplicationTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return err
	}
	ss.serverApplicationTrafficSecret = serverApplicationTrafficSecret

	clientApplicationKey, err := HkdfExpandLabel(
		clientApplicationTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return err
	}
	ss.ClientApplicationKey = clientApplicationKey

	serverApplicationKey, err := HkdfExpandLabel(
		serverApplicationTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return err
	}
	ss.ServerApplicationKey = serverApplicationKey

	clientApplicationIv, err := HkdfExpandLabel(
		clientApplicationTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return err
	}
	ss.ClientApplicationIv = clientApplicationIv

	serverApplicationIv, err := HkdfExpandLabel(
		serverApplicationTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return err
	}
	ss.ServerApplicationIv = serverApplicationIv

	//Create the Client Application Cipher
	blockApplicationClient, err := aes.NewCipher(clientApplicationKey)
	if err != nil {
		return err
	}

	ss.ClientApplicationCipher, err = cipher.NewGCM(blockApplicationClient)
	if err != nil {
		return err
	}

	//Create the Server Application Cipher
	blockApplicationServer, err := aes.NewCipher(serverApplicationKey)
	if err != nil {
		return err
	}

	ss.ServerApplicationCipher, err = cipher.NewGCM(blockApplicationServer)
	if err != nil {
		return err
	}

	return nil
}

func HkdfExpandLabel(secret []byte, label string, hashVal []byte, outputLength int) ([]byte, error) {
	hkdfLabel := hkdfEncodeLabel(label, hashVal, outputLength)

	reader := hkdf.Expand(sha3.New256, secret, hkdfLabel)
	output := make([]byte, outputLength)
	if _, err := io.ReadFull(reader, output); err != nil {
		return nil, err
	}

	return output, nil
}

func hkdfEncodeLabel(label string, hashVal []byte, outputLength int) []byte {
	if time.Now().UTC().Unix() >= defaults.DefaultConfig.KemSwitchTime {
		return hkdfEncodeLabelFixed(label, hashVal, outputLength)
	}
	return hkdfEncodeLabelLegacy(label, hashVal, outputLength)
}

func hkdfEncodeLabelFixed(label string, hashVal []byte, outputLength int) []byte {
	fullLabel := "pqkem " + label

	fullLabelLen := len(fullLabel)
	hashLen := len(hashVal)
	hkdfLabel := make([]byte, 2+1+fullLabelLen+1+hashLen)
	hkdfLabel[0] = byte(outputLength >> 8)
	hkdfLabel[1] = byte(outputLength)
	hkdfLabel[2] = byte(fullLabelLen)
	copy(hkdfLabel[3:3+fullLabelLen], []byte(fullLabel))
	hkdfLabel[3+fullLabelLen] = byte(hashLen)
	copy(hkdfLabel[3+fullLabelLen+1:], hashVal)

	return hkdfLabel
}

// Function will be in-scope till KemSwitchTime and then be removed.
// NOTE: this function intentionally copies `label` (not `fullLabel`) into the
// buffer even though the length byte is set to len(fullLabel). This is a
// historical bug that all existing nodes already use. Changing it would break
// key derivation compatibility with deployed nodes. The fixed version
// (hkdfEncodeLabelFixed) is used after KemSwitchTime.
func hkdfEncodeLabelLegacy(label string, hashVal []byte, outputLength int) []byte {
	fullLabel := "pqkem " + label

	fullLabelLen := len(fullLabel)
	hashLen := len(hashVal)
	hkdfLabel := make([]byte, 2+1+fullLabelLen+1+hashLen)
	hkdfLabel[0] = byte(outputLength >> 8)
	hkdfLabel[1] = byte(outputLength)
	hkdfLabel[2] = byte(fullLabelLen)
	copy(hkdfLabel[3:3+fullLabelLen], []byte(label))
	hkdfLabel[3+fullLabelLen] = byte(hashLen)
	copy(hkdfLabel[3+fullLabelLen+1:], hashVal)

	return hkdfLabel
}

// ComputeServerFinished computes the server's Finished verify data for explicit
// key confirmation. The HMAC is keyed with a finished-key derived from the
// server application traffic secret.
func (ss *SessionSecret) ComputeServerFinished() ([]byte, error) {
	finishedKey, err := HkdfExpandLabel(ss.serverApplicationTrafficSecret, "s finished", nil, shaLength)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(finishedKey)
	mac := hmac.New(sha3.New256, finishedKey)
	mac.Write(ss.TranscriptHash)
	return mac.Sum(nil), nil
}

// ComputeClientFinished computes the client's Finished verify data for explicit
// key confirmation. The HMAC is keyed with a finished-key derived from the
// client application traffic secret.
func (ss *SessionSecret) ComputeClientFinished() ([]byte, error) {
	finishedKey, err := HkdfExpandLabel(ss.clientApplicationTrafficSecret, "c finished", nil, shaLength)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(finishedKey)
	mac := hmac.New(sha3.New256, finishedKey)
	mac.Write(ss.TranscriptHash)
	return mac.Sum(nil), nil
}

// ZeroHandshakeSecrets zeros handshake-phase key material that is no longer
// needed once the Finished messages have been exchanged. This limits the
// window during which an attacker with memory access could recover keys.
func (ss *SessionSecret) ZeroHandshakeSecrets() {
	zeroBytes(ss.handshakeSecret)
	ss.handshakeSecret = nil
	zeroBytes(ss.clientHandshakeTrafficSecret)
	ss.clientHandshakeTrafficSecret = nil
	zeroBytes(ss.serverHandshakeTrafficSecret)
	ss.serverHandshakeTrafficSecret = nil
	zeroBytes(ss.ClientHandshakeKey)
	ss.ClientHandshakeKey = nil
	zeroBytes(ss.ServerHandshakeKey)
	ss.ServerHandshakeKey = nil
	zeroBytes(ss.ClientHandshakeIv)
	ss.ClientHandshakeIv = nil
	zeroBytes(ss.ServerHandshakeIv)
	ss.ServerHandshakeIv = nil
	ss.ClientHandshakeCipher = nil
	ss.ServerHandshakeCipher = nil

	zeroBytes(ss.masterSecret)
	ss.masterSecret = nil
	zeroBytes(ss.clientApplicationTrafficSecret)
	ss.clientApplicationTrafficSecret = nil
	zeroBytes(ss.serverApplicationTrafficSecret)
	ss.serverApplicationTrafficSecret = nil
	zeroBytes(ss.TranscriptHash)
	ss.TranscriptHash = nil
}

func (ss *SessionSecret) ZeroSecrets() {
	zeroBytes(ss.handshakeSecret)
	zeroBytes(ss.clientHandshakeTrafficSecret)
	zeroBytes(ss.serverHandshakeTrafficSecret)
	zeroBytes(ss.ClientHandshakeKey)
	zeroBytes(ss.ServerHandshakeKey)
	zeroBytes(ss.ClientHandshakeIv)
	zeroBytes(ss.ServerHandshakeIv)
	zeroBytes(ss.clientApplicationTrafficSecret)
	zeroBytes(ss.serverApplicationTrafficSecret)
	zeroBytes(ss.ClientApplicationKey)
	zeroBytes(ss.ServerApplicationKey)
	zeroBytes(ss.ClientApplicationIv)
	zeroBytes(ss.ServerApplicationIv)
	zeroBytes(ss.masterSecret)
	zeroBytes(ss.TranscriptHash)

	ss.ClientHandshakeCipher = nil
	ss.ServerHandshakeCipher = nil
	ss.ClientApplicationCipher = nil
	ss.ServerApplicationCipher = nil
}
