package cachemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const BlockTransactionPageKey = "block-transaction-list-%d-%d" //%d is block number, %d is page number

type BlockTransactionList struct {
	BlockNumber  uint64               `json:"blockNumber"`
	Transactions []TransactionCompact `json:"transactions"`
}

type ListBlockTransactionsResponse struct {
	PageCount uint64               `json:"pageCount"`
	Items     []TransactionCompact `json:"items"`
}

func getBlockTransactionPageKey(blockNumber uint64, pageCount uint64) []byte {
	pageKey := fmt.Sprintf(BlockTransactionPageKey, blockNumber, pageCount)
	return []byte(pageKey)
}

func (c *CacheManager) processBlockTransactions(block *Block, txnList *[]*TransactionDetails, batch *ethdb.Batch) error {
	log.Debug("CacheManager address", "blockNumber", block.BlockNumber)
	txnBatch := *batch
	var blockTransactionList BlockTransactionList

	blockTransactionList.Transactions = make([]TransactionCompact, 0)
	blockTransactionList.BlockNumber = uint64(block.BlockNumber)
	log.Debug("CacheManager processBlockTransactions", "BlockNumber", block.BlockNumber, "txnCount", len(*txnList))

	for i, txn := range *txnList {
		log.Trace("CacheManager processBlockTransactions", "BlockNumber", block.BlockNumber, "txn", txn.Hash)
		blockTxn := transactionCompactFromTransaction(txn)
		blockTransactionList.Transactions = append([]TransactionCompact{blockTxn}, blockTransactionList.Transactions...) //prepend for backward compat

		if len(blockTransactionList.Transactions) == int(PageSize) || i == len(*txnList)-1 {
			transactionListBlob, err := json.Marshal(blockTransactionList)
			if err != nil {
				log.Error("CacheManager processBlockTransactions json.Marshal transactionListBlob", "error", err)
				return err
			}

			runningTxnCount := uint64(i) + 1
			txnPageCount := getPageCount(runningTxnCount)
			txnPageKey := getBlockTransactionPageKey(uint64(block.BlockNumber), txnPageCount)

			err = txnBatch.Put(txnPageKey, transactionListBlob)
			if err != nil {
				log.Error("CacheManager processBlockTransactions txnBatch.Put transactionListBlob", "error", err)
				return err
			}
			log.Info("CacheManager txnBatch.Put", "runningTxnCount", runningTxnCount, "txnPageCount", txnPageCount)
			blockTransactionList.Transactions = make([]TransactionCompact, 0) //reset
		}
	}

	log.Trace("CacheManager processBlockTransactions done", "BlockNumber", block.BlockNumber, "txnCount", len(*txnList))

	return nil
}

func (c *CacheManager) getBlockTxnCount(blockNumber uint64) (uint64, error) {
	block, err := c.GetBlockDetails(blockNumber)
	if err != nil {
		return 0, err
	}
	return uint64(block.BlockNumber), nil
}

func (c *CacheManager) ListTransactionsByBlock(blockNumber uint64, pageNumberInput int64) (ListBlockTransactionsResponse, error) {
	listResponse := ListBlockTransactionsResponse{}

	var pageCount uint64
	BlockTxnCount, err := c.getBlockTxnCount(blockNumber)
	if err != nil {
		return ListBlockTransactionsResponse{}, err
	}
	if BlockTxnCount%PageSize == 0 {
		pageCount = BlockTxnCount / PageSize
	} else {
		pageCount = (BlockTxnCount / PageSize) + 1
	}

	if pageCount == 0 {
		return ListBlockTransactionsResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}

	log.Info("ListTransactionByBlock", "blockNumber", blockNumber, "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "BlockTxnCount", BlockTxnCount)
	if pageNumber > pageCount {
		return ListBlockTransactionsResponse{PageCount: pageCount}, nil
	}

	pageKey := fmt.Sprintf(BlockTransactionPageKey, blockNumber, pageNumber)
	BlockTxnPageKey := []byte(pageKey)
	log.Info("cache get", "key", pageKey)

	BlockTransactionListBlob, err := c.cacheDb.Get(BlockTxnPageKey)
	if err != nil {
		log.Error("ListTransactionByBlock cacheDb.Get fromBlockTxnPageKey", "error", err)
		return ListBlockTransactionsResponse{}, err
	}
	var blockTransactionList BlockTransactionList
	err = json.Unmarshal(BlockTransactionListBlob, &blockTransactionList)
	if err != nil {
		log.Error("ListTransactionByBlock json.Unmarshal BlockTransactionListBlob", "error", err)
		return ListBlockTransactionsResponse{}, err
	}

	if blockTransactionList.BlockNumber != blockNumber {
		log.Error("unexpected address BlockTransactionList.Address", "blockNumber", blockNumber, "blockTransactionList.BlockNumber", blockTransactionList.BlockNumber)
		return ListBlockTransactionsResponse{}, errors.New("unexpected address BlockTransactionList.blockNumber")
	}

	listResponse.Items = blockTransactionList.Transactions
	listResponse.PageCount = pageCount

	return listResponse, nil
}
