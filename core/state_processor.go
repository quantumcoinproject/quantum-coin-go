// Copyright 2015 The go-ethereum Authors
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
	"errors"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/misc"
	"github.com/quantumcoinproject/quantum-coin-go/conversionutil"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/conversion"
	"math/big"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/core/vm"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

type ProcessMode byte

const (
	ProcessModeWorker                     ProcessMode = 1
	ProcessModeInsertChainReturnOnError   ProcessMode = 2
	ProcessModeInsertChainNoReturnOnError ProcessMode = 3
)

const DeepCheckStartBlock = uint64(3000000)
const DefaultGasLimit = 300000000

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)

	err := p.engine.PostPare(p.bc, header)
	if err != nil {
		return nil, nil, 0, err
	}
	// Mutate the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}

	var processMode ProcessMode
	if header.Number.Uint64() < DeepCheckStartBlock {
		processMode = ProcessModeInsertChainReturnOnError
	} else {
		processMode = ProcessModeInsertChainNoReturnOnError
	}

	// Iterate over and process the individual transactions
	signer := types.MakeSigner(p.config, header.Number)
	txnList := block.Transactions()
	receipts, allLogs, passedTransactions, skippedTransactions, errorTransactions, err := ProcessTransactions(p.config, p.bc, gp, statedb, header,
		&txnList, usedGas, cfg, &signer, processMode)
	if err != nil {
		return nil, nil, 0, err
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	err = p.engine.Finalize(p.bc, header, statedb, block.Transactions(), receipts, passedTransactions, skippedTransactions, errorTransactions, "StateProcessor.Process")
	if err != nil {
		return nil, nil, 0, err
	}

	backupManager := backupmanager.GetInstance()
	if backupManager != nil {
		err := backupManager.BackupBlock(block)
		if err != nil {
			return nil, nil, 0, err
		}
	}

	return receipts, allLogs, *usedGas, nil
}

func applyTransaction(msg types.Message, config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int,
	blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM) (*types.Receipt, error) {
	// Create a new context to be used in the EVM environment.
	txContext := NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	// Apply the transaction to the current state (included in the env).
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	// Update the state with pending changes.
	var root []byte
	if config.IsByzantium(blockNumber) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
	}
	*usedGas += result.UsedGas

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt := &types.Receipt{Type: tx.Type(), PostState: root, CumulativeGasUsed: *usedGas}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}

	// Set the receipt logs and create the bloom filter.
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockHash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = blockHash
	receipt.BlockNumber = blockNumber
	receipt.TransactionIndex = uint(statedb.TxIndex())
	return receipt, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction,
	usedGas *uint64, cfg vm.Config, signer *types.Signer) (*types.Receipt, error) {
	if bc == nil {
		return nil, errors.New("ChainContext is nil")
	}

	isGasExemptTxn := false

	if tx.To().IsEqualTo(conversion.CONVERSION_CONTRACT_ADDRESS) == true {
		isGasExempt, err := conversionutil.IsGasExemptTxn(tx, *signer)
		if err == nil && isGasExempt {
			log.Info("commitTransaction GasExemptTxn", "txn", tx.Hash())
			isGasExemptTxn = true
		}
	}

	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number))
	if err != nil {
		return nil, err
	}

	vmConfig := cfg
	if isGasExemptTxn {
		vmConfig = *cfg.DeepCopy()
		vmConfig.OverrideGasFailure = true
		msg.OverrideGasPrice(big.NewInt(0))
		log.Trace("ApplyTransaction OverrideGasPrice", "txn", tx.Hash(), "price", msg.GasPrice())
	}

	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, nil)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, config, vmConfig)
	return applyTransaction(msg, config, bc, nil, gp, statedb, header.Number, header.Hash(), tx, usedGas, vmenv)
}

func ProcessTransactions(config *params.ChainConfig, bc ChainContext, gp *GasPool, statedb *state.StateDB, header *types.Header, txList *types.Transactions,
	usedGas *uint64, cfg vm.Config, signer *types.Signer, processMode ProcessMode) (receipts []*types.Receipt, logs []*types.Log,
	passedTransactions types.Transactions, errorTransactions types.Transactions, skippedTransactions types.Transactions, err error) {
	receipts = make([]*types.Receipt, 0)
	logs = make([]*types.Log, 0)

	if len(*txList) == 0 {
		if header.GasUsed != 0 {
			return nil, nil, nil, nil, nil, errors.New("GasUsed is invalid")
		}
		return receipts, logs, passedTransactions, errorTransactions, skippedTransactions, nil
	}

	txs, skipped, err := types.NewTransactionsByNonceFromList(*signer, txList, header.ParentHash)
	if err != nil {
		log.Error("ProcessTransactions NewTransactionsByNonceFromList error", "error", err)
		return nil, nil, nil, nil, nil, err
	}
	log.Debug("ProcessTransactions NewTransactionsByNonceFromList", "skipped count", len(skipped))
	skippedTransactions = append(skippedTransactions, skipped...)

	count := 0

	for {
		hasRecords := txs.NextCursor()
		if hasRecords == false {
			log.Debug("ProcessTransactions loop done")
			break
		}
		tx := txs.PeekCursor()

		if gp.Gas() < params.TxGas {
			log.Debug("Not enough gas for further transactions", "have", gp, "want", params.TxGas)
			if processMode == ProcessModeInsertChainReturnOnError {
				return nil, nil, nil, nil, nil, errors.New("unexpected txn failure Gas")
			}
			skippedTransactions = append(skippedTransactions, tx)
			continue
		}

		if tx.Protected() && !config.IsEIP155(header.Number) {
			if processMode == ProcessModeInsertChainReturnOnError {
				return nil, nil, nil, nil, nil, errors.New("unexpected txn failure Protected")
			}
			skippedTransactions = append(skippedTransactions, tx)
			log.Trace("Ignoring reply protected transaction", "hash", tx.Hash(), "eip155", config.EIP155Block)
			continue
		}
		from, err := types.Sender(*signer, tx)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}

		statedb.Prepare(tx.Hash(), count)
		snap := statedb.Snapshot()
		receipt, err := ApplyTransaction(config, bc, gp, statedb, header, tx, usedGas, cfg, signer)

		if err != nil {
			if processMode == ProcessModeInsertChainReturnOnError {
				errOut := fmt.Errorf("could not apply tx [%v]: %w", tx.Hash().Hex(), err)
				return nil, nil, nil, nil, nil, errOut
			} else {
				if processMode == ProcessModeWorker {
					statedb.RevertToSnapshot(snap)
				}
				errorTransactions = append(errorTransactions, tx)
				switch {
				case errors.Is(err, ErrGasLimitReached):
					// Pop the current out-of-gas transaction without shifting in the next from the account
					log.Debug("Gas limit exceeded for current block", "sender", from)

				case errors.Is(err, ErrNonceTooLow):
					// New head notification data race between the transaction pool and miner, shift
					log.Debug("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())

				case errors.Is(err, ErrNonceTooHigh):
					// Reorg notification data race between the transaction pool and miner, skip account =
					log.Debug("Skipping account with high nonce", "sender", from, "nonce", tx.Nonce())

				case errors.Is(err, ErrTxTypeNotSupported):
					// Pop the unsupported transaction without shifting in the next from the account
					log.Debug("Skipping unsupported transaction type", "sender", from, "type", tx.Type())

				default:
					// Strange error, discard the transaction and get the next in line (note, the
					// nonce-too-high clause will prevent us from executing in vain).
					log.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
				}
			}
			continue
		}
		log.Debug("ProcessTransactions", "tx", tx.Hash().Hex(), "gp Gas", gp.Gas())
		count = count + 1
		receipts = append(receipts, receipt)
		logs = append(logs, receipt.Logs...)
		passedTransactions = append(passedTransactions, tx)

	}

	gasUsed := DefaultGasLimit - gp.Gas()
	if header.GasUsed != gasUsed {
		log.Error("ProcessTransactions() gas limit exceeded", "block", header.Number.Uint64(), "DefaultGasLimit", DefaultGasLimit,
			"gasUsed", gasUsed, "header.GasUsed", header.GasUsed, "gp.Gas()", gp.Gas(), "block txn count", len(*txList),
			"passed txn count", len(passedTransactions), "skipped txn count", len(skippedTransactions), "error txn count", len(errorTransactions))
		return nil, nil, nil, nil, nil, errors.New("gas limit exceeded")
	}

	return receipts, logs, passedTransactions, errorTransactions, skippedTransactions, nil
}
