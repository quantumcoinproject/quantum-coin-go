// Copyright 2024 The go-ethereum Authors
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

package core

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/event"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// txPoolVTestChainConfig pins ChainID to the default chain id used by the
// transaction types so the pool signer matches signed transactions, with all
// forks active from genesis.
var txPoolVTestChainConfig = &params.ChainConfig{
	ChainID:             big.NewInt(types.DEFAULT_CHAIN_ID),
	HomesteadBlock:      big.NewInt(0),
	EIP150Block:         big.NewInt(0),
	EIP155Block:         big.NewInt(0),
	EIP158Block:         big.NewInt(0),
	ByzantiumBlock:      big.NewInt(0),
	ConstantinopleBlock: big.NewInt(0),
	PetersburgBlock:     big.NewInt(0),
	IstanbulBlock:       big.NewInt(0),
	BerlinBlock:         big.NewInt(0),
	LondonBlock:         big.NewInt(0),
}

// txPoolVBlockChain is a minimal blockChain implementation backed by an
// in-memory state, sufficient for exercising TxPool admission.
type txPoolVBlockChain struct {
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

func (bc *txPoolVBlockChain) CurrentBlock() *types.Block {
	return types.NewBlock(&types.Header{
		Number:   big.NewInt(0),
		GasLimit: bc.gasLimit,
	}, nil, nil, trie.NewStackTrie(nil))
}

func (bc *txPoolVBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return bc.CurrentBlock()
}

func (bc *txPoolVBlockChain) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *txPoolVBlockChain) SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

// setupVTxPool builds a TxPool with a single funded sender account and returns
// the pool together with the sender's private key.
func setupVTxPool(t *testing.T) (*TxPool, *signaturealgorithm.PrivateKey) {
	t.Helper()

	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)

	// Fund the sender generously so balance/gas checks are not the limiting factor.
	statedb.AddBalance(addr, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1_000_000)))

	bc := &txPoolVBlockChain{
		statedb:       statedb,
		gasLimit:      300000000,
		chainHeadFeed: new(event.Feed),
	}

	config := DefaultTxPoolConfig
	config.Journal = "" // avoid touching disk

	pool := NewTxPool(config, txPoolVTestChainConfig, bc)
	return pool, key
}

// signedDefaultFeeTx returns a valid, signed DefaultFeeTx (V == 1) from key.
func signedDefaultFeeTx(t *testing.T, pool *TxPool, key *signaturealgorithm.PrivateKey) *types.Transaction {
	t.Helper()

	to := common.BytesToAddress([]byte{0x11})
	tx := types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      0,
		To:         &to,
		Value:      big.NewInt(0),
		Gas:        params.TxGas,
		MaxGasTier: types.GAS_TIER_DEFAULT,
	})

	signed, err := types.SignTx(tx, pool.signer, key)
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}
	return signed
}

// TestTxPoolAddRemoteAcceptsValidV is the positive case: a properly signed
// transaction (V == 1) is accepted by the pool admission path.
func TestTxPoolAddRemoteAcceptsValidV(t *testing.T) {
	pool, key := setupVTxPool(t)
	defer pool.Stop()

	tx := signedDefaultFeeTx(t, pool, key)

	if err := pool.addRemoteSync(tx); err != nil {
		t.Fatalf("expected valid (V==1) tx to be accepted, got error: %v", err)
	}
	if pool.Get(tx.Hash()) == nil {
		t.Fatalf("expected accepted tx to be present in the pool")
	}
}

// TestTxPoolAddRemoteRejectsMalleatedV is the negative case: a transaction that
// is identical to a valid one except its signature V value has been malleated
// to 2 must be rejected at admission. Enforcement now happens both in base
// Sender (recoverPlain rejects V != 1) and in SenderV2. This guards against a
// V-malleated variant entering the mempool only to fail later at block Finalize
// (which enforces v == 1).
func TestTxPoolAddRemoteRejectsMalleatedV(t *testing.T) {
	pool, key := setupVTxPool(t)
	defer pool.Stop()

	valid := signedDefaultFeeTx(t, pool, key)
	_, r, s := valid.RawSignatureValues()

	// Rebuild the same transaction but with a malleated V (2 instead of 1),
	// keeping the otherwise-valid R (public key) and S (signature).
	bad := types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      valid.Nonce(),
		To:         valid.To(),
		Value:      valid.Value(),
		Gas:        valid.Gas(),
		MaxGasTier: types.GAS_TIER_DEFAULT,
		V:          big.NewInt(2),
		R:          r,
		S:          s,
	})

	// recoverPlain now enforces V == 1, so base Sender itself rejects the
	// malleated-V tx (it no longer relies solely on SenderV2). The R (public
	// key) and S (signature) are otherwise valid, so the only reason for
	// rejection is the strict V check.
	if _, err := types.Sender(pool.signer, bad); err != types.ErrInvalidSig {
		t.Fatalf("expected base Sender to reject malleated-V tx with ErrInvalidSig, got: %v", err)
	}

	if err := pool.addRemoteSync(bad); err != ErrInvalidSender {
		t.Fatalf("expected ErrInvalidSender for malleated V==2 tx, got: %v", err)
	}
	if pool.Get(bad.Hash()) != nil {
		t.Fatalf("malleated-V tx must not be present in the pool")
	}
}

// TestTxPoolAddRemoteRejectsTruncatedV guards the big.Int truncation gap: a V of
// 2^64+1 has low 64 bits equal to 1, so a v.Uint64()==1 check would wrongly accept
// it. Both base Sender and SenderV2 now use exact big.Int comparison and must
// reject it, and the pool must not admit it.
func TestTxPoolAddRemoteRejectsTruncatedV(t *testing.T) {
	pool, key := setupVTxPool(t)
	defer pool.Stop()

	valid := signedDefaultFeeTx(t, pool, key)
	_, r, s := valid.RawSignatureValues()

	// V = 2^64 + 1: congruent to 1 mod 2^64, so v.Uint64() == 1.
	truncatedV := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))

	bad := types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      valid.Nonce(),
		To:         valid.To(),
		Value:      valid.Value(),
		Gas:        valid.Gas(),
		MaxGasTier: types.GAS_TIER_DEFAULT,
		V:          truncatedV,
		R:          r,
		S:          s,
	})

	if _, err := types.Sender(pool.signer, bad); err != types.ErrInvalidSig {
		t.Fatalf("expected base Sender to reject V=2^64+1 with ErrInvalidSig, got: %v", err)
	}
	if _, err := types.SenderV2(pool.signer, bad); err != types.ErrInvalidSig {
		t.Fatalf("expected SenderV2 to reject V=2^64+1 with ErrInvalidSig, got: %v", err)
	}
	if err := pool.addRemoteSync(bad); err != ErrInvalidSender {
		t.Fatalf("expected ErrInvalidSender for V=2^64+1 tx, got: %v", err)
	}
	if pool.Get(bad.Hash()) != nil {
		t.Fatalf("V=2^64+1 tx must not be present in the pool")
	}
}
