// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package tokenv2

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/quantumcoinproject/quantum-coin-go"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi/bind"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// Tokenv2MetaData contains all meta data concerning the Tokenv2 contract.
var Tokenv2MetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"string\",\"name\":\"tokenName\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"tokenSymbol\",\"type\":\"string\"},{\"internalType\":\"uint256\",\"name\":\"tokenTotalSupply\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"tokenDecimals\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"ownerAccount\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"accountOwner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"accountOwner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"subtractedValue\",\"type\":\"uint256\"}],\"name\":\"decreaseAllowance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"addedValue\",\"type\":\"uint256\"}],\"name\":\"increaseAllowance\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"receivers\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"name\":\"multiTransfer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60806040523480156200001157600080fd5b506040516200153538038062001535833981810160405260a08110156200003757600080fd5b81019080805160405193929190846401000000008211156200005857600080fd5b838201915060208201858111156200006f57600080fd5b82518660018202830111640100000000821117156200008d57600080fd5b8083526020830192505050908051906020019080838360005b83811015620000c3578082015181840152602081019050620000a6565b50505050905090810190601f168015620000f15780820380516001836020036101000a031916815260200191505b50604052602001805160405193929190846401000000008211156200011557600080fd5b838201915060208201858111156200012c57600080fd5b82518660018202830111640100000000821117156200014a57600080fd5b8083526020830192505050908051906020019080838360005b838110156200018057808201518184015260208101905062000163565b50505050905090810190601f168015620001ae5780820380516001836020036101000a031916815260200191505b506040526020018051906020019092919080519060200190929190805190602001909291905050508460039080519060200190620001ee929190620003e2565b50836004908051906020019062000207929190620003e2565b508260028190555081600560006101000a81548160ff021916908360ff160217905550806006819055506200024381846200024e60201b60201c565b505050505062000498565b600060025414620002c7576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260158152602001807f546f6b656e3a20616c7265616479206d696e746564000000000000000000000081525060200191505060405180910390fd5b600081141562000323576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602c81526020018062001509602c913960400191505060405180910390fd5b6200033f81600254620003c560201b62000f3a1790919060201c565b600281905550620003718160008085815260200190815260200160002054620003c560201b62000f3a1790919060201c565b600080848152602001908152602001600020819055508160007fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a35050565b600080828401905083811015620003d857fe5b8091505092915050565b828054600181600116156101000203166002900490600052602060002090601f0160209004810192826200041a576000855562000466565b82601f106200043557805160ff191683800117855562000466565b8280016001018555821562000466579182015b828111156200046557825182559160200191906001019062000448565b5b50905062000475919062000479565b5090565b5b80821115620004945760008160009055506001016200047a565b5090565b61106180620004a86000396000f3fe608060405234801561001057600080fd5b50600436106100f55760003560e01c806370a0823111610097578063a457c2d711610066578063a457c2d7146104e9578063a9059cbb14610537578063dd62ed3e14610585578063f2fde38b146105d1576100f5565b806370a08231146103fc578063715018a61461043e5780638da5cb5b1461044857806395d89b4114610466576100f5565b80631e89d545116100d35780631e89d545146101e957806323b872dd14610335578063313ce5671461038d57806339509351146103ae576100f5565b806306fdde03146100fa578063095ea7b31461017d57806318160ddd146101cb575b600080fd5b6101026105ff565b6040518080602001828103825283818151815260200191508051906020019080838360005b83811015610142578082015181840152602081019050610127565b50505050905090810190601f16801561016f5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6101b36004803603604081101561019357600080fd5b8101908080359060200190929190803590602001909291905050506106a1565b60405180821515815260200191505060405180910390f35b6101d361071c565b6040518082815260200191505060405180910390f35b610333600480360360408110156101ff57600080fd5b810190808035906020019064010000000081111561021c57600080fd5b82018360208201111561022e57600080fd5b8035906020019184602083028401116401000000008311171561025057600080fd5b919080806020026020016040519081016040528093929190818152602001838360200280828437600081840152601f19601f820116905080830192505050505050509192919290803590602001906401000000008111156102b057600080fd5b8201836020820111156102c257600080fd5b803590602001918460208302840111640100000000831117156102e457600080fd5b919080806020026020016040519081016040528093929190818152602001838360200280828437600081840152601f19601f820116905080830192505050505050509192919290505050610726565b005b6103756004803603606081101561034b57600080fd5b81019080803590602001909291908035906020019092919080359060200190929190505050610863565b60405180821515815260200191505060405180910390f35b610395610aa0565b604051808260ff16815260200191505060405180910390f35b6103e4600480360360408110156103c457600080fd5b810190808035906020019092919080359060200190929190505050610ab7565b60405180821515815260200191505060405180910390f35b6104286004803603602081101561041257600080fd5b8101908080359060200190929190505050610b8c565b6040518082815260200191505060405180910390f35b610446610ba8565b005b610450610bbc565b6040518082815260200191505060405180910390f35b61046e610bc6565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156104ae578082015181840152602081019050610493565b50505050905090810190601f1680156104db5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b61051f600480360360408110156104ff57600080fd5b810190808035906020019092919080359060200190929190505050610c68565b60405180821515815260200191505060405180910390f35b61056d6004803603604081101561054d57600080fd5b810190808035906020019092919080359060200190929190505050610d3d565b60405180821515815260200191505060405180910390f35b6105bb6004803603604081101561059b57600080fd5b810190808035906020019092919080359060200190929190505050610e80565b6040518082815260200191505060405180910390f35b6105fd600480360360208110156105e757600080fd5b8101908080359060200190929190505050610eaf565b005b606060038054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156106975780601f1061066c57610100808354040283529160200191610697565b820191906000526020600020905b81548152906001019060200180831161067a57829003601f168201915b5050505050905090565b6000808314156106b057600080fd5b816001600033815260200190815260200160002060008581526020019081526020016000208190555082337f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040518082815260200191505060405180910390a36001905092915050565b6000600254905090565b805182511461079d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601d8152602001807f546f6b656e3a20617272617973206c656e677468206d69736d6174636800000081525060200191505060405180910390fd5b6000825111610814576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260138152602001807f546f6b656e3a20656d707479206172726179730000000000000000000000000081525060200191505060405180910390fd5b60005b825181101561085e5761085083828151811061082f57fe5b602002602001015183838151811061084357fe5b6020026020010151610d3d565b508080600101915050610817565b505050565b6000806000858152602001908152602001600020548211156108ed576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601b8152602001807f546f6b656e3a20696e73756666696369656e742062616c616e6365000000000081525060200191505060405180910390fd5b60016000858152602001908152602001600020600033815260200190815260200160002054821115610987576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601d8152602001807f546f6b656e3a20696e73756666696369656e7420616c6c6f77616e636500000081525060200191505060405180910390fd5b6109ac8260008087815260200190815260200160002054610f5690919063ffffffff16565b600080868152602001908152602001600020819055506109e78260008086815260200190815260200160002054610f3a90919063ffffffff16565b60008085815260200190815260200160002081905550610a348260016000878152602001908152602001600020600033815260200190815260200160002054610f5690919063ffffffff16565b6001600086815260200190815260200160002060003381526020019081526020016000208190555082847fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a3600190509392505050565b6000600560009054906101000a900460ff16905090565b600080831415610ac657600080fd5b610afd8260016000338152602001908152602001600020600086815260200190815260200160002054610f3a90919063ffffffff16565b6001600033815260200190815260200160002060008581526020019081526020016000208190555082337f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600160003381526020019081526020016000206000878152602001908152602001600020546040518082815260200191505060405180910390a36001905092915050565b6000806000838152602001908152602001600020549050919050565b610bb0610f6d565b610bba6000610feb565b565b6000600654905090565b606060048054600181600116156101000203166002900480601f016020809104026020016040519081016040528092919081815260200182805460018160011615610100020316600290048015610c5e5780601f10610c3357610100808354040283529160200191610c5e565b820191906000526020600020905b815481529060010190602001808311610c4157829003601f168201915b5050505050905090565b600080831415610c7757600080fd5b610cae8260016000338152602001908152602001600020600086815260200190815260200160002054610f5690919063ffffffff16565b6001600033815260200190815260200160002060008581526020019081526020016000208190555082337f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925600160003381526020019081526020016000206000878152602001908152602001600020546040518082815260200191505060405180910390a36001905092915050565b600080600033815260200190815260200160002054821115610dc7576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601b8152602001807f546f6b656e3a20696e73756666696369656e742062616c616e6365000000000081525060200191505060405180910390fd5b610dec8260008033815260200190815260200160002054610f5690919063ffffffff16565b60008033815260200190815260200160002081905550610e278260008086815260200190815260200160002054610f3a90919063ffffffff16565b6000808581526020019081526020016000208190555082337fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040518082815260200191505060405180910390a36001905092915050565b600060016000848152602001908152602001600020600083815260200190815260200160002054905092915050565b610eb7610f6d565b6000811415610f2e576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260138152602001807f4f776e61626c65496e76616c69644f776e65720000000000000000000000000081525060200191505060405180910390fd5b610f3781610feb565b50565b600080828401905083811015610f4c57fe5b8091505092915050565b600082821115610f6257fe5b818303905092915050565b33610f76610bbc565b14610fe9576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601a8152602001807f546f6b656e3a2073656e646572206973206e6f74206f776e657200000000000081525060200191505060405180910390fd5b565b600060065490508160068190555081817f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e060405160405180910390a3505056fea2646970667358221220e1f1fec53cc14f3b9d6f2f843c4e77c8eff81f147be1ba1b3f525ed65706b4e764736f6c63430007060033546f6b656e3a206d696e7420616d6f756e74206d7573742062652067726561746572207468616e207a65726f",
}

// Tokenv2ABI is the input ABI used to generate the binding from.
// Deprecated: Use Tokenv2MetaData.ABI instead.
var Tokenv2ABI = Tokenv2MetaData.ABI

// Tokenv2Bin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use Tokenv2MetaData.Bin instead.
var Tokenv2Bin = Tokenv2MetaData.Bin

// DeployTokenv2 deploys a new Ethereum contract, binding an instance of Tokenv2 to it.
func DeployTokenv2(auth *bind.TransactOpts, backend bind.ContractBackend, tokenName string, tokenSymbol string, tokenTotalSupply *big.Int, tokenDecimals uint8, ownerAccount common.Address) (common.Address, *types.Transaction, *Tokenv2, error) {
	parsed, err := Tokenv2MetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(Tokenv2Bin), backend, tokenName, tokenSymbol, tokenTotalSupply, tokenDecimals, ownerAccount)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Tokenv2{Tokenv2Caller: Tokenv2Caller{contract: contract}, Tokenv2Transactor: Tokenv2Transactor{contract: contract}, Tokenv2Filterer: Tokenv2Filterer{contract: contract}}, nil
}

// Tokenv2 is an auto generated Go binding around an Ethereum contract.
type Tokenv2 struct {
	Tokenv2Caller     // Read-only binding to the contract
	Tokenv2Transactor // Write-only binding to the contract
	Tokenv2Filterer   // Log filterer for contract events
}

// Tokenv2Caller is an auto generated read-only Go binding around an Ethereum contract.
type Tokenv2Caller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// Tokenv2Transactor is an auto generated write-only Go binding around an Ethereum contract.
type Tokenv2Transactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// Tokenv2Filterer is an auto generated log filtering Go binding around an Ethereum contract events.
type Tokenv2Filterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// Tokenv2Session is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type Tokenv2Session struct {
	Contract     *Tokenv2          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// Tokenv2CallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type Tokenv2CallerSession struct {
	Contract *Tokenv2Caller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// Tokenv2TransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type Tokenv2TransactorSession struct {
	Contract     *Tokenv2Transactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// Tokenv2Raw is an auto generated low-level Go binding around an Ethereum contract.
type Tokenv2Raw struct {
	Contract *Tokenv2 // Generic contract binding to access the raw methods on
}

// Tokenv2CallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type Tokenv2CallerRaw struct {
	Contract *Tokenv2Caller // Generic read-only contract binding to access the raw methods on
}

// Tokenv2TransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type Tokenv2TransactorRaw struct {
	Contract *Tokenv2Transactor // Generic write-only contract binding to access the raw methods on
}

// NewTokenv2 creates a new instance of Tokenv2, bound to a specific deployed contract.
func NewTokenv2(address common.Address, backend bind.ContractBackend) (*Tokenv2, error) {
	contract, err := bindTokenv2(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Tokenv2{Tokenv2Caller: Tokenv2Caller{contract: contract}, Tokenv2Transactor: Tokenv2Transactor{contract: contract}, Tokenv2Filterer: Tokenv2Filterer{contract: contract}}, nil
}

// NewTokenv2Caller creates a new read-only instance of Tokenv2, bound to a specific deployed contract.
func NewTokenv2Caller(address common.Address, caller bind.ContractCaller) (*Tokenv2Caller, error) {
	contract, err := bindTokenv2(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Tokenv2Caller{contract: contract}, nil
}

// NewTokenv2Transactor creates a new write-only instance of Tokenv2, bound to a specific deployed contract.
func NewTokenv2Transactor(address common.Address, transactor bind.ContractTransactor) (*Tokenv2Transactor, error) {
	contract, err := bindTokenv2(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &Tokenv2Transactor{contract: contract}, nil
}

// NewTokenv2Filterer creates a new log filterer instance of Tokenv2, bound to a specific deployed contract.
func NewTokenv2Filterer(address common.Address, filterer bind.ContractFilterer) (*Tokenv2Filterer, error) {
	contract, err := bindTokenv2(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &Tokenv2Filterer{contract: contract}, nil
}

// bindTokenv2 binds a generic wrapper to an already deployed contract.
func bindTokenv2(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(Tokenv2ABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Tokenv2 *Tokenv2Raw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Tokenv2.Contract.Tokenv2Caller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Tokenv2 *Tokenv2Raw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Tokenv2.Contract.Tokenv2Transactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Tokenv2 *Tokenv2Raw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Tokenv2.Contract.Tokenv2Transactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Tokenv2 *Tokenv2CallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Tokenv2.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Tokenv2 *Tokenv2TransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Tokenv2.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Tokenv2 *Tokenv2TransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Tokenv2.Contract.contract.Transact(opts, method, params...)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address accountOwner, address spender) view returns(uint256)
func (_Tokenv2 *Tokenv2Caller) Allowance(opts *bind.CallOpts, accountOwner common.Address, spender common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "allowance", accountOwner, spender)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address accountOwner, address spender) view returns(uint256)
func (_Tokenv2 *Tokenv2Session) Allowance(accountOwner common.Address, spender common.Address) (*big.Int, error) {
	return _Tokenv2.Contract.Allowance(&_Tokenv2.CallOpts, accountOwner, spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address accountOwner, address spender) view returns(uint256)
func (_Tokenv2 *Tokenv2CallerSession) Allowance(accountOwner common.Address, spender common.Address) (*big.Int, error) {
	return _Tokenv2.Contract.Allowance(&_Tokenv2.CallOpts, accountOwner, spender)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address accountOwner) view returns(uint256)
func (_Tokenv2 *Tokenv2Caller) BalanceOf(opts *bind.CallOpts, accountOwner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "balanceOf", accountOwner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address accountOwner) view returns(uint256)
func (_Tokenv2 *Tokenv2Session) BalanceOf(accountOwner common.Address) (*big.Int, error) {
	return _Tokenv2.Contract.BalanceOf(&_Tokenv2.CallOpts, accountOwner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address accountOwner) view returns(uint256)
func (_Tokenv2 *Tokenv2CallerSession) BalanceOf(accountOwner common.Address) (*big.Int, error) {
	return _Tokenv2.Contract.BalanceOf(&_Tokenv2.CallOpts, accountOwner)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_Tokenv2 *Tokenv2Caller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "decimals")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_Tokenv2 *Tokenv2Session) Decimals() (uint8, error) {
	return _Tokenv2.Contract.Decimals(&_Tokenv2.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_Tokenv2 *Tokenv2CallerSession) Decimals() (uint8, error) {
	return _Tokenv2.Contract.Decimals(&_Tokenv2.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_Tokenv2 *Tokenv2Caller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_Tokenv2 *Tokenv2Session) Name() (string, error) {
	return _Tokenv2.Contract.Name(&_Tokenv2.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_Tokenv2 *Tokenv2CallerSession) Name() (string, error) {
	return _Tokenv2.Contract.Name(&_Tokenv2.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Tokenv2 *Tokenv2Caller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Tokenv2 *Tokenv2Session) Owner() (common.Address, error) {
	return _Tokenv2.Contract.Owner(&_Tokenv2.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Tokenv2 *Tokenv2CallerSession) Owner() (common.Address, error) {
	return _Tokenv2.Contract.Owner(&_Tokenv2.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_Tokenv2 *Tokenv2Caller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_Tokenv2 *Tokenv2Session) Symbol() (string, error) {
	return _Tokenv2.Contract.Symbol(&_Tokenv2.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_Tokenv2 *Tokenv2CallerSession) Symbol() (string, error) {
	return _Tokenv2.Contract.Symbol(&_Tokenv2.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_Tokenv2 *Tokenv2Caller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Tokenv2.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_Tokenv2 *Tokenv2Session) TotalSupply() (*big.Int, error) {
	return _Tokenv2.Contract.TotalSupply(&_Tokenv2.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_Tokenv2 *Tokenv2CallerSession) TotalSupply() (*big.Int, error) {
	return _Tokenv2.Contract.TotalSupply(&_Tokenv2.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2Transactor) Approve(opts *bind.TransactOpts, spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "approve", spender, value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2Session) Approve(spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.Approve(&_Tokenv2.TransactOpts, spender, value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address spender, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2TransactorSession) Approve(spender common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.Approve(&_Tokenv2.TransactOpts, spender, value)
}

// DecreaseAllowance is a paid mutator transaction binding the contract method 0xa457c2d7.
//
// Solidity: function decreaseAllowance(address spender, uint256 subtractedValue) returns(bool)
func (_Tokenv2 *Tokenv2Transactor) DecreaseAllowance(opts *bind.TransactOpts, spender common.Address, subtractedValue *big.Int) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "decreaseAllowance", spender, subtractedValue)
}

// DecreaseAllowance is a paid mutator transaction binding the contract method 0xa457c2d7.
//
// Solidity: function decreaseAllowance(address spender, uint256 subtractedValue) returns(bool)
func (_Tokenv2 *Tokenv2Session) DecreaseAllowance(spender common.Address, subtractedValue *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.DecreaseAllowance(&_Tokenv2.TransactOpts, spender, subtractedValue)
}

// DecreaseAllowance is a paid mutator transaction binding the contract method 0xa457c2d7.
//
// Solidity: function decreaseAllowance(address spender, uint256 subtractedValue) returns(bool)
func (_Tokenv2 *Tokenv2TransactorSession) DecreaseAllowance(spender common.Address, subtractedValue *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.DecreaseAllowance(&_Tokenv2.TransactOpts, spender, subtractedValue)
}

// IncreaseAllowance is a paid mutator transaction binding the contract method 0x39509351.
//
// Solidity: function increaseAllowance(address spender, uint256 addedValue) returns(bool)
func (_Tokenv2 *Tokenv2Transactor) IncreaseAllowance(opts *bind.TransactOpts, spender common.Address, addedValue *big.Int) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "increaseAllowance", spender, addedValue)
}

// IncreaseAllowance is a paid mutator transaction binding the contract method 0x39509351.
//
// Solidity: function increaseAllowance(address spender, uint256 addedValue) returns(bool)
func (_Tokenv2 *Tokenv2Session) IncreaseAllowance(spender common.Address, addedValue *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.IncreaseAllowance(&_Tokenv2.TransactOpts, spender, addedValue)
}

// IncreaseAllowance is a paid mutator transaction binding the contract method 0x39509351.
//
// Solidity: function increaseAllowance(address spender, uint256 addedValue) returns(bool)
func (_Tokenv2 *Tokenv2TransactorSession) IncreaseAllowance(spender common.Address, addedValue *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.IncreaseAllowance(&_Tokenv2.TransactOpts, spender, addedValue)
}

// MultiTransfer is a paid mutator transaction binding the contract method 0x1e89d545.
//
// Solidity: function multiTransfer(address[] receivers, uint256[] amounts) returns()
func (_Tokenv2 *Tokenv2Transactor) MultiTransfer(opts *bind.TransactOpts, receivers []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "multiTransfer", receivers, amounts)
}

// MultiTransfer is a paid mutator transaction binding the contract method 0x1e89d545.
//
// Solidity: function multiTransfer(address[] receivers, uint256[] amounts) returns()
func (_Tokenv2 *Tokenv2Session) MultiTransfer(receivers []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.MultiTransfer(&_Tokenv2.TransactOpts, receivers, amounts)
}

// MultiTransfer is a paid mutator transaction binding the contract method 0x1e89d545.
//
// Solidity: function multiTransfer(address[] receivers, uint256[] amounts) returns()
func (_Tokenv2 *Tokenv2TransactorSession) MultiTransfer(receivers []common.Address, amounts []*big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.MultiTransfer(&_Tokenv2.TransactOpts, receivers, amounts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Tokenv2 *Tokenv2Transactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Tokenv2 *Tokenv2Session) RenounceOwnership() (*types.Transaction, error) {
	return _Tokenv2.Contract.RenounceOwnership(&_Tokenv2.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Tokenv2 *Tokenv2TransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Tokenv2.Contract.RenounceOwnership(&_Tokenv2.TransactOpts)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2Transactor) Transfer(opts *bind.TransactOpts, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "transfer", to, value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2Session) Transfer(to common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.Transfer(&_Tokenv2.TransactOpts, to, value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address to, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2TransactorSession) Transfer(to common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.Transfer(&_Tokenv2.TransactOpts, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2Transactor) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "transferFrom", from, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2Session) TransferFrom(from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.TransferFrom(&_Tokenv2.TransactOpts, from, to, value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address from, address to, uint256 value) returns(bool)
func (_Tokenv2 *Tokenv2TransactorSession) TransferFrom(from common.Address, to common.Address, value *big.Int) (*types.Transaction, error) {
	return _Tokenv2.Contract.TransferFrom(&_Tokenv2.TransactOpts, from, to, value)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Tokenv2 *Tokenv2Transactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Tokenv2.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Tokenv2 *Tokenv2Session) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Tokenv2.Contract.TransferOwnership(&_Tokenv2.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Tokenv2 *Tokenv2TransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Tokenv2.Contract.TransferOwnership(&_Tokenv2.TransactOpts, newOwner)
}

// Tokenv2ApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the Tokenv2 contract.
type Tokenv2ApprovalIterator struct {
	Event *Tokenv2Approval // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *Tokenv2ApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(Tokenv2Approval)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(Tokenv2Approval)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *Tokenv2ApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *Tokenv2ApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// Tokenv2Approval represents a Approval event raised by the Tokenv2 contract.
type Tokenv2Approval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_Tokenv2 *Tokenv2Filterer) FilterApproval(opts *bind.FilterOpts, owner []common.Address, spender []common.Address) (*Tokenv2ApprovalIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _Tokenv2.contract.FilterLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return &Tokenv2ApprovalIterator{contract: _Tokenv2.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_Tokenv2 *Tokenv2Filterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *Tokenv2Approval, owner []common.Address, spender []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}
	var spenderRule []interface{}
	for _, spenderItem := range spender {
		spenderRule = append(spenderRule, spenderItem)
	}

	logs, sub, err := _Tokenv2.contract.WatchLogs(opts, "Approval", ownerRule, spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(Tokenv2Approval)
				if err := _Tokenv2.contract.UnpackLog(event, "Approval", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed owner, address indexed spender, uint256 value)
func (_Tokenv2 *Tokenv2Filterer) ParseApproval(log types.Log) (*Tokenv2Approval, error) {
	event := new(Tokenv2Approval)
	if err := _Tokenv2.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// Tokenv2OwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Tokenv2 contract.
type Tokenv2OwnershipTransferredIterator struct {
	Event *Tokenv2OwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *Tokenv2OwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(Tokenv2OwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(Tokenv2OwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *Tokenv2OwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *Tokenv2OwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// Tokenv2OwnershipTransferred represents a OwnershipTransferred event raised by the Tokenv2 contract.
type Tokenv2OwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Tokenv2 *Tokenv2Filterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*Tokenv2OwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Tokenv2.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &Tokenv2OwnershipTransferredIterator{contract: _Tokenv2.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Tokenv2 *Tokenv2Filterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *Tokenv2OwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Tokenv2.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(Tokenv2OwnershipTransferred)
				if err := _Tokenv2.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Tokenv2 *Tokenv2Filterer) ParseOwnershipTransferred(log types.Log) (*Tokenv2OwnershipTransferred, error) {
	event := new(Tokenv2OwnershipTransferred)
	if err := _Tokenv2.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// Tokenv2TransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the Tokenv2 contract.
type Tokenv2TransferIterator struct {
	Event *Tokenv2Transfer // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *Tokenv2TransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(Tokenv2Transfer)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(Tokenv2Transfer)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *Tokenv2TransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *Tokenv2TransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// Tokenv2Transfer represents a Transfer event raised by the Tokenv2 contract.
type Tokenv2Transfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_Tokenv2 *Tokenv2Filterer) FilterTransfer(opts *bind.FilterOpts, from []common.Address, to []common.Address) (*Tokenv2TransferIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _Tokenv2.contract.FilterLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &Tokenv2TransferIterator{contract: _Tokenv2.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_Tokenv2 *Tokenv2Filterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *Tokenv2Transfer, from []common.Address, to []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _Tokenv2.contract.WatchLogs(opts, "Transfer", fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(Tokenv2Transfer)
				if err := _Tokenv2.contract.UnpackLog(event, "Transfer", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed from, address indexed to, uint256 value)
func (_Tokenv2 *Tokenv2Filterer) ParseTransfer(log types.Log) (*Tokenv2Transfer, error) {
	event := new(Tokenv2Transfer)
	if err := _Tokenv2.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
