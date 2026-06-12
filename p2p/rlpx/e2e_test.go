package rlpx

import (
	"bytes"
	"math/rand"
	"net"
	"sync"
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
	t.Run("V2_KemSwitchTimeInPast", func(t *testing.T) {
		orig := defaults.DefaultConfig.KemSwitchTime
		defaults.DefaultConfig.KemSwitchTime = 0
		defer func() {
			defaults.DefaultConfig.KemSwitchTime = orig
		}()
		testFn(t)
	})
	t.Run("V1_KemSwitchTimeInFuture", func(t *testing.T) {
		orig := defaults.DefaultConfig.KemSwitchTime
		defaults.DefaultConfig.KemSwitchTime = time.Now().UTC().Unix() + 86400*365*10
		defer func() {
			defaults.DefaultConfig.KemSwitchTime = orig
		}()
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

// newHandshakedPair sets up a TCP pipe, runs the encryption handshake for both
// ends (v1 or v2 depending on the current KemSwitchTime), and returns the two
// fully established transports together with their underlying connections.
func newHandshakedPair(t *testing.T, waitTime time.Duration) (handshakeClient, handshakeServer, net.Conn, net.Conn) {
	t.Helper()

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
	clientKey, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	var client handshakeClient
	var server handshakeServer
	if useV2() {
		server = NewServerV2(serverConn, serverSigningKey, "test")
		client = NewClientV2(clientConn, clientKey, &serverSigningKey.PublicKey, "test")
	} else {
		server = NewServer(serverConn, serverSigningKey, "test")
		client = NewClient(clientConn, clientKey, &serverSigningKey.PublicKey, "test")
	}

	hsErr := make(chan error, 1)
	go func() {
		hsErr <- server.PerformHandshake()
	}()
	if err := client.PerformHandshake(); err != nil {
		clientConn.Close()
		serverConn.Close()
		t.Fatalf("client handshake failed: %v", err)
	}
	if err := <-hsErr; err != nil {
		clientConn.Close()
		serverConn.Close()
		t.Fatalf("server handshake failed: %v", err)
	}
	return client, server, clientConn, serverConn
}

// withV2KemSwitchTime forces the v2 transport (KemSwitchTime is a positive,
// already-elapsed timestamp so useV2() reports true) for the duration of fn.
// The use-after-cleanup crash these tests guard against is specific to v2:
// the legacy transport's Cleanup() does not zero the cipher material.
func withV2KemSwitchTime(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	orig := defaults.DefaultConfig.KemSwitchTime
	defaults.DefaultConfig.KemSwitchTime = 1
	defer func() {
		defaults.DefaultConfig.KemSwitchTime = orig
	}()
	if !useV2() {
		t.Fatal("expected v2 transport to be selected")
	}
	fn(t)
}

// testWriteReadAfterCleanup reproduces the exact crash scenario from the
// downloader panic: a transport whose session secrets have already been zeroed
// by Cleanup() (connection teardown) while handshakeDone is still true, and a
// protocol goroutine subsequently issues a write/read. Before the fix this
// dereferenced a nil cipher.AEAD inside WriteEncrypted (cipher.Overhead) and
// panicked. After the fix the calls must return an error instead.
func testWriteReadAfterCleanup(t *testing.T) {
	waitTime := 5 * time.Second
	client, server, clientConn, serverConn := newHandshakedPair(t, waitTime)
	defer clientConn.Close()
	defer serverConn.Close()

	data := make([]byte, 64)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}

	// Simulate connection teardown zeroing the session secrets.
	client.Cleanup()

	// Must not panic; must surface an error.
	if err := client.WriteEncrypted(data, 1, PacketTypeApplicationData); err == nil {
		t.Fatal("expected error writing after Cleanup, got nil")
	}
	if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadAndDecrypt(PacketTypeApplicationData); err == nil {
		t.Fatal("expected error reading after Cleanup, got nil")
	}

	// Same expectation on the server side.
	server.Cleanup()
	if err := server.WriteEncrypted(data, 1, PacketTypeApplicationData); err == nil {
		t.Fatal("expected error writing after server Cleanup, got nil")
	}
	if _, err := server.ReadAndDecrypt(PacketTypeApplicationData); err == nil {
		t.Fatal("expected error reading after server Cleanup, got nil")
	}
}

func Test_WriteReadAfterCleanup(t *testing.T) {
	withV2KemSwitchTime(t, testWriteReadAfterCleanup)
}

// testConcurrentCloseAndWrite hammers WriteEncrypted from many goroutines while
// Cleanup() runs concurrently, mimicking the production race between a peer's
// protocol handlers (e.g. the downloader fetching headers) and connection
// teardown. Any nil-cipher dereference would panic and crash the test binary.
// Run with -race to also catch the data race on the cipher/IV fields.
func testConcurrentCloseAndWrite(t *testing.T) {
	waitTime := 10 * time.Second
	client, server, clientConn, serverConn := newHandshakedPair(t, waitTime)
	defer clientConn.Close()
	defer serverConn.Close()

	// Continuously drain the server side so client writes never block on a full
	// pipe buffer. The loop exits once reads start failing after teardown.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for {
			if err := serverConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
				return
			}
			if _, err := server.ReadAndDecrypt(PacketTypeApplicationData); err != nil {
				return
			}
		}
	}()

	const writers = 8
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := make([]byte, 64)
			rand.Read(data)
			for {
				if err := clientConn.SetDeadline(time.Now().Add(waitTime)); err != nil {
					return
				}
				// This must never panic, even when Cleanup() races it.
				if err := client.WriteEncrypted(data, 1, PacketTypeApplicationData); err != nil {
					return
				}
			}
		}()
	}

	// Let the writers ramp up, then tear down concurrently with in-flight writes.
	time.Sleep(25 * time.Millisecond)
	client.Cleanup()
	wg.Wait()

	// After Cleanup the transport must consistently reject writes (without
	// panicking) rather than dereferencing the now-nil cipher.
	if err := client.WriteEncrypted([]byte{0x1}, 1, PacketTypeApplicationData); err == nil {
		t.Fatal("expected error writing after concurrent Cleanup, got nil")
	}

	clientConn.Close()
	serverConn.Close()
	<-serverDone
}

func Test_ConcurrentCloseAndWrite(t *testing.T) {
	withV2KemSwitchTime(t, testConcurrentCloseAndWrite)
}
