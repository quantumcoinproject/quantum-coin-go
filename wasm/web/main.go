//go:build js && wasm
// +build js,wasm

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"syscall/js"

	"github.com/google/uuid"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereds"
	"github.com/quantumcoinproject/quantum-coin-go/params"
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
	FromAddress common.Address `json:"fromAddress"`
	ToAddress   common.Address `json:"toAddress"`
	Nonce       uint64         `json:"nonce"`
	GasLimit    uint64         `json:"gasLimit"`
	Value       *big.Int       `json:"value"`
	Data        []byte         `json:"data"`
	ChainId     *big.Int       `json:"chainId"`
	Remarks     []byte         `json:"remarks"`
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
	js.Global().Set("TxnSigningHash2", js.FuncOf(TxnSigningHash2))
	js.Global().Set("TxnHash2", js.FuncOf(TxnHash2))
	js.Global().Set("TxnData2", js.FuncOf(TxnData2))
	<-done
}

// PackMethodDataWrapper is a wrapper function for PackMethodData to be used with js.FuncOf
func PackMethodDataWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) < 3 {
		return nil
	}
	abiJSON := args[0].String()
	methodName := args[1].String()
	methodArgs := args[2:]
	return PackMethodData(abiJSON, methodName, methodArgs)
}

// UnpackMethodDataWrapper is a wrapper function for UnpackMethodData to be used with js.FuncOf
func UnpackMethodDataWrapper(this js.Value, args []js.Value) interface{} {
	if len(args) != 3 {
		return nil
	}
	abiJSON := args[0].String()
	methodName := args[1].String()
	hexData := args[2]
	return UnpackMethodData(abiJSON, methodName, hexData)
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
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
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
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data, ts.Transaction[0].Remarks)

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
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data, ts.Transaction[0].Remarks)

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

	toAddress := common.HexToAddress(args[2].String())

	weiVal, err := hexutil.DecodeBig(args[3].String())
	if err != nil {
		return Transaction2{}, err
	}

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

	remarksString := js.Global().Get("Uint8Array").New(args[7])
	remarks := make([]byte, remarksString.Get("length").Int())
	js.CopyBytesToGo(remarks, remarksString)

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
//   - interface{}: The packed transaction data as a string, or nil on error
func PackMethodData(abiJSON string, methodName string, args []js.Value) interface{} {
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil
	}

	method, exist := abiData.Methods[methodName]
	if !exist {
		return nil
	}

	if len(args) != len(method.Inputs) {
		return nil
	}

	// Convert js.Value arguments to Go types based on ABI types
	convertedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		converted, err := convertJsValueToGoType(arg, method.Inputs[i].Type)
		if err != nil {
			return nil
		}
		convertedArgs[i] = converted
	}

	data, err := abiData.Pack(methodName, convertedArgs...)
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

// UnpackMethodData unpacks the return values of a Solidity method call.
// It returns the unpacked values as a JSON string that can be parsed in JavaScript.
//
// Parameters:
//   - abiJSON: The Solidity ABI file content as a JSON string
//   - methodName: The name of the method whose return values to unpack
//   - hexData: The hex-encoded return data as a js.Value (string, with or without 0x prefix)
//
// Returns:
//   - interface{}: The unpacked return values as a JSON string, or nil on error
func UnpackMethodData(abiJSON string, methodName string, hexData js.Value) interface{} {
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil
	}

	method, exist := abiData.Methods[methodName]
	if !exist {
		return nil
	}

	// Get hex string from js.Value and decode to bytes
	hexStr := hexData.String()
	if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
		hexStr = "0x" + hexStr
	}
	data, err := hexutil.Decode(hexStr)
	if err != nil {
		return nil
	}

	// Unpack the return values using method.Outputs (not Inputs)
	// Return values don't include method ID, just the raw return data
	unpacked, err := method.Outputs.Unpack(data)
	if err != nil {
		return nil
	}

	// Convert unpacked values to JavaScript-compatible format
	jsValues := make([]interface{}, len(unpacked))
	for i, val := range unpacked {
		jsValues[i] = convertGoTypeToJsValue(val)
	}

	// Return as JSON string
	jsonData, err := json.Marshal(jsValues)
	if err != nil {
		return nil
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
		// Convert big.Int to hex string
		return hexutil.EncodeBig(v)
	case []byte:
		// Convert bytes to hex string
		return hexutil.Encode(v)
	case bool:
		return v
	case string:
		return v
	case []interface{}:
		// Recursively convert array elements
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
		// Convert from hex string to big.Int
		hexStr := jsVal.String()
		if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
			hexStr = "0x" + hexStr
		}
		val, err := hexutil.DecodeBig(hexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid uint value: %s", hexStr)
		}
		return val, nil

	case abi.IntTy:
		// Signed integer types (int8, int256, etc.)
		// Convert from hex string to big.Int
		hexStr := jsVal.String()
		if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
			hexStr = "0x" + hexStr
		}
		val, err := hexutil.DecodeBig(hexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid int value: %s", hexStr)
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
