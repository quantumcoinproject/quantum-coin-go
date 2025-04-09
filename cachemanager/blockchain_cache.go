package cachemanager

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/ethdb"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"math/big"
)

func (c *CacheManager) updateSummary(internalBlockData *PrimordialBlockData, runningSummary *BlockchainDetails, batch *ethdb.Batch) error {

	blockNumber := internalBlockData.Block.Number.Uint64()
	leftBlock := blockNumber
	rightBlock := runningSummary.BlockNumber + 1
	if leftBlock != rightBlock {
		log.Error("CacheManager updateSummary", "leftBlock", leftBlock, "rightBlock", rightBlock)
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
	runningSummary.BlockNumber = blockNumber

	if len(blockRewardsInfo.BaseBlockProposerRewards) > 0 {
		baseBlockProposerRewards, err = hexutil.DecodeBig(blockRewardsInfo.BaseBlockProposerRewards)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig", "error", err)
			return err
		}
		baseBlockRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.BaseBlockRewardsCoins)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig runningSummary baseBlockRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.BaseBlockRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(baseBlockRewardsCoinsBig, baseBlockProposerRewards))
	}

	if len(blockRewardsInfo.BlockProposerRewards) > 0 {
		blockProposerRewards, err = hexutil.DecodeBig(blockRewardsInfo.BlockProposerRewards)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig", "error", err)
			return err
		}
		blockRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.BlockRewardsCoins)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig runningSummary blockRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.BlockRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(blockRewardsCoinsBig, blockProposerRewards))
	}

	if len(blockRewardsInfo.TxnFeeRewards) > 0 {
		txnFeeRewards, err = hexutil.DecodeBig(blockRewardsInfo.TxnFeeRewards)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig", "error", err)
			return err
		}
		txnFeeRewardsCoinsBig, err := hexutil.DecodeBig(runningSummary.TxnFeeRewardsCoins)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig runningSummary txnFeeRewardsCoinsBig", "error", err)
			return err
		}
		runningSummary.TxnFeeRewardsCoins = hexutil.EncodeBig(common.SafeAddBigInt(txnFeeRewardsCoinsBig, txnFeeRewards))
	}

	if len(blockRewardsInfo.BurntTxnFee) > 0 {
		burntTxnFee, err = hexutil.DecodeBig(blockRewardsInfo.BurntTxnFee)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig", "error", err)
			return err
		}
		txnFeeBurntCoinsBig, err := hexutil.DecodeBig(runningSummary.TxnFeeBurntCoins)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig runningSummary txnFeeBurntCoinsBig", "error", err)
			return err
		}
		runningSummary.TxnFeeBurntCoins = hexutil.EncodeBig(common.SafeAddBigInt(txnFeeBurntCoinsBig, burntTxnFee))
	}

	if len(blockRewardsInfo.SlashAmount) > 0 {
		slashAmount, err = hexutil.DecodeBig(blockRewardsInfo.SlashAmount)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig", "error", err)
			return err
		}
		slashedCoinsBig, err := hexutil.DecodeBig(runningSummary.SlashedCoins)
		if err != nil {
			log.Error("CacheManager updateSummary DecodeBig runningSummary slashedCoinsBig", "error", err)
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
		log.Error("CacheManager updateSummary putSummary", "error", err)
		return err
	}

	return nil
}
