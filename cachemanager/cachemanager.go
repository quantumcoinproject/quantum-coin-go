package cachemanager

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type CacheManager struct {
	cacheDir  string
	nodeUrl   string
	catchLock sync.Mutex
	catchDb   ethdb.Database
}

var BlockGetKey = "quantum-coin-block-count-get"
var AccountGetKey = "quantum-coin-account-count-get-%s"
var AccountTransactionListKey = "quantum-coin-account-transaction-list-%s-%d-%d"

var accountPagination int64 = 20

type AccountTransaction struct {
	Address     string `json:"address"`
	Transaction Transaction
}

type Transaction struct {
	BlockNumber       string       `json:"blockNumber"`
	Hash              string       `json:"hash"`
	Value             string       `json:"value"`
	Nonce             uint64       `json:"nonce"`
	Data              []byte       `json:"data"`
	TimeStamp         uint64       `json:"timeStamp"`
	From              string       `json:"from"`
	To                string       `json:"to"`
	Type              uint64       `json:"type"`
	Gas               uint64       `json:"gas"`
	GasPrice          string       `json:"gasPrice"`
	MaxGasTier        string       `json:"maxGasTier"`
	GasUsed           uint64       `json:"gasUsed"`
	CumulativeGasUsed uint64       `json:"cumulativeGasUsed"`
	Status            uint64       `json:"status"`
	Logs              []ReceiptLog `json:"logs"`
}

type ReceiptLog struct {
	Address     common.Address `json:"address" gencodec:"required"`
	Topics      []common.Hash  `json:"topics" gencodec:"required"`
	Data        []byte         `json:"data" gencodec:"required"`
	BlockNumber uint64         `json:"blockNumber"`
	TxHash      common.Hash    `json:"transactionHash" gencodec:"required"`
	TxIndex     uint           `json:"transactionIndex"`
	BlockHash   common.Hash    `json:"blockHash"`
	Index       uint           `json:"logIndex"`
	Removed     bool           `json:"removed"`
}

type AccountTransactionCompact struct {
	Hash string `json:"hash,omitempty"`

	BlockNumber int64 `json:"blockNumber,omitempty"`

	CreatedAt time.Time `json:"createdAt,omitempty"`

	From *string `json:"from,omitempty"`

	To *string `json:"to,omitempty"`

	Value *string `json:"value,omitempty"`

	TxnFee *string `json:"txnFee,omitempty"`

	Status *string `json:"status,omitempty"`

	TransactionType TransactionType `json:"transactionType,omitempty"`

	ErrorReason *string `json:"errorReason,omitempty"`
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

type ListAccountTransactionsResponse struct {
	PageCount int64                       `json:"pageCount,omitempty"`
	Items     []AccountTransactionCompact `json:"items,omitempty"`
}

func NewCacheManager(cacheDir string, nodeUrl string) error {
	cManager := &CacheManager{
		nodeUrl:  nodeUrl,
		cacheDir: cacheDir,
	}

	err := cManager.initialize()
	if err != nil {
		return err
	}
	cManager.start()

	return nil
}

func ListTransactionByAccount(address string, page string, cacheDir string) (ListAccountTransactionsResponse, error) {
	c := &CacheManager{
		cacheDir: cacheDir,
	}

	listResponse := ListAccountTransactionsResponse{}

	err := c.initialize()
	if err != nil {
		return listResponse, err
	}

	accountKey := []byte(fmt.Sprintf(AccountGetKey, address))

	pagination, err := c.catchDb.Get(accountKey)
	if err != nil {
		return listResponse, err
	}

	if len(pagination) != 0 {

		pageArray := decode(pagination)

		var pageRow, pageCount int64
		fmt.Sscan(pageArray[0], &pageRow)
		fmt.Sscan(pageArray[1], &pageCount)

		pageNumber := pageCount

		if len(strings.TrimSpace(page)) != 0 {
			fmt.Sscan(page, &pageNumber)
			if pageNumber > pageCount {
				pageNumber = pageCount
			}
		}

		var i int64
		var accountTransactions []AccountTransactionCompact

		for i = 0; i < pageRow; i++ {

			accountTransactionKey := []byte(fmt.Sprintf(AccountTransactionListKey, address, pageRow-i, pageNumber))
			accountTrans, err := decodeToAccountTransaction(accountTransactionKey)
			if err != nil {
				return listResponse, err
			}

			var accountTransaction AccountTransactionCompact
			accountTransaction.Hash = accountTrans.Transaction.Hash

			var b int64
			fmt.Sscan(accountTrans.Transaction.BlockNumber, &b)
			accountTransaction.BlockNumber = b

			formattedTime, err := time.Parse("2006-01-02T15:04:05", string(accountTrans.Transaction.TimeStamp))
			if err != nil {
				return listResponse, err
			}

			accountTransaction.CreatedAt = formattedTime
			accountTransaction.From = &accountTrans.Transaction.From
			accountTransaction.To = &accountTrans.Transaction.To
			accountTransaction.Value = &accountTrans.Transaction.Value

			var TxnFee = new(big.Int)
			TxnFee.Mul(new(big.Int).SetUint64(accountTrans.Transaction.CumulativeGasUsed), new(big.Int).SetUint64(accountTrans.Transaction.GasUsed))
			TxnFeeStr := fmt.Sprint(TxnFee)

			accountTransaction.TxnFee = &TxnFeeStr

			if accountTrans.Transaction.Type == 1 {
				accountTransaction.TransactionType = COIN_TRANSFER
			} else if accountTrans.Transaction.Type == 2 {
				accountTransaction.TransactionType = NEW_TOKEN
			} else if accountTrans.Transaction.Type == 3 {
				accountTransaction.TransactionType = TOKEN_TRANSFER
			} else if accountTrans.Transaction.Type == 4 {
				accountTransaction.TransactionType = NEW_SMART_CONTRACT
			} else if accountTrans.Transaction.Type == 5 {
				accountTransaction.TransactionType = SMART_CONTRACT
			}

			var status string
			if accountTrans.Transaction.Status == 1 {
				status = "0x1"
			} else {
				status = "0x0"
			}
			accountTransaction.Status = &status
			accountTransactions = append(accountTransactions, accountTransaction)
		}

		listResponse.PageCount = pageCount
		listResponse.Items = accountTransactions

		return listResponse, nil
	}

	return listResponse, nil
}

func (c *CacheManager) initialize() error {

	log.Info("Quantum Coin initialize cache manager", "cacheDir", c.cacheDir, "nodeUrl", c.nodeUrl)

	catchManagerFilePath := filepath.Join(c.cacheDir, "cacheManager.db")
	catchManager, err := rawdb.NewLevelDBDatabase(catchManagerFilePath, 64, 0, "", false)
	if err != nil {
		return err
	}
	c.catchDb = catchManager

	return nil
}

func (c *CacheManager) start() {
	c.catchLock.Lock()
	defer c.catchLock.Unlock()
	//var prevBlockNumber uint64 = 0

	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	blockNumber, err := c.getLastBlockNumberByDb(BlockGetKey)
	if err != nil {
		log.Error("GetLastBlockByDb ", err.Error())
	}

	delayNumber := int64(100 * time.Millisecond)
	cacheTimer := time.NewTimer(time.Duration(delayNumber))

	go func() {
		for {
			select {
			case <-cacheTimer.C:
				blockNumber = blockNumber + 1
				//if blockNumber > prevBlockNumber {
				log.Info("Batch Start ", "Block Number ", blockNumber, "Catch Time", time.Now().String())
				//prevBlockNumber = blockNumber
				err := c.processByCatchManager(blockNumber)
				if err == nil {
					//prevBlockNumber = blockNumber
					log.Info("Batch Complete", "Block number", blockNumber)
				} else {
					blockNumber = blockNumber - 1
					log.Error("Batch Error", err.Error(), "Block number", blockNumber)

					latestBlock, err := c.latestBlockByNode()
					if err == nil {
						log.Debug("Block number didn't match in prevBlockNumber.", "Block number", blockNumber)
						if blockNumber >= latestBlock {
							delayNumber = int64(5 * time.Second)
						}
					}
				}
				//}
				cacheTimer.Reset(time.Duration(delayNumber))

			case <-cancel:
				cacheTimer.Stop()
				c.close()
				log.Info("Quit signal received")
				os.Exit(1)
				return
			}
		}
	}()

}

func (c *CacheManager) processByCatchManager(blockNumber uint64) error {

	client, err := ethclient.Dial(c.nodeUrl)
	if err != nil {
		return err
	}

	block, err := client.BlockByNumber(context.Background(), new(big.Int).SetUint64(blockNumber))
	fmt.Println("block ", block, "Error ", err.Error())
	if err != nil {
		return err
	}

	accountTransactionBatch := c.catchDb.NewBatch()

	blockKey := []byte(BlockGetKey)
	accountTransactionBatch.Put(blockKey, uint64ToBytes(blockNumber))

	for _, tx := range block.Transactions() {

		fmt.Println("1")

		var fromAddress string
		var toAddress string

		var transaction Transaction

		toAddress = tx.To().Hex()
		transaction.Hash = tx.Hash().Hex()
		transaction.Value = tx.Value().String()
		transaction.Nonce = tx.Nonce()
		transaction.Data = tx.Data()
		transaction.To = toAddress
		transaction.Type = uint64(tx.Type())
		transaction.Gas = tx.Gas()
		transaction.GasPrice = tx.GasPrice().String()
		transaction.MaxGasTier = tx.MaxGasTier().String()
		transaction.TimeStamp = block.Time()

		chainID, err := client.NetworkID(context.Background())
		if err != nil {
			return err
		}

		if msg, err := tx.AsMessage(types.NewLondonSigner(chainID)); err != nil {
			fromAddress = msg.From().Hex()
		}

		receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			return err
		}

		var receiptLogs []ReceiptLog

		for _, vLog := range receipt.Logs {
			var receiptLog ReceiptLog
			receiptLog.Address = vLog.Address
			receiptLog.Topics = vLog.Topics
			receiptLog.Data = vLog.Data
			receiptLog.BlockNumber = vLog.BlockNumber
			receiptLog.BlockHash = vLog.BlockHash
			receiptLog.TxHash = vLog.TxHash
			receiptLog.TxIndex = vLog.TxIndex
			receiptLog.Index = vLog.Index
			receiptLog.Removed = vLog.Removed
			receiptLogs = append(receiptLogs, receiptLog)
		}

		transaction.GasUsed = receipt.GasUsed
		transaction.CumulativeGasUsed = receipt.CumulativeGasUsed
		transaction.Status = receipt.Status
		transaction.Logs = receiptLogs

		transaction.From = fromAddress

		//transactions = append(transactions, transaction)

		//Account
		accountFromKey := []byte(fmt.Sprintf(AccountGetKey, fromAddress))
		accountToKey := []byte(fmt.Sprintf(AccountGetKey, toAddress))

		paginationFrom, err := c.catchDb.Get(accountFromKey)
		if err != nil {
			return err
		}
		//fromPage := strings.Split(string(paginationFrom[:]), ",")
		fromPage := decode(paginationFrom)

		var pageFrom, pageFromCount int64
		fmt.Sscan(fromPage[0], &pageFrom)
		fmt.Sscan(fromPage[1], &pageFromCount)
		if accountPagination < (pageFrom + 1) {
			pageFromCount = pageFromCount + 1
		}

		paginationTo, err := c.catchDb.Get(accountToKey)
		if err != nil {
			return err
		}
		//toPage := strings.Split(string(paginationTo[:]), ",")
		toPage := decode(paginationTo)

		var pageTo, pageToCount int64
		fmt.Sscan(toPage[0], &pageTo)
		fmt.Sscan(toPage[1], &pageToCount)
		if accountPagination < (pageTo + 1) {
			pageToCount = pageToCount + 1
		}

		fromPage[0] = strconv.FormatInt(pageFrom, 10)
		fromPage[1] = strconv.FormatInt(pageFromCount, 10)

		toPage[0] = strconv.FormatInt(pageTo, 10)
		toPage[1] = strconv.FormatInt(pageToCount, 10)

		accountTransactionBatch.Put(accountFromKey, encode(fromPage))
		accountTransactionBatch.Put(accountToKey, encode(toPage))

		//Transaction
		var fromAccountTransaction AccountTransaction
		fromAccountTransaction.Address = fromAddress
		fromAccountTransaction.Transaction = transaction

		var toAccountTransaction AccountTransaction
		toAccountTransaction.Address = toAddress
		toAccountTransaction.Transaction = transaction

		accountFromTransactionKey := []byte(fmt.Sprintf(AccountTransactionListKey, fromAddress, pageFrom, pageFromCount))
		accountToTransactionKey := []byte(fmt.Sprintf(AccountTransactionListKey, toAddress, pageTo, pageToCount))

		ft, err := encodeToBytes(fromAccountTransaction)
		if err != nil {
			return err
		}

		tt, err := encodeToBytes(toAccountTransaction)
		if err != nil {
			return err
		}
		accountTransactionBatch.Put(accountFromTransactionKey, ft)
		accountTransactionBatch.Put(accountToTransactionKey, tt)
	}

	accountTransactionBatch.Write()

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

	fmt.Println("latestBlockByNode ", latestBlock)

	return latestBlock, nil

}

func (c *CacheManager) getLastBlockNumberByDb(blockKey string) (uint64, error) {
	db := c.catchDb
	mySlice, err := db.Get([]byte(blockKey))
	if err != nil {
		return uint64(0), err
	}

	var blockNumber uint64
	blockNumber = *(*uint64)(unsafe.Pointer(&mySlice[0]))

	//fmt.Println("getLastByBlockNumber ", blockNumber)

	return blockNumber, nil
}

func (c *CacheManager) close() error {
	//c.catchLock.Lock()
	//defer c.catchLock.Unlock()

	catchDb := c.catchDb
	err := catchDb.Close()
	if err != nil {
		log.Debug("cache manager account transaction db close error", "err", err)
		return err
	}

	return nil
}

func uint64ToBytes(val uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, val)
	return b
}

const maxInt32 = 1<<(32-1) - 1

func writeLen(b []byte, l int) []byte {
	if 0 > l || l > maxInt32 {
		panic("writeLen: invalid length")
	}
	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(l))
	return append(b, lb[:]...)
}

func readLen(b []byte) ([]byte, int) {
	if len(b) < 4 {
		panic("readLen: invalid length")
	}
	l := binary.BigEndian.Uint32(b)
	if l > maxInt32 {
		panic("readLen: invalid length")
	}
	return b[4:], int(l)
}

func decode(b []byte) []string {
	b, ls := readLen(b)
	s := make([]string, ls)
	for i := range s {
		b, ls = readLen(b)
		s[i] = string(b[:ls])
		b = b[ls:]
	}
	return s
}

func encode(s []string) []byte {
	var b []byte
	b = writeLen(b, len(s))
	for _, ss := range s {
		b = writeLen(b, len(ss))
		b = append(b, ss...)
	}
	return b
}

func encodeToBytes(p interface{}) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(p)
	if err != nil {
		return nil, err
	}
	//fmt.Println("uncompressed size (bytes): ", len(buf.Bytes()))
	return buf.Bytes(), nil
}

func decodeToAccountTransaction(s []byte) (AccountTransaction, error) {
	var p AccountTransaction
	dec := gob.NewDecoder(bytes.NewReader(s))
	err := dec.Decode(&p)
	if err != nil {
		return AccountTransaction{}, err
	}
	return p, nil
}
