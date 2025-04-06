package cachemanager

import "github.com/quantumcoinproject/quantum-coin-go/ethclient"

const AccountSummaryKey = "account-%s"

type AccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
}

/*func (c *CacheManager) getAccount(address common.Address, blockNumber *big.Int, batch *ethdb.Batch) (*AccountDetails, error) {
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
	result, _, err := c.client.GetAccountType(address, blockNumber)
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

	accountDetails = &AccountDetails{}
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
}*/
