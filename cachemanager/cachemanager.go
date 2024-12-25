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
	catchLock sync.Mutex
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

type BlockAccountLivePageDetails struct {
	accountTxnList     AccountTransactionList
	accountTxnListBlob []byte
	pageKeyBlob        []byte
}

type BlockAccountLive struct {
	txnCount  uint64
	pageCount uint64
	pageMap   map[uint64]*BlockAccountLivePageDetails
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
	c.catchLock.Lock()
	defer c.catchLock.Unlock()

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
				log.Info("Batch Start ", "Block Number ", blockNumberToGet, "Catch Time", time.Now().String())
				err := c.processByCacheManager(blockNumberToGet)
				if err == nil {
					blockNumber = blockNumberToGet
					log.Info("Batch Complete", "Block number", blockNumberToGet)
					delayNumber = 0
					if blockNumber == 4508 {
						cacheTimer.Stop()
						return
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

	var liveAccountMap map[string]BlockAccountLive
	liveAccountMap = make(map[string]BlockAccountLive)

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
		toAddress := tx.To().Hex()

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

		err = c.processAccountTransaction(fromAddress, &transaction, &liveAccountMap)
		if err != nil {
			log.Error("putTransaction", "error", err, "fromAddress", fromAddress)
			return err
		}

		err = c.processAccountTransaction(toAddress, &transaction, &liveAccountMap)
		if err != nil {
			log.Error("putTransaction", "error", err, "toAddress", toAddress)
			return err
		}
	}

	for k, v := range liveAccountMap {
		err = c.putAccountTxnCount(k, v.txnCount, &txnBatch)
		if err != nil {
			return err
		}

		for _, inner := range v.pageMap {
			err = txnBatch.Put(inner.pageKeyBlob, inner.accountTxnListBlob)
			if err != nil {
				log.Error("txnBatch.Put accountTransactionListBlob", "error", err)
				return err
			}
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
	catchDb := c.cacheDb
	err := catchDb.Close()
	if err != nil {
		log.Debug("cache manager account transaction db close error", "err", err)
		return err
	}

	return nil
}

func (c *CacheManager) processAccountTransaction(address string, txn *AccountTransactionCompact, liveBlockMap *map[string]BlockAccountLive) error {
	liveMap := *liveBlockMap

	var accountTxnPageKey []byte
	var accountTxnCount uint64
	var txnPageCount uint64
	var accountTransactionList AccountTransactionList
	var accountTransactionListBlob []byte
	var txnCount uint64
	var err error

	blockAccountTransactions, ok := liveMap[address]
	if ok {
		txnCount = blockAccountTransactions.txnCount
		accountTxnCount = txnCount + 1
		if accountTxnCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache

		}
	} else {
		txnCount, err = c.getAccountTxnCount(address)
		if err != nil {
			return err
		}

		accountTxnCount = txnCount + 1
		txnPageCount = getPageCount(txnCount)

		accountTxnPageKey = getAccountPageKey(address, txnPageCount)

		if accountTxnCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
			accountTransactionList.Transactions = make([]AccountTransactionCompact, 0)
			accountTransactionList.Address = address
		} else {
			accountTransactionListBlob, err = c.cacheDb.Get(accountTxnPageKey)
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
				log.Error("unexpected transactions count from address", "actual", len(accountTransactionList.Transactions), "expected", int(accountTxnCount%PageSize)-1, "txnCount", txnCount)
				return errors.New("unexpected transactions count")
			}
		}
	}

	log.Info("processAccountTransaction", "address", address, "accountTxnCount", accountTxnCount)
	accountTransactionList.Transactions = append(accountTransactionList.Transactions, *txn)
	accountTransactionListBlob, err = json.Marshal(accountTransactionList)
	if err != nil {
		log.Error("json.Marshal accountTransactionListBlob", "error", err)
	}

	log.Info("inserted from account txn", "txnPageCount", txnPageCount, "accountTxnCount", accountTxnCount, "address", address)

	return nil
}

func getPageCount(txnCount uint64) uint64 {
	if txnCount%PageSize == 0 {
		return txnCount / PageSize
	} else {
		return (txnCount / PageSize) + 1
	}
}

func (c *CacheManager) getAccountTxnCount(address string) (uint64, error) {
	address = strings.ToLower(address)
	fromAccountTxnCountKey := []byte(fmt.Sprintf(AccountTxnCountKey, address))
	fromAccountTxnCountBlob, err := c.cacheDb.Get(fromAccountTxnCountKey)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			return 0, nil
		} else {
			log.Error("processByCacheManager cacheDb.Get address", "error", err)
			return 0, err
		}
	} else {
		return common.BytesToUint64(fromAccountTxnCountBlob), nil
	}
}

func (c *CacheManager) putAccountTxnCount(address string, txnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	fromAccountTxnCountKey := []byte(fmt.Sprintf(AccountTxnCountKey, address))
	blob := common.Uint64ToBytes(txnCount)
	err := txnBatch.Put(fromAccountTxnCountKey, blob)
	if err != nil {
		log.Error("putAccountTxnCount address", "error", err, "address", address, "txnCount", txnCount)
		return err
	}

	checkTxnCount, err := c.getAccountTxnCount(address)
	if err != nil {
		log.Error("checkTxnCount", "error", err)
		return err
	}
	if checkTxnCount != txnCount {
		log.Error("checkTxnCount mismatch", "checkTxnCount", checkTxnCount, "txnCount", txnCount)
		return err
	}

	return nil
}

func (c *CacheManager) ListTransactionByAccount(accountAddress common.Address, pageNumber uint64) (ListAccountTransactionsResponse, error) {
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

	log.Info("ListTransactionByAccount", "address", address, "pageNumber", pageNumber, "pageCount", pageCount, "accountTxnCount", accountTxnCount)
	if pageNumber == 0 {
		pageNumber = 1
	} else if pageNumber > pageCount {
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
