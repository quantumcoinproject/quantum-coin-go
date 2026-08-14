package rlpx

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"

	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

type Serializer interface {
	Serialize(msg interface{}) ([]byte, error)
	SerializeDeterministic(msg interface{}, padLen int) ([]byte, error)
	Deserialize(msg interface{}, reader io.Reader) ([]byte, error)
	SetContext(context string)
}

type RlpxSerializer struct {
	rbuf    ReadBuffer
	wbuf    WriteBuffer
	context string
	mutex   sync.Mutex
}

func NewRlpxSerializer() Serializer {
	return &RlpxSerializer{
		mutex: sync.Mutex{},
	}
}

func (rs *RlpxSerializer) SetContext(context string) {
	rs.context = context
}

func (rs *RlpxSerializer) serializeDeterministicLocked(msg interface{}, padLen int) ([]byte, error) {
	rs.wbuf.Reset()

	// Write the message plaintext.
	if err := rlp.Encode(&rs.wbuf, msg); err != nil {
		return nil, err
	}

	// Pad with random amount of data. the amount needs to be at least 100 bytes to make
	// the message distinguishable from pre-EIP-8 handshakes.
	rs.wbuf.AppendZero(padLen)

	if len(rs.wbuf.Data) > 65535 {
		return nil, fmt.Errorf("message too large for 16-bit length prefix: %d bytes", len(rs.wbuf.Data))
	}

	prefix := make([]byte, 2)

	binary.BigEndian.PutUint16(prefix, uint16(len(rs.wbuf.Data)))

	return append(prefix, rs.wbuf.Data...), nil
}

func (rs *RlpxSerializer) SerializeDeterministic(msg interface{}, padLen int) ([]byte, error) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	return rs.serializeDeterministicLocked(msg, padLen)
}

func (rs *RlpxSerializer) Serialize(msg interface{}) ([]byte, error) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	padLenRand, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return nil, err
	}
	padLen := int(padLenRand.Int64()) + 100
	return rs.serializeDeterministicLocked(msg, padLen)
}

// deserializeBoundedV2 reads one length-prefixed plaintext handshake frame
// (ClientHello/ServerHello) for the v2 transport. Unlike the shared
// Deserialize (which legacy client.go/server.go also use and which must stay
// byte-for-byte compatible), this bounds the pre-authentication allocation to
// maxSize and decodes strictly: the frame must be exactly one RLP value
// followed by the serializer's zero padding (Serialize appends 100-199 zero
// bytes after the RLP value); any nonzero trailing byte is rejected.
// The returned bytes are the full frame without the 2-byte prefix, suitable
// for transcript binding.
func deserializeBoundedV2(msg interface{}, reader io.Reader, maxSize int) ([]byte, error) {
	prefix := make([]byte, 2)
	if _, err := io.ReadFull(reader, prefix); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint16(prefix))
	if size > maxSize {
		return nil, errors.New("handshake message exceeds maximum allowed size")
	}
	packet := make([]byte, size)
	if _, err := io.ReadFull(reader, packet); err != nil {
		return nil, err
	}

	_, _, rest, err := rlp.Split(packet)
	if err != nil {
		return nil, err
	}
	if err := rlp.DecodeBytes(packet[:len(packet)-len(rest)], msg); err != nil {
		return nil, err
	}
	for _, b := range rest {
		if b != 0 {
			return nil, errors.New("nonzero trailing bytes in handshake message")
		}
	}
	return packet, nil
}

func (rs *RlpxSerializer) Deserialize(msg interface{}, reader io.Reader) ([]byte, error) {
	//rs.rbuf.Reset()

	// Read the size prefix.

	prefixSize := 2
	prefix := make([]byte, prefixSize)
	bytesRead, err := io.ReadAtLeast(reader, prefix, prefixSize)
	if err != nil {

		return nil, err
	}
	if bytesRead != prefixSize {

		return nil, errors.New("prefix size less")
	}

	size := binary.BigEndian.Uint16(prefix)

	packet := make([]byte, int(size))
	bytesRead, err = io.ReadAtLeast(reader, packet, int(size))
	if err != nil {

		return nil, err
	}

	if bytesRead != int(size) {

		return nil, errors.New("prefix size less")
	}

	if len(packet) != int(size) {

	}

	s := rlp.NewStream(bytes.NewReader(packet), 0)
	err = s.Decode(msg)

	return packet, err
}
