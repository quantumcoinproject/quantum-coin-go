// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package rlpx implements the RLPx transport protocol.
package rlpx

import (
	"net"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

// handshakeClient is implemented by both Client (old) and ClientV2 (new).
// Conn uses this so it can work with either based on KemSwitchTime.
type handshakeClient interface {
	SetClientSigningPrivateKey(*signaturealgorithm.PrivateKey)
	SetServerSigningPublicKey(*signaturealgorithm.PublicKey)
	PerformHandshake() error
	ReadAndDecrypt(PacketType) (*DataPacket, error)
	WriteEncrypted([]byte, uint64, PacketType) error
	InitWithSecrets(SessionSecret)
	ServerSigningPublicKey() *signaturealgorithm.PublicKey
}

// handshakeServer is implemented by both Server (old) and ServerV2 (new).
type handshakeServer interface {
	SetServerSigningPrivateKey(*signaturealgorithm.PrivateKey)
	PerformHandshake() error
	ReadAndDecrypt(PacketType) (*DataPacket, error)
	WriteEncrypted([]byte, uint64, PacketType) error
	InitWithSecrets(SessionSecret)
	ClientSigningPublicKey() *signaturealgorithm.PublicKey
}

// Conn is an RLPx network connection. It wraps a low-level network connection. The
// underlying connection should not be used for other activity when it is wrapped by Conn.
//
// Before sending messages, a handshake must be performed by calling the Handshake method.
// This type is not generally safe for concurrent use, but reading and writing of messages
// may happen concurrently after the handshake.
//
// Before KemSwitchTime the connection uses Client/Server (legacy protocol). After
// KemSwitchTime it uses ClientV2/ServerV2 (v2 protocol).
type Conn struct {
	dialDest *signaturealgorithm.PublicKey
	conn     net.Conn

	snappyReadBuffer  []byte
	snappyWriteBuffer []byte

	client handshakeClient
	server handshakeServer

	context string
}

// NewConn wraps the given network connection. If dialDest is non-nil, the connection
// behaves as the initiator during the handshake. The implementation (old vs v2) is
// chosen based on defaults.DefaultConfig.KemSwitchTime.
func NewConn(conn net.Conn, dialDest *signaturealgorithm.PublicKey, context string) *Conn {
	connection := &Conn{
		dialDest: dialDest,
		conn:     conn,
		context:  context,
	}

	useV2 := time.Now().UTC().Unix() >= defaults.DefaultConfig.KemSwitchTime

	if dialDest == nil {
		if useV2 {
			connection.server = NewServerV2(conn, nil, context)
		} else {
			connection.server = NewServer(conn, nil, context)
		}
	} else {
		if useV2 {
			connection.client = NewClientV2(conn, nil, dialDest, context)
		} else {
			connection.client = NewClient(conn, nil, dialDest, context)
		}
	}

	return connection
}

// SetSnappy enables or disables snappy compression of messages. This is usually called
// after the devp2p Hello message exchange when the negotiated version indicates that
// compression is available on both ends of the connection.
func (c *Conn) SetSnappy(snappy bool) {
	if snappy {

		c.snappyReadBuffer = []byte{}
		c.snappyWriteBuffer = []byte{}
	} else {

		c.snappyReadBuffer = nil
		c.snappyWriteBuffer = nil
	}
}

// SetReadDeadline sets the deadline for all future read operations.
func (c *Conn) SetReadDeadline(deadlineTime time.Time) error {

	return c.conn.SetReadDeadline(deadlineTime)
}

// SetWriteDeadline sets the deadline for all future write operations.
func (c *Conn) SetWriteDeadline(deadlineTime time.Time) error {
	return c.conn.SetWriteDeadline(deadlineTime)
}

// SetDeadline sets the deadline for all future read and write operations.
func (c *Conn) SetDeadline(deadlineTime time.Time) error {
	return c.conn.SetDeadline(deadlineTime)
}

// Read reads a message from the connection.
// The returned data buffer is valid until the next call to Read.
func (c *Conn) Read() (code uint64, data []byte, wireSize int, err error) {

	if c.client != nil {
		dataPacket, err := c.client.ReadAndDecrypt(PacketTypeApplicationData)
		if err != nil {

			return 0, nil, 0, err
		}

		return dataPacket.context, dataPacket.fragment, len(dataPacket.fragment), nil
	}
	dataPacket, err := c.server.ReadAndDecrypt(PacketTypeApplicationData)
	if err != nil {
		return 0, nil, 0, err
	}
	return dataPacket.context, dataPacket.fragment, len(dataPacket.fragment), nil
}

// Write writes a message to the connection.
//
// Write returns the written size of the message data. This may be less than or equal to
// len(data) depending on whether snappy compression is enabled.
func (c *Conn) Write(code uint64, data []byte) (uint32, error) {

	size := uint32(len(data))

	if c.client != nil {
		err := c.client.WriteEncrypted(data, code, PacketTypeApplicationData)
		if err != nil {
			return size, err
		}
		return size, nil
	}
	err := c.server.WriteEncrypted(data, code, PacketTypeApplicationData)
	if err != nil {
		return size, err
	}
	return size, nil
}

// Handshake performs the handshake. This must be called before any data is written
// or read from the connection.
func (c *Conn) Handshake(prv *signaturealgorithm.PrivateKey) (*signaturealgorithm.PublicKey, error) {
	if c.client != nil {
		c.client.SetClientSigningPrivateKey(prv)
		if err := c.client.PerformHandshake(); err != nil {
			return nil, err
		}
		return c.client.ServerSigningPublicKey(), nil
	}
	c.server.SetServerSigningPrivateKey(prv)
	if err := c.server.PerformHandshake(); err != nil {
		return nil, err
	}
	return c.server.ClientSigningPublicKey(), nil
}

// Close closes the underlying network connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) InitWithSecrets(secret SessionSecret) {
	if c.client != nil {
		c.client.InitWithSecrets(secret)
		return
	}
	c.server.InitWithSecrets(secret)
}
