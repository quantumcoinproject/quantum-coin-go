package cachemanager

import (
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/ethdb"
	"github.com/QuantumCoinProject/qc/log"
	"path/filepath"
	"sync"
	"time"
)

type CacheManager struct {
	cacheDir  string
	nodeUrl   string
	cacheLock sync.Mutex
	cacheDb   *ethdb.Database
	quitChan  <-chan any
}

func NewCacheManager(cacheDir string, nodeUrl string, quitChan <-chan any) (*CacheManager, error) {
	cManager := &CacheManager{
		nodeUrl:  nodeUrl,
		cacheDir: cacheDir,
		quitChan: quitChan,
	}

	err := cManager.initialize()
	if err != nil {
		return nil, err
	}

	return cManager, nil
}

func (c *CacheManager) initialize() error {
	log.Debug("Initialize cache", "cacheDir", c.cacheDir, "nodeUrl", c.nodeUrl)

	cachedbFilePath := filepath.Join(c.cacheDir, "cache.db")
	var cachedb ethdb.Database
	var err error

	cachedb, err = rawdb.NewLevelDBDatabase(cachedbFilePath, 32, 0, "", false)
	if err != nil {
		return err
	}

	c.cacheDb = &cachedb

	c.start()

	return nil
}

func (c *CacheManager) start() {
	c.cacheLock.Lock()
	defer c.cacheLock.Lock()
	log.Debug("Start cache", "cacheDir", c.cacheDir, "nodeUrl", c.nodeUrl)

	cacheTimer := time.NewTimer(5 * time.Second)
	go func() {
		<-cacheTimer.C
		log.Debug("cacheTimer fired")

		<-c.quitChan
		log.Info("Quit signal received")
		cacheTimer.Stop()
		return
	}()

}
