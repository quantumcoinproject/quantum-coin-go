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
)

// Constants for the handshake.
const (
	//pubLen          = oqs.PublicKeyLen
	shaLength        = 32 // hash length (for nonce etc)
	kemPublicKeyLen  = 1138
	symmetricKeySize = 32
	ivSize           = 12

	majorVersion = 1
	minorVersion = 1

	minorVersionV2 = 2

	padLen = 0
	shaLen = 32

	maxRecordLength     = 96 * 1024 * 1024  // 96 MB
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
	seqNum     uint
	fragment   []byte
	context    uint64
}

type Header struct {
	PacketType     uint
	MinorVersion   uint
	MajorVersion   uint
	RecordLength   uint
	Context        uint64
	AdditionalData [common.HashLength]byte
	Rest           []rlp.RawValue `rlp:"tail"`
}

type AdditionalData struct {
	PacketType   uint
	MinorVersion uint
	MajorVersion uint
	DataLength   uint
	Rest         []rlp.RawValue `rlp:"tail"`
}

func CalculateNonce(recordCount uint, input []byte) ([]byte, error) {
	inputLen := len(input)
	if inputLen < 8 {
		return nil, errors.New("IV too short for counter")
	}
	if recordCount == ^uint(0) {
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

func Encrypt(cipher1 cipher.AEAD, fragment []byte, additionalData []byte, packetType PacketType, iv []byte, seqNum uint) (encrypted []byte, err error) {
	dataLen := len(fragment)

	nonce, err := CalculateNonce(seqNum, iv)
	if err != nil {
		return nil, err
	}

	//Calculate packet overhead
	beforeEncryptLen := dataLen + 1 + padLen
	encryptedLen := beforeEncryptLen + cipher1.Overhead()

	//Create array to store encrypted data with overhead
	buffer := make([]byte, encryptedLen)
	copy(buffer, fragment)
	buffer[dataLen] = byte(packetType)
	for i := 1; i <= padLen; i++ {
		buffer[dataLen+i] = 0
	}

	//Encrypt the data
	if len(buffer) < beforeEncryptLen {
		return nil, errors.New("buffer too short")
	}
	payload := buffer[:beforeEncryptLen]
	encryptedData := cipher1.Seal(payload[:0], nonce, payload, additionalData)

	return encryptedData, nil
}

func Decrypt(cipher1 cipher.AEAD, encryptedData []byte, additionalData []byte, packetType PacketType, iv []byte, seqNum uint) (*DataPacket, error) {
	if len(encryptedData) < cipher1.Overhead() {
		return nil, errors.New("invalid data")
	}

	dataLen := len(encryptedData) - cipher1.Overhead()
	dataPacket := &DataPacket{
		packetType: packetType,
		fragment:   make([]byte, dataLen),
	}

	//Compute the nonce
	nonce, err := CalculateNonce(seqNum, iv)
	if err != nil {
		return nil, err
	}

	// Decrypt
	_, err = cipher1.Open(dataPacket.fragment[:0], nonce, encryptedData, additionalData)
	if err != nil {
		return nil, err
	}

	// Find the padding boundary
	padLen1 := padLen

	if len(dataPacket.fragment) < dataLen-padLen1-1 {
		return nil, errors.New("data length malformed (a)")
	}

	for ; padLen1 < dataLen+1 && dataPacket.fragment[dataLen-padLen1-1] == 0; padLen1++ {

	}

	// Transfer the content type
	newLen := dataLen - padLen1 - 1
	if newLen > len(dataPacket.fragment) {
		return nil, errors.New("data length malformed (c)")
	}
	dataPacket.packetType = PacketType(dataPacket.fragment[newLen])

	dataPacket.fragment = dataPacket.fragment[:newLen]
	dataPacket.seqNum = seqNum

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
