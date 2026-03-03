//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/google/uuid"
	circlwasm "github.com/quantumcoinproject/circl/sign/wasm"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereds" // hybrid PQC (NIST) helper for compact signatures
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	abi "github.com/quantumcoinproject/quantum-coin-go/wasm/accounts/abi"
	ks "github.com/quantumcoinproject/quantum-coin-go/wasm/accounts/keystore"
	wasm "github.com/quantumcoinproject/quantum-coin-go/wasm/core/types"
	"golang.org/x/crypto/scrypt"
)

type Transaction struct {
	Transaction []TransactionDetails `json:"transaction"`
}

type TransactionDetails struct {
	FromAddress common.Address `json:"fromAddress"`
	ToAddress   common.Address `json:"toAddress"`
	Nonce       uint64         `json:"nonce"`
	GasLimit    uint64         `json:"gasLimit"`
	Value       *big.Int       `json:"value"`
	Data        []byte         `json:"data"`
	ChainId     *big.Int       `json:"chainId"`
}

type Transaction2 struct {
	Transaction []TransactionDetails2 `json:"transaction"`
}

type TransactionDetails2 struct {
	FromAddress common.Address  `json:"fromAddress"`
	ToAddress   *common.Address `json:"toAddress"`
	Nonce       uint64          `json:"nonce"`
	GasLimit    uint64          `json:"gasLimit"`
	Value       *big.Int        `json:"value"`
	Data        []byte          `json:"data"`
	ChainId     *big.Int        `json:"chainId"`
	Remarks     []byte          `json:"remarks"`
}

func main() {
	done := make(chan struct{}, 0)
	js.Global().Set("Scrypt", js.FuncOf(Scrypt))
	js.Global().Set("PublicKeyToAddress", js.FuncOf(PublicKeyToAddress))
	js.Global().Set("TxnSigningHash", js.FuncOf(TxnSigningHash))
	js.Global().Set("TxnHash", js.FuncOf(TxnHash))
	js.Global().Set("TxnData", js.FuncOf(TxnData))
	js.Global().Set("ContractData", js.FuncOf(ContractData))
	js.Global().Set("TokenTransfer", js.FuncOf(TokenTransfer))
	js.Global().Set("KeyPairToWalletJson", js.FuncOf(KeyPairToWalletJson))
	js.Global().Set("JsonToWalletKeyPair", js.FuncOf(JsonToWalletKeyPair))
	js.Global().Set("ParseBigFloat", js.FuncOf(ParseBigFloat))
	js.Global().Set("IsValidAddress", js.FuncOf(IsValidAddress))
	js.Global().Set("SingleAddressArgumentMethod", js.FuncOf(SingleAddressArgumentMethod))
	js.Global().Set("SingleAmountArgumentMethod", js.FuncOf(SingleAmountArgumentMethod))
	js.Global().Set("NoArgumentMethod", js.FuncOf(NoArgumentMethod))
	js.Global().Set("PublicKeyFromSignature", js.FuncOf(PublicKeyFromSignature))
	js.Global().Set("PublicKeyFromPrivateKey", js.FuncOf(PublicKeyFromPrivateKey))
	js.Global().Set("CombinePublicKeySignature", js.FuncOf(CombinePublicKeySignature))
	js.Global().Set("PackMethodData", js.FuncOf(PackMethodDataWrapper))
	js.Global().Set("UnpackMethodData", js.FuncOf(UnpackMethodDataWrapper))
	js.Global().Set("PackCreateContractData", js.FuncOf(PackCreateContractDataWrapper))
	js.Global().Set("CreateAddress", js.FuncOf(CreateAddressWrapper))
	js.Global().Set("CreateAddress2", js.FuncOf(CreateAddress2Wrapper))
	js.Global().Set("EncodeEventLog", js.FuncOf(EncodeEventLogWrapper))
	js.Global().Set("DecodeEventLog", js.FuncOf(DecodeEventLogWrapper))
	js.Global().Set("EncodeRlp", js.FuncOf(EncodeRlpWrapper))
	js.Global().Set("DecodeRlp", js.FuncOf(DecodeRlpWrapper))
	js.Global().Set("TxnSigningHash2", js.FuncOf(TxnSigningHash2))
	js.Global().Set("TxnHash2", js.FuncOf(TxnHash2))
	js.Global().Set("TxnData2", js.FuncOf(TxnData2))
	circlwasm.Register()
	<-done
}

// PackMethodDataWrapper is a wrapper function for PackMethodData to be used with js.FuncOf
func PackMethodDataWrapper(this js.Value, args []js.Value) interface{} {
	// Require at least 2 arguments: abiJSON and methodName
	// Method arguments are optional (can be 0 or more)
	if len(args) < 2 {
		return js.Global().Get("Error").New("PackMethodData: insufficient arguments, expected at least 2 (abiJSON and methodName)")
	}
	abiJSON := args[0].String()
	methodName := args[1].String()
	methodArgs := args[2:] // This will be an empty slice for zero-argument methods
	return PackMethodData(abiJSON, methodName, methodArgs)
}

// UnpackMethodDataWrapper is a wrapper function for UnpackMethodData to be used with js.FuncOf
func UnpackMethodDataWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) != 3 {
		return js.Global().Get("Error").New("UnpackMethodData: invalid number of arguments, expected 3")
	}
	abiJSON := args[0].String()
	methodName := args[1].String()
	hexData := args[2]
	return UnpackMethodData(abiJSON, methodName, hexData)
}

// PackCreateContractDataWrapper is a wrapper function for PackCreateContractData to be used with js.FuncOf
func PackCreateContractDataWrapper(this js.Value, args []js.Value) interface{} {
	// Require at least 2 arguments: abiJSON and bytecode
	// Constructor arguments are optional (can be 0 or more)
	if len(args) < 2 {
		return js.Global().Get("Error").New("PackCreateContractData: insufficient arguments, expected at least 2 (abiJSON and bytecode)")
	}
	abiJSON := args[0].String()
	bytecode := args[1].String()
	constructorArgs := args[2:] // This will be an empty slice for parameterless constructors
	return PackCreateContractData(abiJSON, bytecode, constructorArgs)
}

func Scrypt(this js.Value, args []js.Value) interface{} {
	secret := args[0].String()

	salt, err := base64.StdEncoding.DecodeString(args[1].String())
	if err != nil {
		return nil
	}

	derivedKey, err := scrypt.Key([]byte(secret), salt, 262144, 8, 1, 32)
	if err != nil {
		return nil
	}

	return base64.StdEncoding.EncodeToString(derivedKey)
}

func PublicKeyToAddress(this js.Value, args []js.Value) interface{} {
	pubData := js.Global().Get("Uint8Array").New(args[0])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)
	return common.BytesToAddress(crypto.Keccak256(pubBytes[:])[common.AddressTruncateBytes:]).String()
}

func IsValidAddress(this js.Value, args []js.Value) interface{} {
	address := args[0].String()
	return common.IsHexAddress(address)
}

func TxnSigningHash(this js.Value, args []js.Value) interface{} {
	ts, err := transactionData(args)
	if err != nil {
		return nil
	}

	tx := wasm.NewDefaultFeeTransaction(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	signerHash, err := signer.Hash(tx)
	if err != nil {
		return nil
	}

	var message strings.Builder
	for i := 0; i < len(signerHash); i++ {
		sh := signerHash[i]
		message.WriteString(string(sh))
	}
	return message.String()
}

func TxnSigningHash2(this js.Value, args []js.Value) interface{} {
	ts, err := transactionData2(args)
	if err != nil {
		return nil
	}

	tx := wasm.NewDefaultFeeTransactionExtended(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data, ts.Transaction[0].Remarks)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	signerHash, err := signer.Hash(tx)
	if err != nil {
		return nil
	}

	var message strings.Builder
	for i := 0; i < len(signerHash); i++ {
		sh := signerHash[i]
		message.WriteString(string(sh))
	}
	return message.String()
}

func TxnHash(this js.Value, args []js.Value) interface{} {
	ts, err := transactionData(args)
	if err != nil {
		return nil
	}

	tx := wasm.NewDefaultFeeTransaction(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	pubData := js.Global().Get("Uint8Array").New(args[7])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)

	sigData := js.Global().Get("Uint8Array").New(args[8])
	sigBytes := make([]byte, sigData.Get("length").Int())
	js.CopyBytesToGo(sigBytes, sigData)

	signTx, err := signTxHash(tx, signer, pubBytes, sigBytes)
	if err != nil {
		return nil
	}

	return signTx.Hash().String()
}

func TxnHash2(this js.Value, args []js.Value) interface{} {
	ts, err := transactionData2(args)
	if err != nil {
		return nil
	}

	tx := wasm.NewDefaultFeeTransactionExtended(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data, ts.Transaction[0].Remarks)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	pubData := js.Global().Get("Uint8Array").New(args[8])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)

	sigData := js.Global().Get("Uint8Array").New(args[9])
	sigBytes := make([]byte, sigData.Get("length").Int())
	js.CopyBytesToGo(sigBytes, sigData)

	signTx, err := signTxHash(tx, signer, pubBytes, sigBytes)
	if err != nil {
		return nil
	}

	return signTx.Hash().String()
}

func TxnData(this js.Value, args []js.Value) interface{} {
	ts, err := transactionData(args)
	if err != nil {
		return nil
	}

	tx := wasm.NewDefaultFeeTransaction(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	pubData := js.Global().Get("Uint8Array").New(args[7])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)

	sigData := js.Global().Get("Uint8Array").New(args[8])
	sigBytes := make([]byte, sigData.Get("length").Int())
	js.CopyBytesToGo(sigBytes, sigData)

	signTx, err := signTxHash(tx, signer, pubBytes, sigBytes)
	if err != nil {
		return nil
	}

	signTxBinary, err := signTx.MarshalBinary()
	if err != nil {
		return nil
	}

	signTxEncode := hexutil.Encode(signTxBinary)

	return signTxEncode
}

func TxnData2(this js.Value, args []js.Value) interface{} {
	ts, err := transactionData2(args)
	if err != nil {
		return nil
	}

	tx := wasm.NewDefaultFeeTransactionExtended(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data, ts.Transaction[0].Remarks)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	pubData := js.Global().Get("Uint8Array").New(args[8])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)

	sigData := js.Global().Get("Uint8Array").New(args[9])
	sigBytes := make([]byte, sigData.Get("length").Int())
	js.CopyBytesToGo(sigBytes, sigData)

	signTx, err := signTxHash(tx, signer, pubBytes, sigBytes)
	if err != nil {
		return nil
	}

	signTxBinary, err := signTx.MarshalBinary()
	if err != nil {
		return nil
	}

	signTxEncode := hexutil.Encode(signTxBinary)

	return signTxEncode
}

func ContractData(this js.Value, args []js.Value) interface{} {
	method := args[0].String()

	abiData, err := abi.JSON(strings.NewReader((args[1].String())))

	if err != nil {
		return nil
	}

	arguments := make([]interface{}, 0, len(args)-2)
	for _, i := range args[2:] {
		arguments = append(arguments, i.String())
	}

	data, err := abiData.Pack(method, arguments...)
	if err != nil {
		return nil
	}

	var d strings.Builder
	for i := 0; i < len(data); i++ {
		sh := data[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

func TokenTransfer(this js.Value, args []js.Value) interface{} {
	method := args[0].String()

	abiData, err := abi.JSON(strings.NewReader((args[1].String())))
	if err != nil {
		return nil
	}

	var ethVal *big.Float
	var weiVal *big.Int
	ethVal, err = ParseBigFloatInner(args[3].String())
	if err != nil {
		return nil
	}
	weiVal = etherToWeiFloat(ethVal)

	arguments := make([]interface{}, 0, 2)
	arguments = append(arguments, common.HexToAddress(args[2].String()))
	arguments = append(arguments, weiVal)

	data, err := abiData.Pack(method, arguments...)
	if err != nil {
		return nil
	}

	var d strings.Builder
	for i := 0; i < len(data); i++ {
		sh := data[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

func KeyPairToWalletJson(this js.Value, args []js.Value) interface{} {
	privData := js.Global().Get("Uint8Array").New(args[0])
	privBytes := make([]byte, privData.Get("length").Int())
	js.CopyBytesToGo(privBytes, privData)

	pubData := js.Global().Get("Uint8Array").New(args[1])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)

	passphrase := args[2].String()

	var pubKeyAddress = crypto.PublicKeyBytesToAddress(pubBytes)

	id, err := uuid.NewRandom()
	if err != nil {
		panic(fmt.Sprintf("Could not create random uuid: %v", err))
	}

	publicKey := ks.PublicKey{
		PubData: pubBytes,
	}

	privateKey := &ks.PrivateKey{
		PublicKey: publicKey,
		PriData:   privBytes,
	}

	key := &ks.Key{
		Id:         id,
		Address:    pubKeyAddress,
		PrivateKey: privateKey,
	}

	keyJson, err := ks.EncryptKey(key, pubKeyAddress.Bytes(), passphrase, ks.StandardScryptN, ks.StandardScryptP)
	if err != nil {
		return nil
	}

	return string(keyJson[:])
}

func JsonToWalletKeyPair(this js.Value, args []js.Value) interface{} {
	keyJson := []byte(args[0].String())
	passphrase := args[1].String()

	key, err := ks.DecryptKey(keyJson, passphrase)
	if err != nil {
		return nil
	}
	return base64.StdEncoding.EncodeToString(key.PrivateKey.PriData) + "," + base64.StdEncoding.EncodeToString(key.PrivateKey.PubData)
}

// ParseBigFloat parse string value to big.Float
func ParseBigFloat(this js.Value, args []js.Value) interface{} {
	var value string
	value = args[0].String()
	f := new(big.Float)
	f.SetPrec(236)
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(value, f)
	if err != nil {
		return nil
	}
	return f.String()
}

func ParseBigFloatInner(value string) (*big.Float, error) {
	f := new(big.Float)
	f.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(value, f)
	return f, err
}

func transactionData(args []js.Value) (transaction Transaction, err error) {
	fromAddress := common.HexToAddress(args[0].String())

	var nonceString string
	var nonceUint64 uint64
	fmt.Sscan(args[1].String(), &nonceString, &nonceUint64)
	nonce := nonceUint64

	toAddress := common.HexToAddress(args[2].String())

	var ethVal *big.Float
	var weiVal *big.Int
	ethVal, err = ParseBigFloatInner(args[3].String())
	if err != nil {
		return Transaction{}, err
	}
	weiVal = etherToWeiFloat(ethVal)

	var gasString string
	var gasUint64 uint64
	fmt.Sscan(args[4].String(), &gasString, &gasUint64)
	gasLimit := gasUint64

	var chainIdString string
	var chainIdInt64 int64
	fmt.Sscan(args[5].String(), &chainIdString, &chainIdInt64)
	chainId := big.NewInt(chainIdInt64)

	dataString := js.Global().Get("Uint8Array").New(args[6])
	data := make([]byte, dataString.Get("length").Int())
	js.CopyBytesToGo(data, dataString)

	transactionDetails := TransactionDetails{
		FromAddress: fromAddress, ToAddress: toAddress, Nonce: nonce, GasLimit: gasLimit,
		Value: weiVal, Data: data, ChainId: chainId}

	var t Transaction
	t.Transaction = append(t.Transaction, transactionDetails)

	return t, nil
}

func transactionData2(args []js.Value) (transaction Transaction2, err error) {
	fromAddress := common.HexToAddress(args[0].String())

	var nonceString string
	var nonceUint64 uint64
	fmt.Sscan(args[1].String(), &nonceString, &nonceUint64)
	nonce := nonceUint64

	var toAddress *common.Address

	if args[2].IsNull() || args[2].IsUndefined() {
		toAddress = nil
	} else {
		if common.IsHexAddress(args[2].String()) == false {
			return Transaction2{}, errors.New("invalid address")
		}
		toAddressTemp := common.HexToAddress(args[2].String())
		toAddress = &toAddressTemp
	}

	var weiVal *big.Int
	if args[3].IsNull() == false && args[3].IsUndefined() == false {
		weiVal, err = hexutil.DecodeBig(args[3].String())
		if err != nil {
			return Transaction2{}, err
		}
	} else {
		weiVal = big.NewInt(0)
	}

	var gasString string
	var gasUint64 uint64
	fmt.Sscan(args[4].String(), &gasString, &gasUint64)
	gasLimit := gasUint64

	var chainIdString string
	var chainIdInt64 int64
	fmt.Sscan(args[5].String(), &chainIdString, &chainIdInt64)
	chainId := big.NewInt(chainIdInt64)

	var data []byte
	if args[6].IsNull() == false && args[6].IsUndefined() == false {
		dataString := js.Global().Get("Uint8Array").New(args[6])
		data = make([]byte, dataString.Get("length").Int())
		js.CopyBytesToGo(data, dataString)
	} else {
		data = nil
	}

	var remarks []byte
	if args[7].IsNull() == false && args[7].IsUndefined() == false {
		remarksString := js.Global().Get("Uint8Array").New(args[7])
		remarks = make([]byte, remarksString.Get("length").Int())
		js.CopyBytesToGo(remarks, remarksString)
	} else {
		remarks = nil
	}

	transactionDetails := TransactionDetails2{
		FromAddress: fromAddress, ToAddress: toAddress, Nonce: nonce, GasLimit: gasLimit,
		Value: weiVal, Data: data, ChainId: chainId, Remarks: remarks}

	var t Transaction2
	t.Transaction = append(t.Transaction, transactionDetails)

	return t, nil
}

func signTxHash(tx *wasm.Transaction, signer wasm.Signer, pubBytes, sigBytes []byte) (*wasm.Transaction, error) {
	sig := common.CombineTwoParts(sigBytes, pubBytes)
	return tx.WithSignature(signer, sig)
}

func weiToEther(val *big.Int) *big.Int {
	return new(big.Int).Div(val, big.NewInt(params.Ether))
}

func etherToWeiFloat(eth *big.Float) *big.Int {
	truncInt, _ := eth.Int(nil)
	truncInt = new(big.Int).Mul(truncInt, big.NewInt(params.Ether))
	fracStr := strings.Split(fmt.Sprintf("%.18f", eth), ".")[1]
	fracStr += strings.Repeat("0", 18-len(fracStr))
	fracInt, _ := new(big.Int).SetString(fracStr, 10)
	wei := new(big.Int).Add(truncInt, fracInt)
	return wei
}

func SingleAddressArgumentMethod(this js.Value, args []js.Value) interface{} {
	if len(args) != 3 {
		return nil
	}
	method := args[0].String()

	abiData, err := abi.JSON(strings.NewReader((args[1].String())))
	if err != nil {
		return nil
	}

	arguments := make([]interface{}, 0)
	arguments = append(arguments, common.HexToAddress(args[2].String()))

	data, err := abiData.Pack(method, arguments...)
	if err != nil {
		return nil
	}

	var d strings.Builder
	for i := 0; i < len(data); i++ {
		sh := data[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

func SingleAmountArgumentMethod(this js.Value, args []js.Value) interface{} {
	method := args[0].String()

	abiData, err := abi.JSON(strings.NewReader((args[1].String())))
	if err != nil {
		return nil
	}

	var ethVal *big.Float
	var weiVal *big.Int
	ethVal, err = ParseBigFloatInner(args[2].String())
	if err != nil {
		return nil
	}
	weiVal = etherToWeiFloat(ethVal)

	arguments := make([]interface{}, 0, 1)
	arguments = append(arguments, weiVal)

	data, err := abiData.Pack(method, arguments...)
	if err != nil {
		return nil
	}

	var d strings.Builder
	for i := 0; i < len(data); i++ {
		sh := data[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

func NoArgumentMethod(this js.Value, args []js.Value) interface{} {
	method := args[0].String()

	abiData, err := abi.JSON(strings.NewReader((args[1].String())))
	if err != nil {
		return nil
	}

	data, err := abiData.Pack(method)
	if err != nil {
		return nil
	}

	var d strings.Builder
	for i := 0; i < len(data); i++ {
		sh := data[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

// PackMethodData packs a Solidity method call with the given ABI, method name, and arguments.
// It returns the transaction data as a JavaScript-compatible string that can be included in a transaction.
//
// Parameters:
//   - abiJSON: The Solidity ABI file content as a JSON string
//   - methodName: The name of the method to call
//   - args: An array of js.Value representing the parameters to pass to the method
//
// Returns:
//   - interface{}: The packed transaction data as a string, or a JavaScript Error on error
func PackMethodData(abiJSON string, methodName string, args []js.Value) interface{} {
	var err error

	// Validate abiJSON is not empty
	if strings.TrimSpace(abiJSON) == "" {
		return js.Global().Get("Error").New("PackMethodData: abiJSON cannot be empty")
	}

	// Validate method name is not empty
	if strings.TrimSpace(methodName) == "" {
		return js.Global().Get("Error").New("PackMethodData: method name cannot be empty")
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("PackMethodData: failed to parse ABI JSON: %v", err))
	}

	method, exist := abiData.Methods[methodName]
	if !exist {
		return js.Global().Get("Error").New(fmt.Sprintf("PackMethodData: method '%s' not found in ABI", methodName))
	}

	if len(args) != len(method.Inputs) {
		return js.Global().Get("Error").New(fmt.Sprintf("PackMethodData: argument count mismatch for method '%s', expected %d but got %d", methodName, len(method.Inputs), len(args)))
	}

	var data []byte

	// If no arguments, call Pack without arguments
	if len(args) == 0 {
		data, err = abiData.Pack(methodName)
	} else {
		// Convert js.Value arguments to Go types based on ABI types
		convertedArgs := make([]interface{}, len(args))
		for i, arg := range args {
			converted, err := convertJsValueToGoType(arg, method.Inputs[i].Type)
			if err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("PackMethodData: failed to convert argument %d for method '%s': %v", i, methodName, err))
			}
			// Convert []interface{} to proper typed slice if needed
			converted = convertToTypedSlice(converted, method.Inputs[i].Type)
			convertedArgs[i] = converted
		}
		data, err = abiData.Pack(methodName, convertedArgs...)
	}
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("PackMethodData: failed to pack method '%s': %v", methodName, err))
	}

	var d strings.Builder
	for i := 0; i < len(data); i++ {
		sh := data[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

// PackCreateContractData packs constructor data for contract creation
// Supports both parameterless constructors and constructors with parameters
// The bytecode is prepended to the constructor parameters
func PackCreateContractData(abiJSON string, bytecode string, args []js.Value) interface{} {
	var err error

	// Validate abiJSON is not empty
	if strings.TrimSpace(abiJSON) == "" {
		return js.Global().Get("Error").New("PackCreateContractData: abiJSON cannot be empty")
	}

	// Validate bytecode is not empty
	if strings.TrimSpace(bytecode) == "" {
		return js.Global().Get("Error").New("PackCreateContractData: bytecode cannot be empty")
	}

	// Decode bytecode from hex string
	bytecodeBytes, err := hexutil.Decode(bytecode)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("PackCreateContractData: failed to decode bytecode: %v", err))
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("PackCreateContractData: failed to parse ABI JSON: %v", err))
	}

	// Check if constructor exists in ABI
	// If no constructor is defined, Constructor.Inputs will be empty
	constructorInputs := abiData.Constructor.Inputs

	// Validate argument count matches constructor inputs
	if len(args) != len(constructorInputs) {
		return js.Global().Get("Error").New(fmt.Sprintf("PackCreateContractData: argument count mismatch for constructor, expected %d but got %d", len(constructorInputs), len(args)))
	}

	var constructorData []byte

	// If no arguments (parameterless constructor), call Pack with empty string and no arguments
	if len(args) == 0 {
		constructorData, err = abiData.Pack("")
	} else {
		// Convert js.Value arguments to Go types based on constructor input types
		convertedArgs := make([]interface{}, len(args))
		for i, arg := range args {
			converted, err := convertJsValueToGoType(arg, constructorInputs[i].Type)
			if err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("PackCreateContractData: failed to convert argument %d for constructor: %v", i, err))
			}
			// Convert []interface{} to proper typed slice if needed
			converted = convertToTypedSlice(converted, constructorInputs[i].Type)
			convertedArgs[i] = converted
		}
		// Use empty string "" as method name for constructor
		constructorData, err = abiData.Pack("", convertedArgs...)
	}
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("PackCreateContractData: failed to pack constructor: %v", err))
	}

	// Combine bytecode with constructor data: bytecode + constructor parameters
	// This is the format required for contract creation transactions
	finalData := append(bytecodeBytes, constructorData...)

	var d strings.Builder
	for i := 0; i < len(finalData); i++ {
		sh := finalData[i]
		d.WriteString(string(sh))
	}

	return d.String()
}

// UnpackMethodData unpacks the return values of a Solidity method call.
// It returns the unpacked values as a JSON string that can be parsed in JavaScript.
//
// Parameters:
//   - abiJSON: The Solidity ABI file content as a JSON string
//   - methodName: The name of the method whose return values to unpack
//   - hexData: The hex-encoded return data as a js.Value (string, with or without 0x prefix)
//
// Returns:
//   - interface{}: The unpacked return values as a JSON string, or a JavaScript Error on error
func UnpackMethodData(abiJSON string, methodName string, hexData js.Value) interface{} {
	// Validate abiJSON is not empty
	if strings.TrimSpace(abiJSON) == "" {
		return js.Global().Get("Error").New("UnpackMethodData: abiJSON cannot be empty")
	}

	// Validate method name is not empty
	if strings.TrimSpace(methodName) == "" {
		return js.Global().Get("Error").New("UnpackMethodData: method name cannot be empty")
	}

	// Validate hexData is not null/undefined
	if hexData.IsNull() || hexData.IsUndefined() {
		return js.Global().Get("Error").New("UnpackMethodData: hexData cannot be null or undefined")
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("UnpackMethodData: failed to parse ABI JSON: %v", err))
	}

	method, exist := abiData.Methods[methodName]
	if !exist {
		return js.Global().Get("Error").New(fmt.Sprintf("UnpackMethodData: method '%s' not found in ABI", methodName))
	}

	// Get hex string from js.Value and decode to bytes
	hexStr := hexData.String()
	if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
		hexStr = "0x" + hexStr
	}
	data, err := hexutil.Decode(hexStr)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("UnpackMethodData: failed to decode hex data: %v", err))
	}

	// Unpack the return values using method.Outputs (not Inputs)
	// Return values don't include method ID, just the raw return data
	unpacked, err := method.Outputs.Unpack(data)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("UnpackMethodData: failed to unpack return data for method '%s': %v", methodName, err))
	}

	// Convert unpacked values to JavaScript-compatible format
	jsValues := make([]interface{}, len(unpacked))
	for i, val := range unpacked {
		jsValues[i] = convertGoTypeToJsValue(val)
	}

	// Return as JSON string
	jsonData, err := json.Marshal(jsValues)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("UnpackMethodData: failed to marshal unpacked data to JSON: %v", err))
	}

	return string(jsonData)
}

// convertGoTypeToJsValue converts a Go type to a JavaScript-compatible value
func convertGoTypeToJsValue(val interface{}) interface{} {
	switch v := val.(type) {
	case common.Address:
		// Convert address to hex string
		return v.String()
	case *big.Int:
		// Convert big.Int to number string
		return v.String()
	case []byte:
		// Convert bytes to hex string
		return hexutil.Encode(v)
	case bool:
		return v
	case string:
		return v
	case []*big.Int:
		// Convert []*big.Int to []string (hex strings)
		// Handle typed slices first before []interface{} to avoid ambiguity
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValue(elem)
		}
		return result
	case []common.Address:
		// Convert []common.Address to []string (hex strings)
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValue(elem)
		}
		return result
	case []bool:
		// Convert []bool to []bool
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValue(elem)
		}
		return result
	case []string:
		// Convert []string to []string
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValue(elem)
		}
		return result
	case [][]byte:
		// Convert [][]byte to []string (hex strings)
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValue(elem)
		}
		return result
	case []interface{}:
		// Recursively convert array elements (for tuples, nested arrays, etc.)
		// This should come after specific typed slices to handle generic cases
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValue(elem)
		}
		return result
	default:
		// For unknown types, try to convert to string
		return fmt.Sprintf("%v", v)
	}
}

// convertToTypedSlice converts []interface{} to the proper typed slice required by abi.Pack
func convertToTypedSlice(val interface{}, abiType abi.Type) interface{} {
	// Only convert if it's a slice/array type and val is []interface{}
	if abiType.T != abi.SliceTy && abiType.T != abi.ArrayTy {
		return val
	}

	sliceVal, ok := val.([]interface{})
	if !ok {
		return val
	}

	// Convert based on element type
	switch abiType.Elem.T {
	case abi.AddressTy:
		// Convert []interface{} to []common.Address
		result := make([]common.Address, len(sliceVal))
		for i, v := range sliceVal {
			if addr, ok := v.(common.Address); ok {
				result[i] = addr
			}
		}
		return result

	case abi.UintTy, abi.IntTy:
		// Convert []interface{} to []*big.Int
		result := make([]*big.Int, len(sliceVal))
		for i, v := range sliceVal {
			if bigVal, ok := v.(*big.Int); ok {
				result[i] = bigVal
			}
		}
		return result

	case abi.BoolTy:
		// Convert []interface{} to []bool
		result := make([]bool, len(sliceVal))
		for i, v := range sliceVal {
			if boolVal, ok := v.(bool); ok {
				result[i] = boolVal
			}
		}
		return result

	case abi.StringTy:
		// Convert []interface{} to []string
		result := make([]string, len(sliceVal))
		for i, v := range sliceVal {
			if strVal, ok := v.(string); ok {
				result[i] = strVal
			}
		}
		return result

	case abi.BytesTy:
		// Convert []interface{} to [][]byte
		result := make([][]byte, len(sliceVal))
		for i, v := range sliceVal {
			if bytesVal, ok := v.([]byte); ok {
				result[i] = bytesVal
			}
		}
		return result

	default:
		// For other types, return as-is (might need nested conversion for nested arrays)
		return val
	}
}

// convertJsValueToGoType converts a js.Value to the appropriate Go type based on the ABI Type
func convertJsValueToGoType(jsVal js.Value, abiType abi.Type) (interface{}, error) {
	typeStr := abiType.String()

	switch abiType.T {
	case abi.AddressTy:
		// Address type: convert from hex string
		addrStr := jsVal.String()
		if !common.IsHexAddress(addrStr) {
			return nil, fmt.Errorf("invalid address: %s", addrStr)
		}
		return common.HexToAddress(addrStr), nil

	case abi.UintTy:
		// Unsigned integer types (uint8, uint256, etc.)
		// Convert from number or BigInt to big.Int
		var val *big.Int
		if jsVal.Type() == js.TypeNumber {
			// Regular JavaScript number - convert via float to preserve precision
			numVal := jsVal.Float()
			bigFloat := new(big.Float).SetFloat64(numVal)
			val, _ = bigFloat.Int(nil)
		} else {
			// Try to parse as string (could be BigInt string or number string)
			strVal := jsVal.String()
			val = new(big.Int)
			_, ok := val.SetString(strVal, 10)
			if !ok {
				return nil, fmt.Errorf("invalid uint value: %s", strVal)
			}
		}
		return val, nil

	case abi.IntTy:
		// Signed integer types (int8, int256, etc.)
		// Convert from number or BigInt to big.Int
		var val *big.Int
		if jsVal.Type() == js.TypeNumber {
			// Regular JavaScript number - convert via float to preserve precision
			numVal := jsVal.Float()
			bigFloat := new(big.Float).SetFloat64(numVal)
			val, _ = bigFloat.Int(nil)
		} else {
			// Try to parse as string (could be BigInt string or number string)
			strVal := jsVal.String()
			val = new(big.Int)
			_, ok := val.SetString(strVal, 10)
			if !ok {
				return nil, fmt.Errorf("invalid int value: %s", strVal)
			}
		}
		return val, nil

	case abi.BoolTy:
		// Boolean type
		return jsVal.Bool(), nil

	case abi.StringTy:
		// String type
		return jsVal.String(), nil

	case abi.BytesTy:
		// Dynamic bytes type
		// Check if it's a Uint8Array
		if jsVal.Type() == js.TypeObject {
			uint8Array := js.Global().Get("Uint8Array")
			if !uint8Array.IsUndefined() && jsVal.InstanceOf(uint8Array) {
				bytes := make([]byte, jsVal.Get("length").Int())
				js.CopyBytesToGo(bytes, jsVal)
				return bytes, nil
			}
		}
		// Fallback: treat as hex string
		hexStr := jsVal.String()
		if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
			hexStr = "0x" + hexStr
		}
		return hexutil.Decode(hexStr)

	case abi.FixedBytesTy:
		// Fixed-size bytes type (bytes1, bytes32, etc.)
		// Check if it's a Uint8Array
		if jsVal.Type() == js.TypeObject {
			uint8Array := js.Global().Get("Uint8Array")
			if !uint8Array.IsUndefined() && jsVal.InstanceOf(uint8Array) {
				bytes := make([]byte, abiType.Size)
				js.CopyBytesToGo(bytes, jsVal)
				return bytes, nil
			}
		}
		// Fallback: treat as hex string
		hexStr := jsVal.String()
		if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
			hexStr = "0x" + hexStr
		}
		decoded, err := hexutil.Decode(hexStr)
		if err != nil {
			return nil, err
		}
		if len(decoded) != abiType.Size {
			return nil, fmt.Errorf("fixed bytes size mismatch: expected %d, got %d", abiType.Size, len(decoded))
		}
		return decoded, nil

	case abi.SliceTy:
		// Dynamic array type (uint256[], address[], etc.)
		// Check if it's a JavaScript array
		if jsVal.Type() == js.TypeObject && jsVal.Get("length").Type() == js.TypeNumber {
			length := jsVal.Get("length").Int()
			result := make([]interface{}, length)
			for i := 0; i < length; i++ {
				elem, err := convertJsValueToGoType(jsVal.Index(i), *abiType.Elem)
				if err != nil {
					return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
				}
				result[i] = elem
			}
			return result, nil
		}
		return nil, fmt.Errorf("expected array for slice type %s", typeStr)

	case abi.ArrayTy:
		// Fixed-size array type (uint256[5], address[10], etc.)
		// Check if it's a JavaScript array
		if jsVal.Type() == js.TypeObject && jsVal.Get("length").Type() == js.TypeNumber {
			length := jsVal.Get("length").Int()
			if length != abiType.Size {
				return nil, fmt.Errorf("array size mismatch: expected %d, got %d", abiType.Size, length)
			}
			result := make([]interface{}, length)
			for i := 0; i < length; i++ {
				elem, err := convertJsValueToGoType(jsVal.Index(i), *abiType.Elem)
				if err != nil {
					return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
				}
				result[i] = elem
			}
			return result, nil
		}
		return nil, fmt.Errorf("expected array for fixed array type %s", typeStr)

	case abi.TupleTy:
		// Tuple type - complex struct-like types
		// This is more complex and would require parsing the tuple structure
		// For now, we'll try to handle it as a JavaScript object
		if jsVal.Type() == js.TypeObject {
			result := make([]interface{}, len(abiType.TupleElems))
			for i, elemType := range abiType.TupleElems {
				// Try to get the field by index or by name
				var fieldVal js.Value
				if jsVal.Get("length").Type() == js.TypeNumber {
					// Array-like access
					fieldVal = jsVal.Index(i)
				} else {
					// Object-like access by name
					fieldName := abiType.TupleRawNames[i]
					fieldVal = jsVal.Get(fieldName)
					if fieldVal.IsUndefined() {
						// Try camelCase
						fieldName = abi.ToCamelCase(fieldName)
						fieldVal = jsVal.Get(fieldName)
					}
				}
				if fieldVal.IsUndefined() {
					return nil, fmt.Errorf("missing tuple field %d (%s)", i, abiType.TupleRawNames[i])
				}
				elem, err := convertJsValueToGoType(fieldVal, *elemType)
				if err != nil {
					return nil, fmt.Errorf("failed to convert tuple field %d: %w", i, err)
				}
				result[i] = elem
			}
			return result, nil
		}
		return nil, fmt.Errorf("expected object for tuple type %s", typeStr)

	default:
		// Fallback: try to convert as string for unknown types
		return jsVal.String(), nil
	}
}

func PublicKeyFromSignature(this js.Value, args []js.Value) interface{} {
	if len(args) != 2 {
		return nil
	}
	digestData := js.Global().Get("Uint8Array").New(args[0])
	digestBytes := make([]byte, digestData.Get("length").Int())
	js.CopyBytesToGo(digestBytes, digestData)

	sigData := js.Global().Get("Uint8Array").New(args[1])
	sigBytes := make([]byte, sigData.Get("length").Int())
	js.CopyBytesToGo(sigBytes, sigData)

	publicKeyBytes, err := pqchelpereds.PublicKeyBytesFromSignatureCompact(digestBytes, sigBytes)
	if err != nil {
		return nil
	}

	return common.Bytes2Hex(publicKeyBytes)
}

func PublicKeyFromPrivateKey(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 {
		return nil
	}

	compositePrivateKeyData := js.Global().Get("Uint8Array").New(args[0])
	compositePrivateKeyBytes := make([]byte, compositePrivateKeyData.Get("length").Int())
	js.CopyBytesToGo(compositePrivateKeyBytes, compositePrivateKeyData)

	_, publicKeyBytes, err := pqchelpereds.PrivateAndPublicFromPrivateKey(compositePrivateKeyBytes)
	if err != nil {
		return nil
	}

	return common.Bytes2Hex(publicKeyBytes)
}

func CombinePublicKeySignature(this js.Value, args []js.Value) interface{} {
	if len(args) != 2 {
		return nil
	}

	pubData := js.Global().Get("Uint8Array").New(args[0])
	pubBytes := make([]byte, pubData.Get("length").Int())
	js.CopyBytesToGo(pubBytes, pubData)

	sigData := js.Global().Get("Uint8Array").New(args[1])
	sigBytes := make([]byte, sigData.Get("length").Int())
	js.CopyBytesToGo(sigBytes, sigData)

	combinedSignatureBytes, err := pqchelpereds.CombinePublicKeySignature(sigBytes, pubBytes)
	if err != nil {
		return nil
	}

	return common.Bytes2Hex(combinedSignatureBytes)
}

// CreateAddressWrapper is a wrapper function for CreateAddress to be used with js.FuncOf
func CreateAddressWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) != 2 {
		return js.Global().Get("Error").New("CreateAddress: invalid number of arguments, expected 2 (address and nonce)")
	}

	addressStr := args[0].String()
	if !common.IsHexAddress(addressStr) {
		return js.Global().Get("Error").New("CreateAddress: invalid address format")
	}

	address := common.HexToAddress(addressStr)

	nonceStr := args[1].String()
	var nonce uint64
	_, err := fmt.Sscanf(nonceStr, "%d", &nonce)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("CreateAddress: failed to parse nonce: %v", err))
	}

	result := crypto.CreateAddress(address, nonce)
	return result.String()
}

// CreateAddress2Wrapper is a wrapper function for CreateAddress2 to be used with js.FuncOf
func CreateAddress2Wrapper(this js.Value, args []js.Value) interface{} {
	if len(args) != 3 {
		return js.Global().Get("Error").New("CreateAddress2: invalid number of arguments, expected 3 (address, salt, and initHash)")
	}

	addressStr := args[0].String()
	if !common.IsHexAddress(addressStr) {
		return js.Global().Get("Error").New("CreateAddress2: invalid address format")
	}

	address := common.HexToAddress(addressStr)

	// Decode salt from hex string
	saltStr := args[1].String()
	saltBytes, err := hexutil.Decode(saltStr)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("CreateAddress2: failed to decode salt: %v", err))
	}

	if len(saltBytes) != common.HashLength {
		return js.Global().Get("Error").New(fmt.Sprintf("CreateAddress2: salt must be %d bytes, got %d", common.HashLength, len(saltBytes)))
	}

	var salt [common.HashLength]byte
	copy(salt[:], saltBytes)

	// Decode initHash from hex string
	initHashStr := args[2].String()
	initHash, err := hexutil.Decode(initHashStr)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("CreateAddress2: failed to decode initHash: %v", err))
	}

	result := crypto.CreateAddress2(address, salt, initHash)
	return result.String()
}

// EncodeEventLogWrapper is a wrapper function for EncodeEventLog to be used with js.FuncOf
func EncodeEventLogWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return js.Global().Get("Error").New("EncodeEventLog: insufficient arguments, expected at least 2 (abiJSON and eventName)")
	}
	abiJSON := args[0].String()
	eventName := args[1].String()
	values := args[2:] // Event parameter values
	return EncodeEventLog(abiJSON, eventName, values)
}

// DecodeEventLogWrapper is a wrapper function for DecodeEventLog to be used with js.FuncOf
func DecodeEventLogWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) != 4 {
		return js.Global().Get("Error").New("DecodeEventLog: invalid number of arguments, expected 4 (abiJSON, eventName, topics, data)")
	}
	abiJSON := args[0].String()
	eventName := args[1].String()
	topics := args[2] // Array of topic strings
	data := args[3]   // Data string
	return DecodeEventLog(abiJSON, eventName, topics, data)
}

// EncodeEventLog encodes event parameters into topics and data according to the ABI specification.
// Returns an object with topics (string array) and data (string).
func EncodeEventLog(abiJSON string, eventName string, values []js.Value) interface{} {
	// Validate inputs
	if strings.TrimSpace(abiJSON) == "" {
		return js.Global().Get("Error").New("EncodeEventLog: abiJSON cannot be empty")
	}
	if strings.TrimSpace(eventName) == "" {
		return js.Global().Get("Error").New("EncodeEventLog: eventName cannot be empty")
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("EncodeEventLog: failed to parse ABI JSON: %v", err))
	}

	// Get event from ABI
	event, exist := abiData.Events[eventName]
	if !exist {
		return js.Global().Get("Error").New(fmt.Sprintf("EncodeEventLog: event '%s' not found in ABI", eventName))
	}

	// Validate argument count
	if len(values) != len(event.Inputs) {
		return js.Global().Get("Error").New(fmt.Sprintf("EncodeEventLog: argument count mismatch for event '%s', expected %d but got %d", eventName, len(event.Inputs), len(values)))
	}

	// Separate indexed and non-indexed arguments
	var indexedArgs []interface{}
	var indexedTypes []abi.Argument
	var nonIndexedArgs []interface{}
	var nonIndexedTypes []abi.Argument

	for i, input := range event.Inputs {
		converted, err := convertJsValueToGoType(values[i], input.Type)
		if err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("EncodeEventLog: failed to convert argument %d for event '%s': %v", i, eventName, err))
		}
		converted = convertToTypedSlice(converted, input.Type)

		if input.Indexed {
			indexedArgs = append(indexedArgs, converted)
			indexedTypes = append(indexedTypes, input)
		} else {
			nonIndexedArgs = append(nonIndexedArgs, converted)
			nonIndexedTypes = append(nonIndexedTypes, input)
		}
	}

	// Encode topics
	topics := make([]string, 0)

	// First topic is the event signature hash (unless anonymous)
	if !event.Anonymous {
		topics = append(topics, event.ID.Hex())
	}

	// Pack indexed parameters into topics
	for i, indexedArg := range indexedArgs {
		topic, err := packIndexedValue(indexedArg, indexedTypes[i].Type)
		if err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("EncodeEventLog: failed to pack indexed argument %d: %v", i, err))
		}
		topics = append(topics, common.BytesToHash(topic).Hex())
	}

	// Pack non-indexed parameters into data
	var dataBytes []byte
	if len(nonIndexedArgs) > 0 {
		nonIndexedInputs := abi.Arguments(nonIndexedTypes)
		dataBytes, err = nonIndexedInputs.Pack(nonIndexedArgs...)
		if err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("EncodeEventLog: failed to pack non-indexed arguments: %v", err))
		}
	}

	// Create result object
	result := js.Global().Get("Object").New()

	// Convert topics array to JavaScript array
	topicsArray := js.Global().Get("Array").New(len(topics))
	for i, topic := range topics {
		topicsArray.SetIndex(i, topic)
	}
	result.Set("topics", topicsArray)

	// Set data as hex string
	if len(dataBytes) > 0 {
		result.Set("data", hexutil.Encode(dataBytes))
	} else {
		result.Set("data", "0x")
	}

	return result
}

// DecodeEventLog decodes event log topics and data back into event parameters.
// Returns a JSON string with the decoded values.
func DecodeEventLog(abiJSON string, eventName string, topics js.Value, data js.Value) interface{} {
	// Validate inputs
	if strings.TrimSpace(abiJSON) == "" {
		return js.Global().Get("Error").New("DecodeEventLog: abiJSON cannot be empty")
	}
	if strings.TrimSpace(eventName) == "" {
		return js.Global().Get("Error").New("DecodeEventLog: eventName cannot be empty")
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: failed to parse ABI JSON: %v", err))
	}

	// Get event from ABI
	event, exist := abiData.Events[eventName]
	if !exist {
		return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: event '%s' not found in ABI", eventName))
	}

	// Parse topics array
	topicsLength := topics.Get("length").Int()
	if topicsLength == 0 {
		return js.Global().Get("Error").New("DecodeEventLog: topics array cannot be empty")
	}

	// Extract topic hashes
	topicHashes := make([]common.Hash, topicsLength)
	for i := 0; i < topicsLength; i++ {
		topicStr := topics.Index(i).String()
		if !strings.HasPrefix(topicStr, "0x") && !strings.HasPrefix(topicStr, "0X") {
			topicStr = "0x" + topicStr
		}
		topicHash := common.HexToHash(topicStr)
		topicHashes[i] = topicHash
	}

	// Verify first topic matches event ID (unless anonymous)
	if !event.Anonymous {
		if len(topicHashes) == 0 || topicHashes[0] != event.ID {
			return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: first topic does not match event signature for '%s'", eventName))
		}
	}

	// Separate indexed and non-indexed inputs
	var indexedInputs []abi.Argument
	var nonIndexedInputs []abi.Argument
	for _, input := range event.Inputs {
		if input.Indexed {
			indexedInputs = append(indexedInputs, input)
		} else {
			nonIndexedInputs = append(nonIndexedInputs, input)
		}
	}

	// Decode indexed parameters from topics
	indexedStart := 0
	if !event.Anonymous {
		indexedStart = 1 // Skip event signature topic
	}

	resultMap := make(map[string]interface{})

	// Decode indexed parameters
	if len(indexedInputs) > 0 {
		if len(topicHashes)-indexedStart < len(indexedInputs) {
			return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: insufficient topics for indexed parameters, expected %d but got %d", len(indexedInputs), len(topicHashes)-indexedStart))
		}

		for i, indexedInput := range indexedInputs {
			topicHash := topicHashes[indexedStart+i]
			decoded, err := unpackIndexedValue(topicHash, indexedInput.Type)
			if err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: failed to unpack indexed parameter '%s': %v", indexedInput.Name, err))
			}
			resultMap[indexedInput.Name] = convertGoTypeToJsValue(decoded)
		}
	}

	// Decode non-indexed parameters from data
	if len(nonIndexedInputs) > 0 {
		dataStr := data.String()
		if dataStr == "" || dataStr == "0x" {
			// Empty data, return default values
			for _, nonIndexedInput := range nonIndexedInputs {
				resultMap[nonIndexedInput.Name] = convertGoTypeToJsValue(getDefaultValue(nonIndexedInput.Type))
			}
		} else {
			if !strings.HasPrefix(dataStr, "0x") && !strings.HasPrefix(dataStr, "0X") {
				dataStr = "0x" + dataStr
			}
			dataBytes, err := hexutil.Decode(dataStr)
			if err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: failed to decode data: %v", err))
			}

			nonIndexedArgs := abi.Arguments(nonIndexedInputs)
			unpacked, err := nonIndexedArgs.Unpack(dataBytes)
			if err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: failed to unpack non-indexed parameters: %v", err))
			}

			for i, nonIndexedInput := range nonIndexedInputs {
				resultMap[nonIndexedInput.Name] = convertGoTypeToJsValue(unpacked[i])
			}
		}
	}

	// Return as JSON string
	jsonData, err := json.Marshal(resultMap)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("DecodeEventLog: failed to marshal result to JSON: %v", err))
	}

	return string(jsonData)
}

// packIndexedValue packs an indexed event parameter value into a 32-byte topic.
// For value types, it pads to 32 bytes. For dynamic types (string, bytes, arrays), it hashes with Keccak256.
func packIndexedValue(value interface{}, argType abi.Type) ([]byte, error) {
	// Create a single-argument Arguments to use its Pack method
	args := abi.Arguments{abi.Argument{Type: argType}}
	packed, err := args.Pack(value)
	if err != nil {
		return nil, err
	}

	switch argType.T {
	case abi.StringTy, abi.BytesTy, abi.SliceTy, abi.ArrayTy:
		// Dynamic types: hash with Keccak256
		return crypto.Keccak256(packed), nil
	default:
		// Value types: use first 32 bytes (should already be 32 bytes for value types)
		if len(packed) < 32 {
			padded := make([]byte, 32)
			copy(padded[32-len(packed):], packed)
			return padded, nil
		}
		return packed[:32], nil
	}
}

// unpackIndexedValue unpacks an indexed topic back into a value.
// For dynamic types, it returns the hash (cannot reconstruct original value).
func unpackIndexedValue(topic common.Hash, argType abi.Type) (interface{}, error) {
	switch argType.T {
	case abi.StringTy, abi.BytesTy, abi.SliceTy, abi.ArrayTy:
		// Dynamic types: return the hash (cannot reconstruct original)
		return topic, nil
	default:
		// Value types: unpack from topic bytes using Arguments.Unpack
		args := abi.Arguments{abi.Argument{Type: argType}}
		unpacked, err := args.Unpack(topic.Bytes())
		if err != nil {
			return nil, err
		}
		if len(unpacked) > 0 {
			return unpacked[0], nil
		}
		return nil, fmt.Errorf("failed to unpack indexed value")
	}
}

// getDefaultValue returns a default value for a given ABI type
func getDefaultValue(argType abi.Type) interface{} {
	switch argType.T {
	case abi.UintTy, abi.IntTy:
		return big.NewInt(0)
	case abi.AddressTy:
		return common.Address{}
	case abi.BoolTy:
		return false
	case abi.StringTy:
		return ""
	case abi.BytesTy:
		return []byte{}
	default:
		return nil
	}
}

// EncodeRlpWrapper is a wrapper function for EncodeRlp to be used with js.FuncOf
func EncodeRlpWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.Global().Get("Error").New("EncodeRlp: insufficient arguments, expected at least 1 (value)")
	}
	return EncodeRlp(args[0])
}

// EncodeRlp encodes a JavaScript value to RLP format.
// Supports: strings, numbers, booleans, arrays, objects (maps), and hex-encoded bytes.
// Returns a hex-encoded string of the RLP-encoded data.
func EncodeRlp(value js.Value) interface{} {
	// Convert JavaScript value to Go type
	goValue, err := convertJsValueToRlpType(value)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("EncodeRlp: failed to convert value: %v", err))
	}

	// Encode to RLP bytes
	rlpBytes, err := rlp.EncodeToBytes(goValue)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("EncodeRlp: failed to encode: %v", err))
	}

	// Return as hex string
	return hexutil.Encode(rlpBytes)
}

// convertJsValueToRlpType converts a JavaScript value to a Go type suitable for RLP encoding
func convertJsValueToRlpType(jsVal js.Value) (interface{}, error) {
	// Handle null/undefined
	if jsVal.IsNull() || jsVal.IsUndefined() {
		return []byte{}, nil // Empty bytes for null/undefined
	}

	// Handle strings FIRST (before objects) to avoid String objects being treated as regular objects
	if jsVal.Type() == js.TypeString {
		str := jsVal.String()
		// Check if it's a hex string (starts with 0x)
		if strings.HasPrefix(str, "0x") || strings.HasPrefix(str, "0X") {
			// Decode hex string to bytes
			bytes, err := hexutil.Decode(str)
			if err != nil {
				return nil, fmt.Errorf("invalid hex string: %v", err)
			}
			return bytes, nil
		}
		// Regular string
		return str, nil
	}

	// Handle arrays
	if jsVal.InstanceOf(js.Global().Get("Array")) {
		length := jsVal.Get("length").Int()
		result := make([]interface{}, length)
		for i := 0; i < length; i++ {
			elem, err := convertJsValueToRlpType(jsVal.Index(i))
			if err != nil {
				return nil, fmt.Errorf("array element %d: %v", i, err)
			}
			result[i] = elem
		}
		return result, nil
	}

	// Handle objects (maps) - but exclude String objects which are already handled above
	// Check if it's a String object instance (which would have been handled as TypeString above)
	// Only process plain objects, not String/Number/Boolean objects
	if jsVal.InstanceOf(js.Global().Get("Object")) && !jsVal.InstanceOf(js.Global().Get("Array")) {
		// Check if it's a String object - if so, extract the primitive value
		if jsVal.InstanceOf(js.Global().Get("String")) {
			str := jsVal.String()
			if strings.HasPrefix(str, "0x") || strings.HasPrefix(str, "0X") {
				bytes, err := hexutil.Decode(str)
				if err != nil {
					return nil, fmt.Errorf("invalid hex string: %v", err)
				}
				return bytes, nil
			}
			return str, nil
		}

		keys := js.Global().Get("Object").Call("keys", jsVal)
		length := keys.Get("length").Int()
		result := make([]interface{}, 0, length*2) // RLP encodes maps as alternating key-value pairs
		for i := 0; i < length; i++ {
			key := keys.Index(i).String()
			value := jsVal.Get(key)
			// Pass key as a string directly, not wrapped in String object
			keyVal := key // Use string directly for RLP encoding
			valVal, err := convertJsValueToRlpType(value)
			if err != nil {
				return nil, fmt.Errorf("object value for key %s: %v", key, err)
			}
			result = append(result, keyVal, valVal)
		}
		return result, nil
	}

	// Handle numbers
	if jsVal.Type() == js.TypeNumber {
		num := jsVal.Float()
		// Check if it's an integer
		if num == float64(int64(num)) {
			// Convert to big.Int for RLP encoding
			return big.NewInt(int64(num)), nil
		}
		// For non-integer numbers, convert to string representation
		return strconv.FormatFloat(num, 'f', -1, 64), nil
	}

	// Handle booleans
	if jsVal.Type() == js.TypeBoolean {
		if jsVal.Bool() {
			return uint8(1), nil
		}
		return uint8(0), nil
	}

	return nil, fmt.Errorf("unsupported JavaScript type: %v", jsVal.Type())
}

// DecodeRlpWrapper is a wrapper function for DecodeRlp to be used with js.FuncOf
func DecodeRlpWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.Global().Get("Error").New("DecodeRlp: insufficient arguments, expected at least 1 (data)")
	}
	data := args[0].String()
	return DecodeRlp(data)
}

// DecodeRlp decodes RLP-encoded data back to a JavaScript-compatible value.
// Takes a hex-encoded string and returns a JSON string representation of the decoded value.
func DecodeRlp(data string) interface{} {
	// Validate input
	if strings.TrimSpace(data) == "" {
		return js.Global().Get("Error").New("DecodeRlp: data cannot be empty")
	}

	// Decode hex string to bytes
	if !strings.HasPrefix(data, "0x") && !strings.HasPrefix(data, "0X") {
		data = "0x" + data
	}
	rlpBytes, err := hexutil.Decode(data)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("DecodeRlp: failed to decode hex data: %v", err))
	}

	// Decode RLP bytes into interface{}
	var decoded interface{}
	err = rlp.DecodeBytes(rlpBytes, &decoded)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("DecodeRlp: failed to decode RLP: %v", err))
	}

	// Convert decoded Go value to JavaScript-compatible format
	jsValue := convertRlpDecodedToJsValue(decoded)

	// Return as JSON string
	jsonData, err := json.Marshal(jsValue)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("DecodeRlp: failed to marshal to JSON: %v", err))
	}

	return string(jsonData)
}

// convertRlpDecodedToJsValue converts a decoded RLP value to a JavaScript-compatible format
func convertRlpDecodedToJsValue(val interface{}) interface{} {
	if val == nil {
		return nil
	}

	// Use reflection to handle different types
	rv := reflect.ValueOf(val)
	rt := rv.Type()

	// Handle slices/arrays
	if rt.Kind() == reflect.Slice {
		length := rv.Len()
		result := make([]interface{}, length)
		for i := 0; i < length; i++ {
			result[i] = convertRlpDecodedToJsValue(rv.Index(i).Interface())
		}
		return result
	}

	// Handle byte slices - convert to hex string
	if rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.Uint8 {
		bytes := val.([]byte)
		return hexutil.Encode(bytes)
	}

	// Handle big.Int - convert to number string
	if bigInt, ok := val.(*big.Int); ok {
		return bigInt.String()
	}

	// Handle uint8 (boolean representation)
	if u8, ok := val.(uint8); ok {
		return u8 != 0
	}

	// Handle unsigned integers
	switch v := val.(type) {
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint()
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int()
	}

	// Handle strings
	if str, ok := val.(string); ok {
		return str
	}

	// Handle booleans
	if b, ok := val.(bool); ok {
		return b
	}

	// For other types, try to convert to string
	return fmt.Sprintf("%v", val)
}
