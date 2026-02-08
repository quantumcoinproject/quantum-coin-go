// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package restsync implements the REST sync client (round-robin over peer URLs).
package restsync

import (
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
func NewClientFromProvider(peerURLsFn func() []string, dl Deliverer) *Client {
	return &Client{
		urlsFn: peerURLsFn,
		dl:     dl,
		peerID: RestSyncPeerID,
		client: &http.Client{Timeout: defaultHTTPTimeout},
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
		resp, err := c.client.Get(baseURL + "/status")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		var s StatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
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
			resp, err := c.client.Get(u)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return c.dl.DeliverHeaders(c.peerID, nil)
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
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
			u := baseURL + "/headers?hash=0x" + origin.Hex()
			resp, err := c.client.Get(u)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				return c.dl.DeliverHeaders(c.peerID, nil)
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("status %d", resp.StatusCode)
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
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
					parts[i] = "0x" + h.Hex()
				}
				u := baseURL + "/blocks?hash=" + strings.Join(parts, ",")
				resp, err := c.client.Get(u)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					data, err := io.ReadAll(resp.Body)
					if err != nil {
						return err
					}
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
				u := baseURL + "/block?hash=0x" + hash.Hex()
				resp, err := c.client.Get(u)
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					return fmt.Errorf("status %d for %s", resp.StatusCode, hash.Hex())
				}
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					return err
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
