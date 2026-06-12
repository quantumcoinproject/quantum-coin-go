package rlpx

import (
	"bytes"
	cipher2 "crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
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

// ServerV2 implements the v2 RLPx transport (after KemSwitchTime).
type ServerV2 struct {
	ephemeralKemPrivateKey  *keyestablishmentalgorithm.PrivateKey
	kem                     *keyestablishmentalgorithm.KeyEncapsulation
	serverSigningPrivateKey *signaturealgorithm.PrivateKey
	clientSigningPublicKey  *signaturealgorithm.PublicKey

	rbuf ReadBuffer
	wbuf WriteBuffer

	cliHelloMessage  *ClientHelloMessage
	srvHelloMessage  *ServerHelloMessage
	srvVerifyMessage *ServerVerifyMessage
	cliVerifyMessage *ClientVerifyMessage

	kemCipherText   []byte
	kemSharedSecret []byte

	serverSeqNumHandshake uint64
	clientSeqNumHandshake uint64

	serverSeqNumApplication uint64
	clientSeqNumApplication uint64

	secret SessionSecret

	conn io.ReadWriter

	serializer Serializer

	transcript []byte

	client *ClientV2

	handshakeDone atomic.Bool
	closed        atomic.Bool
	mutex         sync.Mutex
	writeMutex    sync.Mutex
	readMutex     sync.Mutex

	context string
}

func (s *ServerV2) SetClient(client *ClientV2) {
	s.client = client
}

func NewServerV2(conn io.ReadWriter, serverSigningPrivateKey *signaturealgorithm.PrivateKey, context string) *ServerV2 {
	log.Debug("NewServerV2")
	server := ServerV2{
		conn:                    conn,
		serverSigningPrivateKey: serverSigningPrivateKey,
		context:                 context,
	}

	server.serializer = NewRlpxSerializer()
	server.serializer.SetContext("server " + context)
	server.serverSeqNumHandshake = 0
	server.clientSeqNumHandshake = 0
	server.serverSeqNumApplication = 0
	server.clientSeqNumApplication = 0

	return &server
}

func (s *ServerV2) SetServerSigningPrivateKey(prv *signaturealgorithm.PrivateKey) {
	s.serverSigningPrivateKey = prv
}

func (s *ServerV2) ClientSigningPublicKey() *signaturealgorithm.PublicKey {
	return s.clientSigningPublicKey
}

func (s *ServerV2) PerformHandshake() (retErr error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// #11: zero all secrets on handshake failure
	defer func() {
		if retErr != nil {
			zeroBytes(s.kemSharedSecret)
			s.kemSharedSecret = nil
			s.secret.ZeroSecrets()
			if s.kem != nil {
				k := *s.kem
				k.Clean()
				s.kem = nil
			}
		}
	}()

	if s.handshakeDone.Load() {
		retErr = errors.New("Handshake already done")
		return
	}

	s.kem, retErr = NewKem("server")
	if retErr != nil {
		return
	}

	clientHelloMessage := new(ClientHelloMessage)
	// #12: save actual wire bytes for transcript
	clientHelloRaw, err := s.serializer.Deserialize(clientHelloMessage, s.conn)
	if err != nil {
		retErr = err
		return
	}

	s.cliHelloMessage = clientHelloMessage
	retErr = s.handleClientHelloV2()
	if retErr != nil {
		return
	}

	retErr = s.makeServerHelloV2()
	if retErr != nil {
		return
	}

	serverHelloPacket, err := s.serializer.Serialize(s.srvHelloMessage)
	if err != nil {
		retErr = err
		return
	}
	if _, err = s.conn.Write(serverHelloPacket); err != nil {
		retErr = err
		return
	}

	// #12: use wire bytes for transcript to prevent padding malleability
	s.transcript = append(clientHelloRaw, serverHelloPacket[2:]...)
	transcriptHash := sha3Sum256(s.transcript)

	secret, err := NewSessionSecretV2(transcriptHash, s.kemSharedSecret[:])
	if err != nil {
		retErr = err
		return
	}
	s.secret = *secret
	zeroBytes(s.kemSharedSecret)
	s.kemSharedSecret = nil
	if s.kem != nil {
		k := *s.kem
		k.Clean()
		s.kem = nil
	}

	signature, err := cryptobase.SigAlg.Sign(transcriptHash, s.serverSigningPrivateKey)
	if err != nil {
		retErr = err
		return
	}

	serverVerifyMessage := new(ServerVerifyMessage)
	serverVerifyMessage.Signature = make([]byte, cryptobase.SigAlg.SignatureWithPublicKeyLength())
	copy(serverVerifyMessage.Signature[:], signature)
	serverVerifyMessage.SignatureLen = uint(len(signature))
	s.srvVerifyMessage = serverVerifyMessage

	serverVerifyPacket, err := s.serializer.Serialize(serverVerifyMessage)
	if err != nil {
		retErr = err
		return
	}

	retErr = s.WriteEncrypted(serverVerifyPacket, 0, PacketTypeHandshake)
	if retErr != nil {
		return
	}

	serverVerifyTranscript, err := s.serializer.SerializeDeterministic(s.srvVerifyMessage, 0)
	if err != nil {
		retErr = err
		return
	}
	s.transcript = append(s.transcript, serverVerifyTranscript...)

	retErr = s.handleClientVerifyV2()
	if retErr != nil {
		return
	}

	serverFinishedData, err := s.secret.ComputeServerFinished()
	if err != nil {
		retErr = err
		return
	}
	serverFinishedMessage := &FinishedMessage{VerifyData: serverFinishedData}
	serverFinishedPacket, err := s.serializer.Serialize(serverFinishedMessage)
	if err != nil {
		retErr = err
		return
	}
	retErr = s.WriteEncrypted(serverFinishedPacket, 0, PacketTypeHandshake)
	if retErr != nil {
		return
	}

	// Extend transcript with ServerFinished so the ClientFinished HMAC
	// cryptographically binds to the ServerFinished, matching TLS 1.3.
	serverFinishedTranscript, err := s.serializer.SerializeDeterministic(serverFinishedMessage, 0)
	if err != nil {
		retErr = err
		return
	}
	s.transcript = append(s.transcript, serverFinishedTranscript...)
	s.secret.TranscriptHash = sha3Sum256(s.transcript)

	clientFinishedMessage := new(FinishedMessage)
	retErr = s.ReadAndDecryptMessageV2(clientFinishedMessage, PacketTypeHandshake)
	if retErr != nil {
		return
	}
	if len(clientFinishedMessage.Rest) > 0 {
		retErr = errors.New("unexpected data in finished message")
		return
	}
	expectedClientFinished, err := s.secret.ComputeClientFinished()
	if err != nil {
		retErr = err
		return
	}
	if subtle.ConstantTimeCompare(clientFinishedMessage.VerifyData, expectedClientFinished) != 1 {
		retErr = errors.New("client finished verification failed")
		return
	}

	s.secret.ZeroPostHandshakeKeyMaterial()
	s.handshakeDone.Store(true)
	return nil
}

func (s *ServerV2) makeServerHelloV2() error {
	serverHelloMessage := new(ServerHelloMessage)
	serverHelloMessage.Version = handshakeVersion

	randomData := make([]byte, shaLength)
	if _, err := rand.Read(randomData); err != nil {
		return err
	}
	copy(serverHelloMessage.ServerHelloRandomData[:], randomData)

	k := *s.kem
	serverHelloMessage.CipherText = make([]byte, k.Details().LengthCiphertext)
	copy(serverHelloMessage.CipherText[:], s.kemCipherText[:])
	s.srvHelloMessage = serverHelloMessage
	return nil
}

func (s *ServerV2) handleClientHelloV2() error {
	if s.cliHelloMessage.Version != handshakeVersion {
		return errors.New("unsupported handshake version")
	}
	k := *s.kem
	// #5: validate public key length before KEM operation
	if len(s.cliHelloMessage.ClientKemPublicKey) != k.Details().LengthPublicKey {
		return errors.New("invalid KEM public key length")
	}
	ciphertext, sharedSecret, err := k.EncapsulateSecret(s.cliHelloMessage.ClientKemPublicKey[:])
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
	s.kemCipherText = make([]byte, k.Details().LengthCiphertext)
	copy(s.kemCipherText[:], ciphertext[:])
	s.kemSharedSecret = make([]byte, k.Details().LengthSharedSecret)
	copy(s.kemSharedSecret[:], sharedSecret[:])
	return nil
}

// handleClientVerifyV2 verifies the client's signature over the transcript.
//
// Cryptographic verification performed here:
//   - The client signs the transcript hash (covering ClientHello, ServerHello,
//     and ServerVerify) with its signing private key.
//   - This function recovers the client's public key from that signature
//     (PublicKeyBytesFromSignature) and then explicitly verifies the signature
//     against the recovered key (DynamicSigVerifier.Verify). This proves the
//     client possesses the private key corresponding to the recovered public key.
//   - The recovered key is stored in s.clientSigningPublicKey. It is returned
//     to the caller via ClientSigningPublicKey().
//
// Identity binding (why no expected-key check here):
//
//	The server intentionally does NOT check the recovered public key against
//	any pre-known identity at this layer. Identity verification is performed
//	by the caller in p2p/server.go setupConn (line ~1044–1075):
//
//	Step 1 (line ~1053): For inbound connections, setupConn calls
//	  nodeFromConn(remotePubkey) → enode.NewV4 → V4ID.NodeAddr, which computes
//	  c.node.ID() = Keccak256(SerializePublicKey(remotePubkey))
//	  where remotePubkey is the key authenticated by this function.
//
//	Step 2 (line ~1065): setupConn runs a protocol handshake (doProtoHandshake)
//	  over the now-encrypted channel. The client sends phs.ID = its serialized
//	  public key bytes (set in server.go line ~537 the same way for all peers).
//
//	Step 3 (line ~1072): setupConn verifies
//	  Keccak256(phs.ID) == c.node.ID()
//	  If this fails, the connection is rejected with DiscUnexpectedIdentity.
//	  This binds the protocol-level identity claim to the RLPx-authenticated
//	  key: an attacker who cannot produce a valid signature in Step 1 cannot
//	  forge a node ID, and an attacker who passes Step 1 but claims a different
//	  key in Step 2 will fail the hash comparison in Step 3.
//
//	This two-layer design allows the RLPx layer to remain identity-agnostic
//	while the p2p layer enforces that only the holder of the corresponding
//	private key can assume a given node ID.
func (s *ServerV2) handleClientVerifyV2() error {
	clientVerifyMessage := new(ClientVerifyMessage)
	err := s.ReadAndDecryptMessageV2(clientVerifyMessage, PacketTypeHandshake)
	if err != nil {
		return err
	}
	s.cliVerifyMessage = clientVerifyMessage

	clientVerifyTranscript, err := s.serializer.SerializeDeterministic(s.cliVerifyMessage, 0)
	if err != nil {
		return err
	}
	transcriptHash := sha3Sum256(s.transcript)

	// #2: reject empty signatures at the protocol level
	if clientVerifyMessage.SignatureLen == 0 {
		return errors.New("empty signature")
	}
	if clientVerifyMessage.SignatureLen > uint(len(clientVerifyMessage.Signature)) {
		return errors.New("invalid signature length")
	}
	sig := clientVerifyMessage.Signature[:clientVerifyMessage.SignatureLen]
	clientPubKeyDataRemote, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(transcriptHash, sig)
	if err != nil {
		return err
	}
	// #13: use SigAlg.Verify (static algorithm) instead of DynamicSigVerifier
	if !cryptobase.SigAlg.Verify(clientPubKeyDataRemote, transcriptHash, sig) {
		return errors.New("client's signature verification failed")
	}
	// Store the recovered key; the caller (p2p/server.go) reads it via
	// ClientSigningPublicKey() and performs the identity binding check.
	s.clientSigningPublicKey, err = cryptobase.SigAlg.DeserializePublicKey(clientPubKeyDataRemote)
	if err != nil {
		return err
	}

	s.transcript = append(s.transcript, clientVerifyTranscript...)
	transcriptHash = sha3Sum256(s.transcript)
	return s.secret.CreateApplicationSecrets(transcriptHash)
}

func (s *ServerV2) ReadAndDecryptMessageV2(msg interface{}, packetType PacketType) error {
	dataPacket, err := s.ReadAndDecrypt(packetType)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(dataPacket.fragment)
	_, err = s.serializer.Deserialize(msg, reader)
	return err
}

func (s *ServerV2) WriteEncrypted(data []byte, context uint64, packetType PacketType) error {
	if packetType == PacketTypeApplicationData && !s.handshakeDone.Load() {
		return errors.New("handshake not completed")
	}
	// #3: lock unconditionally to prevent nonce reuse from concurrent calls
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	// Cleanup() acquires writeMutex before zeroing the session secrets, so once
	// we hold the lock the closed flag and cipher fields are stable. Reject
	// writes that race connection teardown instead of dereferencing nil ciphers.
	if s.closed.Load() {
		return errors.New("connection closed")
	}

	var cipher cipher2.AEAD
	var seqNum uint64
	var serverIv []byte
	if packetType == PacketTypeHandshake {
		cipher = s.secret.ServerHandshakeCipher
		seqNum = s.serverSeqNumHandshake
		serverIv = s.secret.ServerHandshakeIv
	} else {
		cipher = s.secret.ServerApplicationCipher
		seqNum = s.serverSeqNumApplication
		serverIv = s.secret.ServerApplicationIv
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

	nonce, err := CalculateNonceV2(seqNum, serverIv)
	if err != nil {
		return err
	}
	encryptedData := cipher.Seal(nil, nonce, payloadData, header.AdditionalData[:])

	headerPacket, err := s.serializer.Serialize(header)
	if err != nil {
		return err
	}
	buf := make([]byte, len(headerPacket)+len(encryptedData))
	copy(buf, headerPacket)
	copy(buf[len(headerPacket):], encryptedData)
	if _, err = s.conn.Write(buf); err != nil {
		return err
	}

	if packetType == PacketTypeHandshake {
		s.serverSeqNumHandshake++
	} else {
		s.serverSeqNumApplication++
	}
	return nil
}

func (s *ServerV2) ReadAndDecrypt(packetType PacketType) (*DataPacket, error) {
	if packetType == PacketTypeApplicationData {
		if !s.handshakeDone.Load() {
			return nil, errors.New("handshake not completed")
		}
		s.readMutex.Lock()
		defer s.readMutex.Unlock()
		if s.closed.Load() {
			return nil, errors.New("connection closed")
		}
	}

	var cipher cipher2.AEAD
	var seqNum uint64
	var clientIv []byte
	if packetType == PacketTypeHandshake {
		cipher = s.secret.ClientHandshakeCipher
		seqNum = s.clientSeqNumHandshake
		clientIv = s.secret.ClientHandshakeIv
	} else {
		cipher = s.secret.ClientApplicationCipher
		seqNum = s.clientSeqNumApplication
		clientIv = s.secret.ClientApplicationIv
	}
	if cipher == nil {
		return nil, errors.New("cipher unavailable")
	}

	header := new(Header)
	_, err := s.serializer.Deserialize(header, s.conn)
	if err != nil {
		return nil, err
	}
	if header.MinorVersion != minorVersionV2 {
		return nil, errors.New("unsupported transport version")
	}
	// #4: reject headers with unexpected trailing data
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
	bytesRead, err := io.ReadAtLeast(s.conn, encryptedData, recLen)
	if err != nil {
		return nil, err
	}
	if bytesRead != recLen {
		return nil, errors.New("prefix size less")
	}

	nonce, err := CalculateNonceV2(seqNum, clientIv)
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
		s.clientSeqNumHandshake++
	} else {
		s.clientSeqNumApplication++
	}
	return dataPacket, nil
}

func (s *ServerV2) Cleanup() {
	// Mark closed first so that any later WriteEncrypted/ReadAndDecrypt bail out
	// once they observe the flag under the relevant lock.
	s.closed.Store(true)
	// Acquire both locks so we cannot zero the cipher/IV material while a
	// concurrent WriteEncrypted or (application-phase) ReadAndDecrypt is using
	// it. Conn.Close closes the underlying connection before calling Cleanup,
	// so any blocked I/O returns promptly and releases these locks.
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()
	s.readMutex.Lock()
	defer s.readMutex.Unlock()
	if s.kem != nil {
		k := *s.kem
		k.Clean()
	}
	zeroBytes(s.kemSharedSecret)
	s.secret.ZeroSecrets()
}

func (s *ServerV2) InitWithSecrets(secret SessionSecret) {
	s.secret = secret
	s.handshakeDone.Store(true)
}
