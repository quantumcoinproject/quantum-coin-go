package cachemanager

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"math/big"
	"strings"
	"time"
)

func (c *CacheManager) getTransactionType(txn *PrimordialTransaction, receipt *PrimordialReceipt) (txnType TransactionType, err error) {
	txHash := txn.Hash
	if (txHash == receipt.TxHash) == false {
		return "", errors.New("hash mismatch between txn and receipt")
	}

	if txn.To != nil {
		acc, err := c.primordialCache.getAccountFromCacheOrDb(*txn.To)
		if err != nil {
			if err.Error() == LevelDbNoTFoundErrMsg && receipt.Status == 0 {
				if len(txn.Data) == 0 {
					return COIN_TRANSFER, nil
				} else {
					return SMART_CONTRACT, nil //todo: fix
				}
			}
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
		if receipt.Status == 0 {
			return NEW_SMART_CONTRACT, nil
		}
		acc, err := c.primordialCache.getAccountFromCacheOrDb(receipt.ContractAddress)
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
	}
}

func (c *CacheManager) processByCacheManager(internalBlockData *PrimordialBlockData, runningSummary *BlockchainDetails) error {
	var err error
	block := internalBlockData.Block
	blockNum := block.Number
	blockNumber := blockNum.Uint64()

	log.Debug("CacheManager processByCacheManager", "blockNumber", blockNumber)

	txnBatch := c.cacheDb.NewBatch()

	blockInfo := fromPrimordialBlockData(internalBlockData)

	//Add to block cache
	err = c.processBlock(blockInfo, &txnBatch)
	if err != nil {
		log.Error("CacheManager processBlock", "error", err)
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
	txnList := make([]*TransactionDetails, len(internalBlockData.TransactionList))
	senderList := make(map[string]bool)

	for i, tx := range internalBlockData.TransactionList {
		log.Trace("CacheManager processByCacheManager", "transaction", tx.Transaction.Hash)
		accountsInvolved := make(map[string]bool)

		txHash := tx.Transaction.Hash
		txnFromMap, ok := txnMap[txHash]
		if ok == false {
			log.Error("CacheManager processByCacheManager txn not found in map", "hash", txHash)
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
		senderList[transaction.From] = true
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
		log.Trace("CacheManager processByCacheManager transaction", "gasUsed", gasUsed, "txnFee", txnFee, "hash", txHash)
		transaction.TxnFee = common.BigIntToHexString(txnFee)

		txType, err := c.getTransactionType(tx.Transaction, receipt)
		if err != nil {
			log.Error("CacheManager getTransactionType", "error", err, "tx", tx.Transaction.Hash)
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
						log.Error("CacheManager putTokenInDb", "error", err, "contractAddress", iTxn.To)
						return err
					}
				}
			}

			//Find all relevant token and internal token transactions, if any
			tokenTransfers, tokenApprovals, err := ParseTokenTransaction(tx.Transaction, receipt)
			if err != nil {
				log.Error("CacheManager ParseTokenTransaction", "err", err, "txn", tx.Transaction.Hash)
				return err
			}

			if tokenTransfers != nil && len(tokenTransfers) > 0 {
				err = c.processAccountTokenTransfers(tokenTransfers, blockNum, &txnBatch)
				if err != nil {
					log.Error("CacheManager processAccountTokenTransfers", "err", err, "txn", tx.Transaction.Hash)
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
					log.Error("CacheManager processByCacheManager missing", "from", transaction.From, "to (contract)", transaction.To)
					transaction.TransactionType = string(SMART_CONTRACT)
				} else {
					transaction.TokenTransaction.ContractAddress = tokenTransfers[0].ContractAddress.HexLower()
					transaction.TokenTransaction.TokenFromAddress = tokenTransfers[0].From.HexLower()
					transaction.TokenTransaction.TokenToAddress = tokenTransfers[0].To.HexLower()

					tokenDetails, err := c.getTokenDetailsInternal(transaction.TokenTransaction.ContractAddress) //token should already have been saved to db, when it was created
					if err != nil {
						log.Error("CacheManager getTokenDetailsInternal", "error", err)
						return err
					}
					transaction.TokenTransaction.TokenCount = hexutil.EncodeBig(tokenTransfers[0].Tokens)
					transaction.TokenTransaction.TokenName = tokenDetails.Name
					transaction.TokenTransaction.TokenSymbol = tokenDetails.Symbol
				}
			}

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
		}

		_, ok = liveAccountTxnMap[tx.Transaction.From]
		if ok == false {
			liveAccountTxnMap[tx.Transaction.From] = make([]*TransactionDetails, 0)
		}
		liveAccountTxnMap[tx.Transaction.From] = append(liveAccountTxnMap[tx.Transaction.From], &transaction)

		accountsInvolved[tx.Transaction.From] = true

		//Loop through all accounts and update account cache
		for account, _ := range accountsInvolved {
			_, err = c.primordialCache.getAccountFromCacheOrDb(account)
			if err != nil {
				log.Error("CacheManager getAccountFromCacheOrDb", "account", account)
				return err
			}
		}

		txnList[i] = &transaction
	}

	err = c.processTransactions(&txnList, &txnBatch)
	if err != nil {
		log.Error("CacheManager processTransactions", "error", err)
		return err
	}

	blockTime := time.Unix(int64(block.Time), 0)
	err = c.incrementDailyBlockDetailsInDb(blockTime, &txnBatch)
	if err != nil {
		log.Error("CacheManager incrementDailyBlockDetailsInDb", "error", err)
		return err
	}

	err = c.incrementDailySpecificValidatorDetailsInDb(blockInfo, blockTime, &txnBatch)
	if err != nil {
		log.Error("CacheManager incrementDailySpecificValidatorDetailsInDb", "error", err)
		return err
	}

	err = c.incrementDailyTransactionDetailsInDb(uint64(len(txnList)), blockTime, &txnBatch)
	if err != nil {
		log.Error("CacheManager incrementDailyTransactionDetailsInDb", "error", err)
		return err
	}

	err = c.processBlockTransactions(blockInfo, &txnList, &txnBatch)
	if err != nil {
		log.Error("CacheManager processBlockTransactions", "error", err)
		return err
	}

	if blockNumber == 1 {
		err = c.refreshGenesis(&txnBatch)
		if err != nil {
			log.Error("CacheManager refreshGenesis", "error", err)
			return err
		}
	}

	for k, v := range liveAccountTxnMap {
		err = c.processAccountTransactions(k, &v, &txnBatch)
		if err != nil {
			log.Error("CacheManager processAccountTransaction", "error", err, "address", k)
			return err
		}
		_, shouldUpdateNonce := senderList[k]
		err = c.refreshAccount(blockNum, shouldUpdateNonce, k, &txnBatch)
		if err != nil {
			log.Error("CacheManager refreshAccount", "error", err, "address", k)
			return err
		}
	}

	_, ok := liveAccountTxnMap[staking.STAKING_CONTRACT]
	if ok || blockNumber%32 == 0 {
		err = c.refreshValidators(blockNum, &txnBatch)
		if err != nil {
			log.Error("CacheManager refreshValidators", "error", err)
			return err
		}

		stakingContractBalance, err := c.refreshStakingDetails(blockNum, &txnBatch)
		if err != nil {
			log.Error("CacheManager refreshStakingDetails", "error", err)
			return err
		}
		runningSummary.StakedCoins = common.BigIntToHexString(stakingContractBalance)
	}

	if c.enableExtendedApis {
		err = c.updateSummary(internalBlockData, runningSummary, &txnBatch)
		if err != nil {
			log.Error("CacheManager updateSummary", "error", err)
			return err
		}
	}

	err = txnBatch.Write()
	if err != nil {
		log.Error("CacheManager processByCacheManager txnBatch Write", "error", err)
		return err
	}

	return nil
}
