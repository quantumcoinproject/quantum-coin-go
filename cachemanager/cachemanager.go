package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
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
	cacheDir  string
	nodeUrl   string
	cacheLock sync.Mutex
	cacheDb   ethdb.Database
}

var LastBlockKey = "last-block"
var AccountTxnCountKey = "account-txn-count-%s"                  //%s is account address
var AccountTransactionPageKey = "account-transaction-list-%s-%d" //%s is account address, %d is page number
var chainID *big.Int

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

type ListAccountTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount,omitempty"`
	Items     []AccountTransactionCompact `json:"items,omitempty"`
}

func NewCacheManager(cacheDir string, nodeUrl string) (*CacheManager, error) {
	cManager := &CacheManager{
		nodeUrl:  nodeUrl,
		cacheDir: cacheDir,
	}

	err := cManager.initialize()
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

	chainID, err = client.NetworkID(context.Background())
	if err != nil {
		log.Error("initialize NetworkID", "error", err)
		return err
	}

	client.Close()

	return nil
}

func (c *CacheManager) start() error {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	blockNumber, err := c.getLastBlockNumberByDb(LastBlockKey)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			log.Warn("First time start")
			blockNumber = 0
		} else {
			log.Error("GetLastBlockByDb", "err", err.Error())
			return err
		}
	}

	delayNumber := int64(100 * time.Millisecond)
	cacheTimer := time.NewTimer(time.Duration(delayNumber))

	go func() {
		for {
			select {
			case <-cacheTimer.C:
				blockNumberToGet := blockNumber + 1
				log.Info("Batch Start ", "Block Number ", blockNumberToGet)
				err := c.processByCacheManager(blockNumberToGet)
				if err == nil {
					blockNumber = blockNumberToGet
					log.Info("Batch Complete", "Block number", blockNumberToGet)
					delayNumber = 0
					if blockNumber == 4508 {
						//cacheTimer.Stop()
						//return
					}

				} else {
					if err.Error() == "not found" {
						log.Info("Block not found", "Block number", blockNumberToGet)
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

func (c *CacheManager) processByCacheManager(blockNumber uint64) error {
	client, err := ethclient.Dial(c.nodeUrl)
	if err != nil {
		return err
	}

	blockNum := new(big.Int).SetUint64(blockNumber)
	block, err := client.BlockByNumber(context.Background(), blockNum)
	if err != nil {
		if err.Error() != "not found" {
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

	var liveAccountMap map[string][]AccountTransactionCompact //address to transactions in block mapping
	liveAccountMap = make(map[string][]AccountTransactionCompact)

	for _, tx := range block.Transactions() {
		receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			log.Error("processByCacheManager TransactionReceipt", "error", err)
			return err
		}

		msg, err := tx.AsMessage(types.NewLondonSigner(chainID))
		if err != nil {
			log.Error("processByCacheManager AsMessage", "error", err)
			return err
		}

		fromAddress := msg.From().Hex()
		var toAddress string
		if tx.To() != nil {
			toAddress = tx.To().Hex()
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

		//todo: fix
		transaction.TransactionType = "CoinTransfer"

		_, ok := liveAccountMap[strings.ToLower(fromAddress)]
		if ok == false {
			liveAccountMap[strings.ToLower(fromAddress)] = make([]AccountTransactionCompact, 0)
		}
		liveAccountMap[strings.ToLower(fromAddress)] = append(liveAccountMap[strings.ToLower(fromAddress)], transaction)

		if tx.To() != nil {
			if strings.ToLower(fromAddress) != strings.ToLower(toAddress) {
				_, ok := liveAccountMap[strings.ToLower(toAddress)]
				if ok == false {
					liveAccountMap[strings.ToLower(toAddress)] = make([]AccountTransactionCompact, 0)
				}
				liveAccountMap[strings.ToLower(toAddress)] = append(liveAccountMap[strings.ToLower(toAddress)], transaction)
			}
		}
	}

	for k, v := range liveAccountMap {
		err = c.processAccountTransactions(k, &v, &txnBatch)
		if err != nil {
			log.Error("processAccountTransaction", "error", err, "address", k)
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

func (c *CacheManager) close() error {
	cacheDb := c.cacheDb
	err := cacheDb.Close()
	if err != nil {
		log.Debug("cache manager account transaction db close error", "err", err)
		return err
	}

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

		if strings.ToLower(accountTransactionList.Address) != strings.ToLower(address) {
			return errors.New("unexpected fromaddress")
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
		accountTransactionList.Transactions = append(accountTransactionList.Transactions, txn)

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
		if err.Error() == "leveldb: not found" {
			log.Info("getAccountTxnCount not found", "address", address, "accountTxnCountKey", accountTxnCountKey)
			return 0, nil
		} else {
			log.Error("processByCacheManager cacheDb.Get address", "address", address, "accountTxnCountKey", accountTxnCountKey, "error", err)
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

func (c *CacheManager) ListTransactionByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTransactionsResponse, error) {
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

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = 1
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListTransactionByAccount", "address", address, "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "accountTxnCount", accountTxnCount)
	if pageNumber > pageCount {
		return ListAccountTransactionsResponse{}, nil
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

	if strings.ToLower(accountTransactionList.Address) != strings.ToLower(address) {
		log.Error("unexpected address accountTransactionList.Address", "address", address, "accountTransactionList.Address", accountTransactionList.Address)
		return ListAccountTransactionsResponse{}, errors.New("unexpected address accountTransactionList.Address")
	}

	listResponse.Items = accountTransactionList.Transactions
	listResponse.PageCount = pageCount

	return listResponse, nil
}

func getAccountPageKey(address string, pageCount uint64) []byte {
	pageKey := fmt.Sprintf(AccountTransactionPageKey, strings.ToLower(address), pageCount)
	return []byte(pageKey)
}
