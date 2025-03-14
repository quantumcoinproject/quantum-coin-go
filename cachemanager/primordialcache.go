package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/consensus/proofofstake"
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/systemcontracts/conversion"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const LastInternalBlockKey = "internal-last-block"
const InternalBlockDetailsKey = "internal-block-%d"
const PrimordialAccountSummaryKey = "paccount-%s"

type PrimordialAccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
	Code    []byte                `json:"code,omitempty"` //for contracts only
}

type PrimordialCache struct {
	cacheDir   string
	nodeUrl    string
	cacheLock  sync.Mutex
	cacheDb    ethdb.Database
	client     *ethclient.Client
	addressMap map[string]*PrimordialAccountDetails
}

type TransactionDetailsExpanded struct {
	InternalTransactions []*InternalTransactionDetail `json:"internalTransactions,omitempty"`
	Receipt              *types.Receipt               `json:"receipt,omitempty"`
}

type InternalBlockData struct {
	Block                     *types.Block                  `json:"block,omitempty"`
	ConsensusData             *proofofstake.ConsensusData   `json:"consensusData,omitempty"`
	ZeroAddressBalance        *big.Int                      `json:"zeroAddressBalance,omitempty"`
	StakingContractBalance    *big.Int                      `json:"stakingContractBalance,omitempty"`
	ConversionContractBalance *big.Int                      `json:"conversionContractBalance,omitempty"`
	TransactionList           []*TransactionDetailsExpanded `json:"transactionList,omitempty"`
}

func NewPrimordialCache(cacheDir string, nodeUrl string) (*PrimordialCache, error) {
	pCache := &PrimordialCache{
		nodeUrl:    nodeUrl,
		cacheDir:   cacheDir,
		addressMap: make(map[string]*PrimordialAccountDetails),
	}

	var err error
	err = pCache.initialize()
	if err != nil {
		return nil, err
	}

	err = pCache.start()
	if err != nil {
		return nil, err
	}

	return pCache, nil
}

func (c *PrimordialCache) initialize() error {
	log.Info("PrimordialCache initialize", "cacheDir", c.cacheDir, "nodeUrl", c.nodeUrl)

	primordialCacheFilePath := filepath.Join(c.cacheDir, "primordialcache.db")
	primordialCacheDb, err := rawdb.NewLevelDBDatabase(primordialCacheFilePath, 64, 0, "", false)
	if err != nil {
		return err
	}
	c.cacheDb = primordialCacheDb

	c.addressMap = make(map[string]*PrimordialAccountDetails)
	c.addressMap[staking.STAKING_CONTRACT] = &PrimordialAccountDetails{Address: staking.STAKING_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}
	c.addressMap[conversion.CONVERSION_CONTRACT] = &PrimordialAccountDetails{Address: conversion.CONVERSION_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}

	return nil
}

func (c *PrimordialCache) clientInitialize() {
	for {
		client, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("PrimordialCache initialize client Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		blockClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("PrimordialCache initialize blockClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		chainID, err = client.NetworkID(context.Background())
		if err != nil {
			log.Error("PrimordialCache initialize NetworkID", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		c.client = blockClient

		break
	}
}

func (c *PrimordialCache) start() error {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	blockNumber, err := c.getLastBlockNumberByDb(LastInternalBlockKey)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Warn("PrimordialCache First time start")
			blockNumber = 0
		} else {
			log.Error("PrimordialCacheGetLastBlockByDb", "err", err.Error())
			return err
		}
	}

	c.clientInitialize()
	c.downloadBlocks(int64(blockNumber + 1))

	return nil
}

func (c *PrimordialCache) downloadBlocks(startBlockNumber int64) {
	delayNumber := int64(100 * time.Millisecond)
	blockTimer := time.NewTimer(time.Duration(delayNumber))
	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	blockNumberToGet := startBlockNumber
	isLagging := true

	go func() {
		for {
			select {
			case <-blockTimer.C:
				log.Info("PrimordialCache downloadBlock BlockByNumber", "Block Number ", blockNumberToGet)

				if blockNumberToGet == startBlockNumber || blockNumberToGet%int64(50) == 0 {
					var latestBlockNumberHex *hexutil.Uint64
					err := c.client.GetRpcClient().CallContext(context.Background(), &latestBlockNumberHex, "eth_blockNumber")
					if err != nil {
						log.Error("PrimordialCache downloadBlocks eth_blockNumber", "error", err)
					} else {
						latestBlockNumber, err := hexutil.DecodeBig(latestBlockNumberHex.String())
						if err != nil {
							log.Error("PrimordialCache downloadBlocks DecodeBig", "error", err)
						} else {
							if uint64(blockNumberToGet) < latestBlockNumber.Uint64() && latestBlockNumber.Uint64()-uint64(blockNumberToGet) > 50 {
								isLagging = true
								log.Info("PrimordialCache downloadBlocks Lagging behind latest blocks", "latestBlockNumber", latestBlockNumber.Uint64(), "blockNumberToGet", blockNumberToGet)
							} else {
								isLagging = false
							}
						}
					}
				}

				blockNumBig := big.NewInt(blockNumberToGet)
				block, err := c.client.BlockByNumber(context.Background(), blockNumBig)
				if err != nil {
					if err.Error() != NotFoundErrMsg {
						log.Error("PrimordialCache downloadBlock BlockByNumber", "error", err, "blockNumberToGet", blockNumberToGet)
					}
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				consensusData, err := c.client.GetBlockConsensusData(context.Background(), blockNumBig)
				if err != nil {
					log.Error("PrimordialCache downloadBlock GetBlockConsensusData", "error", err, "blockNumberToGet", blockNumberToGet)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				zeroAddressBalance, err := c.client.BalanceAt(context.Background(), common.ZERO_ADDRESS, blockNumBig)
				if err != nil {
					log.Error("PrimordialCache downloadBlock BalanceAt zeroAddressBalance", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				stakingContractBalance, err := c.client.BalanceAt(context.Background(), staking.STAKING_CONTRACT_ADDRESS, blockNumBig)
				if err != nil {
					log.Error("PrimordialCache downloadBlock BalanceAt stakingContractBalance", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				conversionContractBalance, err := c.client.BalanceAt(context.Background(), conversion.CONVERSION_CONTRACT_ADDRESS, blockNumBig)
				if err != nil {
					log.Error("PrimordialCache downloadBlock BalanceAt stakingContractBalance", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				internalBlockData := &InternalBlockData{
					Block:                     block,
					ConsensusData:             consensusData,
					ZeroAddressBalance:        zeroAddressBalance,
					StakingContractBalance:    stakingContractBalance,
					ConversionContractBalance: conversionContractBalance,
					TransactionList:           make([]*TransactionDetailsExpanded, len(block.Transactions())),
				}

				accountsInvolved := make(map[string]bool)
				for i, tx := range block.Transactions() {
					var txnDetailsExpanded TransactionDetailsExpanded

					msg, err := tx.AsMessage(types.NewLondonSigner(chainID))
					if err != nil {
						log.Error("PrimordialCache AsMessage", "error", err)
						delayNumber = int64(3000 * time.Millisecond)
						blockTimer.Reset(time.Duration(delayNumber))
						continue
					}

					fromAddress := strings.ToLower(msg.From().Hex())
					accountsInvolved[fromAddress] = true

					var toAddress string
					if tx.To() != nil {
						toAddress = strings.ToLower(tx.To().Hex())
					}

					receipt, err := c.client.TransactionReceipt(context.Background(), tx.Hash())
					if err != nil {
						log.Error("PrimordialCache TransactionReceipt", "error", err)
						delayNumber = int64(3000 * time.Millisecond)
						blockTimer.Reset(time.Duration(delayNumber))
						continue
					}

					if receipt.Status == 1 {
						if tx.To() == nil {
							accountsInvolved[strings.ToLower(receipt.ContractAddress.Hex())] = true
						} else {
							accountsInvolved[toAddress] = true
						}

						internalTransactions, err := c.client.GetInternalTransactions(context.Background(), tx.Hash())
						if err != nil {
							log.Warn("PrimordialCache GetInternalTransactions", "err", err, "txn", tx.Hash())
							if errors.Is(err, ethclient.TracingGasError) {

							} else {
								delayNumber = int64(3000 * time.Millisecond)
								blockTimer.Reset(time.Duration(delayNumber))
								continue
							}
						}

						if internalTransactions != nil {
							internalTxnList := flattenInternalTransactionDetails(internalTransactions)

							for _, iTxn := range internalTxnList {
								accountsInvolved[strings.ToLower(iTxn.From)] = true
								accountsInvolved[strings.ToLower(iTxn.To)] = true
							}

							txnDetailsExpanded.InternalTransactions = internalTxnList
						}

						txnDetailsExpanded.Receipt = receipt
					}

					internalBlockData.TransactionList[i] = &txnDetailsExpanded
				}

				txnBatch := c.cacheDb.NewBatch()
				for addr, _ := range accountsInvolved {
					_, err := c.UpsertAccount(common.HexToAddress(addr), blockNumBig, &txnBatch)
					if err != nil {
						log.Error("PrimordialCache getAccountFromCacheOrDb", "error", err, "address", addr)
						delayNumber = int64(3000 * time.Millisecond)
						blockTimer.Reset(time.Duration(delayNumber))
						continue
					}
				}

				err = c.putBlockInDb(internalBlockData, &txnBatch)
				if err != nil {
					log.Error("PrimordialCache putBlockInDb", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				err = txnBatch.Write()
				if err != nil {
					log.Error("PrimordialCache txnBatch Write", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}
				blockNumberToGet = blockNumberToGet + 1

				if isLagging {
					delayNumber = 0
				} else {
					delayNumber = int64(100 * time.Millisecond)
				}
				blockTimer.Reset(time.Duration(delayNumber))

			case <-cancel:
				blockTimer.Stop()
				log.Info("PrimordialCache downloadBlocks Quit signal received")
				return
			}
		}
	}()
}

func (c *PrimordialCache) getLastBlockNumberByDb(blockKey string) (uint64, error) {
	db := c.cacheDb
	mySlice, err := db.Get([]byte(blockKey))
	if err != nil {
		return uint64(0), err
	}

	blockNumber := common.BytesToUint64(mySlice)

	return blockNumber, nil
}

func (c *PrimordialCache) getBlockKey(blockNumber uint64) (key string, blob []byte) {
	key = fmt.Sprintf(InternalBlockDetailsKey, blockNumber)
	blob = []byte(key)
	return key, blob
}

func (c *PrimordialCache) putBlockInDb(blockDetails *InternalBlockData, batch *ethdb.Batch) error {
	txnBatch := *batch
	blockNumber := blockDetails.Block.Number().Uint64()
	log.Info("PrimordialCache putBlockInDb", "blockNumber", blockNumber)

	blob, err := json.Marshal(blockDetails)
	if err != nil {
		return err
	}
	blockKey, blockKeyBlob := c.getBlockKey(blockNumber)

	err = txnBatch.Put(blockKeyBlob, blob)
	if err != nil {
		log.Error("PrimordialCache putBlockInDb", "error", err, "blockKey", blockKey)
		return err
	}

	return nil
}

func (c *PrimordialCache) getBlockFromDb(blockNumber uint64) (*InternalBlockData, error) {
	blockKey, blockKeyBlob := c.getBlockKey(blockNumber)
	log.Info("PrimordialCache getBlockFromDb", "blockNumber", blockNumber, "blockKey", blockKey)

	var internalBlockData InternalBlockData
	err := json.Unmarshal(blockKeyBlob, &internalBlockData)
	if err != nil {
		return nil, err
	}

	return &internalBlockData, nil
}

func (c *PrimordialCache) GetBlock(blockNumber uint64) (*InternalBlockData, error) {
	return c.getBlockFromDb(blockNumber)
}

func (c *PrimordialCache) UpsertAccount(address common.Address, blockNumber *big.Int, batch *ethdb.Batch) (*PrimordialAccountDetails, error) {
	accountDetails, err := c.getAccountFromCacheOrDb(address)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			accType, byteCode, err := c.client.GetAccountType(address, blockNumber)
			if err != nil {
				return nil, err
			}
			acc := &PrimordialAccountDetails{
				Address: strings.ToLower(address.Hex()),
				AccType: accType,
				Code:    byteCode,
			}
			err = c.putAccountInCacheAndDb(acc, batch)
			if err != nil {
				return nil, err
			}
			return acc, err
		} else {
			return nil, err
		}
	}

	return accountDetails, err
}

// gets account from in-memory cache or persistent cache
func (c *PrimordialCache) getAccountFromCacheOrDb(address common.Address) (*PrimordialAccountDetails, error) {
	var accountDetails *PrimordialAccountDetails
	addr := strings.ToLower(address.Hex())

	accountDetails, ok := c.addressMap[addr]
	if ok == true {
		log.Trace("getAccountFromCacheOrDb return from in memory cache", "address", address)
		return accountDetails, nil
	}

	key := fmt.Sprintf(PrimordialAccountSummaryKey, addr)

	db := c.cacheDb
	accountBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	accountDetails = &PrimordialAccountDetails{}
	err = json.Unmarshal(accountBlob, accountDetails)
	if err != nil {
		return nil, err
	}

	return accountDetails, nil
}

// puts account in in-memory cache and in persistent store
func (c *PrimordialCache) putAccountInCacheAndDb(accountDetails *PrimordialAccountDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(accountDetails)
	if err != nil {
		return err
	}

	accountAddress := strings.ToLower(accountDetails.Address)
	key := fmt.Sprintf(PrimordialAccountSummaryKey, accountAddress)
	keyBlob := []byte(key)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	c.addressMap[accountDetails.Address] = accountDetails

	return nil
}
