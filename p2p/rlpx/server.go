// LEGACY (pre-KemSwitchTime): This file implements the v1 RLPx server handshake
// and record layer. It is only used when time.Now() < defaults.KemSwitchTime.
// After KemSwitchTime all new connections use ServerV2 (serverv2.go) instead.
// This file will be removed once all nodes have passed KemSwitchTime.
package rlpx

import (
	"bytes"
	cipher2 "crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"sync"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

type ServerHelloMessage struct {
	CipherText            []byte //kemCipherTextLength
	ServerHelloRandomData [shaLen]byte
	Version               uint
	Rest                  []rlp.RawValue `rlp:"tail"`
}

type ServerVerifyMessage struct {
	Signature    []byte //SignPublicKeyLen
	SignatureLen uint
	Rest         []rlp.RawValue `rlp:"tail"`
}

// Server is the legacy (v1) RLPx server, used only before KemSwitchTime.
// After KemSwitchTime, ServerV2 replaces this entirely. See serverv2.go.
type Server struct {
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

	kemCipherText   []byte //kemCipherTextLength
	kemSharedSecret []byte //kemSecretLength

	serverSeqNumHandshake uint
	clientSeqNumHandshake uint

	serverSeqNumApplication uint
	clientSeqNumApplication uint

	secret SessionSecret

	conn io.ReadWriter

	serializer Serializer

	transcript []byte

	client *Client

	handshakeDone bool
	mutex         sync.Mutex

	context string
}

func NewServer(conn io.ReadWriter, serverSigningPrivateKey *signaturealgorithm.PrivateKey, context string) *Server {
	server := Server{
		conn:                    conn,
		serverSigningPrivateKey: serverSigningPrivateKey,
		context:                 context,
	}

	server.serializer = NewRlpxSerializer()
	server.serializer.SetContext("server " + context)
	server.serverSeqNumHandshake = 1
	server.clientSeqNumHandshake = 1
	server.serverSeqNumApplication = 1
	server.clientSeqNumApplication = 1

	return &server
}

// ClientSigningPublicKey returns the client's signing public key (after handshake). Used by Conn.Handshake.
func (s *Server) ClientSigningPublicKey() *signaturealgorithm.PublicKey {
	return s.clientSigningPublicKey
}

func (s *Server) SetClient(client *Client) {
	s.client = client
}

func (s *Server) SetServerSigningPrivateKey(serverSigningPrivateKey *signaturealgorithm.PrivateKey) {
	s.serverSigningPrivateKey = serverSigningPrivateKey
}

func (s *Server) PerformHandshake() error {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.handshakeDone == true {
		return errors.New("Handshake already done")
	}

	var err error
	s.kem, err = NewKem("server")
	if err != nil {
		return err
	}

	//Receive client hello message
	clientHelloMessage := new(ClientHelloMessage)
	_, err = s.serializer.Deserialize(clientHelloMessage, s.conn)
	if err != nil {
		return err
	}

	//Handle client hello message
	s.cliHelloMessage = clientHelloMessage
	err = s.handleClientHello()
	if err != nil {
		return err
	}

	//Make server hello message
	err = s.makeServerHello()
	if err != nil {
		return err
	}

	serverHelloPacket, err := s.serializer.Serialize(s.srvHelloMessage)
	if err != nil {
		return err
	}

	//Write server hello message
	if _, err = s.conn.Write(serverHelloPacket); err != nil {
		return err
	}

	//Find the transcript of the session
	clientHelloTranscript, err := s.serializer.SerializeDeterministic(s.cliHelloMessage, 0)
	if err != nil {
		return err
	}

	serverHelloTranscript, err := s.serializer.SerializeDeterministic(s.srvHelloMessage, 0)
	if err != nil {
		return err
	}
	s.transcript = append(clientHelloTranscript, serverHelloTranscript...)
	transcriptHash := crypto.Keccak256(s.transcript)

	//Create the secrets
	secret, err := NewSessionSecret(transcriptHash, s.kemSharedSecret[:])
	if err != nil {
		return err
	}
	s.secret = *secret

	//Sign the transcript hash
	signature, err := cryptobase.SigAlg.Sign(transcriptHash, s.serverSigningPrivateKey)
	if err != nil {
		return err
	}

	//Serialize the server verify message
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

	//Create the transcript
	serverVerifyTranscript, err := s.serializer.SerializeDeterministic(s.srvVerifyMessage, 0)
	if err != nil {
		return err
	}

	s.transcript = append(s.transcript, serverVerifyTranscript...)

	err = s.handleClientVerify()
	if err != nil {
		return err
	}

	s.handshakeDone = true

	return nil
}

func (s *Server) Read() error {
	//Receive the server verify message header (legacy format)
	header := new(LegacyHeader)
	_, err := s.serializer.Deserialize(header, s.conn)
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) makeServerHello() error {
	serverHelloMessage := new(ServerHelloMessage)
	serverHelloMessage.Version = 1

	// Generate ServerRandomData
	randomData := make([]byte, shaLength)
	_, err := rand.Read(randomData)
	if err != nil {
		return err
	}
	copy(serverHelloMessage.ServerHelloRandomData[:], randomData)

	k := *s.kem
	serverHelloMessage.CipherText = make([]byte, k.Details().LengthCiphertext)
	copy(serverHelloMessage.CipherText[:], s.kemCipherText[:])
	s.srvHelloMessage = serverHelloMessage

	return nil
}

func (s *Server) handleClientHello() error {
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

// handleClientVerify verifies the client's signature over the transcript.
// Client identity is checked at the application layer after the initial connection is established.
func (s *Server) handleClientVerify() error {

	//Receive the client verify message
	clientVerifyMessage := new(ClientVerifyMessage)
	err := s.ReadAndDecryptMessage(clientVerifyMessage, PacketTypeHandshake)
	if err != nil {
		return err
	}

	s.cliVerifyMessage = clientVerifyMessage

	//Find the transcript of the session
	clientVerifyTranscript, err := s.serializer.SerializeDeterministic(s.cliVerifyMessage, 0)
	if err != nil {
		return err
	}

	transcriptHash := crypto.Keccak256(s.transcript)

	if clientVerifyMessage.SignatureLen > uint(len(clientVerifyMessage.Signature)) {
		return errors.New("invalid signature length")
	}

	//Recover the public key from the signature
	clientPubKeyDataRemote, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(transcriptHash, clientVerifyMessage.Signature[:clientVerifyMessage.SignatureLen])
	if err != nil {

		return err
	}

	if !cryptobase.DynamicSigVerifier.Verify(clientPubKeyDataRemote, transcriptHash, clientVerifyMessage.Signature[:clientVerifyMessage.SignatureLen]) {
		return errors.New("client's signature verification failed")
	}

	s.clientSigningPublicKey, err = cryptobase.SigAlg.DeserializePublicKey(clientPubKeyDataRemote)
	if err != nil {
		return err
	}

	s.transcript = append(s.transcript, clientVerifyTranscript...)
	transcriptHash = crypto.Keccak256(s.transcript)

	err = s.secret.CreateApplicationSecrets(transcriptHash)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) createApplicationSecrets() error {
	return nil
}

func (s *Server) WriteEncrypted(data []byte, context uint64, packetType PacketType) error {
	if packetType == PacketTypeApplicationData {
		if s.handshakeDone != true {
			return errors.New("handshake not completed")
		}
	}

	additionalData := make([]byte, shaLength)
	if _, err := rand.Read(additionalData); err != nil {
		return err
	}

	var cipher cipher2.AEAD
	var seqNum uint
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

	legacyHeader := &LegacyHeader{
		PacketType:     uint(packetType),
		MinorVersion:   minorVersion,
		MajorVersion:   majorVersion,
		Context:        context,
		AdditionalData: [common.HashLength]byte{},
	}
	copy(legacyHeader.AdditionalData[:], additionalData)

	encryptedData, err := EncryptLegacy(cipher, data, legacyHeader.AdditionalData[:], packetType, serverIv, seqNum)
	if err != nil {
		return err
	}
	legacyHeader.RecordLength = uint(len(encryptedData))

	headerPacket, err := s.serializer.Serialize(legacyHeader)
	if err != nil {
		return err
	}
	if _, err = s.conn.Write(headerPacket); err != nil {
		return err
	}
	if _, err = s.conn.Write(encryptedData); err != nil {
		return err
	}

	if packetType == PacketTypeHandshake {
		s.serverSeqNumHandshake = s.serverSeqNumHandshake + 1
	} else {
		s.serverSeqNumApplication = s.serverSeqNumApplication + 1
	}
	return nil
}

func (s *Server) ReadAndDecrypt(packetType PacketType) (*DataPacket, error) {
	if packetType == PacketTypeApplicationData {
		if s.handshakeDone != true {
			return nil, errors.New("handshake not completed")
		}
	}

	var cipher cipher2.AEAD
	var seqNum uint
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

	legacyHeader := new(LegacyHeader)
	_, err := s.serializer.Deserialize(legacyHeader, s.conn)
	if err != nil {
		return nil, err
	}

	recLen := int(legacyHeader.RecordLength)
	if recLen < 0 || recLen > maxRecordLength {
		return nil, errors.New("record length exceeds maximum allowed size")
	}
	encryptedData := make([]byte, recLen)
	bytesRead, err := io.ReadAtLeast(s.conn, encryptedData, recLen)
	if err != nil {
		return nil, err
	}
	if bytesRead != recLen {
		return nil, errors.New("prefix size less")
	}

	dataPacket, err := DecryptLegacy(cipher, encryptedData, legacyHeader.AdditionalData[:], clientIv, seqNum)
	if err != nil {
		return nil, err
	}
	if dataPacket.packetType != packetType {
		return nil, errors.New("packetType mismatch")
	}
	dataPacket.context = legacyHeader.Context

	if legacyHeader.MinorVersion >= minorVersionV2 {
		dataPacket.fragment, err = decompress(dataPacket.fragment)
		if err != nil {
			return nil, err
		}
	}

	if packetType == PacketTypeHandshake {
		s.clientSeqNumHandshake = s.clientSeqNumHandshake + 1
	} else {
		s.clientSeqNumApplication = s.clientSeqNumApplication + 1
	}
	return dataPacket, nil
}

func (s *Server) ReadAndDecryptMessage(msg interface{}, packetType PacketType) error {
	dataPacket, err := s.ReadAndDecrypt(packetType)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(dataPacket.fragment)
	_, err = s.serializer.Deserialize(msg, reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) Cleanup() {
	if s.kem != nil {
		k := *s.kem
		k.Clean()
	}
}

func (s *Server) InitWithSecrets(secret SessionSecret) {
	s.secret = secret
	s.handshakeDone = true
}
