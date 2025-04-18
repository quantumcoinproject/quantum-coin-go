package cachemanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"math/big"
	"strings"
)

const AccountDetailsKey = "account-%s"

type AccountDetails struct {
	Address                  string                `json:"address,omitempty"`
	AccType                  ethclient.AccountType `json:"accountType,omitempty"`
	Balance                  string                `json:"balance,omitempty"`
	Nonce                    uint64                `json:"nonce,omitempty"`
	Code                     []byte                `json:"code,omitempty"`
	LastRefreshedBlockNumber string                `json:"lastRefreshedBlockNumber,omitempty"`
}

func getAccountKey(address string) []byte {
	pageKey := fmt.Sprintf(AccountDetailsKey, strings.ToLower(address))
	return []byte(pageKey)
}

func (c *CacheManager) refreshAccount(blockNumber *big.Int, shouldRefreshNonce bool, address string, batch *ethdb.Batch) error {
	var account *AccountDetails
	account, err := c.getAccountFromDb(address)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			primordialAccount, err := c.primordialCache.getAccountFromCacheOrDb(address)
			if err != nil {
				log.Error("refreshAccount", "address", address)
				return err
			}
			account = &AccountDetails{
				Address: address,
				AccType: primordialAccount.AccType,
			}
			if primordialAccount.Code != nil {
				account.Code = make([]byte, len(primordialAccount.Code))
				copy(account.Code, primordialAccount.Code)
			}
		} else {
			return err
		}
	}

	addr := common.HexToAddress(address)
	balance, err := c.client.BalanceAt(context.Background(), addr, blockNumber)
	if err != nil {
		log.Error("refreshAccount BalanceAt", "address", address)
		return err
	}
	account.Balance = hexutil.EncodeBig(balance)

	if shouldRefreshNonce {
		var nonce *hexutil.Big
		err = c.client.GetRpcClient().CallContext(context.Background(), &nonce, "eth_getTransactionCount", common.HexToAddress(address), blockNumber.Uint64())
		if err != nil {
			log.Error("refreshAccount eth_getTransactionCount", "address", address, "error", err)
			return err
		}
		account.Nonce = nonce.ToInt().Uint64()
	}

	err = c.putAccountInDb(account, batch)
	if err != nil {
		log.Error("refreshAccount putAccountInDb", "address", address, "error", err)
		return err
	}

	return nil
}

func (c *CacheManager) getAccountFromDb(address string) (*AccountDetails, error) {
	keyBlob := getAccountKey(address)
	blob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountFromDb not found", "address", address)
			return nil, nil
		} else {
			log.Error("getAccountFromDb", "address", address, "error", err)
			return nil, err
		}
	}

	item := AccountDetails{}
	err = json.Unmarshal(blob, &item)
	if err != nil {
		log.Error("getAccountFromDb", "error", err, "address", address, "error", err)
		return nil, err
	}

	return &item, nil
}

func (c *CacheManager) putAccountInDb(item *AccountDetails, batch *ethdb.Batch) error {
	txnBatch := *batch
	keyBlob := getAccountKey(item.Address)
	log.Info("putAccountInDb", "address", item.Address)

	blob, err := json.Marshal(item)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountInDb", "error", err, "Hash", item.Address)
		return err
	}

	return nil
}
