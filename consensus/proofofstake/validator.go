package proofofstake

import (
	"context"
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core/state"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/internal/ethapi"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
)

type ValidatorDetails struct {
	Depositor                     common.Address `json:"depositor"     gencodec:"required"`
	Validator                     common.Address `json:"validator"     gencodec:"required"`
	Balance                       string         `json:"balance"       gencodec:"required"`
	NetBalance                    string         `json:"netBalance"    gencodec:"required"`
	BlockRewards                  string         `json:"blockRewards"  gencodec:"required"`
	Slashings                     string         `json:"slashings"  gencodec:"required"`
	IsValidationPaused            bool           `json:"isValidationPaused"  gencodec:"required"`
	WithdrawalBlock               string         `json:"withdrawalBlock"  gencodec:"required"`
	WithdrawalAmount              string         `json:"withdrawalAmount"  gencodec:"required"`
	LastNiLBlock                  string         `json:"lastNiLBlock" gencodec:"required"`
	NilBlockCount                 string         `json:"nilBlockCount" gencodec:"required"`
	BlockProposerResetBlock       string         `json:"blockProposerResetBlock" gencodec:"required"`
	ValidatorResetBlock           string         `json:"validatorResetBlock" gencodec:"required"`
	ValidatorNetBalanceAfterDecay string         `json:"validatorNetBalanceAfterDecay" gencodec:"required"`
	IsActive                      bool           `json:"isActive" gencodec:"required"`
}

type ValidatorDetailsV2 struct {
	Depositor          common.Address `json:"depositor"     gencodec:"required"`
	Validator          common.Address `json:"validator"     gencodec:"required"`
	Balance            *big.Int       `json:"balance"       gencodec:"required"`
	NetBalance         *big.Int       `json:"netBalance"    gencodec:"required"`
	BlockRewards       *big.Int       `json:"blockRewards"  gencodec:"required"`
	Slashings          *big.Int       `json:"slashings"     gencodec:"required"`
	IsValidationPaused bool           `json:"isValidationPaused"  gencodec:"required"`
	WithdrawalBlock    *big.Int       `json:"withdrawalBlock"  gencodec:"required"`
	WithdrawalAmount   *big.Int       `json:"withdrawalAmount" gencodec:"required"`
	LastNiLBlock       *big.Int       `json:"lastNiLBlock" gencodec:"required"`
	NilBlockCount      *big.Int       `json:"nilBlockCount" gencodec:"required"`
}

// validatorStakingDetailsBatchSize is the number of raw validator-list entries fetched per
// listValidatorStakingDetails call. A window of this size keeps each system eth_call's gas well
// under the 50M RPCGasCap while still amortizing the per-call EVM setup over many validators.
const validatorStakingDetailsBatchSize = 250

// listValidatorStakingDetailsV3 fetches the staking details for all live validators using the V3
// contract's paginated listValidatorStakingDetails(start, count) method, avoiding the per-validator
// eth_calls of the legacy path. It first reads getValidatorListLength() to learn how many raw list
// entries exist, then pages over that index space in batches of validatorStakingDetailsBatchSize and
// concatenates the results. Stale (rotated-away) entries are skipped inside the contract, so a page
// can return fewer than the requested count.
func (p *ProofOfStake) listValidatorStakingDetailsV3(blockHash common.Hash) ([]*ValidatorDetailsV2, error) {
	startTime := time.Now()
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	abiData, err := staking.GetStakingContractV3_ABI()
	if err != nil {
		log.Error("listValidatorStakingDetailsV3 abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	// Read the raw validator-list length so we know how many pages to fetch.
	lengthMethod := staking.GetContract_Method_GetValidatorListLength()
	lengthData, err := abiData.Pack(lengthMethod)
	if err != nil {
		log.Error("Unable to pack getValidatorListLength", "error", err)
		return nil, err
	}
	lengthMsgData := (hexutil.Bytes)(lengthData)
	lengthCallStart := time.Now()
	lengthResult, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &lengthMsgData,
	}, blockNr, nil)
	if err != nil {
		return nil, err
	}
	log.Debug("listValidatorStakingDetailsV3 getValidatorListLength call done", "elapsed", time.Since(lengthCallStart))
	if len(lengthResult) == 0 {
		return nil, errors.New("getValidatorListLength result is 0")
	}
	var total *big.Int
	if err := abiData.UnpackIntoInterface(&total, lengthMethod, lengthResult); err != nil {
		log.Error("getValidatorListLength UnpackIntoInterface", "err", err)
		return nil, err
	}

	listMethod := staking.GetContract_Method_ListValidatorStakingDetails()
	totalCount := total.Uint64()
	batchSize := uint64(validatorStakingDetailsBatchSize)
	validatorList := make([]*ValidatorDetailsV2, 0, totalCount)
	pageCount := 0

	for start := uint64(0); start < totalCount; start += batchSize {
		data, err := abiData.Pack(listMethod, new(big.Int).SetUint64(start), new(big.Int).SetUint64(batchSize))
		if err != nil {
			log.Error("Unable to pack listValidatorStakingDetails", "error", err)
			return nil, err
		}
		msgData := (hexutil.Bytes)(data)
		pageStart := time.Now()
		result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
			To:   &contractAddress,
			Data: &msgData,
		}, blockNr, nil)
		if err != nil {
			return nil, err
		}
		pageCount++
		if len(result) == 0 {
			log.Debug("listValidatorStakingDetailsV3 page empty", "start", start, "count", batchSize, "elapsed", time.Since(pageStart))
			continue
		}

		var page []ValidatorDetailsV2
		if err := abiData.UnpackIntoInterface(&page, listMethod, result); err != nil {
			log.Error("listValidatorStakingDetails UnpackIntoInterface", "err", err)
			return nil, err
		}
		for i := range page {
			validatorList = append(validatorList, &page[i])
		}
		log.Debug("listValidatorStakingDetailsV3 page done", "start", start, "count", batchSize, "rows", len(page), "elapsed", time.Since(pageStart))
	}

	log.Debug("listValidatorStakingDetailsV3 done", "listLength", totalCount, "pages", pageCount, "validators", len(validatorList), "elapsed", time.Since(startTime))
	return validatorList, nil
}

func (p *ProofOfStake) GetValidators(blockHash common.Hash) (map[common.Address]*big.Int, error) {
	header := p.blockchain.GetHeaderByHash(blockHash)
	blockNumber := header.Number.Uint64()

	if blockNumber >= defaults.DefaultConfig.PosConfig.SystemContractV3StartBlock {
		startTime := time.Now()
		detailsList, err := p.listValidatorStakingDetailsV3(blockHash)
		if err != nil {
			log.Debug("GetValidators listValidatorStakingDetailsV3 failed", "err", err)
			return nil, err
		}

		proposalsTxnsMap := make(map[common.Address]*big.Int)
		for _, details := range detailsList {
			if details.Validator.IsEqualTo(ZERO_ADDRESS) {
				return nil, errors.New("invalid validator")
			}
			if details.IsValidationPaused {
				log.Debug("Validator is paused, skipping", "val", details.Validator)
				continue
			}
			if details.Depositor.IsEqualTo(ZERO_ADDRESS) {
				log.Debug("GetValidators invalid depositor", "validator", details.Validator)
				continue
			}
			proposalsTxnsMap[details.Validator] = details.NetBalance
			log.Debug("GetValidators", "validator", details.Validator, "depositor", details.Depositor, "depositAmount", details.NetBalance)
		}
		log.Debug("GetValidators V3 done", "blockNumber", blockNumber, "validators", len(proposalsTxnsMap), "elapsed", time.Since(startTime))
		return proposalsTxnsMap, nil
	}

	depositorCount, err := p.GetDepositorCount(blockHash)
	if err != nil {
		return nil, err
	} else {
		log.Debug("depositorCount", "depositorCount", depositorCount)
	}
	totalDepositedBalance, err := p.GetTotalDepositedBalance(blockHash, blockNumber)
	if err != nil {
		log.Debug("totalDepositedBalance error", "err", err)
		return nil, err
	} else {
		log.Debug("totalDepositedBalance", "totalDepositedBalance", totalDepositedBalance)
	}

	err = staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_ListValidators()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetValidators error getting abidata", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())
	// call
	data, err := abiData.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for get filteredValidatorsDepositMap", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)

	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		log.Debug("result 0 length")
		return nil, nil
	}
	_, err = abiData.Unpack(method, result)
	if err != nil {
		log.Error("Unpack", "err", err)
		return nil, err
	}

	var (
		ret0 = new([]common.Address)
	)
	out := ret0

	if err := abiData.UnpackIntoInterface(out, method, result); err != nil {
		log.Info("UnpackIntoInterface error")
		return nil, err
	}

	proposalsTxnsMap := make(map[common.Address]*big.Int)
	for _, val := range *out {
		if val.IsEqualTo(ZERO_ADDRESS) {
			return nil, errors.New("invalid validator")
		}
		log.Debug("GetValidators Validator", "val", val)
	}

	for _, val := range *out {
		isPaused, err := p.IsValidatorPaused(val, blockHash)
		if err != nil {
			log.Debug("IsValidatorPaused failed", "err", err)
			return nil, err
		}

		if isPaused {
			log.Debug("Validator is paused, skipping", "val", isPaused)
			continue
		}

		depositor, err := p.GetDepositorOfValidator(val, blockHash)
		if err != nil {
			log.Debug("GetDepositorOfValidator failed", "err", err)
			continue
		}

		if depositor.IsEqualTo(ZERO_ADDRESS) {
			log.Debug("GetValidators invalid depositor", "validator", val)
			continue
		}

		balance, err := p.GetNetBalanceOfDepositor(depositor, blockHash)
		if err != nil {
			log.Debug("GetBalanceOfDepositor failed", "err", err)
			continue
		}

		proposalsTxnsMap[val] = balance
		log.Debug("GetValidators", "validator", val, "depositor", depositor, "depositAmount", balance)
	}

	return proposalsTxnsMap, nil
}

func (p *ProofOfStake) GetValidatorOfDepositor(depositor common.Address, blockHash common.Hash) (common.Address, error) {
	log.Trace("GetValidatorOfDepositor depositor", "depositor", depositor)
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return common.Address{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetValidatorOfDepositor()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetValidatorOfDepositor abi error", "err", err)
		return common.Address{}, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, depositor)
	if err != nil {
		log.Error("Unable to pack tx for get validator", "error", err)
		return common.Address{}, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		//Gas:  &gas,
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return common.Address{}, err
	}
	if len(result) == 0 {
		return common.Address{}, errors.New("no depositor found")
	}

	var (
		ret0 = new(common.Address)
	)
	out := ret0

	if err := abiData.UnpackIntoInterface(out, method, result); err != nil {
		return common.Address{}, err
	}

	return *out, nil
}

func (p *ProofOfStake) GetDepositorOfValidator(validator common.Address, blockHash common.Hash) (common.Address, error) {
	log.Trace("GetDepositorOfValidator validator", "validator", validator)
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return common.Address{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetDepositorOfValidator()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetDepositorOfValidator abi error", "err", err)
		return common.Address{}, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, validator)
	if err != nil {
		log.Error("Unable to pack tx for get depositor", "error", err)
		return common.Address{}, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		//Gas:  &gas,
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return common.Address{}, err
	}
	if len(result) == 0 {
		return common.Address{}, errors.New("no depositor found")
	}

	var (
		ret0 = new(common.Address)
	)
	out := ret0

	if err := abiData.UnpackIntoInterface(out, method, result); err != nil {
		return common.Address{}, err
	}

	return *out, nil
}

func (p *ProofOfStake) GetNetBalanceOfDepositor(depositor common.Address, blockHash common.Hash) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetNetBalanceOfDepositor() //todo: change once initial storage is set
	//method := staking.GetContract_Method_GetBalanceOfDepositor()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetNetBalanceOfDepositor abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, depositor)
	if err != nil {
		log.Error("Unable to pack tx for GetNetBalanceOfDepositor", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetNetBalanceOfDepositor result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) GetDepositorCount(blockHash common.Hash) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetDepositorCount()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Trace("GetDepositorCount abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for get filteredValidatorsDepositMap", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)

	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Trace("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetDepositorCount result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) GetTotalDepositedBalance(blockHash common.Hash, blockNumber uint64) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetTotalDepositedBalance()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Trace("GetTotalDepositedBalance abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())
	// call
	data, err := abiData.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for get filteredValidatorsDepositMap", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Trace("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetTotalDepositedBalance result is 0")
	}
	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err)
		return nil, err
	}
	return out, nil
}

func (p *ProofOfStake) DoesDepositorExist(address common.Address, blockHash common.Hash) (bool, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return false, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_DoesDepositorExist()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("DoesDepositorExist abi error", "err", err)
		return false, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, address)
	if err != nil {
		log.Error("Unable to pack tx for get depositor exist", "error", err)
		return false, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		//Gas:  &gas,
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, errors.New("no depositor exist found")
	}

	var ret0 bool
	out := ret0
	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		return false, err
	}

	return out, nil
}

func (p *ProofOfStake) DidDepositorEverExists(address common.Address, blockHash common.Hash) (bool, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return false, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_DidDepositorEverExist()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("DidDepositorEverExists abi error", "err", err)
		return false, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, address)
	if err != nil {
		log.Error("Unable to pack tx for get depositor ever exists", "error", err)
		return false, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		//Gas:  &gas,
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, errors.New("no depositor ever exists found")
	}

	var ret0 bool
	out := ret0
	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		return false, err
	}

	return out, nil
}

func (p *ProofOfStake) DoesValidatorExist(address common.Address, blockHash common.Hash) (bool, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return false, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_DoesValidatorExist()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("DoesValidatorExist abi error", "err", err)
		return false, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, address)
	if err != nil {
		log.Error("Unable to pack tx for get validator exist", "error", err)
		return false, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		//Gas:  &gas,
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, errors.New("no validator exist found")
	}

	var ret0 bool
	out := ret0

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		return false, err
	}

	return out, nil
}

func (p *ProofOfStake) DidValidatorEverExists(address common.Address, blockHash common.Hash) (bool, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return false, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_DidValidatorEverExist()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("DidValidatorEverExists abi error", "err", err)
		return false, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, address)
	if err != nil {
		log.Error("Unable to pack tx for get validator ever exists", "error", err)
		return false, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		//Gas:  &gas,
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, errors.New("no validator ever exists found")
	}

	var ret0 bool
	out := ret0
	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		return false, err
	}

	return out, nil
}

func encodeCall(abi *abi.ABI, method string, args ...interface{}) ([]byte, error) {
	return abi.Pack(method, args...)
}

func (p *ProofOfStake) AddDepositorSlashing(blockHash common.Hash,
	depositor common.Address, slashedAmount *big.Int,
	state *state.StateDB, header *types.Header) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}

	method := staking.GetContract_Method_AddDepositorSlashing()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("AddDepositorSlashing abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := encodeCall(&abiData, method, depositor, slashedAmount)
	if err != nil {
		log.Error("Unable to pack AddDepositorSlashing", "error", err)
		return nil, err
	}

	msgData := (hexutil.Bytes)(data)
	var from common.Address
	from.CopyFrom(ZERO_ADDRESS)
	args := ethapi.TransactionArgs{
		From: &from,
		To:   &contractAddress,
		Data: &msgData,
	}

	msg, err := args.ToMessage(math.MaxUint64)
	if err != nil {
		return nil, err
	}

	result, err := p.blockchain.ExecuteNoGas(msg, state, header)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("AddDepositorSlashing result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) GetStakingContractAbi() (abi.ABI, error) {
	blockNumber := p.blockchain.CurrentBlock().NumberU64()

	if blockNumber < defaults.DefaultConfig.PosConfig.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		return staking.GetStakingContract_ABI()
	} else if blockNumber < defaults.DefaultConfig.PosConfig.SystemContractV3StartBlock {
		return staking.GetStakingContractV2_ABI()
	} else {
		return staking.GetStakingContractV3_ABI()
	}
}

func (p *ProofOfStake) AddDepositorReward(blockHash common.Hash,
	depositor common.Address, rewardAmount *big.Int,
	state *state.StateDB, header *types.Header) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}

	method := staking.GetContract_Method_AddDepositorReward()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("AddDepositorReward abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := encodeCall(&abiData, method, depositor, rewardAmount)
	if err != nil {
		log.Error("Unable to pack AddDepositorReward", "error", err)
		return nil, err
	}

	msgData := (hexutil.Bytes)(data)
	var from common.Address
	from.CopyFrom(ZERO_ADDRESS)
	args := ethapi.TransactionArgs{
		From: &from,
		To:   &contractAddress,
		Data: &msgData,
	}

	msg, err := args.ToMessage(math.MaxUint64)
	if err != nil {
		return nil, err
	}

	result, err := p.blockchain.ExecuteNoGas(msg, state, header)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("AddDepositorReward result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Trace("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) IsValidatorPaused(validator common.Address, blockHash common.Hash) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_IsValidationPaused()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("IsValidatorPaused abi error", "err", err)
		return false, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, validator)
	if err != nil {
		log.Error("IsValidatorPaused Unable to pack", "error", err)
		return false, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return false, err
	}
	if len(result) == 0 {
		return false, errors.New("IsValidatorPaused result is 0")
	}

	var out bool

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("IsValidatorPaused UnpackIntoInterface", "err", err, "validator", validator)
		return false, err
	}

	return out, nil
}

func (p *ProofOfStake) GetBalanceOfDepositor(depositor common.Address, blockHash common.Hash) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetBalanceOfDepositor()

	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetBalanceOfDepositor abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, depositor)
	if err != nil {
		log.Error("Unable to pack tx for GetBalanceOfDepositor", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetBalanceOfDepositor result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) GetDepositorRewards(depositor common.Address, blockHash common.Hash) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetDepositorRewards()

	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetDepositorRewards abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, depositor)
	if err != nil {
		log.Error("Unable to pack tx for GetDepositorRewards", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetDepositorRewards result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) GetDepositorSlashings(depositor common.Address, blockHash common.Hash) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetDepositorSlashings()

	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetDepositorSlashings abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, depositor)
	if err != nil {
		log.Error("Unable to pack tx for GetDepositorSlashings", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetDepositorSlashings result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) GetWithdrawalBlock(depositor common.Address, blockHash common.Hash) (*big.Int, error) {
	err := staking.IsStakingContract()
	if err != nil {
		log.Warn("DP_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetWithdrawalBlock()

	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetWithdrawalBlock abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := abiData.Pack(method, depositor)
	if err != nil {
		log.Error("Unable to pack tx for GetWithdrawalBlock", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetWithdrawalBlock result is 0")
	}

	var out *big.Int

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("UnpackIntoInterface", "err", err, "depositor", depositor)
		return nil, err
	}

	return out, nil
}

func (p *ProofOfStake) ListValidators(blockHash common.Hash, blockNumber uint64) ([]*ValidatorDetails, error) {
	depositorCount, err := p.GetDepositorCount(blockHash)
	if err != nil {
		return nil, err
	} else {
		log.Debug("depositorCount", "depositorCount", depositorCount)
	}

	err = staking.IsStakingContract()
	if err != nil {
		log.Warn("GETH_STAKING_CONTRACT_ADDRESS: Contract1 address is empty")
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_ListValidators()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetValidators error getting abidata", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())
	// call
	data, err := abiData.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for get filteredValidatorsDepositMap", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		log.Debug("result 0 length")
		return nil, nil
	}
	_, err = abiData.Unpack(method, result)
	if err != nil {
		log.Error("Unpack", "err", err)
		return nil, err
	}

	var (
		ret0 = new([]common.Address)
	)
	out := ret0

	if err := abiData.UnpackIntoInterface(out, method, result); err != nil {
		log.Info("UnpackIntoInterface error")
		return nil, err
	}
	var validatorList []*ValidatorDetails
	for _, val := range *out {
		if val.IsEqualTo(ZERO_ADDRESS) {
			return nil, errors.New("invalid validator")
		}
		log.Debug("GetValidators Validator", "val", val)
	}
	for _, val := range *out {
		var validatorDetails *ValidatorDetails

		if blockNumber < defaults.DefaultConfig.PosConfig.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
			validatorDetails, err = p.GetStakingDetailsByValidatorAddress(val, blockHash)
			if err != nil {
				return nil, err
			}
		} else {
			depositor, err := p.GetDepositorOfValidator(val, blockHash)
			if err != nil {
				log.Debug("GetDepositorOfValidator failed", "err", err)
				continue
			}

			if depositor.IsEqualTo(ZERO_ADDRESS) {
				log.Debug("ListValidators invalid depositor", "validator", val)
				continue
			}

			validatorDetailsV2, err := p.GetStakingDetailsByValidatorAddressV2(val, blockHash)
			if err != nil {
				return nil, err
			}

			canVal, validatorResetBlock := canValidate(validatorDetailsV2, blockNumber)
			canProp, blockProposerResetBlock := canPropose(validatorDetailsV2, blockNumber)
			validatorNetBalanceAfterDecay := getOfflineValidatorDepositAfterPenalty(validatorDetailsV2, blockNumber, validatorDetailsV2.NetBalance)
			isActive := canVal && validatorNetBalanceAfterDecay.Cmp(MIN_VALIDATOR_DEPOSIT) >= 0 && !validatorDetailsV2.IsValidationPaused

			validatorDetails = &ValidatorDetails{
				Depositor:                     validatorDetailsV2.Depositor,
				Validator:                     validatorDetailsV2.Validator,
				Balance:                       hexutil.EncodeBig(validatorDetailsV2.Balance),
				NetBalance:                    hexutil.EncodeBig(validatorDetailsV2.NetBalance),
				BlockRewards:                  hexutil.EncodeBig(validatorDetailsV2.BlockRewards),
				Slashings:                     hexutil.EncodeBig(validatorDetailsV2.Slashings),
				IsValidationPaused:            validatorDetailsV2.IsValidationPaused,
				WithdrawalBlock:               hexutil.EncodeBig(validatorDetailsV2.WithdrawalBlock),
				WithdrawalAmount:              hexutil.EncodeBig(validatorDetailsV2.WithdrawalAmount),
				LastNiLBlock:                  hexutil.EncodeBig(validatorDetailsV2.LastNiLBlock),
				NilBlockCount:                 hexutil.EncodeBig(validatorDetailsV2.NilBlockCount),
				ValidatorNetBalanceAfterDecay: hexutil.EncodeBig(validatorNetBalanceAfterDecay),
				IsActive:                      isActive,
			}
			if validatorDetailsV2.NilBlockCount.Uint64() > 0 {
				if canVal == false {
					validatorDetails.ValidatorResetBlock = hexutil.EncodeUint64(validatorResetBlock)
				}
				if canProp == false {
					validatorDetails.BlockProposerResetBlock = hexutil.EncodeUint64(blockProposerResetBlock)
				}
			}
		}

		validatorList = append(validatorList, validatorDetails)
	}

	return validatorList, nil
}

func (p *ProofOfStake) GetStakingDetailsByValidatorAddress(val common.Address, blockHash common.Hash) (*ValidatorDetails, error) {
	isPaused, err := p.IsValidatorPaused(val, blockHash)
	if err != nil {
		log.Debug("IsValidatorPaused failed", "err", err)
		return nil, err
	}

	depositor, err := p.GetDepositorOfValidator(val, blockHash)
	if err != nil {
		log.Debug("GetDepositorOfValidator failed", "err", err)
		return nil, err
	}

	if depositor.IsEqualTo(ZERO_ADDRESS) {
		return nil, errors.New("GetStakingDetailsByValidatorAddress invalid depositor")
	}

	balance, err := p.GetBalanceOfDepositor(depositor, blockHash)
	if err != nil {
		log.Debug("GetBalanceOfDepositor failed", "err", err)
		return nil, err
	}

	netBalance, err := p.GetNetBalanceOfDepositor(depositor, blockHash)
	if err != nil {
		log.Debug("GetNetBalanceOfDepositor failed", "err", err)
		return nil, err
	}

	depositorRewards, err := p.GetDepositorRewards(depositor, blockHash)
	if err != nil {
		log.Debug("GetDepositorRewards failed", "err", err)
		return nil, err
	}

	depositorSlashings, err := p.GetDepositorSlashings(depositor, blockHash)
	if err != nil {
		log.Debug("GetDepositorSlashings failed", "err", err)
		return nil, err
	}

	withdrawalBlock, err := p.GetWithdrawalBlock(depositor, blockHash)
	if err != nil {
		log.Debug("GetWithdrawalBlock failed", "err", err)
		return nil, err
	}

	validatorDetails := &ValidatorDetails{
		Validator:          val,
		Depositor:          depositor,
		Balance:            hexutil.EncodeBig(balance),
		NetBalance:         hexutil.EncodeBig(netBalance),
		BlockRewards:       hexutil.EncodeBig(depositorRewards),
		Slashings:          hexutil.EncodeBig(depositorSlashings),
		IsValidationPaused: isPaused,
		WithdrawalBlock:    hexutil.EncodeBig(withdrawalBlock),
	}

	if withdrawalBlock.Cmp(big.NewInt(0)) > 0 {
		validatorDetails.WithdrawalAmount = validatorDetails.NetBalance //legacy
	}

	return validatorDetails, nil
}

func (p *ProofOfStake) GetStakingDetailsByValidatorAddressV2(val common.Address, blockHash common.Hash) (*ValidatorDetailsV2, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_GetStakingDetails() //todo: change once initial storage is set
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("GetStakingDetails abi error", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())
	// call
	data, err := abiData.Pack(method, val)
	if err != nil {
		log.Error("Unable to pack tx for GetStakingDetails", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)
	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		log.Error("Call", "err", err)
		return nil, err
	}
	if len(result) == 0 {
		return nil, errors.New("GetStakingDetails result is 0")
	}
	var out *ValidatorDetailsV2
	out = new(ValidatorDetailsV2)

	if err := abiData.UnpackIntoInterface(&out, method, result); err != nil {
		log.Debug("UnpackIntoInterface", "err", err, "validator", val)
		return nil, err
	}
	return out, nil
}

func (p *ProofOfStake) SetNilBlock(
	validator common.Address, state *state.StateDB, header *types.Header) error {
	method := staking.GetContract_Method_SetNilBlock()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("SetNilBlock abi error", "err", err, "blockNumber", header.Number.Uint64())
		return err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := encodeCall(&abiData, method, validator)
	if err != nil {
		log.Error("Unable to pack SetNilBlock", "error", err)
		return err
	}

	msgData := (hexutil.Bytes)(data)
	var from common.Address
	from.CopyFrom(ZERO_ADDRESS)
	args := ethapi.TransactionArgs{
		From: &from,
		To:   &contractAddress,
		Data: &msgData,
	}

	msg, err := args.ToMessage(math.MaxUint64)
	if err != nil {
		return err
	}

	_, err = p.blockchain.ExecuteNoGas(msg, state, header)
	if err != nil {
		return err
	}

	return nil
}

func (p *ProofOfStake) ResetNilBlock(
	validator common.Address, state *state.StateDB, header *types.Header) error {
	method := staking.GetContract_Method_ResetNilBlock()
	abiData, err := p.GetStakingContractAbi()
	if err != nil {
		log.Error("ResetNilBlock abi error", "err", err)
		return err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())

	// call
	data, err := encodeCall(&abiData, method, validator)
	if err != nil {
		log.Error("Unable to pack ResetNilBlock", "error", err)
		return err
	}

	msgData := (hexutil.Bytes)(data)
	var from common.Address
	from.CopyFrom(ZERO_ADDRESS)
	args := ethapi.TransactionArgs{
		From: &from,
		To:   &contractAddress,
		Data: &msgData,
	}

	msg, err := args.ToMessage(math.MaxUint64)
	if err != nil {
		return err
	}

	_, err = p.blockchain.ExecuteNoGas(msg, state, header)
	if err != nil {
		return err
	}

	return nil
}

func (p *ProofOfStake) ListValidatorsAsMap(blockHash common.Hash) (map[common.Address]*ValidatorDetailsV2, error) {
	header := p.blockchain.GetHeaderByHash(blockHash)
	if header != nil && header.Number.Uint64() >= defaults.DefaultConfig.PosConfig.SystemContractV3StartBlock {
		startTime := time.Now()
		detailsList, err := p.listValidatorStakingDetailsV3(blockHash)
		if err != nil {
			log.Debug("ListValidatorsAsMap listValidatorStakingDetailsV3 failed", "err", err)
			return nil, err
		}

		validatorMap := make(map[common.Address]*ValidatorDetailsV2)
		for _, details := range detailsList {
			if details.Validator.IsEqualTo(ZERO_ADDRESS) {
				return nil, errors.New("invalid validator")
			}
			if details.Depositor.IsEqualTo(ZERO_ADDRESS) {
				log.Debug("ListValidatorsAsMap invalid depositor", "validator", details.Validator)
				continue
			}
			validatorMap[details.Validator] = details
		}
		log.Debug("ListValidatorsAsMap V3 done", "blockNumber", header.Number.Uint64(), "validators", len(validatorMap), "elapsed", time.Since(startTime))
		return validatorMap, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	method := staking.GetContract_Method_ListValidators()
	abiData, err := staking.GetStakingContractV2_ABI()
	if err != nil {
		log.Error("ListValidatorsAsMap error getting abidata", "err", err)
		return nil, err
	}
	contractAddress := common.HexToAddress(staking.GetStakingContract_Address_String())
	// call
	data, err := abiData.Pack(method)
	if err != nil {
		log.Error("Unable to pack tx for get ListValidatorsAsMap", "error", err)
		return nil, err
	}
	// block
	blockNr := rpc.BlockNumberOrHashWithHash(blockHash, false)

	msgData := (hexutil.Bytes)(data)
	result, err := p.ethAPI.Call(ctx, ethapi.TransactionArgs{
		To:   &contractAddress,
		Data: &msgData,
	}, blockNr, nil)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		log.Debug("result 0 length")
		return nil, nil
	}
	_, err = abiData.Unpack(method, result)
	if err != nil {
		log.Error("Unpack", "err", err)
		return nil, err
	}

	var (
		ret0 = new([]common.Address)
	)
	out := ret0

	if err := abiData.UnpackIntoInterface(out, method, result); err != nil {
		log.Info("UnpackIntoInterface error")
		return nil, err
	}
	var validatorMap map[common.Address]*ValidatorDetailsV2
	validatorMap = make(map[common.Address]*ValidatorDetailsV2)

	for _, val := range *out {
		if val.IsEqualTo(ZERO_ADDRESS) {
			return nil, errors.New("invalid validator")
		}
		log.Debug("ListValidatorsAsMap Validator", "val", val)
	}
	for _, val := range *out {

		depositor, err := p.GetDepositorOfValidator(val, blockHash)
		if err != nil {
			log.Debug("GetDepositorOfValidator failed", "err", err)
			continue
		}

		if depositor.IsEqualTo(ZERO_ADDRESS) {
			log.Debug("ListValidatorsAsMap invalid depositor", "validator", val)
			continue
		}

		validatorDetailsV2, err := p.GetStakingDetailsByValidatorAddressV2(val, blockHash)

		if err != nil {
			return nil, err
		}

		validatorMap[val] = validatorDetailsV2
	}

	return validatorMap, nil
}
