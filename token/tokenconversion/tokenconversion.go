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
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"quantumAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ethereumSignature\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"OnRequestConversion\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"submitterAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"OnSubmitBurnProof\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"BurnProofs\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"ConversionRequests\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"ethAddress\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ethSignature\",\"type\":\"string\"}],\"name\":\"requestConversion\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"burnProof\",\"type\":\"string\"}],\"name\":\"submitBurnProof\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b506109e3806100206000396000f3fe608060405234801561001057600080fd5b506004361061004c5760003560e01c80631947f47d1461005157806325a22ff2146100815780639b51e522146100b1578063bdf36177146100e1575b600080fd5b61006b600480360381019061006691906106f1565b610112565b60405161007891906108d0565b60405180910390f35b61009b60048036038101906100969190610766565b61026b565b6040516100a8919061085c565b60405180910390f35b6100cb60048036038101906100c691906106ac565b610327565b6040516100d891906108d0565b60405180910390f35b6100fb60048036038101906100f69190610766565b6103b0565b60405161010992919061087e565b60405180910390f35b600080604051806040016040528087878080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f82011690508083019250505050505050815260200185858080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f82011690508083019250505050505050815250908060018154018082558091505060019003906000526020600020906002020160009091909190915060008201518160000190805190602001906101f7929190610514565b506020820151816001019080519060200190610214929190610514565b505050337f9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e86868686600160008054905003604051610257959493929190610813565b60405180910390a260009050949350505050565b6001818154811061027b57600080fd5b906000526020600020016000915090508054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561031f5780601f106102f45761010080835404028352916020019161031f565b820191906000526020600020905b81548152906001019060200180831161030257829003601f168201915b505050505081565b600060018383909180600181540180825580915050600190039060005260206000200160009091929091929091929091925091906103669291906105a2565b50337f2662c9d425c9a0213bcf36501662bad353f18b695745ced3aba5567c363551e2600180805490500360405161039e91906108b5565b60405180910390a26000905092915050565b600081815481106103c057600080fd5b9060005260206000209060020201600091509050806000018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561046c5780601f106104415761010080835404028352916020019161046c565b820191906000526020600020905b81548152906001019060200180831161044f57829003601f168201915b505050505090806001018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561050a5780601f106104df5761010080835404028352916020019161050a565b820191906000526020600020905b8154815290600101906020018083116104ed57829003601f168201915b5050505050905082565b828054600181600116156101000203166002900490600052602060002090601f01602090048101928261054a5760008555610591565b82601f1061056357805160ff1916838001178555610591565b82800160010185558215610591579182015b82811115610590578251825591602001919060010190610575565b5b50905061059e9190610630565b5090565b828054600181600116156101000203166002900490600052602060002090601f0160209004810192826105d8576000855561061f565b82601f106105f157803560ff191683800117855561061f565b8280016001018555821561061f579182015b8281111561061e578235825591602001919060010190610603565b5b50905061062c9190610630565b5090565b5b80821115610649576000816000905550600101610631565b5090565b60008083601f84011261065f57600080fd5b8235905067ffffffffffffffff81111561067857600080fd5b60208301915083600182028301111561069057600080fd5b9250929050565b6000813590506106a681610971565b92915050565b600080602083850312156106bf57600080fd5b600083013567ffffffffffffffff8111156106d957600080fd5b6106e58582860161064d565b92509250509250929050565b6000806000806040858703121561070757600080fd5b600085013567ffffffffffffffff81111561072157600080fd5b61072d8782880161064d565b9450945050602085013567ffffffffffffffff81111561074c57600080fd5b6107588782880161064d565b925092505092959194509250565b60006020828403121561077857600080fd5b600061078684828501610697565b91505092915050565b600061079b83856108f6565b93506107a883858461091e565b6107b183610960565b840190509392505050565b60006107c7826108eb565b6107d181856108f6565b93506107e181856020860161092d565b6107ea81610960565b840191505092915050565b6107fe81610907565b82525050565b61080d81610911565b82525050565b6000606082019050818103600083015261082e81878961078f565b9050818103602083015261084381858761078f565b905061085260408301846107f5565b9695505050505050565b6000602082019050818103600083015261087681846107bc565b905092915050565b6000604082019050818103600083015261089881856107bc565b905081810360208301526108ac81846107bc565b90509392505050565b60006020820190506108ca60008301846107f5565b92915050565b60006020820190506108e56000830184610804565b92915050565b600081519050919050565b600082825260208201905092915050565b6000819050919050565b600060ff82169050919050565b82818337600083830152505050565b60005b8381101561094b578082015181840152602081019050610930565b8381111561095a576000848401525b50505050565b6000601f19601f8301169050919050565b61097a81610907565b811461098557600080fd5b5056fea2646970667358221220d956d21e6af8d2ce8b648c8fa7fa93096f4581ea931128bf75fc5e682de97a7864736f6c637827302e372e362d646576656c6f702e323032352e322e31332b636f6d6d69742e34663635373333610058",
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
// Solidity: function ConversionRequests(uint256 ) view returns(string ethAddress, string ethSignature)
func (_Tokenconversion *TokenconversionCaller) ConversionRequests(opts *bind.CallOpts, arg0 *big.Int) (struct {
	EthAddress   string
	EthSignature string
}, error) {
	var out []interface{}
	err := _Tokenconversion.contract.Call(opts, &out, "ConversionRequests", arg0)

	outstruct := new(struct {
		EthAddress   string
		EthSignature string
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.EthAddress = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.EthSignature = *abi.ConvertType(out[1], new(string)).(*string)

	return *outstruct, err

}

// ConversionRequests is a free data retrieval call binding the contract method 0xbdf36177.
//
// Solidity: function ConversionRequests(uint256 ) view returns(string ethAddress, string ethSignature)
func (_Tokenconversion *TokenconversionSession) ConversionRequests(arg0 *big.Int) (struct {
	EthAddress   string
	EthSignature string
}, error) {
	return _Tokenconversion.Contract.ConversionRequests(&_Tokenconversion.CallOpts, arg0)
}

// ConversionRequests is a free data retrieval call binding the contract method 0xbdf36177.
//
// Solidity: function ConversionRequests(uint256 ) view returns(string ethAddress, string ethSignature)
func (_Tokenconversion *TokenconversionCallerSession) ConversionRequests(arg0 *big.Int) (struct {
	EthAddress   string
	EthSignature string
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
	Index            *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterOnSubmitBurnProof is a free log retrieval operation binding the contract event 0x2662c9d425c9a0213bcf36501662bad353f18b695745ced3aba5567c363551e2.
//
// Solidity: event OnSubmitBurnProof(address indexed submitterAddress, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) FilterOnSubmitBurnProof(opts *bind.FilterOpts, submitterAddress []common.Address) (*TokenconversionOnSubmitBurnProofIterator, error) {

	var submitterAddressRule []interface{}
	for _, submitterAddressItem := range submitterAddress {
		submitterAddressRule = append(submitterAddressRule, submitterAddressItem)
	}

	logs, sub, err := _Tokenconversion.contract.FilterLogs(opts, "OnSubmitBurnProof", submitterAddressRule)
	if err != nil {
		return nil, err
	}
	return &TokenconversionOnSubmitBurnProofIterator{contract: _Tokenconversion.contract, event: "OnSubmitBurnProof", logs: logs, sub: sub}, nil
}

// WatchOnSubmitBurnProof is a free log subscription operation binding the contract event 0x2662c9d425c9a0213bcf36501662bad353f18b695745ced3aba5567c363551e2.
//
// Solidity: event OnSubmitBurnProof(address indexed submitterAddress, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) WatchOnSubmitBurnProof(opts *bind.WatchOpts, sink chan<- *TokenconversionOnSubmitBurnProof, submitterAddress []common.Address) (event.Subscription, error) {

	var submitterAddressRule []interface{}
	for _, submitterAddressItem := range submitterAddress {
		submitterAddressRule = append(submitterAddressRule, submitterAddressItem)
	}

	logs, sub, err := _Tokenconversion.contract.WatchLogs(opts, "OnSubmitBurnProof", submitterAddressRule)
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

// ParseOnSubmitBurnProof is a log parse operation binding the contract event 0x2662c9d425c9a0213bcf36501662bad353f18b695745ced3aba5567c363551e2.
//
// Solidity: event OnSubmitBurnProof(address indexed submitterAddress, uint256 index)
func (_Tokenconversion *TokenconversionFilterer) ParseOnSubmitBurnProof(log types.Log) (*TokenconversionOnSubmitBurnProof, error) {
	event := new(TokenconversionOnSubmitBurnProof)
	if err := _Tokenconversion.contract.UnpackLog(event, "OnSubmitBurnProof", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
