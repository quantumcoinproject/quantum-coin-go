// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package graphql

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	graphqlgo "github.com/graph-gophers/graphql-go"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/internal/ethapi"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
)

// testBackend is a minimal ethapi.Backend for the graphql resolver tests. The embedded
// interface is nil: any method the tests do not exercise panics rather than silently
// returning a zero value.
type testBackend struct {
	ethapi.Backend

	mu        sync.Mutex
	headers   map[uint64]*types.Header
	poolNonce uint64
	lastCtx   context.Context
	numCalls  int
}

func newTestBackend(n int) *testBackend {
	b := &testBackend{headers: make(map[uint64]*types.Header)}
	var parent common.Hash
	for i := 0; i < n; i++ {
		h := &types.Header{
			Number:     big.NewInt(int64(i)),
			ParentHash: parent,
			Difficulty: big.NewInt(1),
		}
		b.headers[uint64(i)] = h
		parent = h.Hash()
	}
	return b
}

func (b *testBackend) header(nr uint64) *types.Header {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.headers[nr]
}

func (b *testBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	b.mu.Lock()
	b.lastCtx = ctx
	b.numCalls++
	b.mu.Unlock()

	if nr, ok := blockNrOrHash.Number(); ok {
		if nr < 0 {
			return b.header(uint64(len(b.headers) - 1)), nil
		}
		return b.header(uint64(nr)), nil
	}
	hash, _ := blockNrOrHash.Hash()
	return b.HeaderByHash(ctx, hash)
}

func (b *testBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, h := range b.headers {
		if h.Hash() == hash {
			return h, nil
		}
	}
	return nil, nil
}

func (b *testBackend) CurrentBlock() *types.Block {
	return types.NewBlockWithHeader(b.header(uint64(len(b.headers) - 1)))
}

func (b *testBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return nil, common.Hash{}, 0, 0, nil
}

func (b *testBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction { return nil }

func (b *testBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.poolNonce, nil
}

// probeSchema is a tiny schema used to observe the context the handler executes with.
// The production schema cannot be used here: it declares a `baseFeePerGas` field that no
// resolver implements, so ParseSchema rejects it (a pre-existing defect unrelated to
// these fixes).
const probeSchema = `
	schema { query: Query }
	type Query {
		ping: Int!
		nest: Nest!
	}
	type Nest {
		nest: Nest!
		value: Int!
	}
`

type probeResolver struct {
	mu      sync.Mutex
	lastCtx context.Context
}

func (p *probeResolver) Ping(ctx context.Context) int32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastCtx = ctx
	return 1
}

func (p *probeResolver) Nest(ctx context.Context) *probeNest { return &probeNest{} }

type probeNest struct{}

func (n *probeNest) Nest(ctx context.Context) *probeNest { return &probeNest{} }
func (n *probeNest) Value(ctx context.Context) int32     { return 1 }

// TestUBF059_GraphQLQueryTimeout checks that query execution is bounded by a deadline.
// Upstream ee9ff0646 (#26116).
func TestUBF059_GraphQLQueryTimeout(t *testing.T) {
	if queryTimeout != 60*time.Second {
		t.Fatalf("queryTimeout = %v, want 60s", queryTimeout)
	}

	res := new(probeResolver)
	s, err := graphqlgo.ParseSchema(probeSchema, res, graphqlgo.MaxDepth(maxQueryDepth))
	if err != nil {
		t.Fatal(err)
	}
	h := handler{Schema: s}

	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ping}"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	res.mu.Lock()
	ctx := res.lastCtx
	res.mu.Unlock()
	if ctx == nil {
		t.Fatal("resolver was never reached")
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("query context has no deadline; Exec is unbounded")
	}
	if d := time.Until(deadline); d <= 0 || d > queryTimeout+time.Second {
		t.Fatalf("query deadline is %v away, want ~%v", d, queryTimeout)
	}
}

// TestUBF060_BlocksNoGreedyAlloc checks that the `blocks` range query does not preallocate
// the whole requested range and honours context cancellation.
// Upstream 0d772b9f0 (#27873).
func TestUBF060_BlocksNoGreedyAlloc(t *testing.T) {
	backend := newTestBackend(4)
	r := &Resolver{backend}

	// A huge range whose blocks mostly don't exist must not be preallocated, and must
	// stop at the first missing block.
	from, to := Long(0), Long(1<<40)
	ret, err := r.Blocks(context.Background(), struct {
		From *Long
		To   *Long
	}{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 4 {
		t.Fatalf("got %d blocks, want 4", len(ret))
	}

	// A canceled context must abort the loop.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Blocks(ctx, struct {
		From *Long
		To   *Long
	}{From: &from, To: &to}); err == nil {
		t.Fatal("Blocks ignored a canceled context")
	}
}

// TestUBF061_ConcurrentResolversRace exercises the lazily resolved fields of Block and
// Transaction from several goroutines, as graphql-go does. Run with -race.
// Upstream a236e03d0 (#26965).
func TestUBF061_ConcurrentResolversRace(t *testing.T) {
	backend := newTestBackend(8)

	for iter := 0; iter < 20; iter++ {
		num := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(3))
		block := &Block{backend: backend, numberOrHash: &num}
		tx := &Transaction{backend: backend, hash: common.Hash{1}}

		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				block.Number(ctx)
				block.Hash(ctx)
				block.Parent(ctx)
				block.GasLimit(ctx)
				tx.Nonce(ctx)
				tx.Block(ctx)
				tx.Index(ctx)
				tx.Type(ctx)
			}()
		}
		wg.Wait()
	}
}

// TestUBF062_BlocksNilFrom checks that `{blocks {number}}` (no `from`) reports an error
// instead of panicking on a nil dereference. Upstream abe3fca1d (#28416).
func TestUBF062_BlocksNilFrom(t *testing.T) {
	r := &Resolver{newTestBackend(3)}
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("Blocks panicked with a nil 'from': %v", p)
		}
	}()
	ret, err := r.Blocks(context.Background(), struct {
		From *Long
		To   *Long
	}{})
	if err == nil {
		t.Fatalf("want an error, got %v", ret)
	}
	if !strings.Contains(err.Error(), "from block number must be specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestUBF063_TransactionTypeUnknownTx checks that resolving `type` on a transaction the
// node does not know about returns nil rather than panicking.
// Upstream aa36bcd0a (#33184).
func TestUBF063_TransactionTypeUnknownTx(t *testing.T) {
	tx := &Transaction{backend: newTestBackend(3), hash: common.Hash{0xde, 0xad}}
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("Transaction.Type panicked for an unknown tx: %v", p)
		}
	}()
	typ, err := tx.Type(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if typ != nil {
		t.Fatalf("want nil type for an unknown tx, got %v", *typ)
	}
}

// TestUBF064_GraphQLBodyAndDepthLimits checks the request body-size limit and the query
// depth cap. Upstream 1c74f2376 (#32344) + c782197d4 (#35034).
func TestUBF064_GraphQLBodyAndDepthLimits(t *testing.T) {
	t.Run("body-limit", func(t *testing.T) {
		for name, body := range map[string]string{
			"oversized query":         `{"query":"` + strings.Repeat("a", maxRequestBodySize) + `"}`,
			"oversized trailing data": `{"query":"{block{number}}"}` + strings.Repeat(" ", maxRequestBodySize),
		} {
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
			w := httptest.NewRecorder()
			handler{}.ServeHTTP(w, req)
			if w.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("%s: got status %d, want %d", name, w.Code, http.StatusRequestEntityTooLarge)
			}
		}
	})

	t.Run("depth-limit", func(t *testing.T) {
		if maxQueryDepth != 20 {
			t.Fatalf("maxQueryDepth = %d, want 20", maxQueryDepth)
		}
		s, err := graphqlgo.ParseSchema(probeSchema, new(probeResolver), graphqlgo.MaxDepth(maxQueryDepth))
		if err != nil {
			t.Fatal(err)
		}
		nested := func(depth int) string {
			var b strings.Builder
			for i := 0; i < depth; i++ {
				b.WriteString("nest{")
			}
			b.WriteString("value")
			for i := 0; i < depth; i++ {
				b.WriteString("}")
			}
			return "{" + b.String() + "}"
		}
		hasDepthError := func(q string) bool {
			for _, err := range s.Exec(context.Background(), q, "", nil).Errors {
				if err.Rule == "MaxDepthExceeded" {
					return true
				}
			}
			return false
		}
		if !hasDepthError(nested(maxQueryDepth + 5)) {
			t.Fatal("a query deeper than the cap was accepted")
		}
		if hasDepthError(nested(2)) {
			t.Fatal("a shallow query was rejected")
		}
	})
}

// TestUBF065_BlocksStopsAtMissingBlock checks that the range query stops at the first
// missing block instead of returning entries for blocks that do not exist.
// Upstream c0d17bca5 (#24190).
func TestUBF065_BlocksStopsAtMissingBlock(t *testing.T) {
	backend := newTestBackend(3)
	r := &Resolver{backend}
	from, to := Long(0), Long(10)
	ret, err := r.Blocks(context.Background(), struct {
		From *Long
		To   *Long
	}{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 3 {
		t.Fatalf("got %d blocks, want 3 (the chain is 3 blocks long)", len(ret))
	}
	for i, blk := range ret {
		n, err := blk.Number(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if int(n) != i {
			t.Fatalf("block %d has number %d", i, n)
		}
	}
}

// TestUBF066_BlockParentResolution checks that `parent` resolves for a block created from
// a block number alone. Upstream 3ccd6b6db (#24191).
func TestUBF066_BlockParentResolution(t *testing.T) {
	backend := newTestBackend(5)
	num := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(3))
	block := &Block{backend: backend, numberOrHash: &num}

	parent, err := block.Parent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil {
		t.Fatal("parent is nil; the header was never resolved")
	}
	n, err := parent.Number(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("parent number = %d, want 2", n)
	}

	// Genesis has no parent.
	zero := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(0))
	genesis := &Block{backend: backend, numberOrHash: &zero}
	parent, err = genesis.Parent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if parent != nil {
		t.Fatal("genesis reported a parent")
	}
}

// TestUBF067_TransactionCountPending checks that the nonce of a pending account comes from
// the transaction pool. Upstream 862f8e98b (#24443).
func TestUBF067_TransactionCountPending(t *testing.T) {
	backend := newTestBackend(3)
	backend.poolNonce = 42

	pending := &Account{
		backend:       backend,
		address:       common.Address{1},
		blockNrOrHash: rpc.BlockNumberOrHashWithNumber(rpc.PendingBlockNumber),
	}
	got, err := pending.TransactionCount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if uint64(got) != 42 {
		t.Fatalf("pending TransactionCount = %d, want 42 (the pool nonce)", got)
	}
}

// TestUBF068_LongAcceptsFloat64 checks that a Long supplied through a query variable, which
// encoding/json decodes as a float64, is accepted. Upstream 440c9fcf7 (#24864).
func TestUBF068_LongAcceptsFloat64(t *testing.T) {
	var l Long
	if err := l.UnmarshalGraphQL(float64(12345)); err != nil {
		t.Fatalf("float64 rejected: %v", err)
	}
	if l != 12345 {
		t.Fatalf("Long = %d, want 12345", l)
	}
	// The other supported types must still work.
	for _, in := range []interface{}{int32(7), int64(7), "7"} {
		var v Long
		if err := v.UnmarshalGraphQL(in); err != nil {
			t.Fatalf("%T rejected: %v", in, err)
		}
		if v != 7 {
			t.Fatalf("%T decoded to %d", in, v)
		}
	}
	// Unsupported types must still be rejected.
	var v Long
	if err := v.UnmarshalGraphQL(true); err == nil {
		t.Fatal("bool was accepted as a Long")
	}
}
