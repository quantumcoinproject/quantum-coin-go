package cachemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const BlockCountKey = "block-count"
const BlockPageKey = "-block-list-%d" //%d is page number

func getBlockPageKey(pageCount uint64) []byte {
	pageKey := fmt.Sprintf(BlockPageKey, pageCount)
	return []byte(pageKey)
}

func getBlockCountKey() (key string, blob []byte) {
	key = BlockCountKey
	blob = []byte(key)
	return key, blob
}

func (c *CacheManager) getBlockCount() (uint64, error) {
	blockCountKey, keyBlob := getBlockCountKey()
	blockCountBlob, err := c.cacheDb.Get(keyBlob)
	if err != nil {
		if err.Error() == LevelDbNoTFoundErrMsg {
			log.Info("getBlockCount not found", "blockCountKey", blockCountKey)
			return 0, nil
		} else {
			log.Error("getBlockCount cacheDb.Get", "blockCountKey", blockCountKey, "error", err)
			return 0, err
		}
	} else {
		blockCount := common.BytesToUint64(blockCountBlob)
		log.Info("getBlockCount", "blockCountKey", blockCountKey, "blockCount", blockCount)
		return blockCount, nil
	}
}

func (c *CacheManager) putBlockCount(blockCount uint64, batch *ethdb.Batch) error {
	blockBatch := *batch
	blockCountKey, keyBlob := getBlockCountKey()
	log.Info("putBlockCount", "blockCountKey", blockCountKey, "blockCount", blockCount)

	blob := common.Uint64ToBytes(blockCount)
	err := blockBatch.Put(keyBlob, blob)
	if err != nil {
		log.Error("putBlockCount", "error", err, "blockCount", blockCount)
		return err
	}

	return nil
}

func (c *CacheManager) processBlock(block *Block, batch *ethdb.Batch) error {
	log.Debug("CacheManager processBlocks")
	blockBatch := *batch
	var blockCount uint64
	var err error

	err = c.putLastBlockNumberInDb(block.Number, batch)
	if err != nil {
		log.Error("CacheManager processBlock putLastBlockNumberInDb", "error", err, "blockNumber", block.Number)
		return err
	}

	//add to block cache
	err = c.putBlockInDb(block, &blockBatch)
	if err != nil {
		log.Error("CacheManager putBlockInDb", "error", err)
		return err
	}

	blockCount, err = c.getBlockCount()
	if err != nil {
		return err
	}
	newBlockCount := blockCount + 1
	var blockList BlockList
	blockCompact := fromBlock(block)
	var blockPageCount uint64
	var blockPageKey []byte
	var blockListBlob []byte

	log.Debug("CacheManager processBlocks", "blockCount", blockCount, "blockNumber", block.Number)

	if newBlockCount%PageSize == 1 { //if it's the first block of the page, won't be in the cache
		blockList.Blocks = make([]BlockCompact, 0)
		log.Debug("CacheManager processBlocks", "newBlockCount", newBlockCount)
	} else {
		//Load current state form the cache
		blockPageCount = getPageCount(newBlockCount)

		log.Debug("CacheManager processBlocks loading from cache", "newBlockCount", newBlockCount, "blockPageCount", blockPageCount)

		blockListBlob, err = c.cacheDb.Get(blockPageKey)
		if err != nil {
			log.Error("CacheManager cacheDb.Get BlockPageKey", "error", err)
			return err
		}
		err = json.Unmarshal(blockListBlob, &blockList)
		if err != nil {
			log.Error("CacheManager json.Unmarshal blockListBlob", "error", err)
			return err
		}

		if blockList.Blocks == nil {
			return errors.New("unexpected blocks is nul")
		}

		if len(blockList.Blocks) != int(blockCount%PageSize) {
			log.Error("CacheManager unexpected blocks count", "actual", len(blockList.Blocks), "expected", int(blockCount%PageSize), "blockCount", blockCount)
			return errors.New("unexpected blocks count")
		}

	}
	blockPageKey = getBlockPageKey(blockPageCount)
	blockList.Blocks = append(blockList.Blocks, *blockCompact)

	blockListBlob, err = json.Marshal(blockList)
	if err != nil {
		log.Error("CacheManager json.Marshal BlockListBlob", "error", err)
		return err
	}
	err = blockBatch.Put(blockPageKey, blockListBlob)
	if err != nil {
		log.Error("CacheManager blockBatch.Put BlockListBlob", "error", err)
		return err
	}

	log.Trace("CacheManager putBlockCount blockCount", "blockCount", blockCount)
	err = c.putBlockCount(blockCount, batch)
	if err != nil {
		return err
	}

	log.Info("CacheManager blockBatch.Put", "newBlockCount", newBlockCount, "blockPageCount", blockPageCount)

	return nil
}

func (c *CacheManager) ListBlocks(pageNumberInput int64) (ListBlocksResponse, error) {
	listResponse := ListBlocksResponse{}

	var pageCount uint64
	BlockCount, err := c.getBlockCount()
	if err != nil {
		return ListBlocksResponse{}, err
	}
	if BlockCount%PageSize == 0 {
		pageCount = BlockCount / PageSize
	} else {
		pageCount = (BlockCount / PageSize) + 1
	}

	if pageCount == 0 {
		return ListBlocksResponse{PageCount: 0}, nil
	}

	var pageNumber uint64
	if pageNumberInput < 1 {
		pageNumber = pageCount
	} else {
		pageNumber = uint64(pageNumberInput)
	}
	log.Info("ListBlockBy", "pageNumberInput", pageNumberInput, "pageNumber", pageNumber, "pageCount", pageCount, "BlockCount", BlockCount)
	if pageNumber > pageCount {
		return ListBlocksResponse{PageCount: pageCount}, nil
	}

	pageKey := BlockPageKey
	blockPageKey := []byte(pageKey)
	log.Info("cache get", "key", pageKey)

	blockListBlob, err := c.cacheDb.Get(blockPageKey)
	if err != nil {
		log.Error("ListBlockBy cacheDb.Get fromBlockPageKey", "error", err)
		return ListBlocksResponse{}, err
	}
	var blockList BlockList
	err = json.Unmarshal(blockListBlob, &blockList)
	if err != nil {
		log.Error("ListBlockBy json.Unmarshal blockListBlob", "error", err)
		return ListBlocksResponse{}, err
	}

	listResponse.Items = blockList.Blocks
	listResponse.PageCount = pageCount

	return listResponse, nil
}
