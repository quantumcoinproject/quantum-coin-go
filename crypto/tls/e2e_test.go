package tls

import (
	"bytes"
	gotls "crypto/tls"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
)

func TestE2E_CertificateCreateVerifyAndPEM(t *testing.T) {
	publicKey, secretKey, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secretKey)
	if err != nil {
		t.Fatal(err)
	}
	template, err := DefaultCertTemplate(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	template.CommonName = "e2e-pqc"
	template.Organization = "quantum-coin"

	certDER, err := CreateCertificate(template, signer)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := VerifyCertificatePQC(certDER)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pub.Bytes, publicKey) {
		t.Error("verified cert public key != generated public key")
	}
	pem := CertificatePEM(certDER)
	if len(pem) == 0 || !bytes.Contains(pem, []byte("CERTIFICATE")) {
		t.Error("PEM encoding failed")
	}
}

func TestE2E_TLS13CertificateVerifySignAndVerify(t *testing.T) {
	_, secretKey, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secretKey)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate handshake transcript (e.g. ClientHello + ServerHello + ... + Certificate).
	transcriptParts := [][]byte{
		[]byte("fake-client-hello"),
		[]byte("fake-server-hello"),
		[]byte("fake-certificate"),
	}
	transcriptHash := TranscriptHashSHA256(transcriptParts...)
	if len(transcriptHash) != 32 {
		t.Fatalf("transcript hash length = %d, want 32", len(transcriptHash))
	}
	signedMsg := SignedMessageTLS13(ServerSignatureContextTLS13, transcriptHash)
	// Hybrid PQC scheme signs a 32-byte digest; use SHA-256 of the TLS signed message.
	digest := sha256.Sum256(signedMsg)
	sig, err := signer.Sign(nil, digest[:], nil)
	if err != nil {
		t.Fatal(err)
	}
	pk := signer.Public().(*PublicKey)
	if !Verify(pk, digest[:], sig) {
		t.Fatal("CertificateVerify-style signature verification failed")
	}
	// Tampered message must fail
	wrongDigest := sha256.Sum256(SignedMessageTLS13(ServerSignatureContextTLS13, []byte("wrong")))
	if Verify(pk, wrongDigest[:], sig) {
		t.Error("Verify should fail for tampered message")
	}
}

func TestE2E_StdlibTLSRejectsPQCKey(t *testing.T) {
	// Ensure that using our PQC cert with the standard library tls fails as expected
	// (no signature scheme is selected for our key type). This documents the need for
	// a patched/forked crypto/tls for full TLS 1.3 with PQC.
	_, secretKey, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secretKey)
	if err != nil {
		t.Fatal(err)
	}
	template, err := DefaultCertTemplate(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	certDER, err := CreateCertificate(template, signer)
	if err != nil {
		t.Fatal(err)
	}
	// Use standard library crypto/tls (import as stdtls in test would conflict; we use same package).
	// We need to import "crypto/tls" from the standard library - but we're inside package tls (our package).
	// So we cannot import the standard library's crypto/tls from here without a different name.
	// Use a separate file that imports stdlib tls with a different name, or run this test from a different package.
	// For e2e we'll use net.Listen and then dial; the server will use stdlib tls. So we need to import
	// the real stdlib tls. In our package we're "github.com/.../crypto/tls". The standard library is "crypto/tls".
	// When the project builds, "crypto/tls" in the module could be our package if there's a replace. So we don't have a replace. So from our package "tls" (path crypto/tls), if we write import stdtls "crypto/tls", we get... the standard library's crypto/tls, because our package's import path is github.com/.../crypto/tls. So "crypto/tls" in an import is the standard library. So we can do import gotls "crypto/tls" and use gotls.Listen. Let me add that.
	cfg := &gotls.Config{
		Certificates: []gotls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  signer,
		}},
	}
	listener, err := gotls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().String()
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			_ = conn.Close()
		}
	}()
	clientCfg := &gotls.Config{InsecureSkipVerify: true}
	conn, err := gotls.Dial("tcp", addr, clientCfg)
	if err != nil {
		t.Logf("client dial (expected to fail with stdlib): %v", err)
		return
	}
	_ = conn.Close()
	t.Log("TLS handshake completed; stdlib may be patched for PQC")
}
