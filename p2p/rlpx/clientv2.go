package rlpx

import (
	"bytes"
	cipher2 "crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// ClientV2 implements the v2 RLPx transport (after KemSwitchTime): encrypted headers,
// EncryptedPayload RLP format, sequence numbers starting at 0, deterministic AAD.
type ClientV2 struct {
	ephemeralKemPrivateKey  *keyestablishmentalgorithm.PrivateKey
	kem                     *keyestablishmentalgorithm.KeyEncapsulation
	kemCipherText           []byte
	kemSharedSecret         []byte
	Nonce                   uint
	clientSigningPrivateKey *signaturealgorithm.PrivateKey
	serverSigningPublicKey  *signaturealgorithm.PublicKey

	cliHelloMessage  *ClientHelloMessage
	srvHelloMessage  *ServerHelloMessage
	srvVerifyMessage *ServerVerifyMessage
	cliVerifyMessage *ClientVerifyMessage

	rbuf        ReadBuffer
	wbuf        WriteBuffer
	RecordCount uint

	secret SessionSecret

	conn io.ReadWriter

	serializer Serializer

	serverSeqNumHandshake uint64
	clientSeqNumHandshake uint64

	serverSeqNumApplication uint64
	clientSeqNumApplication uint64

	transcript []byte

	server *ServerV2

	handshakeDone atomic.Bool
	closed        atomic.Bool
	mutex         sync.Mutex
	writeMutex    sync.Mutex
	readMutex     sync.Mutex

	context string
}

func (c *ClientV2) SetServer(server *ServerV2) {
	c.server = server
}

func NewClientV2(conn io.ReadWriter, clientSigningPrivateKey *signaturealgorithm.PrivateKey, serverSigningPublicKey *signaturealgorithm.PublicKey, context string) *ClientV2 {
	log.Debug("NewClientV2")
	client := ClientV2{
		conn:                    conn,
		clientSigningPrivateKey: clientSigningPrivateKey,
		serverSigningPublicKey:  serverSigningPublicKey,
	}

	client.serializer = NewRlpxSerializer()
	client.serializer.SetContext("client " + context)
	client.context = context
	client.serverSeqNumHandshake = 0
	client.clientSeqNumHandshake = 0
	client.serverSeqNumApplication = 0
	client.clientSeqNumApplication = 0

	return &client
}

func (c *ClientV2) SetClientSigningPrivateKey(prv *signaturealgorithm.PrivateKey) {
	c.clientSigningPrivateKey = prv
}

func (c *ClientV2) SetServerSigningPublicKey(pub *signaturealgorithm.PublicKey) {
	c.serverSigningPublicKey = pub
}

func (c *ClientV2) ServerSigningPublicKey() *signaturealgorithm.PublicKey {
	return c.serverSigningPublicKey
}

func (c *ClientV2) PerformHandshake() (retErr error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// #11: zero all secrets on handshake failure
	defer func() {
		if retErr != nil {
			zeroBytes(c.kemSharedSecret)
			c.kemSharedSecret = nil
			c.secret.ZeroSecrets()
			if c.kem != nil {
				k := *c.kem
				k.Clean()
				c.kem = nil
			}
			if c.ephemeralKemPrivateKey != nil {
				zeroBytes(c.ephemeralKemPrivateKey.D)
				c.ephemeralKemPrivateKey = nil
			}
		}
	}()

	if c.handshakeDone.Load() {
		retErr = errors.New("Handshake already done")
		return
	}

	c.kem, retErr = NewKem("client")
	if retErr != nil {
		return
	}

	retErr = c.makeClientHelloV2()
	if retErr != nil {
		return
	}

	// #12: save actual wire bytes for transcript (not re-serialized)
	clientHelloPacket, err := c.serializer.Serialize(c.cliHelloMessage)
	if err != nil {
		retErr = err
		return
	}
	if _, err = c.conn.Write(clientHelloPacket); err != nil {
		retErr = err
		return
	}

	serverHelloMessage := new(ServerHelloMessage)
	serverHelloRaw, err := c.serializer.Deserialize(serverHelloMessage, c.conn)
	if err != nil {
		retErr = err
		return
	}

	c.srvHelloMessage = serverHelloMessage
	retErr = c.handleServerHelloV2()
	if retErr != nil {
		return
	}

	// #12: use wire bytes for transcript to prevent padding malleability
	transcript := append(clientHelloPacket[2:], serverHelloRaw...)
	transcriptHash := sha3Sum256(transcript)
	c.transcript = transcript

	secret, err := NewSessionSecretV2(transcriptHash, c.kemSharedSecret[:])
	if err != nil {
		retErr = err
		return
	}
	c.secret = *secret
	zeroBytes(c.kemSharedSecret)
	c.kemSharedSecret = nil
	if c.kem != nil {
		k := *c.kem
		k.Clean()
		c.kem = nil
	}
	// #10: zero KEM private key bytes before dropping reference
	if c.ephemeralKemPrivateKey != nil {
		zeroBytes(c.ephemeralKemPrivateKey.D)
		c.ephemeralKemPrivateKey = nil
	}

	serverVerifyMessage := new(ServerVerifyMessage)
	retErr = c.ReadAndDecryptMessageV2(serverVerifyMessage, PacketTypeHandshake)
	if retErr != nil {
		return
	}

	serverPubKeyDataLocal, err := cryptobase.SigAlg.SerializePublicKey(c.serverSigningPublicKey)
	if err != nil {
		retErr = err
		return
	}
	// #2: reject empty signatures at the protocol level
	if serverVerifyMessage.SignatureLen == 0 {
		retErr = errors.New("empty signature")
		return
	}
	if serverVerifyMessage.SignatureLen > uint(len(serverVerifyMessage.Signature)) {
		retErr = errors.New("invalid signature length")
		return
	}
	sig := serverVerifyMessage.Signature[:serverVerifyMessage.SignatureLen]
	serverPubKeyDataRemote, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(transcriptHash, sig)
	if err != nil {
		retErr = err
		return
	}
	if !bytes.Equal(serverPubKeyDataLocal, serverPubKeyDataRemote) {
		log.Trace("Public Key mismatch",
			"serverSigningPublicKey", base64.StdEncoding.EncodeToString(c.serverSigningPublicKey.PubData),
			"signature", base64.StdEncoding.EncodeToString(sig),
			"serverPubKeyDataRemote", base64.StdEncoding.EncodeToString(serverPubKeyDataRemote))
		retErr = errors.New("Public key mismatch")
		return
	}
	// #13: use SigAlg.Verify (static algorithm) instead of DynamicSigVerifier
	if !cryptobase.SigAlg.Verify(serverPubKeyDataLocal, transcriptHash, sig) {
		retErr = errors.New("server's signature verification failed")
		return
	}

	serverVerifyTranscript, err := c.serializer.SerializeDeterministic(serverVerifyMessage, 0)
	if err != nil {
		retErr = err
		return
	}
	transcript = append(transcript, serverVerifyTranscript...)
	transcriptHash = sha3Sum256(transcript)
	c.transcript = transcript
	c.srvVerifyMessage = serverVerifyMessage

	signature, err := cryptobase.SigAlg.Sign(transcriptHash, c.clientSigningPrivateKey)
	if err != nil {
		retErr = err
		return
	}
	clientVerifyMessage := new(ClientVerifyMessage)
	clientVerifyMessage.Signature = make([]byte, cryptobase.SigAlg.SignatureWithPublicKeyLength())
	copy(clientVerifyMessage.Signature[:], signature)
	clientVerifyMessage.SignatureLen = uint(len(signature))
	c.cliVerifyMessage = clientVerifyMessage

	clientVerifyPacket, err := c.serializer.Serialize(clientVerifyMessage)
	if err != nil {
		retErr = err
		return
	}

	retErr = c.WriteEncrypted(clientVerifyPacket, 0, PacketTypeHandshake)
	if retErr != nil {
		return
	}

	clientVerifyTranscript, err := c.serializer.SerializeDeterministic(clientVerifyMessage, 0)
	if err != nil {
		retErr = err
		return
	}
	transcript = append(transcript, clientVerifyTranscript...)
	c.transcript = transcript

	tHash := sha3Sum256(c.transcript)
	retErr = c.secret.CreateApplicationSecrets(tHash)
	if retErr != nil {
		return
	}

	serverFinishedMessage := new(FinishedMessage)
	retErr = c.ReadAndDecryptMessageV2(serverFinishedMessage, PacketTypeHandshake)
	if retErr != nil {
		return
	}
	if len(serverFinishedMessage.Rest) > 0 {
		retErr = errors.New("unexpected data in finished message")
		return
	}
	expectedServerFinished, err := c.secret.ComputeServerFinished()
	if err != nil {
		retErr = err
		return
	}
	if subtle.ConstantTimeCompare(serverFinishedMessage.VerifyData, expectedServerFinished) != 1 {
		retErr = errors.New("server finished verification failed")
		return
	}

	// Extend transcript with ServerFinished so the ClientFinished HMAC
	// cryptographically binds to the ServerFinished, matching TLS 1.3.
	serverFinishedTranscript, err := c.serializer.SerializeDeterministic(serverFinishedMessage, 0)
	if err != nil {
		retErr = err
		return
	}
	transcript = append(transcript, serverFinishedTranscript...)
	c.transcript = transcript
	c.secret.TranscriptHash = sha3Sum256(c.transcript)

	clientFinishedData, err := c.secret.ComputeClientFinished()
	if err != nil {
		retErr = err
		return
	}
	clientFinishedMessage := &FinishedMessage{VerifyData: clientFinishedData}
	clientFinishedPacket, err := c.serializer.Serialize(clientFinishedMessage)
	if err != nil {
		retErr = err
		return
	}
	retErr = c.WriteEncrypted(clientFinishedPacket, 0, PacketTypeHandshake)
	if retErr != nil {
		return
	}

	c.secret.ZeroPostHandshakeKeyMaterial()
	c.handshakeDone.Store(true)
	return nil
}

func (c *ClientV2) makeClientHelloV2() error {
	clientHelloMessage := new(ClientHelloMessage)
	clientHelloMessage.Version = handshakeVersion

	k := *c.kem
	kemPrivateKey, err := k.GenerateKemKeyPair()
	if err != nil {
		return err
	}
	c.ephemeralKemPrivateKey = kemPrivateKey
	clientHelloMessage.ClientKemPublicKey = make([]byte, k.Details().LengthPublicKey)
	copy(clientHelloMessage.ClientKemPublicKey[:], c.ephemeralKemPrivateKey.N)

	randomData := make([]byte, shaLength)
	if _, err := rand.Read(randomData); err != nil {
		return err
	}
	copy(clientHelloMessage.ClientHelloRandomData[:], randomData)
	c.Nonce = 1
	c.cliHelloMessage = clientHelloMessage
	return nil
}

func (c *ClientV2) handleServerHelloV2() error {
	if c.srvHelloMessage.Version != handshakeVersion {
		return errors.New("unsupported handshake version")
	}
	k := *c.kem
	// #5: validate ciphertext length before KEM operation
	if len(c.srvHelloMessage.CipherText) != k.Details().LengthCiphertext {
		return errors.New("invalid KEM ciphertext length")
	}
	sharedSecret, err := k.DecapsulateSecret(c.srvHelloMessage.CipherText[:])
	if err != nil {
		return err
	}
	// #6: validate shared secret is non-empty and non-zero
	if len(sharedSecret) != k.Details().LengthSharedSecret {
		return errors.New("KEM shared secret has unexpected length")
	}
	if isAllZeros(sharedSecret) {
		return errors.New("KEM shared secret is all zeros")
	}
	c.kemSharedSecret = make([]byte, k.Details().LengthSharedSecret)
	copy(c.kemSharedSecret[:], sharedSecret[:])
	return nil
}

func (c *ClientV2) Cleanup() {
	// Mark closed first so that any later WriteEncrypted/ReadAndDecrypt bail out
	// once they observe the flag under the relevant lock.
	c.closed.Store(true)
	// Acquire both locks so we cannot zero the cipher/IV material while a
	// concurrent WriteEncrypted or (application-phase) ReadAndDecrypt is using
	// it. Conn.Close closes the underlying connection before calling Cleanup,
	// so any blocked I/O returns promptly and releases these locks.
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	c.readMutex.Lock()
	defer c.readMutex.Unlock()
	if c.kem != nil {
		k := *c.kem
		k.Clean()
	}
	if c.ephemeralKemPrivateKey != nil {
		zeroBytes(c.ephemeralKemPrivateKey.D)
		c.ephemeralKemPrivateKey = nil
	}
	zeroBytes(c.kemSharedSecret)
	c.secret.ZeroSecrets()
}

func (c *ClientV2) ReadAndDecryptMessageV2(msg interface{}, packetType PacketType) error {
	dataPacket, err := c.ReadAndDecrypt(packetType)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(dataPacket.fragment)
	_, err = c.serializer.Deserialize(msg, reader)
	return err
}

func (c *ClientV2) WriteEncrypted(data []byte, context uint64, packetType PacketType) error {
	if packetType == PacketTypeApplicationData && !c.handshakeDone.Load() {
		return errors.New("handshake not completed")
	}
	// #3: lock unconditionally to prevent nonce reuse from concurrent calls
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	// Cleanup() acquires writeMutex before zeroing the session secrets, so once
	// we hold the lock the closed flag and cipher fields are stable. Reject
	// writes that race connection teardown instead of dereferencing nil ciphers.
	if c.closed.Load() {
		return errors.New("connection closed")
	}

	var cipher cipher2.AEAD
	var seqNum uint64
	var clientIv []byte
	if packetType == PacketTypeHandshake {
		cipher = c.secret.ClientHandshakeCipher
		seqNum = c.clientSeqNumHandshake
		clientIv = c.secret.ClientHandshakeIv
	} else {
		cipher = c.secret.ClientApplicationCipher
		seqNum = c.clientSeqNumApplication
		clientIv = c.secret.ClientApplicationIv
	}
	if cipher == nil {
		return errors.New("cipher unavailable")
	}

	payload := &EncryptedPayload{
		PacketType: uint(packetType),
		Context:    context,
		Fragment:   data,
	}
	payloadData, err := rlp.EncodeToBytes(payload)
	if err != nil {
		return err
	}

	encryptedLen := uint(len(payloadData) + cipher.Overhead())
	header := new(Header)
	header.MinorVersion = minorVersionV2
	header.RecordLength = encryptedLen
	header.AdditionalData = BuildAADV2(minorVersionV2, encryptedLen)

	nonce, err := CalculateNonceV2(seqNum, clientIv)
	if err != nil {
		return err
	}
	encryptedData := cipher.Seal(nil, nonce, payloadData, header.AdditionalData[:])

	headerPacket, err := c.serializer.Serialize(header)
	if err != nil {
		return err
	}
	buf := make([]byte, len(headerPacket)+len(encryptedData))
	copy(buf, headerPacket)
	copy(buf[len(headerPacket):], encryptedData)
	if _, err = c.conn.Write(buf); err != nil {
		return err
	}

	if packetType == PacketTypeHandshake {
		c.clientSeqNumHandshake++
	} else {
		c.clientSeqNumApplication++
	}
	return nil
}

func (c *ClientV2) ReadAndDecrypt(packetType PacketType) (*DataPacket, error) {
	if packetType == PacketTypeApplicationData {
		if !c.handshakeDone.Load() {
			return nil, errors.New("handshake not completed")
		}
		c.readMutex.Lock()
		defer c.readMutex.Unlock()
		if c.closed.Load() {
			return nil, errors.New("connection closed")
		}
	}

	var cipher cipher2.AEAD
	var seqNum uint64
	var serverIv []byte
	if packetType == PacketTypeHandshake {
		cipher = c.secret.ServerHandshakeCipher
		seqNum = c.serverSeqNumHandshake
		serverIv = c.secret.ServerHandshakeIv
	} else {
		cipher = c.secret.ServerApplicationCipher
		seqNum = c.serverSeqNumApplication
		serverIv = c.secret.ServerApplicationIv
	}
	if cipher == nil {
		return nil, errors.New("cipher unavailable")
	}

	header := new(Header)
	_, err := c.serializer.Deserialize(header, c.conn)
	if err != nil {
		return nil, err
	}
	if header.MinorVersion != minorVersionV2 {
		return nil, errors.New("unsupported transport version")
	}
	if len(header.Rest) > 0 {
		return nil, errors.New("unexpected data in header")
	}

	maxRecLen := uint(maxRecordLengthV2)
	if packetType == PacketTypeHandshake {
		maxRecLen = maxHandshakeRecordLengthV2
	}
	if header.RecordLength > maxRecLen {
		return nil, errors.New("record length exceeds maximum allowed size")
	}

	reconstructedAAD := BuildAADV2(header.MinorVersion, header.RecordLength)
	if reconstructedAAD != header.AdditionalData {
		return nil, errors.New("header AAD mismatch")
	}

	recLen := int(header.RecordLength)
	encryptedData := make([]byte, recLen)
	bytesRead, err := io.ReadAtLeast(c.conn, encryptedData, recLen)
	if err != nil {
		return nil, err
	}
	if bytesRead != recLen {
		return nil, errors.New("prefix size less")
	}

	nonce, err := CalculateNonceV2(seqNum, serverIv)
	if err != nil {
		return nil, err
	}
	decryptedPayloadBytes, err := cipher.Open(nil, nonce, encryptedData, reconstructedAAD[:])
	if err != nil {
		return nil, err
	}

	var encryptedPayload EncryptedPayload
	if err := rlp.DecodeBytes(decryptedPayloadBytes, &encryptedPayload); err != nil {
		return nil, err
	}
	if len(encryptedPayload.Rest) > 0 {
		return nil, errors.New("unexpected data in encrypted payload")
	}

	dataPacket := &DataPacket{
		packetType: PacketType(encryptedPayload.PacketType),
		seqNum:     seqNum,
		fragment:   encryptedPayload.Fragment,
		context:    encryptedPayload.Context,
	}
	if dataPacket.packetType != packetType {
		return nil, errors.New("packetType mismatch")
	}

	if packetType == PacketTypeHandshake {
		c.serverSeqNumHandshake++
	} else {
		c.serverSeqNumApplication++
	}
	return dataPacket, nil
}

func (c *ClientV2) InitWithSecrets(secret SessionSecret) {
	c.secret = secret
	c.handshakeDone.Store(true)
}
