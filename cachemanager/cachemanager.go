package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/core"
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/params"
	"github.com/QuantumCoinProject/qc/systemcontracts/conversion"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking"
	"github.com/QuantumCoinProject/qc/token"
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
	enableExtendedApis       bool
	genesisCirculatingSupply string
	maxSupply                string
	pendingTxLock            sync.Mutex
	pendingTxMapLock         sync.RWMutex
	pendingTransactions      *map[string]map[string]map[string]*ethclient.TxPoolTransaction
	addressMap               map[string]*AccountDetails
}

var SummaryKey = "summary"
var LastBlockKey = "last-block"
var AccountSummaryKey = "account-%s"
var AccountTxnCountKey = "account-txn-count-%s"                  //%s is account address
var AccountTransactionPageKey = "account-transaction-list-%s-%d" //%s is account address, %d is page number
var TokenDetailsKey = "erc20-%s"

var AccountTokenCountKey = "account-token-count-%s"  //%s is account address
var AccountTokenPageKey = "account-token-list-%s-%d" //%s is account address, %d is page number

var AccountTokenTxnCountKey = "account-token-txn-count-%s"          //%s is account address
var AccountTokenTransactionPageKey = "account-token-txn-list-%s-%d" //%s is account address,%d is page number

var chainID *big.Int
var LevelDbNoTFoundErrMsg = "leveldb: not found"
var NotFoundErrMsg = "not found"

const TimeLayout = "2006-01-02T15:04:05Z"

const PageSize uint64 = 20

type AccountTransactionList struct {
	Address      string                      `json:"address"`
	Transactions []AccountTransactionCompact `json:"transactions"`
}

type TransactionType string

// List of TransactionType
const (
	COIN_TRANSFER      TransactionType = "CoinTransfer"
	NEW_TOKEN          TransactionType = "NewToken"
	TOKEN_TRANSFER     TransactionType = "TokenTransfer"
	NEW_SMART_CONTRACT TransactionType = "NewSmartContract"
	SMART_CONTRACT     TransactionType = "SmartContract"
)

type ContractType string

type TokenDetails struct {
	ContractAddress        string `json:"contractAddress,omitempty"`
	CreatorAddress         string `json:"creatorAddress,omitempty"`
	CreatedBlockNumber     uint64 `json:"createdBlockNumber,omitempty"`
	CreatedTransactionHash string `json:"createdTransactionHash,omitempty"`
	Name                   string `json:"name,omitempty"`
	Symbol                 string `json:"symbol,omitempty"`
	TotalSupply            string `json:"totalSupply,omitempty"`
	Decimals               string `json:"decimals,omitempty"`
}

type GetTokenDetailsResponse struct {
	Result TokenDetails `json:"result,omitempty"`
}

type AccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
}

type AccountTransactionCompact struct {
	Hash string `json:"hash,omitempty"`

	BlockNumber uint64 `json:"blockNumber,omitempty"`

	CreatedAt string `json:"createdAt,omitempty"`

	From string `json:"from,omitempty"`

	To string `json:"to,omitempty"`

	Value string `json:"value,omitempty"`

	TxnFee string `json:"txnFee,omitempty"`

	Status string `json:"status,omitempty"`

	TransactionType string `json:"transactionType,omitempty"`
}

type AccountTokenTransactionCompact struct {
	AccountTransactionCompact

	Tokens string `json:"tokens,omitempty"`

	TokenSymbol string `json:"tokenSymbol,omitempty"`

	TokenName string `json:"tokenName,omitempty"`
}

type ListAccountTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount"`
	Items     []AccountTransactionCompact `json:"items"`
}

type AccountPendingTransactionCompact struct {
	Hash string `json:"hash,omitempty"`

	From string `json:"from,omitempty"`

	To string `json:"to,omitempty"`

	Value string `json:"value,omitempty"`

	Nonce uint64 `json:"nonce,omitempty"`
}

type ListAccountPendingTransactionsResponse struct {
	Items     []AccountPendingTransactionCompact `json:"items"`
	PageCount uint64                             `json:"pageCount"`
}

type BlockchainDetails struct {
	BlockNumber           uint64 `json:"blockNumber" gencodec:"required"`
	MaxSupply             string `json:"maxSupply" gencodec:"required"`
	TotalSupply           string `json:"totalSupply" gencodec:"required"`
	CirculatingSupply     string `json:"circulatingSupply" gencodec:"required"`
	BurntCoins            string `json:"burntCoins" gencodec:"required"`
	BlockRewardsCoins     string `json:"blockRewardsCoins" gencodec:"required"` //baseBlockRewardsCoins + TxnFeeRewardsCoins
	BaseBlockRewardsCoins string `json:"baseBlockRewardsCoins" gencodec:"required"`
	TxnFeeRewardsCoins    string `json:"txnFeeRewardsCoins" gencodec:"required"`
	TxnFeeBurntCoins      string `json:"txnFeeBurntCoins" gencodec:"required"`
	SlashedCoins          string `json:"slashedCoins" gencodec:"required"`
}

type GetBlockchainDetailsResponse struct {
	BlockchainDetails
}

type AccountTokenSummary struct {
	AccountAddress  string `json:"accountAddress,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"`
	Name            string `json:"name,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	TokenBalance    string `json:"tokenBalance,omitempty"`
}

func NewCacheManager(cacheDir string, nodeUrl string, enableExtendedApis bool, genesisFilePath string, maxSupply string) (*CacheManager, error) {
	cManager := &CacheManager{
		nodeUrl:            nodeUrl,
		cacheDir:           cacheDir,
		enableExtendedApis: enableExtendedApis,
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
	catchManager, err := rawdb.NewLevelDBDatabase(catchManagerFilePath, 64, 0, "", false)
	if err != nil {
		return err
	}
	c.cacheDb = catchManager

	client, err := ethclient.Dial(c.nodeUrl)
	if err != nil {
		return err
	}

	pendingTxClient, err := ethclient.Dial(c.nodeUrl)
	if err != nil {
		return err
	}

	chainID, err = client.NetworkID(context.Background())
	if err != nil {
		log.Error("initialize NetworkID", "error", err)
		return err
	}

	c.client = client
	c.pendingTxClient = pendingTxClient
	c.addressMap = make(map[string]*AccountDetails)
	c.addressMap[staking.STAKING_CONTRACT] = &AccountDetails{Address: staking.STAKING_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}
	c.addressMap[conversion.CONVERSION_CONTRACT] = &AccountDetails{Address: conversion.CONVERSION_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}

	return nil
}

func (c *CacheManager) start() error {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	var runningSummary *BlockchainDetails

	blockNumber, err := c.getLastBlockNumberByDb(LastBlockKey)
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

	delayNumber := int64(100 * time.Millisecond)
	cacheTimer := time.NewTimer(time.Duration(delayNumber))
	blockNumber = 14321

	go func() {
		for {
			select {
			case <-cacheTimer.C:
				go c.processPendingTransactions()

				blockNumberToGet := blockNumber + 1
				//log.Info("Batch Start ", "Block Number ", blockNumberToGet)
				err := c.processByCacheManager(blockNumberToGet, runningSummary)
				if err == nil {
					blockNumber = blockNumberToGet
					//log.Info("Batch Complete", "Block number", blockNumberToGet)
					delayNumber = 0
				} else {
					if err.Error() == NotFoundErrMsg {
						log.Info("Waiting for Block...", "Block number", blockNumberToGet)
					} else {
						log.Error("Batch Error", "error", err.Error(), "Block number", blockNumberToGet)
					}
					delayNumber = int64(5 * time.Second)
				}

				cacheTimer.Reset(time.Duration(delayNumber))
			case <-cancel:
				cacheTimer.Stop()
				err = c.close()
				if err != nil {
					log.Error("c.close()", "error", err)
				}
				log.Info("Quit signal received")
				os.Exit(1)
				return
			}
		}
	}()

	return nil
}

func (c *CacheManager) processPendingTransactions() {
	c.pendingTxLock.Lock()
	defer c.pendingTxLock.Unlock()

	err, txnList := c.pendingTxClient.TxPoolContent(context.Background())
	if err != nil {
		log.Error("processPendingTransactions", "err", err)
		return
	}

	if txnList == nil {
		log.Warn("processPendingTransactions txnList is nil")
		return
	}

	c.pendingTxMapLock.Lock()
	defer c.pendingTxMapLock.Unlock()

	c.pendingTransactions = txnList
}

func (c *CacheManager) processByCacheManager(blockNumber uint64, runningSummary *BlockchainDetails) error {
	blockNum := new(big.Int).SetUint64(blockNumber)
	block, err := c.client.BlockByNumber(context.Background(), blockNum)
	if err != nil {
		if err.Error() != NotFoundErrMsg {
			log.Error("BlockByNumber", "error", err)
		}
		return err
	}

	txnBatch := c.cacheDb.NewBatch()
	blockKey := []byte(LastBlockKey)
	err = txnBatch.Put(blockKey, common.Uint64ToBytes(blockNumber))
	if err != nil {
		log.Error("processByCacheManager txnBatch.Put", "error", err)
		return err
	}

	liveAccountTxnMap := make(map[string][]AccountTransactionCompact) //address to transactions in block mapping

	var receipts types.Receipts
	receipts = make(types.Receipts, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		accountsInvolved := make(map[string]bool)

		receipt, err := c.client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			log.Error("processByCacheManager TransactionReceipt", "error", err)
			return err
		}
		receipts[i] = receipt

		msg, err := tx.AsMessage(types.NewLondonSigner(chainID))
		if err != nil {
			log.Error("processByCacheManager AsMessage", "error", err)
			return err
		}

		fromAddress := strings.ToLower(msg.From().Hex())
		var toAddress string
		if tx.To() != nil {
			toAddress = strings.ToLower(tx.To().Hex())
		}

		var transaction AccountTransactionCompact

		transaction.Hash = tx.Hash().Hex()
		transaction.BlockNumber = blockNumber

		//Timestamp
		tm := time.Unix(int64(block.Time()), 0)
		transaction.CreatedAt = tm.UTC().Format(TimeLayout)

		transaction.From = fromAddress
		transaction.To = toAddress
		transaction.Value = common.BigIntToHexString(tx.Value())

		gasUsed := big.NewInt(1).SetUint64(receipt.GasUsed)
		txnFee := common.SafeMulBigInt(gasUsed, tx.GasPrice())
		transaction.TxnFee = common.BigIntToHexString(txnFee)

		if receipt.Status == 1 {
			transaction.Status = "0x1"
		} else {
			transaction.Status = "0x0"
		}

		txType, err := c.getTransactionType(tx, receipt, blockNum, &txnBatch)
		if err != nil {
			log.Error("getTransactionType", "error", err, "tx", tx.Hash())
			return err
		}
		transaction.TransactionType = string(txType)

		_, ok := liveAccountTxnMap[fromAddress]
		if ok == false {
			liveAccountTxnMap[fromAddress] = make([]AccountTransactionCompact, 0)
		}
		liveAccountTxnMap[fromAddress] = append(liveAccountTxnMap[fromAddress], transaction)

		if tx.To() != nil {
			if fromAddress != toAddress {
				_, ok = liveAccountTxnMap[toAddress]
				if ok == false {
					liveAccountTxnMap[toAddress] = make([]AccountTransactionCompact, 0)
				}
				liveAccountTxnMap[toAddress] = append(liveAccountTxnMap[toAddress], transaction)
			}
			accountsInvolved[strings.ToLower(tx.To().Hex())] = true
		}
		accountsInvolved[fromAddress] = true

		//Find all internal transactions
		internalTransactions, err := c.client.GetInternalTransactions(context.Background(), tx.Hash())
		log.Error("GetInternalTransactions", "err", err, "txn", tx.Hash())
		if err != nil {
			return err
		}

		//Created tokens
		internalTxnList := c.flattenInternalTransactionDetails(internalTransactions)
		for _, iTxn := range internalTxnList {
			accountsInvolved[strings.ToLower(iTxn.From)] = true
			accountsInvolved[strings.ToLower(iTxn.To)] = true

			if iTxn.Type == "CREATE" || iTxn.Type == "CREATE2" {
				tokenDetails, err := c.client.GetTokenDetails(common.HexToAddress(iTxn.To), blockNum)
				if err != nil {
					if errors.Is(err, token.NotATokenError) {
						continue
					} else {
						return err
					}
				}
				//new token created via internal transaction
				tkn := &TokenDetails{
					ContractAddress:        strings.ToLower(iTxn.To),
					CreatorAddress:         strings.ToLower(iTxn.From),
					CreatedTransactionHash: strings.ToLower(tx.Hash().Hex()),
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
		tokenTransfers, tokenApprovals, err := token.ParseTokenTransaction(tx, receipt)
		if err != nil {
			log.Error("ParseTokenTransaction", "err", err, "txn", tx.Hash())
			return err
		}

		if tokenTransfers != nil && len(tokenTransfers) > 0 {
			err = c.processAccountTokenTransfers(fromAddress, tokenTransfers, &txnBatch)
			if err != nil {
				log.Error("processAccountTokenTransfers", "err", err, "txn", tx.Hash(), "fromAddress", fromAddress)
				return err
			}
		}

		if tokenApprovals != nil && len(tokenApprovals) > 0 {
		}

		if tx.To() == nil {
			accountsInvolved[strings.ToLower(receipt.ContractAddress.Hex())] = true
		}

		//Loop through all accounts and update account cache
		for account, _ := range accountsInvolved {
			_, err = c.getAccount(common.HexToAddress(account), blockNum, &txnBatch)
			if err != nil {
				return err
			}
		}

		if transaction.TransactionType == string(TOKEN_TRANSFER) { //only root level transaction (no internal transactions)
			var accountTokenTransaction AccountTokenTransactionCompact
			accountTokenTransaction.Hash = transaction.Hash
			accountTokenTransaction.BlockNumber = blockNumber
			accountTokenTransaction.CreatedAt = transaction.CreatedAt
			accountTokenTransaction.From = transaction.From
			accountTokenTransaction.To = transaction.To
			accountTokenTransaction.Value = transaction.Value
			accountTokenTransaction.TxnFee = transaction.TxnFee
			accountTokenTransaction.Status = transaction.Status
			accountTokenTransaction.TransactionType = transaction.TransactionType

			tokenDetails, err := c.GetTokenDetails(accountTokenTransaction.To) //token should already have been saved tp db, when it was created
			if err != nil {
				log.Error("GetTokenDetails", "error", err)
				return err
			}
			accountTokenTransaction.TokenName = tokenDetails.Result.Name
			accountTokenTransaction.TokenSymbol = tokenDetails.Result.Symbol

			//First transfer is root level by from address
			if len(tokenTransfers) == 0 || strings.ToLower(tokenTransfers[0].From.Hex()) != transaction.From {
				log.Error("processByCacheManager missing", "from", transaction.From, "to (contract)", transaction.To)
				return errors.New("unexpected token transfer missing")
			}
			accountTokenTransaction.Tokens = hexutil.EncodeBig(tokenTransfers[0].Tokens)

			err = c.processAccountTokenTransaction(&accountTokenTransaction, &txnBatch)
			if err != nil {
				log.Error("processByCacheManager processAccountTokenTransaction", "error", "hash", transaction.Hash)
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
		err = c.updateSummary(blockNum, runningSummary, &txnBatch)
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

func (c *CacheManager) updateSummary(blockNumber *big.Int, runningSummary *BlockchainDetails, batch *ethdb.Batch) error {

	leftBlock := blockNumber.Uint64()
	rightBlock := runningSummary.BlockNumber + 1
	if leftBlock != rightBlock {
		log.Error("updateSummary", "leftBlock", leftBlock, "rightBlock", rightBlock)
		return errors.New("updateSummary unexpected blockNumber")
	}

	consensusData, err := c.client.GetBlockConsensusData(context.Background(), blockNumber)
	if err != nil {
		return err
	}

	txnBatch := *batch
	blockRewardsInfo := consensusData.BlockRewardsInfo

	var baseBlockProposerRewards *big.Int
	var blockProposerRewards *big.Int
	var txnFeeRewards *big.Int
	var burntTxnFee *big.Int
	var slashAmount *big.Int

	//Update running summary
	runningSummary.BlockNumber = blockNumber.Uint64()

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
	burntCoinsWei, err := c.client.BalanceAt(context.Background(), common.ZERO_ADDRESS, blockNumber)
	if err != nil {
		log.Error("updateSummary BalanceAt", "error", err)
		return err
	}

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

func (c *CacheManager) getLastBlockNumberByDb(blockKey string) (uint64, error) {
	db := c.cacheDb
	mySlice, err := db.Get([]byte(blockKey))
	if err != nil {
		return uint64(0), err
	}

	blockNumber := common.BytesToUint64(mySlice)

	return blockNumber, nil
}

func (c *CacheManager) getSummaryFromDb() (*BlockchainDetails, error) {
	db := c.cacheDb
	summaryBlob, err := db.Get([]byte(SummaryKey))
	if err != nil {
		return nil, err
	}

	var summary BlockchainDetails
	err = json.Unmarshal(summaryBlob, &summary)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

func (c *CacheManager) putSummary(summary *BlockchainDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	keyBlob := []byte(SummaryKey)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) close() error {
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
	return nil
}

func (c *CacheManager) processAccountTransactions(address string, txnList *[]AccountTransactionCompact, batch *ethdb.Batch) error {
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
		txnPageKey := getAccountPageKey(address, txnPageCount)

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
		accountTransactionList.Transactions = append([]AccountTransactionCompact{txn}, accountTransactionList.Transactions...) //prepend for backward compat

		if len(accountTransactionList.Transactions) == int(PageSize) || i == len(*txnList)-1 {
			accountTransactionListBlob, err := json.Marshal(accountTransactionList)
			if err != nil {
				log.Error("json.Marshal accountTransactionListBlob", "error", err)
				return err
			}

			runningTxnCount := txnCount + uint64(i) + 1
			txnPageCount := getPageCount(runningTxnCount)
			txnPageKey := getAccountPageKey(address, txnPageCount)
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

func getPageCount(txnCount uint64) uint64 {
	if txnCount%PageSize == 0 {
		return txnCount / PageSize
	} else {
		return (txnCount / PageSize) + 1
	}
}

func getAccountTxnCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTxnCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTxnCount(address string) (uint64, error) {
	accountTxnCountKey, keyBlob := getAccountTxnCountKey(address)
	accountTxnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTxnCount not found", "address", address, "accountTxnCountKey", accountTxnCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTxnCount cacheDb.Get address", "address", address, "accountTxnCountKey", accountTxnCountKey, "error", err)
			return 0, err
		}
	} else {
		txnCount := common.BytesToUint64(accountTxnCountBlob)
		log.Info("getAccountTxnCount", "address", address, "accountTxnCountKey", accountTxnCountKey, "txnCount", txnCount)
		return txnCount, nil
	}
}

func (c *CacheManager) putAccountTxnCount(address string, txnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTxnCountKey, keyBlob := getAccountTxnCountKey(address)
	log.Info("putAccountTxnCount", "address", address, "accountTxnCountKey", accountTxnCountKey, "txnCount", txnCount)

	blob := common.Uint64ToBytes(txnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTxnCount address", "error", err, "address", address, "txnCount", txnCount)
		return err
	}

	return nil
}

func (c *CacheManager) GetBlockchainDetails() (GetBlockchainDetailsResponse, error) {
	if c.enableExtendedApis == false {
		return GetBlockchainDetailsResponse{}, errors.New("enableExtendedApis is false")
	}
	getResponse := GetBlockchainDetailsResponse{}
	details, err := c.getSummaryFromDb()
	if err != nil {
		return getResponse, err
	}

	getResponse.BlockchainDetails = *details

	return getResponse, nil
}

func (c *CacheManager) ListTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTransactionsResponse, error) {
	listResponse := ListAccountTransactionsResponse{}
	address := strings.ToLower(accountAddress.Hex())

	var pageCount uint64
	accountTxnCount, err := c.getAccountTxnCount(address)
	if err != nil {
		return ListAccountTransactionsResponse{}, err
	}
	if accountTxnCount%PageSize == 0 {
		pageCount = accountTxnCount / PageSize
	} else {
		pageCount = (accountTxnCount / PageSize) + 1
	}

	if pageCount == 0 {
		return ListAccountTransactionsResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListTransactionByAccount", "address", address, "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "accountTxnCount", accountTxnCount)
	if pageNumber > pageCount {
		return ListAccountTransactionsResponse{PageCount: pageCount}, nil
	}

	pageKey := fmt.Sprintf(AccountTransactionPageKey, address, pageNumber)
	accountTxnPageKey := []byte(pageKey)
	log.Info("cache get", "key", pageKey)

	accountTransactionListBlob, err := c.cacheDb.Get(accountTxnPageKey)
	if err != nil {
		log.Error("ListTransactionByAccount cacheDb.Get fromAccountTxnPageKey", "error", err)
		return ListAccountTransactionsResponse{}, err
	}
	var accountTransactionList AccountTransactionList
	err = json.Unmarshal(accountTransactionListBlob, &accountTransactionList)
	if err != nil {
		log.Error("ListTransactionByAccount json.Unmarshal accountTransactionListBlob", "error", err)
		return ListAccountTransactionsResponse{}, err
	}

	if strings.ToLower(accountTransactionList.Address) != address {
		log.Error("unexpected address accountTransactionList.Address", "address", address, "accountTransactionList.Address", accountTransactionList.Address)
		return ListAccountTransactionsResponse{}, errors.New("unexpected address accountTransactionList.Address")
	}

	/*for i, v := range accountTransactionList.Transactions {
		v.From = strings.ToLower(v.From)
		if len(v.To) != 0 {
			v.To = strings.ToLower(v.To)
		}
		accountTransactionList.Transactions[i] = v
	}*/

	listResponse.Items = accountTransactionList.Transactions
	listResponse.PageCount = pageCount

	return listResponse, nil
}

func (c *CacheManager) ListPendingTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountPendingTransactionsResponse, error) {
	c.pendingTxMapLock.RLock()
	defer c.pendingTxMapLock.RUnlock()

	log.Info("ListPendingTransactionsByAccount", "account", accountAddress)

	address := accountAddress.Hex()

	response := ListAccountPendingTransactionsResponse{
		Items: make([]AccountPendingTransactionCompact, 0),
	}

	txnMap := *c.pendingTransactions
	pendingTxnMap := txnMap["pending"]
	queuedTxnMap := txnMap["queued"]

	log.Debug("ListPendingTransactionsByAccount", "txn count", len(txnMap))
	for k, v := range txnMap {
		log.Debug("level0", "k", k, "v=%v", v)
		for k1, v1 := range v {
			log.Debug("     level1", "k", k1, "v=%v", v1)
			for k2, v2 := range v1 {
				log.Debug("          level2", "k", k2, "v=%v", v2)
			}
		}
	}

	if queuedTxnMap != nil {
		queuedAccountTxnMap := queuedTxnMap[address]
		if queuedAccountTxnMap != nil {
			for _, tx := range queuedAccountTxnMap {
				txn := AccountPendingTransactionCompact{
					From:  strings.ToLower(tx.From.Hex()),
					Value: tx.Value.String(),
				}
				if tx.To != nil {
					txn.To = strings.ToLower(tx.To.Hex())
				}
				txn.Hash = tx.Hash.Hex()
				txn.Nonce = uint64(tx.Nonce)
				response.Items = append(response.Items, txn)
				if len(response.Items) == int(PageSize) {
					break
				}
			}
		}
	}

	if pendingTxnMap != nil && len(response.Items) < int(PageSize) {
		pendingAccountTxnMap := pendingTxnMap[address]
		if pendingAccountTxnMap != nil {
			for _, tx := range pendingAccountTxnMap {
				txn := AccountPendingTransactionCompact{
					From:  strings.ToLower(tx.From.Hex()),
					Value: tx.Value.String(),
				}
				if tx.To != nil {
					txn.To = strings.ToLower(tx.To.Hex())
				}
				txn.Hash = tx.Hash.Hex()
				txn.Nonce = uint64(tx.Nonce)
				response.Items = append(response.Items, txn)
				if len(response.Items) == int(PageSize) {
					break
				}
			}
		}
	}

	if len(response.Items) > 0 {
		response.PageCount = 1
	}

	return response, nil
}

func getAccountPageKey(address string, pageCount uint64) []byte {
	pageKey := fmt.Sprintf(AccountTransactionPageKey, strings.ToLower(address), pageCount)
	return []byte(pageKey)
}

func (c *CacheManager) getTransactionType(txn *types.Transaction, receipt *types.Receipt, blockNumber *big.Int, batch *ethdb.Batch) (TransactionType, error) {
	txHash := txn.Hash()
	if txHash.IsEqualTo(receipt.TxHash) == false {
		return "", errors.New("hash mismatch between txn and receipt")
	}

	if txn.To() != nil {
		if receipt.Status == 0 {
			return COIN_TRANSFER, nil //todo: fix
		}

		acc, err := c.getAccount(*txn.To(), blockNumber, batch)
		if err != nil {
			return "", err
		}
		if acc.AccType == ethclient.ACCOUNT_TYPE_REGULAR {
			return COIN_TRANSFER, nil
		} else {
			isTokenTransfer, err := token.IsMainTransactionTokenTransfer(txn, receipt)
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
			acc, err := c.getAccount(receipt.ContractAddress, blockNumber, batch)
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

func (c *CacheManager) GetTokenDetails(contractAddress string) (*GetTokenDetailsResponse, error) {
	contractAddress = strings.ToLower(contractAddress)
	key := fmt.Sprintf(TokenDetailsKey, contractAddress)

	db := c.cacheDb
	tokenBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	var tokenDetails TokenDetails
	err = json.Unmarshal(tokenBlob, &tokenDetails)
	if err != nil {
		return nil, err
	}

	return &GetTokenDetailsResponse{
		Result: tokenDetails,
	}, nil
}

func (c *CacheManager) putIfNotExistsTokenInDb(tokenDetails *TokenDetails, batch *ethdb.Batch) error {
	_, err := c.GetTokenDetails(tokenDetails.ContractAddress)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			return c.putTokenInDb(tokenDetails, batch)
		} else {
			return err
		}
	}
	return nil
}

func (c *CacheManager) putTokenInDb(tokenDetails *TokenDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(tokenDetails)
	if err != nil {
		return err
	}

	contractAddress := strings.ToLower(tokenDetails.ContractAddress)
	key := fmt.Sprintf(TokenDetailsKey, contractAddress)
	keyBlob := []byte(key)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) getAccount(address common.Address, blockNumber *big.Int, batch *ethdb.Batch) (*AccountDetails, error) {
	accountDetails, err := c.getAccountFromCacheOrDb(address)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			acc, err := c.newAccount(address, blockNumber, batch)
			if err != nil {
				return nil, err
			}
			return acc, nil
		} else {
			return nil, err
		}
	} else {
		return accountDetails, err
	}
}

// gets account from blockchain node and saves to cache
func (c *CacheManager) newAccount(address common.Address, blockNumber *big.Int, batch *ethdb.Batch) (*AccountDetails, error) {
	result, err := c.client.GetAccountType(address, blockNumber)
	if err != nil {
		return nil, err
	}
	acc := &AccountDetails{
		AccType: result,
		Address: strings.ToLower(address.Hex()),
	}

	err = c.putAccountInCacheAndDb(acc, batch)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

// Gets account from in-memory cache or persistent cache
func (c *CacheManager) getAccountFromCacheOrDb(address common.Address) (*AccountDetails, error) {
	var accountDetails *AccountDetails
	addr := strings.ToLower(address.Hex())

	accountDetails, ok := c.addressMap[addr]
	if ok == true {
		log.Trace("getAccountFromCacheOrDb return from in memory cache", "address", address)
		return accountDetails, nil
	}

	key := fmt.Sprintf(AccountSummaryKey, addr)

	db := c.cacheDb
	accountBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(accountBlob, accountDetails)
	if err != nil {
		return nil, err
	}

	return accountDetails, nil
}

// puts account in in-memory cache and in persistent store
func (c *CacheManager) putAccountInCacheAndDb(accountDetails *AccountDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(accountDetails)
	if err != nil {
		return err
	}

	accountAddress := strings.ToLower(accountDetails.Address)
	key := fmt.Sprintf(AccountSummaryKey, accountAddress)
	keyBlob := []byte(key)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	c.addressMap[accountDetails.Address] = accountDetails

	return nil
}

type InternalTransactionDetail struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Value string `json:"value,omitempty"`
	Type  string `json:"type,omitempty"`
	Level byte   `json:"level,omitempty"`
}

type InternalTransactionDetailWithLevel struct {
	txn   *ethclient.InternalTransactionDetails
	level byte
}

func (c *CacheManager) flattenInternalTransactionDetails(details *ethclient.InternalTransactionDetails) []*InternalTransactionDetail {
	txnStack := newStack()
	txnList := make([]*InternalTransactionDetail, 0)

	detailsWithLevel := &InternalTransactionDetailWithLevel{
		txn:   details,
		level: byte(0),
	}
	txnStack.Push(detailsWithLevel)

	for txnStack.IsEmpty() == false {
		txnWithLevel := txnStack.Pop()
		txn := txnWithLevel.txn
		txnDetail := InternalTransactionDetail{
			From:  txn.From,
			To:    txn.To,
			Value: txn.Value,
			Type:  txn.Type,
			Level: txnWithLevel.level,
		}
		txnList = append(txnList, &txnDetail)
		if txn.Calls != nil {
			for _, t := range txn.Calls {
				detailsWithLevel = &InternalTransactionDetailWithLevel{
					txn:   &t,
					level: txnWithLevel.level + 1,
				}
				txnStack.Push(detailsWithLevel)
			}
		}
	}
	return txnList
}

type stack struct { //not thread safe
	internalTxnDetails []*InternalTransactionDetailWithLevel
}

func newStack() *stack {
	s := stack{
		internalTxnDetails: make([]*InternalTransactionDetailWithLevel, 0),
	}
	return &s
}

func (s stack) IsEmpty() bool {
	return len(s.internalTxnDetails) == 0
}

func (s stack) Push(v *InternalTransactionDetailWithLevel) {
	s.internalTxnDetails = append(s.internalTxnDetails, v)
}

func (s stack) Pop() *InternalTransactionDetailWithLevel {
	count := len(s.internalTxnDetails)
	last := s.internalTxnDetails[count-1]
	s.internalTxnDetails = s.internalTxnDetails[:count-1]

	return last
}

func getAccountTokenCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTokenCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTokenCount(address string) (uint64, error) {
	accountTokenCountKey, keyBlob := getAccountTokenCountKey(address)
	accountTokenCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTokenCount not found", "address", address, "accountTokenCountKey", accountTokenCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTokenCount cacheDb.Get address", "address", address, "accountTokenCountKey", accountTokenCountKey, "error", err)
			return 0, err
		}
	} else {
		tokenCount := common.BytesToUint64(accountTokenCountBlob)
		log.Info("getAccountTxnCount", "address", address, "accountTokenCountKey", accountTokenCountKey, "tokenCount", tokenCount)
		return tokenCount, nil
	}
}

func (c *CacheManager) putAccountTokenCount(address string, tokenCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTokenCountKey, keyBlob := getAccountTokenCountKey(address)
	log.Info("putAccountTokenCount", "address", address, "accountTokenCountKey", accountTokenCountKey, "tokenCount", tokenCount)

	blob := common.Uint64ToBytes(tokenCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTokenCount address", "error", err, "address", address, "tokenCount", tokenCount)
		return err
	}

	return nil
}

func getAccountTokenTxnCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTokenTxnCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTokenTxnCount(address string) (uint64, error) {
	accountTokenTxnCountKey, keyBlob := getAccountTokenTxnCountKey(address)
	accountTokenTxnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTokenTxnCount not found", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTokenTxnCount cacheDb.Get address", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "error", err)
			return 0, err
		}
	} else {
		tokenTxnCount := common.BytesToUint64(accountTokenTxnCountBlob)
		log.Info("getAccountTokenTxnCount", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "tokenTxnCount", tokenTxnCount)
		return tokenTxnCount, nil
	}
}

func (c *CacheManager) putAccountTokenTxnCount(address string, tokenTxnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTokenTxnCountKey, keyBlob := getAccountTokenTxnCountKey(address)
	log.Info("putAccountTokenTxnCount", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "tokenTxnCount", tokenTxnCount)

	blob := common.Uint64ToBytes(tokenTxnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTokenTxnCount address", "error", err, "address", address, "tokenTxnCount", tokenTxnCount)
		return err
	}

	return nil
}

// Parses and stores list of tokens and balance for each token
func (c *CacheManager) processAccountTokenTransfers(accountAddress string, tokenTransfers []*token.LogTransfer, batch *ethdb.Batch) error {

	return nil
}

// Parses and stores list of token transactions with transaction summary
func (c *CacheManager) processAccountTokenTransaction(txn *AccountTokenTransactionCompact, batch *ethdb.Batch) error {

	return nil
}
