package cachemanager

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/conversion"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
	"io"
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

var (
	errorSignature = []byte{0x08, 0xc3, 0x79, 0xa0} // Keccak256("Error(string)")[:4]
	abiString, _   = abi.NewType("string", "", nil)
)

type PrimordialCache struct {
	cacheDir   string
	nodeUrl    string
	cacheLock  sync.Mutex
	cacheDb    ethdb.Database
	client     *ethclient.Client
	addressMap map[string]*PrimordialAccountDetails
	cancelChan *chan bool
	genesis    *core.Genesis
}

func NewPrimordialCache(cacheDir string, nodeUrl string, genesis *core.Genesis, cancelChan *chan bool) (*PrimordialCache, error) {
	pCache := &PrimordialCache{
		nodeUrl:    nodeUrl,
		cacheDir:   cacheDir,
		addressMap: make(map[string]*PrimordialAccountDetails),
		cancelChan: cancelChan,
		genesis:    genesis,
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
		blockClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("PrimordialCache initialize blockClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		chainID, err = blockClient.NetworkID(context.Background())
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

	blockNumberToGet := startBlockNumber
	isLagging := true

	go func() {
		for {
			select {
			case <-*c.cancelChan:
				blockTimer.Stop()
				log.Info("PrimordialCache downloadBlocks Quit signal received")
				return

			case <-blockTimer.C:
				log.Debug("PrimordialCache downloadBlock BlockByNumber", "PrimordialBlock Number ", blockNumberToGet)

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

				validatorList, err := c.client.ListValidators(context.Background(), blockNumBig)
				if err != nil {
					log.Error("PrimordialCache downloadBlock ListValidators", "error", err, "blockNumberToGet", blockNumberToGet)
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

				internalBlockData := &PrimordialBlockData{
					Block:                     fromNativeBlock(block),
					ConsensusData:             consensusData,
					ZeroAddressBalance:        zeroAddressBalance,
					StakingContractBalance:    stakingContractBalance,
					ConversionContractBalance: conversionContractBalance,
					TransactionList:           make([]*TransactionDetailsExpanded, len(block.Transactions())),
					ValidatorList:             validatorList,
				}

				accountsInvolved := make(map[string]bool)
				for i, tx := range block.Transactions() {
					var txnDetailsExpanded TransactionDetailsExpanded
					txnDetailsExpanded.Transaction = fromNativeTransaction(tx)

					msg, err := tx.AsMessage(types.NewLondonSigner(chainID))
					if err != nil {
						log.Error("PrimordialCache AsMessage", "error", err)
						delayNumber = int64(3000 * time.Millisecond)
						blockTimer.Reset(time.Duration(delayNumber))
						continue
					}

					fromAddress := msg.From().HexLower()
					accountsInvolved[fromAddress] = true

					var toAddress string
					if tx.To() != nil {
						toAddress = tx.To().HexLower()
					}

					receipt, err := c.client.TransactionReceipt(context.Background(), tx.Hash())
					if err != nil {
						log.Error("PrimordialCache TransactionReceipt", "error", err)
						delayNumber = int64(3000 * time.Millisecond)
						blockTimer.Reset(time.Duration(delayNumber))
						continue
					}
					txnDetailsExpanded.Receipt = fromNativeReceipt(receipt)

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
					var internalTxnList []*InternalTransactionDetail
					if internalTransactions != nil {
						internalTxnList = flattenInternalTransactionDetails(internalTransactions)
						txnDetailsExpanded.InternalTransactions = internalTxnList
					}

					if receipt.Status == 1 {
						if tx.To() == nil {
							accountsInvolved[receipt.ContractAddress.HexLower()] = true
						} else {
							accountsInvolved[toAddress] = true
						}

						if internalTransactions != nil {
							for _, iTxn := range internalTxnList {
								accountsInvolved[strings.ToLower(iTxn.From)] = true
								accountsInvolved[strings.ToLower(iTxn.To)] = true
							}
						}
					} else {
						if internalTransactions != nil {
							if len(internalTransactions.Output) > 0 {
								txnDetailsExpanded.RevertReason, err = unpackError(internalTransactions.Output)
								if err != nil {
									log.Warn("unpackError", "error", err)
									//don't skip processing
								}
							} else {
								txnDetailsExpanded.RevertReason = internalTransactions.Error
							}
						}
					}

					internalBlockData.TransactionList[i] = &txnDetailsExpanded
				}

				txnBatch := c.cacheDb.NewBatch()
				if blockNumBig.Uint64() == 1 {
					err = c.refreshGenesis(blockNumBig, &txnBatch)
					if err != nil {
						log.Error("PrimordialCache refreshGenesis", "error", err)
						delayNumber = int64(3000 * time.Millisecond)
						blockTimer.Reset(time.Duration(delayNumber))
						continue
					}
				}

				blockKey := []byte(LastInternalBlockKey)
				err = txnBatch.Put(blockKey, common.Uint64ToBytes(blockNumBig.Uint64()))
				if err != nil {
					log.Error("PrimordialCache txnBatch.Put LastInternalBlockKey", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				for addr, _ := range accountsInvolved {
					_, err := c.UpsertAccount(addr, blockNumBig, &txnBatch)
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

				retBlockData, err := c.getBlockFromDb(blockNumBig.Uint64())
				if err != nil {
					log.Error("PrimordialCache getBlockFromDb", "error", err)
					delayNumber = int64(3000 * time.Millisecond)
					blockTimer.Reset(time.Duration(delayNumber))
					continue
				}

				if retBlockData.Block.Number.Cmp(block.Number()) != 0 {
					log.Error("PrimordialCache block compare failed")
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

func (c *PrimordialCache) putBlockInDb(blockDetails *PrimordialBlockData, batch *ethdb.Batch) error {
	txnBatch := *batch
	blockNumber := blockDetails.Block.Number.Uint64()
	log.Debug("PrimordialCache putBlockInDb", "blockNumber", blockNumber)

	blob, err := json.Marshal(blockDetails)
	if err != nil {
		return err
	}
	blockKey, blockKeyBlob := c.getBlockKey(blockNumber)

	compressed, err := compress(blob)
	err = txnBatch.Put(blockKeyBlob, compressed)
	if err != nil {
		log.Error("PrimordialCache putBlockInDb", "error", err, "blockKey", blockKey)
		return err
	}
	log.Debug("compression stat", "compressed size", len(compressed), "uncompressed size", len(blob), "blockKey", blockKey)

	return nil
}

func (c *PrimordialCache) getBlockFromDb(blockNumber uint64) (*PrimordialBlockData, error) {
	blockKey, blockKeyBlob := c.getBlockKey(blockNumber)
	log.Debug("PrimordialCache getBlockFromDb", "blockNumber", blockNumber, "blockKey", blockKey)

	db := c.cacheDb
	compressed, err := db.Get([]byte(blockKeyBlob))
	if err != nil {
		return nil, err
	}
	uncompressed, err := decompress(compressed)
	if err != nil {
		return nil, err
	}

	var internalBlockData PrimordialBlockData
	err = json.Unmarshal(uncompressed, &internalBlockData)
	if err != nil {
		return nil, err
	}

	return &internalBlockData, nil
}

func (c *PrimordialCache) GetBlock(blockNumber uint64) (*PrimordialBlockData, error) {
	return c.getBlockFromDb(blockNumber)
}

func (c *PrimordialCache) UpsertAccount(address string, blockNumber *big.Int, batch *ethdb.Batch) (*PrimordialAccountDetails, error) {
	accountDetails, err := c.getAccountFromCacheOrDb(address)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			accType, byteCode, err := c.client.GetAccountType(common.HexToAddress(address), blockNumber)
			if err != nil {
				return nil, err
			}
			acc := &PrimordialAccountDetails{
				Address: address,
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
func (c *PrimordialCache) getAccountFromCacheOrDb(addr string) (*PrimordialAccountDetails, error) {
	var accountDetails *PrimordialAccountDetails

	accountDetails, ok := c.addressMap[addr]
	if ok == true {
		log.Trace("getAccountFromCacheOrDb return from in memory cache", "address", addr)
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

func (c *PrimordialCache) refreshGenesis(blockNumber *big.Int, batch *ethdb.Batch) error {
	for addr, _ := range c.genesis.Alloc {
		accType, code, err := c.client.GetAccountType(addr, blockNumber)
		if err != nil {
			log.Error("PrimordialCache refreshGenesis GetAccountType", "error", err)
			return err
		}
		accountDetails := &PrimordialAccountDetails{
			Address: addr.HexLower(),
			AccType: accType,
		}
		if code != nil {
			accountDetails.Code = make([]byte, 0)
			copy(accountDetails.Code, code)
		}
		err = c.putAccountInCacheAndDb(accountDetails, batch)
		if err != nil {
			log.Error("PrimordialCache refreshGenesis putAccountInCacheAndDb", "error", err)
			return err
		}
	}

	return nil
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

func (c *PrimordialCache) Close() error {
	*c.cancelChan <- true
	c.client.Close()

	cacheDb := c.cacheDb
	err := cacheDb.Close()
	if err != nil {
		log.Debug("PrimordialCache cache manager account transaction db close error", "err", err)
		return err
	}
	log.Info("PrimordialCache closed")
	return nil
}

func unpackError(output string) (string, error) {
	output = strings.Replace(output, "0x", "", 1)
	result := common.Hex2Bytes(output)
	if len(result) < 5 {
		return "", errors.New("invalid error signature")
	}
	if !bytes.Equal(result[:4], errorSignature) {
		return "", errors.New("incorrect error signature")
	}
	unpacked, err := abi.Arguments{{Type: abiString}}.UnpackValues(result[4:])
	if err != nil {
		return "", errors.New("UnpackValues failed")
	}
	if len(unpacked) == 0 {
		return "", errors.New("unexpected unpacked length")
	}
	return unpacked[0].(string), nil
}

func compress(uncompressed []byte) ([]byte, error) {
	compressedData := new(bytes.Buffer)
	compressor, err := flate.NewWriter(compressedData, 9)
	if err != nil {
		return nil, err
	}
	_, err = compressor.Write(uncompressed)
	if err != nil {
		return nil, err
	}
	err = compressor.Close()
	if err != nil {
		return nil, err
	}
	return compressedData.Bytes(), nil
}

func decompress(compressed []byte) ([]byte, error) {
	compressedData := new(bytes.Buffer)
	compressedData.Write(compressed)
	decompressedData := new(bytes.Buffer)
	decompressor := flate.NewReader(compressedData)
	_, err := io.Copy(decompressedData, decompressor)
	if err != nil {
		return nil, err
	}
	err = decompressor.Close()
	if err != nil {
		return nil, err
	}
	return decompressedData.Bytes(), nil
}
