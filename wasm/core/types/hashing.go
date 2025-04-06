package types

import (
	"bytes"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hashingalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"sync"
)

// hasherPool holds LegacyKeccak256 hashers for rlpHash.
var hasherPool = sync.Pool{
	New: func() interface{} { return hashingalgorithm.NewHashState() },
}

// deriveBufferPool holds temporary encoder buffers for DeriveSha and TX encoding.
var encodeBufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// rlpHash encodes x and hashes the encoded bytes.
func rlpHash(x interface{}) (h common.Hash) {
	buff := new(bytes.Buffer)
	rlp.Encode(buff, x)
	h.SetBytes(crypto.Keccak256(buff.Bytes()))
	return h
}

// prefixedRlpHash writes the prefix into the hasher before rlp-encoding x.
// It's used for typed transactions.
func prefixedRlpHash(prefix byte, x interface{}) (h common.Hash) {
	buff := new(bytes.Buffer)
	buff.Write([]byte{prefix})
	rlp.Encode(buff, x)
	h.SetBytes(crypto.Keccak256(buff.Bytes()))
	return h
}
