package cachemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"math/big"
	"strings"
)

const AccountTokenKey = "account-token-count-%s-%s"    //%s is account address, %s is contract address
const AccountTokenCountKey = "account-token-count-%s"  //%s is account address
const AccountTokenPageKey = "account-token-list-%s-%d" //%s is account address, %d is page number

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

// Parses and stores list of tokens and balance for each token
func (c *CacheManager) processAccountTokenTransfers(tokenTransfers []*LogTransfer, blockNum *big.Int, batch *ethdb.Batch) error {
	for _, t := range tokenTransfers {
		contractAddress := t.ContractAddress.HexLower()
		tokenDetails, err := c.getTokenDetailsInternal(contractAddress)
		if err != nil {
			log.Error("CacheManager processAccountTokenTransfers getTokenDetailsInternal", "contractAddress", contractAddress, "error", err)
			return err
		}

		tokenBalance, err := c.client.GetAccountTokenBalance(t.ContractAddress, t.From)
		if err != nil {
			if errors.Is(err, ethclient.NotATokenError) {
				log.Error("CacheManager processAccountTokenTransfers GetAccountTokenBalance from", "contractAddress", contractAddress, "error", err, "from", t.From)
				return err
			}
		} else {
			err = c.processAccountTokenTransfer(t.From, tokenDetails, tokenBalance, batch)
			if err != nil {
				log.Error("CacheManager processAccountTokenTransfers processAccountTokenTransfer from", "contractAddress", contractAddress, "error", err, "from", t.From)
				return err
			}
		}

		tokenBalance, err = c.client.GetAccountTokenBalance(t.ContractAddress, t.To)
		if err != nil {
			if errors.Is(err, ethclient.NotATokenError) {
				log.Error("CacheManager processAccountTokenTransfers GetAccountTokenBalance from", "contractAddress", contractAddress, "error", err, "to", t.To)
				return err
			}
		} else {
			err = c.processAccountTokenTransfer(t.To, tokenDetails, tokenBalance, batch)
			if err != nil {
				log.Error("CacheManager processAccountTokenTransfers processAccountTokenTransfer to", "contractAddress", contractAddress, "error", err, "to", t.To)
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
		log.Error("CacheManager putAccountTokenInDb", "address", address, "contractAddress", contractAddress)
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

	log.Debug("CacheManager processAccountTokenTransfer", "address", address, "tokenCount", tokenCount, "newTokenCount", newTokenCount)

	if newTokenCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
		accountTokenList.Tokens = make([]AccountTokenSummary, 0)
		accountTokenList.Address = address
		log.Debug("CacheManager processAccountTokenTransfer", "address", address, "newTokenCount", newTokenCount)
	} else {
		//Load current state form the cache
		tokenPageCount := getPageCount(newTokenCount)
		pageKey, tokenPageKey := getAccountTokenPageKey(address, tokenPageCount)

		log.Debug("processAccountTokenTransfer loading from cache", "address", address, "newTokenCount", newTokenCount, "tokenPageCount", tokenPageCount, "pageKey", pageKey)

		accountTokenListBlob, err := c.cacheDb.Get(tokenPageKey)
		if err != nil {
			log.Error("CacheManager cacheDb.Get tokenPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(accountTokenListBlob, &accountTokenList)
		if err != nil {
			log.Error("CacheManager json.Unmarshal accountTokenListBlob", "error", err, "address", address, "pageKey", pageKey)
			return err
		}

		if strings.ToLower(accountTokenList.Address) != address {
			return errors.New("unexpected address")
		}

		if accountTokenList.Tokens == nil {
			return errors.New("unexpected tokens is nul")
		}

		if len(accountTokenList.Tokens) != int(tokenCount%PageSize) {
			log.Error("CacheManager unexpected token count from address", "actual", len(accountTokenList.Tokens), "expected", int(tokenCount%PageSize), "tokenCount", tokenCount)
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
		log.Error("CacheManager json.Marshal accountTokenListBlob", "error", err)
		return err
	}

	tokenPageCount := getPageCount(newTokenCount)
	log.Debug("tokenPageCount", tokenPageCount, "newTokenCount", newTokenCount)
	key, tokenPageKeyBlob := getAccountTokenPageKey(address, tokenPageCount)
	err = txnBatch.Put(tokenPageKeyBlob, accountTokenListBlob)
	if err != nil {
		log.Error("CacheManager txnBatch.Put accountTokenListBlob", "error", err, "key", key)
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

	log.Debug("CacheManager inserted account token list", "newTokenCount", newTokenCount, "tokenPageCount", tokenPageCount, "address", address)

	return nil
}

func getAccountTokenPageKey(address string, page uint64) (key string, blob []byte) {
	log.Debug("getAccountTokenPageKey", "address", address, "page", page)
	key = fmt.Sprintf(AccountTokenPageKey, address, page)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTokenPage(address string, pageNumber uint64) (*AccountTokenList, error) {
	accountTokenPageKey, keyBlob := getAccountTokenPageKey(address, pageNumber)
	accountTokenPageBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTokenPage not found", "address", address, "accountTokenPageKey", accountTokenPageKey)
			return nil, nil
		} else {
			log.Error("getAccountTokenCount cacheDb.Get address", "address", address, "accountTokenPageKey", accountTokenPageKey, "error", err)
			return nil, err
		}
	} else {
		accountTokenList := AccountTokenList{}
		err = json.Unmarshal(accountTokenPageBlob, &accountTokenList)
		if err != nil {
			log.Error("getAccountTokenPage", "error", err, "address", address, "pageNumber", pageNumber, "accountTokenPageKey", accountTokenPageKey)
			return nil, err
		}

		log.Info("getAccountTokenPage", "address", address, "accountTokenPageKey", accountTokenPageKey, "accountTokenList", accountTokenList)

		return &accountTokenList, nil
	}
}

func (c *CacheManager) putAccountTokenPage(address string, accountTokenList *AccountTokenList, pageNumber uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTokenPageKey, keyBlob := getAccountTokenPageKey(address, pageNumber)
	log.Info("putAccountTokenCount", "address", address, "accountTokenPageKey", accountTokenPageKey, "pageNumber", pageNumber)

	blob, err := json.Marshal(accountTokenList)
	if err != nil {
		return err
	}

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTokenCount address", "error", err, "address", address, "pageNumber", pageNumber)
		return err
	}

	return nil
}

func (c *CacheManager) getAccountTokenDetailsKey(accountAddress string, contractAddress string) (string, []byte) {
	key := fmt.Sprintf(AccountTokenKey, strings.ToLower(accountAddress), strings.ToLower(contractAddress))
	keyBlob := []byte(key)
	return key, keyBlob
}

func (c *CacheManager) GetAccountTokenDetails(accountAddress string, contractAddress string) (*AccountTokenSummary, error) {
	return c.getAccountTokenDetailsFromDb(accountAddress, contractAddress)
}

func (c *CacheManager) getAccountTokenDetailsFromDb(accountAddress string, contractAddress string) (*AccountTokenSummary, error) {
	key, keyBlob := c.getAccountTokenDetailsKey(accountAddress, contractAddress)
	db := c.cacheDb
	blob, err := db.Get(keyBlob)
	if err != nil {
		return nil, err
	}

	var details AccountTokenSummary
	err = json.Unmarshal(blob, &details)
	if err != nil {
		return nil, err
	}

	log.Debug("getAccountTokenDetailsFromDb", "accountAddress", accountAddress, "contractAddress", contractAddress, "key", key, "balance", details.TokenBalance)

	return &details, nil
}

func (c *CacheManager) putAccountTokenInDb(details *AccountTokenSummary, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(details)
	if err != nil {
		return err
	}

	key, keyBlob := c.getAccountTokenDetailsKey(details.AccountAddress, details.ContractAddress)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	log.Debug("putAccountTokenInDb", "key", key, "details TokenBalance", details.TokenBalance, "ContractAddress", details.ContractAddress)

	return nil
}

func (c *CacheManager) ListTokensByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTokensResponse, error) {
	listResponse := ListAccountTokensResponse{}
	address := accountAddress.HexLower()

	var pageCount uint64
	accountTokenCount, err := c.getAccountTokenCount(address)
	if err != nil {
		log.Error("ListTokensByAccount getAccountTokenCount", "error", err)
		if err.Error() == LevelDbNoTFoundErrMsg {
			return ListAccountTokensResponse{PageCount: 0}, nil
		}
		return ListAccountTokensResponse{}, err
	}
	if accountTokenCount%PageSize == 0 {
		pageCount = accountTokenCount / PageSize
	} else {
		pageCount = (accountTokenCount / PageSize) + 1
	}

	if pageCount == 0 {
		return ListAccountTokensResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListTokensByAccount", "address", address, "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "accountTokenCount", accountTokenCount)
	if pageNumber > pageCount {
		log.Error("ListTokensByAccount getAccountTokenCount", "pageNumber", pageNumber, "pageCount", pageCount)
		return ListAccountTokensResponse{PageCount: pageCount}, nil
	}

	pageKey, keyBlob := getAccountTokenPageKey(address, pageNumber)
	log.Debug("cache get", "key", pageKey)

	accountTTokenListBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		log.Error("ListTokensByAccount cacheDb.Get", "error", err)
		return ListAccountTokensResponse{}, err
	}
	var accountTokenList AccountTokenList
	err = json.Unmarshal(accountTTokenListBlob, &accountTokenList)
	if err != nil {
		log.Error("ListTokensByAccount json.Unmarshal accountTokenList", "error", err)
		return ListAccountTokensResponse{}, err
	}

	if strings.ToLower(accountTokenList.Address) != address {
		log.Error("unexpected address accountTokenList.Address", "address", address, "accountTokenList.Address", accountTokenList.Address)
		return ListAccountTokensResponse{}, errors.New("unexpected address accountTokenList.Address")
	}

	for i, item := range accountTokenList.Tokens {
		//get new info from db, since paginated cache doesn't store latest balance
		accountTokenSummary, err := c.getAccountTokenDetailsFromDb(address, strings.ToLower(item.ContractAddress))
		if err != nil {
			return ListAccountTokensResponse{}, err
		}
		accountTokenList.Tokens[i] = *accountTokenSummary
	}

	listResponse.Items = accountTokenList.Tokens
	listResponse.PageCount = pageCount

	return listResponse, nil
}
