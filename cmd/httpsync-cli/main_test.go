package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/consensus/mockconsensus"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/eth/httpsync"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

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

func TestRun_Status(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	srv := httpsync.NewServer(chain, "", "", nil, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var buf bytes.Buffer
	err := Run([]string{"-server", ts.URL, "-cmd", "status"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	var status httpsync.StatusResponse
	if err := json.Unmarshal(buf.Bytes(), &status); err != nil {
		t.Fatal("invalid JSON:", err)
	}
	head := chain.CurrentHeader()
	if head == nil {
		t.Fatal("no head")
	}
	if status.Height != head.Number.Uint64() || status.Hash != head.Hash() {
		t.Errorf("status: height=%d hash=%v; want %d %v", status.Height, status.Hash, head.Number.Uint64(), head.Hash())
	}
}

func TestRun_HeadersFromCount(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	srv := httpsync.NewServer(chain, "", "", nil, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var buf bytes.Buffer
	err := Run([]string{"-server", ts.URL, "-cmd", "headers", "-from", "0", "-count", "2"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	var headers []struct {
		Number uint64 `json:"number"`
		Hash   string `json:"hash"`
	}
	if err := json.Unmarshal(buf.Bytes(), &headers); err != nil {
		t.Fatal("invalid JSON:", err)
	}
	if len(headers) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(headers))
	}
	if headers[0].Number != 0 {
		t.Errorf("first header number: got %d, want 0", headers[0].Number)
	}
}

func TestRun_HeadersNumber(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	srv := httpsync.NewServer(chain, "", "", nil, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var buf bytes.Buffer
	err := Run([]string{"-server", ts.URL, "-cmd", "headers", "-number", "0"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	var headers []struct {
		Number uint64 `json:"number"`
	}
	if err := json.Unmarshal(buf.Bytes(), &headers); err != nil {
		t.Fatal("invalid JSON:", err)
	}
	if len(headers) != 1 || headers[0].Number != 0 {
		t.Fatalf("expected 1 header with number 0, got %+v", headers)
	}
}

func TestRun_BlockNumber(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	srv := httpsync.NewServer(chain, "", "", nil, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var buf bytes.Buffer
	err := Run([]string{"-server", ts.URL, "-cmd", "block", "-number", "0"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal("invalid JSON:", err)
	}
	if _, ok := out["transactions"]; !ok {
		t.Errorf("expected transactions key: %+v", out)
	}
}

func TestRun_BlocksNumber(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Stop()
	srv := httpsync.NewServer(chain, "", "", nil, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	var buf bytes.Buffer
	err := Run([]string{"-server", ts.URL, "-cmd", "blocks", "-number", "0,1"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	var summaries []struct {
		Index   int `json:"index"`
		TxCount int `json:"tx_count"`
	}
	if err := json.Unmarshal(buf.Bytes(), &summaries); err != nil {
		t.Fatal("invalid JSON:", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 block summaries, got %d", len(summaries))
	}
}

func TestRun_MissingServerOrCmd(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"-server", "http://x", "-cmd", ""}, &buf)
	if err == nil {
		t.Fatal("expected error when -cmd empty")
	}
	err = Run([]string{"-cmd", "status"}, &buf)
	if err == nil {
		t.Fatal("expected error when -server missing")
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"-server", "http://localhost:30304", "-cmd", "unknown"}, &buf)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

// TestRun_HTTPS_UsesInMemoryCert verifies that Run with https URL does not require -cert/-key (in-memory cert is generated).
// With no server listening we get a connection error, not a "missing cert" error.
func TestRun_HTTPS_UsesInMemoryCert(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"-server", "https://127.0.0.1:39999", "-cmd", "status"}, &buf)
	// Should not complain about missing cert/key; we generate in-memory. Failure is connection/refused or handshake.
	if err != nil && strings.Contains(err.Error(), "cert") && strings.Contains(err.Error(), "required") {
		t.Fatalf("HTTPS should use in-memory cert, not require -cert/-key: %v", err)
	}
}

// TestRun_MTLS_Success would run the CLI against a real HTTPS server with mTLS.
// Skipped: standard library TLS does not support PQC certs, so the handshake would fail with "unsupported certificate key".
func TestRun_MTLS_Success(t *testing.T) {
	t.Skip("stdlib TLS does not support PQC certs; full mTLS CLI test requires PQC-aware TLS stack")
}
