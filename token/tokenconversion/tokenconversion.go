// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package tokenconversion

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/QuantumCoinProject/qc"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/accounts/abi/bind"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/event"
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

// TokenconversionMetaData contains all meta data concerning the Tokenconversion contract.
var TokenconversionMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"quantumAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ethereumSignature\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"OnRequestConversion\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"submitterAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"string\",\"name\":\"burnProof\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"OnSubmitBurnProof\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"BurnProofs\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"ConversionRequests\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"quantumAddress\",\"type\":\"address\"},{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"name\":\"requestConversion\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"burnProof\",\"type\":\"string\"}],\"name\":\"submitBurnProof\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50610a8a806100206000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c80631947f47d1461005157806325a22ff2146100815780639b51e522146100b1578063bdf36177146100e1575b600080fd5b61006b60048036038101906100669190610720565b610113565b604051610078919061095a565b60405180910390f35b61009b60048036038101906100969190610795565b61027c565b6040516100a8919061091d565b60405180910390f35b6100cb60048036038101906100c691906106db565b610338565b6040516100d8919061095a565b60405180910390f35b6100fb60048036038101906100f69190610795565b6103d9565b60405161010a9392919061088f565b60405180910390f35b600080604051806060016040528033815260200187878080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f82011690508083019250505050505050815260200185858080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050508152509080600181540180825580915050600190039060005260206000209060030201600090919091909150600082015181600001556020820151816001019080519060200190610208929190610543565b506040820151816002019080519060200190610225929190610543565b505050337f9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e868686866001600080549050036040516102689594939291906108d4565b60405180910390a260009050949350505050565b6001818154811061028c57600080fd5b906000526020600020016000915090508054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156103305780601f1061030557610100808354040283529160200191610330565b820191906000526020600020905b81548152906001019060200180831161031357829003601f168201915b505050505081565b600060018383909180600181540180825580915050600190039060005260206000200160009091929091929091929091925091906103779291906105d1565b508282604051610388929190610876565b6040518091039020337f71fff81ad7d08b41949ed8f7b7d774270ad346639beef32f30c5cfa5de3518c360018080549050036040516103c7919061093f565b60405180910390a36000905092915050565b600081815481106103e957600080fd5b9060005260206000209060030201600091509050806000015490806001018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561049b5780601f106104705761010080835404028352916020019161049b565b820191906000526020600020905b81548152906001019060200180831161047e57829003601f168201915b505050505090806002018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105395780601f1061050e57610100808354040283529160200191610539565b820191906000526020600020905b81548152906001019060200180831161051c57829003601f168201915b5050505050905083565b828054600181600116156101000203166002900490600052602060002090601f01602090048101928261057957600085556105c0565b82601f1061059257805160ff19168380011785556105c0565b828001600101855582156105c0579182015b828111156105bf5782518255916020019190600101906105a4565b5b5090506105cd919061065f565b5090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282610607576000855561064e565b82601f1061062057803560ff191683800117855561064e565b8280016001018555821561064e579182015b8281111561064d578235825591602001919060010190610632565b5b50905061065b919061065f565b5090565b5b80821115610678576000816000905550600101610660565b5090565b60008083601f84011261068e57600080fd5b8235905067ffffffffffffffff8111156106a757600080fd5b6020830191508360018202830111156106bf57600080fd5b9250929050565b6000813590506106d581610a18565b92915050565b600080602083850312156106ee57600080fd5b600083013567ffffffffffffffff81111561070857600080fd5b6107148582860161067c565b92509250509250929050565b6000806000806040858703121561073657600080fd5b600085013567ffffffffffffffff81111561075057600080fd5b61075c8782880161067c565b9450945050602085013567ffffffffffffffff81111561077b57600080fd5b6107878782880161067c565b925092505092959194509250565b6000602082840312156107a757600080fd5b60006107b5848285016106c6565b91505092915050565b6107c78161099c565b82525050565b60006107d98385610980565b93506107e68385846109c5565b6107ef83610a07565b840190509392505050565b60006108068385610991565b93506108138385846109c5565b82840190509392505050565b600061082a82610975565b6108348185610980565b93506108448185602086016109d4565b61084d81610a07565b840191505092915050565b610861816109ae565b82525050565b610870816109b8565b82525050565b60006108838284866107fa565b91508190509392505050565b60006060820190506108a460008301866107be565b81810360208301526108b6818561081f565b905081810360408301526108ca818461081f565b9050949350505050565b600060608201905081810360008301526108ef8187896107cd565b905081810360208301526109048185876107cd565b90506109136040830184610858565b9695505050505050565b60006020820190508181036000830152610937818461081f565b905092915050565b60006020820190506109546000830184610858565b92915050565b600060208201905061096f6000830184610867565b92915050565b600081519050919050565b600082825260208201905092915050565b600081905092915050565b60006109a7826109ae565b9050919050565b6000819050919050565b600060ff82169050919050565b82818337600083830152505050565b60005b838110156109f25780820151818401526020810190506109d7565b83811115610a01576000848401525b50505050565b6000601f19601f8301169050919050565b610a21816109ae565b8114610a2c57600080fd5b5056fea26469706673582212206d06226e930f1a0cb2f0e478347d985f8e70c217a8268d8d1cb04d72f07bfc7f64736f6c637827302e372e362d646576656c6f702e323032352e322e31332b636f6d6d69742e34663635373333610058",
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
