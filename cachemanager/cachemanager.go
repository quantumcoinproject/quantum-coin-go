package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/core"
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/params"
	"io/ioutil"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type CacheManager struct {
	cacheDir                 string
	nodeUrl                  string
	cacheLock                sync.Mutex
	cacheDb                  ethdb.Database
	client                   *ethclient.Client
	pendingTxClient          *ethclient.Client
	blockClient              *ethclient.Client
	enableExtendedApis       bool
	genesisCirculatingSupply string
	maxSupply                string
	pendingTxLock            sync.Mutex
	pendingTxMapLock         sync.RWMutex
	pendingTransactions      *map[string]map[string]map[string]*ethclient.TxPoolTransaction
	//addressMap               map[string]*AccountDetails
	tokenMap                 map[string]*TokenDetails
	accountTokenMap          map[string]*map[string]bool //map[accountAddress]map[contractAddress]bool
	blockChan                chan *PrimordialBlockData
	primodialCacheCancelChan chan bool
	primordialCache          *PrimordialCache
}

var chainID *big.Int
var LevelDbNoTFoundErrMsg = "leveldb: not found"
var NotFoundErrMsg = "not found"

const TimeLayout = "2006-01-02T15:04:05Z"

const PageSize uint64 = 20

func NewCacheManager(cacheDir string, nodeUrl string, enableExtendedApis bool, genesisFilePath string, maxSupply string) (*CacheManager, error) {
	cManager := &CacheManager{
		nodeUrl:            nodeUrl,
		cacheDir:           cacheDir,
		enableExtendedApis: enableExtendedApis,
		//addressMap:         make(map[string]*AccountDetails),
		tokenMap:        make(map[string]*TokenDetails),
		accountTokenMap: make(map[string]*map[string]bool),
	}

	var err error

	if enableExtendedApis {
		if len(maxSupply) == 0 {
			return nil, errors.New("max supply is nil")
		}
		maxSupplyBig, err := hexutil.DecodeBig(maxSupply)
		if err != nil {
			return nil, err
		}

		cManager.maxSupply = maxSupply

		genesisBytes, err := ioutil.ReadFile(genesisFilePath)
		if err != nil {
			log.Error("ReadFile", "error", err)
			return nil, err
		}

		genesis := core.Genesis{}
		err = json.Unmarshal(genesisBytes, &genesis)
		if err != nil {
			log.Error("Unmarshal", "error", err)
			return nil, err
		}

		genesisCirculatingSupply := big.NewInt(0)
		if genesis.Alloc != nil {
			for _, v := range genesis.Alloc {
				genesisCirculatingSupply = common.SafeAddBigInt(genesisCirculatingSupply, v.Balance)
			}
		}
		cManager.genesisCirculatingSupply = hexutil.EncodeBig(genesisCirculatingSupply)
		log.Error("genesis genesisCirculatingSupply", "genesisCirculatingSupply", params.WeiToEther(genesisCirculatingSupply), "maxSupply", params.WeiToEther(maxSupplyBig))
	}

	err = cManager.initialize()
	if err != nil {
		return nil, err
	}

	err = cManager.start()
	if err != nil {
		return nil, err
	}

	return cManager, nil
}

func (c *CacheManager) initialize() error {
	log.Info("Quantum Coin initialize cache manager", "cacheDir", c.cacheDir, "nodeUrl", c.nodeUrl)

	catchManagerFilePath := filepath.Join(c.cacheDir, "cacheManager.db")
	cacheManagerDb, err := rawdb.NewLevelDBDatabase(catchManagerFilePath, 64, 0, "", false)
	if err != nil {
		return err
	}
	c.cacheDb = cacheManagerDb

	//c.addressMap = make(map[string]*AccountDetails)
	//c.addressMap[staking.STAKING_CONTRACT] = &AccountDetails{Address: staking.STAKING_CONTRACT.HexLower(), AccType: ethclient.ACCOUNT_TYPE_CONTRACT}
	//c.addressMap[conversion.CONVERSION_CONTRACT] = &AccountDetails{Address: conversion.CONVERSION_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}

	c.primodialCacheCancelChan = make(chan bool)
	c.primordialCache, err = NewPrimordialCache(c.cacheDir, c.nodeUrl, &c.primodialCacheCancelChan)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) clientInitialize() {
	for {
		client, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("initialize client Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		blockClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("initialize blockClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		pendingTxClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("initialize pendingTxClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		chainID, err = client.NetworkID(context.Background())
		if err != nil {
			log.Error("initialize NetworkID", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		c.client = client
		c.blockClient = blockClient
		c.pendingTxClient = pendingTxClient

		break
	}
}

func (c *CacheManager) start() error {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	var runningSummary *BlockchainDetails

	blockNumber, err := c.getLastBlockNumberFromDb()
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Warn("First time start")
			blockNumber = 0
			if c.enableExtendedApis {
				runningSummary = &BlockchainDetails{
					BlockNumber:           0,
					MaxSupply:             c.maxSupply,
					TotalSupply:           c.genesisCirculatingSupply,
					CirculatingSupply:     c.genesisCirculatingSupply,
					BurntCoins:            "0x0",
					BlockRewardsCoins:     "0x0",
					BaseBlockRewardsCoins: "0x0",
					TxnFeeRewardsCoins:    "0x0",
					TxnFeeBurntCoins:      "0x0",
					SlashedCoins:          "0x0",
				}
			}
		} else {
			log.Error("GetLastBlockByDb", "err", err.Error())
			return err
		}
	} else {
		if c.enableExtendedApis {
			runningSummary, err = c.getSummaryFromDb()
			if err != nil {
				log.Error("getSummaryFromDb", "err", err.Error())
				return err
			}
		}
	}
	log.Info("runningSummary", runningSummary)

	c.blockChan = make(chan *PrimordialBlockData, 25)
	blockNumber = blockNumber + 1

	go func() {
		c.clientInitialize()
		c.processPendingTransactions()
		c.closeLoop()
		delayNumber := int64(100 * time.Millisecond)
		blockTimer := time.NewTimer(time.Duration(delayNumber))

		log.Info("start block loop")
		for {
			select {
			case <-blockTimer.C:
				internalBlockData, err := c.primordialCache.GetBlock(blockNumber)
				if err != nil {
					if err.Error() == LevelDbNoTFoundErrMsg {
						log.Info("Waiting for PrimordialBlock...", "PrimordialBlock number", blockNumber)
					} else {
						log.Error("GetBlock Error", "error", err.Error(), "PrimordialBlock number", blockNumber)
					}
					delayNumber = int64(3 * time.Second)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}
				err = c.processByCacheManager(internalBlockData, runningSummary)
				if err == nil {
					blockNumber = blockNumber + 1
					log.Info("Batch Complete", "PrimordialBlock number", blockNumber)
					delayNumber = 0
				} else {
					log.Error("Batch Error", "error", err.Error(), "PrimordialBlock number", blockNumber)
					delayNumber = int64(3 * time.Second)
					continue
				}
				blockTimer.Reset(time.Duration(delayNumber))

			case <-cancel:
				log.Warn("start() Quit signal received")
				blockTimer.Stop()
				return
			}
		}
	}()

	log.Info("start done")

	return nil
}

func (c *CacheManager) closeLoop() {
	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)
	delayNumber := int64(100 * time.Millisecond)
	closeTimer := time.NewTimer(time.Duration(delayNumber))

	go func() {
		for {
			select {
			case <-closeTimer.C:
				closeTimer.Reset(time.Duration(delayNumber))

			case <-cancel:
				log.Warn("closeLoop quit signal received")
				closeTimer.Stop()
				c.close()
				return
			}
		}
	}()
}

func (c *CacheManager) processByCacheManager(internalBlockData *PrimordialBlockData, runningSummary *BlockchainDetails) error {
	var err error
	block := internalBlockData.Block
	blockNum := block.Number
	blockNumber := blockNum.Uint64()

	log.Info("processByCacheManager", "blockNumber", blockNumber)

	txnBatch := c.cacheDb.NewBatch()
	err = c.putLastBlockNumberInDb(blockNumber, &txnBatch)
	if err != nil {
		log.Error("processByCacheManagerputLastBlockNumberInDb", "error", err)
		return err
	}

	blockInfo := fromPrimordialBlockData(internalBlockData)
	err = c.putBlockInDb(blockInfo, &txnBatch)
	if err != nil {
		log.Error("putBlockInDb", "error", err)
		return err
	}

	liveAccountTxnMap := make(map[string][]*TransactionDetails) //address to transactions in block mapping

	txnMap := make(map[string]*TransactionDetailsExpanded)
	if internalBlockData.TransactionList != nil {
		for _, txn := range internalBlockData.TransactionList {
			txnMap[txn.Receipt.TxHash] = txn
		}
	}

	receipts := make([]*PrimordialReceipt, len(internalBlockData.TransactionList))

	for i, tx := range internalBlockData.TransactionList {
		log.Trace("processByCacheManager", "transaction", tx.Transaction.Hash)
		accountsInvolved := make(map[string]bool)

		txHash := tx.Transaction.Hash
		txnFromMap, ok := txnMap[txHash]
		if ok == false {
			log.Error("processByCacheManager txn not found in map", "hash", txHash)
			return errors.New("unexpected transaction not found")
		}
		receipt := txnFromMap.Receipt
		receipts[i] = receipt

		var transaction TransactionDetails

		transaction.Hash = tx.Transaction.Hash
		transaction.BlockHash = block.Hash
		transaction.BlockNumber = blockNumber
		transaction.Origin = tx.Transaction.From
		transaction.From = tx.Transaction.From
		if tx.Transaction.To != nil {
			transaction.To = *tx.Transaction.To
		}
		transaction.Gas = common.BigIntToHexString(big.NewInt(0).SetUint64(tx.Transaction.Gas))
		transaction.GasPrice = common.BigIntToHexString(tx.Transaction.GasPrice)
		if tx.Transaction.Data != nil {
			transaction.Data = make([]byte, len(tx.Transaction.Data))
			copy(transaction.Data, tx.Transaction.Data)
		}
		transaction.Nonce = tx.Transaction.Nonce
		transaction.Value = common.BigIntToHexString(tx.Transaction.Value)
		transaction.Receipt = TransactionReceipt{
			CumulativeGasUsed: common.BigIntToHexString(big.NewInt(0).SetUint64(receipt.CumulativeGasUsed)),
			EffectiveGasPrice: transaction.GasPrice,
			GasUsed:           common.BigIntToHexString(big.NewInt(0).SetUint64(receipt.GasUsed)),
			ContractAddress:   receipt.ContractAddress,
			Hash:              tx.Transaction.Hash,
			Type:              common.BigIntToHexString(big.NewInt(0).SetUint64(uint64(receipt.Type))),
		}
		if receipt.Status == 0 {
			transaction.Receipt.Status = "0x0"
		} else if receipt.Status == 1 {
			transaction.Receipt.Status = "0x1"
		} else {
			return errors.New("unexpected transaction receipt value")
		}
		//Timestamp
		tm := time.Unix(int64(block.Time), 0)
		transaction.CreatedAt = tm.UTC().Format(TimeLayout)
		gasUsed := big.NewInt(0).SetUint64(receipt.GasUsed)
		txnFee := common.SafeMulBigInt(gasUsed, tx.Transaction.GasPrice)
		log.Debug("transaction", "gasUsed", gasUsed, "txnFee", txnFee, "hash", txHash)
		transaction.TxnFee = common.BigIntToHexString(txnFee)

		txType, err := c.getTransactionType(tx.Transaction, receipt, blockNum, &txnBatch)
		if err != nil {
			log.Error("getTransactionType", "error", err, "tx", tx.Transaction.Hash)
			return err
		}
		transaction.TransactionType = string(txType)

		if receipt.Status == 1 {
			internalTxnList := txnFromMap.InternalTransactions

			for _, iTxn := range internalTxnList {
				accountsInvolved[strings.ToLower(iTxn.From)] = true
				accountsInvolved[strings.ToLower(iTxn.To)] = true

				if strings.ToUpper(iTxn.Type) == "CREATE" || strings.ToUpper(iTxn.Type) == "CREATE2" {
					tokenDetails, err := c.client.GetTokenDetails(common.HexToAddress(iTxn.To), blockNum)
					if err != nil {
						if errors.Is(err, ethclient.NotATokenError) {
							continue
						} else {
							return err
						}
					}
					//new token created via internal transaction
					tkn := &TokenDetails{
						ContractAddress:        strings.ToLower(iTxn.To),
						CreatorAddress:         strings.ToLower(iTxn.From),
						CreatedTransactionHash: tx.Transaction.Hash,
						CreatedBlockNumber:     receipt.BlockNumber.Uint64(),
						Name:                   tokenDetails.Name,
						Symbol:                 tokenDetails.Symbol,
						TotalSupply:            hexutil.EncodeBig(tokenDetails.TotalSupply),
						Decimals:               hexutil.EncodeUint64(uint64(tokenDetails.Decimals)),
					}

					err = c.putTokenInDb(tkn, &txnBatch)
					if err != nil {
						log.Error("putTokenInDb", "error", err, "contractAddress", iTxn.To)
						return err
					}
				}
			}

			//Find all relevant token and internal token transactions, if any
			tokenTransfers, tokenApprovals, err := ParseTokenTransaction(tx.Transaction, receipt)
			if err != nil {
				log.Error("ParseTokenTransaction", "err", err, "txn", tx.Transaction.Hash)
				return err
			}

			if tokenTransfers != nil && len(tokenTransfers) > 0 {
				err = c.processAccountTokenTransfers(tokenTransfers, blockNum, &txnBatch)
				if err != nil {
					log.Error("processAccountTokenTransfers", "err", err, "txn", tx.Transaction.Hash)
					return err
				}
				for _, transfer := range tokenTransfers {
					accountsInvolved[transfer.ContractAddress.HexLower()] = true
					accountsInvolved[transfer.From.HexLower()] = true
					accountsInvolved[transfer.To.HexLower()] = true
				}
			}

			if tokenApprovals != nil && len(tokenApprovals) > 0 {
				for _, approval := range tokenApprovals {
					accountsInvolved[approval.ContractAddress.HexLower()] = true
					accountsInvolved[approval.TokenOwner.HexLower()] = true
					accountsInvolved[approval.Spender.HexLower()] = true
				}
			}

			if transaction.TransactionType == string(TOKEN_TRANSFER) { //only root level transaction (no internal transactions)
				//First transfer is root level by from address
				if tokenTransfers == nil || len(tokenTransfers) == 0 || tokenTransfers[0].From.HexLower() != transaction.From {
					log.Error("processByCacheManager missing", "from", transaction.From, "to (contract)", transaction.To)
					transaction.TransactionType = string(SMART_CONTRACT)
				} else {
					transaction.TokenTransaction.ContractAddress = tokenTransfers[0].ContractAddress.HexLower()
					transaction.TokenTransaction.TokenFromAddress = tokenTransfers[0].From.HexLower()
					transaction.TokenTransaction.TokenToAddress = tokenTransfers[0].To.HexLower()

					tokenDetails, err := c.getTokenDetailsInternal(transaction.TokenTransaction.ContractAddress) //token should already have been saved to db, when it was created
					if err != nil {
						log.Error("getTokenDetailsInternal", "error", err)
						return err
					}
					transaction.TokenTransaction.TokenCount = hexutil.EncodeBig(tokenTransfers[0].Tokens)
					transaction.TokenTransaction.TokenName = tokenDetails.Name
					transaction.TokenTransaction.TokenSymbol = tokenDetails.Symbol
				}
			}
		}

		_, ok = liveAccountTxnMap[tx.Transaction.From]
		if ok == false {
			liveAccountTxnMap[tx.Transaction.From] = make([]*TransactionDetails, 0)
		}
		liveAccountTxnMap[tx.Transaction.From] = append(liveAccountTxnMap[tx.Transaction.From], &transaction)

		if tx.Transaction.To != nil {
			if tx.Transaction.From != *tx.Transaction.To {
				_, ok = liveAccountTxnMap[*tx.Transaction.To]
				if ok == false {
					liveAccountTxnMap[*tx.Transaction.To] = make([]*TransactionDetails, 0)
				}
				liveAccountTxnMap[*tx.Transaction.To] = append(liveAccountTxnMap[*tx.Transaction.To], &transaction)
			}
			accountsInvolved[*tx.Transaction.To] = true
		} else {
			accountsInvolved[strings.ToLower(receipt.ContractAddress)] = true
		}
		accountsInvolved[tx.Transaction.From] = true

		//Loop through all accounts and update account cache
		for account, _ := range accountsInvolved {
			_, err = c.primordialCache.getAccountFromCacheOrDb(account)
			if err != nil {
				return err
			}
		}
	}

	for k, v := range liveAccountTxnMap {
		err = c.processAccountTransactions(k, &v, &txnBatch)
		if err != nil {
			log.Error("processAccountTransaction", "error", err, "address", k)
			return err
		}
	}

	if c.enableExtendedApis {
		err = c.updateSummary(internalBlockData, runningSummary, &txnBatch)
		if err != nil {
			log.Error("updateSummary", "error", err)
			return err
		}
	}

	err = txnBatch.Write()
	if err != nil {
		log.Error("processByCacheManager txnBatch Write", "error", err)
		return err
	}

	return nil
}

func (c *CacheManager) updateSummary(internalBlockData *PrimordialBlockData, runningSummary *BlockchainDetails, batch *ethdb.Batch) error {

	blockNumber := internalBlockData.Block.Number.Uint64()
	leftBlock := blockNumber
	rightBlock := runningSummary.BlockNumber + 1
	if leftBlock != rightBlock {
		log.Error("updateSummary", "leftBlock", leftBlock, "rightBlock", rightBlock)
		return errors.New("updateSummary unexpected blockNumber")
	}

	consensusData := internalBlockData.ConsensusData

	txnBatch := *batch
	blockRewardsInfo := consensusData.BlockRewardsInfo

	var baseBlockProposerRewards *big.Int
	var blockProposerRewards *big.Int
	var txnFeeRewards *big.Int
	var burntTxnFee *big.Int
	var slashAmount *big.Int
	var err error

	//Update running summary
	runningSummary.BlockNumber = blockNumber

	if len(blockRewardsInfo.BaseBlockProposerRewards) > 0 {
		baseBlockProposerRewards, err = hexutil.DecodeBig(blockRewardsInfo.BaseBlockProposerRewards)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		baseBlockRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.BaseBlockRewardsCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary baseBlockRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.BaseBlockRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(baseBlockRewardsCoinsBig, baseBlockProposerRewards))
	}

	if len(blockRewardsInfo.BlockProposerRewards) > 0 {
		blockProposerRewards, err = hexutil.DecodeBig(blockRewardsInfo.BlockProposerRewards)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		blockRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.BlockRewardsCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary blockRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.BlockRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(blockRewardsCoinsBig, blockProposerRewards))
	}

	if len(blockRewardsInfo.TxnFeeRewards) > 0 {
		txnFeeRewards, err = hexutil.DecodeBig(blockRewardsInfo.TxnFeeRewards)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		txnFeeRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.TxnFeeRewardsCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary txnFeeRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.TxnFeeRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(txnFeeRewardsCoinsBig, txnFeeRewards))
	}

	if len(blockRewardsInfo.BurntTxnFee) > 0 {
		burntTxnFee, err = hexutil.DecodeBig(blockRewardsInfo.BurntTxnFee)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		txnFeeBurntCoinsBig, err := hexutil.DecodeBig(runningSummary.TxnFeeBurntCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary txnFeeBurntCoinsBig", "error", err)
			return err
		}
		runningSummary.TxnFeeBurntCoins = hexutil.EncodeBig(common.SafeAddBigInt(txnFeeBurntCoinsBig, burntTxnFee))
	}

	if len(blockRewardsInfo.SlashAmount) > 0 {
		slashAmount, err = hexutil.DecodeBig(blockRewardsInfo.SlashAmount)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		slashedCoinsBig, err := hexutil.DecodeBig(runningSummary.SlashedCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary slashedCoinsBig", "error", err)
			return err
		}
		runningSummary.SlashedCoins = hexutil.EncodeBig(common.SafeAddBigInt(slashedCoinsBig, slashAmount))
	}

	//Get latest burnt coins info
	burntCoinsWei := internalBlockData.ZeroAddressBalance

	runningSummary.BurntCoins = hexutil.EncodeBig(burntCoinsWei)
	genesisCirculatingSupplyBig, _ := hexutil.DecodeBig(c.genesisCirculatingSupply)
	blockRewardsCoinsBig, _ := hexutil.DecodeBig(runningSummary.BlockRewardsCoins)
	coinsNew := common.SafeAddBigInt(genesisCirculatingSupplyBig, blockRewardsCoinsBig)
	runningSummary.CirculatingSupply = hexutil.EncodeBig(common.SafeSubBigInt(coinsNew, burntCoinsWei))
	runningSummary.TotalSupply = runningSummary.CirculatingSupply

	err = c.putSummary(runningSummary, &txnBatch)
	if err != nil {
		log.Error("updateSummary putSummary", "error", err)
		return err
	}

	return nil
}

func (c *CacheManager) latestBlockByNode() (uint64, error) {

	client, err := ethclient.Dial(c.nodeUrl)
	if err != nil {
		return 0, err
	}

	latestBlock, err := client.BlockNumber(context.Background())
	if err != nil {
		return 0, err
	}

	log.Info("latestBlockByNode", "number", latestBlock)

	return latestBlock, nil

}

func (c *CacheManager) close() error {
	close(c.blockChan)
	c.pendingTxLock.Lock()
	defer c.pendingTxLock.Unlock()
	c.pendingTxClient.Close()

	cacheDb := c.cacheDb
	err := cacheDb.Close()
	if err != nil {
		log.Debug("cache manager account transaction db close error", "err", err)
		return err
	}
	c.client.Close()
	c.blockClient.Close()
	c.primordialCache.Close()
	log.Info("CacheManager closed")
	os.Exit(1)
	return nil
}

func (c *CacheManager) processAccountTransactions(address string, txnList *[]*TransactionDetails, batch *ethdb.Batch) error {
	txnBatch := *batch
	var txnCount uint64
	var err error

	address = strings.ToLower(address)

	txnCount, err = c.getAccountTxnCount(address)
	if err != nil {
		return err
	}
	newTxnCount := txnCount + 1
	var accountTransactionList AccountTransactionList

	log.Info("processAccountTransactions", "address", address, "txnCount", txnCount, "transaction count in block", len(*txnList))

	if newTxnCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
		accountTransactionList.Transactions = make([]AccountTransactionCompact, 0)
		accountTransactionList.Address = address
		log.Info("processAccountTransactions", "address", address, "newTxnCount", newTxnCount)
	} else {
		//Load current state form the cache
		txnPageCount := getPageCount(newTxnCount)
		txnPageKey := getAccountTransactionPageKey(address, txnPageCount)

		log.Info("processAccountTransactions loading from cache", "address", address, "newTxnCount", newTxnCount, "txnPageCount", txnPageCount)

		accountTransactionListBlob, err := c.cacheDb.Get(txnPageKey)
		if err != nil {
			log.Error("cacheDb.Get accountTxnPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(accountTransactionListBlob, &accountTransactionList)
		if err != nil {
			log.Error("json.Unmarshal accountTransactionListBlob", "error", err)
			return err
		}

		if strings.ToLower(accountTransactionList.Address) != address {
			return errors.New("unexpected address")
		}

		if accountTransactionList.Transactions == nil {
			return errors.New("unexpected transactions is nul")
		}

		if len(accountTransactionList.Transactions) != int(txnCount%PageSize) {
			log.Error("unexpected transactions count from address", "actual", len(accountTransactionList.Transactions), "expected", int(txnCount%PageSize), "txnCount", txnCount)
			return errors.New("unexpected transactions count")
		}
	}

	for i, txn := range *txnList {
		atxn := accountTransactionCompactFromTransaction(txn)
		accountTransactionList.Transactions = append([]AccountTransactionCompact{atxn}, accountTransactionList.Transactions...) //prepend for backward compat

		if len(accountTransactionList.Transactions) == int(PageSize) || i == len(*txnList)-1 {
			accountTransactionListBlob, err := json.Marshal(accountTransactionList)
			if err != nil {
				log.Error("json.Marshal accountTransactionListBlob", "error", err)
				return err
			}

			runningTxnCount := txnCount + uint64(i) + 1
			txnPageCount := getPageCount(runningTxnCount)
			txnPageKey := getAccountTransactionPageKey(address, txnPageCount)
			err = txnBatch.Put(txnPageKey, accountTransactionListBlob)
			if err != nil {
				log.Error("txnBatch.Put accountTransactionListBlob", "error", err)
				return err
			}
			log.Info("txnBatch.Put", "runningTxnCount", runningTxnCount, "txnPageCount", txnPageCount)
			accountTransactionList.Transactions = make([]AccountTransactionCompact, 0) //reset
		}
	}

	txnCount = txnCount + uint64(len(*txnList))
	err = c.putAccountTxnCount(address, txnCount, batch)
	if err != nil {
		return err
	}

	log.Info("inserted account txn list", "txnCount", txnCount, "txnPageCount", getPageCount(txnCount), "txnCountInBlock", len(*txnList), "address", address)

	return nil
}

func getPageCount(itemCount uint64) uint64 {
	if itemCount%PageSize == 0 {
		return itemCount / PageSize
	} else {
		return (itemCount / PageSize) + 1
	}
}

func (c *CacheManager) getTransactionType(txn *PrimordialTransaction, receipt *PrimordialReceipt, blockNumber *big.Int, batch *ethdb.Batch) (TransactionType, error) {
	txHash := txn.Hash
	if (txHash == receipt.TxHash) == false {
		return "", errors.New("hash mismatch between txn and receipt")
	}

	if txn.To != nil {
		if receipt.Status == 0 {
			return COIN_TRANSFER, nil //todo: fix
		}

		acc, err := c.primordialCache.getAccountFromCacheOrDb(*txn.To)
		if err != nil {
			return "", err
		}
		if acc.AccType == ethclient.ACCOUNT_TYPE_REGULAR {
			return COIN_TRANSFER, nil
		} else {
			isTokenTransfer, err := IsMainTransactionTokenTransfer(txn, receipt)
			if err != nil {
				return "", err
			}
			if isTokenTransfer {
				return TOKEN_TRANSFER, nil
			} else {
				return SMART_CONTRACT, nil
			}
		}
	} else {
		if receipt.Status == 1 {
			acc, err := c.primordialCache.getAccountFromCacheOrDb(*txn.To)
			if err != nil {
				return "", err
			}
			if acc.AccType == ethclient.ACCOUNT_TYPE_TOKEN {
				return NEW_TOKEN, nil
			} else if acc.AccType == ethclient.ACCOUNT_TYPE_CONTRACT {
				return NEW_SMART_CONTRACT, nil
			} else {
				return "", errors.New("unexpected account type")
			}
		} else {
			return NEW_SMART_CONTRACT, nil
		}
	}
}

// Parses and stores list of tokens and balance for each token
func (c *CacheManager) processAccountTokenTransfers(tokenTransfers []*LogTransfer, blockNum *big.Int, batch *ethdb.Batch) error {
	for _, t := range tokenTransfers {
		contractAddress := t.ContractAddress.HexLower()
		tokenDetails, err := c.getTokenDetailsInternal(contractAddress)
		if err != nil {
			log.Error("processAccountTokenTransfers getTokenDetailsInternal", "contractAddress", contractAddress, "error", err)
			return err
		}

		tokenBalance, err := c.client.GetAccountTokenBalance(t.ContractAddress, t.From)
		if err != nil {
			if errors.Is(err, ethclient.NotATokenError) {
				log.Error("processAccountTokenTransfers GetAccountTokenBalance from", "contractAddress", contractAddress, "error", err, "from", t.From)
				return err
			}
		} else {
			err = c.processAccountTokenTransfer(t.From, tokenDetails, tokenBalance, batch)
			if err != nil {
				log.Error("processAccountTokenTransfers processAccountTokenTransfer from", "contractAddress", contractAddress, "error", err, "from", t.From)
				return err
			}
		}

		tokenBalance, err = c.client.GetAccountTokenBalance(t.ContractAddress, t.To)
		if err != nil {
			if errors.Is(err, ethclient.NotATokenError) {
				log.Error("processAccountTokenTransfers GetAccountTokenBalance from", "contractAddress", contractAddress, "error", err, "to", t.To)
				return err
			}
		} else {
			err = c.processAccountTokenTransfer(t.To, tokenDetails, tokenBalance, batch)
			if err != nil {
				log.Error("processAccountTokenTransfers processAccountTokenTransfer to", "contractAddress", contractAddress, "error", err, "to", t.To)
				return err
			}
		}
	}
	return nil
}

func (c *CacheManager) processAccountTokenTransfer(addr common.Address, tokenDetails *TokenDetails, tokenBalance *big.Int, batch *ethdb.Batch) error {
	txnBatch := *batch
	var tokenCount uint64
	var err error

	address := addr.HexLower()
	contractAddress := strings.ToLower(tokenDetails.ContractAddress)

	accountTokenSummary := AccountTokenSummary{
		AccountAddress:  address,
		ContractAddress: contractAddress,
		Name:            tokenDetails.Name,
		Symbol:          tokenDetails.Symbol,
		TokenBalance:    hexutil.EncodeBig(tokenBalance),
	}

	err = c.putAccountTokenInDb(&accountTokenSummary, &txnBatch)
	if err != nil {
		log.Error("putAccountTokenInDb", "address", address, "contractAddress", contractAddress)
		return err
	}

	//Check if already inserted
	tempMap, ok := c.accountTokenMap[address]
	if ok == true {
		cMap := *tempMap
		_, ok = cMap[contractAddress]
		if ok {
			return nil
		}
	}

	tokenCount, err = c.getAccountTokenCount(address)
	if err != nil {
		return err
	}
	newTokenCount := tokenCount + 1

	accountTokenList := AccountTokenList{}

	log.Info("processAccountTokenTransfer", "address", address, "tokenCount", tokenCount, "newTokenCount", newTokenCount)

	if newTokenCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
		accountTokenList.Tokens = make([]AccountTokenSummary, 0)
		accountTokenList.Address = address
		log.Info("processAccountTokenTransfer", "address", address, "newTokenCount", newTokenCount)
	} else {
		//Load current state form the cache
		tokenPageCount := getPageCount(newTokenCount)
		pageKey, tokenPageKey := getAccountTokenPageKey(address, tokenPageCount)

		log.Debug("processAccountTokenTransfer loading from cache", "address", address, "newTokenCount", newTokenCount, "tokenPageCount", tokenPageCount, "pageKey", pageKey)

		accountTokenListBlob, err := c.cacheDb.Get(tokenPageKey)
		if err != nil {
			log.Error("cacheDb.Get tokenPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(accountTokenListBlob, &accountTokenList)
		if err != nil {
			log.Error("json.Unmarshal accountTokenListBlob", "error", err, "address", address, "pageKey", pageKey)
			return err
		}

		if strings.ToLower(accountTokenList.Address) != address {
			return errors.New("unexpected address")
		}

		if accountTokenList.Tokens == nil {
			return errors.New("unexpected tokens is nul")
		}

		if len(accountTokenList.Tokens) != int(tokenCount%PageSize) {
			log.Error("unexpected token count from address", "actual", len(accountTokenList.Tokens), "expected", int(tokenCount%PageSize), "tokenCount", tokenCount)
			return errors.New("unexpected transactions count")
		}

		//todo: make this work for pages greater than 1
		for _, t := range accountTokenList.Tokens {
			if t.ContractAddress == contractAddress { //token already in list
				return nil
			}
		}
	}

	accountTokenList.Tokens = append(accountTokenList.Tokens, accountTokenSummary)
	accountTokenListBlob, err := json.Marshal(accountTokenList)
	if err != nil {
		log.Error("json.Marshal accountTokenListBlob", "error", err)
		return err
	}

	tokenPageCount := getPageCount(newTokenCount)
	log.Debug("tokenPageCount", tokenPageCount, "newTokenCount", newTokenCount)
	key, tokenPageKeyBlob := getAccountTokenPageKey(address, tokenPageCount)
	err = txnBatch.Put(tokenPageKeyBlob, accountTokenListBlob)
	if err != nil {
		log.Error("txnBatch.Put accountTokenListBlob", "error", err, "key", key)
		return err
	}

	err = c.putAccountTokenCount(address, newTokenCount, batch)
	if err != nil {
		return err
	}

	contractMap, ok := c.accountTokenMap[address]
	if ok == false {
		cMap := make(map[string]bool)
		contractMap = &cMap
	}
	conMap := *contractMap
	conMap[contractAddress] = true
	c.accountTokenMap[address] = contractMap

	log.Info("inserted account token list", "newTokenCount", newTokenCount, "tokenPageCount", tokenPageCount, "address", address)

	return nil
}
