// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package restsync implements the REST sync client (round-robin over peer URLs).
package restsync

import (
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/eth/downloader"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

const (
	// RestSyncPeerID is the logical peer id used when registering the REST sync peer.
	RestSyncPeerID = "restsync"
	// Default HTTP timeout for REST requests.
	defaultHTTPTimeout = 30 * time.Second
)

// Deliverer is the interface used to deliver headers and bodies to the downloader.
type Deliverer interface {
	DeliverHeaders(id string, headers []*types.Header) error
	DeliverBodies(id string, transactions [][]*types.Transaction) error
}

// Client implements downloader.Peer by round-robin HTTP GETs to REST peer URLs.
// Peer URLs are obtained from the provider (e.g. derived from P2P peers at 30304).
type Client struct {
	urlsFn  func() []string
	index   uint32
	dl      Deliverer
	peerID  string
	client  *http.Client
	log     log.Logger
}

// NewClientFromProvider creates a REST sync client that round-robins over base URLs
// returned by the provider (e.g. from connected P2P peers, assuming each has REST on 30304).
// Uses HTTPS with TLS 1.3 and accepts self-signed certs; requests gzip when supported.
func NewClientFromProvider(peerURLsFn func() []string, dl Deliverer) *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true},
	}
	return &Client{
		urlsFn: peerURLsFn,
		dl:     dl,
		peerID: RestSyncPeerID,
		client: &http.Client{Timeout: defaultHTTPTimeout, Transport: tr},
		log:    log.New("restsync", "client"),
	}
}

// urls returns the current list of peer URLs (may be empty if no peers).
func (c *Client) urls() []string {
	if c.urlsFn == nil {
		return nil
	}
	return c.urlsFn()
}

// nextURL returns the next base URL in round-robin order, or "" if no peers.
func (c *Client) nextURL() string {
	urls := c.urls()
	if len(urls) == 0 {
		return ""
	}
	i := atomic.AddUint32(&c.index, 1) - 1
	return urls[i%uint32(len(urls))]
}

// readBody reads the response body and decompresses it if Content-Encoding is gzip.
func readBody(resp *http.Response) ([]byte, error) {
	var r io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return io.ReadAll(r)
}

// doGet performs a GET request with Accept-Encoding: gzip and returns the decompressed body and status code.
func (c *Client) doGet(url string) (body []byte, statusCode int, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err = readBody(resp)
	return body, resp.StatusCode, err
}

// tryAllURLs runs fn for each URL in round-robin order until one succeeds.
func (c *Client) tryAllURLs(fn func(baseURL string) error) error {
	urls := c.urls()
	if len(urls) == 0 {
		c.log.Info("REST sync no peer URLs available", "reason", "no P2P peers with REST on 30304")
		return fmt.Errorf("restsync: no peer URLs (no P2P peers with REST on 30304)")
	}
	c.log.Info("REST sync trying peers", "count", len(urls))
	for i := 0; i < len(urls); i++ {
		baseURL := c.nextURL()
		if baseURL == "" {
			continue
		}
		err := fn(baseURL)
		if err == nil {
			return nil
		}
		c.log.Info("REST sync request failed, trying next peer", "remotepeer", baseURL, "attempt", i+1, "total", len(urls), "err", err)
	}
	c.log.Info("REST sync all peers failed", "count", len(urls))
	return fmt.Errorf("restsync: all %d peers failed", len(urls))
}

// Head returns the chain head hash and total difficulty from the next peer.
func (c *Client) Head() (common.Hash, *big.Int) {
	var hash common.Hash
	var td *big.Int
	err := c.tryAllURLs(func(baseURL string) error {
		body, statusCode, err := c.doGet(baseURL + "/status")
		if err != nil {
			return err
		}
		if statusCode != http.StatusOK {
			return fmt.Errorf("status %d", statusCode)
		}
		var s StatusResponse
		if err := json.Unmarshal(body, &s); err != nil {
			return err
		}
		hash = s.Hash
		td = new(big.Int)
		if _, ok := td.SetString(s.TD, 0); !ok {
			td = common.Big0
		}
		c.log.Info("REST sync Head ok", "hash", hash, "td", td, "remotepeer", baseURL)
		return nil
	})
	if err != nil {
		c.log.Info("REST sync Head failed", "err", err)
		return common.Hash{}, common.Big0
	}
	return hash, td
}

// RequestHeadersByNumber fetches headers by block number (ignore skip/reverse) and delivers them.
func (c *Client) RequestHeadersByNumber(from uint64, amount int, skip int, reverse bool) error {
	c.log.Info("REST sync RequestHeadersByNumber", "from", from, "amount", amount)
	go func() {
		_ = c.tryAllURLs(func(baseURL string) error {
			u := baseURL + "/headers?from=" + strconv.FormatUint(from, 10) + "&count=" + strconv.Itoa(amount)
			data, statusCode, err := c.doGet(u)
			if err != nil {
				return err
			}
			if statusCode == http.StatusNotFound {
				return c.dl.DeliverHeaders(c.peerID, nil)
			}
			if statusCode != http.StatusOK {
				return fmt.Errorf("status %d", statusCode)
			}
			var headers []*types.Header
			if err := rlp.DecodeBytes(data, &headers); err != nil {
				return err
			}
			c.log.Info("REST sync delivered headers by number", "from", from, "count", len(headers), "remotepeer", baseURL)
			return c.dl.DeliverHeaders(c.peerID, headers)
		})
	}()
	return nil
}

// RequestHeadersByHash fetches a single header by hash (amount must be 1 for REST) and delivers it.
func (c *Client) RequestHeadersByHash(origin common.Hash, amount int, skip int, reverse bool) error {
	c.log.Info("REST sync RequestHeadersByHash", "hash", origin)
	go func() {
		_ = c.tryAllURLs(func(baseURL string) error {
			u := baseURL + "/headers?hash=" + origin.Hex()
			data, statusCode, err := c.doGet(u)
			if err != nil {
				return err
			}
			if statusCode == http.StatusNotFound {
				return c.dl.DeliverHeaders(c.peerID, nil)
			}
			if statusCode != http.StatusOK {
				return fmt.Errorf("status %d", statusCode)
			}
			var headers []*types.Header
			if err := rlp.DecodeBytes(data, &headers); err != nil {
				return err
			}
			c.log.Info("REST sync delivered header by hash", "hash", origin, "count", len(headers), "remotepeer", baseURL)
			return c.dl.DeliverHeaders(c.peerID, headers)
		})
	}()
	return nil
}

// RequestBodies fetches block bodies by hash (round-robin, optionally batched) and delivers them.
func (c *Client) RequestBodies(hashes []common.Hash) error {
	c.log.Info("REST sync RequestBodies", "count", len(hashes))
	go func() {
		_ = c.tryAllURLs(func(baseURL string) error {
			if len(hashes) == 0 {
				return c.dl.DeliverBodies(c.peerID, nil)
			}
			// Prefer batch endpoint if multiple hashes
			if len(hashes) > 1 {
				parts := make([]string, len(hashes))
				for i, h := range hashes {
					parts[i] = h.Hex()
				}
				u := baseURL + "/blocks?hash=" + strings.Join(parts, ",")
				data, statusCode, err := c.doGet(u)
				if err != nil {
					return err
				}
				if statusCode == http.StatusOK {
					var bodies []rlp.RawValue
					if err := rlp.DecodeBytes(data, &bodies); err != nil {
						return err
					}
					txs := make([][]*types.Transaction, len(bodies))
					for i, raw := range bodies {
						var body types.Body
						if err := rlp.DecodeBytes(raw, &body); err != nil {
							return err
						}
						txs[i] = body.Transactions
					}
					c.log.Info("REST sync delivered bodies (batch)", "count", len(txs), "remotepeer", baseURL)
					return c.dl.DeliverBodies(c.peerID, txs)
				}
			}
			// Single or fallback: request one by one
			txs := make([][]*types.Transaction, 0, len(hashes))
			for _, hash := range hashes {
				u := baseURL + "/block?hash=" + hash.Hex()
				data, statusCode, err := c.doGet(u)
				if err != nil {
					return err
				}
				if statusCode != http.StatusOK {
					return fmt.Errorf("status %d for %s", statusCode, hash.Hex())
				}
				var body types.Body
				if err := rlp.DecodeBytes(data, &body); err != nil {
					return err
				}
				txs = append(txs, body.Transactions)
			}
			c.log.Info("REST sync delivered bodies (single)", "count", len(txs), "remotepeer", baseURL)
			return c.dl.DeliverBodies(c.peerID, txs)
		})
	}()
	return nil
}

// RequestReceipts is not supported by REST sync (returns nil without error).
func (c *Client) RequestReceipts(hashes []common.Hash) error {
	return nil
}

// RequestNodeData is not supported by REST sync (returns nil without error).
func (c *Client) RequestNodeData(hashes []common.Hash) error {
	return nil
}

// Ensure Client implements downloader.Peer at compile time.
var _ downloader.Peer = (*Client)(nil)
