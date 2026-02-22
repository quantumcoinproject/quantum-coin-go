package rlpx

import (
	"bytes"
	"math/rand"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/p2p/pipes"
)

// useV2 returns true when the test is running in "after KemSwitchTime" mode (v2 protocol).
func useV2() bool {
	return defaults.DefaultConfig.KemSwitchTime > 0 && time.Now().UTC().Unix() >= defaults.DefaultConfig.KemSwitchTime
}

func runWithKemSwitchTimeMatrix(t *testing.T, testFn func(t *testing.T)) {
	t.Run("KemSwitchTime_BeforeAug22", func(t *testing.T) {
		orig := defaults.DefaultConfig.KemSwitchTime
		defaults.DefaultConfig.KemSwitchTime = 0
		defer func() { defaults.DefaultConfig.KemSwitchTime = orig }()
		testFn(t)
	})
	t.Run("KemSwitchTime_AfterAug22", func(t *testing.T) {
		orig := defaults.DefaultConfig.KemSwitchTime
		defaults.DefaultConfig.KemSwitchTime = time.Now().UTC().Unix() + 86400*365*10
		defer func() { defaults.DefaultConfig.KemSwitchTime = orig }()
		testFn(t)
	})
}

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

	if useV2() {
		server := NewServerV2(serverConn, serverSigningKey, "test")
		handshakeDone := make(chan error, 1)
		go func() {
			defer serverConn.Close()
			handshakeDone <- server.PerformHandshake()
		}()
		clientKey, err := cryptobase.SigAlg.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		client := NewClientV2(clientConn, clientKey, &serverSigningKey.PublicKey, "test")
		defer client.Cleanup()
		if err := client.PerformHandshake(); err != nil {
			t.Fatal(err)
		}
		return
	}

	server := NewServer(serverConn, serverSigningKey, "test")
	handshakeDone := make(chan error, 1)
	go func() {
		defer serverConn.Close()
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
	runWithKemSwitchTimeMatrix(t, testHandshake)
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

	if useV2() {
		server := NewServerV2(serverConn, serverSigningKey, "test")
		go func() {
			defer serverConn.Close()
			if err := server.PerformHandshake(); err != nil {
				t.Error(err)
				return
			}
			if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
				t.Fatal(err)
			}
			dataPacket, err := server.ReadAndDecrypt(PacketTypeApplicationData)
			if err != nil {
				t.Error(err)
				return
			}
			if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
				t.Fatal(err)
			}
			if err := server.WriteEncrypted(dataPacket.fragment, 1, PacketTypeApplicationData); err != nil {
				t.Error(err)
			}
		}()
		clientKey, err := cryptobase.SigAlg.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		client := NewClientV2(clientConn, clientKey, &serverSigningKey.PublicKey, "test")
		defer client.Cleanup()
		if err := client.PerformHandshake(); err != nil {
			t.Fatal(err)
		}
		randomData := make([]byte, 1024)
		if _, err := rand.Read(randomData); err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}
		if err := client.WriteEncrypted(randomData, 1, PacketTypeApplicationData); err != nil {
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
			t.Fatal("fragment mismatch")
		}
		return
	}

	server := NewServer(serverConn, serverSigningKey, "test")
	handshakeDone := make(chan error, 1)
	go func() {
		defer serverConn.Close()
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
	runWithKemSwitchTimeMatrix(t, testSinglePingPong)
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

	if useV2() {
		server := NewServerV2(serverConn, serverSigningKey, "test")
		go func() {
			defer serverConn.Close()
			if err := server.PerformHandshake(); err != nil {
				return
			}
			for i := 1; ; i++ {
				if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
					return
				}
				dataPacket, err := server.ReadAndDecrypt(PacketTypeApplicationData)
				if err != nil {
					return
				}
				if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
					return
				}
				if err := server.WriteEncrypted(dataPacket.fragment, uint64(i), PacketTypeApplicationData); err != nil {
					return
				}
			}
		}()
		clientKey, err := cryptobase.SigAlg.GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		client := NewClientV2(clientConn, clientKey, &serverSigningKey.PublicKey, "test")
		client.SetServer(server)
		server.SetClient(client)
		defer client.Cleanup()
		if err := client.PerformHandshake(); err != nil {
			t.Fatal(err)
		}
		runE2ePingPong(t, client, clientConn, waitTime)
		return
	}

	server := NewServer(serverConn, serverSigningKey, "test")
	handshakeDone := make(chan error, 1)
	go func() {
		defer serverConn.Close()
		err := server.PerformHandshake()
		handshakeDone <- err
		if err != nil {
			return
		}
		for i := 1; ; i++ {
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

	runE2ePingPong(t, client, clientConn, waitTime)
}

// runE2ePingPong runs 15 ping-pong rounds. client must implement ReadAndDecrypt and WriteEncrypted.
func runE2ePingPong(t *testing.T, client handshakeClient, clientConn interface{ SetDeadline(time.Time) error }, waitTime time.Duration) {
	for i := 1; i <= 15; i++ {
		size := rand.Intn(99) + 1
		randomData := make([]byte, size)
		if _, err := rand.Read(randomData); err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}
		if err := client.WriteEncrypted(randomData, uint64(i), PacketTypeApplicationData); err != nil {
			t.Fatal(err)
		}
		if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
			t.Fatal(err)
		}
		if _, err := client.ReadAndDecrypt(PacketTypeApplicationData); err != nil {
			t.Fatal(err)
		}
	}
}

func Test_e2eHandshake(t *testing.T) {
	runWithKemSwitchTimeMatrix(t, func(t *testing.T) {
		testE2eHandShake(t)
		testE2eHandShake(t)
	})
}

func Test_SinglePingPongHybrid(t *testing.T) {
	runWithKemSwitchTimeMatrix(t, testSinglePingPong)
}

func Test_SinglePingPongCompression(t *testing.T) {
	runWithKemSwitchTimeMatrix(t, testSinglePingPong)
}
