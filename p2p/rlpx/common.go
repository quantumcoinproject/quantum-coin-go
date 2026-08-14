package rlpx

import (
	"bytes"
	"compress/gzip"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"golang.org/x/crypto/sha3"
)

// Constants for the handshake.
const (
	//pubLen          = oqs.PublicKeyLen
	shaLength        = 32 // hash length (for nonce etc)
	kemPublicKeyLen  = 1138
	symmetricKeySize = 32
	ivSize           = 12

	majorVersion     = 1
	minorVersion     = 1
	minorVersionV2   = 2
	shaLen           = 32
	handshakeVersion = 1

	padLen = 0 // legacy format padding length

	maxRecordLength     = 96 * 1024 * 1024  // 96 MB (legacy)
	maxRecordLengthV2   = 64 * 1024 * 1024  // 64 MB (max application record on wire)
	maxDecompressedSize = 128 * 1024 * 1024 // 128 MB (legacy decompression limit)

	// V2 record header protection. Every v2 record starts with a fixed-size
	// encrypted header: headerPlainLenV2 bytes of plaintext sealed with a
	// dedicated header AEAD, producing exactly headerCiphertextLenV2 bytes on
	// the wire (plaintext + 16-byte GCM tag). The body ciphertext length is
	// carried inside the header plaintext and is therefore authenticated
	// before the receiver allocates or reads any body bytes.
	headerPlainLenV2      = 16
	headerCiphertextLenV2 = 32

	// maxHelloSizeV2 bounds the plaintext ClientHello/ServerHello frames (the
	// only unprotected records; no keys exist yet). A ClientHello is ~1.4 KB:
	// 1138-byte KEM public key + 32-byte random + RLP overhead + up to 199
	// bytes of serializer zero padding. ServerHello is similar with the KEM
	// ciphertext. 4 KB gives ~3x margin.
	maxHelloSizeV2 = 4096

	// maxHandshakeRecordLengthV2 caps the record size for V2 handshake messages.
	// The largest handshake record is a Verify message (~4.2 KB with current PQC
	// signature sizes). 16 KB provides ~4x safety margin while preventing a
	// remote peer from forcing large allocations before AEAD authentication.
	maxHandshakeRecordLengthV2 = 16 * 1024 // 16 KB
)

type PacketType byte

const (
	PacketTypeHandshake       PacketType = 21
	PacketTypeApplicationData PacketType = 23
	ReadTimeout                          = time.Second * 10
	WriteTimeout                         = time.Second * 20
)

type DataPacket struct {
	packetType PacketType
	seqNum     uint64
	fragment   []byte
	context    uint64
}

// LegacyHeader is the pre-KemSwitchTime frame format for backward compatibility.
// Used when useEncryptedHeader is false so old and new nodes can interoperate.
type LegacyHeader struct {
	PacketType     uint
	MinorVersion   uint
	MajorVersion   uint
	RecordLength   uint
	Context        uint64
	AdditionalData [common.HashLength]byte
	Rest           []rlp.RawValue `rlp:"tail"`
}

type EncryptedPayload struct {
	PacketType uint
	Context    uint64
	Fragment   []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

type FinishedMessage struct {
	VerifyData []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

func CalculateNonce(recordCount uint64, input []byte) ([]byte, error) {
	inputLen := len(input)
	if inputLen < 8 {
		return nil, errors.New("IV too short for counter")
	}
	if recordCount == ^uint64(0) {
		return nil, errors.New("recordCount reached maximum value, nonce reuse imminent")
	}
	output := make([]byte, inputLen)
	copy(output, input)

	rec := recordCount

	for i := 0; i < 8; i++ {
		output[(inputLen-i)-1] ^= byte(rec & 0xff)
		rec >>= 8
	}

	return output, nil
}

// CalculateNonceV2 implements the V2-specific nonce calculation (XORing 64-bit counter).
// It ensures the IV is exactly 12 bytes and handles the XOR operation explicitly.
func CalculateNonceV2(recordCount uint64, iv []byte) ([]byte, error) {
	if len(iv) != ivSize {
		return nil, errors.New("invalid IV length for V2 nonce")
	}
	if recordCount == ^uint64(0) {
		return nil, errors.New("recordCount reached maximum value, nonce reuse imminent")
	}
	nonce := make([]byte, ivSize)
	copy(nonce, iv)

	// XOR the 8-byte recordCount into the last 8 bytes of the 12-byte IV.
	// This follows the TLS 1.3 nonce construction (RFC 8446 Section 5.3).
	for i := 0; i < 8; i++ {
		nonce[ivSize-1-i] ^= byte(recordCount >> (i * 8))
	}
	return nonce, nil
}

func Encrypt(cipher1 cipher.AEAD, fragment []byte, additionalData []byte, iv []byte, seqNum uint64) (encrypted []byte, err error) {
	nonce, err := CalculateNonce(seqNum, iv)
	if err != nil {
		return nil, err
	}

	encryptedData := cipher1.Seal(nil, nonce, fragment, additionalData)

	return encryptedData, nil
}

func Decrypt(cipher1 cipher.AEAD, encryptedData []byte, additionalData []byte, iv []byte, seqNum uint64) ([]byte, error) {
	if len(encryptedData) < cipher1.Overhead() {
		return nil, errors.New("invalid encrypted data")
	}

	//Compute the nonce
	nonce, err := CalculateNonce(seqNum, iv)
	if err != nil {
		return nil, err
	}

	// Decrypt
	fragment, err := cipher1.Open(nil, nonce, encryptedData, additionalData)
	if err != nil {
		return nil, err
	}

	return fragment, nil
}

// EncryptLegacy is the old (pre-v2) encrypt: plaintext = fragment || packetType || zeros(padLen).
// Used by Client and Server (old implementation only).
func EncryptLegacy(cipher1 cipher.AEAD, fragment []byte, additionalData []byte, packetType PacketType, iv []byte, seqNum uint) (encrypted []byte, err error) {
	dataLen := len(fragment)
	beforeEncryptLen := dataLen + 1 + padLen
	encryptedLen := beforeEncryptLen + cipher1.Overhead()
	buffer := make([]byte, encryptedLen)
	copy(buffer, fragment)
	buffer[dataLen] = byte(packetType)
	for i := 1; i <= padLen; i++ {
		buffer[dataLen+i] = 0
	}
	nonce, err := CalculateNonce(uint64(seqNum), iv)
	if err != nil {
		return nil, err
	}
	return cipher1.Seal(buffer[:0], nonce, buffer[:beforeEncryptLen], additionalData), nil
}

// DecryptLegacy is the old (pre-v2) decrypt: plaintext = fragment || packetType || zeros(padLen).
// Used by Client and Server (old implementation only).
func DecryptLegacy(cipher1 cipher.AEAD, encryptedData []byte, additionalData []byte, iv []byte, seqNum uint) (*DataPacket, error) {
	if len(encryptedData) < cipher1.Overhead() {
		return nil, errors.New("invalid encrypted data")
	}
	dataLen := len(encryptedData) - cipher1.Overhead()
	dataPacket := &DataPacket{
		fragment: make([]byte, dataLen),
	}
	nonce, err := CalculateNonce(uint64(seqNum), iv)
	if err != nil {
		return nil, err
	}
	_, err = cipher1.Open(dataPacket.fragment[:0], nonce, encryptedData, additionalData)
	if err != nil {
		return nil, err
	}
	padLen1 := padLen
	if len(dataPacket.fragment) < dataLen-padLen1-1 {
		return nil, errors.New("data length malformed (a)")
	}
	for ; padLen1 < dataLen+1 && dataPacket.fragment[dataLen-padLen1-1] == 0; padLen1++ {
	}
	newLen := dataLen - padLen1 - 1
	if newLen > len(dataPacket.fragment) {
		return nil, errors.New("data length malformed (c)")
	}
	dataPacket.packetType = PacketType(dataPacket.fragment[newLen])
	dataPacket.fragment = dataPacket.fragment[:newLen]
	dataPacket.seqNum = uint64(seqNum)
	return dataPacket, nil
}

func NewKem(context string) (*keyestablishmentalgorithm.KeyEncapsulation, error) {
	var kem keyestablishmentalgorithm.KeyEncapsulation
	var err error

	k, err := keyestablishmentalgorithm.NewKeyEncap()
	if err != nil {
		return nil, err
	}
	kem = k
	return &kem, err
}

func compress(data []byte) ([]byte, error) {
	var buff bytes.Buffer

	// Create a new gzip writer that writes to the buffer
	gzWriter := gzip.NewWriter(&buff)

	// Write the uncompressed data to the gzip writer
	_, err := gzWriter.Write(data)
	if err != nil {
		return nil, err
	}

	// Close the gzip writer to flush any buffered data and write the gzip footer
	err = gzWriter.Close()
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func decompressWithLimit(compressedData []byte, maxSize int) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	limitedReader := io.LimitReader(gzReader, int64(maxSize)+1)
	decompressedData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if len(decompressedData) > maxSize {
		return nil, errors.New("decompressed data exceeds maximum allowed size")
	}

	return decompressedData, nil
}

// decompress is used by legacy (V1) read paths only.
func decompress(compressedData []byte) ([]byte, error) {
	return decompressWithLimit(compressedData, maxDecompressedSize)
}

func BuildAAD(minorVersion uint, packetType PacketType) [common.HashLength]byte {
	var aad [common.HashLength]byte
	aad[0] = byte(minorVersion >> 24)
	aad[1] = byte(minorVersion >> 16)
	aad[2] = byte(minorVersion >> 8)
	aad[3] = byte(minorVersion)
	aad[4] = byte(packetType)
	return aad
}

// packHeaderV2 builds the fixed 16-byte header plaintext for a v2 record:
//
//	[0]    minor version (2)
//	[1]    flags (0; reserved for future negotiation)
//	[2:6]  body ciphertext length, big-endian uint32 (includes the AEAD tag)
//	[6:16] reserved, must be zero
//
// The plaintext is sealed with the direction's header AEAD (own nonce, no
// AAD) into exactly headerCiphertextLenV2 wire bytes, and doubles as the AAD
// of the body AEAD so a body cannot be spliced onto a different header.
func packHeaderV2(bodyLen uint32) [headerPlainLenV2]byte {
	var plain [headerPlainLenV2]byte
	plain[0] = minorVersionV2
	binary.BigEndian.PutUint32(plain[2:6], bodyLen)
	return plain
}

// unpackHeaderV2 validates an authenticated header plaintext and returns the
// body ciphertext length. Every non-length byte is checked against its exact
// expected value (fail closed; a future format change negotiates via the
// ClientHello version, not via silently-ignored bits).
func unpackHeaderV2(plain []byte) (uint32, error) {
	if len(plain) != headerPlainLenV2 {
		return 0, errors.New("invalid header plaintext length")
	}
	if plain[0] != minorVersionV2 {
		return 0, errors.New("unsupported transport version")
	}
	if plain[1] != 0 {
		return 0, errors.New("unsupported header flags")
	}
	for _, b := range plain[6:] {
		if b != 0 {
			return 0, errors.New("nonzero reserved bytes in header")
		}
	}
	return binary.BigEndian.Uint32(plain[2:6]), nil
}

func sha3Sum256(data []byte) []byte {
	h := sha3.New256()
	h.Write(data)
	return h.Sum(nil)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func isAllZeros(b []byte) bool {
	var acc byte
	for _, v := range b {
		acc |= v
	}
	return acc == 0
}
