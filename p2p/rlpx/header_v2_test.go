package rlpx

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// newV2TestSecret derives a full v2 SessionSecret (both epochs, including
// header-protection keys) from fixed inputs.
func newV2TestSecret(t testing.TB) *SessionSecret {
	t.Helper()
	secret, err := NewSessionSecretV2(sha3Sum256([]byte("transcript")), sha3Sum256([]byte("shared")))
	if err != nil {
		t.Fatal(err)
	}
	if err := secret.CreateApplicationSecrets(sha3Sum256([]byte("app transcript"))); err != nil {
		t.Fatal(err)
	}
	return secret
}

// trackingConn serves reads from a fixed byte slice and records how many bytes
// the transport actually consumed, so tests can prove no body bytes are read
// after a failed header authentication.
type trackingConn struct {
	data []byte
	pos  int
}

func (c *trackingConn) Read(p []byte) (int, error) {
	if c.pos >= len(c.data) {
		return 0, io.EOF
	}
	n := copy(p, c.data[c.pos:])
	c.pos += n
	return n, nil
}

func (c *trackingConn) Write(p []byte) (int, error) { return len(p), nil }

// captureRecord writes one record through a real ClientV2 (client->server
// direction) and returns the wire bytes. seqOffset pre-advances the client's
// sequence number so records can be captured "as if" they were the Nth record.
func captureRecord(t *testing.T, secret *SessionSecret, packetType PacketType, fragment []byte, seqOffset uint64) []byte {
	t.Helper()
	var buf bytes.Buffer
	client := NewClientV2(&buf, nil, nil, "test")
	client.InitWithSecrets(*secret)
	if packetType == PacketTypeHandshake {
		client.clientSeqNumHandshake = seqOffset
	} else {
		client.clientSeqNumApplication = seqOffset
	}
	if err := client.WriteEncrypted(fragment, 1, packetType); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newReadingServer returns a ServerV2 that reads from the given wire bytes
// (client->server direction) and the trackingConn for consumption assertions.
func newReadingServer(secret *SessionSecret, wire []byte) (*ServerV2, *trackingConn) {
	conn := &trackingConn{data: wire}
	server := NewServerV2(conn, nil, "test")
	server.InitWithSecrets(*secret)
	return server, conn
}

func TestPackUnpackHeaderV2RoundTrip(t *testing.T) {
	for _, bodyLen := range []uint32{0, 1, 16, 17, 65536, maxRecordLengthV2, 1<<32 - 1} {
		plain := packHeaderV2(bodyLen)
		got, err := unpackHeaderV2(plain[:])
		if err != nil {
			t.Fatalf("bodyLen %d: %v", bodyLen, err)
		}
		if got != bodyLen {
			t.Fatalf("bodyLen %d: round-trip returned %d", bodyLen, got)
		}
	}
}

func TestUnpackHeaderV2Rejects(t *testing.T) {
	base := packHeaderV2(1024)

	if _, err := unpackHeaderV2(base[:15]); err == nil {
		t.Fatal("short plaintext accepted")
	}
	if _, err := unpackHeaderV2(append(base[:], 0)); err == nil {
		t.Fatal("long plaintext accepted")
	}

	badVersion := base
	badVersion[0] = 1
	if _, err := unpackHeaderV2(badVersion[:]); err == nil {
		t.Fatal("wrong version accepted")
	}

	badFlags := base
	badFlags[1] = 1
	if _, err := unpackHeaderV2(badFlags[:]); err == nil {
		t.Fatal("nonzero flags accepted")
	}

	for i := 6; i < headerPlainLenV2; i++ {
		bad := base
		bad[i] = 0xff
		if _, err := unpackHeaderV2(bad[:]); err == nil {
			t.Fatalf("nonzero reserved byte at offset %d accepted", i)
		}
	}
}

// TestRecordRoundTripBothEpochs proves the rewritten write/read paths agree
// with themselves in both the handshake and application epochs.
func TestRecordRoundTripBothEpochs(t *testing.T) {
	secret := newV2TestSecret(t)
	for _, packetType := range []PacketType{PacketTypeHandshake, PacketTypeApplicationData} {
		fragment := make([]byte, 300)
		if _, err := rand.Read(fragment); err != nil {
			t.Fatal(err)
		}
		wire := captureRecord(t, secret, packetType, fragment, 0)
		server, _ := newReadingServer(secret, wire)
		dataPacket, err := server.ReadAndDecrypt(packetType)
		if err != nil {
			t.Fatalf("packetType %d: %v", packetType, err)
		}
		if !bytes.Equal(dataPacket.fragment, fragment) {
			t.Fatalf("packetType %d: fragment mismatch", packetType)
		}
	}
}

// TestTamperedHeaderRejected flips every byte of the 32-byte encrypted header
// in turn; each must fail authentication before any body byte is consumed.
func TestTamperedHeaderRejected(t *testing.T) {
	secret := newV2TestSecret(t)
	wire := captureRecord(t, secret, PacketTypeApplicationData, []byte("payload data"), 0)

	for i := 0; i < headerCiphertextLenV2; i++ {
		tampered := bytes.Clone(wire)
		tampered[i] ^= 0x01
		server, conn := newReadingServer(secret, tampered)
		_, err := server.ReadAndDecrypt(PacketTypeApplicationData)
		if err == nil {
			t.Fatalf("tampered header byte %d accepted", i)
		}
		if !strings.Contains(err.Error(), "header authentication failed") {
			t.Fatalf("tampered header byte %d: unexpected error %v", i, err)
		}
		if conn.pos != headerCiphertextLenV2 {
			t.Fatalf("tampered header byte %d: consumed %d bytes, want exactly %d (no body read)", i, conn.pos, headerCiphertextLenV2)
		}
	}
}

// TestTamperedBodyRejected flips a body ciphertext byte; the body AEAD must
// reject it.
func TestTamperedBodyRejected(t *testing.T) {
	secret := newV2TestSecret(t)
	wire := captureRecord(t, secret, PacketTypeApplicationData, []byte("payload data"), 0)

	tampered := bytes.Clone(wire)
	tampered[headerCiphertextLenV2] ^= 0x01
	server, _ := newReadingServer(secret, tampered)
	if _, err := server.ReadAndDecrypt(PacketTypeApplicationData); err == nil {
		t.Fatal("tampered body accepted")
	}
}

// TestForgedHeaderNoPreAuthAllocation is the F1 regression: garbage that a
// keyless attacker could send must fail header authentication after exactly 32
// bytes, with no body read (and hence no length-driven allocation).
func TestForgedHeaderNoPreAuthAllocation(t *testing.T) {
	garbage := make([]byte, 4096)
	if _, err := rand.Read(garbage); err != nil {
		t.Fatal(err)
	}
	server, conn := newReadingServer(newV2TestSecret(t), garbage)
	_, err := server.ReadAndDecrypt(PacketTypeApplicationData)
	if err == nil {
		t.Fatal("forged header accepted")
	}
	if !strings.Contains(err.Error(), "header authentication failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.pos != headerCiphertextLenV2 {
		t.Fatalf("consumed %d bytes, want exactly %d", conn.pos, headerCiphertextLenV2)
	}
}

// TestAuthenticatedOversizeLengthRejected covers the other half of F1: even a
// peer holding valid keys cannot force a read past the per-epoch record cap —
// the authenticated length is bounds-checked before any body byte is read.
func TestAuthenticatedOversizeLengthRejected(t *testing.T) {
	secret := newV2TestSecret(t)

	cases := []struct {
		name       string
		packetType PacketType
		claimed    uint32
	}{
		{"application", PacketTypeApplicationData, maxRecordLengthV2 + 1},
		{"handshake", PacketTypeHandshake, maxHandshakeRecordLengthV2 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hdrIv []byte
			var hdrCipherAEAD = secret.ClientApplicationHdrCipher
			hdrIv = secret.ClientApplicationHdrIv
			if tc.packetType == PacketTypeHandshake {
				hdrCipherAEAD = secret.ClientHandshakeHdrCipher
				hdrIv = secret.ClientHandshakeHdrIv
			}
			plain := packHeaderV2(tc.claimed)
			nonce, err := CalculateNonceV2(0, hdrIv)
			if err != nil {
				t.Fatal(err)
			}
			hdrCt := hdrCipherAEAD.Seal(nil, nonce, plain[:], nil)

			server, conn := newReadingServer(secret, hdrCt)
			_, err = server.ReadAndDecrypt(tc.packetType)
			if err == nil {
				t.Fatal("oversize length accepted")
			}
			if !strings.Contains(err.Error(), "exceeds maximum allowed size") {
				t.Fatalf("unexpected error: %v", err)
			}
			if conn.pos != headerCiphertextLenV2 {
				t.Fatalf("consumed %d bytes, want exactly %d (no body read)", conn.pos, headerCiphertextLenV2)
			}
		})
	}
}

// TestSplicedRecordRejected pairs record A's header with record B's body; the
// header-plaintext AAD and per-record nonce must reject the splice whether or
// not the lengths happen to match.
func TestSplicedRecordRejected(t *testing.T) {
	secret := newV2TestSecret(t)

	// Same length, different sequence positions: nonce/AAD binding.
	recA := captureRecord(t, secret, PacketTypeApplicationData, bytes.Repeat([]byte{0xaa}, 100), 0)
	recB := captureRecord(t, secret, PacketTypeApplicationData, bytes.Repeat([]byte{0xbb}, 100), 1)
	spliced := append(bytes.Clone(recA[:headerCiphertextLenV2]), recB[headerCiphertextLenV2:]...)
	server, _ := newReadingServer(secret, spliced)
	if _, err := server.ReadAndDecrypt(PacketTypeApplicationData); err == nil {
		t.Fatal("same-length splice accepted")
	}

	// Different lengths at the same sequence position: length binding.
	recC := captureRecord(t, secret, PacketTypeApplicationData, bytes.Repeat([]byte{0xcc}, 500), 0)
	spliced2 := append(bytes.Clone(recA[:headerCiphertextLenV2]), recC[headerCiphertextLenV2:]...)
	server2, _ := newReadingServer(secret, spliced2)
	if _, err := server2.ReadAndDecrypt(PacketTypeApplicationData); err == nil {
		t.Fatal("different-length splice accepted")
	}
}

// TestReorderedRecordRejected delivers record #2 first; the per-record header
// nonce must reject it.
func TestReorderedRecordRejected(t *testing.T) {
	secret := newV2TestSecret(t)
	rec1 := captureRecord(t, secret, PacketTypeApplicationData, []byte("second record"), 1)
	server, _ := newReadingServer(secret, rec1)
	_, err := server.ReadAndDecrypt(PacketTypeApplicationData)
	if err == nil {
		t.Fatal("reordered record accepted")
	}
	if !strings.Contains(err.Error(), "header authentication failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestWideTypeConfusionRejected is the F4 regression: an EncryptedPayload
// whose PacketType only matches after truncation to a byte must be rejected.
// The record is crafted directly with the session keys.
func TestWideTypeConfusionRejected(t *testing.T) {
	secret := newV2TestSecret(t)

	payload := &EncryptedPayload{
		PacketType: uint(PacketTypeApplicationData) + 0x100, // truncates to 23
		Context:    1,
		Fragment:   []byte("data"),
	}
	payloadData, err := rlp.EncodeToBytes(payload)
	if err != nil {
		t.Fatal(err)
	}
	bodyLen := len(payloadData) + secret.ClientApplicationCipher.Overhead()
	headerPlain := packHeaderV2(uint32(bodyLen))
	hdrNonce, err := CalculateNonceV2(0, secret.ClientApplicationHdrIv)
	if err != nil {
		t.Fatal(err)
	}
	bodyNonce, err := CalculateNonceV2(0, secret.ClientApplicationIv)
	if err != nil {
		t.Fatal(err)
	}
	wire := secret.ClientApplicationHdrCipher.Seal(nil, hdrNonce, headerPlain[:], nil)
	wire = secret.ClientApplicationCipher.Seal(wire, bodyNonce, payloadData, headerPlain[:])

	server, _ := newReadingServer(secret, wire)
	_, err = server.ReadAndDecrypt(PacketTypeApplicationData)
	if err == nil {
		t.Fatal("wide packet type accepted")
	}
	if !strings.Contains(err.Error(), "packetType mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeserializeBoundedV2(t *testing.T) {
	serializer := NewRlpxSerializer()

	// Round trip: the random 100-199 byte zero padding must be accepted.
	msg := &FinishedMessage{VerifyData: []byte("verify data bytes")}
	frame, err := serializer.Serialize(msg)
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(FinishedMessage)
	raw, err := deserializeBoundedV2(decoded, bytes.NewReader(frame), maxHelloSizeV2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.VerifyData, msg.VerifyData) {
		t.Fatal("round trip mismatch")
	}
	if !bytes.Equal(raw, frame[2:]) {
		t.Fatal("returned wire bytes do not match the frame")
	}

	// Nonzero trailing padding byte must be rejected.
	bad := bytes.Clone(frame)
	bad[len(bad)-1] = 0x01
	if _, err := deserializeBoundedV2(new(FinishedMessage), bytes.NewReader(bad), maxHelloSizeV2); err == nil {
		t.Fatal("nonzero trailing byte accepted")
	}

	// A frame larger than the cap must be rejected before the body is read.
	oversize := []byte{0x20, 0x00} // 8192-byte frame claim
	consumed := &trackingConn{data: append(oversize, make([]byte, 8192)...)}
	if _, err := deserializeBoundedV2(new(FinishedMessage), consumed, maxHelloSizeV2); err == nil {
		t.Fatal("oversize frame accepted")
	}
	if consumed.pos != 2 {
		t.Fatalf("consumed %d bytes of an oversize frame, want only the 2-byte prefix", consumed.pos)
	}

	// Truncated frame must error out cleanly.
	if _, err := deserializeBoundedV2(new(FinishedMessage), bytes.NewReader(frame[:len(frame)-3]), maxHelloSizeV2); err == nil {
		t.Fatal("truncated frame accepted")
	}
}

// TestConcurrentCleanupDuringHandshakeRead is the F3 regression: Cleanup()
// racing an in-flight *handshake*-phase read must not zero cipher material out
// from under it (run with -race), and post-Cleanup reads must fail cleanly.
func TestConcurrentCleanupDuringHandshakeRead(t *testing.T) {
	secret := newV2TestSecret(t)
	clientSide, serverSide := net.Pipe()
	defer clientSide.Close()

	server := NewServerV2(serverSide, nil, "test")
	server.InitWithSecrets(*secret)

	readErr := make(chan error, 1)
	go func() {
		_, err := server.ReadAndDecrypt(PacketTypeHandshake)
		readErr <- err
	}()

	// Let the read block on the pipe, then tear down concurrently the same way
	// Conn.Close does: close the conn (unblocks the read), then Cleanup.
	time.Sleep(20 * time.Millisecond)
	cleanupDone := make(chan struct{})
	go func() {
		serverSide.Close()
		server.Cleanup()
		close(cleanupDone)
	}()

	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("expected the interrupted handshake read to fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handshake read did not return after close")
	}
	select {
	case <-cleanupDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Cleanup did not complete")
	}

	if _, err := server.ReadAndDecrypt(PacketTypeHandshake); err == nil {
		t.Fatal("expected error reading after Cleanup, got nil")
	}
}

func FuzzUnpackHeaderV2(f *testing.F) {
	seed := packHeaderV2(1024)
	f.Add(seed[:])
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		unpackHeaderV2(data)
	})
}

func FuzzReadAndDecryptV2(f *testing.F) {
	secret, err := NewSessionSecretV2(sha3Sum256([]byte("transcript")), sha3Sum256([]byte("shared")))
	if err != nil {
		f.Fatal(err)
	}
	if err := secret.CreateApplicationSecrets(sha3Sum256([]byte("app transcript"))); err != nil {
		f.Fatal(err)
	}
	var valid bytes.Buffer
	client := NewClientV2(&valid, nil, nil, "fuzz")
	client.InitWithSecrets(*secret)
	if err := client.WriteEncrypted([]byte("seed fragment"), 1, PacketTypeApplicationData); err != nil {
		f.Fatal(err)
	}
	f.Add(valid.Bytes())
	f.Add(make([]byte, headerCiphertextLenV2))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		server, conn := newReadingServer(secret, data)
		_, err := server.ReadAndDecrypt(PacketTypeApplicationData)
		// On any failed header authentication, nothing past the fixed-size
		// header may be consumed.
		if err != nil && strings.Contains(err.Error(), "header authentication failed") && conn.pos > headerCiphertextLenV2 {
			t.Fatalf("consumed %d bytes after failed header auth", conn.pos)
		}
	})
}

func FuzzDeserializeBoundedV2(f *testing.F) {
	serializer := NewRlpxSerializer()
	frame, err := serializer.Serialize(&FinishedMessage{VerifyData: []byte("seed")})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(frame)
	f.Add([]byte{0x00, 0x01, 0x00})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		deserializeBoundedV2(new(FinishedMessage), bytes.NewReader(data), maxHelloSizeV2)
	})
}
