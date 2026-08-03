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
	"github.com/quantumcoinproject/quantum-coin-go/consensus"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// These tests exercise the two-pass (gas-tip) executor in ProcessTransactions directly, without a
// full consensus engine. They use:
//   - real default-config fork blocks: a post-fork block (>= GasTipStartBlock) for the two-pass
//     path and a pre-fork block (>= GasV2StartBlock but < GasTipStartBlock) for the legacy path;
//   - a stub engine that returns a chosen (small) gas limit so the 50/50 pools can be filled with a
//     handful of 21000-gas transactions;
//   - 21000-gas transactions only: EOA recipients are basic transfers, recipients carrying a STOP
//     (0x00) contract are non-basic but still consume exactly 21000 gas, keeping pool math exact.

const spGas = params.TxGas // 21000

// spDynBaseFee is the fixed base fee of a default-signing-context DynamicFeeTx.
func spDynBaseFee() *big.Int { return big.NewInt(defaults.DEFAULT_PRICE / 10) }

// spTestEngine is a minimal consensus.Engine that only answers GetGasLimit/Author; every other
// method is left nil via embedding and must not be called by the executor.
type spTestEngine struct {
	consensus.Engine
	gasLimit uint64
}

func (e *spTestEngine) GetGasLimit(header *types.Header, statedb *state.StateDB) (uint64, error) {
	return e.gasLimit, nil
}
func (e *spTestEngine) Author(header *types.Header) (common.Address, error) {
	return common.ZERO_ADDRESS, nil
}

// spTestChain is a minimal ChainContext wrapping the stub engine.
type spTestChain struct {
	engine consensus.Engine
}

func (c *spTestChain) Engine() consensus.Engine                          { return c.engine }
func (c *spTestChain) GetHeader(common.Hash, uint64) *types.Header        { return nil }

func spPostForkBlock() uint64 { return defaults.DefaultConfig.PosConfig.GasTipStartBlock }
func spPreForkBlock() uint64  { return defaults.DefaultConfig.PosConfig.GasV2StartBlock }

func spSigner(blockNum uint64) types.Signer {
	return types.MakeSigner(txPoolVTestChainConfig, new(big.Int).SetUint64(blockNum))
}

func spNewAccount(t *testing.T) (*signaturealgorithm.PrivateKey, common.Address) {
	t.Helper()
	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key, cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
}

// spSignDyn builds and signs a 21000-gas DynamicFeeTx (default signing context) whose effective tip
// equals tip (gasFeeCap = base + tip).
func spSignDyn(t *testing.T, signer types.Signer, key *signaturealgorithm.PrivateKey, nonce uint64, to common.Address, tip int64) *types.Transaction {
	t.Helper()
	tipBig := big.NewInt(tip)
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:        big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:          nonce,
		GasTipCap:      tipBig,
		GasFeeCap:      new(big.Int).Add(spDynBaseFee(), tipBig),
		Gas:            spGas,
		To:             &to,
		Value:          big.NewInt(0),
		SigningContext: byte(crypto.SigningContextDefault),
	})
	signed, err := types.SignTx(tx, signer, key)
	if err != nil {
		t.Fatalf("failed to sign dynamic tx: %v", err)
	}
	return signed
}

// spSignDef builds and signs a 21000-gas DefaultFeeTx (always zero effective tip).
func spSignDef(t *testing.T, signer types.Signer, key *signaturealgorithm.PrivateKey, nonce uint64, to common.Address) *types.Transaction {
	t.Helper()
	tx := types.NewTx(&types.DefaultFeeTx{
		ChainID:    big.NewInt(types.DEFAULT_CHAIN_ID),
		Nonce:      nonce,
		Gas:        spGas,
		MaxGasTier: types.GAS_TIER_DEFAULT,
		To:         &to,
		Value:      big.NewInt(0),
	})
	signed, err := types.SignTx(tx, signer, key)
	if err != nil {
		t.Fatalf("failed to sign default tx: %v", err)
	}
	return signed
}

func spNewState(t *testing.T) *state.StateDB {
	t.Helper()
	statedb, err := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil, nil)
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}
	return statedb
}

// spFund funds an account so balance is never the limiting factor.
func spFund(statedb *state.StateDB, addr common.Address) {
	statedb.AddBalance(addr, new(big.Int).Mul(big.NewInt(params.Ether), big.NewInt(1_000_000)))
}

// spStopContract returns a fresh address seeded with STOP (0x00) code in statedb. A 21000-gas
// transfer to it is non-basic (recipient has code) yet still consumes exactly 21000 gas.
func spStopContract(statedb *state.StateDB, b byte) common.Address {
	addr := common.BytesToAddress([]byte{0xC0, b})
	statedb.SetCode(addr, []byte{0x00})
	return addr
}

// spRun executes txs through ProcessTransactions at blockNum with the given gas limit and claimed
// header.GasUsed, returning the passed/error sets and any fatal error.
func spRun(t *testing.T, statedb *state.StateDB, blockNum, gasLimit, headerGasUsed uint64, txs []*types.Transaction) (types.Transactions, types.Transactions, error) {
	t.Helper()
	engine := &spTestEngine{gasLimit: gasLimit}
	bc := &spTestChain{engine: engine}
	header := &types.Header{
		Number:     new(big.Int).SetUint64(blockNum),
		Difficulty: new(big.Int).SetUint64(blockNum),
		GasLimit:   gasLimit,
		GasUsed:    headerGasUsed,
		Time:       1,
	}
	signer := spSigner(blockNum)
	gp := new(GasPool).AddGas(gasLimit)
	usedGas := new(uint64)
	txList := types.Transactions(txs)
	_, _, passed, errored, err := ProcessTransactions(txPoolVTestChainConfig, bc, gp, statedb, header,
		&txList, usedGas, vm.Config{}, &signer, ProcessModeInsertChainNoReturnOnError)
	return passed, errored, err
}

// TestProcessTransactionsBasicOnlyFit: basic transfers that fit entirely within the basic pool all
// pass and header.GasUsed matches.
func TestProcessTransactionsBasicOnlyFit(t *testing.T) {
	statedb := spNewState(t)
	signer := spSigner(spPostForkBlock())
	recipient := common.BytesToAddress([]byte{0x11})

	var txs []*types.Transaction
	for i := 0; i < 2; i++ {
		key, from := spNewAccount(t)
		spFund(statedb, from)
		txs = append(txs, spSignDyn(t, signer, key, 0, recipient, int64(100+i)))
	}

	// gl 84000 -> basic 42000 (2 txns), general 42000.
	passed, errored, err := spRun(t, statedb, spPostForkBlock(), 84000, 2*spGas, txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passed) != 2 || len(errored) != 0 {
		t.Fatalf("passed=%d errored=%d, want 2 and 0", len(passed), len(errored))
	}
}

// TestProcessTransactionsBasicOverflowToGeneral: basic volume beyond the basic pool spills into the
// general pool; once both pools are full the excess basic txn lands in errorTransactions.
func TestProcessTransactionsBasicOverflowToGeneral(t *testing.T) {
	statedb := spNewState(t)
	signer := spSigner(spPostForkBlock())
	recipient := common.BytesToAddress([]byte{0x11})

	var txs []*types.Transaction
	for i := 0; i < 5; i++ {
		key, from := spNewAccount(t)
		spFund(statedb, from)
		txs = append(txs, spSignDyn(t, signer, key, 0, recipient, int64(100+i)))
	}

	// gl 84000 -> basic 42000 (2), general 42000 (2) => capacity 4, one basic txn dropped.
	passed, errored, err := spRun(t, statedb, spPostForkBlock(), 84000, 4*spGas, txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passed) != 4 || len(errored) != 1 {
		t.Fatalf("passed=%d errored=%d, want 4 and 1", len(passed), len(errored))
	}
}

// TestProcessTransactionsGeneralIsolation: non-basic txns use only the general pool; the idle basic
// pool is never lent to them.
func TestProcessTransactionsGeneralIsolation(t *testing.T) {
	statedb := spNewState(t)
	signer := spSigner(spPostForkBlock())
	contract := spStopContract(statedb, 0x01)

	var txs []*types.Transaction
	for i := 0; i < 3; i++ {
		key, from := spNewAccount(t)
		spFund(statedb, from)
		txs = append(txs, spSignDyn(t, signer, key, 0, contract, int64(100+i)))
	}

	// gl 84000 -> basic 42000 (idle), general 42000 (2). Only 2 of 3 non-basic txns fit.
	passed, errored, err := spRun(t, statedb, spPostForkBlock(), 84000, 2*spGas, txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passed) != 2 || len(errored) != 1 {
		t.Fatalf("passed=%d errored=%d, want 2 and 1 (general pool only)", len(passed), len(errored))
	}
}

// TestProcessTransactionsHeaderGasUsedMismatch: the executor rejects a block whose claimed
// header.GasUsed does not equal basicUsed + generalUsed.
//
// Note: the sibling guards (basicUsed > basicBudget, generalUsed > generalBudget) are defensive and
// cannot be reached through ApplyTransaction, because gpBasic/gpGeneral are sized to exactly those
// budgets and SubGas rejects any transaction that would exceed them. They are therefore not
// unit-testable without bypassing the gas pools.
func TestProcessTransactionsHeaderGasUsedMismatch(t *testing.T) {
	signer := spSigner(spPostForkBlock())
	recipient := common.BytesToAddress([]byte{0x11})

	build := func() (*state.StateDB, []*types.Transaction) {
		statedb := spNewState(t)
		var txs []*types.Transaction
		for i := 0; i < 2; i++ {
			key, from := spNewAccount(t)
			spFund(statedb, from)
			txs = append(txs, spSignDyn(t, signer, key, 0, recipient, int64(100+i)))
		}
		return statedb, txs
	}

	// Too high.
	statedb, txs := build()
	if _, _, err := spRun(t, statedb, spPostForkBlock(), 84000, 2*spGas+1, txs); err == nil {
		t.Fatalf("expected error for overstated header.GasUsed")
	}
	// Too low.
	statedb, txs = build()
	if _, _, err := spRun(t, statedb, spPostForkBlock(), 84000, spGas, txs); err == nil {
		t.Fatalf("expected error for understated header.GasUsed")
	}
}

// TestProcessTransactionsBackwardCompatSinglePass: before GasTipStartBlock the legacy single-pass
// path runs against one full pool, so transactions that the 50/50 split would reject still execute.
func TestProcessTransactionsBackwardCompatSinglePass(t *testing.T) {
	statedb := spNewState(t)
	blockNum := spPreForkBlock() // >= GasV2StartBlock but < GasTipStartBlock
	if !(blockNum >= defaults.DefaultConfig.PosConfig.GasV2StartBlock && blockNum < defaults.DefaultConfig.PosConfig.GasTipStartBlock) {
		t.Fatalf("pre-fork block %d not in [GasV2, GasTip) range", blockNum)
	}
	signer := spSigner(blockNum)
	recipient := common.BytesToAddress([]byte{0x11})

	var txs []*types.Transaction
	for i := 0; i < 3; i++ {
		key, from := spNewAccount(t)
		spFund(statedb, from)
		txs = append(txs, spSignDyn(t, signer, key, 0, recipient, int64(100+i)))
	}

	// gl 63000: a two-pass split would be basic 31500 (1 txn) + general 31500 (1 txn) = capacity 2,
	// dropping one txn. The legacy single pool fits all three.
	passed, errored, err := spRun(t, statedb, blockNum, 63000, 3*spGas, txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passed) != 3 || len(errored) != 0 {
		t.Fatalf("passed=%d errored=%d, want 3 and 0 (single-pass)", len(passed), len(errored))
	}
}

// TestProcessTransactionsMixedFeeCharging: a DefaultFeeTx is charged the base fee only, while a
// tipped DynamicFeeTx is charged baseFee + effectiveTip; both execute in the same block.
func TestProcessTransactionsMixedFeeCharging(t *testing.T) {
	statedb := spNewState(t)
	signer := spSigner(spPostForkBlock())
	recipient := common.BytesToAddress([]byte{0x11})

	defKey, defFrom := spNewAccount(t)
	dynKey, dynFrom := spNewAccount(t)
	spFund(statedb, defFrom)
	spFund(statedb, dynFrom)

	const tip = int64(777)
	txs := []*types.Transaction{
		spSignDef(t, signer, defKey, 0, recipient),
		spSignDyn(t, signer, dynKey, 0, recipient, tip),
	}

	defBefore := new(big.Int).Set(statedb.GetBalance(defFrom))
	dynBefore := new(big.Int).Set(statedb.GetBalance(dynFrom))

	// gl 84000 -> basic 42000 fits both basic transfers.
	passed, errored, err := spRun(t, statedb, spPostForkBlock(), 84000, 2*spGas, txs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(passed) != 2 || len(errored) != 0 {
		t.Fatalf("passed=%d errored=%d, want 2 and 0", len(passed), len(errored))
	}

	defSpent := new(big.Int).Sub(defBefore, statedb.GetBalance(defFrom))
	dynSpent := new(big.Int).Sub(dynBefore, statedb.GetBalance(dynFrom))

	wantDef := new(big.Int).Mul(types.GetDefaultGasPrice(), new(big.Int).SetUint64(spGas))
	if defSpent.Cmp(wantDef) != 0 {
		t.Fatalf("default-fee tx spent %v, want base-only %v", defSpent, wantDef)
	}
	wantDyn := new(big.Int).Mul(new(big.Int).Add(spDynBaseFee(), big.NewInt(tip)), new(big.Int).SetUint64(spGas))
	if dynSpent.Cmp(wantDyn) != 0 {
		t.Fatalf("dynamic-fee tx spent %v, want base+tip %v", dynSpent, wantDyn)
	}
}
