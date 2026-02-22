package rlpx

import (
	"bytes"
	"compress/gzip"
	"crypto/cipher"
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
	maxRecordLengthV2   = 16 * 1024 * 1024  // 16 MB
	maxDecompressedSize = 128 * 1024 * 1024 // 128 MB
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

type Header struct {
	MinorVersion   uint
	RecordLength   uint
	AdditionalData [common.HashLength]byte
	Rest           []rlp.RawValue `rlp:"tail"`
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

func decompress(compressedData []byte) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	limitedReader := io.LimitReader(gzReader, maxDecompressedSize+1)
	decompressedData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if len(decompressedData) > maxDecompressedSize {
		return nil, errors.New("decompressed data exceeds maximum allowed size")
	}

	return decompressedData, nil
}

func maybeDecompress(data []byte) ([]byte, error) {
	if len(data) >= 3 && data[0] == 0x1f && data[1] == 0x8b && data[2] == 0x08 {
		return decompress(data)
	}
	return data, nil
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

// BuildAADV2 constructs AAD for the v2 protocol. It includes the record
// length so the AEAD authenticates the header field, and uses a fixed content
// type to prevent leaking whether a record is handshake or application data.
func BuildAADV2(minorVersion uint, recordLength uint) [common.HashLength]byte {
	var aad [common.HashLength]byte
	aad[0] = byte(minorVersion >> 24)
	aad[1] = byte(minorVersion >> 16)
	aad[2] = byte(minorVersion >> 8)
	aad[3] = byte(minorVersion)
	aad[4] = byte(PacketTypeApplicationData)
	aad[5] = byte(recordLength >> 24)
	aad[6] = byte(recordLength >> 16)
	aad[7] = byte(recordLength >> 8)
	aad[8] = byte(recordLength)
	return aad
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
