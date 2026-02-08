// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package httpsync implements HTTP streaming sync: block/header serving on port 30304 over HTTPS with chunked transfer.
package httpsync

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	pqctls "github.com/quantumcoinproject/quantum-coin-go/crypto/tls"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

const (
	contentTypeRLP   = "application/octet-stream"
	streamChunkSize = 32 * 1024 // write RLP payloads in 32KB chunks for HTTP streaming
)

// StatusResponse is the JSON response for GET /status.
type StatusResponse struct {
	Height uint64      `json:"height"`
	Hash   common.Hash `json:"hash"`
	TD     string      `json:"td"` // hex-encoded total difficulty
}

// Server serves block headers and bodies over HTTPS (RLP-encoded) with HTTP streaming (chunked transfer, flush per chunk).
// The TLS certificate can be renewed at runtime (when expiry is within 30 days) by a background goroutine without restart.
type Server struct {
	chain   *core.BlockChain
	srv     *http.Server
	log     log.Logger
	certFile string
	keyFile  string
	nodeKey  *signaturealgorithm.PrivateKey // optional; when set and compact PQC type, TLS cert uses it (no key file)

	// certCommonName is the Subject CN for the TLS cert (e.g. node peer ID from ENR); used when creating/renewing the cert.
	certCommonName string

	// certMu protects currentCert; GetCertificate returns it and the renewal goroutine updates it.
	certMu      sync.RWMutex
	currentCert *tls.Certificate
	stopRenew   chan struct{} // closed when server is shutting down to stop the renewal goroutine
}

// NewServer creates a new HTTP sync HTTPS server. Call Start to listen.
// dataDir is used to load or create a self-signed TLS cert (httpsync-tls.crt, and httpsync-tls.key only if not using nodeKey).
// nodeKey may be nil; when non-nil and the key is the compact PQC type (same as TLS), the cert is created from it and no key file is written.
// peerID is the node's peer ID (from ENR); used as the cert Subject CN when creating/renewing the cert. If empty, a default CN is used.
func NewServer(chain *core.BlockChain, listenAddr, dataDir string, nodeKey *signaturealgorithm.PrivateKey, peerID string) *Server {
	s := &Server{chain: chain, log: log.New("httpsync", "server"), nodeKey: nodeKey, certCommonName: peerID}
	mux := http.NewServeMux()
	mux.HandleFunc("/headers", s.serveHeaders)
	mux.HandleFunc("/block", s.serveBlock)
	mux.HandleFunc("/blocks", s.serveBlocks)
	mux.HandleFunc("/status", s.serveStatus)
	// Gzip compression for all responses
	handler := gzipHandler(mux)
	s.srv = &http.Server{Addr: listenAddr, Handler: handler}
	if dataDir != "" {
		s.certFile, s.keyFile, _ = ensureCertKey(dataDir, nodeKey, s.certCommonName)
	}
	return s
}

// Handler returns the HTTP handler for use with httptest or custom servers.
func (s *Server) Handler() http.Handler {
	return s.srv.Handler
}

// gzipHandler wraps an http.Handler and compresses responses with gzip when the client accepts it.
func gzipHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !contains(r.Header.Values("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		h.ServeHTTP(&gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	})
}

func contains(vals []string, s string) bool {
	for _, v := range vals {
		if strings.Contains(strings.ToLower(v), s) {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) { return w.Writer.Write(b) }

// Flush sends any buffered data to the client (implements http.Flusher for HTTP streaming).
func (w *gzipResponseWriter) Flush() {
	if gz, ok := w.Writer.(*gzip.Writer); ok {
		gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// writeStreamChunked writes data in chunks and flushes after each chunk so the client receives data incrementally (HTTP streaming).
func writeStreamChunked(w http.ResponseWriter, data []byte) {
	if len(data) == 0 {
		return
	}
	for off := 0; off < len(data); off += streamChunkSize {
		end := off + streamChunkSize
		if end > len(data) {
			end = len(data)
		}
		if _, err := w.Write(data[off:end]); err != nil {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// loadCurrentCert loads cert and key from disk (or nodeKey) and returns a tls.Certificate. Caller must not hold certMu.
func (s *Server) loadCurrentCert() (*tls.Certificate, error) {
	certDER, signer, err := loadCertKey(s.certFile, s.keyFile, s.nodeKey)
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  signer,
	}, nil
}

// logCertMetadata reads the current cert file and logs CN, organization, NotBefore, and NotAfter.
func (s *Server) logCertMetadata() {
	if s.certFile == "" {
		return
	}
	pemBytes, err := os.ReadFile(s.certFile)
	if err != nil {
		s.log.Warn("HTTP sync: could not read cert for metadata", "err", err)
		return
	}
	certDER, err := pqctls.PEMToCertDER(pemBytes)
	if err != nil {
		s.log.Warn("HTTP sync: could not parse cert for metadata", "err", err)
		return
	}
	meta, err := pqctls.CertMetadata(certDER)
	if err != nil {
		s.log.Warn("HTTP sync: could not get cert metadata", "err", err)
		return
	}
	s.log.Info("HTTP sync TLS cert",
		"CN", meta.CommonName,
		"organization", meta.Organization,
		"notBefore", meta.NotBefore,
		"notAfter", meta.NotAfter)
}

// reloadCertIfExpiringSoon checks if the cert expires within 30 days; if so, regenerates it and updates currentCert.
// Must be called without holding certMu (it takes the write lock internally).
func (s *Server) reloadCertIfExpiringSoon() {
	if s.certFile == "" {
		return
	}
	if !certExpiresSoon(s.certFile) {
		return
	}
	dataDir := filepath.Dir(s.certFile)
	if _, _, err := ensureCertKey(dataDir, s.nodeKey, s.certCommonName); err != nil {
		s.log.Warn("HTTP sync: cert renewal (ensure) failed", "err", err)
		return
	}
	cert, err := s.loadCurrentCert()
	if err != nil {
		s.log.Warn("HTTP sync: cert renewal (load) failed", "err", err)
		return
	}
	s.certMu.Lock()
	s.currentCert = cert
	s.certMu.Unlock()
	s.log.Info("HTTP sync: TLS certificate renewed (expiry was within 30 days)")
}

// tlsConfig returns a TLS config that uses GetCertificate so the cert can be hot-reloaded by the renewal goroutine.
func (s *Server) tlsConfig() (*tls.Config, error) {
	cert, err := s.loadCurrentCert()
	if err != nil {
		return nil, err
	}
	s.certMu.Lock()
	s.currentCert = cert
	s.certMu.Unlock()
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			s.certMu.RLock()
			defer s.certMu.RUnlock()
			return s.currentCert, nil
		},
	}, nil
}

// Start starts the HTTPS server (TLS 1.3). It blocks until the server is stopped.
// A background goroutine checks cert expiry once per day and renews the cert if within 30 days of expiry (no restart needed).
func (s *Server) Start() error {
	if s.certFile == "" {
		s.log.Info("HTTP sync: no TLS cert (missing dataDir?), skipping server")
		return nil
	}
	if s.keyFile == "" && s.nodeKey == nil {
		s.log.Info("HTTP sync: no TLS key and no node key, skipping server")
		return nil
	}
	config, err := s.tlsConfig()
	if err != nil {
		s.log.Error("HTTP sync: failed to load TLS cert", "err", err)
		return err
	}
	s.srv.TLSConfig = config
	s.stopRenew = make(chan struct{})
	go s.certRenewalLoop()
	s.logCertMetadata()
	log.Info("HTTP sync server listening (HTTPS, TLS 1.3, PQC cert)", "addr", s.srv.Addr)
	return s.srv.ListenAndServeTLS("", "")
}

// certRenewalLoop runs until stopRenew is closed; checks cert expiry every certCheckInterval and renews if needed.
func (s *Server) certRenewalLoop() {
	ticker := time.NewTicker(certCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopRenew:
			return
		case <-ticker.C:
			s.reloadCertIfExpiringSoon()
		}
	}
}

// Shutdown gracefully shuts down the server and stops the cert renewal goroutine.
func (s *Server) Shutdown() error {
	if s.stopRenew != nil {
		close(s.stopRenew)
		s.stopRenew = nil
	}
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

func (s *Server) serveHeaders(w http.ResponseWriter, r *http.Request) {
	reqLog := s.log.New("remotepeer", r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	var headers []*types.Header
	if hashStr := q.Get("hash"); hashStr != "" {
		var hash common.Hash
		if err := hash.UnmarshalText([]byte(hashStr)); err != nil {
			reqLog.Info("HTTP sync GET /headers bad request", "query", "hash", "value", hashStr, "err", err)
			http.Error(w, "invalid hash", http.StatusBadRequest)
			return
		}
		reqLog.Info("HTTP sync GET /headers by hash", "hash", hash)
		h := s.chain.GetHeaderByHash(hash)
		if h != nil {
			headers = []*types.Header{h}
		}
	} else if nStr := q.Get("number"); nStr != "" {
		n, err := strconv.ParseUint(nStr, 10, 64)
		if err != nil {
			reqLog.Info("HTTP sync GET /headers bad request", "query", "number", "value", nStr, "err", err)
			http.Error(w, "invalid number", http.StatusBadRequest)
			return
		}
		reqLog.Info("HTTP sync GET /headers by number", "number", n)
		headers = eth.HeadersByNumber(s.chain, n, 1)
	} else {
		fromStr := q.Get("from")
		countStr := q.Get("count")
		if fromStr == "" {
			reqLog.Info("HTTP sync GET /headers bad request", "reason", "missing from or number")
			http.Error(w, "missing from or number", http.StatusBadRequest)
			return
		}
		from, err := strconv.ParseUint(fromStr, 10, 64)
		if err != nil {
			reqLog.Info("HTTP sync GET /headers bad request", "query", "from", "value", fromStr, "err", err)
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
		count := uint64(1024)
		if countStr != "" {
			count, err = strconv.ParseUint(countStr, 10, 64)
			if err != nil {
				reqLog.Info("HTTP sync GET /headers bad request", "query", "count", "value", countStr, "err", err)
				http.Error(w, "invalid count", http.StatusBadRequest)
				return
			}
		}
		reqLog.Info("HTTP sync GET /headers range", "from", from, "count", count)
		headers = eth.HeadersByNumber(s.chain, from, count)
	}
	if len(headers) == 0 {
		reqLog.Info("HTTP sync GET /headers not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	reqLog.Info("HTTP sync GET /headers ok", "count", len(headers))
	w.Header().Set("Content-Type", contentTypeRLP)
	// Do not set Content-Length so response uses Transfer-Encoding: chunked (HTTP streaming).
	var buf bytes.Buffer
	if err := rlp.Encode(&buf, headers); err != nil {
		reqLog.Info("HTTP sync encode headers failed", "err", err)
		return
	}
	writeStreamChunked(w, buf.Bytes())
}

func (s *Server) serveBlock(w http.ResponseWriter, r *http.Request) {
	reqLog := s.log.New("remotepeer", r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	var body rlp.RawValue
	if hashStr := q.Get("hash"); hashStr != "" {
		var hash common.Hash
		if err := hash.UnmarshalText([]byte(hashStr)); err != nil {
			reqLog.Info("HTTP sync GET /block bad request", "query", "hash", "err", err)
			http.Error(w, "invalid hash", http.StatusBadRequest)
			return
		}
		reqLog.Info("HTTP sync GET /block by hash", "hash", hash)
		body = eth.BodyByHash(s.chain, hash)
	} else if nStr := q.Get("number"); nStr != "" {
		n, err := strconv.ParseUint(nStr, 10, 64)
		if err != nil {
			reqLog.Info("HTTP sync GET /block bad request", "query", "number", "err", err)
			http.Error(w, "invalid number", http.StatusBadRequest)
			return
		}
		reqLog.Info("HTTP sync GET /block by number", "number", n)
		body = eth.BodyByNumber(s.chain, n)
	} else {
		http.Error(w, "missing hash or number", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		reqLog.Info("HTTP sync GET /block not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	reqLog.Info("HTTP sync GET /block ok", "size", len(body))
	w.Header().Set("Content-Type", contentTypeRLP)
	writeStreamChunked(w, body)
}

func (s *Server) serveBlocks(w http.ResponseWriter, r *http.Request) {
	reqLog := s.log.New("remotepeer", r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	var hashes []common.Hash
	if hashStr := q.Get("hash"); hashStr != "" {
		reqLog.Info("HTTP sync GET /blocks by hashes", "count", len(strings.Split(hashStr, ",")))
		for _, part := range strings.Split(hashStr, ",") {
			part = strings.TrimSpace(part)
			var h common.Hash
			if err := h.UnmarshalText([]byte(part)); err != nil {
				reqLog.Info("HTTP sync GET /blocks bad request", "query", "hash", "value", part, "err", err)
				http.Error(w, "invalid hash", http.StatusBadRequest)
				return
			}
			hashes = append(hashes, h)
		}
	} else if nStr := q.Get("number"); nStr != "" {
		reqLog.Info("HTTP sync GET /blocks by numbers", "count", len(strings.Split(nStr, ",")))
		for _, numStr := range strings.Split(nStr, ",") {
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseUint(numStr, 10, 64)
			if err != nil {
				reqLog.Info("HTTP sync GET /blocks bad request", "query", "number", "value", numStr, "err", err)
				http.Error(w, "invalid number", http.StatusBadRequest)
				return
			}
			header := s.chain.GetHeaderByNumber(n)
			if header != nil {
				hashes = append(hashes, header.Hash())
			}
		}
	} else {
		http.Error(w, "missing hash or number", http.StatusBadRequest)
		return
	}
	if len(hashes) == 0 {
		reqLog.Info("HTTP sync GET /blocks not found", "reason", "no hashes resolved")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	bodies := eth.BodiesByHashes(s.chain, hashes)
	if len(bodies) == 0 {
		reqLog.Info("HTTP sync GET /blocks not found", "reason", "no bodies")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	reqLog.Info("HTTP sync GET /blocks ok", "count", len(bodies))
	w.Header().Set("Content-Type", contentTypeRLP)
	var buf bytes.Buffer
	if err := rlp.Encode(&buf, bodies); err != nil {
		reqLog.Info("HTTP sync encode bodies failed", "err", err)
		return
	}
	writeStreamChunked(w, buf.Bytes())
}

func (s *Server) serveStatus(w http.ResponseWriter, r *http.Request) {
	reqLog := s.log.New("remotepeer", r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	head := s.chain.CurrentHeader()
	if head == nil {
		reqLog.Info("HTTP sync GET /status no header")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	td := s.chain.GetTd(head.Hash(), head.Number.Uint64())
	if td == nil {
		td = common.Big0
	}
	reqLog.Info("HTTP sync GET /status ok", "height", head.Number.Uint64(), "hash", head.Hash())
	resp := StatusResponse{
		Height: head.Number.Uint64(),
		Hash:   head.Hash(),
		TD:     hexutil.EncodeBig(td),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
