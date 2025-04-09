// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package tokenconversion

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

// TokenConversionContractConversionRequest is an auto generated low-level Go binding around an user-defined struct.
type TokenConversionContractConversionRequest struct {
	QuantumAddress common.Address
	EthAddress     string
	EthSignature   string
}

// TokenconversionMetaData contains all meta data concerning the Tokenconversion contract.
var TokenconversionMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"quantumAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ethereumSignature\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"OnRequestConversion\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"submitterAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"burnProof\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"OnSubmitBurnProof\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"BurnProofs\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"ConversionRequests\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"quantumAddress\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getBurnProof\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getBurnProofsCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"getConversionRequest\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"quantumAddress\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"internalType\":\"structTokenConversionContract.ConversionRequest\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getConversionRequestsCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"name\":\"requestConversion\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"burnProof\",\"type\":\"string\"}],\"name\":\"submitBurnProof\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50610f9a806100206000396000f3fe608060405234801561001057600080fd5b50600436106100885760003560e01c80639b51e5221161005b5780639b51e5221461014d578063bdf361771461017d578063de222fcf146101af578063eac5b245146101cd57610088565b80631947f47d1461008d57806325a22ff2146100bd57806333de954a146100ed5780636629b07b1461011d575b600080fd5b6100a760048036038101906100a29190610afe565b6101eb565b6040516100b49190610e59565b60405180910390f35b6100d760048036038101906100d29190610b73565b610354565b6040516100e49190610dda565b60405180910390f35b61010760048036038101906101029190610b73565b610410565b6040516101149190610e1c565b60405180910390f35b61013760048036038101906101329190610b73565b6105db565b6040516101449190610dda565b60405180910390f35b61016760048036038101906101629190610ab9565b6106dc565b6040516101749190610e59565b60405180910390f35b61019760048036038101906101929190610b73565b61077d565b6040516101a693929190610d4c565b60405180910390f35b6101b76108e7565b6040516101c49190610e3e565b60405180910390f35b6101d56108f4565b6040516101e29190610e3e565b60405180910390f35b600080604051806060016040528033815260200187878080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f82011690508083019250505050505050815260200185858080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505081525090806001815401808255809150506001900390600052602060002090600302016000909190919091506000820151816000015560208201518160010190805190602001906102e0929190610900565b5060408201518160020190805190602001906102fd929190610900565b505050337f9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e86868686600160008054905003604051610340959493929190610d91565b60405180910390a260009050949350505050565b6001818154811061036457600080fd5b906000526020600020016000915090508054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156104085780601f106103dd57610100808354040283529160200191610408565b820191906000526020600020905b8154815290600101906020018083116103eb57829003601f168201915b505050505081565b61041861098e565b600080549050821115610460576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161045790610dfc565b60405180910390fd5b6000828154811061046d57fe5b906000526020600020906003020160405180606001604052908160008201548152602001600182018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105295780601f106104fe57610100808354040283529160200191610529565b820191906000526020600020905b81548152906001019060200180831161050c57829003601f168201915b50505050508152602001600282018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105cb5780601f106105a0576101008083540402835291602001916105cb565b820191906000526020600020905b8154815290600101906020018083116105ae57829003601f168201915b5050505050815250509050919050565b6060600180549050821115610625576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161061c90610dfc565b60405180910390fd5b6001828154811061063257fe5b906000526020600020018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156106d05780601f106106a5576101008083540402835291602001916106d0565b820191906000526020600020905b8154815290600101906020018083116106b357829003601f168201915b50505050509050919050565b6000600183839091806001815401808255809150506001900390600052602060002001600090919290919290919290919250919061071b9291906109af565b50828260405161072c929190610d33565b6040518091039020337f71fff81ad7d08b41949ed8f7b7d774270ad346639beef32f30c5cfa5de3518c3600180805490500360405161076b9190610e3e565b60405180910390a36000905092915050565b6000818154811061078d57600080fd5b9060005260206000209060030201600091509050806000015490806001018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561083f5780601f106108145761010080835404028352916020019161083f565b820191906000526020600020905b81548152906001019060200180831161082257829003601f168201915b505050505090806002018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156108dd5780601f106108b2576101008083540402835291602001916108dd565b820191906000526020600020905b8154815290600101906020018083116108c057829003601f168201915b5050505050905083565b6000600180549050905090565b60008080549050905090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282610936576000855561097d565b82601f1061094f57805160ff191683800117855561097d565b8280016001018555821561097d579182015b8281111561097c578251825591602001919060010190610961565b5b50905061098a9190610a3d565b5090565b60405180606001604052806000815260200160608152602001606081525090565b828054600181600116156101000203166002900490600052602060002090601f0160209004810192826109e55760008555610a2c565b82601f106109fe57803560ff1916838001178555610a2c565b82800160010185558215610a2c579182015b82811115610a2b578235825591602001919060010190610a10565b5b509050610a399190610a3d565b5090565b5b80821115610a56576000816000905550600101610a3e565b5090565b60008083601f840112610a6c57600080fd5b8235905067ffffffffffffffff811115610a8557600080fd5b602083019150836001820283011115610a9d57600080fd5b9250929050565b600081359050610ab381610f28565b92915050565b60008060208385031215610acc57600080fd5b600083013567ffffffffffffffff811115610ae657600080fd5b610af285828601610a5a565b92509250509250929050565b60008060008060408587031215610b1457600080fd5b600085013567ffffffffffffffff811115610b2e57600080fd5b610b3a87828801610a5a565b9450945050602085013567ffffffffffffffff811115610b5957600080fd5b610b6587828801610a5a565b925092505092959194509250565b600060208284031215610b8557600080fd5b6000610b9384828501610aa4565b91505092915050565b610ba581610eac565b82525050565b610bb481610eac565b82525050565b6000610bc68385610e90565b9350610bd3838584610ed5565b610bdc83610f17565b840190509392505050565b6000610bf38385610ea1565b9350610c00838584610ed5565b82840190509392505050565b6000610c1782610e74565b610c218185610e7f565b9350610c31818560208601610ee4565b610c3a81610f17565b840191505092915050565b6000610c5082610e74565b610c5a8185610e90565b9350610c6a818560208601610ee4565b610c7381610f17565b840191505092915050565b6000610c8b600d83610e90565b91507f696e76616c696420696e646578000000000000000000000000000000000000006000830152602082019050919050565b6000606083016000830151610cd66000860182610b9c565b5060208301518482036020860152610cee8282610c0c565b91505060408301518482036040860152610d088282610c0c565b9150508091505092915050565b610d1e81610ebe565b82525050565b610d2d81610ec8565b82525050565b6000610d40828486610be7565b91508190509392505050565b6000606082019050610d616000830186610bab565b8181036020830152610d738185610c45565b90508181036040830152610d878184610c45565b9050949350505050565b60006060820190508181036000830152610dac818789610bba565b90508181036020830152610dc1818587610bba565b9050610dd06040830184610d15565b9695505050505050565b60006020820190508181036000830152610df48184610c45565b905092915050565b60006020820190508181036000830152610e1581610c7e565b9050919050565b60006020820190508181036000830152610e368184610cbe565b905092915050565b6000602082019050610e536000830184610d15565b92915050565b6000602082019050610e6e6000830184610d24565b92915050565b600081519050919050565b600082825260208201905092915050565b600082825260208201905092915050565b600081905092915050565b6000610eb782610ebe565b9050919050565b6000819050919050565b600060ff82169050919050565b82818337600083830152505050565b60005b83811015610f02578082015181840152602081019050610ee7565b83811115610f11576000848401525b50505050565b6000601f19601f8301169050919050565b610f3181610ebe565b8114610f3c57600080fd5b5056fea2646970667358221220c785bd464a07f72e5125cfbf1cf21a94a67a87828f7005d1e2de05b4c1d7b14964736f6c637827302e372e362d646576656c6f702e323032352e322e31332b636f6d6d69742e34663635373333610058",
}

// TokenconversionABI is the input ABI used to generate the binding from.
// Deprecated: Use TokenconversionMetaData.ABI instead.
var TokenconversionABI = TokenconversionMetaData.ABI

// TokenconversionBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use TokenconversionMetaData.Bin instead.
var TokenconversionBin = TokenconversionMetaData.Bin

// DeployTokenconversion deploys a new Ethereum contract, binding an instance of Tokenconversion to it.
func DeployTokenconversion(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Tokenconversion, error) {
	parsed, err := TokenconversionMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(TokenconversionBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Tokenconversion{TokenconversionCaller: TokenconversionCaller{contract: contract}, TokenconversionTransactor: TokenconversionTransactor{contract: contract}, TokenconversionFilterer: TokenconversionFilterer{contract: contract}}, nil
}

// Tokenconversion is an auto generated Go binding around an Ethereum contract.
type Tokenconversion struct {
	TokenconversionCaller     // Read-only binding to the contract
	TokenconversionTransactor // Write-only binding to the contract
	TokenconversionFilterer   // Log filterer for contract events
}

// TokenconversionCaller is an auto generated read-only Go binding around an Ethereum contract.
type TokenconversionCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenconversionTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TokenconversionTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenconversionFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type TokenconversionFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TokenconversionSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TokenconversionSession struct {
	Contract     *Tokenconversion  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TokenconversionCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TokenconversionCallerSession struct {
	Contract *TokenconversionCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// TokenconversionTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TokenconversionTransactorSession struct {
	Contract     *TokenconversionTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// TokenconversionRaw is an auto generated low-level Go binding around an Ethereum contract.
type TokenconversionRaw struct {
	Contract *Tokenconversion // Generic contract binding to access the raw methods on
}

// TokenconversionCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TokenconversionCallerRaw struct {
	Contract *TokenconversionCaller // Generic read-only contract binding to access the raw methods on
}

// TokenconversionTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TokenconversionTransactorRaw struct {
	Contract *TokenconversionTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTokenconversion creates a new instance of Tokenconversion, bound to a specific deployed contract.
func NewTokenconversion(address common.Address, backend bind.ContractBackend) (*Tokenconversion, error) {
	contract, err := bindTokenconversion(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Tokenconversion{TokenconversionCaller: TokenconversionCaller{contract: contract}, TokenconversionTransactor: TokenconversionTransactor{contract: contract}, TokenconversionFilterer: TokenconversionFilterer{contract: contract}}, nil
}

// NewTokenconversionCaller creates a new read-only instance of Tokenconversion, bound to a specific deployed contract.
func NewTokenconversionCaller(address common.Address, caller bind.ContractCaller) (*TokenconversionCaller, error) {
	contract, err := bindTokenconversion(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &TokenconversionCaller{contract: contract}, nil
}

// NewTokenconversionTransactor creates a new write-only instance of Tokenconversion, bound to a specific deployed contract.
func NewTokenconversionTransactor(address common.Address, transactor bind.ContractTransactor) (*TokenconversionTransactor, error) {
	contract, err := bindTokenconversion(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &TokenconversionTransactor{contract: contract}, nil
}

// NewTokenconversionFilterer creates a new log filterer instance of Tokenconversion, bound to a specific deployed contract.
func NewTokenconversionFilterer(address common.Address, filterer bind.ContractFilterer) (*TokenconversionFilterer, error) {
	contract, err := bindTokenconversion(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &TokenconversionFilterer{contract: contract}, nil
}

// bindTokenconversion binds a generic wrapper to an already deployed contract.
func bindTokenconversion(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TokenconversionABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Tokenconversion *TokenconversionRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Tokenconversion.Contract.TokenconversionCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Tokenconversion *TokenconversionRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Tokenconversion.Contract.TokenconversionTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Tokenconversion *TokenconversionRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Tokenconversion.Contract.TokenconversionTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Tokenconversion *TokenconversionCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Tokenconversion.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Tokenconversion *TokenconversionTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Tokenconversion.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Tokenconversion *TokenconversionTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Tokenconversion.Contract.contract.Transact(opts, method, params...)
}

// BurnProofs is a free data retrieval call binding the contract method 0x25a22ff2.
//
// Solidity: function BurnProofs(uint256 ) view returns(string)
func (_Tokenconversion *TokenconversionCaller) BurnProofs(opts *bind.CallOpts, arg0 *big.Int) (string, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "BurnProofs", arg0)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// BurnProofs is a free data retrieval call binding the contract method 0x25a22ff2.
//
// Solidity: function BurnProofs(uint256 ) view returns(string)
func (_Tokenconversion *TokenconversionSession) BurnProofs(arg0 *big.Int) (string, error) {
	return _Tokenconversion.Contract.BurnProofs(&_Tokenconversion.CallOpts, arg0)
}

// BurnProofs is a free data retrieval call binding the contract method 0x25a22ff2.
//
// Solidity: function BurnProofs(uint256 ) view returns(string)
func (_Tokenconversion *TokenconversionCallerSession) BurnProofs(arg0 *big.Int) (string, error) {
	return _Tokenconversion.Contract.BurnProofs(&_Tokenconversion.CallOpts, arg0)
}

// ConversionRequests is a free data retrieval call binding the contract method 0xbdf36177.
//
// Solidity: function ConversionRequests(uint256 ) view returns(address quantumAddress, string ethAddress, string ethSignature)
func (_Tokenconversion *TokenconversionCaller) ConversionRequests(opts *bind.CallOpts, arg0 *big.Int) (struct {
	QuantumAddress common.Address
	EthAddress     string
	EthSignature   string
}, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "ConversionRequests", arg0)

	outstruct := new(struct {
		QuantumAddress common.Address
		EthAddress     string
		EthSignature   string
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.QuantumAddress = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.EthAddress = *abi.ConvertType(out[1], new(string)).(*string)
	outstruct.EthSignature = *abi.ConvertType(out[2], new(string)).(*string)

	return *outstruct, err

}

// ConversionRequests is a free data retrieval call binding the contract method 0xbdf36177.
//
// Solidity: function ConversionRequests(uint256 ) view returns(address quantumAddress, string ethAddress, string ethSignature)
func (_Tokenconversion *TokenconversionSession) ConversionRequests(arg0 *big.Int) (struct {
	QuantumAddress common.Address
	EthAddress     string
	EthSignature   string
}, error) {
	return _Tokenconversion.Contract.ConversionRequests(&_Tokenconversion.CallOpts, arg0)
}

// ConversionRequests is a free data retrieval call binding the contract method 0xbdf36177.
//
// Solidity: function ConversionRequests(uint256 ) view returns(address quantumAddress, string ethAddress, string ethSignature)
func (_Tokenconversion *TokenconversionCallerSession) ConversionRequests(arg0 *big.Int) (struct {
	QuantumAddress common.Address
	EthAddress     string
	EthSignature   string
}, error) {
	return _Tokenconversion.Contract.ConversionRequests(&_Tokenconversion.CallOpts, arg0)
}

// GetBurnProof is a free data retrieval call binding the contract method 0x6629b07b.
//
// Solidity: function getBurnProof(uint256 index) view returns(string)
func (_Tokenconversion *TokenconversionCaller) GetBurnProof(opts *bind.CallOpts, index *big.Int) (string, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "getBurnProof", index)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// GetBurnProof is a free data retrieval call binding the contract method 0x6629b07b.
//
// Solidity: function getBurnProof(uint256 index) view returns(string)
func (_Tokenconversion *TokenconversionSession) GetBurnProof(index *big.Int) (string, error) {
	return _Tokenconversion.Contract.GetBurnProof(&_Tokenconversion.CallOpts, index)
}

// GetBurnProof is a free data retrieval call binding the contract method 0x6629b07b.
//
// Solidity: function getBurnProof(uint256 index) view returns(string)
func (_Tokenconversion *TokenconversionCallerSession) GetBurnProof(index *big.Int) (string, error) {
	return _Tokenconversion.Contract.GetBurnProof(&_Tokenconversion.CallOpts, index)
}

// GetBurnProofsCount is a free data retrieval call binding the contract method 0xde222fcf.
//
// Solidity: function getBurnProofsCount() view returns(uint256)
func (_Tokenconversion *TokenconversionCaller) GetBurnProofsCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "getBurnProofsCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetBurnProofsCount is a free data retrieval call binding the contract method 0xde222fcf.
//
// Solidity: function getBurnProofsCount() view returns(uint256)
func (_Tokenconversion *TokenconversionSession) GetBurnProofsCount() (*big.Int, error) {
	return _Tokenconversion.Contract.GetBurnProofsCount(&_Tokenconversion.CallOpts)
}

// GetBurnProofsCount is a free data retrieval call binding the contract method 0xde222fcf.
//
// Solidity: function getBurnProofsCount() view returns(uint256)
func (_Tokenconversion *TokenconversionCallerSession) GetBurnProofsCount() (*big.Int, error) {
	return _Tokenconversion.Contract.GetBurnProofsCount(&_Tokenconversion.CallOpts)
}

// GetConversionRequest is a free data retrieval call binding the contract method 0x33de954a.
//
// Solidity: function getConversionRequest(uint256 index) view returns((address,string,string))
func (_Tokenconversion *TokenconversionCaller) GetConversionRequest(opts *bind.CallOpts, index *big.Int) (TokenConversionContractConversionRequest, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "getConversionRequest", index)

	if err != nil {
		return *new(TokenConversionContractConversionRequest), err
	}

	out0 := *abi.ConvertType(out[0], new(TokenConversionContractConversionRequest)).(*TokenConversionContractConversionRequest)

	return out0, err

}

// GetConversionRequest is a free data retrieval call binding the contract method 0x33de954a.
//
// Solidity: function getConversionRequest(uint256 index) view returns((address,string,string))
func (_Tokenconversion *TokenconversionSession) GetConversionRequest(index *big.Int) (TokenConversionContractConversionRequest, error) {
	return _Tokenconversion.Contract.GetConversionRequest(&_Tokenconversion.CallOpts, index)
}

// GetConversionRequest is a free data retrieval call binding the contract method 0x33de954a.
//
// Solidity: function getConversionRequest(uint256 index) view returns((address,string,string))
func (_Tokenconversion *TokenconversionCallerSession) GetConversionRequest(index *big.Int) (TokenConversionContractConversionRequest, error) {
	return _Tokenconversion.Contract.GetConversionRequest(&_Tokenconversion.CallOpts, index)
}

// GetConversionRequestsCount is a free data retrieval call binding the contract method 0xeac5b245.
//
// Solidity: function getConversionRequestsCount() view returns(uint256)
func (_Tokenconversion *TokenconversionCaller) GetConversionRequestsCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "getConversionRequestsCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetConversionRequestsCount is a free data retrieval call binding the contract method 0xeac5b245.
//
// Solidity: function getConversionRequestsCount() view returns(uint256)
func (_Tokenconversion *TokenconversionSession) GetConversionRequestsCount() (*big.Int, error) {
	return _Tokenconversion.Contract.GetConversionRequestsCount(&_Tokenconversion.CallOpts)
}

// GetConversionRequestsCount is a free data retrieval call binding the contract method 0xeac5b245.
//
// Solidity: function getConversionRequestsCount() view returns(uint256)
func (_Tokenconversion *TokenconversionCallerSession) GetConversionRequestsCount() (*big.Int, error) {
	return _Tokenconversion.Contract.GetConversionRequestsCount(&_Tokenconversion.CallOpts)
}

// RequestConversion is a paid mutator transaction binding the contract method 0x1947f47d.
//
// Solidity: function requestConversion(string ethAddress, string ethSignature) returns(uint8)
func (_Tokenconversion *TokenconversionTransactor) RequestConversion(opts *bind.TransactOpts, ethAddress string, ethSignature string) (*types.Transaction, error) {
	return _Tokenconversion.contract.Transact(opts, "requestConversion", ethAddress, ethSignature)
}

// RequestConversion is a paid mutator transaction binding the contract method 0x1947f47d.
//
// Solidity: function requestConversion(string ethAddress, string ethSignature) returns(uint8)
func (_Tokenconversion *TokenconversionSession) RequestConversion(ethAddress string, ethSignature string) (*types.Transaction, error) {
	return _Tokenconversion.Contract.RequestConversion(&_Tokenconversion.TransactOpts, ethAddress, ethSignature)
}

// RequestConversion is a paid mutator transaction binding the contract method 0x1947f47d.
//
// Solidity: function requestConversion(string ethAddress, string ethSignature) returns(uint8)
func (_Tokenconversion *TokenconversionTransactorSession) RequestConversion(ethAddress string, ethSignature string) (*types.Transaction, error) {
	return _Tokenconversion.Contract.RequestConversion(&_Tokenconversion.TransactOpts, ethAddress, ethSignature)
}

// SubmitBurnProof is a paid mutator transaction binding the contract method 0x9b51e522.
//
// Solidity: function submitBurnProof(string burnProof) returns(uint8)
func (_Tokenconversion *TokenconversionTransactor) SubmitBurnProof(opts *bind.TransactOpts, burnProof string) (*types.Transaction, error) {
	return _Tokenconversion.contract.Transact(opts, "submitBurnProof", burnProof)
}

// SubmitBurnProof is a paid mutator transaction binding the contract method 0x9b51e522.
//
// Solidity: function submitBurnProof(string burnProof) returns(uint8)
func (_Tokenconversion *TokenconversionSession) SubmitBurnProof(burnProof string) (*types.Transaction, error) {
	return _Tokenconversion.Contract.SubmitBurnProof(&_Tokenconversion.TransactOpts, burnProof)
}

// SubmitBurnProof is a paid mutator transaction binding the contract method 0x9b51e522.
//
// Solidity: function submitBurnProof(string burnProof) returns(uint8)
func (_Tokenconversion *TokenconversionTransactorSession) SubmitBurnProof(burnProof string) (*types.Transaction, error) {
	return _Tokenconversion.Contract.SubmitBurnProof(&_Tokenconversion.TransactOpts, burnProof)
}

// TokenconversionOnRequestConversionIterator is returned from FilterOnRequestConversion and is used to iterate over the raw logs and unpacked data for OnRequestConversion events raised by the Tokenconversion contract.
type TokenconversionOnRequestConversionIterator struct {
	Event *TokenconversionOnRequestConversion // Event containing the contract specifics and raw log

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
func (it *TokenconversionOnRequestConversionIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenconversionOnRequestConversion)
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
		it.Event = new(TokenconversionOnRequestConversion)
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
func (it *TokenconversionOnRequestConversionIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenconversionOnRequestConversionIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenconversionOnRequestConversion represents a OnRequestConversion event raised by the Tokenconversion contract.
type TokenconversionOnRequestConversion struct {
	QuantumAddress    common.Address
	EthAddress        string
	EthereumSignature string
	Index             *big.Int
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterOnRequestConversion is a free log retrieval operation binding the contract event 0x9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e.
//
// Solidity: event OnRequestConversion(address indexed quantumAddress, string ethAddress, string ethereumSignature, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) FilterOnRequestConversion(opts *bind.FilterOpts, quantumAddress []common.Address) (*TokenconversionOnRequestConversionIterator, error) {

	var quantumAddressRule []interface{}
	for _, quantumAddressItem := range quantumAddress {
		quantumAddressRule = append(quantumAddressRule, quantumAddressItem)
	}

	logs, sub, err := _Tokenconversion.contract.FilterLogs(opts, "OnRequestConversion", quantumAddressRule)
	if err != nil {
		return nil, err
	}
	return &TokenconversionOnRequestConversionIterator{contract: _Tokenconversion.contract, event: "OnRequestConversion", logs: logs, sub: sub}, nil
}

// WatchOnRequestConversion is a free log subscription operation binding the contract event 0x9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e.
//
// Solidity: event OnRequestConversion(address indexed quantumAddress, string ethAddress, string ethereumSignature, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) WatchOnRequestConversion(opts *bind.WatchOpts, sink chan<- *TokenconversionOnRequestConversion, quantumAddress []common.Address) (event.Subscription, error) {

	var quantumAddressRule []interface{}
	for _, quantumAddressItem := range quantumAddress {
		quantumAddressRule = append(quantumAddressRule, quantumAddressItem)
	}

	logs, sub, err := _Tokenconversion.contract.WatchLogs(opts, "OnRequestConversion", quantumAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenconversionOnRequestConversion)
				if err := _Tokenconversion.contract.UnpackLog(event, "OnRequestConversion", log); err != nil {
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

// ParseOnRequestConversion is a log parse operation binding the contract event 0x9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e.
//
// Solidity: event OnRequestConversion(address indexed quantumAddress, string ethAddress, string ethereumSignature, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) ParseOnRequestConversion(log types.Log) (*TokenconversionOnRequestConversion, error) {
	event := new(TokenconversionOnRequestConversion)
	if err := _Tokenconversion.contract.UnpackLog(event, "OnRequestConversion", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// TokenconversionOnSubmitBurnProofIterator is returned from FilterOnSubmitBurnProof and is used to iterate over the raw logs and unpacked data for OnSubmitBurnProof events raised by the Tokenconversion contract.
type TokenconversionOnSubmitBurnProofIterator struct {
	Event *TokenconversionOnSubmitBurnProof // Event containing the contract specifics and raw log

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
func (it *TokenconversionOnSubmitBurnProofIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(TokenconversionOnSubmitBurnProof)
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
		it.Event = new(TokenconversionOnSubmitBurnProof)
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
func (it *TokenconversionOnSubmitBurnProofIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *TokenconversionOnSubmitBurnProofIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// TokenconversionOnSubmitBurnProof represents a OnSubmitBurnProof event raised by the Tokenconversion contract.
type TokenconversionOnSubmitBurnProof struct {
	SubmitterAddress common.Address
	BurnProof        common.Hash
	Index            *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterOnSubmitBurnProof is a free log retrieval operation binding the contract event 0x71fff81ad7d08b41949ed8f7b7d774270ad346639beef32f30c5cfa5de3518c3.
//
// Solidity: event OnSubmitBurnProof(address indexed submitterAddress, string indexed burnProof, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) FilterOnSubmitBurnProof(opts *bind.FilterOpts, submitterAddress []common.Address, burnProof []string) (*TokenconversionOnSubmitBurnProofIterator, error) {

	var submitterAddressRule []interface{}
	for _, submitterAddressItem := range submitterAddress {
		submitterAddressRule = append(submitterAddressRule, submitterAddressItem)
	}
	var burnProofRule []interface{}
	for _, burnProofItem := range burnProof {
		burnProofRule = append(burnProofRule, burnProofItem)
	}

	logs, sub, err := _Tokenconversion.contract.FilterLogs(opts, "OnSubmitBurnProof", submitterAddressRule, burnProofRule)
	if err != nil {
		return nil, err
	}
	return &TokenconversionOnSubmitBurnProofIterator{contract: _Tokenconversion.contract, event: "OnSubmitBurnProof", logs: logs, sub: sub}, nil
}

// WatchOnSubmitBurnProof is a free log subscription operation binding the contract event 0x71fff81ad7d08b41949ed8f7b7d774270ad346639beef32f30c5cfa5de3518c3.
//
// Solidity: event OnSubmitBurnProof(address indexed submitterAddress, string indexed burnProof, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) WatchOnSubmitBurnProof(opts *bind.WatchOpts, sink chan<- *TokenconversionOnSubmitBurnProof, submitterAddress []common.Address, burnProof []string) (event.Subscription, error) {

	var submitterAddressRule []interface{}
	for _, submitterAddressItem := range submitterAddress {
		submitterAddressRule = append(submitterAddressRule, submitterAddressItem)
	}
	var burnProofRule []interface{}
	for _, burnProofItem := range burnProof {
		burnProofRule = append(burnProofRule, burnProofItem)
	}

	logs, sub, err := _Tokenconversion.contract.WatchLogs(opts, "OnSubmitBurnProof", submitterAddressRule, burnProofRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(TokenconversionOnSubmitBurnProof)
				if err := _Tokenconversion.contract.UnpackLog(event, "OnSubmitBurnProof", log); err != nil {
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

// ParseOnSubmitBurnProof is a log parse operation binding the contract event 0x71fff81ad7d08b41949ed8f7b7d774270ad346639beef32f30c5cfa5de3518c3.
//
// Solidity: event OnSubmitBurnProof(address indexed submitterAddress, string indexed burnProof, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) ParseOnSubmitBurnProof(log types.Log) (*TokenconversionOnSubmitBurnProof, error) {
	event := new(TokenconversionOnSubmitBurnProof)
	if err := _Tokenconversion.contract.UnpackLog(event, "OnSubmitBurnProof", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
