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

	handshakeDone bool
	mutex         sync.Mutex
	writeMutex    sync.Mutex
	readMutex     sync.Mutex

	context string
}

func (c *ClientV2) SetServer(server *ServerV2) {
	c.server = server
}

func NewClientV2(conn io.ReadWriter, clientSigningPrivateKey *signaturealgorithm.PrivateKey, serverSigningPublicKey *signaturealgorithm.PublicKey, context string) *ClientV2 {
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

func (c *ClientV2) PerformHandshake() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.handshakeDone {
		return errors.New("Handshake already done")
	}

	var err error
	c.kem, err = NewKem("client")
	if err != nil {
		return err
	}

	err = c.makeClientHelloV2()
	if err != nil {
		return err
	}

	clientHelloPacket, err := c.serializer.Serialize(c.cliHelloMessage)
	if err != nil {
		return err
	}
	if _, err = c.conn.Write(clientHelloPacket); err != nil {
		return err
	}

	serverHelloMessage := new(ServerHelloMessage)
	_, err = c.serializer.Deserialize(serverHelloMessage, c.conn)
	if err != nil {
		return err
	}

	c.srvHelloMessage = serverHelloMessage
	err = c.handleServerHelloV2()
	if err != nil {
		return err
	}

	clientHelloTranscript, err := c.serializer.SerializeDeterministic(c.cliHelloMessage, 0)
	if err != nil {
		return err
	}
	serverHelloTranscript, err := c.serializer.SerializeDeterministic(c.srvHelloMessage, 0)
	if err != nil {
		return err
	}
	transcript := append(clientHelloTranscript, serverHelloTranscript...)
	transcriptHash := sha3Sum256(transcript)
	c.transcript = transcript

	secret, err := NewSessionSecretV2(transcriptHash, c.kemSharedSecret[:])
	if err != nil {
		return err
	}
	c.secret = *secret
	zeroBytes(c.kemSharedSecret)
	if c.kem != nil {
		k := *c.kem
		k.Clean()
		c.kem = nil
	}
	c.ephemeralKemPrivateKey = nil

	serverVerifyMessage := new(ServerVerifyMessage)
	err = c.ReadAndDecryptMessageV2(serverVerifyMessage, PacketTypeHandshake)
	if err != nil {
		return err
	}

	serverPubKeyDataLocal, err := cryptobase.SigAlg.SerializePublicKey(c.serverSigningPublicKey)
	if err != nil {
		return err
	}
	if serverVerifyMessage.SignatureLen > uint(len(serverVerifyMessage.Signature)) {
		return errors.New("invalid signature length")
	}
	serverPubKeyDataRemote, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(transcriptHash, serverVerifyMessage.Signature[:serverVerifyMessage.SignatureLen])
	if err != nil {
		return err
	}
	if !bytes.Equal(serverPubKeyDataLocal, serverPubKeyDataRemote) {
		log.Trace("Public Key mismatch",
			"serverSigningPublicKey", base64.StdEncoding.EncodeToString(c.serverSigningPublicKey.PubData),
			"signature", base64.StdEncoding.EncodeToString(serverVerifyMessage.Signature[:serverVerifyMessage.SignatureLen]),
			"serverPubKeyDataRemote", base64.StdEncoding.EncodeToString(serverPubKeyDataRemote))
		return errors.New("Public key mismatch")
	}
	if !cryptobase.DynamicSigVerifier.Verify(serverPubKeyDataLocal, transcriptHash, serverVerifyMessage.Signature[:serverVerifyMessage.SignatureLen]) {
		return errors.New("server's signature verification failed")
	}

	serverVerifyTranscript, err := c.serializer.SerializeDeterministic(serverVerifyMessage, 0)
	if err != nil {
		return err
	}
	transcript = append(transcript, serverVerifyTranscript...)
	transcriptHash = sha3Sum256(transcript)
	c.transcript = transcript
	c.srvVerifyMessage = serverVerifyMessage

	signature, err := cryptobase.SigAlg.Sign(transcriptHash, c.clientSigningPrivateKey)
	if err != nil {
		return err
	}
	clientVerifyMessage := new(ClientVerifyMessage)
	clientVerifyMessage.Signature = make([]byte, cryptobase.SigAlg.SignatureWithPublicKeyLength())
	copy(clientVerifyMessage.Signature[:], signature)
	clientVerifyMessage.SignatureLen = uint(len(signature))
	c.cliVerifyMessage = clientVerifyMessage

	clientVerifyPacket, err := c.serializer.Serialize(clientVerifyMessage)
	if err != nil {
		return err
	}

	err = c.WriteEncrypted(clientVerifyPacket, 0, PacketTypeHandshake)
	if err != nil {
		return err
	}

	clientVerifyTranscript, err := c.serializer.SerializeDeterministic(clientVerifyMessage, 0)
	if err != nil {
		return err
	}
	transcript = append(transcript, clientVerifyTranscript...)
	c.transcript = transcript

	tHash := sha3Sum256(c.transcript)
	err = c.secret.CreateApplicationSecrets(tHash)
	if err != nil {
		return err
	}

	serverFinishedMessage := new(FinishedMessage)
	err = c.ReadAndDecryptMessageV2(serverFinishedMessage, PacketTypeHandshake)
	if err != nil {
		return err
	}
	expectedServerFinished, err := c.secret.ComputeServerFinished()
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(serverFinishedMessage.VerifyData, expectedServerFinished) != 1 {
		return errors.New("server finished verification failed")
	}

	clientFinishedData, err := c.secret.ComputeClientFinished()
	if err != nil {
		return err
	}
	clientFinishedMessage := &FinishedMessage{VerifyData: clientFinishedData}
	clientFinishedPacket, err := c.serializer.Serialize(clientFinishedMessage)
	if err != nil {
		return err
	}
	err = c.WriteEncrypted(clientFinishedPacket, 0, PacketTypeHandshake)
	if err != nil {
		return err
	}

	c.secret.ZeroHandshakeSecrets()
	c.handshakeDone = true
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
	sharedSecret, err := k.DecapsulateSecret(c.srvHelloMessage.CipherText[:])
	if err != nil {
		return err
	}
	c.kemSharedSecret = make([]byte, k.Details().LengthSharedSecret)
	copy(c.kemSharedSecret[:], sharedSecret[:])
	return nil
}

func (c *ClientV2) Cleanup() {
	if c.kem != nil {
		k := *c.kem
		k.Clean()
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
	if packetType == PacketTypeApplicationData {
		if !c.handshakeDone {
			return errors.New("handshake not completed")
		}
		c.writeMutex.Lock()
		defer c.writeMutex.Unlock()
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

	encryptedData, err := Encrypt(cipher, payloadData, header.AdditionalData[:], clientIv, seqNum)
	if err != nil {
		return err
	}

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
		if !c.handshakeDone {
			return nil, errors.New("handshake not completed")
		}
		c.readMutex.Lock()
		defer c.readMutex.Unlock()
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

	header := new(Header)
	_, err := c.serializer.Deserialize(header, c.conn)
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
	bytesRead, err := io.ReadAtLeast(c.conn, encryptedData, recLen)
	if err != nil {
		return nil, err
	}
	if bytesRead != recLen {
		return nil, errors.New("prefix size less")
	}

	decryptedPayloadBytes, err := Decrypt(cipher, encryptedData, reconstructedAAD[:], serverIv, seqNum)
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
		c.serverSeqNumHandshake++
	} else {
		c.serverSeqNumApplication++
	}
	return dataPacket, nil
}

func (c *ClientV2) InitWithSecrets(secret SessionSecret) {
	c.secret = secret
	c.handshakeDone = true
}
