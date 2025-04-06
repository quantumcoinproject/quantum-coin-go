package cachemanager

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/rawdb"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"io/ioutil"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
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
	tokenMap                 map[string]*TokenDetails
	accountTokenMap          map[string]*map[string]bool //map[accountAddress]map[contractAddress]bool
	blockChan                chan *PrimordialBlockData
	primodialCacheCancelChan chan bool
	primordialCache          *PrimordialCache
}

var chainID *big.Int
var LevelDbNoTFoundErrMsg = "leveldb: not found"
var NotFoundErrMsg = "not found"

const TimeLayout = "2006-01-02T15:04:05Z"

const PageSize uint64 = 20

func NewCacheManager(cacheDir string, nodeUrl string, enableExtendedApis bool, genesisFilePath string, maxSupply string) (*CacheManager, error) {
	cManager := &CacheManager{
		nodeUrl:            nodeUrl,
		cacheDir:           cacheDir,
		enableExtendedApis: enableExtendedApis,
		//addressMap:         make(map[string]*AccountDetails),
		tokenMap:        make(map[string]*TokenDetails),
		accountTokenMap: make(map[string]*map[string]bool),
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
			log.Error("CacheManager ReadFile", "error", err)
			return nil, err
		}

		genesis := core.Genesis{}
		err = json.Unmarshal(genesisBytes, &genesis)
		if err != nil {
			log.Error("CacheManager Unmarshal", "error", err)
			return nil, err
		}

		genesisCirculatingSupply := big.NewInt(0)
		if genesis.Alloc != nil {
			for _, v := range genesis.Alloc {
				genesisCirculatingSupply = common.SafeAddBigInt(genesisCirculatingSupply, v.Balance)
			}
		}
		cManager.genesisCirculatingSupply = hexutil.EncodeBig(genesisCirculatingSupply)
		log.Error("CacheManager genesis genesisCirculatingSupply", "genesisCirculatingSupply", params.WeiToEther(genesisCirculatingSupply), "maxSupply", params.WeiToEther(maxSupplyBig))
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
	log.Info("CacheManager Quantum Coin initialize cache manager", "cacheDir", c.cacheDir, "nodeUrl", c.nodeUrl)

	catchManagerFilePath := filepath.Join(c.cacheDir, "cacheManager.db")
	cacheManagerDb, err := rawdb.NewLevelDBDatabase(catchManagerFilePath, 64, 0, "", false)
	if err != nil {
		return err
	}
	c.cacheDb = cacheManagerDb

	c.primodialCacheCancelChan = make(chan bool)
	c.primordialCache, err = NewPrimordialCache(c.cacheDir, c.nodeUrl, &c.primodialCacheCancelChan)
	if err != nil {
		return err
	}

	return nil
}

func (c *CacheManager) clientInitialize() {
	for {
		client, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("CacheManager initialize client Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		blockClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("CacheManager initialize blockClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		pendingTxClient, err := ethclient.Dial(c.nodeUrl)
		if err != nil {
			log.Error("CacheManager initialize pendingTxClient Dial", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		chainID, err = client.NetworkID(context.Background())
		if err != nil {
			log.Error("CacheManager initialize NetworkID", "error", err)
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

	blockNumber, err := c.getLastBlockNumberFromDb()
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Warn("CacheManager First time start")
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
			log.Error("CacheManager GetLastBlockByDb", "err", err.Error())
			return err
		}
	} else {
		if c.enableExtendedApis {
			runningSummary, err = c.getSummaryFromDb()
			if err != nil {
				log.Error("CacheManager getSummaryFromDb", "err", err.Error())
				return err
			}
		}
	}
	log.Info("CacheManager runningSummary", runningSummary)

	c.blockChan = make(chan *PrimordialBlockData, 25)
	blockNumber = blockNumber + 1

	go func() {
		c.clientInitialize()
		c.processPendingTransactions()
		c.closeLoop()
		delayNumber := int64(100 * time.Millisecond)
		blockTimer := time.NewTimer(time.Duration(delayNumber))

		log.Info("CacheManager start block loop")
		for {
			select {
			case <-blockTimer.C:
				log.Debug("CacheManager blockTimer")
				internalBlockData, err := c.primordialCache.GetBlock(blockNumber)
				if err != nil {
					if err.Error() == LevelDbNoTFoundErrMsg {
						log.Info("CacheManager Waiting for PrimordialBlock...", "PrimordialBlock number", blockNumber)
					} else {
						log.Error("CacheManager GetBlock Error", "error", err.Error(), "PrimordialBlock number", blockNumber)
					}
					delayNumber = int64(3 * time.Second)
				} else {
					err = c.processByCacheManager(internalBlockData, runningSummary)
					if err == nil {
						blockNumber = blockNumber + 1
						log.Info("CacheManager Batch Complete", "PrimordialBlock number", blockNumber)
						delayNumber = 0
					} else {
						log.Error("CacheManager Batch Error", "error", err.Error(), "PrimordialBlock number", blockNumber)
						delayNumber = int64(3 * time.Second)
					}
				}
				blockTimer.Reset(time.Duration(delayNumber))

			case <-cancel:
				log.Warn("CacheManager start() Quit signal received")
				blockTimer.Stop()
				return
			}
		}
	}()

	log.Info("CacheManager start done")

	return nil
}

func (c *CacheManager) closeLoop() {
	cancel := make(chan os.Signal)
	signal.Notify(cancel, os.Interrupt, syscall.SIGTERM)
	delayNumber := int64(100 * time.Millisecond)
	closeTimer := time.NewTimer(time.Duration(delayNumber))

	go func() {
		for {
			select {
			case <-closeTimer.C:
				closeTimer.Reset(time.Duration(delayNumber))

			case <-cancel:
				log.Warn("CacheManager closeLoop quit signal received")
				closeTimer.Stop()
				c.close()
				return
			}
		}
	}()
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

	log.Debug("CacheManager latestBlockByNode", "number", latestBlock)

	return latestBlock, nil

}

func (c *CacheManager) close() error {
	close(c.blockChan)
	c.pendingTxLock.Lock()
	defer c.pendingTxLock.Unlock()
	c.pendingTxClient.Close()

	cacheDb := c.cacheDb
	err := cacheDb.Close()
	if err != nil {
		log.Debug("CacheManager account transaction db close error", "err", err)
		return err
	}
	c.client.Close()
	c.blockClient.Close()
	c.primordialCache.Close()
	log.Info("CacheManager CacheManager closed")
	os.Exit(1)
	return nil
}
