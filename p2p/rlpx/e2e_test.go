package rlpx

import (
	"bytes"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/keyestablishmentalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/p2p/pipes"
	"math/rand"
	"testing"
	"time"
)

func testHandshake(t *testing.T) {
	waitTime := time.Second
	clientConn, serverConn, err := pipes.TCPPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := clientConn.SetDeadline(time.Now().Add(waitTime * 5)); err != nil {
		t.Fatal(err)
	}

	if err := serverConn.SetDeadline(time.Now().Add(waitTime * 5)); err != nil {
		t.Fatal(err)
	}

	serverSigningKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(serverConn, serverSigningKey, "test")
	handshakeDone := make(chan error, 1)

	go func() {
		defer serverConn.Close()
		// Perform handshake.
		err := server.PerformHandshake()
		handshakeDone <- err
		if err != nil {
			t.Fatal(err)
		}

	}()

	clientKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(clientConn, clientKey, &serverSigningKey.PublicKey, "test")

	defer client.Cleanup()

	err = client.PerformHandshake()
	if err != nil {
		t.Fatal(err)
	}
}

func Test_HandshakeOnly(t *testing.T) {
	testHandshake(t)
}

func testSinglePingPong(t *testing.T) {
	waitTime := 5 * time.Second
	clientConn, serverConn, err := pipes.TCPPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
		t.Fatal(err)
	}

	if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
		t.Fatal(err)
	}

	serverSigningKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(serverConn, serverSigningKey, "test")
	handshakeDone := make(chan error, 1)

	go func() {
		defer serverConn.Close()
		// Perform handshake.
		err := server.PerformHandshake()
		handshakeDone <- err
		if err != nil {
			t.Fatal(err)
		}

		if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}
		dataPacket, err := server.ReadAndDecrypt(PacketTypeApplicationData)
		if err != nil {
			t.Fatal(err)
		}

		if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}
		err = server.WriteEncrypted(dataPacket.fragment, 1, PacketTypeApplicationData)
		if err != nil {
			t.Fatal(err)
		}

	}()

	clientKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(clientConn, clientKey, &serverSigningKey.PublicKey, "test")

	defer client.Cleanup()

	err = client.PerformHandshake()
	if err != nil {
		t.Fatal(err)
	}

	randomData := make([]byte, 1024)
	_, err = rand.Read(randomData)
	if err != nil {
		t.Fatal(err)
	}

	if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
		t.Fatal(err)
	}

	err = client.WriteEncrypted(randomData, 1, PacketTypeApplicationData)
	if err != nil {
		t.Fatal(err)
	}

	if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
		t.Fatal(err)
	}

	dataPacket, err := client.ReadAndDecrypt(PacketTypeApplicationData)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(dataPacket.fragment, randomData) {
		t.Fatal(err)
	}
}

func Test_SinglePingPong(t *testing.T) {
	testSinglePingPong(t)
}

func testE2eHandShake(t *testing.T) {
	waitTime := 5 * time.Second
	clientConn, serverConn, err := pipes.TCPPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := clientConn.SetDeadline(time.Now().Add(waitTime * 5)); err != nil {
		t.Fatal(err)
	}

	if err := serverConn.SetDeadline(time.Now().Add(waitTime * 5)); err != nil {
		t.Fatal(err)
	}

	serverSigningKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(serverConn, serverSigningKey, "test")

	handshakeDone := make(chan error, 1)

	// Server side.
	go func() {
		defer serverConn.Close()
		// Perform handshake.
		err := server.PerformHandshake()
		handshakeDone <- err
		if err != nil {

			return
		}

		i := 0
		for {
			i = i + 1

			if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
				t.Fatal(err)
			}
			dataPacket, err := server.ReadAndDecrypt(PacketTypeApplicationData)
			if err != nil {
				t.Fatal(err)
			}

			if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
				t.Fatal(err)
			}
			err = server.WriteEncrypted(dataPacket.fragment, uint64(i), PacketTypeApplicationData)
			if err != nil {
				t.Fatal(err)
			}
		}
	}()

	clientKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(clientConn, clientKey, &serverSigningKey.PublicKey, "test")
	client.SetServer(server)
	server.SetClient(client)

	defer client.Cleanup()

	err = client.PerformHandshake()
	if err != nil {

		t.Fatal(err)
	}

	count := 15
	for i := 1; i <= count; i++ {

		minVal := 1
		maxVal := 100
		size := rand.Intn(maxVal-minVal) + minVal
		randomData := make([]byte, size)

		_, err := rand.Read(randomData)
		if err != nil {
			t.Fatal(err)
		}

		if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}

		err = client.WriteEncrypted(randomData, uint64(i), PacketTypeApplicationData)
		if err != nil {
			t.Fatal(err)
		}

		if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}

		_, err = client.ReadAndDecrypt(PacketTypeApplicationData)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func Test_e2eHandshake(t *testing.T) {
	testE2eHandShake(t)
	keyestablishmentalgorithm.SetSchemeHybrid()
	testE2eHandShake(t)
}

func Test_SinglePingPongHybrid(t *testing.T) {
	keyestablishmentalgorithm.SetSchemeHybrid()
	testSinglePingPong(t)
}

func Test_SinglePingPongCompression(t *testing.T) {
	keyestablishmentalgorithm.SetSchemeHybrid()
	defaults.DefaultConfig.KemSwitchTime = time.Now().UTC().Add(-24 * time.Hour).Unix()
	testSinglePingPong(t)
}
