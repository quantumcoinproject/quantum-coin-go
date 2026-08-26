package core

import (
	"math/big"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/event"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// txPoolNumberedBlockChain is txPoolVBlockChain with a configurable head block
// number, so admission rules that depend on the chain height can be exercised.
type txPoolNumberedBlockChain struct {
	txPoolVBlockChain
	number uint64
}

func (bc *txPoolNumberedBlockChain) CurrentBlock() *types.Block {
	return types.NewBlock(&types.Header{
		Number:   new(big.Int).SetUint64(bc.number),
		GasLimit: bc.gasLimit,
	}, nil, nil, trie.NewStackTrie(nil))
}

func (bc *txPoolNumberedBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return bc.CurrentBlock()
}

// setupNumberedTxPool is setupVTxPool with the chain head at the given block number.
func setupNumberedTxPool(t *testing.T, number uint64) (*TxPool, *signaturealgorithm.PrivateKey) {
	t.Helper()

	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	statedb.AddBalance(addr, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1_000_000)))

	bc := &txPoolNumberedBlockChain{
		txPoolVBlockChain: txPoolVBlockChain{
			statedb:       statedb,
			gasLimit:      300000000,
			chainHeadFeed: new(event.Feed),
		},
		number: number,
	}
	config := DefaultTxPoolConfig
	config.Journal = ""
	return NewTxPool(config, txPoolVTestChainConfig, bc), key
}

func signedDynamicFeeTxWithCaps(t *testing.T, pool *TxPool, key *signaturealgorithm.PrivateKey, feeCap, tipCap *big.Int) *types.Transaction {
	t.Helper()
	to := common.BytesToAddress([]byte{0x11})
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          0,
		GasTipCap:      tipCap,
		GasFeeCap:      feeCap,
		Gas:            params.TxGas,
		To:             &to,
		Value:          big.NewInt(0),
		SigningContext: byte(crypto.SigningContextDefault),
	})
	signed, err := types.SignTx(tx, pool.signer, key)
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}
	return signed
}

// TestTxPoolPreGasTipRejectsFeeCapThatTruncatesToZero: before GasTipStartBlock the pool
// rejects dynamic-fee transactions carrying a non-zero gasFeeCap/gasTipCap. That check
// must look at the whole big.Int, not its low 64 bits: a cap of exactly 2^64 has
// Uint64() == 0 and used to slip through the filter.
func TestTxPoolPreGasTipRejectsFeeCapThatTruncatesToZero(t *testing.T) {
	head := defaults.DefaultConfig.PosConfig.DynamicFeeTxStartBlock
	if defaults.IsGasTipActive(head + 1) {
		t.Skipf("gas tip already active at block %d in this config; pre-activation branch unreachable", head+1)
	}
	pool, key := setupNumberedTxPool(t, head)
	defer pool.Stop()

	// Control: zero caps are the legacy opt-out and must be admitted.
	zero := signedDynamicFeeTxWithCaps(t, pool, key, big.NewInt(0), big.NewInt(0))
	if errs := pool.AddRemotes([]*types.Transaction{zero}); errs[0] != nil {
		t.Fatalf("zero-cap dynamic fee tx must be admitted before gas tip activation, got: %v", errs[0])
	}

	twoPow64 := new(big.Int).Lsh(big.NewInt(1), 64) // Uint64() == 0, Sign() == 1
	for _, tc := range []struct {
		name           string
		feeCap, tipCap *big.Int
	}{
		{"feeCap=2^64", twoPow64, big.NewInt(0)},
		{"tipCap=2^64", big.NewInt(0), twoPow64},
		{"feeCap=3*2^64", new(big.Int).Mul(twoPow64, big.NewInt(3)), big.NewInt(0)},
		{"feeCap=tipCap=2^64", twoPow64, twoPow64},
		{"feeCap=2^128", new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(0)},
		{"feeCap=1", big.NewInt(1), big.NewInt(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := signedDynamicFeeTxWithCaps(t, pool, key, tc.feeCap, tc.tipCap)
			errs := pool.AddRemotes([]*types.Transaction{tx})
			if errs[0] == nil || !strings.Contains(errs[0].Error(), "gasFeeCap or gasTipCap non nil") {
				t.Fatalf("expected rejection for non-zero caps before gas tip activation, got: %v", errs[0])
			}
		})
	}
}
