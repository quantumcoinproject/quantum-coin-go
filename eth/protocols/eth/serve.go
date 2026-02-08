// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software; you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation version 3.
//
// This package provides shared chain lookup helpers for serving block headers
// and bodies by number or hash, used by both the P2P protocol and the REST sync server.

package eth

import (
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// HeadersByNumber returns up to `count` consecutive block headers starting at block number `from`.
// It respects softResponseLimit and maxHeadersServe.
func HeadersByNumber(chain *core.BlockChain, from, count uint64) []*types.Header {
	if count == 0 || count > maxHeadersServe {
		count = maxHeadersServe
	}
	var (
		bytes   common.StorageSize
		headers []*types.Header
	)
	for n := from; uint64(len(headers)) < count && bytes < softResponseLimit; n++ {
		h := chain.GetHeaderByNumber(n)
		if h == nil {
			break
		}
		headers = append(headers, h)
		bytes += estHeaderSize
	}
	return headers
}

// BodyByNumber returns the RLP-encoded block body for the given block number, or nil if not found.
func BodyByNumber(chain *core.BlockChain, number uint64) rlp.RawValue {
	header := chain.GetHeaderByNumber(number)
	if header == nil {
		return nil
	}
	return chain.GetBodyRLP(header.Hash())
}

// BodyByHash returns the RLP-encoded block body for the given block hash, or nil if not found.
func BodyByHash(chain *core.BlockChain, hash common.Hash) rlp.RawValue {
	return chain.GetBodyRLP(hash)
}

// BodiesByHashes returns RLP-encoded block bodies for the given hashes, up to maxBodiesServe
// and softResponseLimit.
func BodiesByHashes(chain *core.BlockChain, hashes []common.Hash) []rlp.RawValue {
	var (
		bytes int
		out   []rlp.RawValue
	)
	for _, hash := range hashes {
		if bytes >= int(softResponseLimit) || len(out) >= maxBodiesServe {
			break
		}
		data := chain.GetBodyRLP(hash)
		if len(data) != 0 {
			out = append(out, data)
			bytes += len(data)
		}
	}
	return out
}
