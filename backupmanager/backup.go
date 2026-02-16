package backupmanager

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"

	"github.com/quantumcoinproject/quantum-coin-go/common"
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

const BlockValidatorContextValidator = "1"
const BlockValidatorContextBlockVerify = "2"

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

type BlockValidatorDetails struct {
	BlockNumber                  *big.Int             `json:"blockNumber" gencodec:"required"`
	ParentHash                   common.Hash          `json:"parentHash" gencodec:"required"`
	FilteredValidatorDepositList []ValidatorDeposit   `json:"filteredValidatorDepositList" gencodec:"required"`
	ValidatorDetailsList         []ValidatorDetailsV2 `json:"validatorDetailsList" gencodec:"optional"`

	PreFilterValidatorCount *big.Int    `json:"preFilterValidatorCount" gencodec:"required"`
	ConsensusContext        common.Hash `json:"consensusContext" gencodec:"required"`
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

func (b *BackupManager) BackupBlockValidatorDetails(details *BlockValidatorDetails, context string) error {
	b.consensusBackupLock.Lock()
	defer b.consensusBackupLock.Unlock()

	data, err := rlp.EncodeToBytes(details)
	if err != nil {
		log.Trace("EncodeToBytes BlockValidatorDetails", "err", err)
		return err
	}

	key := []byte(fmt.Sprintf("%d-%s", details.BlockNumber.Uint64(), context))

	db := *b.consensusdb
	err = db.Put(key, data)
	if err != nil {
		return err
	}

	log.Debug("BackupBlockValidatorDetails", "block", details.BlockNumber.Uint64(), "context", context)
	return nil
}

func (b *BackupManager) GetBlockValidatorDetails(blockNumber uint64, context string) (*BlockValidatorDetails, error) {
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

	details := BlockValidatorDetails{}
	//details.FilteredValidatorDepositList = make([]*ValidatorDeposit, 0)

	err = rlp.DecodeBytes(detailsBytes, &details)
	if err != nil {
		return nil, err
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
