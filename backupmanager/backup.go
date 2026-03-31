package backupmanager

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

type BackupManager struct {
	backupDir           string
	txBackupLock        sync.Mutex
	blkBackupLock       sync.Mutex
	consensusBackupLock sync.Mutex
	blockdb             *ethdb.Database
	txndb               *ethdb.Database
	consensusdb         *ethdb.Database
}

const BlockExtendedContextValidator = "1"
const BlockExtendedContextBlockVerify = "2"
const BlockExtendedContextValidatorError = "3"
const BlockExtendedContextBlockVerifyError = "4"

type ValidatorDeposit struct {
	ValidatorAddress  common.Address `json:"validatorAddress" gencodec:"required"`
	PostFilterDeposit *big.Int       `json:"postFilterDeposit" gencodec:"required"`
}

type ValidatorDetailsV2 struct {
	Depositor          common.Address `json:"depositor"     gencodec:"required"`
	Validator          common.Address `json:"validator"     gencodec:"required"`
	Balance            *big.Int       `json:"balance"       gencodec:"required"`
	NetBalance         *big.Int       `json:"netBalance"    gencodec:"required"`
	BlockRewards       *big.Int       `json:"blockRewards"  gencodec:"required"`
	Slashings          *big.Int       `json:"slashings"     gencodec:"required"`
	IsValidationPaused bool           `json:"isValidationPaused"  gencodec:"required"`
	WithdrawalBlock    *big.Int       `json:"withdrawalBlock"  gencodec:"required"`
	WithdrawalAmount   *big.Int       `json:"withdrawalAmount" gencodec:"required"`
	LastNiLBlock       *big.Int       `json:"lastNiLBlock" gencodec:"required"`
	NilBlockCount      *big.Int       `json:"nilBlockCount" gencodec:"required"`
}

// PreparedConsensusState is a JSON snapshot of the derived validator set and per-round consensus inputs
// produced by proof-of-stake preparation (filtered deposits, quorum totals, round proposers, NIL-vote hashes).
// It mirrors the in-memory proofofstake.PreparedConsensusState used during block consensus validation.
type PreparedConsensusState struct {
	// FilteredValidatorsDepositMap maps each validator address that survived filtering to its post-filter deposit weight (stake used in quorum math).
	FilteredValidatorsDepositMap map[common.Address]*big.Int `json:"filteredValidatorsDepositMap,omitempty"`

	// ValidatorDetailsMap is staking metadata per filtered validator (balance, pause flags, withdrawal fields, etc.).
	// Omitted or empty when the block height does not load extended validator records.
	ValidatorDetailsMap map[common.Address]ValidatorDetailsV2 `json:"validatorDetailsMap,omitempty"`

	// TotalBlockDepositValue is the sum of filtered validator deposits for the block (denominator for vote weights).
	TotalBlockDepositValue *big.Int `json:"totalBlockDepositValue,omitempty"`

	// MinDepositRequired is the minimum combined stake required to carry a proposal or vote phase at this height.
	MinDepositRequired *big.Int `json:"minDepositRequired,omitempty"`

	// RoundProposers maps consensus round index (1-based, through the chain’s maximum round count) to the scheduled block proposer address.
	RoundProposers map[byte]common.Address `json:"roundProposers,omitempty"`

	// NilVoteProposalHashes maps each round index to the canonical NIL proposal hash expected for that round.
	NilVoteProposalHashes map[byte]common.Hash `json:"nilVoteProposalHashes,omitempty"`

	// NilVotePrecommitHashes maps each round index to the canonical NIL precommit hash expected for that round.
	NilVotePrecommitHashes map[byte]common.Hash `json:"nilVotePrecommitHashes,omitempty"`
}

// BlockProposerV2TraceInput is passed into getBlockProposerV2 so the returned trace records how the selection hash relates to the parent block.
type BlockProposerV2TraceInput struct {
	// ParentHash is the parent block hash for the block being prepared (always the real parent, not the context key).
	ParentHash common.Hash `json:"parentHash"`
	// UsedConsensusContext is true when the first argument to getBlockProposerV2 is the derived consensus context hash; false when it is the parent hash (pre-context-based or transitional path).
	UsedConsensusContext bool `json:"usedConsensusContext"`
}

// BlockProposerV2ValidatorEval captures one validator's full staking record and the boolean result of canPropose for that height.
type BlockProposerV2ValidatorEval struct {
	// ValidatorAddress is the validator key in the preparation map (same as ValidatorDetails.Validator in normal cases).
	ValidatorAddress common.Address `json:"validatorAddress"`
	// ValidatorDetails is the full ValidatorDetailsV2 snapshot used for eligibility (deep copy for debugging).
	ValidatorDetails ValidatorDetailsV2 `json:"validatorDetails"`
	// CanPropose is whether canPropose returned true for this validator at this block (first pass, before MIN_VALIDATORS fallback).
	CanPropose bool `json:"canPropose"`
}

// BlockProposerV2SortComparison records one sort.SliceStable less(i,j) call (tie-break digests and validators at comparison time).
type BlockProposerV2SortComparison struct {
	IndexI     int            `json:"indexI"`
	IndexJ     int            `json:"indexJ"`
	ValidatorI common.Address `json:"validatorI"`
	ValidatorJ common.Address `json:"validatorJ"`
	Vi         common.Hash    `json:"vi"`
	Vj         common.Hash    `json:"vj"`
	CmpResult  int            `json:"cmpResult"`
	// ContextHash is the selection hash (consensus context or parent) fed into Keccak256 for this comparison.
	ContextHash common.Hash `json:"contextHash"`
	Round       byte        `json:"round"`
	// BlockBytes is the uint64 block number as little-endian bytes (same slice passed to Keccak when SortUsesBlockNumberInHash is true).
	BlockBytes hexutil.Bytes `json:"blockBytes"`
}

// BlockProposerV2RoundTrace is a full, JSON-friendly record of one getBlockProposerV2 invocation (inputs, per-validator canPropose evaluation, sort keys, and winner).
type BlockProposerV2RoundTrace struct {
	// Round is the consensus round index (1-based).
	Round byte `json:"round"`
	// BlockNumber is the child block height for which the proposer is computed.
	BlockNumber uint64 `json:"blockNumber"`
	// ParentHash is the parent block hash (from trace input).
	ParentHash common.Hash `json:"parentHash"`
	// SelectionHash is the hash passed into getBlockProposerV2 as the first argument (consensus context or parent, per fork rules).
	SelectionHash common.Hash `json:"selectionHash"`
	// UsedConsensusContext matches BlockProposerV2TraceInput.UsedConsensusContext.
	UsedConsensusContext bool `json:"usedConsensusContext"`
	// MinValidatorsRequired is MIN_VALIDATORS for this build.
	MinValidatorsRequired int `json:"minValidatorsRequired"`
	// InputValidatorCount is len(validatorMap) before filtering.
	InputValidatorCount int `json:"inputValidatorCount"`
	// AfterFilterCount is how many validators passed canPropose before any fallback.
	AfterFilterCount int `json:"afterFilterCount"`
	// FallbackExpandedToAll is true when fewer than MIN_VALIDATORS passed the filter and everyone was re-included.
	FallbackExpandedToAll bool `json:"fallbackExpandedToAll"`
	// SortUsesBlockNumberInHash is true when tie-break hashing includes uint64 block bytes (OfflineValidatorV4StartBlock and above).
	SortUsesBlockNumberInHash bool `json:"sortUsesBlockNumberInHash"`
	// Config snapshots (defaults at evaluation time) affecting canPropose and sort.
	ConfigOfflineValidatorDeferStartBlock  uint64 `json:"configOfflineValidatorDeferStartBlock"`
	ConfigBlockProposerOfflineV2StartBlock uint64 `json:"configBlockProposerOfflineV2StartBlock"`
	ConfigOfflineValidatorV4StartBlock     uint64 `json:"configOfflineValidatorV4StartBlock"`
	ConfigMinOfflineProposerBlockDelay     uint64 `json:"configMinOfflineProposerBlockDelay"`
	ConfigMaxBlockDelayV1                  uint64 `json:"configMaxBlockDelayV1"`
	ConfigMaxBlockDelayV2                  uint64 `json:"configMaxBlockDelayV2"`
	ConfigMaxBlockDelayV3                  uint64 `json:"configMaxBlockDelayV3"`
	// ValidatorEvaluations lists every validator in the input map with full details and the canPropose result (sorted by validator address for stable diffs).
	ValidatorEvaluations []BlockProposerV2ValidatorEval `json:"validatorEvaluations"`
	// SortComparisons lists each less(i,j) evaluation during sort.SliceStable (vi/vj digests and addresses at comparison time).
	SortComparisons []BlockProposerV2SortComparison `json:"sortComparisons"`
	// SelectedProposer is validators[0] after sorting (the algorithm output).
	SelectedProposer common.Address `json:"selectedProposer"`
}

type BlockExtendedDetails struct {
	BlockNumber             *big.Int             `json:"blockNumber" gencodec:"required"`
	ParentHash              common.Hash          `json:"parentHash" gencodec:"required"`
	FilteredDeposits        []ValidatorDeposit   `json:"filteredValidatorDepositList" gencodec:"required"`
	StakingValidatorDetails []ValidatorDetailsV2 `json:"validatorDetailsList" gencodec:"optional"`

	PreFilterValidatorCount *big.Int    `json:"preFilterValidatorCount" gencodec:"required"`
	ConsensusContext        common.Hash `json:"consensusContext" gencodec:"required"`

	// PreparedConsensusState is the full preparation snapshot (maps, quorum totals, per-round proposer and NIL hashes). Stored as JSON in the consensus backup DB (see BackupBlockExtendedDetails).
	PreparedConsensusState *PreparedConsensusState `json:"preparedConsensusState,omitempty" rlp:"-"`

	// BlockProposerV2Traces has one entry per consensus round (index round-1). Entries are nil for rounds that used the legacy getBlockProposer (non-V2) path.
	BlockProposerV2Traces []*BlockProposerV2RoundTrace `json:"blockProposerV2Traces,omitempty" rlp:"-"`
}

var singleInstance *BackupManager

var singleConsensusInstance *BackupManager

func GetInstance() *BackupManager {
	return singleInstance
}

func GetConsensusInstance() *BackupManager {
	return singleConsensusInstance
}

var instanceLock sync.Mutex

func NewBackupManager(backupDir string) (*BackupManager, error) {
	instanceLock.Lock()
	defer instanceLock.Unlock()

	if singleInstance != nil {
		return singleInstance, nil
	}

	bm := &BackupManager{}

	err := bm.Initialize(backupDir)
	if err != nil {
		return nil, err
	}

	singleInstance = bm
	return bm, nil
}

func NewConsensusBackupManager(backupDir string) (*BackupManager, error) {
	instanceLock.Lock()
	defer instanceLock.Unlock()

	if singleConsensusInstance != nil {
		return singleConsensusInstance, nil
	}

	bm := &BackupManager{}

	err := bm.InitializeConsensusBlockManager(backupDir)
	if err != nil {
		return nil, err
	}

	singleConsensusInstance = bm
	return bm, nil
}

func (b *BackupManager) InitializeConsensusBlockManager(backupDir string) error {
	log.Debug("Initialize consensus backup", "backupDir", backupDir)

	filePath := filepath.Join(backupDir, "consensus.db")
	var consensusdb ethdb.Database
	consensusdb, err := rawdb.NewLevelDBDatabase(filePath, 32, 0, "", false)
	if err != nil {
		return err
	}

	b.backupDir = backupDir
	b.consensusdb = &consensusdb

	return nil
}

func (b *BackupManager) Initialize(backupDir string) error {
	log.Debug("Initialize backup", "backupDir", backupDir)

	blockdbFilePath := filepath.Join(backupDir, "blockbackup.db")
	var blkdb ethdb.Database
	blkdb, err := rawdb.NewLevelDBDatabase(blockdbFilePath, 32, 0, "", false)
	if err != nil {
		return err
	}

	txndbFilePath := filepath.Join(backupDir, "txnbackup.db")
	var txndb ethdb.Database
	txndb, err = rawdb.NewLevelDBDatabase(txndbFilePath, 64, 0, "", false)
	if err != nil {
		return err
	}

	consensusFilePath := filepath.Join(backupDir, "consensus.db")
	var consensusdb ethdb.Database
	consensusdb, err = rawdb.NewLevelDBDatabase(consensusFilePath, 32, 0, "", false)
	if err != nil {
		return err
	}

	b.backupDir = backupDir
	b.blockdb = &blkdb
	b.txndb = &txndb
	b.consensusdb = &consensusdb

	return nil
}

func (b *BackupManager) BackupTransaction(tx *types.Transaction) error {
	b.txBackupLock.Lock()
	defer b.txBackupLock.Unlock()

	var buff bytes.Buffer
	buffWriter := bufio.NewWriter(&buff)

	err := tx.EncodeRLP(buffWriter)
	if err != nil {
		return err
	}
	err = buffWriter.Flush()
	if err != nil {
		return err
	}

	db := *b.txndb
	err = db.Put(tx.Hash().Bytes(), buff.Bytes())
	if err != nil {
		return err
	}

	log.Trace("BackupTransaction", "tx", tx.Hash())
	return nil
}

func (b *BackupManager) BackupBlock(blk *types.Block) error {
	b.blkBackupLock.Lock()
	defer b.blkBackupLock.Unlock()

	for _, tx := range blk.Transactions() {
		err := b.BackupTransaction(tx)
		if err != nil {
			return err
		}
	}

	var buff bytes.Buffer
	buffWriter := bufio.NewWriter(&buff)

	err := blk.EncodeRLP(buffWriter)
	if err != nil {
		return err
	}
	err = buffWriter.Flush()
	if err != nil {
		return err
	}

	db := *b.blockdb
	err = db.Put(blk.Hash().Bytes(), buff.Bytes())
	if err != nil {
		return err
	}

	//Mapping from block number to hash
	blkNumberBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blkNumberBytes, blk.NumberU64())
	blkNumberHash := crypto.Keccak256(blkNumberBytes)
	err = db.Put(blkNumberHash, blk.Hash().Bytes())
	if err != nil {
		return err
	}

	log.Trace("BackupBlock", "number", blk.Number(), "hash", blk.Hash())
	return nil
}

func (b *BackupManager) BlockExists(hash common.Hash) error {
	b.blkBackupLock.Lock()
	defer b.blkBackupLock.Unlock()

	db := *b.blockdb
	_, err := db.Get(hash.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (b *BackupManager) GetBlock(hash common.Hash) (*types.Block, error) {
	b.blkBackupLock.Lock()
	defer b.blkBackupLock.Unlock()

	db := *b.blockdb
	blockBytes, err := db.Get(hash.Bytes())
	if err != nil {
		return nil, err
	}

	return types.DecodeBlockFromRLP(blockBytes)
}

func (b *BackupManager) GetBlockHash(number uint64) (common.Hash, error) {
	b.blkBackupLock.Lock()
	defer b.blkBackupLock.Unlock()

	blkNumberBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blkNumberBytes, number)
	blkNumberHash := crypto.Keccak256(blkNumberBytes)

	db := *b.blockdb
	blockHashBytes, err := db.Get(blkNumberHash)
	if err != nil {
		return common.ZERO_HASH, err
	}

	if len(blockHashBytes) != len(common.ZERO_HASH.Bytes()) {
		return common.ZERO_HASH, errors.New("block hash length mismatch")
	}

	return common.BytesToHash(blockHashBytes), nil
}

func (b *BackupManager) TrsansactionExists(hash common.Hash) error {
	b.txBackupLock.Lock()
	defer b.txBackupLock.Unlock()

	db := *b.txndb
	_, err := db.Get(hash.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (b *BackupManager) BackupBlockExtendedDetails(details *BlockExtendedDetails, context string) error {
	b.consensusBackupLock.Lock()
	defer b.consensusBackupLock.Unlock()

	data, err := json.Marshal(details)
	if err != nil {
		log.Trace("json.Marshal BlockExtendedDetails", "err", err)
		return err
	}

	key := []byte(fmt.Sprintf("%d-%s", details.BlockNumber.Uint64(), context))

	db := *b.consensusdb
	err = db.Put(key, data)
	if err != nil {
		return err
	}

	log.Debug("BackupBlockExtendedDetails", "block", details.BlockNumber.Uint64(), "context", context)
	return nil
}

func (b *BackupManager) GetBlockExtendedDetails(blockNumber uint64, context string) (*BlockExtendedDetails, error) {
	b.consensusBackupLock.Lock()
	defer b.consensusBackupLock.Unlock()

	if b.consensusdb == nil {
		return nil, errors.New("consensusdb is nil")
	}

	key := []byte(fmt.Sprintf("%d-%s", blockNumber, context))

	db := *b.consensusdb
	detailsBytes, err := db.Get(key)
	if err != nil {
		return nil, err
	}

	details := BlockExtendedDetails{}

	if err := json.Unmarshal(detailsBytes, &details); err != nil {
		if err2 := rlp.DecodeBytes(detailsBytes, &details); err2 != nil {
			return nil, fmt.Errorf("decode BlockExtendedDetails: json=%v; rlp=%w", err, err2)
		}
	}

	return &details, nil
}

func (b *BackupManager) Close() error {
	b.consensusBackupLock.Lock()
	defer b.consensusBackupLock.Unlock()

	b.blkBackupLock.Lock()
	defer b.blkBackupLock.Unlock()

	b.txBackupLock.Lock()
	defer b.txBackupLock.Unlock()

	if b.blockdb != nil {
		blkdb := *b.blockdb
		err := blkdb.Close()
		if err != nil {
			log.Debug("backup manager blockdb close error", "err", err)
			return err
		}
	}

	if b.txndb != nil {
		txndb := *b.txndb
		err := txndb.Close()
		log.Debug("backup manager txndb close error", "err", err)
		if err != nil {
			return err
		}
	}

	if b.consensusdb != nil {
		consensusdb := *b.consensusdb
		err := consensusdb.Close()
		log.Debug("backup manager consensusdb close error", "err", err)
		if err != nil {
			return err
		}
	}

	return nil
}
