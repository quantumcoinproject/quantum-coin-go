// Regression tests for the upstream go-ethereum fixes ported into
// internal/ethapi (audit batch H, docs/upstream-bugfix-audit-2026-08.md §4.11).
package ethapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/accounts"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/consensus"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/bloombits"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/eth/downloader"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/event"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
)

// ubfBackend is a hand-rolled Backend for these tests. Only the methods the
// exercised code paths actually touch are implemented; everything else panics so
// that an unexpected call shows up as a failure instead of a silent zero value.
type ubfBackend struct {
	statedb *state.StateDB
	header  *types.Header
	block   *types.Block
	chainDb ethdb.Database

	poolNonce uint64

	// evmErr, when set, is returned by GetEVM. It lets a test stop execution at a
	// known point without standing up a whole EVM.
	evmErr error
	// lastMsg records the message GetEVM was last asked to run.
	lastMsg core.Message

	// GetTransaction / GetReceipts fixtures.
	tx       *types.Transaction
	txIndex  uint64
	receipts types.Receipts
}

func newUBFBackend(t *testing.T) *ubfBackend {
	t.Helper()
	memdb := rawdb.NewMemoryDatabase()
	statedb, err := state.New(common.Hash{}, state.NewDatabase(memdb), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	header := &types.Header{
		Number:     big.NewInt(1),
		Difficulty: big.NewInt(1),
		GasLimit:   8_000_000,
		Root:       common.Hash{},
	}
	return &ubfBackend{
		statedb: statedb,
		header:  header,
		block:   types.NewBlockWithHeader(header),
		chainDb: memdb,
		evmErr:  errors.New("ubf: EVM intentionally unavailable"),
	}
}

func (b *ubfBackend) ChainConfig() *params.ChainConfig { return params.TestChainConfig }
func (b *ubfBackend) ChainDb() ethdb.Database          { return b.chainDb }
func (b *ubfBackend) RPCGasCap() uint64                { return 0 }
func (b *ubfBackend) RPCTxFeeCap() float64             { return 0 }
func (b *ubfBackend) UnprotectedAllowed() bool         { return false }
func (b *ubfBackend) ExtRPCEnabled() bool              { return false }

func (b *ubfBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	return b.statedb, b.header, nil
}

func (b *ubfBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	return b.block, nil
}

func (b *ubfBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.poolNonce, nil
}

func (b *ubfBackend) GetEVM(ctx context.Context, msg core.Message, st *state.StateDB, header *types.Header, vmConfig *vm.Config) (*vm.EVM, func() error, error) {
	b.lastMsg = msg
	return nil, nil, b.evmErr
}

func (b *ubfBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return b.tx, common.Hash{}, 0, b.txIndex, nil
}

func (b *ubfBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.receipts, nil
}

// Everything below is unused by these tests.
func (b *ubfBackend) Downloader() *downloader.Downloader { panic("not implemented") }
func (b *ubfBackend) FeeHistory(ctx context.Context, blockCount int, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (*big.Int, [][]*big.Int, []float64, error) {
	panic("not implemented")
}
func (b *ubfBackend) AccountManager() *accounts.Manager { panic("not implemented") }
func (b *ubfBackend) SetHead(number uint64)             { panic("not implemented") }
func (b *ubfBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	panic("not implemented")
}
func (b *ubfBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	panic("not implemented")
}
func (b *ubfBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	panic("not implemented")
}
func (b *ubfBackend) CurrentHeader() *types.Header { panic("not implemented") }
func (b *ubfBackend) CurrentBlock() *types.Block   { panic("not implemented") }
func (b *ubfBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	panic("not implemented")
}
func (b *ubfBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	panic("not implemented")
}
func (b *ubfBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	panic("not implemented")
}
func (b *ubfBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int { panic("not implemented") }
func (b *ubfBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	panic("not implemented")
}
func (b *ubfBackend) GetPoolTransactions() (types.Transactions, error) { panic("not implemented") }
func (b *ubfBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	panic("not implemented")
}
func (b *ubfBackend) Stats() (int, int) { panic("not implemented") }
func (b *ubfBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	panic("not implemented")
}
func (b *ubfBackend) TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions) {
	panic("not implemented")
}
func (b *ubfBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) BloomStatus() (uint64, uint64) { panic("not implemented") }
func (b *ubfBackend) GetLogs(ctx context.Context, blockHash common.Hash) ([][]*types.Log, error) {
	panic("not implemented")
}
func (b *ubfBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	panic("not implemented")
}
func (b *ubfBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	panic("not implemented")
}
func (b *ubfBackend) Engine() consensus.Engine { panic("not implemented") }

var _ Backend = (*ubfBackend)(nil)

func ubfAddress(b byte) common.Address {
	var a common.Address
	for i := range a {
		a[i] = b
	}
	return a
}

// TestUBF083_AccessListRespectsCancel covers upstream 5e070545: the access-list
// refinement loop re-executes the transaction on every pass, so a client that
// walks away must not leave the node spinning.
func TestUBF083_AccessListRespectsCancel(t *testing.T) {
	b := newUBFBackend(t)
	from, to := ubfAddress(0x11), ubfAddress(0x22)
	nonce := hexutil.Uint64(0)
	gas := hexutil.Uint64(100000)
	args := TransactionArgs{
		From:    &from,
		To:      &to,
		Gas:     &gas,
		Nonce:   &nonce,
		ChainID: (*hexutil.Big)(params.TestChainConfig.ChainID),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _, err := AccessList(ctx, b, rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber), args)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AccessList on a cancelled context returned %v, want context.Canceled", err)
	}
	if b.lastMsg != nil {
		t.Fatal("AccessList executed the transaction even though the context was already cancelled")
	}
}

// TestUBF085_GetProofCodeHashForEmptyStorageContract covers upstream 8b99ad460:
// eth_getProof must report the account's real code hash rather than substituting
// the empty-code hash whenever no storage trie could be opened.
//
// Note on quantum-coin-go: StateDB.StorageTrie returns a (possibly empty) trie
// for every account that exists, so the substitution here only ever fired for an
// account that is absent from the state. That is the case the second subtest
// pins; the first subtest guards the invariant upstream was protecting.
func TestUBF085_GetProofCodeHashForEmptyStorageContract(t *testing.T) {
	b := newUBFBackend(t)
	api := NewPublicBlockChainAPI(b)

	contract := ubfAddress(0x33)
	code := []byte{0x60, 0x00, 0x60, 0x00, 0xf3}
	b.statedb.SetNonce(contract, 1)
	b.statedb.SetCode(contract, code)

	blockNr := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)

	t.Run("contract with code and no storage", func(t *testing.T) {
		res, err := api.GetProof(context.Background(), contract, nil, blockNr)
		if err != nil {
			t.Fatalf("GetProof: %v", err)
		}
		if want := crypto.Keccak256Hash(code); res.CodeHash != want {
			t.Fatalf("codeHash = %x, want %x", res.CodeHash, want)
		}
	})

	t.Run("account not in state", func(t *testing.T) {
		res, err := api.GetProof(context.Background(), ubfAddress(0x44), nil, blockNr)
		if err != nil {
			t.Fatalf("GetProof: %v", err)
		}
		// The code hash must come from the state, not be synthesised. An absent
		// account has no code hash at all, so the zero hash is what is reported.
		if res.CodeHash != (common.Hash{}) {
			t.Fatalf("codeHash for a missing account = %x, want the zero hash (got the synthesised empty-code hash?)", res.CodeHash)
		}
	})
}

// TestUBF087_StructLogErrorMarshalsAsString covers upstream f311488d2: an `error`
// interface has no exported fields, so encoding/json rendered every trace error
// as "{}".
func TestUBF087_StructLogErrorMarshalsAsString(t *testing.T) {
	formatted := FormatLogs([]vm.StructLog{{Err: vm.ErrOutOfGas}})
	if len(formatted) != 1 {
		t.Fatalf("FormatLogs returned %d entries, want 1", len(formatted))
	}
	if formatted[0].Error != vm.ErrOutOfGas.Error() {
		t.Fatalf("Error = %q, want %q", formatted[0].Error, vm.ErrOutOfGas.Error())
	}
	blob, err := json.Marshal(formatted[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(blob), `"error":"`+vm.ErrOutOfGas.Error()+`"`) {
		t.Fatalf("struct log marshalled as %s, want the error rendered as a string", blob)
	}

	// A log without an error must still be omitted entirely.
	clean, err := json.Marshal(FormatLogs([]vm.StructLog{{}})[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(clean), `"error"`) {
		t.Fatalf("struct log without an error marshalled as %s, want no error field", clean)
	}
}

// TestUBF088_EstimateGasBalanceCap covers upstream a879c42bd: the balance cap in
// DoEstimateGas used to key off gasPrice alone, so a call that supplied only
// maxFeePerGas was never checked against the sender's funds.
func TestUBF088_EstimateGasBalanceCap(t *testing.T) {
	from, to := ubfAddress(0x11), ubfAddress(0x22)
	blockNr := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)

	newArgs := func() TransactionArgs {
		return TransactionArgs{
			From:  &from,
			To:    &to,
			Value: (*hexutil.Big)(big.NewInt(5000)), // more than the balance below
		}
	}

	t.Run("maxFeePerGas only", func(t *testing.T) {
		b := newUBFBackend(t)
		b.statedb.SetBalance(from, big.NewInt(1000))
		args := newArgs()
		args.MaxFeePerGas = (*hexutil.Big)(big.NewInt(1))

		_, err := DoEstimateGas(context.Background(), b, args, blockNr, 0)
		if err == nil || !strings.Contains(err.Error(), "insufficient funds for transfer") {
			t.Fatalf("DoEstimateGas = %v, want the balance cap to reject the transfer", err)
		}
	})

	t.Run("gasPrice still capped", func(t *testing.T) {
		b := newUBFBackend(t)
		b.statedb.SetBalance(from, big.NewInt(1000))
		args := newArgs()
		args.GasPrice = (*hexutil.Big)(big.NewInt(1))

		_, err := DoEstimateGas(context.Background(), b, args, blockNr, 0)
		if err == nil || !strings.Contains(err.Error(), "insufficient funds for transfer") {
			t.Fatalf("DoEstimateGas = %v, want the balance cap to reject the transfer", err)
		}
	})

	t.Run("ambiguous fee cap rejected", func(t *testing.T) {
		b := newUBFBackend(t)
		b.statedb.SetBalance(from, big.NewInt(1e9))
		args := newArgs()
		args.GasPrice = (*hexutil.Big)(big.NewInt(1))
		args.MaxFeePerGas = (*hexutil.Big)(big.NewInt(2))

		_, err := DoEstimateGas(context.Background(), b, args, blockNr, 0)
		if err == nil || !strings.Contains(err.Error(), "both gasPrice and") {
			t.Fatalf("DoEstimateGas = %v, want the conflicting fee fields to be rejected", err)
		}
	})
}

// TestUBF089_SetDefaultsUsesInput covers upstream ffae2043f: setDefaults forwarded
// args.Data to the gas estimator, so a caller that used the newer "input" field
// had its gas estimated against empty calldata.
func TestUBF089_SetDefaultsUsesInput(t *testing.T) {
	b := newUBFBackend(t)
	b.statedb.SetBalance(ubfAddress(0x11), big.NewInt(1e18))

	from, to := ubfAddress(0x11), ubfAddress(0x22)
	nonce := hexutil.Uint64(0)
	input := hexutil.Bytes{0xde, 0xad, 0xbe, 0xef}
	args := TransactionArgs{From: &from, To: &to, Nonce: &nonce, Input: &input}

	// The estimation itself cannot complete without an EVM; all we need is the
	// message that was handed to it.
	if err := args.setDefaults(context.Background(), b); err == nil {
		t.Fatal("expected the stub EVM to abort the estimation")
	}
	if b.lastMsg == nil {
		t.Fatal("setDefaults never reached the gas estimator")
	}
	if !bytes.Equal(b.lastMsg.Data(), input) {
		t.Fatalf("gas estimated against data %#x, want %#x", b.lastMsg.Data(), []byte(input))
	}
}

// TestUBF090_ReceiptEmptyLogsType covers upstream f01e2fab0: a receipt with no
// logs reported `[][]*types.Log{}`, which marshals to the right JSON but is the
// wrong Go type for any in-process consumer and misdescribes the field.
func TestUBF090_ReceiptEmptyLogsType(t *testing.T) {
	b := newUBFBackend(t)
	b.tx = ubfDummyTx()
	b.receipts = types.Receipts{{Status: types.ReceiptStatusSuccessful, Logs: nil}}

	api := NewPublicTransactionPoolAPI(b, new(AddrLocker))
	fields, err := api.GetTransactionReceipt(context.Background(), common.Hash{})
	if err != nil {
		t.Fatalf("GetTransactionReceipt: %v", err)
	}
	logs, ok := fields["logs"].([]*types.Log)
	if !ok {
		t.Fatalf("logs field has type %T, want []*types.Log", fields["logs"])
	}
	if len(logs) != 0 {
		t.Fatalf("logs = %v, want an empty slice", logs)
	}
	blob, err := json.Marshal(fields["logs"])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(blob) != "[]" {
		t.Fatalf("logs marshalled as %s, want []", blob)
	}
}

// TestUBF091_RejectsInvalidHexStorageKey covers upstream 8ade5e6c1 and 88132afc3:
// a storage key that is not valid hex, or that is too long, used to be silently
// coerced by common.HexToHash instead of being reported.
func TestUBF091_RejectsInvalidHexStorageKey(t *testing.T) {
	b := newUBFBackend(t)
	api := NewPublicBlockChainAPI(b)
	addr := ubfAddress(0x33)
	slot1 := common.BigToHash(big.NewInt(1))
	want := common.HexToHash("0x2a")
	b.statedb.SetState(addr, slot1, want)

	blockNr := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	ctx := context.Background()

	bad := []string{
		"0xZZ",                          // not hex at all
		"0xdeadbeefzz",                  // trailing garbage
		"nonsense",                      // no prefix, not hex
		"0x" + strings.Repeat("11", 33), // 33 bytes, one too many
	}
	for _, key := range bad {
		if _, err := api.GetStorageAt(ctx, addr, key, blockNr); err == nil {
			t.Errorf("GetStorageAt(%q) succeeded, want an error", key)
		}
		if _, err := api.GetProof(ctx, addr, []string{key}, blockNr); err == nil {
			t.Errorf("GetProof(%q) succeeded, want an error", key)
		}
	}

	// An odd number of nibbles is still accepted, left-padded, per 88132afc3.
	got, err := api.GetStorageAt(ctx, addr, "0x1", blockNr)
	if err != nil {
		t.Fatalf("GetStorageAt(0x1): %v", err)
	}
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("GetStorageAt(0x1) = %#x, want %#x", []byte(got), want[:])
	}
}

// TestUBF092_RejectsChainIDMismatch covers upstream 434ca026c: a chainId that does
// not belong to this node used to be silently replaced with the local one, so the
// node signed for a chain the caller never asked for.
func TestUBF092_RejectsChainIDMismatch(t *testing.T) {
	from, to := ubfAddress(0x11), ubfAddress(0x22)
	nonce := hexutil.Uint64(0)
	gas := hexutil.Uint64(21000)
	newArgs := func() TransactionArgs {
		return TransactionArgs{From: &from, To: &to, Nonce: &nonce, Gas: &gas}
	}

	b := newUBFBackend(t)
	local := params.TestChainConfig.ChainID

	args := newArgs()
	args.ChainID = (*hexutil.Big)(new(big.Int).Add(local, big.NewInt(1)))
	err := args.setDefaults(context.Background(), b)
	if err == nil || !strings.Contains(err.Error(), "chainId does not match") {
		t.Fatalf("setDefaults with a foreign chainId = %v, want a mismatch error", err)
	}

	// The matching and unset cases must still work.
	args = newArgs()
	args.ChainID = (*hexutil.Big)(new(big.Int).Set(local))
	if err := args.setDefaults(context.Background(), b); err != nil {
		t.Fatalf("setDefaults with the local chainId: %v", err)
	}
	args = newArgs()
	if err := args.setDefaults(context.Background(), b); err != nil {
		t.Fatalf("setDefaults without a chainId: %v", err)
	}
	if args.ChainID == nil || (*big.Int)(args.ChainID).Cmp(local) != 0 {
		t.Fatalf("chainId defaulted to %v, want %v", args.ChainID, local)
	}
}

// compactRecorder records the ranges debug_chaindbCompact asks for.
type compactRecorder struct {
	ethdb.Database
	ranges [][2][]byte
}

func (r *compactRecorder) Compact(start, limit []byte) error {
	r.ranges = append(r.ranges, [2][]byte{start, limit})
	return nil
}

// TestUBF097_CompactCoversFullRange covers upstream 46c850a94: the old
// `for b := byte(0); b < 255` loop stopped before the last prefix, leaving
// everything from 0xff upwards uncompacted.
func TestUBF097_CompactCoversFullRange(t *testing.T) {
	rec := &compactRecorder{}
	b := newUBFBackend(t)
	b.chainDb = rec

	if err := NewPrivateDebugAPI(b).ChaindbCompact(); err != nil {
		t.Fatalf("ChaindbCompact: %v", err)
	}
	if len(rec.ranges) != 256 {
		t.Fatalf("compacted %d ranges, want 256", len(rec.ranges))
	}
	for i, r := range rec.ranges {
		if len(r[0]) != 1 || r[0][0] != byte(i) {
			t.Fatalf("range %d starts at %#x, want %#x", i, r[0], []byte{byte(i)})
		}
	}
	// The final range must be open-ended so that keys beyond 0xff are included.
	if last := rec.ranges[255]; last[1] != nil {
		t.Fatalf("the 0xff range ends at %#x, want an open upper bound", last[1])
	}
}

// TestUBF098_ReceiptIndexOverflow covers upstream f73365738: index is a uint64, so
// int(index) wraps negative on a large value and the bounds check is skipped,
// indexing the receipt slice out of range.
func TestUBF098_ReceiptIndexOverflow(t *testing.T) {
	b := newUBFBackend(t)
	b.tx = ubfDummyTx()
	b.receipts = types.Receipts{{Status: types.ReceiptStatusSuccessful}}
	b.txIndex = math.MaxUint64

	fields, err := NewPublicTransactionPoolAPI(b, new(AddrLocker)).GetTransactionReceipt(context.Background(), common.Hash{})
	if err != nil {
		t.Fatalf("GetTransactionReceipt: %v", err)
	}
	if fields != nil {
		t.Fatalf("GetTransactionReceipt returned %v for an out-of-range index, want nil", fields)
	}
}

// TestUBF099_ChainIdAlwaysReturns covers upstream 647c6f2db: eth_chainId used to
// consult the head block and fail with a spurious "not synced" error. The backend
// used here panics from CurrentBlock, so the head must not be touched at all.
func TestUBF099_ChainIdAlwaysReturns(t *testing.T) {
	api := NewPublicBlockChainAPI(newUBFBackend(t))
	got := api.ChainId()
	if got == nil || (*big.Int)(got).Cmp(params.TestChainConfig.ChainID) != 0 {
		t.Fatalf("ChainId = %v, want %v", got, params.TestChainConfig.ChainID)
	}
}

func ubfDummyTx() *types.Transaction {
	to := ubfAddress(0x22)
	return types.NewTx(&types.DefaultFeeTx{
		To: &to,
		// Deliberately not this chain's id: the receipt path ignores the sender
		// recovery error, and this keeps the unsigned transaction out of the
		// signature-recovery code entirely.
		ChainID:    big.NewInt(999999),
		Nonce:      0,
		Gas:        21000,
		MaxGasTier: types.GAS_TIER_DEFAULT,
		Value:      big.NewInt(0),
	})
}
