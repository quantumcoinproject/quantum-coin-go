// LEGACY (pre-KemSwitchTime): This file implements the v1 RLPx client handshake
// and record layer. It is only used when time.Now() < defaults.KemSwitchTime.
// After KemSwitchTime all new connections use ClientV2 (clientv2.go) instead.
// This file will be removed once all nodes have passed KemSwitchTime.
package rlpx

import (
	"bytes"
	cipher2 "crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"sync"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

type ClientHelloMessage struct {
	ClientKemPublicKey    []byte //kemPublicKeyLen
	ClientHelloRandomData [shaLen]byte
	Version               uint
	Rest                  []rlp.RawValue `rlp:"tail"`
}

type ClientVerifyMessage struct {
	Signature    []byte //SignPublicKeyLen
	SignatureLen uint
	Rest         []rlp.RawValue `rlp:"tail"`
}

// Client is the legacy (v1) RLPx client, used only before KemSwitchTime.
// After KemSwitchTime, ClientV2 replaces this entirely. See clientv2.go.
type Client struct {
	ephemeralKemPrivateKey  *keyestablishmentalgorithm.PrivateKey
	kem                     *keyestablishmentalgorithm.KeyEncapsulation
	kemCipherText           []byte //kemCipherTextLength
	kemSharedSecret         []byte //kemSecretLength
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

	serverSeqNumHandshake uint
	clientSeqNumHandshake uint

	serverSeqNumApplication uint
	clientSeqNumApplication uint

	transcript []byte

	server *Server

	handshakeDone bool
	mutex         sync.Mutex

	context string
}

func (c *Client) SetServer(server *Server) {
	c.server = server
}

func NewClient(conn io.ReadWriter, clientSigningPrivateKey *signaturealgorithm.PrivateKey, serverSigningPublicKey *signaturealgorithm.PublicKey, context string) *Client {
	client := Client{
		conn:                    conn,
		clientSigningPrivateKey: clientSigningPrivateKey,
		serverSigningPublicKey:  serverSigningPublicKey,
	}

	client.serializer = NewRlpxSerializer()
	client.serializer.SetContext("client " + context)
	client.context = context
	client.serverSeqNumHandshake = 1
	client.clientSeqNumHandshake = 1
	client.serverSeqNumApplication = 1
	client.clientSeqNumApplication = 1

	return &client
}

func (c *Client) SetClientSigningPrivateKey(clientSigningPrivateKey *signaturealgorithm.PrivateKey) {
	c.clientSigningPrivateKey = clientSigningPrivateKey
}

func (c *Client) SetServerSigningPublicKey(serverSigningPublicKey *signaturealgorithm.PublicKey) {
	c.serverSigningPublicKey = serverSigningPublicKey
}

func (c *Client) PerformHandshake() error {

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.handshakeDone == true {
		return errors.New("Handshake already done")
	}

	var err error
	c.kem, err = NewKem("client")
	if err != nil {
		return err
	}

	//Make client hello message
	err = c.makeClientHello()
	if err != nil {
		return err
	}

	clientHelloPacket, err := c.serializer.Serialize(c.cliHelloMessage)
	if err != nil {
		return err
	}

	//Write client hello message
	if _, err = c.conn.Write(clientHelloPacket); err != nil {
		return err
	}

	//Receive server hello message
	serverHelloMessage := new(ServerHelloMessage)
	_, err = c.serializer.Deserialize(serverHelloMessage, c.conn)
	if err != nil {
		return err
	}

	//Handle server hello message
	c.srvHelloMessage = serverHelloMessage
	err = c.handleServerHello()
	if err != nil {
		return err
	}

	//Find the transcript of the session
	clientHelloTranscript, err := c.serializer.SerializeDeterministic(c.cliHelloMessage, 0)
	if err != nil {
		return err
	}

	serverHelloTranscript, err := c.serializer.SerializeDeterministic(c.srvHelloMessage, 0)
	if err != nil {
		return err
	}
	transcript := append(clientHelloTranscript, serverHelloTranscript...)
	transcriptHash := crypto.Keccak256(transcript)
	c.transcript = transcript

	//Create the secrets
	secret, err := NewSessionSecret(transcriptHash, c.kemSharedSecret[:])
	if err != nil {
		return err
	}
	c.secret = *secret

	//Receive the server verify message
	serverVerifyMessage := new(ServerVerifyMessage)
	err = c.ReadAndDecryptMessage(serverVerifyMessage, PacketTypeHandshake)
	if err != nil {
		return err
	}

	//Verify the signature to make sure the server is what it is claiming to be
	serverPubKeyDataLocal, err := cryptobase.SigAlg.SerializePublicKey(c.serverSigningPublicKey)
	if err != nil {
		return err
	}

	if serverVerifyMessage.SignatureLen > uint(len(serverVerifyMessage.Signature)) {
		return errors.New("invalid signature length")
	}

	//Recover the public key from the signature
	serverPubKeyDataRemote, err := cryptobase.SigAlg.PublicKeyBytesFromSignature(transcriptHash, serverVerifyMessage.Signature[:serverVerifyMessage.SignatureLen])
	if err != nil {
		return err
	}

	//Validate that expected public key and remote public key are the same (additional sanity check)
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

	//Create the transcript
	serverVerifyTranscript, err := c.serializer.SerializeDeterministic(serverVerifyMessage, 0)
	if err != nil {
		return err
	}

	transcript = append(transcript, serverVerifyTranscript...)
	transcriptHash = crypto.Keccak256(transcript)
	c.transcript = transcript
	c.srvVerifyMessage = serverVerifyMessage

	//Sign the transcript hash
	signature, err := cryptobase.SigAlg.Sign(transcriptHash, c.clientSigningPrivateKey)
	if err != nil {
		return err
	}

	//Serialize the server verify message
	clientVerifyMessage := new(ClientVerifyMessage)
	clientVerifyMessage.Signature = make([]byte, cryptobase.SigAlg.SignatureWithPublicKeyLength())
	copy(clientVerifyMessage.Signature[:], signature)
	clientVerifyMessage.SignatureLen = uint(len(signature))
	c.cliVerifyMessage = clientVerifyMessage

	clientVerifyPacket, err := c.serializer.Serialize(clientVerifyMessage)
	if err != nil {
		return err
	}

	clientVerifyTranscript, err := c.serializer.SerializeDeterministic(clientVerifyMessage, 0)
	if err != nil {
		return err
	}

	err = c.WriteEncrypted(clientVerifyPacket, 0, PacketTypeHandshake)
	if err != nil {
		return err
	}

	transcript = append(transcript, clientVerifyTranscript...)
	c.transcript = transcript

	tHash := crypto.Keccak256(c.transcript)
	err = c.secret.CreateApplicationSecrets(tHash)
	if err != nil {
		return err
	}

	c.handshakeDone = true

	return nil
}

func (c *Client) makeClientHello() error {
	clientHelloMessage := new(ClientHelloMessage)
	clientHelloMessage.Version = 1

	//Generate an ephemeral kem keypair
	k := *c.kem

	kemPrivateKey, err := k.GenerateKemKeyPair()
	if err != nil {
		return err
	}
	c.ephemeralKemPrivateKey = kemPrivateKey
	clientHelloMessage.ClientKemPublicKey = make([]byte, k.Details().LengthPublicKey)
	copy(clientHelloMessage.ClientKemPublicKey[:], c.ephemeralKemPrivateKey.N)

	// Generate ClientRandomData
	randomData := make([]byte, shaLength)
	_, err = rand.Read(randomData)
	if err != nil {
		return err
	}
	copy(clientHelloMessage.ClientHelloRandomData[:], randomData)
	c.Nonce = 1
	c.cliHelloMessage = clientHelloMessage

	return nil
}

func (c *Client) Cleanup() {
	if c.kem != nil {
		k := *c.kem
		k.Clean()
	}
}

// ServerSigningPublicKey returns the server's signing public key (after handshake). Used by Conn.Handshake.
func (c *Client) ServerSigningPublicKey() *signaturealgorithm.PublicKey {
	return c.serverSigningPublicKey
}

func (c *Client) handleServerHello() error {
	k := *c.kem
	sharedSecret, err := k.DecapsulateSecret(c.srvHelloMessage.CipherText[:])
	if err != nil {
		return err
	}

	c.kemSharedSecret = make([]byte, k.Details().LengthSharedSecret)
	copy(c.kemSharedSecret[:], sharedSecret[:])

	return nil
}

func (c *Client) ReadAndDecryptMessage(msg interface{}, packetType PacketType) error {
	dataPacket, err := c.ReadAndDecrypt(packetType)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(dataPacket.fragment)
	_, err = c.serializer.Deserialize(msg, reader)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) WriteEncrypted(data []byte, context uint64, packetType PacketType) error {
	if packetType == PacketTypeApplicationData {
		if c.handshakeDone != true {
			return errors.New("handshake not completed")
		}
	}

	additionalData := make([]byte, shaLength)
	if _, err := rand.Read(additionalData); err != nil {
		return err
	}

	var cipher cipher2.AEAD
	var seqNum uint
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

	legacyHeader := &LegacyHeader{
		PacketType:     uint(packetType),
		MinorVersion:   minorVersion,
		MajorVersion:   majorVersion,
		Context:        context,
		AdditionalData: [common.HashLength]byte{},
	}
	copy(legacyHeader.AdditionalData[:], additionalData)

	encryptedData, err := EncryptLegacy(cipher, data, legacyHeader.AdditionalData[:], packetType, clientIv, seqNum)
	if err != nil {
		return err
	}
	legacyHeader.RecordLength = uint(len(encryptedData))

	headerPacket, err := c.serializer.Serialize(legacyHeader)
	if err != nil {
		return err
	}
	if _, err = c.conn.Write(headerPacket); err != nil {
		return err
	}
	if _, err = c.conn.Write(encryptedData); err != nil {
		return err
	}

	if packetType == PacketTypeHandshake {
		c.clientSeqNumHandshake = c.clientSeqNumHandshake + 1
	} else {
		c.clientSeqNumApplication = c.clientSeqNumApplication + 1
	}
	return nil
}

func (c *Client) ReadAndDecrypt(packetType PacketType) (*DataPacket, error) {
	if packetType == PacketTypeApplicationData {
		if c.handshakeDone != true {
			return nil, errors.New("handshake not completed")
		}
	}

	var cipher cipher2.AEAD
	var seqNum uint
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

	legacyHeader := new(LegacyHeader)
	_, err := c.serializer.Deserialize(legacyHeader, c.conn)
	if err != nil {
		return nil, err
	}

	recLen := int(legacyHeader.RecordLength)
	if recLen < 0 || recLen > maxRecordLength {
		return nil, errors.New("record length exceeds maximum allowed size")
	}
	encryptedData := make([]byte, recLen)
	bytesRead, err := io.ReadAtLeast(c.conn, encryptedData, recLen)
	if err != nil {
		return nil, err
	}
	if bytesRead != recLen {
		return nil, errors.New("prefix size less")
	}

	dataPacket, err := DecryptLegacy(cipher, encryptedData, legacyHeader.AdditionalData[:], serverIv, seqNum)
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
		c.serverSeqNumHandshake = c.serverSeqNumHandshake + 1
	} else {
		c.serverSeqNumApplication = c.serverSeqNumApplication + 1
	}
	return dataPacket, nil
}

func (c *Client) InitWithSecrets(secret SessionSecret) {
	c.secret = secret
	c.handshakeDone = true
}
