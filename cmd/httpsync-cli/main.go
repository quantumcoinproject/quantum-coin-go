// httpsync-cli is a CLI client for the HTTP sync server (block/header serving over HTTPS).
// It queries a remote server and outputs status, headers, block(s) as requested.
package main

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/eth/httpsync"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

const (
	defaultTimeout = 30 * time.Second
)

func main() {
	if err := Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Run runs the CLI with the given args and writes output to w. For HTTPS, a new in-memory client cert is generated for mTLS.
func Run(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("httpsync-cli", flag.ContinueOnError)
	server := fs.String("server", "", "Base URL of HTTP sync server")
	cmd := fs.String("cmd", "", "Command: status | headers | block | blocks")
	flagHash := fs.String("hash", "", "Block hash (hex)")
	flagNumber := fs.String("number", "", "Block number(s)")
	flagFrom := fs.Uint64("from", 0, "Start block number (for headers range)")
	flagCount := fs.Uint64("count", 1, "Count of headers (for headers range)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *server == "" || *cmd == "" {
		printUsageTo(os.Stderr)
		return fmt.Errorf("missing -server or -cmd")
	}
	baseURL := strings.TrimSuffix(*server, "/")
	var client *http.Client
	if strings.HasPrefix(baseURL, "https://") {
		var err error
		client, err = newClientMTLS()
		if err != nil {
			return err
		}
	} else {
		client = newClientInsecure()
	}
	switch *cmd {
	case "status":
		return runStatusTo(client, baseURL, w)
	case "headers":
		return runHeadersTo(client, baseURL, w, *flagHash, *flagNumber, *flagFrom, *flagCount)
	case "block":
		return runBlockTo(client, baseURL, w, *flagHash, *flagNumber)
	case "blocks":
		return runBlocksTo(client, baseURL, w, *flagHash, *flagNumber)
	default:
		printUsageTo(os.Stderr)
		return fmt.Errorf("unknown command: %s", *cmd)
	}
}

func printUsageTo(out io.Writer) {
	fmt.Fprintf(out, `Usage: httpsync-cli -server <base_url> -cmd <command> [options]

For HTTPS, a new in-memory client certificate is generated automatically for mTLS.

Commands and options:
  -cmd status
      GET /status, output JSON (height, hash, td).

  -cmd headers
      GET /headers. Exactly one of:
        -hash <hex>           single header by hash
        -number <n>            single header by number
        -from <n> -count <n>  range of headers (default count=1)

  -cmd block
      GET /block. One of:
        -hash <hex>   block body by hash
        -number <n>   block body by number

  -cmd blocks
      GET /blocks. One of:
        -hash <hex,hex,...>   bodies by hashes
        -number <n,n,...>    bodies by numbers

Example:
  httpsync-cli -server https://127.0.0.1:30304 -cmd status
  httpsync-cli -server https://127.0.0.1:30304 -cmd headers -from 0 -count 5
`)
}

// newClientMTLS returns a client with a newly generated in-memory PQC client cert and server cert verification.
func newClientMTLS() (*http.Client, error) {
	tlsConfig, err := httpsync.ClientTLSConfigInMemory()
	if err != nil {
		return nil, fmt.Errorf("client TLS: %w", err)
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Timeout: defaultTimeout, Transport: tr}, nil
}

// newClientInsecure returns a client with no client cert and InsecureSkipVerify (for http:// or tests).
func newClientInsecure() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true},
	}
	return &http.Client{Timeout: defaultTimeout, Transport: tr}
}

func doGet(client *http.Client, url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var r io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, resp.StatusCode, err
		}
		defer gz.Close()
		r = gz
	}
	body, err := io.ReadAll(r)
	return body, resp.StatusCode, err
}

func runStatusTo(client *http.Client, baseURL string, w io.Writer) error {
	body, code, err := doGet(client, baseURL+"/status")
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("GET /status: %d %s", code, string(body))
	}
	var s httpsync.StatusResponse
	if err := json.Unmarshal(body, &s); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func runHeadersTo(client *http.Client, baseURL string, w io.Writer, hashStr, numberStr string, from, count uint64) error {
	var url string
	if hashStr != "" {
		url = baseURL + "/headers?hash=" + strings.TrimPrefix(hashStr, "0x")
	} else if numberStr != "" {
		url = baseURL + "/headers?number=" + numberStr
	} else {
		url = baseURL + "/headers?from=" + strconv.FormatUint(from, 10) + "&count=" + strconv.FormatUint(count, 10)
	}
	body, code, err := doGet(client, url)
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		fmt.Fprintln(w, "[]")
		return nil
	}
	if code != http.StatusOK {
		return fmt.Errorf("GET /headers: %d %s", code, string(body))
	}
	var headers []*types.Header
	if err := rlp.DecodeBytes(body, &headers); err != nil {
		return err
	}
	type headerInfo struct {
		Number uint64      `json:"number"`
		Hash   common.Hash `json:"hash"`
	}
	out := make([]headerInfo, len(headers))
	for i, h := range headers {
		out[i] = headerInfo{Number: h.Number.Uint64(), Hash: h.Hash()}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func runBlockTo(client *http.Client, baseURL string, w io.Writer, hashStr, numberStr string) error {
	if hashStr == "" && numberStr == "" {
		return fmt.Errorf("block requires -hash or -number")
	}
	var url string
	if hashStr != "" {
		url = baseURL + "/block?hash=" + strings.TrimPrefix(hashStr, "0x")
	} else {
		url = baseURL + "/block?number=" + numberStr
	}
	body, code, err := doGet(client, url)
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		return fmt.Errorf("block not found")
	}
	if code != http.StatusOK {
		return fmt.Errorf("GET /block: %d %s", code, string(body))
	}
	var blockBody types.Body
	if err := rlp.DecodeBytes(body, &blockBody); err != nil {
		fmt.Fprintln(w, hex.EncodeToString(body))
		return nil
	}
	out := map[string]interface{}{
		"transactions": len(blockBody.Transactions),
		"rlp_bytes":    len(body),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func runBlocksTo(client *http.Client, baseURL string, w io.Writer, hashStr, numberStr string) error {
	if hashStr == "" && numberStr == "" {
		return fmt.Errorf("blocks requires -hash or -number (comma-separated)")
	}
	var url string
	if hashStr != "" {
		url = baseURL + "/blocks?hash=" + strings.ReplaceAll(strings.TrimSpace(hashStr), " ", ",")
	} else {
		url = baseURL + "/blocks?number=" + strings.ReplaceAll(strings.TrimSpace(numberStr), " ", ",")
	}
	body, code, err := doGet(client, url)
	if err != nil {
		return err
	}
	if code == http.StatusNotFound {
		fmt.Fprintln(w, "[]")
		return nil
	}
	if code != http.StatusOK {
		return fmt.Errorf("GET /blocks: %d %s", code, string(body))
	}
	var bodies []rlp.RawValue
	if err := rlp.DecodeBytes(body, &bodies); err != nil {
		return err
	}
	type blockSummary struct {
		Index    int `json:"index"`
		TxCount  int `json:"tx_count"`
		RLPBytes int `json:"rlp_bytes"`
	}
	summaries := make([]blockSummary, len(bodies))
	for i, raw := range bodies {
		var b types.Body
		_ = rlp.DecodeBytes(raw, &b)
		summaries[i] = blockSummary{Index: i, TxCount: len(b.Transactions), RLPBytes: len(raw)}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(summaries)
}
