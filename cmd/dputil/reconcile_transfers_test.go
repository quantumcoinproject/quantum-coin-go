package main

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	dp "github.com/quantumcoinproject/quantum-coin-go"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/token"
)

// fakeFilterBackend is a bind.ContractBackend that only serves FilterLogs; every
// other method fails. It records each FilterQuery so tests can assert the chunking
// and the topic filter that heisenTransfersFrom sends to the node.
type fakeFilterBackend struct {
	queries []dp.FilterQuery
	logsFor func(q dp.FilterQuery) ([]types.Log, error)
}

var errFakeUnsupported = errors.New("fakeFilterBackend: not supported")

func (b *fakeFilterBackend) CodeAt(context.Context, common.Address, *big.Int) ([]byte, error) {
	return nil, errFakeUnsupported
}
func (b *fakeFilterBackend) CallContract(context.Context, dp.CallMsg, *big.Int) ([]byte, error) {
	return nil, errFakeUnsupported
}
func (b *fakeFilterBackend) HeaderByNumber(context.Context, *big.Int) (*types.Header, error) {
	return nil, errFakeUnsupported
}
func (b *fakeFilterBackend) PendingCodeAt(context.Context, common.Address) ([]byte, error) {
	return nil, errFakeUnsupported
}
func (b *fakeFilterBackend) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	return 0, errFakeUnsupported
}
func (b *fakeFilterBackend) SuggestGasPrice(context.Context) (*big.Int, error) {
	return nil, errFakeUnsupported
}
func (b *fakeFilterBackend) EstimateGas(context.Context, dp.CallMsg) (uint64, error) {
	return 0, errFakeUnsupported
}
func (b *fakeFilterBackend) SendTransaction(context.Context, *types.Transaction) error {
	return errFakeUnsupported
}
func (b *fakeFilterBackend) SubscribeFilterLogs(context.Context, dp.FilterQuery, chan<- types.Log) (dp.Subscription, error) {
	return nil, errFakeUnsupported
}
func (b *fakeFilterBackend) FilterLogs(_ context.Context, q dp.FilterQuery) ([]types.Log, error) {
	b.queries = append(b.queries, q)
	return b.logsFor(q)
}

func testAddr(b byte) common.Address {
	var a common.Address
	for i := range a {
		a[i] = b
	}
	return a
}

// transferLog builds a Heisen Transfer(from, to, value) log as the node would return it.
func transferLog(tokenAddr, from, to common.Address, value *big.Int, block uint64, tx byte) types.Log {
	data := make([]byte, 32)
	value.FillBytes(data)
	return types.Log{
		Address:     tokenAddr,
		Topics:      []common.Hash{common.HexToHash(reconcileTransferTopic), common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())},
		Data:        data,
		BlockNumber: block,
		TxHash:      common.BytesToHash([]byte{tx}),
	}
}

func inRange(q dp.FilterQuery, block uint64) bool {
	return q.FromBlock.Uint64() <= block && block <= q.ToBlock.Uint64()
}

func newTransfersTest(t *testing.T, backend *fakeFilterBackend) (*token.Token, common.Address, common.Address) {
	t.Helper()
	tokenAddr := testAddr(0xaa)
	holder := testAddr(0x11)
	tok, err := token.NewToken(tokenAddr, backend)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	return tok, tokenAddr, holder
}

func zeroRetryDelay(t *testing.T) {
	t.Helper()
	prev := heisenLogRetryDelay
	heisenLogRetryDelay = 0
	t.Cleanup(func() { heisenLogRetryDelay = prev })
}

// TestHeisenTransfersFromChunksAndAggregates: the scan covers [0, head] in
// heisenLogChunk-sized ranges, filters on the token address and from == holder, and
// aggregates per recipient. A transfer submitted by someone other than the holder
// (transferFrom by an approved spender) is indistinguishable at the event level and is
// counted — that is the reason the log filter replaced the read-api transaction list.
func TestHeisenTransfersFromChunksAndAggregates(t *testing.T) {
	backend := &fakeFilterBackend{}
	tok, tokenAddr, holder := newTransfersTest(t, backend)
	recipientA, recipientB := testAddr(0x22), testAddr(0x33)
	head := 2*heisenLogChunk + 20000

	all := []types.Log{
		transferLog(tokenAddr, holder, recipientA, big.NewInt(100), 10, 1),
		transferLog(tokenAddr, holder, recipientB, big.NewInt(50), heisenLogChunk+5, 2),   // spender-initiated in reality; same event
		transferLog(tokenAddr, holder, recipientA, big.NewInt(25), 2*heisenLogChunk+1, 3), // last chunk
	}
	backend.logsFor = func(q dp.FilterQuery) ([]types.Log, error) {
		var out []types.Log
		for _, lg := range all {
			if inRange(q, lg.BlockNumber) {
				out = append(out, lg)
			}
		}
		return out, nil
	}

	sent, count, err := heisenTransfersFrom(tok, holder, head)
	if err != nil {
		t.Fatalf("heisenTransfersFrom: %v", err)
	}
	if count != 3 {
		t.Fatalf("event count = %d, want 3", count)
	}
	if len(sent) != 2 {
		t.Fatalf("recipients = %d, want 2", len(sent))
	}
	a := sent[recipientA.HexLower()]
	if a == nil || a.Total.Int64() != 125 || len(a.TxHashes) != 2 || len(a.Amounts) != 2 {
		t.Fatalf("recipient A aggregate wrong: %+v", a)
	}
	b := sent[recipientB.HexLower()]
	if b == nil || b.Total.Int64() != 50 || len(b.TxHashes) != 1 {
		t.Fatalf("recipient B aggregate wrong: %+v", b)
	}

	// Exactly three contiguous, non-overlapping chunks covering [0, head].
	wantRanges := [][2]uint64{{0, heisenLogChunk - 1}, {heisenLogChunk, 2*heisenLogChunk - 1}, {2 * heisenLogChunk, head}}
	if len(backend.queries) != len(wantRanges) {
		t.Fatalf("queries = %d, want %d", len(backend.queries), len(wantRanges))
	}
	for i, q := range backend.queries {
		if q.FromBlock.Uint64() != wantRanges[i][0] || q.ToBlock.Uint64() != wantRanges[i][1] {
			t.Fatalf("query %d range = [%s,%s], want %v", i, q.FromBlock, q.ToBlock, wantRanges[i])
		}
		if len(q.Addresses) != 1 || !q.Addresses[0].IsEqualTo(tokenAddr) {
			t.Fatalf("query %d addresses = %v, want token %s", i, q.Addresses, tokenAddr.Hex())
		}
		if len(q.Topics) < 2 || len(q.Topics[0]) != 1 || q.Topics[0][0] != common.HexToHash(reconcileTransferTopic) {
			t.Fatalf("query %d topic[0] = %v, want Transfer signature", i, q.Topics)
		}
		if len(q.Topics[1]) != 1 || q.Topics[1][0] != common.BytesToHash(holder.Bytes()) {
			t.Fatalf("query %d topic[1] = %v, want from == holder", i, q.Topics)
		}
		if len(q.Topics) > 2 && len(q.Topics[2]) != 0 {
			t.Fatalf("query %d must not filter on recipient, got %v", i, q.Topics[2])
		}
	}
}

func TestHeisenTransfersFromHeadZeroSingleQuery(t *testing.T) {
	backend := &fakeFilterBackend{logsFor: func(dp.FilterQuery) ([]types.Log, error) { return nil, nil }}
	tok, _, holder := newTransfersTest(t, backend)
	sent, count, err := heisenTransfersFrom(tok, holder, 0)
	if err != nil || count != 0 || len(sent) != 0 {
		t.Fatalf("got sent=%v count=%d err=%v", sent, count, err)
	}
	if len(backend.queries) != 1 || backend.queries[0].FromBlock.Sign() != 0 || backend.queries[0].ToBlock.Sign() != 0 {
		t.Fatalf("expected a single [0,0] query, got %+v", backend.queries)
	}
}

// Negative: a log whose indexed from is not the holder must be an error, never silently
// counted or dropped (the node's filter is the only thing that should select events).
func TestHeisenTransfersFromRejectsForeignSender(t *testing.T) {
	backend := &fakeFilterBackend{}
	tok, tokenAddr, holder := newTransfersTest(t, backend)
	other := testAddr(0x44)
	backend.logsFor = func(dp.FilterQuery) ([]types.Log, error) {
		return []types.Log{transferLog(tokenAddr, other, testAddr(0x22), big.NewInt(7), 5, 9)}, nil
	}
	_, _, err := heisenTransfersFrom(tok, holder, 10)
	if err == nil || !strings.Contains(err.Error(), "filter returned from=") {
		t.Fatalf("expected foreign-sender error, got %v", err)
	}
}

// Negative: an undecodable Transfer event is a hard error. Skipping it would undercount
// AlreadySent and produce a duplicate payout.
func TestHeisenTransfersFromRejectsUndecodableEvent(t *testing.T) {
	backend := &fakeFilterBackend{}
	tok, tokenAddr, holder := newTransfersTest(t, backend)
	zeroRetryDelay(t)
	bad := transferLog(tokenAddr, holder, testAddr(0x22), big.NewInt(7), 5, 9)
	bad.Data = bad.Data[:5] // truncated uint256
	backend.logsFor = func(dp.FilterQuery) ([]types.Log, error) { return []types.Log{bad}, nil }
	_, _, err := heisenTransfersFrom(tok, holder, 10)
	if err == nil {
		t.Fatalf("expected decode error for truncated Transfer data")
	}
}

// Negative: a persistently failing node surfaces as an error after the retry budget;
// it must not be reported as "no transfers".
func TestHeisenTransfersFromPropagatesBackendError(t *testing.T) {
	backend := &fakeFilterBackend{}
	tok, _, holder := newTransfersTest(t, backend)
	zeroRetryDelay(t)
	backend.logsFor = func(dp.FilterQuery) ([]types.Log, error) { return nil, errors.New("node: range too large") }
	sent, count, err := heisenTransfersFrom(tok, holder, 10)
	if err == nil || !strings.Contains(err.Error(), "range too large") {
		t.Fatalf("expected backend error, got sent=%v count=%d err=%v", sent, count, err)
	}
	if len(backend.queries) != 4 {
		t.Fatalf("expected 4 attempts (1 + 3 retries), got %d", len(backend.queries))
	}
}

// A transient failure followed by success is retried and completes.
func TestHeisenTransfersFromRetriesTransientError(t *testing.T) {
	backend := &fakeFilterBackend{}
	tok, tokenAddr, holder := newTransfersTest(t, backend)
	zeroRetryDelay(t)
	calls := 0
	backend.logsFor = func(dp.FilterQuery) ([]types.Log, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("transient")
		}
		return []types.Log{transferLog(tokenAddr, holder, testAddr(0x22), big.NewInt(3), 1, 1)}, nil
	}
	sent, count, err := heisenTransfersFrom(tok, holder, 10)
	if err != nil || count != 1 || sent[testAddr(0x22).HexLower()].Total.Int64() != 3 {
		t.Fatalf("got sent=%v count=%d err=%v", sent, count, err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
