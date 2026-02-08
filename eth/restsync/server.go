// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package restsync implements the REST sync server (block/header serving on port 30304).
package restsync

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

const (
	contentTypeRLP = "application/octet-stream"
)

// StatusResponse is the JSON response for GET /status.
type StatusResponse struct {
	Height uint64      `json:"height"`
	Hash   common.Hash `json:"hash"`
	TD     string      `json:"td"` // hex-encoded total difficulty
}

// Server serves block headers and bodies over HTTPS (RLP-encoded) for REST sync.
type Server struct {
	chain   *core.BlockChain
	srv     *http.Server
	log     log.Logger
	certFile string
	keyFile  string
}

// NewServer creates a new REST sync HTTPS server. Call Start to listen.
// dataDir is used to load or create a self-signed TLS cert (restsync-tls.crt/key).
func NewServer(chain *core.BlockChain, listenAddr, dataDir string) *Server {
	s := &Server{chain: chain, log: log.New("restsync", "server")}
	mux := http.NewServeMux()
	mux.HandleFunc("/headers", s.serveHeaders)
	mux.HandleFunc("/block", s.serveBlock)
	mux.HandleFunc("/blocks", s.serveBlocks)
	mux.HandleFunc("/status", s.serveStatus)
	// Gzip compression for all responses
	handler := gzipHandler(mux)
	s.srv = &http.Server{Addr: listenAddr, Handler: handler}
	if dataDir != "" {
		s.certFile, s.keyFile, _ = ensureCertKey(dataDir)
	}
	return s
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

// tlsConfig loads the PQC cert and key and returns a TLS config with TLS 1.3 and Certificates set.
func (s *Server) tlsConfig() (*tls.Config, error) {
	certDER, signer, err := loadCertKey(s.certFile, s.keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{{Certificate: [][]byte{certDER}, PrivateKey: signer}},
	}, nil
}

// Start starts the HTTPS server (TLS 1.3). It blocks until the server is stopped.
func (s *Server) Start() error {
	if s.certFile == "" || s.keyFile == "" {
		s.log.Info("REST sync: no TLS cert (missing dataDir?), skipping server")
		return nil
	}
	config, err := s.tlsConfig()
	if err != nil {
		s.log.Error("REST sync: failed to load TLS cert", "err", err)
		return err
	}
	s.srv.TLSConfig = config
	log.Info("REST sync server listening (HTTPS, TLS 1.3, PQC cert)", "addr", s.srv.Addr)
	return s.srv.ListenAndServeTLS("", "")
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
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
			reqLog.Info("REST sync GET /headers bad request", "query", "hash", "value", hashStr, "err", err)
			http.Error(w, "invalid hash", http.StatusBadRequest)
			return
		}
		reqLog.Info("REST sync GET /headers by hash", "hash", hash)
		h := s.chain.GetHeaderByHash(hash)
		if h != nil {
			headers = []*types.Header{h}
		}
	} else if nStr := q.Get("number"); nStr != "" {
		n, err := strconv.ParseUint(nStr, 10, 64)
		if err != nil {
			reqLog.Info("REST sync GET /headers bad request", "query", "number", "value", nStr, "err", err)
			http.Error(w, "invalid number", http.StatusBadRequest)
			return
		}
		reqLog.Info("REST sync GET /headers by number", "number", n)
		headers = eth.HeadersByNumber(s.chain, n, 1)
	} else {
		fromStr := q.Get("from")
		countStr := q.Get("count")
		if fromStr == "" {
			reqLog.Info("REST sync GET /headers bad request", "reason", "missing from or number")
			http.Error(w, "missing from or number", http.StatusBadRequest)
			return
		}
		from, err := strconv.ParseUint(fromStr, 10, 64)
		if err != nil {
			reqLog.Info("REST sync GET /headers bad request", "query", "from", "value", fromStr, "err", err)
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
		count := uint64(1024)
		if countStr != "" {
			count, err = strconv.ParseUint(countStr, 10, 64)
			if err != nil {
				reqLog.Info("REST sync GET /headers bad request", "query", "count", "value", countStr, "err", err)
				http.Error(w, "invalid count", http.StatusBadRequest)
				return
			}
		}
		reqLog.Info("REST sync GET /headers range", "from", from, "count", count)
		headers = eth.HeadersByNumber(s.chain, from, count)
	}
	if len(headers) == 0 {
		reqLog.Info("REST sync GET /headers not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	reqLog.Info("REST sync GET /headers ok", "count", len(headers))
	w.Header().Set("Content-Type", contentTypeRLP)
	if err := rlp.Encode(w, headers); err != nil {
		reqLog.Info("REST sync encode headers failed", "err", err)
	}
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
			reqLog.Info("REST sync GET /block bad request", "query", "hash", "err", err)
			http.Error(w, "invalid hash", http.StatusBadRequest)
			return
		}
		reqLog.Info("REST sync GET /block by hash", "hash", hash)
		body = eth.BodyByHash(s.chain, hash)
	} else if nStr := q.Get("number"); nStr != "" {
		n, err := strconv.ParseUint(nStr, 10, 64)
		if err != nil {
			reqLog.Info("REST sync GET /block bad request", "query", "number", "err", err)
			http.Error(w, "invalid number", http.StatusBadRequest)
			return
		}
		reqLog.Info("REST sync GET /block by number", "number", n)
		body = eth.BodyByNumber(s.chain, n)
	} else {
		http.Error(w, "missing hash or number", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		reqLog.Info("REST sync GET /block not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	reqLog.Info("REST sync GET /block ok", "size", len(body))
	w.Header().Set("Content-Type", contentTypeRLP)
	w.Write(body)
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
		reqLog.Info("REST sync GET /blocks by hashes", "count", len(strings.Split(hashStr, ",")))
		for _, part := range strings.Split(hashStr, ",") {
			part = strings.TrimSpace(part)
			var h common.Hash
			if err := h.UnmarshalText([]byte(part)); err != nil {
				reqLog.Info("REST sync GET /blocks bad request", "query", "hash", "value", part, "err", err)
				http.Error(w, "invalid hash", http.StatusBadRequest)
				return
			}
			hashes = append(hashes, h)
		}
	} else if nStr := q.Get("number"); nStr != "" {
		reqLog.Info("REST sync GET /blocks by numbers", "count", len(strings.Split(nStr, ",")))
		for _, numStr := range strings.Split(nStr, ",") {
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseUint(numStr, 10, 64)
			if err != nil {
				reqLog.Info("REST sync GET /blocks bad request", "query", "number", "value", numStr, "err", err)
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
		reqLog.Info("REST sync GET /blocks not found", "reason", "no hashes resolved")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	bodies := eth.BodiesByHashes(s.chain, hashes)
	if len(bodies) == 0 {
		reqLog.Info("REST sync GET /blocks not found", "reason", "no bodies")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	reqLog.Info("REST sync GET /blocks ok", "count", len(bodies))
	w.Header().Set("Content-Type", contentTypeRLP)
	if err := rlp.Encode(w, bodies); err != nil {
		reqLog.Info("REST sync encode bodies failed", "err", err)
	}
}

func (s *Server) serveStatus(w http.ResponseWriter, r *http.Request) {
	reqLog := s.log.New("remotepeer", r.RemoteAddr)
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	head := s.chain.CurrentHeader()
	if head == nil {
		reqLog.Info("REST sync GET /status no header")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	td := s.chain.GetTd(head.Hash(), head.Number.Uint64())
	if td == nil {
		td = common.Big0
	}
	reqLog.Info("REST sync GET /status ok", "height", head.Number.Uint64(), "hash", head.Hash())
	resp := StatusResponse{
		Height: head.Number.Uint64(),
		Hash:   head.Hash(),
		TD:     hexutil.EncodeBig(td),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
