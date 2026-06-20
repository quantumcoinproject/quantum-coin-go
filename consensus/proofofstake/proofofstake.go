// Copyright 2017 The go-ethereum Authors
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

// Package proofofstake implements the proof-of-authority consensus engine.
package proofofstake

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/conversionutil"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/handler"
	"github.com/quantumcoinproject/quantum-coin-go/internal/ethapi"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/consensuscontext"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/conversion"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"github.com/quantumcoinproject/quantum-coin-go/trie"

	lru "github.com/hashicorp/golang-lru"
	"github.com/quantumcoinproject/quantum-coin-go/accounts"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/consensus"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking/stakingv2"
)

const (
	inmemorySnapshots  = 128  // Number of recent vote snapshots to keep in memory
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory
)

// ProofOfStake proof-of-authority protocol constants.
var (
	extraVanity = 32                                               // Fixed number of extra-data prefix bytes reserved for validator vanity
	extraSeal   = cryptobase.SigAlg.SignatureWithPublicKeyLength() // Fixed number of extra-data suffix bytes reserved for validator seal
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")

	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")

	// errInvalidDifficulty is returned if the difficulty of a block is not the blockNumber
	errInvalidDifficulty = errors.New("invalid difficulty")

	errInvalidCoinbase = errors.New("invalid coinbase")

	errInvalidNonce = errors.New("invalid nonce")

	errInvalidGasLimit = errors.New("invalid gas limit")
)

// SignerFn hashes and signs the data to be signed by a backing account.
type SignerFn func(signer accounts.Account, mimeType string, message []byte, sigAlg byte) ([]byte, error)
type SignerFnWithContext func(signer accounts.Account, mimeType string, message []byte, context []byte) ([]byte, error)
type SignerTxFn func(accounts.Account, *types.Transaction, *big.Int) (*types.Transaction, error)

// ProofOfStake is the proof-of-authority consensus engine proposed to support the
// Ethereum testnet following the Ropsten attacks.
type ProofOfStake struct {
	chainConfig *params.ChainConfig        // Chain config
	config      *params.ProofOfStakeConfig // Consensus engine configuration parameters
	genesisHash common.Hash
	db          ethdb.Database // Database to store and retrieve snapshot checkpoints

	recents    *lru.ARCCache // Snapshots for recent block to speed up reorgs
	signatures *lru.ARCCache // Signatures of recent blocks to speed up mining

	proposals map[common.Address]bool // Current list of proposals we are pushing

	signer            types.Signer
	validator         common.Address
	signFn            SignerFn // Signer function to authorize hashes with
	signFnWithContext SignerFnWithContext
	signTxFn          SignerTxFn

	ethAPI *ethapi.PublicBlockChainAPI

	lock sync.RWMutex // Protects the validator fields

	// The fields below are for testing only
	fakeDiff bool // Skip difficulty verifications

	consensusHandler *ConsensusHandler

	account    *accounts.Account
	blockchain *core.BlockChain
}

// New creates a ProofOfStake proof-of-authority consensus engine with the initial
// signers set to the ones provided by the user.
func New(chainConfig *params.ChainConfig, db ethdb.Database,
	ethAPI *ethapi.PublicBlockChainAPI, genesisHash common.Hash) *ProofOfStake {
	// Set any missing consensus parameters to their defaults
	conf := *chainConfig

	// Allocate the snapshot caches and c.ProofOfStakereate the engine
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)

	packetHandler := NewConsensusPacketHandler()

	proofofstake := &ProofOfStake{
		chainConfig:      chainConfig,
		config:           conf.ProofOfStake,
		genesisHash:      genesisHash,
		db:               db,
		ethAPI:           ethAPI,
		recents:          recents,
		signatures:       signatures,
		proposals:        make(map[common.Address]bool),
		signer:           types.NewLondonSigner(chainConfig.ChainID),
		consensusHandler: packetHandler,
	}

	proofofstake.consensusHandler.getValidatorsFn = proofofstake.GetValidators
	proofofstake.consensusHandler.listValidatorsFn = proofofstake.ListValidatorsAsMap
	proofofstake.consensusHandler.doesFinalizedTransactionExistFn = proofofstake.DoesFinalizedTransactionExist
	proofofstake.consensusHandler.getBlockConsensusContext = proofofstake.GetConsensusContext

	return proofofstake
}

func (c *ProofOfStake) SetP2PHandler(handler *handler.P2PHandler, localPeerId string) {
	log.Info("ProofOfStake SetP2PHandler", "localPeerId", localPeerId)
	if localPeerId == "" || len(localPeerId) == 0 {
		panic("invalid local peer id")
	}

	c.consensusHandler.SetP2PHandler(handler, localPeerId)
}

func (c *ProofOfStake) SetBlockchain(blockchain *core.BlockChain) {
	c.blockchain = blockchain
}

// Author implements consensus.Engine, returning the Ethereum address recovered
// from the signature in the header's extra-data section.
func (c *ProofOfStake) Author(header *types.Header) (common.Address, error) {
	return ZERO_ADDRESS, nil
}

func (c *ProofOfStake) DoesFinalizedTransactionExist(txnHash common.Hash) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	return c.ethAPI.DoesFinalizedTransactionExist(ctx, txnHash)
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *ProofOfStake) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return c.verifyHeader(chain, header, nil)
}

func flattenTxnMap(txnMap map[common.Address]types.Transactions) ([]common.Hash, map[common.Hash]common.Address) {
	if txnMap == nil {
		return nil, nil
	}

	count := 0
	for _, v := range txnMap {
		count = count + v.Len()
	}

	txnList := make([]common.Hash, count)
	txnAddressMap := make(map[common.Hash]common.Address)
	i := 0
	for k, v := range txnMap {
		for _, txn := range v {
			log.Trace("flattenTxnMap", "Hash", txn.Hash())
			txnList[i].CopyFrom(txn.Hash())
			txnAddressMap[txnList[i]] = k
			i = i + 1
		}
	}

	return txnList, txnAddressMap
}

func recreateTxnMap(selectedTxns []common.Hash, txnAddressMap map[common.Hash]common.Address, txnMap map[common.Address]types.Transactions) (map[common.Address]types.Transactions, error) {
	if selectedTxns == nil {
		return nil, nil
	}

	resultMap := make(map[common.Address]types.Transactions)
	for _, txnHash := range selectedTxns {
		addr, ok := txnAddressMap[txnHash]
		if ok == false {
			log.Warn("recreateTxnMap txn not fouud", "tx", txnHash.Hex())
			for k, v := range txnAddressMap {
				log.Trace("recreateTxnMap txnAddressMap", "k", k, "v", v)
			}
			if defaults.SkipMissingTxn() {
				log.Warn("SKIP_TXN is set")
				continue
			}
			return nil, errors.New("unknown transaction") //todo: fail?
		}
		txnList, ok := txnMap[addr]
		if ok == false {
			return nil, errors.New("unknown address")
		}
		for _, txnInner := range txnList {
			hash := txnInner.Hash()
			if hash.IsEqualTo(txnHash) {
				_, ok := resultMap[addr]
				if ok == false {
					resultMap[addr] = make([]*types.Transaction, 0)
				}
				resultMap[addr] = append(resultMap[addr], txnInner)
			}
		}
	}

	return resultMap, nil
}

func (c *ProofOfStake) IsBlockReadyToSeal(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) bool {
	blockState, _, err := c.consensusHandler.getBlockState(header.ParentHash)
	if err != nil {
		log.Trace("IsBlockReadyToSeal", "blockState", blockState, "err", err)
		return false
	}
	if blockState != BLOCK_STATE_RECEIVED_COMMITS {
		return false
	}

	return true
}

// Whether ok to freeze transactions (final set for the block)
func (c *ProofOfStake) ShouldFreezeTransactions(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) (bool, error) {
	if c.signFn == nil {
		return false, errors.New("not a miner")
	}
	blockState, _, err := c.consensusHandler.getBlockState(header.ParentHash)
	if err != nil {
		log.Debug("getBlockState", "err", err)
		return false, err
	}

	return blockState > BLOCK_STATE_WAITING_FOR_PROPOSAL, nil
}

// HandleTransactions selects the transactions for including in the block according to the consensus rules.
func (c *ProofOfStake) HandleTransactions(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB,
	txnMap map[common.Address]types.Transactions) (map[common.Address]types.Transactions, error) {
	if c.signFn == nil {
		return nil, errors.New("not a miner")
	}
	txns, txnAddressMap := flattenTxnMap(txnMap)

	err := c.consensusHandler.HandleConsensus(header.ParentHash, txns, header.Number.Uint64())
	if err != nil {
		return nil, err
	}

	blockState, round, err := c.consensusHandler.getBlockState(header.ParentHash)
	if err != nil {
		log.Debug("getBlockState", "err", err)
		return nil, err
	}
	if blockState != BLOCK_STATE_RECEIVED_COMMITS {
		return nil, errors.New("not ready yet")
	}
	vote, err := c.consensusHandler.getBlockVote(header.ParentHash)
	if err != nil {
		log.Debug("getBlockVote", "err", err)
		return nil, err
	}

	selectedTxns, err := c.consensusHandler.getBlockSelectedTransactions(header.ParentHash)
	if err != nil {
		log.Debug("getBlockSelectedTransactions", "err", err)
		return nil, err
	}
	if selectedTxns == nil {
		log.Debug("getBlockSelectedTransactions nil")
		return nil, nil
	}

	log.Debug("HandleTransactions", "in txn count", len(txns), "out txn count", len(selectedTxns), "round", round, "vote", vote)
	for _, t := range txns {
		log.Trace("HandleTransactions intxns", "txn", t)
	}
	for _, t := range selectedTxns {
		log.Trace("HandleTransactions outtxns", "txn", t)
	}

	resultMap, err := recreateTxnMap(selectedTxns, txnAddressMap, txnMap)
	if err != nil {
		log.Debug("recreateTxnMap", "err", err)
		return nil, err
	}

	return resultMap, nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers. The
// method returns a quit channel to abort the operations and a results channel to
// retrieve the async verifications (the order is that of the input slice).
func (c *ProofOfStake) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := c.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (c *ProofOfStake) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}

	number := header.Number.Uint64()

	// Don't waste time checking blocks from the future
	if header.Time > uint64(time.Now().Unix()) {
		return consensus.ErrFutureBlock
	}

	if header.Coinbase.IsEqualTo(ZERO_ADDRESS) == false {
		return errInvalidCoinbase
	}

	if header.Nonce.Uint64() != 0 {
		return errInvalidNonce
	}

	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errInvalidMixDigest
	}

	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if number > 0 {
		if header.Difficulty == nil || header.Difficulty.Uint64() != number {
			return errInvalidDifficulty
		}
	}
	//Gas limit: below GasV2StartBlock it is the fixed legacy value; from the fork it is
	//dynamic, so only bounds are checked here (no state available). The exact value is
	//enforced authoritatively against parent state in state_processor.ProcessTransactions.
	if number < defaults.DefaultConfig.PosConfig.GasV2StartBlock {
		if header.GasLimit != defaults.GetGasLimit(number) {
			return errInvalidGasLimit
		}
	} else {
		if header.GasLimit > defaults.GetMaxGasLimit(number) || header.GasLimit < defaults.MIN_DYNAMIC_GAS_LIMIT {
			return errInvalidGasLimit
		}
	}

	//GasUsed is checked in state_processor

	_, err := VerifyExtraData(number, header.Extra)
	if err != nil {
		return err
	}

	/*//Extra data
	if header.Number.Uint64() >= core.DeepCheckStartBlock {
		blockExtraData, err := DecodeBlockExtraData(header.Extra)
		if err != nil {
			return err
		}
		//todo: verify blockExtraData
		log.Debug("blockExtraData", "decoded", len(blockExtraData.ExtraData))
	}*/

	// All basic checks passed, verify cascading fields
	return c.verifyCascadingFields(chain, header, parents)
}

// verifyCascadingFields verifies all the header fields that are not standalone,
// rather depend on a batch of previous headers. The caller may optionally pass
// in a batch of parents (ascending order) to avoid looking those up from the
// database. This is useful for concurrently verifying a batch of new headers.
func (c *ProofOfStake) verifyCascadingFields(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	// The genesis block is the always valid dead-end
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}
	// Ensure that the block's timestamp isn't too close to its parent
	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}

	return c.verifySeal(chain, header, parents)
}

// verifySeal checks whether the signature contained in the header satisfies the
// consensus protocol requirements. The method accepts an optional list of parent
// headers that aren't yet part of the local blockchain to generate the snapshots
// from.
func (c *ProofOfStake) verifySeal(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	// Verifying the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return errUnknownBlock
	}

	if header.ConsensusData == nil || header.UnhashedConsensusData == nil {
		log.Trace("VerifyBlockConsensusData nil")
		return errors.New("nil consensusdata")
	}

	blockConsensusData := &BlockConsensusData{}
	err := rlp.DecodeBytes(header.ConsensusData, blockConsensusData)
	if err != nil {
		return err
	}

	blockAdditionalConsensusData := &BlockAdditionalConsensusData{}
	err = rlp.DecodeBytes(header.UnhashedConsensusData, blockAdditionalConsensusData)
	if err != nil {
		return err
	}

	if blockConsensusData.Round < 1 {
		return errors.New("verifySeal round")
	}

	if blockConsensusData.PrecommitHash.IsEqualTo(ZERO_HASH) {
		return errors.New("VerifyBlockConsensusData PrecommitHash ProposalHash zero_hash")
	}

	if blockConsensusData.Round > 1 {
		if len(blockConsensusData.SlashedBlockProposers) < int(blockConsensusData.Round-1) {
			return errors.New("VerifyBlockConsensusData SlashedBlockProposers length")
		}
	}

	if blockConsensusData.VoteType == VOTE_TYPE_NIL {
		if blockConsensusData.BlockProposer.IsEqualTo(ZERO_ADDRESS) == false {
			return errors.New("VerifyBlockConsensusData BlockProposer false")
		}

		//todo: deep validate block proposers
	} else if blockConsensusData.VoteType == VOTE_TYPE_OK {
		if blockConsensusData.BlockProposer.IsEqualTo(ZERO_ADDRESS) {
			return errors.New("VerifyBlockConsensusData BlockProposer true")
		}
	} else {
		return errors.New("unknown VoteType")
	}
	return nil
}

func (c *ProofOfStake) PostPare(chain consensus.ChainHeaderReader, header *types.Header) error {
	number := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = parent.Time + c.config.Period

	return nil
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (c *ProofOfStake) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	header.Coinbase = common.Address{}
	header.Nonce = types.BlockNonce{}
	number := header.Number.Uint64()
	header.Difficulty = header.Number

	if number < defaults.DefaultConfig.PosConfig.ExtraDataV3StartBlock {
		if len(header.Extra) < extraVanity {
			header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
		}
		header.Extra = header.Extra[:extraVanity]
		header.Extra = append(header.Extra, make([]byte, extraSeal)...)
	} else {
		header.Extra = make([]byte, 0)
	}

	header.MixDigest = common.Hash{}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Time = parent.Time + c.config.Period

	return nil
}

func (c *ProofOfStake) VerifyBlock(chain consensus.ChainHeaderReader, block *types.Block) error {
	header := block.Header()
	number := header.Number.Uint64()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	currentNumber := uint64(c.ethAPI.BlockNumber())
	currentHeader, err := c.ethAPI.GetHeaderByNumberInner(ctx, rpc.BlockNumber(currentNumber))
	if err != nil {
		log.Trace("VerifyBlock 1", "err", err)
		return err
	}

	if number != currentNumber+1 || header.ParentHash.IsEqualTo(currentHeader.Hash()) == false {
		log.Warn("VerifyBlock error mismatch", "got number", number, "expected number", currentNumber+1, "got parentHash", header.ParentHash, "expected parentHash", currentHeader.Hash())
		return errors.New("invalid block number or parent hash mismatch")
	}

	if defaults.SkipDeepBlockCheck() {
		//perform only a mini check
		if header.ConsensusData == nil || header.UnhashedConsensusData == nil {
			return errors.New("VerifyBlock ConsensusData is nil")
		}

		blockConsensusData := &BlockConsensusData{}
		err = rlp.DecodeBytes(header.ConsensusData, blockConsensusData)
		if err != nil {
			return err
		}

		blockAdditionalConsensusData := &BlockAdditionalConsensusData{}
		err = rlp.DecodeBytes(header.UnhashedConsensusData, blockAdditionalConsensusData)
		if err != nil {
			return err
		}

		if blockAdditionalConsensusData.ConsensusPackets == nil {
			return errors.New("VerifyBlockConsensusData ConsensusPackets is nil")
		}

		log.Info("VerifyBlock SKIP_BLOCK_DEEP_CHECK is set, skipping deep check. Do not use this mode except for testing", "number", header.Number.Uint64(), "hash", header.Hash())
		return nil
	}

	err = VerifyBlockConsensusData(block, nil, nil, c.GetConsensusContext, c.GetValidators, c.ListValidatorsAsMap)
	if err != nil {
		log.Trace("VerifyBlockConsensusData", "err", err)
	}

	return err
}

// isConversionRequestTxn reports whether calldata invokes requestConversion(string,string).
func isConversionRequestTxn(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	abiData, err := conversion.GetConversionContract_ABI()
	if err != nil {
		return false
	}
	method, ok := abiData.Methods[conversion.GetContract_Method_requestConversion()]
	if !ok {
		return false
	}
	return bytes.Equal(data[:4], method.ID)
}

func (c *ProofOfStake) Convert(header *types.Header, state *state.StateDB, txn *types.Transaction) error {
	msg, err := txn.AsMessage(c.signer)
	if err != nil {
		return err
	}

	log.Info("Conversion txn", "txn", txn.Hash(), "from", msg.From())

	eAddress, err := conversionutil.VerifyDataAndGetEthereumAddress(msg.From(), txn.Data())
	if err != nil {
		return err
	}

	//skip txn and proceed
	ethAddress := common.HexToAddress(eAddress) //representative address
	coins, err := c.GetCoinsForEthereumAddress(ethAddress, state, header)
	if err != nil {
		return err
	}

	if coins.Cmp(big.NewInt(0)) <= 0 {
		return nil
	}

	converted, err := c.GetConversionStatus(ethAddress, state, header)
	if err != nil {
		return err
	}

	if converted == true {
		log.Info("Conversion txn already converted, skipping", "txn", txn.Hash(), "from", msg.From())
		return nil
	}

	retCoins, err := c.SetConverted(ethAddress, msg.From(), state, header)
	if err != nil {
		return err
	}

	retQuantAddress, err := c.GetQuantumAddress(ethAddress, state, header)
	if err != nil {
		return err
	}

	log.Info("=================Conversion successful", "ethAddress", eAddress, "quantumAddress", msg.From(), "coins", coins, "retQuantAddress", retQuantAddress, "retCoins", retCoins)

	return nil
}

// Finalize implements consensus.Engine
func (c *ProofOfStake) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt,
	passedTransactions types.Transactions, errorTransactions types.Transactions, source string) error {
	if txs == nil {
		txs = make([]*types.Transaction, 0)
	} else {
		for _, tx := range txs {
			if tx.VerifyFields() == false {
				log.Trace("Txn VerifyFields failed", "Hash", tx.Hash())
				return errors.New("Transaction VerifyFields failed")
			}
			signerHash, err := c.signer.Hash(tx)
			if err != nil {
				log.Trace("Finalize Hash failed", "Hash", tx.Hash(), "error", err)
				return err
			}
			if !tx.Verify(signerHash.Bytes()) {
				log.Trace("Txn Verify failed", "Hash", tx.Hash())
				return errors.New("Transaction verify failed")
			} else {
				log.Trace("Txn Verify ok", "Hash", tx.Hash())
			}
		}
	}

	blockNumber := header.Number.Uint64()

	// should not happen. Once happen, stop the node is better than broadcast the block
	if header.GasLimit < header.GasUsed {
		return errors.New("gas consumption of system txs exceed the gas limit")
	}

	blockConsensusData := &BlockConsensusData{}
	err := rlp.DecodeBytes(header.ConsensusData, blockConsensusData)
	if err != nil {
		return err
	}

	err = c.verifyTransactions(header, txs, blockConsensusData, passedTransactions, errorTransactions, source)
	if err != nil {
		return err
	}

	//Conversions
	if blockConsensusData.VoteType == VOTE_TYPE_OK && txs != nil {
		for _, txn := range txs {
			if txn.To().IsEqualTo(conversion.CONVERSION_CONTRACT_ADDRESS) == false {
				continue
			}
			//Only calls carrying the requestConversion(string,string) selector are conversions.
			//Read-method calls (getAmount, getConversionStatus, getQuantumAddress) and any other
			//calldata are ignored here so they cannot reach the conversion-processing path.
			if isConversionRequestTxn(txn.Data()) == false {
				log.Trace("skipping non-conversion call to conversion contract", "txn", txn.Hash())
				continue
			}
			err = c.Convert(header, state, txn)
			if err != nil {
				//Abort rather than skip: a Convert error can stem from a transient local
				//issue (state/db), not just a malformed request, so silently skipping could
				//diverge state. The selector gate above already filters out non-conversion
				//calls (e.g. getAmount) that previously reached this path.
				log.Info("Convert error", "err", err)
				return err
			}
		}
	}

	//Block Slashing
	//If Round = 1, then it means PROPOSER was likely offline, as opposed to Round = 2 which means validators were not able to get consensus on time
	if blockConsensusData.Round == 1 && blockConsensusData.SlashedBlockProposers != nil && len(blockConsensusData.SlashedBlockProposers) > 0 && blockNumber >= defaults.DefaultConfig.PosConfig.SlashStartBlockNumber {

		var slashAmount *big.Int
		if blockNumber < defaults.DefaultConfig.PosConfig.SlashV2StartBlock {
			slashAmount = defaults.DefaultConfig.PosConfig.SLASH_AMOUNT
		} else {
			slashAmount = defaults.DefaultConfig.PosConfig.SLASH_AMOUNT_V2
		}

		for _, val := range blockConsensusData.SlashedBlockProposers {
			depositor, err := c.GetDepositorOfValidator(val, header.ParentHash)
			if err != nil {
				return err
			}
			log.Trace("depositor slashing", "depositor", depositor)
			slashTotal, err := c.AddDepositorSlashing(header.ParentHash, depositor, slashAmount, state, header)
			if err != nil {
				log.Trace("AddDepositorSlashing err", "err", err)
				return err
			}
			log.Trace("slashed amount", "slashTotal", slashTotal, "slashAmount", slashAmount, "depositor", depositor)

			if c.signFn != nil && val.IsEqualTo(c.validator) {
				log.Warn("Your account got a slashing!", "parentHash", header.ParentHash)
			}
		}
	}

	//Validator nil block
	//If Round = 1, then it means PROPOSER was likely offline, as opposed to Round = 2 which means validators were not able to get consensus on time
	if blockConsensusData.VoteType == VOTE_TYPE_NIL && blockConsensusData.Round == 1 && blockConsensusData.SlashedBlockProposers != nil &&
		len(blockConsensusData.SlashedBlockProposers) > 0 && blockNumber >= defaults.DefaultConfig.PosConfig.VALIDATOR_NIL_BLOCK_START_BLOCK {
		for _, val := range blockConsensusData.SlashedBlockProposers {
			err = c.SetNilBlock(val, state, header)
			if err != nil {
				log.Error("SetNilBlock err", "err", err, "blockNumber", header.Number.Uint64())
				return err
			}
		}
	}

	//Block Rewards
	if blockConsensusData.VoteType == VOTE_TYPE_OK && blockNumber >= defaults.DefaultConfig.PosConfig.RewardStartBlockNumber {
		blockProposerRewardAmount := GetReward(header.Number)

		//Add same amount of reward to Staking Contract, so that it is available for withdrawal later on
		err := c.accumulateBalance(state, blockProposerRewardAmount, common.HexToAddress(staking.GetStakingContract_Address_String()))
		if err != nil {
			log.Error("accumulateBalance staking contract err", "err", err)
			return err
		}

		//Get depositor of validator
		depositor, err := c.GetDepositorOfValidator(blockConsensusData.BlockProposer, header.ParentHash)
		if err != nil {
			log.Error("GetDepositorOfValidator", "err", err)
			return err
		}

		//If txn fee for proposer criteria is met and the block has transactions
		if blockNumber >= core.TXN_FEE_CUTTOFF_BLOCK && len(txs) > 0 {
			txnFeeTotal, rewardsAmountTxnFee, burnAmountTxnFee, err := calculateTxnFeeSplit(blockProposerRewardAmount, txs, receipts)
			if err != nil {
				return err
			}

			//Accumulate additional fee from transactions to staking contract
			err = c.accumulateBalance(state, rewardsAmountTxnFee, common.HexToAddress(staking.GetStakingContract_Address_String()))
			if err != nil {
				log.Error("accumulateBalance rewardsAmountTxnFee staking contract err", "err", err)
				return err
			}
			blockProposerRewardAmount = common.SafeAddBigInt(blockProposerRewardAmount, rewardsAmountTxnFee)

			//burn whatever needs to be burnt from the txn fee
			burn(state, burnAmountTxnFee)

			log.Trace("Reward amount", "BlockNumber", header.Number, "rewardsAmountTxnFee", rewardsAmountTxnFee, "burnAmountTxnFee", burnAmountTxnFee, "txnFeeTotal", txnFeeTotal)
		}

		//Update staking contract with reward details
		blockProposerRewardAmountTotal, err := c.AddDepositorReward(header.ParentHash, depositor, blockProposerRewardAmount, state, header)
		if err != nil {
			log.Error("AddDepositorReward err", "err", err)
			return err
		}
		log.Trace("Reward amount", "BlockNumber", header.Number, "blockProposerRewardAmountTotal", blockProposerRewardAmountTotal, "blockProposerRewardAmount", blockProposerRewardAmount, "BlockProposer", blockConsensusData.BlockProposer)

		if blockConsensusData.VoteType == VOTE_TYPE_OK && c.signFn != nil && blockConsensusData.BlockProposer.IsEqualTo(c.validator) {
			log.Info("You potentially proposed and mined a new block!", "BlockNumber", header.Number, "parentHash", header.ParentHash)
		}

		//Validator nil block reset
		if blockNumber > defaults.DefaultConfig.PosConfig.VALIDATOR_NIL_BLOCK_START_BLOCK {
			err = c.ResetNilBlock(blockConsensusData.BlockProposer, state, header)
			if err != nil {
				log.Error("ResetNilBlock err", "err", err)
				return err
			}
		}
	}

	//Staking V2
	if blockNumber == defaults.DefaultConfig.PosConfig.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		log.Info("Setting stakingv2 contract code", "blockNumber", defaults.DefaultConfig.PosConfig.STAKING_CONTRACT_V2_CUTOFF_BLOCK)
		stakingContractCode := common.FromHex(stakingv2.STAKING_RUNTIME_BIN)
		state.SetCode(staking.STAKING_CONTRACT_ADDRESS, stakingContractCode)
	}

	//Consensus Context
	if blockNumber == defaults.DefaultConfig.PosConfig.CONSENSUS_CONTEXT_START_BLOCK {
		log.Info("Setting consensus context contract code", "blockNumber", defaults.DefaultConfig.PosConfig.CONSENSUS_CONTEXT_START_BLOCK)
		consensuscontextContractCode := common.FromHex(consensuscontext.CONSENSUS_CONTEXT_RUNTIME_BIN)
		state.SetCode(consensuscontext.CONSENSUS_CONTEXT_CONTRACT_ADDRESS, consensuscontextContractCode)
	}

	if blockNumber > defaults.DefaultConfig.PosConfig.CONSENSUS_CONTEXT_START_BLOCK {
		key, err := GetConsensusContextKey(blockNumber)
		if err != nil {
			log.Error("GetBlockConsensusContextFn err", "err", err)
			return err
		}
		var consensusContext [32]byte
		copy(consensusContext[:], header.ParentHash.Bytes())
		err = c.SetConsensusContext(key, consensusContext, state, header)
		if err != nil {
			log.Error("SetConsensusContext err", "err", err)
			return err
		}

		//Remove the oldest key
		if blockNumber > (defaults.DefaultConfig.PosConfig.CONSENSUS_CONTEXT_START_BLOCK + defaults.DefaultConfig.PosConfig.CONSENSUS_CONTEXT_MAX_BLOCK_COUNT) {
			oldKey, err := GetConsensusContextKey(blockNumber - defaults.DefaultConfig.PosConfig.CONSENSUS_CONTEXT_MAX_BLOCK_COUNT)
			if err != nil {
				log.Error("GetBlockConsensusContextKey oldKey err", "err", err)
				return err
			}

			err = c.DeleteConsensusContext(oldKey, state, header)
			if err != nil {
				log.Error("DeleteConsensusContext oldKey err", "err", err)
				return err
			}
		}
	}

	//Dynamic gas limit: record this block's nil-block status into the round-robin array.
	if blockNumber >= defaults.DefaultConfig.PosConfig.GasV2StartBlock {
		if err := c.writeGasNilStatus(state, header, blockConsensusData); err != nil {
			log.Error("writeGasNilStatus err", "err", err)
			return err
		}
	}

	//Fix blocktime
	parent := chain.GetHeader(header.ParentHash, blockNumber-1)
	if (blockNumber == 1 || blockNumber%BLOCK_PERIOD_TIME_CHANGE == 0 || blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_TIME_ORIG_START_BLOCK) &&
		blockConsensusData.VoteType == VOTE_TYPE_OK && parent.Time < blockConsensusData.BlockTime {
		header.Time = blockConsensusData.BlockTime
	} else {
		header.Time = parent.Time + c.config.Period
	}

	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

	log.Info("Finalize Block", "root", header.Root, "hash", header.Hash(), "number", header.Number, "txn count", len(txs), "source", source, "extraDataLen", len(header.Extra))

	return nil
}

func calculateTxnFeeSplit(originalBlockRewards *big.Int, txs []*types.Transaction, receipts []*types.Receipt) (txnFeeTotal *big.Int, txnFeeRewardsAmount *big.Int, burnAmount *big.Int, err error) {
	if len(receipts) != len(txs) {
		log.Error("Finalize receipts and txn invalid len", "receipts len", len(receipts), "txn len", len(txs))
		return nil, nil, nil, errors.New("finalize receipts and txn invalid length")
	}

	txnFeeTotal = big.NewInt(0)
	var txnMap map[common.Hash]*types.Transaction
	txnMap = make(map[common.Hash]*types.Transaction)
	for _, txn := range txs {
		txnMap[txn.Hash()] = txn
	}
	for _, receipt := range receipts {
		txn, ok := txnMap[receipt.TxHash]
		if ok == false {
			log.Error("Finalize txn not found in receipts", "hash", receipt.TxHash)
			return nil, nil, nil, errors.New("finalize txn not found in receipts")
		}
		gasCoinsUsed := common.SafeMulBigInt(txn.GasPrice(), new(big.Int).SetUint64(receipt.GasUsed))
		txnFeeTotal = common.SafeAddBigInt(txnFeeTotal, gasCoinsUsed)
		log.Trace("calculateTxnFeeSplit", "gasCoinsUsed", gasCoinsUsed, "txn", txn.Hash(), "gasPrice", txn.GasPrice(), "GasUsed", receipt.GasUsed)
	}

	burnAmount, txnFeeRewardsAmount = calculateTxnFeeSplitCoins(txnFeeTotal)

	if len(txs) > 0 {
		log.Trace("calculateTxnFeeSplit", "originalBlockRewards", originalBlockRewards, "txnFeeTotal", txnFeeTotal, "burnAmount", burnAmount, "txnFeeRewardsAmount", txnFeeRewardsAmount)
	}

	return txnFeeTotal, txnFeeRewardsAmount, burnAmount, nil
}

func calculateTxnFeeSplitCoins(txnFeeTotal *big.Int) (burnAmount *big.Int, txnFeeRewardsAmount *big.Int) {
	txnFeeRewardsAmount = common.SafeRelativePercentageBigInt(txnFeeTotal, big.NewInt(defaults.DefaultConfig.PosConfig.TxnFeeRewardsPercentage))
	burnAmount = common.SafeSubBigInt(txnFeeTotal, txnFeeRewardsAmount)
	return burnAmount, txnFeeRewardsAmount
}

func burn(state *state.StateDB, burnAmount *big.Int) {
	state.AddBalance(common.ZERO_ADDRESS, burnAmount)
}

func (c *ProofOfStake) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (*types.Block, error) {
	err := c.Finalize(chain, header, state, txs, receipts, nil, nil, "FinalizeAndAssemble")
	if err != nil {
		return nil, err
	}

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, receipts, trie.NewStackTrie(nil)), nil
}

func (c *ProofOfStake) FinalizeAndAssembleWithConsensus(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt,
	passedTransactions types.Transactions, errorTransactions types.Transactions) (*types.Block, error) {
	// Sealing the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		log.Debug("FinalizeAndAssembleWithConsensus number 0", "error", errUnknownBlock)
		return nil, errUnknownBlock
	}

	blockState, round, err := c.consensusHandler.getBlockState(header.ParentHash)
	if err != nil {
		log.Debug("getBlockState", "err", err)
		return nil, err
	}

	if blockState != BLOCK_STATE_RECEIVED_COMMITS {
		log.Debug("FinalizeAndAssembleWithConsensus BLOCK_STATE_WAITING_FOR_COMMITS", round)
		return nil, errors.New("Block state not yet BLOCK_STATE_WAITING_FOR_COMMITS")
	}

	blockConsensusData, blockAdditionalConsensusData, blockExtendedDetails, err := c.consensusHandler.getBlockConsensusData(header.ParentHash)
	if blockExtendedDetails != nil && backupmanager.GetConsensusInstance() != nil { //save even if error
		blockExtendedContext := backupmanager.BlockExtendedContextValidator
		if err != nil {
			blockExtendedContext = backupmanager.BlockExtendedContextValidatorError
		}
		errBackup := backupmanager.GetConsensusInstance().BackupBlockExtendedDetails(blockExtendedDetails, blockExtendedContext)
		if errBackup != nil {
			log.Warn("VerifyBlockConsensusDataInner backup consensus", "errBackup", errBackup)
		}
	}

	if err != nil {
		log.Warn("FinalizeAndAssembleWithConsensus getBlockConsensusData", "error", err)
		return nil, err
	}
	data, err := rlp.EncodeToBytes(blockConsensusData)
	if err != nil {
		log.Debug("EncodeToBytes blockConsensusData", "err", err)
		return nil, err
	}
	header.ConsensusData = make([]byte, len(data))
	copy(header.ConsensusData, data)

	data, err = rlp.EncodeToBytes(blockAdditionalConsensusData)
	if err != nil {
		log.Debug("EncodeToBytes blockAdditionalConsensusData", "err", err)
		return nil, err
	}
	header.UnhashedConsensusData = make([]byte, len(data))
	copy(header.UnhashedConsensusData, data)

	//Extra data
	if header.Number.Uint64() >= defaults.DefaultConfig.DeepCheckStartBlock {
		extraData, err := EncodeBlockExtraData(errorTransactions, header.Extra, header.Number.Uint64())
		if err != nil {
			log.Debug("EncodeBlockExtraData", "error", err)
			return nil, err
		}
		header.Extra = make([]byte, len(extraData))
		copy(header.Extra, extraData)
	}

	err = c.Finalize(chain, header, state, txs, receipts, passedTransactions, errorTransactions, "FinalizeAndAssembleWithConsensus")
	if err != nil {
		log.Debug("Finalize", "error", err)
		return nil, err
	}

	// Assemble and return the final block for sealing
	block := types.NewBlock(header, txs, receipts, trie.NewStackTrie(nil))

	//Verify block once more before sealing, so that non-validator path is tested
	err = c.VerifyBlock(chain, block)
	if err != nil {
		log.Warn("FinalizeAndAssembleWithConsensus VerifyBlock", "error", err)
		return nil, err
	}
	log.Debug("FinalizeAndAssembleWithConsensus VerifyBlock ok")

	return block, nil
}

// Authorize injects a private key into the consensus engine to mint new blocks
// with.
func (c *ProofOfStake) Authorize(validator common.Address, signFn SignerFn, signFnWithContext SignerFnWithContext, signTxFn SignerTxFn, account accounts.Account) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.validator = validator
	c.signFn = signFn
	c.signTxFn = signTxFn
	c.signFnWithContext = signFnWithContext

	c.consensusHandler.signFn = signFn
	c.consensusHandler.signFnWithContext = signFnWithContext
	c.consensusHandler.account = account

	c.consensusHandler.peerHandler.SetSignFn(signFn, account)
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (c *ProofOfStake) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()
	log.Info("Seal Block", "ParentHash", block.ParentHash().String(), "Number", header.Number)

	delay := time.Second * 1
	go func() {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}

		select {
		case results <- block.WithSeal(header):
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", SealHash(header))
		}
	}()
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have:
// * DIFF_NOTURN(2) if BLOCK_NUMBER % SIGNER_COUNT != SIGNER_INDEX
// * DIFF_INTURN(1) if BLOCK_NUMBER % SIGNER_COUNT == SIGNER_INDEX
func (c *ProofOfStake) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(parent.Number.Int64() + 1)
}

// SealHash returns the hash of a block prior to it being sealed.
func (c *ProofOfStake) SealHash(header *types.Header) common.Hash {
	return SealHash(header)
}

// Close implements consensus.Engine. It's a noop for proofofstake as there are no background threads.
func (c *ProofOfStake) Close() error {
	return nil
}

// APIs implements consensus.Engine, returning the user facing RPC API to allow
// controlling the validator voting.
func (c *ProofOfStake) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{{
		Namespace: "proofofstake",
		Version:   "1.0",
		Service:   &API{chain: chain, proofofstake: c},
		Public:    true,
	}}
}

func (c *ProofOfStake) GetConsensusPacketHandler() *ConsensusHandler {
	return c.consensusHandler
}

func MakeMap(transactions types.Transactions) (map[common.Hash]bool, error) {
	m := make(map[common.Hash]bool)
	for _, tx := range transactions {
		_, ok := m[tx.Hash()]
		if ok {
			log.Error("MakeMap Duplicate transaction", "hash", tx.Hash())
			return nil, errors.New("duplicate transaction")
		}
		m[tx.Hash()] = true
	}
	return m, nil
}

func (c *ProofOfStake) verifyTransactions(header *types.Header, transactions []*types.Transaction, blockConsensusData *BlockConsensusData, passedTransactions types.Transactions,
	errorTransactions types.Transactions, source string) error {
	if header.Number.Uint64() < defaults.DefaultConfig.DeepCheckStartBlock {
		//todo: verify
		return nil
	}

	actualTxnCount := len(passedTransactions) + len(errorTransactions)
	if len(blockConsensusData.SelectedTransactions) != actualTxnCount {
		log.Error("verifyTransactions wrong number of transactions", "blockNumber", header.Number.Uint64(),
			"expected", len(blockConsensusData.SelectedTransactions), "actual", actualTxnCount, "len(blockConsensusData.SelectedTransactions)", len(blockConsensusData.SelectedTransactions),
			"len(passedTransactions)", len(passedTransactions), "len(errorTransactions)", len(errorTransactions), "source", source)
		return errors.New("wrong number of transactions")
	}

	blockTransactions := types.Transactions(transactions)

	if blockTransactions.IsEqualTo(passedTransactions) == false {
		log.Error("verifyTransactions passedTransactions IsEqualTo fail", "blockNumber", header.Number.Uint64(),
			"expected", len(blockConsensusData.SelectedTransactions), "actual", actualTxnCount, "len(blockConsensusData.SelectedTransactions)", len(blockConsensusData.SelectedTransactions),
			"len(passedTransactions)", len(passedTransactions), "len(errorTransactions)", len(errorTransactions), "source", source)
		return errors.New("wrong number of passed transactions")
	}

	blockExtraData, _, err := DecodeBlockExtraData(header.Extra, header.Number.Uint64())
	if err != nil {
		return err
	}

	if errorTransactions.IsEqualTo(blockExtraData.ErrorTransactions) == false {
		log.Error("verifyTransactions errorTransactions IsEqualTo fail",
			"expected", len(blockConsensusData.SelectedTransactions), "actual", actualTxnCount, "len(blockConsensusData.SelectedTransactions)", len(blockConsensusData.SelectedTransactions),
			"len(passedTransactions)", len(passedTransactions), "len(errorTransactions)", len(errorTransactions),
			"blockExtraData.ErrorTransactions", len(blockExtraData.ErrorTransactions))
		return errors.New("wrong number of error transactions")
	}

	if blockConsensusData.VoteType == VOTE_TYPE_NIL {
		if len(errorTransactions) > 0 || len(passedTransactions) > 0 || len(blockConsensusData.SelectedTransactions) > 0 {
			return errors.New("verifyTransactions skippedTransactions errorTransactions is present in NIL BLOCK")
		}
	} else {
		for _, tx := range errorTransactions {
			if tx.VerifyFields() == false {
				log.Trace("errorTransactions Txn VerifyFields failed", "Hash", tx.Hash())
				return errors.New("Transaction VerifyFields failed")
			}
			signerHash, err := c.signer.Hash(tx)
			if err != nil {
				log.Trace("errorTransactionssignerHash failed", "Hash", tx.Hash(), "error", err)
				return err
			}
			if !tx.Verify(signerHash.Bytes()) {
				log.Trace("errorTransactions Txn Verify failed", "Hash", tx.Hash())
				return errors.New("Transaction verify failed")
			} else {
				log.Trace("errorTransactions Txn Verify ok", "Hash", tx.Hash())
			}
		}

		selectTxnMap := make(map[common.Hash]bool)
		for _, t := range blockConsensusData.SelectedTransactions {
			_, ok := selectTxnMap[t]
			if ok {
				log.Error("verifyTransactions selected txn duplicate", "txn", t)
				return errors.New("duplicated transaction found")
			}
			selectTxnMap[t] = true
		}

		passedTxnMap, err := MakeMap(blockTransactions)
		if err != nil {
			log.Error("verifyTransactions passedTxnMap error")
			return err
		}

		errorTxnMap, err := MakeMap(errorTransactions)
		if err != nil {
			log.Error("verifyTransactions errorTxnMap error")
			return err
		}

		for k, _ := range selectTxnMap {
			foundCount := 0
			_, ok1 := passedTxnMap[k]
			if ok1 {
				foundCount = foundCount + 1
			}

			_, ok2 := errorTxnMap[k]
			if ok2 {
				foundCount = foundCount + 1
			}

			if foundCount == 0 {
				log.Error("verifyTransactions couldn't find txn in passed or skipped or error txn list", "txn", k)
				return errors.New("couldn't find txn in passed or skipped or error txn list")
			} else if foundCount != 1 {
				log.Error("verifyTransactions found multiple occurrences of txn", "txn", k, "foundCount", foundCount, "ok1", ok1, "ok2", ok2)
				return errors.New("verifyTransactions found multiple occurrences of txn")
			}
		}
	}

	return nil
}

func (c *ProofOfStake) ParseHeaderDetails(chain consensus.ChainHeaderReader, header *types.Header) (errorTransactions types.Transactions, err error) {
	if header.Number.Uint64() < defaults.DefaultConfig.DeepCheckStartBlock {
		return errorTransactions, nil
	}
	blockExtraInfo, _, err := DecodeBlockExtraData(header.Extra, header.Number.Uint64())
	if err != nil {
		return nil, err
	}
	return blockExtraInfo.ErrorTransactions, nil
}

// SealHash returns the hash of a block prior to it being sealed.
func SealHash(header *types.Header) (hash common.Hash) {
	buff := new(bytes.Buffer)
	encodeSigHeader(buff, header)
	hash.SetBytes(crypto.Keccak256(buff.Bytes()))
	return hash
}

func encodeSigHeader(w io.Writer, header *types.Header) {
	extra := header.Extra
	if header.Number.Uint64() < defaults.DefaultConfig.PosConfig.ExtraDataV3StartBlock {
		extra = header.Extra[:len(header.Extra)-cryptobase.SigAlg.SignatureWithPublicKeyLength()] // Yes, this will panic if extra is too short
	}

	enc := []interface{}{
		header.ParentHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		extra,
		header.MixDigest,
		header.Nonce,
	}

	if err := rlp.Encode(w, enc); err != nil {
		panic("can't encode: " + err.Error())
	}
}
