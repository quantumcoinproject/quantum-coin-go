package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/consensus/proofofstake"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/relay/cachemanager/token"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/conversion"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
)

type CacheManager struct {
	cacheDir                 string
	nodeUrl                  string
	cacheLock                sync.Mutex
	cacheDb                  ethdb.Database
	client                   *ethclient.Client
	pendingTxClient          *ethclient.Client
	blockClient              *ethclient.Client
	enableExtendedApis       bool
	genesisCirculatingSupply string
	maxSupply                string
	pendingTxLock            sync.Mutex
	pendingTxMapLock         sync.RWMutex
	pendingTransactions      *map[string]map[string]map[string]*ethclient.TxPoolTransaction
	addressMap               map[string]*AccountDetails
	tokenMap                 map[string]*TokenDetails
	accountTokenMap          map[string]*map[string]bool //map[accountAddress]map[contractAddress]bool
	blockChan                chan *InternalBlockData
}

var SummaryKey = "summary"
var LastBlockKey = "last-block"
var AccountSummaryKey = "account-%s"
var AccountTxnCountKey = "account-txn-count-%s"                  //%s is account address
var AccountTransactionPageKey = "account-transaction-list-%s-%d" //%s is account address, %d is page number
var TokenDetailsKey = "erc20-%s"

var AccountTokenKey = "account-token-count-%s-%s" //%s is account address, %s is contract address

var AccountTokenCountKey = "account-token-count-%s"  //%s is account address
var AccountTokenPageKey = "account-token-list-%s-%d" //%s is account address, %d is page number

var AccountTokenTxnCountKey = "account-token-txn-count-%s"          //%s is account address
var AccountTokenTransactionPageKey = "account-token-txn-list-%s-%d" //%s is account address,%d is page number

var chainID *big.Int
var LevelDbNoTFoundErrMsg = "leveldb: not found"
var NotFoundErrMsg = "not found"

const TimeLayout = "2006-01-02T15:04:05Z"

const PageSize uint64 = 20

type TransactionType string

// List of TransactionType
const (
	COIN_TRANSFER      TransactionType = "CoinTransfer"
	NEW_TOKEN          TransactionType = "NewToken"
	TOKEN_TRANSFER     TransactionType = "TokenTransfer"
	NEW_SMART_CONTRACT TransactionType = "NewSmartContract"
	SMART_CONTRACT     TransactionType = "SmartContract"
)

type ContractType string

type AccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
}

type InternalTransactionDetailWithLevel struct {
	txn   *ethclient.InternalTransactionDetails
	level byte
}

type TokenTransfers struct {
	ContractAddress common.Address
	From            common.Address
	To              common.Address
	Tokens          *big.Int
}

type TokenApprovals struct {
	ContractAddress common.Address
	TokenOwner      common.Address
	Spender         common.Address
	Tokens          *big.Int
}

type InternalTransactionDetail struct {
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Value string `json:"value,omitempty"`
	Type  string `json:"type,omitempty"`
	Level byte   `json:"level,omitempty"`
}

type TransactionReceipt struct {
	BlockHash string `json:"blockHash,omitempty"`

	CumulativeGasUsed string `json:"cumulativeGasUsed,omitempty"`

	EffectiveGasPrice string `json:"effectiveGasPrice,omitempty"`

	GasUsed string `json:"gasUsed,omitempty"`

	Status string `json:"status,omitempty"`

	Hash string `json:"hash,omitempty"`

	Type string `json:"type,omitempty"`

	ContractAddress string `json:"contractAddress,omitempty"`
}

type TokenTransactionCompact struct {
	TokenFromAddress string `json:"tokenFromAddress,omitempty"`

	TokenToAddress string `json:"tokenToAddress,omitempty"`

	ContractAddress string `json:"contractAddress,omitempty"`

	TokenCount string `json:"tokenCount,omitempty"`

	TokenSymbol string `json:"tokenSymbol,omitempty"`

	TokenName string `json:"tokenName,omitempty"`
}

type TransactionDetails struct {
	BlockHash string `json:"blockHash,omitempty"`

	BlockNumber uint64 `json:"blockNumber,omitempty"`

	From string `json:"from,omitempty"`

	Gas string `json:"gas,omitempty"`

	GasPrice string `json:"gasPrice,omitempty"`

	Hash string `json:"hash,omitempty"`

	Input string `json:"input,omitempty"`

	Nonce uint64 `json:"nonce,omitempty"`

	To string `json:"to,omitempty"`

	Value string `json:"value,omitempty"`

	Receipt TransactionReceipt `json:"receipt,omitempty"`

	CreatedAt string `json:"createdAt,omitempty"`

	TxnFee string `json:"txnFee,omitempty"`

	TransactionType string `json:"transactionType,omitempty"`

	TokenTransaction TokenTransactionCompact `json:"tokenTransaction,omitempty"`
}

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

	TokenTransaction TokenTransactionCompact `json:"tokenTransaction,omitempty"`
}

type AccountTransactionList struct {
	Address      string                      `json:"address"`
	Transactions []AccountTransactionCompact `json:"transactions"`
}

type ListAccountTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount"`
	Items     []AccountTransactionCompact `json:"items"`
}

type AccountPendingTransactionCompact struct {
	Hash string `json:"hash,omitempty"`

	From string `json:"from,omitempty"`

	To string `json:"to,omitempty"`

	Value string `json:"value,omitempty"`

	Nonce uint64 `json:"nonce,omitempty"`
}

type ListAccountPendingTransactionsResponse struct {
	Items     []AccountPendingTransactionCompact `json:"items"`
	PageCount uint64                             `json:"pageCount"`
}

type BlockchainDetails struct {
	BlockNumber           uint64 `json:"blockNumber" gencodec:"required"`
	MaxSupply             string `json:"maxSupply" gencodec:"required"`
	TotalSupply           string `json:"totalSupply" gencodec:"required"`
	CirculatingSupply     string `json:"circulatingSupply" gencodec:"required"`
	BurntCoins            string `json:"burntCoins" gencodec:"required"`
	BlockRewardsCoins     string `json:"blockRewardsCoins" gencodec:"required"` //baseBlockRewardsCoins + TxnFeeRewardsCoins
	BaseBlockRewardsCoins string `json:"baseBlockRewardsCoins" gencodec:"required"`
	TxnFeeRewardsCoins    string `json:"txnFeeRewardsCoins" gencodec:"required"`
	TxnFeeBurntCoins      string `json:"txnFeeBurntCoins" gencodec:"required"`
	SlashedCoins          string `json:"slashedCoins" gencodec:"required"`
}

type GetBlockchainDetailsResponse struct {
	Result BlockchainDetails `json:"result" gencodec:"required"`
}

type TokenDetails struct {
	ContractAddress        string `json:"contractAddress,omitempty"`
	CreatorAddress         string `json:"creatorAddress,omitempty"`
	CreatedBlockNumber     uint64 `json:"createdBlockNumber,omitempty"`
	CreatedTransactionHash string `json:"createdTransactionHash,omitempty"`
	Name                   string `json:"name,omitempty"`
	Symbol                 string `json:"symbol,omitempty"`
	TotalSupply            string `json:"totalSupply,omitempty"`
	Decimals               string `json:"decimals,omitempty"`
}

type GetTokenDetailsResponse struct {
	Result TokenDetails `json:"result,omitempty"`
}

type AccountTokenSummary struct {
	AccountAddress  string `json:"accountAddress,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"`
	Name            string `json:"name,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	TokenBalance    string `json:"tokenBalance,omitempty"`
}

type AccountTokenList struct {
	Address string                `json:"address"`
	Tokens  []AccountTokenSummary `json:"tokens"`
}

type ListAccountTokensResponse struct {
	PageCount      uint64                `json:"pageCount"`
	AccountAddress string                `json:"accountAddress"`
	Items          []AccountTokenSummary `json:"items"`
}

type AccountTokenTransactionList struct {
	Address      string                      `json:"address"`
	Transactions []AccountTransactionCompact `json:"transactions"`
}

type ListAccountTokenTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount"`
	Items     []AccountTransactionCompact `json:"items"`
}

type InternalBlockData struct {
	Block              *types.Block
	ConsensusData      *proofofstake.ConsensusData
	ZeroAddressBalance *big.Int
}

func NewCacheManager(cacheDir string, nodeUrl string, enableExtendedApis bool, genesisFilePath string, maxSupply string) (*CacheManager, error) {
	cManager := &CacheManager{
		nodeUrl:            nodeUrl,
		cacheDir:           cacheDir,
		enableExtendedApis: enableExtendedApis,
		addressMap:         make(map[string]*AccountDetails),
		tokenMap:           make(map[string]*TokenDetails),
		accountTokenMap:    make(map[string]*map[string]bool),
	}

	var err error

	if enableExtendedApis {
		if len(maxSupply) == 0 {
			return nil, errors.New("max supply is nil")
		}
		maxSupplyBig, err := hexutil.DecodeBig(maxSupply)
		if err != nil {
			return nil, err
		}

		cManager.maxSupply = maxSupply

		genesisBytes, err := ioutil.ReadFile(genesisFilePath)
		if err != nil {
			log.Error("ReadFile", "error", err)
			return nil, err
		}

		genesis := core.Genesis{}
		err = json.Unmarshal(genesisBytes, &genesis)
		if err != nil {
			log.Error("Unmarshal", "error", err)
			return nil, err
		}

		genesisCirculatingSupply := big.NewInt(0)
		if genesis.Alloc != nil {
			for _, v := range genesis.Alloc {
				genesisCirculatingSupply = common.SafeAddBigInt(genesisCirculatingSupply, v.Balance)
			}
		}
		cManager.genesisCirculatingSupply = hexutil.EncodeBig(genesisCirculatingSupply)
		log.Error("genesis genesisCirculatingSupply", "genesisCirculatingSupply", params.WeiToEther(genesisCirculatingSupply), "maxSupply", params.WeiToEther(maxSupplyBig))
	}

	err = cManager.initialize()
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

	c.addressMap = make(map[string]*AccountDetails)
	c.addressMap[staking.STAKING_CONTRACT] = &AccountDetails{Address: staking.STAKING_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}
	c.addressMap[conversion.CONVERSION_CONTRACT] = &AccountDetails{Address: conversion.CONVERSION_CONTRACT, AccType: ethclient.ACCOUNT_TYPE_CONTRACT}

	return nil
}

func (c *CacheManager) clientInitialize() {
	for {
		client, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("initialize client Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		blockClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("initialize blockClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		pendingTxClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("initialize pendingTxClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		chainID, err = client.NetworkID(context.Background())
		if err != nil {
			log.Error("initialize NetworkID", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		c.client = client
		c.blockClient = blockClient
		c.pendingTxClient = pendingTxClient

		break
	}
}

func (c *CacheManager) start() error {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)

	var runningSummary *BlockchainDetails

	blockNumber, err := c.getLastBlockNumberByDb(LastBlockKey)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Warn("First time start")
			blockNumber = 0
			if c.enableExtendedApis {
				runningSummary = &BlockchainDetails{
					BlockNumber:           0,
					MaxSupply:             c.maxSupply,
					TotalSupply:           c.genesisCirculatingSupply,
					CirculatingSupply:     c.genesisCirculatingSupply,
					BurntCoins:            "0x0",
					BlockRewardsCoins:     "0x0",
					BaseBlockRewardsCoins: "0x0",
					TxnFeeRewardsCoins:    "0x0",
					TxnFeeBurntCoins:      "0x0",
					SlashedCoins:          "0x0",
				}
			}
		} else {
			log.Error("GetLastBlockByDb", "err", err.Error())
			return err
		}
	} else {
		if c.enableExtendedApis {
			runningSummary, err = c.getSummaryFromDb()
			if err != nil {
				log.Error("getSummaryFromDb", "err", err.Error())
				return err
			}
		}
	}

	c.blockChan = make(chan *InternalBlockData, 25)

	go func() {
		c.clientInitialize()
		c.downloadBlocks(int64(blockNumber+1), c.blockChan)
		c.processPendingTransactions()

		for {
			select {
			case internalBlockData := <-c.blockChan:
				for {
					blockNumberVal := internalBlockData.Block.Number().Uint64()
					err := c.processByCacheManager(internalBlockData, runningSummary)
					if err == nil {
						log.Info("Batch Complete", "Block number", blockNumberVal)
						break
					} else {
						if err.Error() == NotFoundErrMsg {
							log.Info("Waiting for Block...", "Block number", blockNumberVal)
						} else {
							log.Error("Batch Error", "error", err.Error(), "Block number", blockNumberVal)
						}
						time.Sleep(5 * time.Second)
					}
				}

			case <-cancel:
				log.Info("Quit signal received")
				return
			}
		}
	}()

	return nil
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
				log.Info("processPendingTransactions Quit signal received")
				pendingTxnTimer.Stop()
				c.close()
				os.Exit(1)
				return
			}
		}
	}()
}

func (c *CacheManager) downloadBlocks(startBlockNumber int64, resultChan chan<- *InternalBlockData) {
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
				if blockNumberToGet == startBlockNumber || blockNumberToGet%int64(50) == 0 {
					var latestBlockNumberHex *hexutil.Uint64
					err := c.blockClient.GetRpcClient().CallContext(context.Background(), &latestBlockNumberHex, "eth_blockNumber")
					if err != nil {
						log.Error("downloadBlocks eth_blockNumber", "error", err)
					} else {
						latestBlockNumber, err := hexutil.DecodeBig(latestBlockNumberHex.String())
						if err != nil {
							log.Error("downloadBlocks DecodeBig", "error", err)
						} else {
							if uint64(blockNumberToGet) < latestBlockNumber.Uint64() && latestBlockNumber.Uint64()-uint64(blockNumberToGet) > 50 {
								isLagging = true
								log.Info("downloadBlocks Lagging behind latest blocks", "latestBlockNumber", latestBlockNumber.Uint64(), "blockNumberToGet", blockNumberToGet)
							} else {
								isLagging = false
							}
						}
					}
				}

				log.Info("downloadBlock BlockByNumber", "Block Number ", blockNumberToGet)
				blockNumBig := big.NewInt(blockNumberToGet)
				block, err := c.blockClient.BlockByNumber(context.Background(), blockNumBig)
				if err != nil {
					if err.Error() != NotFoundErrMsg {
						log.Error("downloadBlock BlockByNumber", "error", err, "blockNumberToGet", blockNumberToGet)
					}
					delayNumber = int64(3000 * time.Millisecond)
				} else {
					consensusData, err := c.client.GetBlockConsensusData(context.Background(), blockNumBig)
					if err != nil {
						log.Error("downloadBlock GetBlockConsensusData", "error", err, "blockNumberToGet", blockNumberToGet)
						delayNumber = int64(3000 * time.Millisecond)
					} else {
						zeroAddressBalance, err := c.client.BalanceAt(context.Background(), common.ZERO_ADDRESS, blockNumBig)
						if err != nil {
							log.Error("downloadBlock BalanceAt zeroAddressBalance", "error", err)
							delayNumber = int64(3000 * time.Millisecond)
						} else {
							internalBlockData := &InternalBlockData{
								Block:              block,
								ConsensusData:      consensusData,
								ZeroAddressBalance: zeroAddressBalance,
							}
							log.Info("before resultChan")
							resultChan <- internalBlockData
							log.Info("after resultChan")
							blockNumberToGet = blockNumberToGet + 1
						}
					}
					if isLagging {
						delayNumber = 0
					} else {
						delayNumber = int64(100 * time.Millisecond)
					}
				}
				blockTimer.Reset(time.Duration(delayNumber))
			case <-cancel:
				blockTimer.Stop()
				log.Info("downloadBlocks Quit signal received")
				return
			}
		}
	}()
}

func (c *CacheManager) processByCacheManager(internalBlockData *InternalBlockData, runningSummary *BlockchainDetails) error {
	block := internalBlockData.Block
	blockNum := block.Header().Number
	blockNumber := blockNum.Uint64()
	var err error

	log.Info("processByCacheManager", "blockNumber", blockNumber)

	txnBatch := c.cacheDb.NewBatch()
	blockKey := []byte(LastBlockKey)
	err = txnBatch.Put(blockKey, common.Uint64ToBytes(blockNumber))
	if err != nil {
		log.Error("processByCacheManager txnBatch.Put", "error", err)
		return err
	}

	liveAccountTxnMap := make(map[string][]AccountTransactionCompact) //address to transactions in block mapping

	var receipts types.Receipts
	receipts = make(types.Receipts, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		log.Info("processByCacheManager", "txn", tx.Hash().Hex())
		accountsInvolved := make(map[string]bool)

		receipt, err := c.client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			log.Error("processByCacheManager TransactionReceipt", "error", err, "tx", tx.Hash().Hex())
			return err
		}
		receipts[i] = receipt

		msg, err := tx.AsMessage(types.NewLondonSigner(chainID))
		if err != nil {
			log.Error("processByCacheManager AsMessage", "error", err)
			return err
		}

		fromAddress := strings.ToLower(msg.From().Hex())
		var toAddress string
		if tx.To() != nil {
			toAddress = strings.ToLower(tx.To().Hex())
		}

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
		log.Debug("transaction", "gasUsed", gasUsed, "txnFee", txnFee, "hash", tx.Hash().Hex())
		transaction.TxnFee = common.BigIntToHexString(txnFee)

		if receipt.Status == 1 {
			transaction.Status = "0x1"
		} else {
			transaction.Status = "0x0"
		}

		txType, err := c.getTransactionType(tx, receipt, blockNum, &txnBatch)
		if err != nil {
			log.Error("getTransactionType", "error", err, "tx", tx.Hash())
			return err
		}
		transaction.TransactionType = string(txType)

		if receipt.Status == 1 {
			//Find all internal transactions
			internalTransactions, err := c.client.GetInternalTransactions(context.Background(), tx.Hash())
			if err != nil {
				log.Warn("GetInternalTransactions", "err", err, "txn", tx.Hash())
				if errors.Is(err, ethclient.TracingGasError) {

				} else {
					return err
				}
			} else {
				internalTxnList := c.flattenInternalTransactionDetails(internalTransactions)

				for _, iTxn := range internalTxnList {
					accountsInvolved[strings.ToLower(iTxn.From)] = true
					accountsInvolved[strings.ToLower(iTxn.To)] = true

					if strings.ToUpper(iTxn.Type) == "CREATE" || strings.ToUpper(iTxn.Type) == "CREATE2" {
						tokenDetails, err := c.client.GetTokenDetails(common.HexToAddress(iTxn.To), blockNum)
						if err != nil {
							if errors.Is(err, ethclient.NotATokenError) {
								continue
							} else {
								log.Error("processByCacheManager GetTokenDetails", "error", err)
								return err
							}
						}
						//new token created via internal transaction
						tkn := &TokenDetails{
							ContractAddress:        strings.ToLower(iTxn.To),
							CreatorAddress:         strings.ToLower(iTxn.From),
							CreatedTransactionHash: strings.ToLower(tx.Hash().Hex()),
							CreatedBlockNumber:     receipt.BlockNumber.Uint64(),
							Name:                   tokenDetails.Name,
							Symbol:                 tokenDetails.Symbol,
							TotalSupply:            hexutil.EncodeBig(tokenDetails.TotalSupply),
							Decimals:               hexutil.EncodeUint64(uint64(tokenDetails.Decimals)),
						}

						err = c.putTokenInDb(tkn, &txnBatch)
						if err != nil {
							log.Error("putTokenInDb", "error", err, "contractAddress", iTxn.To)
							return err
						}
					}
				}
			}

			//Find all relevant token and internal token transactions, if any
			tokenTransfers, tokenApprovals, err := token.ParseTokenTransaction(tx, receipt)
			if err != nil {
				log.Error("ParseTokenTransaction", "err", err, "txn", tx.Hash())
				return err
			}

			if tokenTransfers != nil && len(tokenTransfers) > 0 {
				err = c.processAccountTokenTransfers(tokenTransfers, blockNum, &txnBatch)
				if err != nil {
					log.Error("processAccountTokenTransfers", "err", err, "txn", tx.Hash().Hex())
					return err
				}
				for _, transfer := range tokenTransfers {
					accountsInvolved[strings.ToLower(transfer.ContractAddress.Hex())] = true
					accountsInvolved[strings.ToLower(transfer.From.Hex())] = true
					accountsInvolved[strings.ToLower(transfer.To.Hex())] = true
				}
			}

			if tokenApprovals != nil && len(tokenApprovals) > 0 {
				for _, approval := range tokenApprovals {
					accountsInvolved[strings.ToLower(approval.ContractAddress.Hex())] = true
					accountsInvolved[strings.ToLower(approval.TokenOwner.Hex())] = true
					accountsInvolved[strings.ToLower(approval.Spender.Hex())] = true
				}
			}

			if transaction.TransactionType == string(TOKEN_TRANSFER) { //only root level transaction (no internal transactions)
				//First transfer is root level by from address
				if tokenTransfers == nil || len(tokenTransfers) == 0 || strings.ToLower(tokenTransfers[0].From.Hex()) != transaction.From {
					log.Error("processByCacheManager missing", "from", transaction.From, "to (contract)", transaction.To)
					transaction.TransactionType = string(SMART_CONTRACT)
				} else {
					transaction.TokenTransaction.ContractAddress = strings.ToLower(tokenTransfers[0].ContractAddress.Hex())
					transaction.TokenTransaction.TokenFromAddress = strings.ToLower(tokenTransfers[0].From.Hex())
					transaction.TokenTransaction.TokenToAddress = strings.ToLower(tokenTransfers[0].To.Hex())

					tokenDetails, err := c.getTokenDetailsInternal(transaction.TokenTransaction.ContractAddress) //token should already have been saved to db, when it was created
					if err != nil {
						if err.Error() == LevelDbNoTFoundErrMsg {
							log.Warn("getTokenDetailsInternal", "error", err)
							transaction.TransactionType = string(SMART_CONTRACT)
						} else {
							log.Error("getTokenDetailsInternal", "error", err)
							return err
						}
					}
					transaction.TokenTransaction.TokenCount = hexutil.EncodeBig(tokenTransfers[0].Tokens)
					transaction.TokenTransaction.TokenName = tokenDetails.Name
					transaction.TokenTransaction.TokenSymbol = tokenDetails.Symbol
				}
			}
		}

		_, ok := liveAccountTxnMap[fromAddress]
		if ok == false {
			liveAccountTxnMap[fromAddress] = make([]AccountTransactionCompact, 0)
		}
		liveAccountTxnMap[fromAddress] = append(liveAccountTxnMap[fromAddress], transaction)

		if tx.To() != nil {
			if fromAddress != toAddress {
				_, ok = liveAccountTxnMap[toAddress]
				if ok == false {
					liveAccountTxnMap[toAddress] = make([]AccountTransactionCompact, 0)
				}
				liveAccountTxnMap[toAddress] = append(liveAccountTxnMap[toAddress], transaction)
			}
			accountsInvolved[strings.ToLower(tx.To().Hex())] = true
		} else {
			accountsInvolved[strings.ToLower(receipt.ContractAddress.Hex())] = true
		}
		accountsInvolved[fromAddress] = true

		//Loop through all accounts and update account cache
		for account, _ := range accountsInvolved {
			_, err = c.getAccount(common.HexToAddress(account), blockNum, &txnBatch)
			if err != nil {
				log.Error("processByCacheManager getAccount", "error", err)
				return err
			}
		}
	}

	for k, v := range liveAccountTxnMap {
		err = c.processAccountTransactions(k, &v, &txnBatch)
		if err != nil {
			log.Error("processAccountTransaction", "error", err, "address", k)
			return err
		}
	}

	if c.enableExtendedApis {
		err = c.updateSummary(internalBlockData, runningSummary, &txnBatch)
		if err != nil {
			log.Error("updateSummary", "error", err)
			return err
		}
	}

	err = txnBatch.Write()
	if err != nil {
		log.Error("processByCacheManager txnBatch Write", "error", err)
		return err
	}

	return nil
}

func (c *CacheManager) updateSummary(internalBlockData *InternalBlockData, runningSummary *BlockchainDetails, batch *ethdb.Batch) error {

	blockNumber := internalBlockData.Block.Number()
	leftBlock := blockNumber.Uint64()
	rightBlock := runningSummary.BlockNumber + 1
	if leftBlock != rightBlock {
		log.Error("updateSummary", "leftBlock", leftBlock, "rightBlock", rightBlock)
		return errors.New("updateSummary unexpected blockNumber")
	}

	consensusData := internalBlockData.ConsensusData

	txnBatch := *batch
	blockRewardsInfo := consensusData.BlockRewardsInfo

	var baseBlockProposerRewards *big.Int
	var blockProposerRewards *big.Int
	var txnFeeRewards *big.Int
	var burntTxnFee *big.Int
	var slashAmount *big.Int
	var err error

	//Update running summary
	runningSummary.BlockNumber = blockNumber.Uint64()

	if len(blockRewardsInfo.BaseBlockProposerRewards) > 0 {
		baseBlockProposerRewards, err = hexutil.DecodeBig(blockRewardsInfo.BaseBlockProposerRewards)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		baseBlockRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.BaseBlockRewardsCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary baseBlockRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.BaseBlockRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(baseBlockRewardsCoinsBig, baseBlockProposerRewards))
	}

	if len(blockRewardsInfo.BlockProposerRewards) > 0 {
		blockProposerRewards, err = hexutil.DecodeBig(blockRewardsInfo.BlockProposerRewards)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		blockRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.BlockRewardsCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary blockRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.BlockRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(blockRewardsCoinsBig, blockProposerRewards))
	}

	if len(blockRewardsInfo.TxnFeeRewards) > 0 {
		txnFeeRewards, err = hexutil.DecodeBig(blockRewardsInfo.TxnFeeRewards)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		txnFeeRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.TxnFeeRewardsCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary txnFeeRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.TxnFeeRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(txnFeeRewardsCoinsBig, txnFeeRewards))
	}

	if len(blockRewardsInfo.BurntTxnFee) > 0 {
		burntTxnFee, err = hexutil.DecodeBig(blockRewardsInfo.BurntTxnFee)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		txnFeeBurntCoinsBig, err := hexutil.DecodeBig(runningSummary.TxnFeeBurntCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary txnFeeBurntCoinsBig", "error", err)
			return err
		}
		runningSummary.TxnFeeBurntCoins = hexutil.EncodeBig(common.SafeAddBigInt(txnFeeBurntCoinsBig, burntTxnFee))
	}

	if len(blockRewardsInfo.SlashAmount) > 0 {
		slashAmount, err = hexutil.DecodeBig(blockRewardsInfo.SlashAmount)
		if err != nil {
			log.Error("updateSummary DecodeBig", "error", err)
			return err
		}
		slashedCoinsBig, err := hexutil.DecodeBig(runningSummary.SlashedCoins)
		if err != nil {
			log.Error("updateSummary DecodeBig runningSummary slashedCoinsBig", "error", err)
			return err
		}
		runningSummary.SlashedCoins = hexutil.EncodeBig(common.SafeAddBigInt(slashedCoinsBig, slashAmount))
	}

	//Get latest burnt coins info
	burntCoinsWei := internalBlockData.ZeroAddressBalance

	runningSummary.BurntCoins = hexutil.EncodeBig(burntCoinsWei)
	genesisCirculatingSupplyBig, _ := hexutil.DecodeBig(c.genesisCirculatingSupply)
	blockRewardsCoinsBig, _ := hexutil.DecodeBig(runningSummary.BlockRewardsCoins)
	coinsNew := common.SafeAddBigInt(genesisCirculatingSupplyBig, blockRewardsCoinsBig)
	runningSummary.CirculatingSupply = hexutil.EncodeBig(common.SafeSubBigInt(coinsNew, burntCoinsWei))
	runningSummary.TotalSupply = runningSummary.CirculatingSupply

	err = c.putSummary(runningSummary, &txnBatch)
	if err != nil {
		log.Error("updateSummary putSummary", "error", err)
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

func (c *CacheManager) getSummaryFromDb() (*BlockchainDetails, error) {
	db := c.cacheDb
	summaryBlob, err := db.Get([]byte(SummaryKey))
	if err != nil {
		return nil, err
	}

	var summary BlockchainDetails
	err = json.Unmarshal(summaryBlob, &summary)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

func (c *CacheManager) putSummary(summary *BlockchainDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	keyBlob := []byte(SummaryKey)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) close() error {
	close(c.blockChan)
	c.pendingTxLock.Lock()
	defer c.pendingTxLock.Unlock()
	c.pendingTxClient.Close()

	cacheDb := c.cacheDb
	err := cacheDb.Close()
	if err != nil {
		log.Debug("cache manager account transaction db close error", "err", err)
		return err
	}

	c.client.Close()
	c.blockClient.Close()
	return nil
}

func (c *CacheManager) processAccountTransactions(address string, txnList *[]AccountTransactionCompact, batch *ethdb.Batch) error {
	txnBatch := *batch
	var txnCount uint64
	var err error

	address = strings.ToLower(address)

	txnCount, err = c.getAccountTxnCount(address)
	if err != nil {
		return err
	}
	newTxnCount := txnCount + 1
	var accountTransactionList AccountTransactionList

	log.Info("processAccountTransactions", "address", address, "txnCount", txnCount, "transaction count in block", len(*txnList))

	if newTxnCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
		accountTransactionList.Transactions = make([]AccountTransactionCompact, 0)
		accountTransactionList.Address = address
		log.Info("processAccountTransactions", "address", address, "newTxnCount", newTxnCount)
	} else {
		//Load current state form the cache
		txnPageCount := getPageCount(newTxnCount)
		txnPageKey := getAccountTransactionPageKey(address, txnPageCount)

		log.Info("processAccountTransactions loading from cache", "address", address, "newTxnCount", newTxnCount, "txnPageCount", txnPageCount)

		accountTransactionListBlob, err := c.cacheDb.Get(txnPageKey)
		if err != nil {
			log.Error("cacheDb.Get accountTxnPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(accountTransactionListBlob, &accountTransactionList)
		if err != nil {
			log.Error("json.Unmarshal accountTransactionListBlob", "error", err)
			return err
		}

		if strings.ToLower(accountTransactionList.Address) != address {
			return errors.New("unexpected address")
		}

		if accountTransactionList.Transactions == nil {
			return errors.New("unexpected transactions is nul")
		}

		if len(accountTransactionList.Transactions) != int(txnCount%PageSize) {
			log.Error("unexpected transactions count from address", "actual", len(accountTransactionList.Transactions), "expected", int(txnCount%PageSize), "txnCount", txnCount)
			return errors.New("unexpected transactions count")
		}
	}

	for i, txn := range *txnList {
		accountTransactionList.Transactions = append([]AccountTransactionCompact{txn}, accountTransactionList.Transactions...) //prepend for backward compat

		if len(accountTransactionList.Transactions) == int(PageSize) || i == len(*txnList)-1 {
			accountTransactionListBlob, err := json.Marshal(accountTransactionList)
			if err != nil {
				log.Error("json.Marshal accountTransactionListBlob", "error", err)
				return err
			}

			runningTxnCount := txnCount + uint64(i) + 1
			txnPageCount := getPageCount(runningTxnCount)
			txnPageKey := getAccountTransactionPageKey(address, txnPageCount)
			err = txnBatch.Put(txnPageKey, accountTransactionListBlob)
			if err != nil {
				log.Error("txnBatch.Put accountTransactionListBlob", "error", err)
				return err
			}
			log.Info("txnBatch.Put", "runningTxnCount", runningTxnCount, "txnPageCount", txnPageCount)
			accountTransactionList.Transactions = make([]AccountTransactionCompact, 0) //reset
		}
	}

	txnCount = txnCount + uint64(len(*txnList))
	err = c.putAccountTxnCount(address, txnCount, batch)
	if err != nil {
		return err
	}

	log.Info("inserted account txn list", "txnCount", txnCount, "txnPageCount", getPageCount(txnCount), "txnCountInBlock", len(*txnList), "address", address)

	return nil
}

func getPageCount(itemCount uint64) uint64 {
	if itemCount%PageSize == 0 {
		return itemCount / PageSize
	} else {
		return (itemCount / PageSize) + 1
	}
}

func getAccountTxnCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTxnCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTxnCount(address string) (uint64, error) {
	accountTxnCountKey, keyBlob := getAccountTxnCountKey(address)
	accountTxnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTxnCount not found", "address", address, "accountTxnCountKey", accountTxnCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTxnCount cacheDb.Get address", "address", address, "accountTxnCountKey", accountTxnCountKey, "error", err)
			return 0, err
		}
	} else {
		txnCount := common.BytesToUint64(accountTxnCountBlob)
		log.Info("getAccountTxnCount", "address", address, "accountTxnCountKey", accountTxnCountKey, "txnCount", txnCount)
		return txnCount, nil
	}
}

func (c *CacheManager) putAccountTxnCount(address string, txnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTxnCountKey, keyBlob := getAccountTxnCountKey(address)
	log.Info("putAccountTxnCount", "address", address, "accountTxnCountKey", accountTxnCountKey, "txnCount", txnCount)

	blob := common.Uint64ToBytes(txnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTxnCount address", "error", err, "address", address, "txnCount", txnCount)
		return err
	}

	return nil
}

func (c *CacheManager) GetBlockchainDetails() (GetBlockchainDetailsResponse, error) {
	if c.enableExtendedApis == false {
		return GetBlockchainDetailsResponse{}, errors.New("enableExtendedApis is false")
	}
	getResponse := GetBlockchainDetailsResponse{}
	details, err := c.getSummaryFromDb()
	if err != nil {
		return getResponse, err
	}

	getResponse.Result = *details

	return getResponse, nil
}

func (c *CacheManager) ListTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTransactionsResponse, error) {
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

	if pageCount == 0 {
		return ListAccountTransactionsResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListTransactionByAccount", "address", address, "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "accountTxnCount", accountTxnCount)
	if pageNumber > pageCount {
		return ListAccountTransactionsResponse{PageCount: pageCount}, nil
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

	if strings.ToLower(accountTransactionList.Address) != address {
		log.Error("unexpected address accountTransactionList.Address", "address", address, "accountTransactionList.Address", accountTransactionList.Address)
		return ListAccountTransactionsResponse{}, errors.New("unexpected address accountTransactionList.Address")
	}

	listResponse.Items = accountTransactionList.Transactions
	listResponse.PageCount = pageCount

	return listResponse, nil
}

func (c *CacheManager) ListPendingTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountPendingTransactionsResponse, error) {
	c.pendingTxMapLock.RLock()
	defer c.pendingTxMapLock.RUnlock()

	log.Info("ListPendingTransactionsByAccount", "account", accountAddress)

	address := accountAddress.Hex()

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
					From:  strings.ToLower(tx.From.Hex()),
					Value: tx.Value.String(),
				}
				if tx.To != nil {
					txn.To = strings.ToLower(tx.To.Hex())
				}
				txn.Hash = tx.Hash.Hex()
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
					From:  strings.ToLower(tx.From.Hex()),
					Value: tx.Value.String(),
				}
				if tx.To != nil {
					txn.To = strings.ToLower(tx.To.Hex())
				}
				txn.Hash = tx.Hash.Hex()
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

func getAccountTransactionPageKey(address string, pageCount uint64) []byte {
	pageKey := fmt.Sprintf(AccountTransactionPageKey, strings.ToLower(address), pageCount)
	return []byte(pageKey)
}

func (c *CacheManager) getTransactionType(txn *types.Transaction, receipt *types.Receipt, blockNumber *big.Int, batch *ethdb.Batch) (TransactionType, error) {
	txHash := txn.Hash()
	if txHash.IsEqualTo(receipt.TxHash) == false {
		return "", errors.New("hash mismatch between txn and receipt")
	}

	if txn.To() != nil {
		if receipt.Status == 0 {
			return COIN_TRANSFER, nil //todo: fix
		}

		acc, err := c.getAccount(*txn.To(), blockNumber, batch)
		if err != nil {
			return "", err
		}
		if acc.AccType == ethclient.ACCOUNT_TYPE_REGULAR {
			return COIN_TRANSFER, nil
		} else {
			isTokenTransfer, err := token.IsMainTransactionTokenTransfer(txn, receipt)
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
		if receipt.Status == 1 {
			acc, err := c.getAccount(receipt.ContractAddress, blockNumber, batch)
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
		} else {
			return NEW_SMART_CONTRACT, nil
		}
	}
}

func (c *CacheManager) getTokenDetailsInternal(contractAddress string) (*TokenDetails, error) {
	contractAddress = strings.ToLower(contractAddress)
	tokenDetails, ok := c.tokenMap[contractAddress]
	if ok {
		return tokenDetails, nil
	}

	key := fmt.Sprintf(TokenDetailsKey, contractAddress)

	db := c.cacheDb
	tokenBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	tokenDetails = &TokenDetails{}
	err = json.Unmarshal(tokenBlob, tokenDetails)
	if err != nil {
		return nil, err
	}

	c.tokenMap[contractAddress] = tokenDetails

	return tokenDetails, nil
}

func (c *CacheManager) GetTokenDetails(contractAddress string) (*GetTokenDetailsResponse, error) {
	contractAddress = strings.ToLower(contractAddress)

	key := fmt.Sprintf(TokenDetailsKey, contractAddress)

	db := c.cacheDb
	tokenBlob, err := db.Get([]byte(key))
	if err != nil {
		return nil, err
	}

	var tokenDetails TokenDetails
	err = json.Unmarshal(tokenBlob, &tokenDetails)
	if err != nil {
		return nil, err
	}

	return &GetTokenDetailsResponse{
		Result: tokenDetails,
	}, nil
}

func (c *CacheManager) putTokenInDb(tokenDetails *TokenDetails, batch *ethdb.Batch) error {
	txnBatch := *batch

	blob, err := json.Marshal(tokenDetails)
	if err != nil {
		return err
	}

	contractAddress := strings.ToLower(tokenDetails.ContractAddress)
	key := fmt.Sprintf(TokenDetailsKey, contractAddress)
	keyBlob := []byte(key)

	err = txnBatch.Put(keyBlob, blob)
	if err != nil {
		return err
	}

	c.tokenMap[contractAddress] = tokenDetails

	log.Info("putTokenInDb", "contractAddress", contractAddress)

	return nil
}

func (c *CacheManager) getAccount(address common.Address, blockNumber *big.Int, batch *ethdb.Batch) (*AccountDetails, error) {
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
	result, err := c.client.GetAccountTypeV1(address, blockNumber)
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
}

func (c *CacheManager) flattenInternalTransactionDetails(details *ethclient.InternalTransactionDetails) []*InternalTransactionDetail {
	txnStack := newStack()
	txnList := make([]*InternalTransactionDetail, 0)

	detailsWithLevel := &InternalTransactionDetailWithLevel{
		txn:   details,
		level: byte(0),
	}
	txnStack.Push(detailsWithLevel)

	for txnStack.IsEmpty() == false {
		txnWithLevel := txnStack.Pop()
		txn := txnWithLevel.txn
		txnDetail := InternalTransactionDetail{
			From:  txn.From,
			To:    txn.To,
			Value: txn.Value,
			Type:  txn.Type,
			Level: txnWithLevel.level,
		}
		txnList = append(txnList, &txnDetail)
		if txn.Calls != nil {
			for _, t := range txn.Calls {
				detailsWithLevel = &InternalTransactionDetailWithLevel{
					txn:   &t,
					level: txnWithLevel.level + 1,
				}
				txnStack.Push(detailsWithLevel)
			}
		}
	}
	return txnList
}

type Stack struct { //not thread safe
	internalTxnDetails []*InternalTransactionDetailWithLevel
	count              int
}

func newStack() *Stack {
	s := Stack{
		internalTxnDetails: make([]*InternalTransactionDetailWithLevel, 0),
	}
	return &s
}

func (s *Stack) Size() int {
	return len(s.internalTxnDetails)
}

func (s *Stack) IsEmpty() bool {
	return len(s.internalTxnDetails) == 0
}

func (s *Stack) Push(v *InternalTransactionDetailWithLevel) {
	s.internalTxnDetails = append(s.internalTxnDetails, v)
	s.count = len(s.internalTxnDetails)
}

func (s *Stack) Pop() *InternalTransactionDetailWithLevel {
	count := len(s.internalTxnDetails)
	last := s.internalTxnDetails[count-1]
	s.internalTxnDetails = s.internalTxnDetails[:count-1]

	return last
}

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

// Parses and stores list of tokens and balance for each token
func (c *CacheManager) processAccountTokenTransfers(tokenTransfers []*token.LogTransfer, blockNum *big.Int, batch *ethdb.Batch) error {
	for _, t := range tokenTransfers {
		contractAddress := strings.ToLower(t.ContractAddress.Hex())
		tokenDetails, err := c.getTokenDetailsInternal(contractAddress)
		if err != nil {
			if err.Error() == LevelDbNoTFoundErrMsg {
				log.Warn("processAccountTokenTransfers getTokenDetailsInternal not found", "contractAddress", contractAddress)
				return nil
			} else {
				log.Error("processAccountTokenTransfers getTokenDetailsInternal", "contractAddress", contractAddress, "error", err)
				return err
			}
		}

		tokenBalance, err := c.client.GetAccountTokenBalance(t.ContractAddress, t.From)
		if err != nil {
			if errors.Is(err, ethclient.NotATokenError) {
				log.Error("processAccountTokenTransfers GetAccountTokenBalance from", "contractAddress", contractAddress, "error", err, "from", t.From)
				return err
			}
		} else {
			err = c.processAccountTokenTransfer(t.From, tokenDetails, tokenBalance, batch)
			if err != nil {
				log.Error("processAccountTokenTransfers processAccountTokenTransfer from", "contractAddress", contractAddress, "error", err, "from", t.From)
				return err
			}
		}

		tokenBalance, err = c.client.GetAccountTokenBalance(t.ContractAddress, t.To)
		if err != nil {
			if errors.Is(err, ethclient.NotATokenError) {
				log.Error("processAccountTokenTransfers GetAccountTokenBalance from", "contractAddress", contractAddress, "error", err, "to", t.To)
				return err
			}
		} else {
			err = c.processAccountTokenTransfer(t.To, tokenDetails, tokenBalance, batch)
			if err != nil {
				log.Error("processAccountTokenTransfers processAccountTokenTransfer to", "contractAddress", contractAddress, "error", err, "to", t.To)
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

	address := strings.ToLower(addr.Hex())
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
		log.Error("putAccountTokenInDb", "address", address, "contractAddress", contractAddress)
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

	log.Info("processAccountTokenTransfer", "address", address, "tokenCount", tokenCount, "newTokenCount", newTokenCount)

	if newTokenCount%PageSize == 1 { //if it's the first transaction of the page, won't be in the cache
		accountTokenList.Tokens = make([]AccountTokenSummary, 0)
		accountTokenList.Address = address
		log.Info("processAccountTokenTransfer", "address", address, "newTokenCount", newTokenCount)
	} else {
		//Load current state form the cache
		tokenPageCount := getPageCount(newTokenCount)
		pageKey, tokenPageKey := getAccountTokenPageKey(address, tokenPageCount)

		log.Debug("processAccountTokenTransfer loading from cache", "address", address, "newTokenCount", newTokenCount, "tokenPageCount", tokenPageCount, "pageKey", pageKey)

		accountTokenListBlob, err := c.cacheDb.Get(tokenPageKey)
		if err != nil {
			log.Error("cacheDb.Get tokenPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(accountTokenListBlob, &accountTokenList)
		if err != nil {
			log.Error("json.Unmarshal accountTokenListBlob", "error", err, "address", address, "pageKey", pageKey)
			return err
		}

		if strings.ToLower(accountTokenList.Address) != address {
			return errors.New("unexpected address")
		}

		if accountTokenList.Tokens == nil {
			return errors.New("unexpected tokens is nul")
		}

		if len(accountTokenList.Tokens) != int(tokenCount%PageSize) {
			log.Error("unexpected token count from address", "actual", len(accountTokenList.Tokens), "expected", int(tokenCount%PageSize), "tokenCount", tokenCount)
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
		log.Error("json.Marshal accountTokenListBlob", "error", err)
		return err
	}

	tokenPageCount := getPageCount(newTokenCount)
	log.Debug("tokenPageCount", tokenPageCount, "newTokenCount", newTokenCount)
	key, tokenPageKeyBlob := getAccountTokenPageKey(address, tokenPageCount)
	err = txnBatch.Put(tokenPageKeyBlob, accountTokenListBlob)
	if err != nil {
		log.Error("txnBatch.Put accountTokenListBlob", "error", err, "key", key)
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

	log.Info("inserted account token list", "newTokenCount", newTokenCount, "tokenPageCount", tokenPageCount, "address", address)

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
	address := strings.ToLower(accountAddress.Hex())

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

func getAccountTokenTxnCountKey(address string) (key string, blob []byte) {
	key = fmt.Sprintf(AccountTokenTxnCountKey, address)
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getAccountTokenTxnCount(address string) (uint64, error) {
	accountTokenTxnCountKey, keyBlob := getAccountTokenTxnCountKey(address)
	accountTokenTxnCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getAccountTokenTxnCount not found", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey)
			return 0, nil
		} else {
			log.Error("getAccountTokenTxnCount cacheDb.Get address", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "error", err)
			return 0, err
		}
	} else {
		tokenTxnCount := common.BytesToUint64(accountTokenTxnCountBlob)
		log.Info("getAccountTokenTxnCount", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "tokenTxnCount", tokenTxnCount)
		return tokenTxnCount, nil
	}
}

func (c *CacheManager) putAccountTokenTxnCount(address string, tokenTxnCount uint64, batch *ethdb.Batch) error {
	txnBatch := *batch
	address = strings.ToLower(address)
	accountTokenTxnCountKey, keyBlob := getAccountTokenTxnCountKey(address)
	log.Info("putAccountTokenTxnCount", "address", address, "accountTokenTxnCountKey", accountTokenTxnCountKey, "tokenTxnCount", tokenTxnCount)

	blob := common.Uint64ToBytes(tokenTxnCount)
	err := txnBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putAccountTokenTxnCount address", "error", err, "address", address, "tokenTxnCount", tokenTxnCount)
		return err
	}

	return nil
}

func (c *CacheManager) ListTokenTransactionsByAccount(accountAddress common.Address, pageNumberInput int64) (ListAccountTokenTransactionsResponse, error) {
	return ListAccountTokenTransactionsResponse{}, nil
}
