package rlpx

import (
	"bytes"
	crypto2 "crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"errors"
	"io"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/sha3"
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
	// useFixedLabel is set once at session creation to select the HKDF label
	// encoding for the lifetime of this session. true for V2 (post-KemSwitchTime),
	// false for legacy. This avoids repeated wall-clock checks (TOCTOU).
	useFixedLabel bool

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
// in the initial HKDF-Extract). Label encoding uses hkdfEncodeLabelLegacy.
func NewSessionSecret(transcriptHash []byte, sharedSecret []byte) (*SessionSecret, error) {
	zeroKey := bytes.Repeat([]byte{0}, common.HashLength)
	earlySecret := hkdf.Extract(sha3.New256, zeroKey, transcriptHash)
	return newSessionSecretImpl(earlySecret, transcriptHash, sharedSecret, false)
}

// NewSessionSecretV2 derives handshake keys (v2 path: uses nil salt in the
// initial HKDF-Extract, matching the TLS 1.3 key schedule). Label encoding
// uses hkdfEncodeLabelFixed.
func NewSessionSecretV2(transcriptHash []byte, sharedSecret []byte) (*SessionSecret, error) {
	zeroKey := bytes.Repeat([]byte{0}, common.HashLength)
	earlySecret := hkdf.Extract(sha3.New256, zeroKey, nil)
	return newSessionSecretImpl(earlySecret, transcriptHash, sharedSecret, true)
}

func newSessionSecretImpl(earlySecret []byte, transcriptHash []byte, sharedSecret []byte, useFixed bool) (*SessionSecret, error) {
	defer zeroBytes(earlySecret)

	expand := func(secret []byte, label string, hashVal []byte, length int) ([]byte, error) {
		return hkdfExpandLabelDirect(secret, label, hashVal, length, useFixed)
	}

	emptyHash := crypto2.SHA3_256.New().Sum(nil)

	derivedSecret, err := expand(
		earlySecret,
		derivedLabelName,
		emptyHash,
		shaLength)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(derivedSecret)

	handshakeSecret := hkdf.Extract(sha3.New256, sharedSecret, derivedSecret)

	clientHandshakeTrafficSecret, err := expand(
		handshakeSecret,
		clientHandshakeTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return nil, err
	}

	serverHandshakeTrafficSecret, err := expand(
		handshakeSecret,
		serverHandshakeTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return nil, err
	}

	clientHandshakeKey, err := expand(
		clientHandshakeTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return nil, err
	}

	serverHandshakeKey, err := expand(
		serverHandshakeTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return nil, err
	}

	clientHandshakeIv, err := expand(
		clientHandshakeTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return nil, err
	}

	serverHandshakeIv, err := expand(
		serverHandshakeTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return nil, err
	}

	secret := &SessionSecret{
		useFixedLabel:                useFixed,
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
	expand := func(secret []byte, label string, hashVal []byte, length int) ([]byte, error) {
		return hkdfExpandLabelDirect(secret, label, hashVal, length, ss.useFixedLabel)
	}

	var hash crypto2.Hash
	hash = crypto2.SHA3_256
	emptyHash := hash.New().Sum(nil)

	derivedSecret, err := expand(
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

	clientApplicationTrafficSecret, err := expand(
		masterSecret,
		clientApplicationTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return err
	}
	ss.clientApplicationTrafficSecret = clientApplicationTrafficSecret

	serverApplicationTrafficSecret, err := expand(
		masterSecret,
		serverApplicationTrafficLabelName,
		transcriptHash,
		shaLength)
	if err != nil {
		return err
	}
	ss.serverApplicationTrafficSecret = serverApplicationTrafficSecret

	clientApplicationKey, err := expand(
		clientApplicationTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return err
	}
	ss.ClientApplicationKey = clientApplicationKey

	serverApplicationKey, err := expand(
		serverApplicationTrafficSecret,
		secretKeyLabelName,
		nil,
		symmetricKeySize)
	if err != nil {
		return err
	}
	ss.ServerApplicationKey = serverApplicationKey

	clientApplicationIv, err := expand(
		clientApplicationTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return err
	}
	ss.ClientApplicationIv = clientApplicationIv

	serverApplicationIv, err := expand(
		serverApplicationTrafficSecret,
		secretIvLabelName,
		nil,
		ivSize)
	if err != nil {
		return err
	}
	ss.ServerApplicationIv = serverApplicationIv

	blockApplicationClient, err := aes.NewCipher(clientApplicationKey)
	if err != nil {
		return err
	}

	ss.ClientApplicationCipher, err = cipher.NewGCM(blockApplicationClient)
	if err != nil {
		return err
	}

	blockApplicationServer, err := aes.NewCipher(serverApplicationKey)
	if err != nil {
		return err
	}

	ss.ServerApplicationCipher, err = cipher.NewGCM(blockApplicationServer)
	if err != nil {
		return err
	}

	// Zero raw key bytes now that the cipher objects hold their own internal
	// copies. The IVs must remain live for nonce calculation during the data
	// phase, so only the keys are zeroed here.
	zeroBytes(ss.ClientApplicationKey)
	ss.ClientApplicationKey = nil
	zeroBytes(ss.ServerApplicationKey)
	ss.ServerApplicationKey = nil

	return nil
}

// HkdfExpandLabel is the public entry point preserved for callers that don't
// have a SessionSecret (e.g. tests). It delegates to hkdfExpandLabelDirect
// using the useFixed flag.
func HkdfExpandLabel(secret []byte, label string, hashVal []byte, outputLength int) ([]byte, error) {
	return hkdfExpandLabelDirect(secret, label, hashVal, outputLength, true)
}

// hkdfExpandLabelDirect performs HKDF-Expand-Label with the label encoding
// selected by useFixed. This is the only function that chooses between fixed
// and legacy encoding; the choice is made once per session (not per call).
func hkdfExpandLabelDirect(secret []byte, label string, hashVal []byte, outputLength int, useFixed bool) ([]byte, error) {
	var hkdfLabel []byte
	if useFixed {
		hkdfLabel = hkdfEncodeLabelFixed(label, hashVal, outputLength)
	} else {
		hkdfLabel = hkdfEncodeLabelLegacy(label, hashVal, outputLength)
	}

	reader := hkdf.Expand(sha3.New256, secret, hkdfLabel)
	output := make([]byte, outputLength)
	if _, err := io.ReadFull(reader, output); err != nil {
		return nil, err
	}

	return output, nil
}

func hkdfEncodeLabelFixed(label string, hashVal []byte, outputLength int) []byte {
	fullLabel := "pqkem " + label

	fullLabelLen := len(fullLabel)
	if fullLabelLen > 255 {
		panic("HKDF label exceeds 255 bytes")
	}
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

// hkdfEncodeLabelLegacy will be removed after KemSwitchTime.
// NOTE: this function intentionally copies `label` (not `fullLabel`) into the
// buffer even though the length byte is set to len(fullLabel). This is a
// historical bug that all existing nodes already use. Changing it would break
// key derivation compatibility with deployed nodes. The fixed version
// (hkdfEncodeLabelFixed) is used after KemSwitchTime.
func hkdfEncodeLabelLegacy(label string, hashVal []byte, outputLength int) []byte {
	fullLabel := "pqkem " + label

	fullLabelLen := len(fullLabel)
	if fullLabelLen > 255 {
		panic("HKDF label exceeds 255 bytes")
	}
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
// server handshake traffic secret, matching TLS 1.3 (RFC 8446 Section 4.4.4).
//
// TranscriptHash must cover ClientHello through ClientVerify at the time this
// is called. The caller extends the transcript with the ServerFinished bytes
// before computing the client Finished, so the two are cryptographically bound.
func (ss *SessionSecret) ComputeServerFinished() ([]byte, error) {
	if ss.serverHandshakeTrafficSecret == nil || ss.TranscriptHash == nil {
		return nil, errors.New("required secrets are nil (handshake secrets already zeroed?)")
	}
	finishedKey, err := hkdfExpandLabelDirect(ss.serverHandshakeTrafficSecret, "s finished", nil, shaLength, ss.useFixedLabel)
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
// client handshake traffic secret. TranscriptHash must cover ClientHello
// through ServerFinished (extended after ServerFinished exchange) at the time
// this is called, so the client Finished cryptographically binds to the server
// Finished.
func (ss *SessionSecret) ComputeClientFinished() ([]byte, error) {
	if ss.clientHandshakeTrafficSecret == nil || ss.TranscriptHash == nil {
		return nil, errors.New("required secrets are nil (handshake secrets already zeroed?)")
	}
	finishedKey, err := hkdfExpandLabelDirect(ss.clientHandshakeTrafficSecret, "c finished", nil, shaLength, ss.useFixedLabel)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(finishedKey)
	mac := hmac.New(sha3.New256, finishedKey)
	mac.Write(ss.TranscriptHash)
	return mac.Sum(nil), nil
}

// ZeroPostHandshakeKeyMaterial zeros all key material that is no longer needed
// once the Finished messages have been exchanged: handshake keys/IVs/ciphers,
// traffic secrets, master secret, and the transcript hash. After this call only
// the application ciphers and IVs remain live for the data phase.
//
// Known limitation: Go's crypto/aes stores an expanded key schedule inside the
// cipher.Block object with no API to zero it. Setting the cipher fields to nil
// here only drops the reference; the expanded key bytes (176 bytes for AES-256)
// persist on the heap until GC reclaims the memory. The raw key slices
// (ClientHandshakeKey etc.) are properly zeroed, but the cipher objects'
// internal copies are not. There is no pure-Go workaround.
func (ss *SessionSecret) ZeroPostHandshakeKeyMaterial() {
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
