package cachemanager

import (
	"context"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (c *CacheManager) ListPendingTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountPendingTransactionsResponse, error) {
	c.pendingTxMapLock.RLock()
	defer c.pendingTxMapLock.RUnlock()

	log.Info("ListPendingTransactionsByAccount", "account", accountAddress)

	address := accountAddress.HexLower()

	response := ListAccountPendingTransactionsResponse{
		Items: make([]AccountPendingTransactionCompact, 0),
	}

	if c.pendingTransactions == nil {
		return ListAccountPendingTransactionsResponse{}, nil
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
					From:  tx.From.HexLower(),
					Value: tx.Value.String(),
				}
				if tx.To != nil {
					txn.To = tx.To.HexLower()
				}
				txn.Hash = tx.Hash.HexLower()
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
					From:  tx.From.HexLower(),
					Value: tx.Value.String(),
				}
				if tx.To != nil {
					txn.To = tx.To.HexLower()
				}
				txn.Hash = tx.Hash.HexLower()
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

func (c *CacheManager) processPendingTransactions() {
	c.pendingTxLock.Lock()
	defer c.pendingTxLock.Unlock()

	delayNumber := int64(3000 * time.Millisecond)
	pendingTxnTimer := time.NewTimer(time.Duration(delayNumber))
	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-pendingTxnTimer.C:
				err, txnList := c.pendingTxClient.TxPoolContent(context.Background())
				if err != nil {
					log.Error("processPendingTransactions", "err", err)
				} else {
					if txnList == nil {
						log.Warn("processPendingTransactions txnList is nil")
					} else {
						c.pendingTxMapLock.Lock()
						c.pendingTransactions = txnList
						c.pendingTxMapLock.Unlock()
					}
				}

				pendingTxnTimer.Reset(time.Duration(delayNumber))
			case <-cancel:
				log.Warn("processPendingTransactions Quit signal received")
				pendingTxnTimer.Stop()
				return
			}
		}
	}()
}
