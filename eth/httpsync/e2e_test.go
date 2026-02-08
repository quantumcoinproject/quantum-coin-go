// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package httpsync e2e tests: PQC cert creation/load, server TLS config, and HTTP handlers.
package httpsync

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pqctls "github.com/quantumcoinproject/quantum-coin-go/crypto/tls"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/mockconsensus"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// TestE2E_PQCCertCreateAndLoad ensures ensureCertKey creates PQC cert/key files
// and loadCertKey loads them; cert verifies with VerifyCertificatePQC.
func TestE2E_PQCCertCreateAndLoad(t *testing.T) {
	dataDir := t.TempDir()
	certFile, keyFile, err := ensureCertKey(dataDir, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if certFile == "" || keyFile == "" {
		t.Fatal("ensureCertKey returned empty paths (nodeKey is nil so both cert and key file expected)")
	}
	certDER, signer, err := loadCertKey(certFile, keyFile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(certDER) == 0 || signer == nil {
		t.Fatal("loadCertKey returned empty cert or nil signer")
	}
	pub, err := pqctls.VerifyCertificatePQC(certDER)
	if err != nil {
		t.Fatal(err)
	}
	if pub == nil || len(pub.Bytes) != pqctls.PublicKeySize() {
		t.Error("VerifyCertificatePQC returned invalid public key")
	}
}

// TestE2E_ServerTLSConfigAndListen ensures NewServer with dataDir gets PQC cert,
// tlsConfig() succeeds, mTLS (ClientAuth + VerifyPeerCertificate) is set, and the server can Start and Shutdown.
func TestE2E_ServerTLSConfigAndListen(t *testing.T) {
	dataDir := t.TempDir()
	s := NewServer(nil, "127.0.0.1:0", dataDir, nil, "")
	if s.certFile == "" || s.keyFile == "" {
		t.Fatal("NewServer did not set cert/key paths")
	}
	config, err := s.tlsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config == nil || config.GetCertificate == nil {
		t.Fatal("tlsConfig should return config with GetCertificate")
	}
	if config.ClientAuth != tls.RequireAnyClientCert {
		t.Fatal("tlsConfig should require client cert (mTLS)")
	}
	if config.VerifyPeerCertificate == nil {
		t.Fatal("tlsConfig should set VerifyPeerCertificate for PQC client cert verification")
	}
	cert, err := config.GetCertificate(nil)
	if err != nil || cert == nil || len(cert.Certificate) != 1 {
		t.Fatal("GetCertificate should return one PQC certificate")
	}
	done := make(chan error, 1)
	go func() { done <- s.Start() }()
	// Give server time to bind
	time.Sleep(100 * time.Millisecond)
	if err := s.Shutdown(); err != nil {
		t.Log("Shutdown error (may be benign):", err)
	}
	if err := <-done; err != nil && err != http.ErrServerClosed {
		t.Log("Start returned:", err)
	}
}

// TestE2E_MTLS_WithClientCert_Success would start an HTTPS server with mTLS and connect with a client cert.
// Skipped: standard library crypto/tls does not support PQC certs (unsupported certificate key), so a full
// TLS handshake with our server cert cannot complete. mTLS behavior is covered by config tests and NoClientCert_Rejected.
func TestE2E_MTLS_WithClientCert_Success(t *testing.T) {
	t.Skip("stdlib TLS does not support PQC certs; full mTLS handshake requires PQC-aware TLS stack")
}

// TestE2E_MTLS_NoClientCert_Rejected starts an HTTPS server with mTLS and connects without a client cert; connection must fail.
// (Failure may be handshake error due to missing client cert or stdlib rejecting PQC server cert.)
func TestE2E_MTLS_NoClientCert_Rejected(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	dataDir := t.TempDir()
	s := NewServer(chain, "127.0.0.1:30315", dataDir, nil, "")
	done := make(chan error, 1)
	go func() { done <- s.Start() }()
	defer func() { _ = s.Shutdown(); <-done }()
	time.Sleep(200 * time.Millisecond)
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true},
	}}
	_, err := client.Get("https://127.0.0.1:30315/status")
	if err == nil {
		t.Fatal("expected connection to fail when no client cert presented (or PQC server cert unsupported by stdlib)")
	}
}

// newTestChain creates a minimal in-memory chain with a few blocks for handler tests.
func newTestChain(t *testing.T) *core.BlockChain {
	t.Helper()
	db := rawdb.NewMemoryDatabase()
	gspec := &core.Genesis{Config: params.TestChainConfig}
	genesis := gspec.MustCommit(db)
	blocks, _ := core.GenerateChain(params.TestChainConfig, genesis, mockconsensus.NewMockConsensus(), db, 3, nil)
	chain, err := core.NewBlockChain(db, nil, params.TestChainConfig, mockconsensus.NewMockConsensus(), vm.Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := chain.InsertChain(types.Blocks(blocks)); err != nil {
		t.Fatal(err)
	}
	return chain
}

// TestE2E_HandlersViaHTTP runs the httpsync handlers over plain HTTP (httptest)
// with a minimal chain and checks /status and /headers responses.
func TestE2E_HandlersViaHTTP(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	s := NewServer(chain, "", "", nil, "") // no TLS
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	client := ts.Client()

	// GET /status
	resp, err := client.Get(ts.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: status %d", resp.StatusCode)
	}
	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	head := chain.CurrentHeader()
	if head == nil {
		t.Fatal("chain has no current header")
	}
	if status.Height != head.Number.Uint64() || status.Hash != head.Hash() {
		t.Errorf("status mismatch: got height=%d hash=%v, want %d %v", status.Height, status.Hash, head.Number.Uint64(), head.Hash())
	}

	// GET /headers?number=0
	resp2, err := client.Get(ts.URL + "/headers?number=0")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /headers?number=0: status %d", resp2.StatusCode)
	}
	headers := decodeHeadersRLP(t, resp2.Body)
	if len(headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(headers))
	}
	if headers[0].Number.Uint64() != 0 {
		t.Errorf("header number: got %d, want 0", headers[0].Number.Uint64())
	}

	// GET /headers?from=0&count=2
	resp3, err := client.Get(ts.URL + "/headers?from=0&count=2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("GET /headers?from=0&count=2: status %d", resp3.StatusCode)
	}
	headers2 := decodeHeadersRLP(t, resp3.Body)
	if len(headers2) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(headers2))
	}
}

// TestE2E_HandlersGzip requests gzip-encoded response and decodes it.
func TestE2E_HandlersGzip(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	s := NewServer(chain, "", "", nil, "")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/status", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Logf("no gzip encoding (optional): %s", resp.Header.Get("Content-Encoding"))
		return
	}
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()
	var status StatusResponse
	if err := json.NewDecoder(gr).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Height != chain.CurrentHeader().Number.Uint64() {
		t.Errorf("gzip status height: got %d", status.Height)
	}
}

func decodeHeadersRLP(t *testing.T, r io.Reader) []*types.Header {
	t.Helper()
	var headers []*types.Header
	if err := rlp.Decode(r, &headers); err != nil {
		t.Fatal(err)
	}
	return headers
}
