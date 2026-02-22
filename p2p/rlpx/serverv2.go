package rlpx

import (
	"bytes"
	cipher2 "crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"io"
	"sync"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
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

	handshakeDone bool
	mutex         sync.Mutex
	writeMutex    sync.Mutex
	readMutex     sync.Mutex

	context string
}

func (s *ServerV2) SetClient(client *ClientV2) {
	s.client = client
}

func NewServerV2(conn io.ReadWriter, serverSigningPrivateKey *signaturealgorithm.PrivateKey, context string) *ServerV2 {
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

func (s *ServerV2) PerformHandshake() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.handshakeDone {
		return errors.New("Handshake already done")
	}

	var err error
	s.kem, err = NewKem("server")
	if err != nil {
		return err
	}

	clientHelloMessage := new(ClientHelloMessage)
	_, err = s.serializer.Deserialize(clientHelloMessage, s.conn)
	if err != nil {
		return err
	}

	s.cliHelloMessage = clientHelloMessage
	err = s.handleClientHelloV2()
	if err != nil {
		return err
	}

	err = s.makeServerHelloV2()
	if err != nil {
		return err
	}

	serverHelloPacket, err := s.serializer.Serialize(s.srvHelloMessage)
	if err != nil {
		return err
	}
	if _, err = s.conn.Write(serverHelloPacket); err != nil {
		return err
	}

	clientHelloTranscript, err := s.serializer.SerializeDeterministic(s.cliHelloMessage, 0)
	if err != nil {
		return err
	}
	serverHelloTranscript, err := s.serializer.SerializeDeterministic(s.srvHelloMessage, 0)
	if err != nil {
		return err
	}
	s.transcript = append(clientHelloTranscript, serverHelloTranscript...)
	transcriptHash := sha3Sum256(s.transcript)

	secret, err := NewSessionSecretV2(transcriptHash, s.kemSharedSecret[:])
	if err != nil {
		return err
	}
	s.secret = *secret
	zeroBytes(s.kemSharedSecret)
	if s.kem != nil {
		k := *s.kem
		k.Clean()
		s.kem = nil
	}

	signature, err := cryptobase.SigAlg.Sign(transcriptHash, s.serverSigningPrivateKey)
	if err != nil {
		return err
	}

	serverVerifyMessage := new(ServerVerifyMessage)
	serverVerifyMessage.Signature = make([]byte, cryptobase.SigAlg.SignatureWithPublicKeyLength())
	copy(serverVerifyMessage.Signature[:], signature)
	serverVerifyMessage.SignatureLen = uint(len(signature))
	s.srvVerifyMessage = serverVerifyMessage

	serverVerifyPacket, err := s.serializer.Serialize(serverVerifyMessage)
	if err != nil {
		return err
	}

	err = s.WriteEncrypted(serverVerifyPacket, 0, PacketTypeHandshake)
	if err != nil {
		return err
	}

	serverVerifyTranscript, err := s.serializer.SerializeDeterministic(s.srvVerifyMessage, 0)
	if err != nil {
		return err
	}
	s.transcript = append(s.transcript, serverVerifyTranscript...)

	err = s.handleClientVerifyV2()
	if err != nil {
		return err
	}

	serverFinishedData, err := s.secret.ComputeServerFinished()
	if err != nil {
		return err
	}
	serverFinishedMessage := &FinishedMessage{VerifyData: serverFinishedData}
	serverFinishedPacket, err := s.serializer.Serialize(serverFinishedMessage)
	if err != nil {
		return err
	}
	err = s.WriteEncrypted(serverFinishedPacket, 0, PacketTypeHandshake)
	if err != nil {
		return err
	}

	clientFinishedMessage := new(FinishedMessage)
	err = s.ReadAndDecryptMessageV2(clientFinishedMessage, PacketTypeHandshake)
	if err != nil {
		return err
	}
	expectedClientFinished, err := s.secret.ComputeClientFinished()
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(clientFinishedMessage.VerifyData, expectedClientFinished) != 1 {
		return errors.New("client finished verification failed")
	}

	s.secret.ZeroHandshakeSecrets()
	s.handshakeDone = true
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
	ciphertext, sharedSecret, err := k.EncapsulateSecret(s.cliHelloMessage.ClientKemPublicKey[:])
	if err != nil {
		return err
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
//   The server intentionally does NOT check the recovered public key against
//   any pre-known identity at this layer. Identity verification is performed
//   by the caller in p2p/server.go setupConn (line ~1044–1075):
//
//   Step 1 (line ~1053): For inbound connections, setupConn calls
//     nodeFromConn(remotePubkey) → enode.NewV4 → V4ID.NodeAddr, which computes
//     c.node.ID() = Keccak256(SerializePublicKey(remotePubkey))
//     where remotePubkey is the key authenticated by this function.
//
//   Step 2 (line ~1065): setupConn runs a protocol handshake (doProtoHandshake)
//     over the now-encrypted channel. The client sends phs.ID = its serialized
//     public key bytes (set in server.go line ~537 the same way for all peers).
//
//   Step 3 (line ~1072): setupConn verifies
//     Keccak256(phs.ID) == c.node.ID()
//     If this fails, the connection is rejected with DiscUnexpectedIdentity.
//     This binds the protocol-level identity claim to the RLPx-authenticated
//     key: an attacker who cannot produce a valid signature in Step 1 cannot
//     forge a node ID, and an attacker who passes Step 1 but claims a different
//     key in Step 2 will fail the hash comparison in Step 3.
//
//   This two-layer design allows the RLPx layer to remain identity-agnostic
//   while the p2p layer enforces that only the holder of the corresponding
//   private key can assume a given node ID.
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

	if clientVerifyMessage.SignatureLen > uint(len(clientVerifyMessage.Signature)) {
		return errors.New("invalid signature length")
	}
	clientPubKeyDataRemote, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(transcriptHash, clientVerifyMessage.Signature[:clientVerifyMessage.SignatureLen])
	if err != nil {
		return err
	}
	if !cryptobase.DynamicSigVerifier.Verify(clientPubKeyDataRemote, transcriptHash, clientVerifyMessage.Signature[:clientVerifyMessage.SignatureLen]) {
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
	if packetType == PacketTypeApplicationData {
		if !s.handshakeDone {
			return errors.New("handshake not completed")
		}
		s.writeMutex.Lock()
		defer s.writeMutex.Unlock()
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

	encryptedData, err := Encrypt(cipher, payloadData, header.AdditionalData[:], serverIv, seqNum)
	if err != nil {
		return err
	}

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
		if !s.handshakeDone {
			return nil, errors.New("handshake not completed")
		}
		s.readMutex.Lock()
		defer s.readMutex.Unlock()
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

	header := new(Header)
	_, err := s.serializer.Deserialize(header, s.conn)
	if err != nil {
		return nil, err
	}
	if header.MinorVersion != minorVersionV2 {
		return nil, errors.New("unsupported transport version")
	}

	recLen := int(header.RecordLength)
	if recLen < 0 || recLen > maxRecordLengthV2 {
		return nil, errors.New("record length exceeds maximum allowed size")
	}

	reconstructedAAD := BuildAADV2(header.MinorVersion, header.RecordLength)
	if reconstructedAAD != header.AdditionalData {
		return nil, errors.New("header AAD mismatch")
	}

	encryptedData := make([]byte, recLen)
	bytesRead, err := io.ReadAtLeast(s.conn, encryptedData, recLen)
	if err != nil {
		return nil, err
	}
	if bytesRead != recLen {
		return nil, errors.New("prefix size less")
	}

	decryptedPayloadBytes, err := Decrypt(cipher, encryptedData, reconstructedAAD[:], clientIv, seqNum)
	if err != nil {
		return nil, err
	}

	var encryptedPayload EncryptedPayload
	if err := rlp.DecodeBytes(decryptedPayloadBytes, &encryptedPayload); err != nil {
		return nil, err
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
	dataPacket.fragment, err = maybeDecompress(dataPacket.fragment)
	if err != nil {
		return nil, err
	}

	if packetType == PacketTypeHandshake {
		s.clientSeqNumHandshake++
	} else {
		s.clientSeqNumApplication++
	}
	return dataPacket, nil
}

func (s *ServerV2) Cleanup() {
	if s.kem != nil {
		k := *s.kem
		k.Clean()
	}
	zeroBytes(s.kemSharedSecret)
	s.secret.ZeroSecrets()
}

func (s *ServerV2) InitWithSecrets(secret SessionSecret) {
	s.secret = secret
	s.handshakeDone = true
}
