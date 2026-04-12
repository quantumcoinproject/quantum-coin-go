//go:build !js
// +build !js

package main

import (
	"bytes"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	ks "github.com/quantumcoinproject/quantum-coin-go/accounts/keystore"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	abi "github.com/quantumcoinproject/quantum-coin-go/wasm/accounts/abi"
)

// mockJsValue is a mock implementation of js.Value for testing
type mockJsValue struct {
	valueType jsType
	stringVal string
	boolVal   bool
	numberVal float64
	objectVal map[string]interface{}
	arrayVal  []interface{}
	length    int
}

type jsType int

const (
	jsTypeString jsType = iota
	jsTypeNumber
	jsTypeBoolean
	jsTypeObject
	jsTypeUndefined
)

func (m *mockJsValue) String() string {
	if m.valueType == jsTypeString {
		return m.stringVal
	}
	if m.valueType == jsTypeNumber {
		return m.stringVal
	}
	if m.valueType == jsTypeBoolean {
		if m.boolVal {
			return "true"
		}
		return "false"
	}
	return ""
}

func (m *mockJsValue) Bool() bool {
	return m.boolVal
}

func (m *mockJsValue) Int() int {
	return int(m.numberVal)
}

func (m *mockJsValue) Float() float64 {
	return m.numberVal
}

func (m *mockJsValue) Type() jsType {
	return m.valueType
}

func (m *mockJsValue) Get(name string) *mockJsValue {
	if m.valueType == jsTypeObject {
		if val, ok := m.objectVal[name]; ok {
			return convertToMockJsValue(val)
		}
	}
	return &mockJsValue{valueType: jsTypeUndefined}
}

func (m *mockJsValue) Index(i int) *mockJsValue {
	if m.valueType == jsTypeObject && m.arrayVal != nil && i < len(m.arrayVal) {
		return convertToMockJsValue(m.arrayVal[i])
	}
	return &mockJsValue{valueType: jsTypeUndefined}
}

func (m *mockJsValue) IsUndefined() bool {
	return m.valueType == jsTypeUndefined
}

func (m *mockJsValue) IsNull() bool {
	return m.valueType == jsTypeUndefined // For testing, treat undefined as null
}

func (m *mockJsValue) InstanceOf(constructor interface{}) bool {
	// For testing, we need to handle Array and Object constructors
	// In the real implementation, constructor would be js.Global().Get("Array") or js.Global().Get("Object")
	// For testing, we'll use string comparison or type checking

	// If it's an object type with arrayVal set, it's an Array
	if m.valueType == jsTypeObject && m.arrayVal != nil {
		// Check if constructor is "Array" (we'll pass a string for testing)
		if str, ok := constructor.(string); ok {
			return str == "Array"
		}
		// Also check for Uint8Array (all elements are numbers)
		allNumbers := true
		for _, v := range m.arrayVal {
			if _, ok := v.(float64); !ok {
				allNumbers = false
				break
			}
		}
		if allNumbers {
			return true // Uint8Array
		}
		return true // Array
	}

	// If it's an object type with objectVal set, it's an Object (not Array)
	if m.valueType == jsTypeObject && m.objectVal != nil {
		if str, ok := constructor.(string); ok {
			return str == "Object"
		}
		return true // Object
	}

	return false
}

func convertToMockJsValue(val interface{}) *mockJsValue {
	switch v := val.(type) {
	case string:
		return &mockJsValue{valueType: jsTypeString, stringVal: v}
	case bool:
		return &mockJsValue{valueType: jsTypeBoolean, boolVal: v}
	case float64:
		return &mockJsValue{valueType: jsTypeNumber, numberVal: v, stringVal: big.NewInt(int64(v)).String()}
	case int:
		return &mockJsValue{valueType: jsTypeNumber, numberVal: float64(v), stringVal: big.NewInt(int64(v)).String()}
	case int64:
		return &mockJsValue{valueType: jsTypeNumber, numberVal: float64(v), stringVal: big.NewInt(v).String()}
	case []interface{}:
		return &mockJsValue{valueType: jsTypeObject, arrayVal: v, length: len(v)}
	case map[string]interface{}:
		return &mockJsValue{valueType: jsTypeObject, objectVal: v}
	case []byte:
		// Convert bytes to array of numbers for Uint8Array simulation
		arr := make([]interface{}, len(v))
		for i, b := range v {
			arr[i] = float64(b)
		}
		return &mockJsValue{valueType: jsTypeObject, arrayVal: arr, length: len(v)}
	default:
		return &mockJsValue{valueType: jsTypeUndefined}
	}
}

// Wrapper to use mockJsValue with the real convertJsValueToGoType function
// We need to create an adapter since the real function expects js.Value
// For actual testing, we'll need to modify the function or create a testable version

// Test helper to create ABI types
func createAddressType() abi.Type {
	typ, _ := abi.NewType("address", "", nil)
	return typ
}

func createUintType(size int) abi.Type {
	typ, _ := abi.NewType(fmt.Sprintf("uint%d", size), "", nil)
	return typ
}

func createIntType(size int) abi.Type {
	typ, _ := abi.NewType(fmt.Sprintf("int%d", size), "", nil)
	return typ
}

func createBoolType() abi.Type {
	typ, _ := abi.NewType("bool", "", nil)
	return typ
}

func createStringType() abi.Type {
	typ, _ := abi.NewType("string", "", nil)
	return typ
}

func createBytesType() abi.Type {
	typ, _ := abi.NewType("bytes", "", nil)
	return typ
}

func createFixedBytesType(size int) abi.Type {
	typ, _ := abi.NewType(fmt.Sprintf("bytes%d", size), "", nil)
	return typ
}

func createSliceType(elemType abi.Type) abi.Type {
	typ, _ := abi.NewType(elemType.String()+"[]", "", nil)
	return typ
}

func createArrayType(elemType abi.Type, size int) abi.Type {
	typ, _ := abi.NewType(fmt.Sprintf("%s[%d]", elemType.String(), size), "", nil)
	return typ
}

// Since we can't easily test js.Value in a non-WASM environment,
// we'll create a testable version that accepts interface{} and converts internally
func convertValueToGoTypeForTest(val interface{}, abiType abi.Type) (interface{}, error) {
	// Convert our test value to a format that can work with the conversion logic
	// For actual testing, we'd need to run this in a WASM environment or mock js.Value properly

	// For now, let's create a simplified test that validates the logic with direct type conversions
	// This is a workaround since js.Value can't be easily mocked

	switch abiType.T {
	case abi.AddressTy:
		addrStr, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string for address")
		}
		if !common.IsHexAddress(addrStr) {
			return nil, fmt.Errorf("invalid address: %s", addrStr)
		}
		return common.HexToAddress(addrStr), nil

	case abi.UintTy, abi.IntTy:
		// Convert from number to big.Int
		switch v := val.(type) {
		case string:
			// Parse as number string
			bigVal := new(big.Int)
			_, ok := bigVal.SetString(v, 10)
			if !ok {
				return nil, fmt.Errorf("invalid int/uint value: %s", v)
			}
			return bigVal, nil
		case int:
			return big.NewInt(int64(v)), nil
		case int64:
			return big.NewInt(v), nil
		case float64:
			bigFloat := new(big.Float).SetFloat64(v)
			bigVal, _ := bigFloat.Int(nil)
			return bigVal, nil
		case *big.Int:
			return v, nil
		default:
			return nil, fmt.Errorf("unsupported type for int/uint: %T", val)
		}

	case abi.BoolTy:
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("expected bool")
		}
		return b, nil

	case abi.StringTy:
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("expected string")
		}
		return s, nil

	case abi.BytesTy:
		var bytes []byte
		switch v := val.(type) {
		case []byte:
			bytes = v
		case string:
			if !strings.HasPrefix(v, "0x") && !strings.HasPrefix(v, "0X") {
				v = "0x" + v
			}
			var err error
			bytes, err = hexutil.Decode(v)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported type for bytes: %T", val)
		}
		return bytes, nil

	case abi.FixedBytesTy:
		var bytes []byte
		switch v := val.(type) {
		case []byte:
			bytes = v
		case string:
			if !strings.HasPrefix(v, "0x") && !strings.HasPrefix(v, "0X") {
				v = "0x" + v
			}
			var err error
			bytes, err = hexutil.Decode(v)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported type for fixed bytes: %T", val)
		}
		if len(bytes) != abiType.Size {
			return nil, fmt.Errorf("fixed bytes size mismatch: expected %d, got %d", abiType.Size, len(bytes))
		}
		return bytes, nil

	case abi.SliceTy:
		arr, ok := val.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected array for slice type")
		}
		result := make([]interface{}, len(arr))
		for i, elem := range arr {
			converted, err := convertValueToGoTypeForTest(elem, *abiType.Elem)
			if err != nil {
				return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
			}
			result[i] = converted
		}
		return result, nil

	case abi.ArrayTy:
		arr, ok := val.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected array for fixed array type")
		}
		if len(arr) != abiType.Size {
			return nil, fmt.Errorf("array size mismatch: expected %d, got %d", abiType.Size, len(arr))
		}
		result := make([]interface{}, len(arr))
		for i, elem := range arr {
			converted, err := convertValueToGoTypeForTest(elem, *abiType.Elem)
			if err != nil {
				return nil, fmt.Errorf("failed to convert array element %d: %w", i, err)
			}
			result[i] = converted
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported ABI type: %d", abiType.T)
	}
}

func TestConvertJsValueToGoType_Address(t *testing.T) {
	// Use a valid 32-byte address (64 hex chars + 0x = 66 chars total)
	// Simple address with all valid hex characters
	testAddr := "0x0000000000000000000000000000000000000000000000000000000000000000"
	if !common.IsHexAddress(testAddr) {
		t.Fatalf("Test address is not valid: %s (length: %d)", testAddr, len(testAddr))
	}
	expectedAddr := common.HexToAddress(testAddr)

	abiType := createAddressType()
	result, err := convertValueToGoTypeForTest(testAddr, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	addr, ok := result.(common.Address)
	if !ok {
		t.Fatalf("Expected common.Address, got %T", result)
	}

	if addr != expectedAddr {
		t.Errorf("Address mismatch: expected %s, got %s", expectedAddr.String(), addr.String())
	}

	// Verify round-trip: convert back to string and check
	addrStr := addr.String()
	if addrStr != testAddr {
		t.Errorf("Round-trip failed: expected %s, got %s", testAddr, addrStr)
	}
}

func TestConvertJsValueToGoType_Uint256(t *testing.T) {
	// Test with number string (1e18 in decimal)
	testValue := "1000000000000000000"
	abiType := createUintType(256)

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	bigVal, ok := result.(*big.Int)
	if !ok {
		t.Fatalf("Expected *big.Int, got %T", result)
	}

	expectedVal := big.NewInt(1000000000000000000)
	if bigVal.Cmp(expectedVal) != 0 {
		t.Errorf("Value mismatch: expected %s, got %s", expectedVal.String(), bigVal.String())
	}

	// Verify round-trip: convert back to number string and check
	numStr := bigVal.String()
	if numStr != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, numStr)
	}
}

func TestConvertJsValueToGoType_Int256(t *testing.T) {
	// Test with number string for negative number
	testValue := "-1"
	abiType := createIntType(256)

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	bigVal, ok := result.(*big.Int)
	if !ok {
		t.Fatalf("Expected *big.Int, got %T", result)
	}

	expectedVal := big.NewInt(-1)
	if bigVal.Cmp(expectedVal) != 0 {
		t.Errorf("Value mismatch: expected %s, got %s", expectedVal.String(), bigVal.String())
	}

	// Verify round-trip: convert back to number string and check
	numStr := bigVal.String()
	if numStr != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, numStr)
	}
}

func TestConvertJsValueToGoType_Bool(t *testing.T) {
	testCases := []bool{true, false}

	abiType := createBoolType()

	for _, testValue := range testCases {
		result, err := convertValueToGoTypeForTest(testValue, abiType)
		if err != nil {
			t.Fatalf("Conversion failed for %v: %v", testValue, err)
		}

		boolVal, ok := result.(bool)
		if !ok {
			t.Fatalf("Expected bool, got %T", result)
		}

		if boolVal != testValue {
			t.Errorf("Value mismatch: expected %v, got %v", testValue, boolVal)
		}

		// Verify round-trip
		if boolVal != testValue {
			t.Errorf("Round-trip failed: expected %v, got %v", testValue, boolVal)
		}
	}
}

func TestConvertJsValueToGoType_String(t *testing.T) {
	testValue := "Hello, World!"
	abiType := createStringType()

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	strVal, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string, got %T", result)
	}

	if strVal != testValue {
		t.Errorf("Value mismatch: expected %s, got %s", testValue, strVal)
	}

	// Verify round-trip
	if strVal != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, strVal)
	}
}

func TestConvertJsValueToGoType_Bytes(t *testing.T) {
	testValue := "0x48656c6c6f" // "Hello" in hex
	expectedBytes, _ := hexutil.Decode(testValue)
	abiType := createBytesType()

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	bytesVal, ok := result.([]byte)
	if !ok {
		t.Fatalf("Expected []byte, got %T", result)
	}

	if !reflect.DeepEqual(bytesVal, expectedBytes) {
		t.Errorf("Value mismatch: expected %v, got %v", expectedBytes, bytesVal)
	}

	// Verify round-trip: convert back to hex and check
	hexStr := hexutil.Encode(bytesVal)
	if hexStr != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, hexStr)
	}
}

func TestConvertJsValueToGoType_BytesWithoutPrefix(t *testing.T) {
	testValue := "48656c6c6f" // without 0x prefix
	expectedValue := "0x" + testValue
	expectedBytes, _ := hexutil.Decode(expectedValue)
	abiType := createBytesType()

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	bytesVal, ok := result.([]byte)
	if !ok {
		t.Fatalf("Expected []byte, got %T", result)
	}

	if !reflect.DeepEqual(bytesVal, expectedBytes) {
		t.Errorf("Value mismatch: expected %v, got %v", expectedBytes, bytesVal)
	}
}

func TestConvertJsValueToGoType_FixedBytes32(t *testing.T) {
	testValue := "0x" + strings.Repeat("01", 32) // 32 bytes
	expectedBytes, _ := hexutil.Decode(testValue)
	abiType := createFixedBytesType(32)

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	bytesVal, ok := result.([]byte)
	if !ok {
		t.Fatalf("Expected []byte, got %T", result)
	}

	if len(bytesVal) != 32 {
		t.Errorf("Expected 32 bytes, got %d", len(bytesVal))
	}

	if !reflect.DeepEqual(bytesVal, expectedBytes) {
		t.Errorf("Value mismatch: expected %v, got %v", expectedBytes, bytesVal)
	}

	// Verify round-trip
	hexStr := hexutil.Encode(bytesVal)
	if hexStr != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, hexStr)
	}
}

func TestConvertJsValueToGoType_Uint256Array(t *testing.T) {
	// Test with number strings
	// 1e18, 2e18, 3e18
	testValue := []interface{}{
		"1000000000000000000",
		"2000000000000000000",
		"3000000000000000000",
	}

	elemType := createUintType(256)
	abiType := createSliceType(elemType)

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	arrVal, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(arrVal) != len(testValue) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValue), len(arrVal))
	}

	// Verify each element
	expectedValues := []*big.Int{
		big.NewInt(1000000000000000000),
		big.NewInt(2000000000000000000),
		big.NewInt(3000000000000000000),
	}

	for i, elem := range arrVal {
		bigVal, ok := elem.(*big.Int)
		if !ok {
			t.Fatalf("Expected *big.Int at index %d, got %T", i, elem)
		}

		expectedVal := expectedValues[i]
		if bigVal.Cmp(expectedVal) != 0 {
			t.Errorf("Element %d mismatch: expected %s, got %s", i, expectedVal.String(), bigVal.String())
		}

		// Verify round-trip: convert back to number string and check
		numStr := bigVal.String()
		if numStr != testValue[i].(string) {
			t.Errorf("Round-trip failed for element %d: expected %s, got %s", i, testValue[i].(string), numStr)
		}
	}
}

func TestConvertJsValueToGoType_AddressArray(t *testing.T) {
	// Use valid 32-byte addresses (64 hex chars + 0x = 66 chars total)
	// Simple addresses with all valid hex characters
	testValue := []interface{}{
		"0x0000000000000000000000000000000000000000000000000000000000001000",
		"0x0000000000000000000000000000000000000000000000000000000000002000",
	}
	// Verify addresses are valid
	for i, addr := range testValue {
		if !common.IsHexAddress(addr.(string)) {
			t.Fatalf("Test address %d is not valid: %s (length: %d)", i, addr, len(addr.(string)))
		}
	}

	elemType := createAddressType()
	abiType := createSliceType(elemType)

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	arrVal, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(arrVal) != len(testValue) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValue), len(arrVal))
	}

	// Verify each element
	for i, elem := range arrVal {
		addr, ok := elem.(common.Address)
		if !ok {
			t.Fatalf("Expected common.Address at index %d, got %T", i, elem)
		}

		expectedAddr := common.HexToAddress(testValue[i].(string))
		if addr != expectedAddr {
			t.Errorf("Element %d mismatch: expected %s, got %s", i, expectedAddr.String(), addr.String())
		}

		// Verify round-trip
		addrStr := addr.String()
		if addrStr != testValue[i].(string) {
			t.Errorf("Round-trip failed for element %d: expected %s, got %s", i, testValue[i].(string), addrStr)
		}
	}
}

func TestConvertJsValueToGoType_FixedArray(t *testing.T) {
	// Test with number strings: 100, 200, 300
	testValue := []interface{}{
		"100",
		"200",
		"300",
	}

	elemType := createUintType(256)
	abiType := createArrayType(elemType, 3)

	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	arrVal, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(arrVal) != 3 {
		t.Fatalf("Array length mismatch: expected 3, got %d", len(arrVal))
	}

	// Verify each element
	expectedValues := []*big.Int{
		big.NewInt(100),
		big.NewInt(200),
		big.NewInt(300),
	}

	for i, elem := range arrVal {
		bigVal, ok := elem.(*big.Int)
		if !ok {
			t.Fatalf("Expected *big.Int at index %d, got %T", i, elem)
		}

		expectedVal := expectedValues[i]
		if bigVal.Cmp(expectedVal) != 0 {
			t.Errorf("Element %d mismatch: expected %s, got %s", i, expectedVal.String(), bigVal.String())
		}
	}
}

func TestConvertJsValueToGoType_FixedArraySizeMismatch(t *testing.T) {
	// Test with number strings
	testValue := []interface{}{
		"100",
		"200",
		// Missing third element
	}

	elemType := createUintType(256)
	abiType := createArrayType(elemType, 3)

	_, err := convertValueToGoTypeForTest(testValue, abiType)
	if err == nil {
		t.Fatal("Expected error for array size mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "array size mismatch") {
		t.Errorf("Expected 'array size mismatch' error, got: %v", err)
	}
}

// Testable version of UnpackMethodData for non-WASM testing
func unpackMethodDataForTest(abiJSON string, methodName string, hexData string) ([]interface{}, error) {
	// Validate method name is not empty
	if strings.TrimSpace(methodName) == "" {
		return nil, fmt.Errorf("method name cannot be empty")
	}

	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}

	method, exist := abiData.Methods[methodName]
	if !exist {
		return nil, fmt.Errorf("method '%s' not found", methodName)
	}

	// Decode hex string to bytes
	if !strings.HasPrefix(hexData, "0x") && !strings.HasPrefix(hexData, "0X") {
		hexData = "0x" + hexData
	}
	data, err := hexutil.Decode(hexData)
	if err != nil {
		return nil, err
	}

	// Unpack the return values using method.Outputs
	unpacked, err := method.Outputs.Unpack(data)
	if err != nil {
		return nil, err
	}

	return unpacked, nil
}

func TestUnpackMethodData_Uint256(t *testing.T) {
	// Create ABI for a method that returns uint256
	abiJSON := `[{
		"name": "getValue",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	}]`

	// Pack a return value: 1000000000000000000 (1e18)
	testValue := big.NewInt(1000000000000000000)
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	packed, err := abiData.Methods["getValue"].Outputs.Pack(testValue)
	if err != nil {
		t.Fatalf("Failed to pack return value: %v", err)
	}

	// Unpack the data
	hexData := hexutil.Encode(packed)
	unpacked, err := unpackMethodDataForTest(abiJSON, "getValue", hexData)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if len(unpacked) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(unpacked))
	}

	unpackedVal, ok := unpacked[0].(*big.Int)
	if !ok {
		t.Fatalf("Expected *big.Int, got %T", unpacked[0])
	}

	if unpackedVal.Cmp(testValue) != 0 {
		t.Errorf("Value mismatch: expected %s, got %s", testValue.String(), unpackedVal.String())
	}
}

func TestUnpackMethodData_Address(t *testing.T) {
	// Create ABI for a method that returns address
	abiJSON := `[{
		"name": "getAddress",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "address"}]
	}]`

	// Pack a return value
	testAddr := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000")
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	packed, err := abiData.Methods["getAddress"].Outputs.Pack(testAddr)
	if err != nil {
		t.Fatalf("Failed to pack return value: %v", err)
	}

	// Unpack the data
	hexData := hexutil.Encode(packed)
	unpacked, err := unpackMethodDataForTest(abiJSON, "getAddress", hexData)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if len(unpacked) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(unpacked))
	}

	unpackedAddr, ok := unpacked[0].(common.Address)
	if !ok {
		t.Fatalf("Expected common.Address, got %T", unpacked[0])
	}

	if unpackedAddr != testAddr {
		t.Errorf("Address mismatch: expected %s, got %s", testAddr.String(), unpackedAddr.String())
	}
}

func TestUnpackMethodData_Bool(t *testing.T) {
	// Create ABI for a method that returns bool
	abiJSON := `[{
		"name": "isActive",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "bool"}]
	}]`

	testCases := []bool{true, false}

	for _, testValue := range testCases {
		abiData, _ := abi.JSON(strings.NewReader(abiJSON))
		packed, err := abiData.Methods["isActive"].Outputs.Pack(testValue)
		if err != nil {
			t.Fatalf("Failed to pack return value: %v", err)
		}

		hexData := hexutil.Encode(packed)
		unpacked, err := unpackMethodDataForTest(abiJSON, "isActive", hexData)
		if err != nil {
			t.Fatalf("Unpack failed: %v", err)
		}

		if len(unpacked) != 1 {
			t.Fatalf("Expected 1 return value, got %d", len(unpacked))
		}

		unpackedBool, ok := unpacked[0].(bool)
		if !ok {
			t.Fatalf("Expected bool, got %T", unpacked[0])
		}

		if unpackedBool != testValue {
			t.Errorf("Bool mismatch: expected %v, got %v", testValue, unpackedBool)
		}
	}
}

func TestUnpackMethodData_String(t *testing.T) {
	// Create ABI for a method that returns string
	abiJSON := `[{
		"name": "getName",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "string"}]
	}]`

	testValue := "Hello, World!"
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	packed, err := abiData.Methods["getName"].Outputs.Pack(testValue)
	if err != nil {
		t.Fatalf("Failed to pack return value: %v", err)
	}

	hexData := hexutil.Encode(packed)
	unpacked, err := unpackMethodDataForTest(abiJSON, "getName", hexData)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if len(unpacked) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(unpacked))
	}

	unpackedStr, ok := unpacked[0].(string)
	if !ok {
		t.Fatalf("Expected string, got %T", unpacked[0])
	}

	if unpackedStr != testValue {
		t.Errorf("String mismatch: expected %s, got %s", testValue, unpackedStr)
	}
}

func TestUnpackMethodData_Bytes(t *testing.T) {
	// Create ABI for a method that returns bytes
	abiJSON := `[{
		"name": "getData",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "bytes"}]
	}]`

	testValue := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f} // "Hello"
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	packed, err := abiData.Methods["getData"].Outputs.Pack(testValue)
	if err != nil {
		t.Fatalf("Failed to pack return value: %v", err)
	}

	hexData := hexutil.Encode(packed)
	unpacked, err := unpackMethodDataForTest(abiJSON, "getData", hexData)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if len(unpacked) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(unpacked))
	}

	unpackedBytes, ok := unpacked[0].([]byte)
	if !ok {
		t.Fatalf("Expected []byte, got %T", unpacked[0])
	}

	if !reflect.DeepEqual(unpackedBytes, testValue) {
		t.Errorf("Bytes mismatch: expected %v, got %v", testValue, unpackedBytes)
	}
}

func TestUnpackMethodData_MultipleReturns(t *testing.T) {
	// Create ABI for a method that returns multiple values
	abiJSON := `[{
		"name": "getInfo",
		"type": "function",
		"inputs": [],
		"outputs": [
			{"name": "value", "type": "uint256"},
			{"name": "flag", "type": "bool"},
			{"name": "addr", "type": "address"}
		]
	}]`

	testValue := big.NewInt(1000000000000000000)
	testFlag := true
	testAddr := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000")

	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	packed, err := abiData.Methods["getInfo"].Outputs.Pack(testValue, testFlag, testAddr)
	if err != nil {
		t.Fatalf("Failed to pack return values: %v", err)
	}

	hexData := hexutil.Encode(packed)
	unpacked, err := unpackMethodDataForTest(abiJSON, "getInfo", hexData)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if len(unpacked) != 3 {
		t.Fatalf("Expected 3 return values, got %d", len(unpacked))
	}

	// Check first return value (uint256)
	unpackedVal, ok := unpacked[0].(*big.Int)
	if !ok {
		t.Fatalf("Expected *big.Int for first return, got %T", unpacked[0])
	}
	if unpackedVal.Cmp(testValue) != 0 {
		t.Errorf("First value mismatch: expected %s, got %s", testValue.String(), unpackedVal.String())
	}

	// Check second return value (bool)
	unpackedFlag, ok := unpacked[1].(bool)
	if !ok {
		t.Fatalf("Expected bool for second return, got %T", unpacked[1])
	}
	if unpackedFlag != testFlag {
		t.Errorf("Second value mismatch: expected %v, got %v", testFlag, unpackedFlag)
	}

	// Check third return value (address)
	unpackedAddr, ok := unpacked[2].(common.Address)
	if !ok {
		t.Fatalf("Expected common.Address for third return, got %T", unpacked[2])
	}
	if unpackedAddr != testAddr {
		t.Errorf("Third value mismatch: expected %s, got %s", testAddr.String(), unpackedAddr.String())
	}
}

func TestUnpackMethodData_Uint256Array(t *testing.T) {
	// Create ABI for a method that returns uint256[]
	abiJSON := `[{
		"name": "getValues",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256[]"}]
	}]`

	testValue := []*big.Int{
		big.NewInt(1000000000000000000),
		big.NewInt(2000000000000000000),
		big.NewInt(3000000000000000000),
	}

	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	// Pack using the slice directly - the ABI library will handle it via reflect
	packed, err := abiData.Methods["getValues"].Outputs.Pack(testValue)
	if err != nil {
		t.Fatalf("Failed to pack return value: %v", err)
	}

	hexData := hexutil.Encode(packed)
	unpacked, err := unpackMethodDataForTest(abiJSON, "getValues", hexData)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	if len(unpacked) != 1 {
		t.Fatalf("Expected 1 return value, got %d", len(unpacked))
	}

	// The unpacked array can be []*big.Int or []interface{}
	var unpackedArr []*big.Int
	switch v := unpacked[0].(type) {
	case []*big.Int:
		unpackedArr = v
	case []interface{}:
		unpackedArr = make([]*big.Int, len(v))
		for i, elem := range v {
			val, ok := elem.(*big.Int)
			if !ok {
				t.Fatalf("Expected *big.Int at index %d, got %T", i, elem)
			}
			unpackedArr[i] = val
		}
	default:
		t.Fatalf("Expected []*big.Int or []interface{}, got %T", unpacked[0])
	}

	if len(unpackedArr) != len(testValue) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValue), len(unpackedArr))
	}

	for i, unpackedVal := range unpackedArr {
		expectedVal := testValue[i]
		if unpackedVal.Cmp(expectedVal) != 0 {
			t.Errorf("Element %d mismatch: expected %s, got %s", i, expectedVal.String(), unpackedVal.String())
		}
	}
}

func TestUnpackMethodData_InvalidMethod(t *testing.T) {
	abiJSON := `[{
		"name": "getValue",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	}]`

	_, err := unpackMethodDataForTest(abiJSON, "nonExistentMethod", "0x00")
	if err == nil {
		t.Fatal("Expected error for non-existent method, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestUnpackMethodData_InvalidHexData(t *testing.T) {
	abiJSON := `[{
		"name": "getValue",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	}]`

	_, err := unpackMethodDataForTest(abiJSON, "getValue", "invalid hex")
	if err == nil {
		t.Fatal("Expected error for invalid hex data, got nil")
	}
}

// convertToTypedSliceForTest converts []interface{} to the proper typed slice required by abi.Pack
func convertToTypedSliceForTest(val interface{}, abiType abi.Type) interface{} {
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

// Testable version of PackMethodData for non-WASM testing
func packMethodDataForTest(abiJSON string, methodName string, args []interface{}) ([]byte, error) {
	// Validate method name is not empty
	if strings.TrimSpace(methodName) == "" {
		return nil, fmt.Errorf("method name cannot be empty")
	}

	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}

	method, exist := abiData.Methods[methodName]
	if !exist {
		return nil, fmt.Errorf("method '%s' not found", methodName)
	}

	if len(args) != len(method.Inputs) {
		return nil, fmt.Errorf("argument count mismatch: expected %d, got %d", len(method.Inputs), len(args))
	}

	var data []byte

	// If no arguments, call Pack without arguments
	if len(args) == 0 {
		data, err = abiData.Pack(methodName)
	} else {
		// Convert arguments to Go types based on ABI types
		convertedArgs := make([]interface{}, len(args))
		for i, arg := range args {
			converted, err := convertValueToGoTypeForTest(arg, method.Inputs[i].Type)
			if err != nil {
				return nil, fmt.Errorf("failed to convert argument %d: %w", i, err)
			}
			// Convert []interface{} to proper typed slice if needed
			converted = convertToTypedSliceForTest(converted, method.Inputs[i].Type)
			convertedArgs[i] = converted
		}
		data, err = abiData.Pack(methodName, convertedArgs...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to pack: %w", err)
	}

	return data, nil
}

func TestUnpackMethodData_EmptyMethodName(t *testing.T) {
	abiJSON := `[{
		"name": "getValue",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	}]`

	_, err := unpackMethodDataForTest(abiJSON, "", "0x00")
	if err == nil {
		t.Fatal("Expected error for empty method name, got nil")
	}

	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}

	// Test with whitespace-only method name
	_, err = unpackMethodDataForTest(abiJSON, "   ", "0x00")
	if err == nil {
		t.Fatal("Expected error for whitespace-only method name, got nil")
	}

	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}

func TestPackMethodData_EmptyMethodName(t *testing.T) {
	abiJSON := `[{
		"name": "getValue",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	}]`

	// Test with empty method name
	_, err := packMethodDataForTest(abiJSON, "", []interface{}{})
	if err == nil {
		t.Fatal("Expected error for empty method name, got nil")
	}

	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}

	// Test with whitespace-only method name
	_, err = packMethodDataForTest(abiJSON, "   ", []interface{}{})
	if err == nil {
		t.Fatal("Expected error for whitespace-only method name, got nil")
	}

	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}

func TestPackMethodData_ZeroArguments(t *testing.T) {
	// Test packing a method with zero arguments (like name(), symbol(), etc.)
	abiJSON := `[{
		"name": "name",
		"type": "function",
		"inputs": [],
		"outputs": [{"name": "", "type": "string"}]
	}]`

	// Pack with zero arguments
	packed, err := packMethodDataForTest(abiJSON, "name", []interface{}{})
	if err != nil {
		t.Fatalf("Failed to pack zero-argument method: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify it contains the method selector (first 4 bytes)
	if len(packed) < 4 {
		t.Fatal("Packed data should contain method selector (at least 4 bytes)")
	}

	// Verify we can unpack it (should just be the method selector for zero-arg methods)
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	method := abiData.Methods["name"]
	expectedSelector := method.ID
	if !reflect.DeepEqual(packed[:4], expectedSelector) {
		t.Errorf("Method selector mismatch: expected %x, got %x", expectedSelector, packed[:4])
	}
}

func TestPackMethodData_AddressArray(t *testing.T) {
	// Test packing a method with address[] argument (like getAmountsIn)
	abiJSON := `[{
		"name": "getAmountsIn",
		"type": "function",
		"inputs": [
			{"name": "amountOut", "type": "uint256"},
			{"name": "path", "type": "address[]"}
		],
		"outputs": [{"name": "amounts", "type": "uint256[]"}]
	}]`

	// Test addresses (32-byte addresses)
	addr1 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	addr2 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000002")

	// Pack with address array
	packed, err := packMethodDataForTest(abiJSON, "getAmountsIn", []interface{}{
		"1000000000000000000",                         // amountOut as number string
		[]interface{}{addr1.String(), addr2.String()}, // path as []interface{} with address strings
	})
	if err != nil {
		t.Fatalf("Failed to pack method with address array: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify it contains the method selector
	if len(packed) < 4 {
		t.Fatal("Packed data should contain method selector (at least 4 bytes)")
	}

	// Verify we can unpack the arguments to verify correctness
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	method := abiData.Methods["getAmountsIn"]
	expectedSelector := method.ID
	if !reflect.DeepEqual(packed[:4], expectedSelector) {
		t.Errorf("Method selector mismatch: expected %x, got %x", expectedSelector, packed[:4])
	}
}

func TestPackMethodData_Uint256Array(t *testing.T) {
	// Test packing a method with uint256[] argument
	abiJSON := `[{
		"name": "getAmounts",
		"type": "function",
		"inputs": [
			{"name": "amounts", "type": "uint256[]"}
		],
		"outputs": [{"name": "", "type": "uint256[]"}]
	}]`

	// Test with uint256 array
	testAmounts := []interface{}{
		"1000000000000000000", // 1 token
		"2000000000000000000", // 2 tokens
		"3000000000000000000", // 3 tokens
	}

	packed, err := packMethodDataForTest(abiJSON, "getAmounts", []interface{}{
		testAmounts,
	})
	if err != nil {
		t.Fatalf("Failed to pack method with uint256 array: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify it contains the method selector
	if len(packed) < 4 {
		t.Fatal("Packed data should contain method selector (at least 4 bytes)")
	}

	// Verify method selector
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	method := abiData.Methods["getAmounts"]
	expectedSelector := method.ID
	if !reflect.DeepEqual(packed[:4], expectedSelector) {
		t.Errorf("Method selector mismatch: expected %x, got %x", expectedSelector, packed[:4])
	}
}

func TestPackMethodData_MixedArguments(t *testing.T) {
	// Test packing a method with mixed arguments including arrays
	abiJSON := `[{
		"name": "swapExactTokensForTokens",
		"type": "function",
		"inputs": [
			{"name": "amountIn", "type": "uint256"},
			{"name": "amountOutMin", "type": "uint256"},
			{"name": "path", "type": "address[]"},
			{"name": "to", "type": "address"},
			{"name": "deadline", "type": "uint256"}
		],
		"outputs": [{"name": "amounts", "type": "uint256[]"}]
	}]`

	addr1 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	addr2 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000002")
	toAddr := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000003")

	amountIn := "1000000000000000000"              // 1 token
	amountOutMin := "1000000000000000000000000000" // Some minimum
	deadline := "100000000"                        // Some deadline

	packed, err := packMethodDataForTest(abiJSON, "swapExactTokensForTokens", []interface{}{
		amountIn,
		amountOutMin,
		[]interface{}{addr1.String(), addr2.String()}, // path
		toAddr.String(), // to
		deadline,
	})
	if err != nil {
		t.Fatalf("Failed to pack method with mixed arguments: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify method selector
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	method := abiData.Methods["swapExactTokensForTokens"]
	expectedSelector := method.ID
	if !reflect.DeepEqual(packed[:4], expectedSelector) {
		t.Errorf("Method selector mismatch: expected %x, got %x", expectedSelector, packed[:4])
	}
}

// Testable version of convertGoTypeToJsValue for non-WASM testing
func convertGoTypeToJsValueForTest(val interface{}) interface{} {
	switch v := val.(type) {
	case common.Address:
		return v.String()
	case common.Hash:
		return v.Hex()
	case *big.Int:
		return v.String()
	case []byte:
		return hexutil.Encode(v)
	case bool:
		return v
	case string:
		return v
	case []*big.Int:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValueForTest(elem)
		}
		return result
	case []common.Address:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValueForTest(elem)
		}
		return result
	case []bool:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValueForTest(elem)
		}
		return result
	case []string:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValueForTest(elem)
		}
		return result
	case [][]byte:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValueForTest(elem)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = convertGoTypeToJsValueForTest(elem)
		}
		return result
	default:
		return fmt.Sprintf("%v", v)
	}
}

func TestConvertGoTypeToJsValue_BigIntArray(t *testing.T) {
	// Test conversion of []*big.Int to array of hex strings
	testValues := []*big.Int{
		big.NewInt(1000000000000000000), // 1e18
		big.NewInt(2000000000000000000), // 2e18
		big.NewInt(3000000000000000000), // 3e18
	}

	result := convertGoTypeToJsValueForTest(testValues)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != len(testValues) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValues), len(resultArray))
	}

	for i, val := range resultArray {
		numStr, ok := val.(string)
		if !ok {
			t.Fatalf("Expected string at index %d, got %T", i, val)
		}

		expectedNum := testValues[i].String()
		if numStr != expectedNum {
			t.Errorf("Value mismatch at index %d: expected %s, got %s", i, expectedNum, numStr)
		}
	}
}

func TestConvertGoTypeToJsValue_AddressArray(t *testing.T) {
	// Test conversion of []common.Address to array of hex strings
	testAddresses := []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001"),
		common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000002"),
		common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000003"),
	}

	result := convertGoTypeToJsValueForTest(testAddresses)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != len(testAddresses) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testAddresses), len(resultArray))
	}

	for i, val := range resultArray {
		hexStr, ok := val.(string)
		if !ok {
			t.Fatalf("Expected string at index %d, got %T", i, val)
		}

		expectedHex := testAddresses[i].String()
		if hexStr != expectedHex {
			t.Errorf("Address mismatch at index %d: expected %s, got %s", i, expectedHex, hexStr)
		}
	}
}

func TestConvertGoTypeToJsValue_BoolArray(t *testing.T) {
	// Test conversion of []bool to array of bools
	testValues := []bool{true, false, true, false}

	result := convertGoTypeToJsValueForTest(testValues)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != len(testValues) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValues), len(resultArray))
	}

	for i, val := range resultArray {
		boolVal, ok := val.(bool)
		if !ok {
			t.Fatalf("Expected bool at index %d, got %T", i, val)
		}

		if boolVal != testValues[i] {
			t.Errorf("Bool mismatch at index %d: expected %v, got %v", i, testValues[i], boolVal)
		}
	}
}

func TestConvertGoTypeToJsValue_StringArray(t *testing.T) {
	// Test conversion of []string to array of strings
	testValues := []string{"hello", "world", "test"}

	result := convertGoTypeToJsValueForTest(testValues)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != len(testValues) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValues), len(resultArray))
	}

	for i, val := range resultArray {
		strVal, ok := val.(string)
		if !ok {
			t.Fatalf("Expected string at index %d, got %T", i, val)
		}

		if strVal != testValues[i] {
			t.Errorf("String mismatch at index %d: expected %s, got %s", i, testValues[i], strVal)
		}
	}
}

func TestConvertGoTypeToJsValue_BytesArray(t *testing.T) {
	// Test conversion of [][]byte to array of hex strings
	testValues := [][]byte{
		{0x48, 0x65, 0x6c, 0x6c, 0x6f}, // "Hello"
		{0x57, 0x6f, 0x72, 0x6c, 0x64}, // "World"
		{0x54, 0x65, 0x73, 0x74},       // "Test"
	}

	result := convertGoTypeToJsValueForTest(testValues)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != len(testValues) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValues), len(resultArray))
	}

	for i, val := range resultArray {
		hexStr, ok := val.(string)
		if !ok {
			t.Fatalf("Expected string at index %d, got %T", i, val)
		}

		expectedHex := hexutil.Encode(testValues[i])
		if hexStr != expectedHex {
			t.Errorf("Bytes mismatch at index %d: expected %s, got %s", i, expectedHex, hexStr)
		}
	}
}

func TestConvertGoTypeToJsValue_InterfaceArray(t *testing.T) {
	// Test conversion of []interface{} with mixed types
	testValues := []interface{}{
		big.NewInt(1000000000000000000),
		common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001"),
		true,
		"test",
	}

	result := convertGoTypeToJsValueForTest(testValues)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != len(testValues) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(testValues), len(resultArray))
	}

	// Check first element (big.Int)
	numStr, ok := resultArray[0].(string)
	if !ok {
		t.Fatalf("Expected string for big.Int, got %T", resultArray[0])
	}
	expectedNum := testValues[0].(*big.Int).String()
	if numStr != expectedNum {
		t.Errorf("BigInt mismatch: expected %s, got %s", expectedNum, numStr)
	}

	// Check second element (address)
	addrStr, ok := resultArray[1].(string)
	if !ok {
		t.Fatalf("Expected string for address, got %T", resultArray[1])
	}
	expectedAddr := testValues[1].(common.Address).String()
	if addrStr != expectedAddr {
		t.Errorf("Address mismatch: expected %s, got %s", expectedAddr, addrStr)
	}

	// Check third element (bool)
	boolVal, ok := resultArray[2].(bool)
	if !ok {
		t.Fatalf("Expected bool, got %T", resultArray[2])
	}
	if boolVal != testValues[2] {
		t.Errorf("Bool mismatch: expected %v, got %v", testValues[2], boolVal)
	}

	// Check fourth element (string)
	strVal, ok := resultArray[3].(string)
	if !ok {
		t.Fatalf("Expected string, got %T", resultArray[3])
	}
	if strVal != testValues[3] {
		t.Errorf("String mismatch: expected %s, got %s", testValues[3], strVal)
	}
}

func TestConvertGoTypeToJsValue_NestedArrays(t *testing.T) {
	// Test conversion of nested arrays ([]interface{} containing arrays)
	innerArray1 := []*big.Int{
		big.NewInt(1000000000000000000),
		big.NewInt(2000000000000000000),
	}
	innerArray2 := []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001"),
		common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000002"),
	}

	testValues := []interface{}{
		innerArray1,
		innerArray2,
	}

	result := convertGoTypeToJsValueForTest(testValues)

	resultArray, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	if len(resultArray) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(resultArray))
	}

	// Check first nested array ([]*big.Int)
	firstArray, ok := resultArray[0].([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{} for first element, got %T", resultArray[0])
	}

	if len(firstArray) != 2 {
		t.Fatalf("Expected 2 elements in first array, got %d", len(firstArray))
	}

	numStr1, ok := firstArray[0].(string)
	if !ok {
		t.Fatalf("Expected string, got %T", firstArray[0])
	}
	expectedNum1 := innerArray1[0].String()
	if numStr1 != expectedNum1 {
		t.Errorf("First nested big.Int mismatch: expected %s, got %s", expectedNum1, numStr1)
	}

	// Check second nested array ([]common.Address)
	secondArray, ok := resultArray[1].([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{} for second element, got %T", resultArray[1])
	}

	if len(secondArray) != 2 {
		t.Fatalf("Expected 2 elements in second array, got %d", len(secondArray))
	}

	addrStr1, ok := secondArray[0].(string)
	if !ok {
		t.Fatalf("Expected string, got %T", secondArray[0])
	}
	expectedAddr1 := innerArray2[0].String()
	if addrStr1 != expectedAddr1 {
		t.Errorf("First nested address mismatch: expected %s, got %s", expectedAddr1, addrStr1)
	}
}

// Testable version of PackCreateContractData for non-WASM testing
func packCreateContractDataForTest(abiJSON string, bytecode string, args []interface{}) ([]byte, error) {
	// Validate abiJSON is not empty
	if strings.TrimSpace(abiJSON) == "" {
		return nil, fmt.Errorf("abiJSON cannot be empty")
	}

	// Validate bytecode is not empty
	if strings.TrimSpace(bytecode) == "" {
		return nil, fmt.Errorf("bytecode cannot be empty")
	}

	// Decode bytecode from hex string
	bytecodeBytes, err := hexutil.Decode(bytecode)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bytecode: %w", err)
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI JSON: %w", err)
	}

	// Check if constructor exists in ABI
	constructorInputs := abiData.Constructor.Inputs

	// Validate argument count matches constructor inputs
	if len(args) != len(constructorInputs) {
		return nil, fmt.Errorf("argument count mismatch: expected %d, got %d", len(constructorInputs), len(args))
	}

	var constructorData []byte

	// If no arguments (parameterless constructor), call Pack with empty string and no arguments
	if len(args) == 0 {
		constructorData, err = abiData.Pack("")
	} else {
		// Convert arguments to Go types based on constructor input types
		convertedArgs := make([]interface{}, len(args))
		for i, arg := range args {
			converted, err := convertValueToGoTypeForTest(arg, constructorInputs[i].Type)
			if err != nil {
				return nil, fmt.Errorf("failed to convert argument %d: %w", i, err)
			}
			// Convert []interface{} to proper typed slice if needed
			converted = convertToTypedSliceForTest(converted, constructorInputs[i].Type)
			convertedArgs[i] = converted
		}
		// Use empty string "" as method name for constructor
		constructorData, err = abiData.Pack("", convertedArgs...)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to pack constructor: %w", err)
	}

	// Combine bytecode with constructor data: bytecode + constructor parameters
	finalData := append(bytecodeBytes, constructorData...)

	return finalData, nil
}

func TestPackCreateContractData_ParameterlessConstructor(t *testing.T) {
	// Test packing a parameterless constructor
	abiJSON := `[{
		"type": "constructor",
		"inputs": [],
		"stateMutability": "nonpayable"
	}]`

	bytecode := "0x6080604052348015600f57600080fd5b506004361060325760003560e01c8063"

	// Pack with no constructor arguments
	packed, err := packCreateContractDataForTest(abiJSON, bytecode, []interface{}{})
	if err != nil {
		t.Fatalf("Failed to pack parameterless constructor: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify bytecode is at the beginning
	expectedBytecode, _ := hexutil.Decode(bytecode)
	if len(packed) < len(expectedBytecode) {
		t.Fatal("Packed data should contain bytecode")
	}

	if !reflect.DeepEqual(packed[:len(expectedBytecode)], expectedBytecode) {
		t.Errorf("Bytecode mismatch: expected %x, got %x", expectedBytecode, packed[:len(expectedBytecode)])
	}

	// For parameterless constructor, there should be no additional data after bytecode
	if len(packed) != len(expectedBytecode) {
		t.Errorf("Expected packed data length %d, got %d (parameterless constructor should only have bytecode)", len(expectedBytecode), len(packed))
	}
}

func TestPackCreateContractData_ConstructorWithParameters(t *testing.T) {
	// Test packing a constructor with parameters
	abiJSON := `[{
		"type": "constructor",
		"inputs": [
			{"name": "a", "type": "uint256"},
			{"name": "b", "type": "uint256"}
		],
		"stateMutability": "nonpayable"
	}]`

	bytecode := "0x6080604052348015600f57600080fd5b506004361060325760003560e01c8063"

	// Pack with constructor arguments
	packed, err := packCreateContractDataForTest(abiJSON, bytecode, []interface{}{
		"1", // a
		"2", // b
	})
	if err != nil {
		t.Fatalf("Failed to pack constructor with parameters: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify bytecode is at the beginning
	expectedBytecode, _ := hexutil.Decode(bytecode)
	if len(packed) < len(expectedBytecode) {
		t.Fatal("Packed data should contain bytecode")
	}

	if !reflect.DeepEqual(packed[:len(expectedBytecode)], expectedBytecode) {
		t.Errorf("Bytecode mismatch: expected %x, got %x", expectedBytecode, packed[:len(expectedBytecode)])
	}

	// Verify constructor data follows bytecode
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	expectedConstructorData, _ := abiData.Pack("", big.NewInt(1), big.NewInt(2))
	if len(packed) != len(expectedBytecode)+len(expectedConstructorData) {
		t.Errorf("Expected packed data length %d, got %d", len(expectedBytecode)+len(expectedConstructorData), len(packed))
	}

	if !reflect.DeepEqual(packed[len(expectedBytecode):], expectedConstructorData) {
		t.Errorf("Constructor data mismatch: expected %x, got %x", expectedConstructorData, packed[len(expectedBytecode):])
	}
}

func TestPackCreateContractData_EmptyAbiJSON(t *testing.T) {
	bytecode := "0x6080604052348015600f57600080fd5b506004361060325760003560e01c8063"

	// Test with empty abiJSON
	_, err := packCreateContractDataForTest("", bytecode, []interface{}{})
	if err == nil {
		t.Fatal("Expected error for empty abiJSON, got nil")
	}

	if !strings.Contains(err.Error(), "abiJSON cannot be empty") {
		t.Errorf("Expected 'abiJSON cannot be empty' error, got: %v", err)
	}

	// Test with whitespace-only abiJSON
	_, err = packCreateContractDataForTest("   ", bytecode, []interface{}{})
	if err == nil {
		t.Fatal("Expected error for whitespace-only abiJSON, got nil")
	}

	if !strings.Contains(err.Error(), "abiJSON cannot be empty") {
		t.Errorf("Expected 'abiJSON cannot be empty' error, got: %v", err)
	}
}

func TestPackCreateContractData_EmptyBytecode(t *testing.T) {
	abiJSON := `[{
		"type": "constructor",
		"inputs": [],
		"stateMutability": "nonpayable"
	}]`

	// Test with empty bytecode
	_, err := packCreateContractDataForTest(abiJSON, "", []interface{}{})
	if err == nil {
		t.Fatal("Expected error for empty bytecode, got nil")
	}

	if !strings.Contains(err.Error(), "bytecode cannot be empty") {
		t.Errorf("Expected 'bytecode cannot be empty' error, got: %v", err)
	}

	// Test with whitespace-only bytecode
	_, err = packCreateContractDataForTest(abiJSON, "   ", []interface{}{})
	if err == nil {
		t.Fatal("Expected error for whitespace-only bytecode, got nil")
	}

	if !strings.Contains(err.Error(), "bytecode cannot be empty") {
		t.Errorf("Expected 'bytecode cannot be empty' error, got: %v", err)
	}
}

func TestPackCreateContractData_InvalidBytecode(t *testing.T) {
	abiJSON := `[{
		"type": "constructor",
		"inputs": [],
		"stateMutability": "nonpayable"
	}]`

	// Test with invalid hex bytecode
	_, err := packCreateContractDataForTest(abiJSON, "0xinvalid", []interface{}{})
	if err == nil {
		t.Fatal("Expected error for invalid bytecode, got nil")
	}

	if !strings.Contains(err.Error(), "failed to decode bytecode") {
		t.Errorf("Expected 'failed to decode bytecode' error, got: %v", err)
	}
}

func TestPackCreateContractData_ArgumentCountMismatch(t *testing.T) {
	abiJSON := `[{
		"type": "constructor",
		"inputs": [
			{"name": "a", "type": "uint256"},
			{"name": "b", "type": "uint256"}
		],
		"stateMutability": "nonpayable"
	}]`

	bytecode := "0x6080604052348015600f57600080fd5b506004361060325760003560e01c8063"

	// Test with too few arguments
	_, err := packCreateContractDataForTest(abiJSON, bytecode, []interface{}{"1"})
	if err == nil {
		t.Fatal("Expected error for too few arguments, got nil")
	}

	if !strings.Contains(err.Error(), "argument count mismatch") {
		t.Errorf("Expected 'argument count mismatch' error, got: %v", err)
	}

	// Test with too many arguments
	_, err = packCreateContractDataForTest(abiJSON, bytecode, []interface{}{"1", "2", "3"})
	if err == nil {
		t.Fatal("Expected error for too many arguments, got nil")
	}

	if !strings.Contains(err.Error(), "argument count mismatch") {
		t.Errorf("Expected 'argument count mismatch' error, got: %v", err)
	}
}

func TestPackCreateContractData_ConstructorWithAddress(t *testing.T) {
	// Test packing a constructor with address parameter
	abiJSON := `[{
		"type": "constructor",
		"inputs": [
			{"name": "owner", "type": "address"}
		],
		"stateMutability": "nonpayable"
	}]`

	bytecode := "0x6080604052348015600f57600080fd5b506004361060325760003560e01c8063"
	ownerAddr := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")

	// Pack with address argument
	packed, err := packCreateContractDataForTest(abiJSON, bytecode, []interface{}{
		ownerAddr.String(),
	})
	if err != nil {
		t.Fatalf("Failed to pack constructor with address: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify bytecode is at the beginning
	expectedBytecode, _ := hexutil.Decode(bytecode)
	if !reflect.DeepEqual(packed[:len(expectedBytecode)], expectedBytecode) {
		t.Errorf("Bytecode mismatch")
	}

	// Verify constructor data follows bytecode
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	expectedConstructorData, _ := abiData.Pack("", ownerAddr)
	if !reflect.DeepEqual(packed[len(expectedBytecode):], expectedConstructorData) {
		t.Errorf("Constructor data mismatch")
	}
}

func TestPackCreateContractData_ConstructorWithArray(t *testing.T) {
	// Test packing a constructor with array parameter
	abiJSON := `[{
		"type": "constructor",
		"inputs": [
			{"name": "values", "type": "uint256[]"}
		],
		"stateMutability": "nonpayable"
	}]`

	bytecode := "0x6080604052348015600f57600080fd5b506004361060325760003560e01c8063"

	// Pack with array argument
	testValues := []interface{}{
		"1000000000000000000", // 1 token
		"2000000000000000000", // 2 tokens
		"3000000000000000000", // 3 tokens
	}

	packed, err := packCreateContractDataForTest(abiJSON, bytecode, []interface{}{
		testValues,
	})
	if err != nil {
		t.Fatalf("Failed to pack constructor with array: %v", err)
	}

	if len(packed) == 0 {
		t.Fatal("Packed data should not be empty")
	}

	// Verify bytecode is at the beginning
	expectedBytecode, _ := hexutil.Decode(bytecode)
	if !reflect.DeepEqual(packed[:len(expectedBytecode)], expectedBytecode) {
		t.Errorf("Bytecode mismatch")
	}

	// Verify constructor data follows bytecode
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	expectedValues := []*big.Int{
		big.NewInt(1000000000000000000),
		big.NewInt(2000000000000000000),
		big.NewInt(3000000000000000000),
	}
	expectedConstructorData, _ := abiData.Pack("", expectedValues)
	if !reflect.DeepEqual(packed[len(expectedBytecode):], expectedConstructorData) {
		t.Errorf("Constructor data mismatch")
	}
}

// Testable version of CreateAddress for non-WASM testing
func createAddressForTest(address common.Address, nonce uint64) common.Address {
	return crypto.CreateAddress(address, nonce)
}

// Testable version of CreateAddress2 for non-WASM testing
func createAddress2ForTest(address common.Address, salt [common.HashLength]byte, initHash []byte) common.Address {
	return crypto.CreateAddress2(address, salt, initHash)
}

func TestCreateAddress_Basic(t *testing.T) {
	// Test basic CreateAddress functionality
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	nonce := uint64(0)

	result := createAddressForTest(address, nonce)
	if result == (common.Address{}) {
		t.Fatal("CreateAddress should not return zero address")
	}

	// Test with different nonce
	nonce2 := uint64(1)
	result2 := createAddressForTest(address, nonce2)
	if result == result2 {
		t.Error("CreateAddress should produce different addresses for different nonces")
	}
}

func TestCreateAddress_WithNonce(t *testing.T) {
	// Test CreateAddress with various nonces
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")

	testCases := []struct {
		nonce uint64
		desc  string
	}{
		{0, "nonce 0"},
		{1, "nonce 1"},
		{42, "nonce 42"},
		{100, "nonce 100"},
		{1000, "nonce 1000"},
	}

	results := make(map[uint64]common.Address)
	for _, tc := range testCases {
		result := createAddressForTest(address, tc.nonce)
		if result == (common.Address{}) {
			t.Errorf("CreateAddress with %s should not return zero address", tc.desc)
		}
		results[tc.nonce] = result
	}

	// Verify all results are unique
	for i := 0; i < len(testCases); i++ {
		for j := i + 1; j < len(testCases); j++ {
			if results[testCases[i].nonce] == results[testCases[j].nonce] {
				t.Errorf("CreateAddress should produce different addresses for nonces %d and %d", testCases[i].nonce, testCases[j].nonce)
			}
		}
	}
}

func TestCreateAddress_DifferentAddresses(t *testing.T) {
	// Test that different base addresses produce different results
	nonce := uint64(0)

	address1 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	address2 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000002")

	result1 := createAddressForTest(address1, nonce)
	result2 := createAddressForTest(address2, nonce)

	if result1 == result2 {
		t.Error("CreateAddress should produce different addresses for different base addresses")
	}
}

func TestCreateAddress_Consistency(t *testing.T) {
	// Test that CreateAddress produces consistent results for same inputs
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	nonce := uint64(42)

	result1 := createAddressForTest(address, nonce)
	result2 := createAddressForTest(address, nonce)

	if result1 != result2 {
		t.Error("CreateAddress should produce the same address for the same inputs")
	}
}

func TestCreateAddress2_Basic(t *testing.T) {
	// Test basic CreateAddress2 functionality
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	var salt [common.HashLength]byte
	copy(salt[:], common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001").Bytes())
	initHash := crypto.Keccak256([]byte("test init code"))

	result := createAddress2ForTest(address, salt, initHash)
	if result == (common.Address{}) {
		t.Fatal("CreateAddress2 should not return zero address")
	}
}

func TestCreateAddress2_WithDifferentSalts(t *testing.T) {
	// Test CreateAddress2 with different salts
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	initHash := crypto.Keccak256([]byte("test init code"))

	salts := []string{
		"0x0000000000000000000000000000000000000000000000000000000000000001",
		"0x0000000000000000000000000000000000000000000000000000000000000002",
		"0xcafebabe00000000000000000000000000000000000000000000000000000000",
		"0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	results := make(map[string]common.Address)
	for _, saltStr := range salts {
		saltHash := common.HexToHash(saltStr)
		var salt [common.HashLength]byte
		copy(salt[:], saltHash.Bytes())

		result := createAddress2ForTest(address, salt, initHash)
		if result == (common.Address{}) {
			t.Errorf("CreateAddress2 with salt %s should not return zero address", saltStr)
		}
		results[saltStr] = result
	}

	// Verify all results are unique
	saltList := make([]string, 0, len(salts))
	for k := range results {
		saltList = append(saltList, k)
	}
	for i := 0; i < len(saltList); i++ {
		for j := i + 1; j < len(saltList); j++ {
			if results[saltList[i]] == results[saltList[j]] {
				t.Errorf("CreateAddress2 should produce different addresses for salts %s and %s", saltList[i], saltList[j])
			}
		}
	}
}

func TestCreateAddress2_WithDifferentInitHashes(t *testing.T) {
	// Test CreateAddress2 with different init hashes
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	var salt [common.HashLength]byte
	copy(salt[:], common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001").Bytes())

	initHashes := [][]byte{
		crypto.Keccak256([]byte("init code 1")),
		crypto.Keccak256([]byte("init code 2")),
		crypto.Keccak256([]byte("different init code")),
		[]byte{0x00, 0x01, 0x02, 0x03},
	}

	results := make(map[string]common.Address)
	for i, initHash := range initHashes {
		result := createAddress2ForTest(address, salt, initHash)
		if result == (common.Address{}) {
			t.Errorf("CreateAddress2 with initHash %d should not return zero address", i)
		}
		key := hexutil.Encode(initHash)
		results[key] = result
	}

	// Verify all results are unique
	hashList := make([]string, 0, len(initHashes))
	for k := range results {
		hashList = append(hashList, k)
	}
	for i := 0; i < len(hashList); i++ {
		for j := i + 1; j < len(hashList); j++ {
			if results[hashList[i]] == results[hashList[j]] {
				t.Errorf("CreateAddress2 should produce different addresses for different init hashes")
			}
		}
	}
}

func TestCreateAddress2_WithDifferentAddresses(t *testing.T) {
	// Test that different base addresses produce different results
	var salt [common.HashLength]byte
	copy(salt[:], common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001").Bytes())
	initHash := crypto.Keccak256([]byte("test init code"))

	address1 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	address2 := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000002")

	result1 := createAddress2ForTest(address1, salt, initHash)
	result2 := createAddress2ForTest(address2, salt, initHash)

	if result1 == result2 {
		t.Error("CreateAddress2 should produce different addresses for different base addresses")
	}
}

func TestCreateAddress2_Consistency(t *testing.T) {
	// Test that CreateAddress2 produces consistent results for same inputs
	address := common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000001")
	var salt [common.HashLength]byte
	copy(salt[:], common.HexToHash("0xcafebabe00000000000000000000000000000000000000000000000000000000").Bytes())
	initHash := crypto.Keccak256([]byte("consistent test init code"))

	result1 := createAddress2ForTest(address, salt, initHash)
	result2 := createAddress2ForTest(address, salt, initHash)

	if result1 != result2 {
		t.Error("CreateAddress2 should produce the same address for the same inputs")
	}
}

// encodeEventLogForTest is a testable version of EncodeEventLog
func encodeEventLogForTest(abiJSON string, eventName string, values []interface{}) ([]string, string, error) {
	// Validate inputs
	if strings.TrimSpace(abiJSON) == "" {
		return nil, "", fmt.Errorf("abiJSON cannot be empty")
	}
	if strings.TrimSpace(eventName) == "" {
		return nil, "", fmt.Errorf("eventName cannot be empty")
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse ABI JSON: %v", err)
	}

	// Get event from ABI
	event, exist := abiData.Events[eventName]
	if !exist {
		return nil, "", fmt.Errorf("event '%s' not found in ABI", eventName)
	}

	// Validate argument count
	if len(values) != len(event.Inputs) {
		return nil, "", fmt.Errorf("argument count mismatch: expected %d, got %d", len(event.Inputs), len(values))
	}

	// Separate indexed and non-indexed arguments
	var indexedArgs []interface{}
	var indexedTypes []abi.Argument
	var nonIndexedArgs []interface{}
	var nonIndexedTypes []abi.Argument

	for i, input := range event.Inputs {
		converted, err := convertValueToGoTypeForTest(values[i], input.Type)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert argument %d: %v", i, err)
		}
		converted = convertToTypedSliceForTest(converted, input.Type)

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
		topic, err := packIndexedValueForTest(indexedArg, indexedTypes[i].Type)
		if err != nil {
			return nil, "", fmt.Errorf("failed to pack indexed argument %d: %v", i, err)
		}
		topics = append(topics, common.BytesToHash(topic).Hex())
	}

	// Pack non-indexed parameters into data
	var dataBytes []byte
	if len(nonIndexedArgs) > 0 {
		nonIndexedInputs := abi.Arguments(nonIndexedTypes)
		dataBytes, err = nonIndexedInputs.Pack(nonIndexedArgs...)
		if err != nil {
			return nil, "", fmt.Errorf("failed to pack non-indexed arguments: %v", err)
		}
	}

	dataStr := hexutil.Encode(dataBytes)
	return topics, dataStr, nil
}

// decodeEventLogForTest is a testable version of DecodeEventLog
func decodeEventLogForTest(abiJSON string, eventName string, topics []string, data string) (map[string]interface{}, error) {
	// Validate inputs
	if strings.TrimSpace(abiJSON) == "" {
		return nil, fmt.Errorf("abiJSON cannot be empty")
	}
	if strings.TrimSpace(eventName) == "" {
		return nil, fmt.Errorf("eventName cannot be empty")
	}

	// Parse ABI JSON
	abiData, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI JSON: %v", err)
	}

	// Get event from ABI
	event, exist := abiData.Events[eventName]
	if !exist {
		return nil, fmt.Errorf("event '%s' not found in ABI", eventName)
	}

	// Parse topics array
	if len(topics) == 0 {
		return nil, fmt.Errorf("topics array cannot be empty")
	}

	// Extract topic hashes
	topicHashes := make([]common.Hash, len(topics))
	for i, topicStr := range topics {
		if !strings.HasPrefix(topicStr, "0x") && !strings.HasPrefix(topicStr, "0X") {
			topicStr = "0x" + topicStr
		}
		topicHash := common.HexToHash(topicStr)
		topicHashes[i] = topicHash
	}

	// Verify first topic matches event ID (unless anonymous)
	if !event.Anonymous {
		if len(topicHashes) == 0 || topicHashes[0] != event.ID {
			return nil, fmt.Errorf("first topic does not match event signature")
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
			return nil, fmt.Errorf("insufficient topics for indexed parameters")
		}

		for i, indexedInput := range indexedInputs {
			topicHash := topicHashes[indexedStart+i]
			decoded, err := unpackIndexedValueForTest(topicHash, indexedInput.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to unpack indexed parameter '%s': %v", indexedInput.Name, err)
			}
			resultMap[indexedInput.Name] = convertGoTypeToJsValueForTest(decoded)
		}
	}

	// Decode non-indexed parameters from data
	if len(nonIndexedInputs) > 0 {
		if data == "" || data == "0x" {
			// Empty data, return default values
			for _, nonIndexedInput := range nonIndexedInputs {
				resultMap[nonIndexedInput.Name] = convertGoTypeToJsValueForTest(getDefaultValueForTest(nonIndexedInput.Type))
			}
		} else {
			if !strings.HasPrefix(data, "0x") && !strings.HasPrefix(data, "0X") {
				data = "0x" + data
			}
			dataBytes, err := hexutil.Decode(data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode data: %v", err)
			}

			nonIndexedArgs := abi.Arguments(nonIndexedInputs)
			unpacked, err := nonIndexedArgs.Unpack(dataBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to unpack non-indexed parameters: %v", err)
			}

			for i, nonIndexedInput := range nonIndexedInputs {
				resultMap[nonIndexedInput.Name] = convertGoTypeToJsValueForTest(unpacked[i])
			}
		}
	}

	return resultMap, nil
}

// packIndexedValueForTest packs an indexed event parameter value into a 32-byte topic
func packIndexedValueForTest(value interface{}, argType abi.Type) ([]byte, error) {
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
		// Value types: use first 32 bytes
		if len(packed) < 32 {
			padded := make([]byte, 32)
			copy(padded[32-len(packed):], packed)
			return padded, nil
		}
		return packed[:32], nil
	}
}

// unpackIndexedValueForTest unpacks an indexed topic back into a value
func unpackIndexedValueForTest(topic common.Hash, argType abi.Type) (interface{}, error) {
	switch argType.T {
	case abi.StringTy, abi.BytesTy, abi.SliceTy, abi.ArrayTy:
		// Dynamic types: return the hash (cannot reconstruct original)
		return topic, nil
	default:
		// Value types: unpack from topic bytes
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

// getDefaultValueForTest returns a default value for a given ABI type
func getDefaultValueForTest(argType abi.Type) interface{} {
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

func TestEncodeEventLog_Basic(t *testing.T) {
	// Test encoding a simple event with indexed and non-indexed parameters
	abiJSON := `[{
		"name": "Transfer",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "from", "type": "address", "indexed": true},
			{"name": "to", "type": "address", "indexed": true},
			{"name": "value", "type": "uint256", "indexed": false}
		]
	}]`

	from := "0x0000000000000000000000000000000000000000000000000000000000000001"
	to := "0x0000000000000000000000000000000000000000000000000000000000000002"
	value := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "Transfer", []interface{}{from, to, value})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	// Should have 3 topics: event signature + 2 indexed parameters
	if len(topics) != 3 {
		t.Fatalf("Expected 3 topics, got %d", len(topics))
	}

	// Verify event signature topic
	abiData, _ := abi.JSON(strings.NewReader(abiJSON))
	event := abiData.Events["Transfer"]
	if topics[0] != event.ID.Hex() {
		t.Errorf("First topic should be event signature, expected %s, got %s", event.ID.Hex(), topics[0])
	}

	// Verify data is not empty
	if data == "" || data == "0x" {
		t.Error("Data should not be empty for non-indexed parameters")
	}
}

func TestEncodeEventLog_AllIndexed(t *testing.T) {
	// Test encoding an event with all indexed parameters
	abiJSON := `[{
		"name": "Approval",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "owner", "type": "address", "indexed": true},
			{"name": "spender", "type": "address", "indexed": true},
			{"name": "value", "type": "uint256", "indexed": true}
		]
	}]`

	owner := "0x0000000000000000000000000000000000000000000000000000000000000001"
	spender := "0x0000000000000000000000000000000000000000000000000000000000000002"
	value := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "Approval", []interface{}{owner, spender, value})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	// Should have 4 topics: event signature + 3 indexed parameters
	if len(topics) != 4 {
		t.Fatalf("Expected 4 topics, got %d", len(topics))
	}

	// Data should be empty (0x) since all parameters are indexed
	if data != "0x" {
		t.Errorf("Data should be empty for all-indexed event, got %s", data)
	}
}

func TestEncodeEventLog_AllNonIndexed(t *testing.T) {
	// Test encoding an event with all non-indexed parameters
	abiJSON := `[{
		"name": "Received",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "sender", "type": "address", "indexed": false},
			{"name": "amount", "type": "uint256", "indexed": false}
		]
	}]`

	sender := "0x0000000000000000000000000000000000000000000000000000000000000001"
	amount := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "Received", []interface{}{sender, amount})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	// Should have 1 topic: event signature only
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic, got %d", len(topics))
	}

	// Data should not be empty
	if data == "" || data == "0x" {
		t.Error("Data should not be empty for non-indexed parameters")
	}
}

func TestEncodeEventLog_Anonymous(t *testing.T) {
	// Test encoding an anonymous event
	abiJSON := `[{
		"name": "AnonymousEvent",
		"type": "event",
		"anonymous": true,
		"inputs": [
			{"name": "value", "type": "uint256", "indexed": true}
		]
	}]`

	value := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "AnonymousEvent", []interface{}{value})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	// Should have 1 topic: only the indexed parameter (no event signature)
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic for anonymous event, got %d", len(topics))
	}

	// Data should be empty
	if data != "0x" {
		t.Errorf("Data should be empty, got %s", data)
	}
}

func TestEncodeEventLog_StringIndexed(t *testing.T) {
	// Test encoding an event with indexed string (should be hashed)
	abiJSON := `[{
		"name": "StringEvent",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "message", "type": "string", "indexed": true}
		]
	}]`

	message := "Hello, World!"

	topics, _, err := encodeEventLogForTest(abiJSON, "StringEvent", []interface{}{message})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	// Should have 2 topics: event signature + hashed string
	if len(topics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(topics))
	}

	// Verify the string is hashed (topic should be Keccak256 hash of ABI-packed string)
	// For indexed strings, we hash the ABI-packed value, not raw bytes
	stringType := createStringType()
	args := abi.Arguments{abi.Argument{Type: stringType}}
	packed, err := args.Pack(message)
	if err != nil {
		t.Fatalf("Failed to pack string: %v", err)
	}
	expectedHash := crypto.Keccak256Hash(packed)
	if topics[1] != expectedHash.Hex() {
		t.Errorf("String should be hashed, expected %s, got %s", expectedHash.Hex(), topics[1])
	}
}

func TestDecodeEventLog_Basic(t *testing.T) {
	// Test decoding a simple event
	abiJSON := `[{
		"name": "Transfer",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "from", "type": "address", "indexed": true},
			{"name": "to", "type": "address", "indexed": true},
			{"name": "value", "type": "uint256", "indexed": false}
		]
	}]`

	// First encode the event
	from := "0x0000000000000000000000000000000000000000000000000000000000000001"
	to := "0x0000000000000000000000000000000000000000000000000000000000000002"
	value := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "Transfer", []interface{}{from, to, value})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	// Then decode it
	decoded, err := decodeEventLogForTest(abiJSON, "Transfer", topics, data)
	if err != nil {
		t.Fatalf("DecodeEventLog failed: %v", err)
	}

	// Verify decoded values
	if decoded["from"] != from {
		t.Errorf("Decoded 'from' mismatch: expected %s, got %v", from, decoded["from"])
	}
	if decoded["to"] != to {
		t.Errorf("Decoded 'to' mismatch: expected %s, got %v", to, decoded["to"])
	}
	// Value is non-indexed, so it's in data
	decodedValue, ok := decoded["value"].(string)
	if !ok {
		t.Fatalf("Decoded 'value' should be string, got %T", decoded["value"])
	}
	if decodedValue != value {
		t.Errorf("Decoded 'value' mismatch: expected %s, got %s", value, decodedValue)
	}
}

func TestDecodeEventLog_AllIndexed(t *testing.T) {
	// Test decoding an event with all indexed parameters
	abiJSON := `[{
		"name": "Approval",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "owner", "type": "address", "indexed": true},
			{"name": "spender", "type": "address", "indexed": true},
			{"name": "value", "type": "uint256", "indexed": true}
		]
	}]`

	owner := "0x0000000000000000000000000000000000000000000000000000000000000001"
	spender := "0x0000000000000000000000000000000000000000000000000000000000000002"
	value := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "Approval", []interface{}{owner, spender, value})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	decoded, err := decodeEventLogForTest(abiJSON, "Approval", topics, data)
	if err != nil {
		t.Fatalf("DecodeEventLog failed: %v", err)
	}

	// Verify all indexed parameters are decoded correctly
	if decoded["owner"] != owner {
		t.Errorf("Decoded 'owner' mismatch: expected %s, got %v", owner, decoded["owner"])
	}
	if decoded["spender"] != spender {
		t.Errorf("Decoded 'spender' mismatch: expected %s, got %v", spender, decoded["spender"])
	}
	decodedValue, ok := decoded["value"].(string)
	if !ok {
		t.Fatalf("Decoded 'value' should be string, got %T", decoded["value"])
	}
	if decodedValue != value {
		t.Errorf("Decoded 'value' mismatch: expected %s, got %s", value, decodedValue)
	}
}

func TestDecodeEventLog_Anonymous(t *testing.T) {
	// Test decoding an anonymous event
	abiJSON := `[{
		"name": "AnonymousEvent",
		"type": "event",
		"anonymous": true,
		"inputs": [
			{"name": "value", "type": "uint256", "indexed": true}
		]
	}]`

	value := "1000000000000000000"

	topics, data, err := encodeEventLogForTest(abiJSON, "AnonymousEvent", []interface{}{value})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	decoded, err := decodeEventLogForTest(abiJSON, "AnonymousEvent", topics, data)
	if err != nil {
		t.Fatalf("DecodeEventLog failed: %v", err)
	}

	decodedValue, ok := decoded["value"].(string)
	if !ok {
		t.Fatalf("Decoded 'value' should be string, got %T", decoded["value"])
	}
	if decodedValue != value {
		t.Errorf("Decoded 'value' mismatch: expected %s, got %s", value, decodedValue)
	}
}

func TestDecodeEventLog_StringIndexed(t *testing.T) {
	// Test decoding an event with indexed string (returns hash, not original value)
	abiJSON := `[{
		"name": "StringEvent",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "message", "type": "string", "indexed": true}
		]
	}]`

	message := "Hello, World!"

	topics, data, err := encodeEventLogForTest(abiJSON, "StringEvent", []interface{}{message})
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	decoded, err := decodeEventLogForTest(abiJSON, "StringEvent", topics, data)
	if err != nil {
		t.Fatalf("DecodeEventLog failed: %v", err)
	}

	// For indexed strings, we get the hash back (as a string), not the original value
	// convertGoTypeToJsValueForTest converts common.Hash to string via .Hex()
	decodedHashStr, ok := decoded["message"].(string)
	if !ok {
		t.Fatalf("Decoded 'message' should be string (hash hex), got %T", decoded["message"])
	}

	// Calculate expected hash (ABI-packed string, then hashed)
	stringType := createStringType()
	args := abi.Arguments{abi.Argument{Type: stringType}}
	packed, err := args.Pack(message)
	if err != nil {
		t.Fatalf("Failed to pack string: %v", err)
	}
	expectedHash := crypto.Keccak256Hash(packed)
	if decodedHashStr != expectedHash.Hex() {
		t.Errorf("Decoded hash mismatch: expected %s, got %s", expectedHash.Hex(), decodedHashStr)
	}
}

func TestEncodeEventLog_InvalidEventName(t *testing.T) {
	abiJSON := `[{
		"name": "Transfer",
		"type": "event",
		"anonymous": false,
		"inputs": []
	}]`

	_, _, err := encodeEventLogForTest(abiJSON, "NonExistentEvent", []interface{}{})
	if err == nil {
		t.Fatal("Expected error for non-existent event, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestEncodeEventLog_ArgumentMismatch(t *testing.T) {
	abiJSON := `[{
		"name": "Transfer",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "from", "type": "address", "indexed": true},
			{"name": "to", "type": "address", "indexed": true}
		]
	}]`

	_, _, err := encodeEventLogForTest(abiJSON, "Transfer", []interface{}{"0x01"})
	if err == nil {
		t.Fatal("Expected error for argument count mismatch, got nil")
	}

	if !strings.Contains(err.Error(), "argument count mismatch") {
		t.Errorf("Expected 'argument count mismatch' error, got: %v", err)
	}
}

func TestDecodeEventLog_InvalidEventName(t *testing.T) {
	abiJSON := `[{
		"name": "Transfer",
		"type": "event",
		"anonymous": false,
		"inputs": []
	}]`

	_, err := decodeEventLogForTest(abiJSON, "NonExistentEvent", []string{"0x00"}, "0x")
	if err == nil {
		t.Fatal("Expected error for non-existent event, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestDecodeEventLog_InvalidTopicSignature(t *testing.T) {
	abiJSON := `[{
		"name": "Transfer",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "from", "type": "address", "indexed": true}
		]
	}]`

	// Use wrong event signature in first topic
	wrongTopics := []string{"0x0000000000000000000000000000000000000000000000000000000000000000", "0x00"}

	_, err := decodeEventLogForTest(abiJSON, "Transfer", wrongTopics, "0x")
	if err == nil {
		t.Fatal("Expected error for invalid topic signature, got nil")
	}

	if !strings.Contains(err.Error(), "does not match event signature") {
		t.Errorf("Expected 'does not match event signature' error, got: %v", err)
	}
}

func TestEncodeDecodeEventLog_RoundTrip(t *testing.T) {
	// Test that encoding and decoding produces the same values
	abiJSON := `[{
		"name": "ComplexEvent",
		"type": "event",
		"anonymous": false,
		"inputs": [
			{"name": "indexedAddr", "type": "address", "indexed": true},
			{"name": "indexedValue", "type": "uint256", "indexed": true},
			{"name": "nonIndexedAddr", "type": "address", "indexed": false},
			{"name": "nonIndexedValue", "type": "uint256", "indexed": false},
			{"name": "flag", "type": "bool", "indexed": false}
		]
	}]`

	indexedAddr := "0x0000000000000000000000000000000000000000000000000000000000000001"
	indexedValue := "1000000000000000000"
	nonIndexedAddr := "0x0000000000000000000000000000000000000000000000000000000000000002"
	nonIndexedValue := "2000000000000000000"
	flag := true

	values := []interface{}{indexedAddr, indexedValue, nonIndexedAddr, nonIndexedValue, flag}

	topics, data, err := encodeEventLogForTest(abiJSON, "ComplexEvent", values)
	if err != nil {
		t.Fatalf("EncodeEventLog failed: %v", err)
	}

	decoded, err := decodeEventLogForTest(abiJSON, "ComplexEvent", topics, data)
	if err != nil {
		t.Fatalf("DecodeEventLog failed: %v", err)
	}

	// Verify all values match
	if decoded["indexedAddr"] != indexedAddr {
		t.Errorf("indexedAddr mismatch: expected %s, got %v", indexedAddr, decoded["indexedAddr"])
	}
	decodedIndexedValue, _ := decoded["indexedValue"].(string)
	if decodedIndexedValue != indexedValue {
		t.Errorf("indexedValue mismatch: expected %s, got %s", indexedValue, decodedIndexedValue)
	}
	if decoded["nonIndexedAddr"] != nonIndexedAddr {
		t.Errorf("nonIndexedAddr mismatch: expected %s, got %v", nonIndexedAddr, decoded["nonIndexedAddr"])
	}
	decodedNonIndexedValue, _ := decoded["nonIndexedValue"].(string)
	if decodedNonIndexedValue != nonIndexedValue {
		t.Errorf("nonIndexedValue mismatch: expected %s, got %s", nonIndexedValue, decodedNonIndexedValue)
	}
	decodedFlag, _ := decoded["flag"].(bool)
	if decodedFlag != flag {
		t.Errorf("flag mismatch: expected %v, got %v", flag, decodedFlag)
	}
}

// encodeRlpForTest is a test helper that encodes a value to RLP
// It works directly with Go types to avoid js.Value mocking issues
func encodeRlpForTest(value interface{}) (string, error) {
	// Convert Go value to RLP-encodable type
	goValue, err := convertGoValueToRlpType(value)
	if err != nil {
		return "", err
	}

	// Encode to RLP bytes
	rlpBytes, err := rlp.EncodeToBytes(goValue)
	if err != nil {
		return "", fmt.Errorf("failed to encode: %v", err)
	}

	// Return as hex string
	return hexutil.Encode(rlpBytes), nil
}

// convertGoValueToRlpType converts a Go value to a type suitable for RLP encoding
func convertGoValueToRlpType(val interface{}) (interface{}, error) {
	if val == nil {
		return []byte{}, nil
	}

	switch v := val.(type) {
	case string:
		// Check if it's a hex string
		if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
			bytes, err := hexutil.Decode(v)
			if err != nil {
				return nil, fmt.Errorf("invalid hex string: %v", err)
			}
			return bytes, nil
		}
		return v, nil
	case float64:
		// Check if it's an integer
		if v == float64(int64(v)) {
			return big.NewInt(int64(v)), nil
		}
		return fmt.Sprintf("%f", v), nil
	case int:
		return big.NewInt(int64(v)), nil
	case int64:
		return big.NewInt(v), nil
	case bool:
		if v {
			return uint8(1), nil
		}
		return uint8(0), nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			converted, err := convertGoValueToRlpType(elem)
			if err != nil {
				return nil, fmt.Errorf("array element %d: %v", i, err)
			}
			result[i] = converted
		}
		return result, nil
	case map[string]interface{}:
		// RLP encodes maps as alternating key-value pairs
		result := make([]interface{}, 0, len(v)*2)
		for key, val := range v {
			keyVal, err := convertGoValueToRlpType(key)
			if err != nil {
				return nil, fmt.Errorf("object key %s: %v", key, err)
			}
			valVal, err := convertGoValueToRlpType(val)
			if err != nil {
				return nil, fmt.Errorf("object value for key %s: %v", key, err)
			}
			result = append(result, keyVal, valVal)
		}
		return result, nil
	case []byte:
		return v, nil
	case *big.Int:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type: %T", val)
	}
}

// decodeRlpForTest is a test helper that decodes RLP data
// It works directly with Go types to avoid js.Value mocking issues
func decodeRlpForTest(data string) (interface{}, error) {
	// Validate input
	if strings.TrimSpace(data) == "" {
		return nil, fmt.Errorf("DecodeRlp: data cannot be empty")
	}

	// Decode hex string to bytes
	if !strings.HasPrefix(data, "0x") && !strings.HasPrefix(data, "0X") {
		data = "0x" + data
	}
	rlpBytes, err := hexutil.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("DecodeRlp: failed to decode hex data: %v", err)
	}

	// Decode RLP bytes into interface{}
	var decoded interface{}
	err = rlp.DecodeBytes(rlpBytes, &decoded)
	if err != nil {
		return nil, fmt.Errorf("DecodeRlp: failed to decode RLP: %v", err)
	}

	// Convert decoded Go value to JavaScript-compatible format
	jsValue := convertRlpDecodedToJsValueForTest(decoded)

	return jsValue, nil
}

// convertRlpDecodedToJsValueForTest converts a decoded RLP value to a JavaScript-compatible format
func convertRlpDecodedToJsValueForTest(val interface{}) interface{} {
	if val == nil {
		return nil
	}

	// Use reflection to handle different types
	rv := reflect.ValueOf(val)
	rt := rv.Type()

	// Handle byte slices first - RLP strings decode as []byte
	if rt.Kind() == reflect.Slice && rt.Elem().Kind() == reflect.Uint8 {
		bytes := val.([]byte)
		if len(bytes) == 0 {
			return ""
		}

		// Check for boolean encoding: single byte with value 0 or 1
		// Booleans are encoded as uint8(0) or uint8(1), which RLP encodes as single-byte strings
		if len(bytes) == 1 && (bytes[0] == 0 || bytes[0] == 1) {
			return bytes[0] != 0
		}

		// Try to interpret as big.Int (for numbers)
		// RLP encodes big.Int as big-endian bytes without leading zeros
		bigVal := new(big.Int).SetBytes(bytes)
		// Check if it's a reasonable number (not too large, and the bytes represent it correctly)
		if bigVal.Sign() >= 0 && len(bytes) <= 32 {
			// For single-byte values (but not 0 or 1, which are booleans), try as number
			if len(bytes) == 1 {
				// Single byte number (already handled 0 and 1 as booleans above)
				return float64(bigVal.Int64())
			}
			// For multi-byte values, check if it looks like a number encoding
			// Numbers encoded as big.Int typically don't have leading zeros
			if bytes[0] != 0 {
				// No leading zero, could be a number
				// Check if it's valid UTF-8 - if so, we need to decide: string or number?
				if str := string(bytes); len(str) == len(bytes) {
					// Check if it's all printable ASCII
					allPrintable := true
					for _, b := range bytes {
						if b < 32 || b > 126 {
							allPrintable = false
							break
						}
					}
					// If it's all printable and looks like text (has letters or common punctuation), prefer string
					// Otherwise, if it's mostly digits or looks like binary data, prefer number
					if allPrintable {
						hasLetters := false
						for _, b := range bytes {
							if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
								hasLetters = true
								break
							}
						}
						if hasLetters || len(bytes) > 10 {
							// Has letters or is long - probably a string
							return str
						}
					}
					// No letters, short, or not all printable - try as number first
					// But if it fits in int64, return as float64; otherwise return as string representation
					if bigVal.IsInt64() {
						return float64(bigVal.Int64())
					}
					// Too large for int64, return as string representation
					return bigVal.String()
				}
				// Not valid UTF-8, return as number
				// For numbers that fit in int64, return as float64
				if bigVal.IsInt64() {
					return float64(bigVal.Int64())
				}
				// Too large, return as string representation
				return bigVal.String()
			}
		}

		// Try to convert to string if it's valid UTF-8
		if str := string(bytes); len(str) == len(bytes) {
			// Valid UTF-8, return as string
			return str
		}
		// Not valid UTF-8 or binary data, return as hex string
		return hexutil.Encode(bytes)
	}

	// Handle slices/arrays (RLP lists)
	if rt.Kind() == reflect.Slice {
		length := rv.Len()
		result := make([]interface{}, length)
		for i := 0; i < length; i++ {
			result[i] = convertRlpDecodedToJsValueForTest(rv.Index(i).Interface())
		}
		return result
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

func TestEncodeRlp_String(t *testing.T) {
	// Test encoding a simple string
	value := "hello"

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Verify it's a valid hex string
	if !strings.HasPrefix(encoded, "0x") {
		t.Errorf("Encoded value should start with 0x, got %s", encoded)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be a string
	decodedStr, ok := decoded.(string)
	if !ok {
		t.Fatalf("Decoded value should be string, got %T", decoded)
	}

	if decodedStr != value {
		t.Errorf("Decoded value mismatch: expected %s, got %s", value, decodedStr)
	}
}

func TestEncodeRlp_Number(t *testing.T) {
	// Test encoding a number
	value := float64(42)

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be a number (float64 in JSON)
	decodedNum, ok := decoded.(float64)
	if !ok {
		t.Fatalf("Decoded value should be float64, got %T", decoded)
	}

	if int(decodedNum) != int(value) {
		t.Errorf("Decoded value mismatch: expected %d, got %d", int(value), int(decodedNum))
	}
}

func TestEncodeRlp_Boolean(t *testing.T) {
	// Test encoding a boolean
	value := true

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be a boolean
	decodedBool, ok := decoded.(bool)
	if !ok {
		t.Fatalf("Decoded value should be bool, got %T", decoded)
	}

	if decodedBool != value {
		t.Errorf("Decoded value mismatch: expected %v, got %v", value, decodedBool)
	}
}

func TestEncodeRlp_Array(t *testing.T) {
	// Test encoding an array
	value := []interface{}{"hello", float64(42), true}

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be an array
	decodedArray, ok := decoded.([]interface{})
	if !ok {
		t.Fatalf("Decoded value should be []interface{}, got %T", decoded)
	}

	if len(decodedArray) != len(value) {
		t.Fatalf("Array length mismatch: expected %d, got %d", len(value), len(decodedArray))
	}

	// Verify elements
	if decodedArray[0].(string) != value[0].(string) {
		t.Errorf("First element mismatch: expected %s, got %v", value[0], decodedArray[0])
	}
	if int(decodedArray[1].(float64)) != int(value[1].(float64)) {
		t.Errorf("Second element mismatch: expected %v, got %v", value[1], decodedArray[1])
	}
	if decodedArray[2].(bool) != value[2].(bool) {
		t.Errorf("Third element mismatch: expected %v, got %v", value[2], decodedArray[2])
	}
}

func TestEncodeRlp_Object(t *testing.T) {
	// Test encoding an object (map)
	value := map[string]interface{}{
		"name":  "test",
		"value": float64(100),
		"flag":  true,
	}

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be an array (RLP encodes maps as alternating key-value pairs)
	decodedArray, ok := decoded.([]interface{})
	if !ok {
		t.Fatalf("Decoded value should be []interface{} (RLP encodes maps as arrays), got %T", decoded)
	}

	// RLP encodes maps as alternating key-value pairs, so we should have 6 elements
	if len(decodedArray) != 6 {
		t.Fatalf("Expected 6 elements (3 key-value pairs), got %d", len(decodedArray))
	}
}

func TestEncodeRlp_HexString(t *testing.T) {
	// Test encoding a hex string (should be decoded as bytes)
	value := "0x48656c6c6f" // "Hello" in hex

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be a string (the bytes "Hello" converted to string)
	// When we encode hex "0x48656c6c6f", it's decoded to bytes []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}
	// which is "Hello" in ASCII. RLP decodes this as []byte, and we convert it to string "Hello"
	decodedStr, ok := decoded.(string)
	if !ok {
		t.Fatalf("Decoded value should be string, got %T", decoded)
	}

	// The decoded string should be "Hello" (the ASCII representation of the hex bytes)
	expectedStr := "Hello"
	if decodedStr != expectedStr {
		t.Errorf("Decoded string mismatch: expected %s, got %s", expectedStr, decodedStr)
	}
}

func TestEncodeRlp_HexString_NoInfiniteRecursion(t *testing.T) {
	// Test the specific case that was causing infinite recursion
	// This test ensures that encoding a hex string doesn't cause infinite recursion
	// The bug was that hex strings were being treated as objects, causing recursion
	value := "0x48656c6c6f" // "Hello" in hex - this was the exact value from example 6

	// This should complete quickly without infinite recursion
	// If there's infinite recursion, the test will hang or timeout
	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Verify it's a valid hex string
	if !strings.HasPrefix(encoded, "0x") {
		t.Errorf("Encoded value should start with 0x, got %s", encoded)
	}

	// Verify the encoded value is not empty
	if len(encoded) < 3 {
		t.Errorf("Encoded value should have content, got %s", encoded)
	}

	// Decode and verify round-trip
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be a string
	decodedStr, ok := decoded.(string)
	if !ok {
		t.Fatalf("Decoded value should be string, got %T", decoded)
	}

	// The decoded string should be "Hello" (the ASCII representation of the hex bytes)
	expectedStr := "Hello"
	if decodedStr != expectedStr {
		t.Errorf("Decoded string mismatch: expected %s, got %s", expectedStr, decodedStr)
	}

	// Verify the encoded hex string decodes to the expected bytes
	// The hex string "0x48656c6c6f" should decode to []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f} = "Hello"
	expectedBytes := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}
	actualBytes := []byte(decodedStr)
	if !bytes.Equal(actualBytes, expectedBytes) {
		t.Errorf("Decoded bytes mismatch: expected %v, got %v", expectedBytes, actualBytes)
	}
}

func TestEncodeRlp_EmptyArray(t *testing.T) {
	// Test encoding an empty array
	value := []interface{}{}

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be an empty array
	decodedArray, ok := decoded.([]interface{})
	if !ok {
		t.Fatalf("Decoded value should be []interface{}, got %T", decoded)
	}

	if len(decodedArray) != 0 {
		t.Errorf("Expected empty array, got %d elements", len(decodedArray))
	}
}

func TestEncodeRlp_NestedArray(t *testing.T) {
	// Test encoding a nested array
	value := []interface{}{
		[]interface{}{"a", "b"},
		[]interface{}{float64(1), float64(2)},
	}

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Decoded should be a nested array
	decodedArray, ok := decoded.([]interface{})
	if !ok {
		t.Fatalf("Decoded value should be []interface{}, got %T", decoded)
	}

	if len(decodedArray) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(decodedArray))
	}

	// Verify nested arrays
	firstNested, ok := decodedArray[0].([]interface{})
	if !ok {
		t.Fatalf("First element should be []interface{}, got %T", decodedArray[0])
	}
	if len(firstNested) != 2 {
		t.Errorf("First nested array should have 2 elements, got %d", len(firstNested))
	}
}

func TestEncodeRlp_BigInt(t *testing.T) {
	// Test encoding a large number (big.Int)
	value := float64(1000000000000000000) // 1e18

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	// Decode and verify
	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// For large numbers, RLP encodes big.Int as bytes, and when decoded,
	// if the number is too large to fit in float64 or if the bytes don't look like a small number,
	// it might be returned as a string representation
	// Check if it's a number (float64) or string (for very large numbers)
	var decodedNum float64
	var ok bool
	if decodedNum, ok = decoded.(float64); !ok {
		// Might be returned as string for very large numbers
		if str, ok2 := decoded.(string); ok2 {
			// Try to parse as number
			bigVal := new(big.Int)
			bigVal.SetString(str, 10)
			decodedNum = float64(bigVal.Int64())
		} else {
			t.Fatalf("Decoded value should be float64 or string, got %T", decoded)
		}
	}

	// For very large numbers, there might be precision loss, so we check the order of magnitude
	expectedVal := int64(value)
	decodedVal := int64(decodedNum)
	// Allow for some precision loss in float64 conversion
	if decodedVal != expectedVal && (decodedVal < expectedVal-1000 || decodedVal > expectedVal+1000) {
		t.Errorf("Decoded value mismatch: expected around %d, got %d", expectedVal, decodedVal)
	}
}

func TestDecodeRlp_InvalidHex(t *testing.T) {
	// Test decoding invalid hex string
	invalidHex := "0xinvalid"

	_, err := decodeRlpForTest(invalidHex)
	if err == nil {
		t.Error("Expected error for invalid hex string, got nil")
	}
}

func TestDecodeRlp_EmptyString(t *testing.T) {
	// Test decoding empty string
	_, err := decodeRlpForTest("")
	if err == nil {
		t.Error("Expected error for empty string, got nil")
	}
}

func TestEncodeRlp_RoundTrip(t *testing.T) {
	// Test round-trip encoding/decoding with complex structure
	value := []interface{}{
		"test",
		float64(42),
		true,
		[]interface{}{"nested", float64(1)},
		map[string]interface{}{
			"key": "value",
		},
	}

	encoded, err := encodeRlpForTest(value)
	if err != nil {
		t.Fatalf("EncodeRlp failed: %v", err)
	}

	decoded, err := decodeRlpForTest(encoded)
	if err != nil {
		t.Fatalf("DecodeRlp failed: %v", err)
	}

	// Verify structure
	decodedArray, ok := decoded.([]interface{})
	if !ok {
		t.Fatalf("Decoded value should be []interface{}, got %T", decoded)
	}

	if len(decodedArray) != 5 {
		t.Fatalf("Expected 5 elements, got %d", len(decodedArray))
	}

	// Verify first three elements
	if decodedArray[0].(string) != value[0].(string) {
		t.Errorf("First element mismatch")
	}
	if int(decodedArray[1].(float64)) != int(value[1].(float64)) {
		t.Errorf("Second element mismatch")
	}
	if decodedArray[2].(bool) != value[2].(bool) {
		t.Errorf("Third element mismatch")
	}
}

func TestEncodeRlp_CompareWithDirectRlp(t *testing.T) {
	// Test that our encoding matches direct RLP encoding for simple cases
	testCases := []interface{}{
		"hello",
		big.NewInt(42),
		[]byte{0x01, 0x02, 0x03},
		[]interface{}{"a", "b"},
	}

	for _, testCase := range testCases {
		// Encode using our test helper
		ourHex, err := encodeRlpForTest(testCase)
		if err != nil {
			t.Fatalf("encodeRlpForTest failed: %v", err)
		}
		ourBytes, _ := hexutil.Decode(ourHex)

		// Encode using direct RLP
		directBytes, err := rlp.EncodeToBytes(testCase)
		if err != nil {
			t.Fatalf("Direct RLP encoding failed: %v", err)
		}

		// Compare
		if !reflect.DeepEqual(ourBytes, directBytes) {
			t.Errorf("Encoding mismatch for %v:\nOur encoding: %x\nDirect encoding: %x", testCase, ourBytes, directBytes)
		}
	}
}

func TestDecryptWalletVer3(t *testing.T) {
	exampleWalletJSON := `{"address":"1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6","crypto":{"cipher":"aes-256-ctr","ciphertext":"ab7e620dd66cb55ac201b9c6796de92bbb06f3681b5932eabe099871f1f7d79acabe30921a39ad13bfe74f42c515734882b6723760142aa3e26e011df514a534ae47bd15d86badd9c6f17c48d4c892711d54d441ee3a0ee0e5b060f816e79c7badd13ff4c235934b1986774223ecf6e8761388969bb239c759b54c8c70e6a2e27c93a4b70129c8159f461d271ae8f3573414c78b88e4d0abfa6365ed45456636d4ed971c7a0c6b84e6f0c2621e819268b135e2bcc169a54d1847b39e6ba2ae8ec969b69f330b7db9e785ed02204d5a1185915ae5338b0f40ef2a7f4d5aaf7563d502135e57f4eb89d5ec1efa5c77e374969d6cd85be625a2ed1225d68ecdd84067bfc69adb83ecd5c6050472eca28a5a646fcdd28077165c629975bec8a79fe1457cb53389b788b25e1f8eff8b2ca326d7dfcaba3f8839225a08057c018a458891fd2caa0d2b27632cffd80f592147ccec9a10dc8a08a48fb55047bff5cf85cda39eb089096bef63842fc3686412f298a54a9e4b0bf4ad36907ba373cbd6d32e7ac494af371da5aa9d38a3463220865114c4adc5e4ac258ba9c6af9fa2ddfd1aec2e16887e4b3977c69561df8599ac9d411c9dd2a4d57f92ea4e5c02aae3f49fb3bc83e16673e6c2dbe96bb181c8dfd0f9757ade2e4ff27215a836058c5ffeab042f6f97c7c02339f76a6284680e01b4bb733690eb3347fbfcc26614b8bf755f9dfce3fea9d4e4d15b164983201732c2e87593a86bca6da6972e128490338f76ae68135888070f4e59e90db54d23834769bdbda9769213faf5357f9167a224523975a946367b68f0cec98658575609f58bfd329e420a921c06713326e4cb20a0df1d77f37e78a320a637a96c604ca3fa89e24beb42313751b8f09b14f9c14c77e4fd13fc6382505d27c771bca0d821ec7c3765acffa99d83c50140a56b0b28101c762bd682fe55cb6f23cbeb3f421d7b36021010e45ac27160dd7ead99c864a1b550c7edb1246950fe32dcc049799f9085287f0a747a6ef7a023df46a23a22f3e833bbf8d404f84344870492658256ee1dfc40fda33bb8d48fc72d4520ba9fc820c9123104a045206809037709f2a5f6723fa77d6bac5a573823d4ec3a7f1cb786a52ee2697e622e5d75962fa554d1024a6c355e21f33a63b2b72e6c4742a8b1c373aa532b40518c38c90b5373c2eb8c9d7be2a9e16047a3ee09dc9a6849deac5183ace6cfe91a9bef2ffc0a7df6ccebfd4c858c84b0e0355650d7466971e66f1e3883013e5ad1be33199b1d110b79070ac1b745ccb14cf63a08f8cca3a21c9525e626ff5f0c34746e10750fb742ad51f11f2acae3676c2111853d7250d01b77821a6ba9e04400ba2c543ca9f2d701ae6f47bfad14ffe3039ee9e71f7b2401359ade9938750ddb9c5a8b018a7929ed8d0e717ff1861446ce17535e9b17c187711190aae3388bd9490837a636c25ed4d42d7079ad1a51e13292c683d5d012abcf46965c534b83ab53f2c1f0cf5830ef7582e06863a33c19a70511df632885d63245965047ea96b56f1af5b3b94a54999f784fb9574fdfcd7c1230e07a2aaa04acd3097b2b9f8ddba05ae9734491deb5c1a513c76ed276cb78bbf4839dae3156d76af444a5805129d5df791167a9c8576a1d7f760b2d2797c4658669608706fbd0ace1be2346f74862dfc9ef518e55632e43c043186e5d070deb34d12fb9e5aba84e5cb50213dc88efd39cc35bf42455aa82d5e3b707b3140be3b8623b34fdd81d08615c188ae8438a13881fdf6bf32f2cb9ff5fa625561040c6b71d4b8eccc90bc3b99650d28dd1ee63773e49664e3d48c484996b290943635a6f2eb1ce9796d3fa144a3f00ef82faaa32d6a413668f7b521517cb68b2b017fcf56c79326fa5e4060e643631ca3f0a0dc0ed718798b6f46b130d437c33f64039e887324b6f5e604b1669d613923794edbf04b1b3caea54793b52b44b170173a4f25c7ecef3b71e2aad76e556b1cb9f1d637ec52ececfa950dd31dbb6a60828a3ad34c1beffe09eb4785786d63bad10a0b0f66ea88c57380f38ea85f018dbd7f538cf1ee7624095b9a01ec5edd528f281168af020609e651ff316aa1320a710134ddfca600cc72174dcdb846d2aa29916488aa1b537b66da92e61af526debef4eb38c984569eaf549ff2129449269b492d030cd74d885f6f5785881cc4804b4a8a09ba4ff7aefe9074ac7d0c4f05d51fe4cc0ff7388a772092b9d02d70e5433a5cf3e02f46a6bd6b818d59a07ce3b9fbbf8b5faba74563bcc5240930c2d406c9aaee3e3ce0429bf68ac2b0a57adb09414cff50817d2a48fb9fa624ab863cb0c31a8b8dc5eaf6fa68cc1d7c6c685c5a33edd5c8933b9e8ab628ee428d0743699b2ff17f25586c7ce959280bb0b8c5342251f0a30b53dbc7bf1ee426ac9619c3560f811f2268ee37f189794e2e4b3db3a2fb2e34b649e504fb467438abfd1082619cc4a0b30d66beb831077812e418d2e2148db10cf4d4a29101ca52ec445b8d83519dd7de85a98e0beae9ee537096d3f1a55a7a80cdfa93d25f07c9f98e8af18cde19ec1f99c5dd4588b717a5039ddb7f177717caf0d0fd45420a70dbd6d3146890d9e450d5224146db4c33b779e3c3a04b976c052bad042ac57dd38be45407808c0fb0d7e2a8819e6cd53c6739e6612996ddaa6f066552590aa0343bc1e62b298ff2514a0cef8be21956c2e942816f7a3a3a0935eaf9b37251409ce444c986c3817e82835555fe18239f3ae33469d7965c2bde9991fde556bd07af01df52bbde0c35bb4ef48e3b5d0db53f8ca4ed35b83f760f0a1bc4ed9f86e85d6039a17df373c85402ef956f01db00eb39c4b74bd0660d29ee746714d9780d738e05c6cca414ce3d7b40dda8036a9eea9ab1388805f913eb19bdd3f09d9e161eaa50231bd9caba61971f194332dd28c696a60458c1c6c2cc5da8b1192611c7c553e9e12fe48ce46bbb891be8bb118721c86222e671ddd1da8f0ccb2b68e02f2014b4925e904e88369aaf7466bd7033a60c265d45955944916ecbdb84bf1b522b01b0149c632e04c568a7eb627c5bb90ece052ebcf79166c28b30d23fe52da0a5ab5dea83ca479a3e3b7a9cfbbfea04dbe6137c19d067317c2ec427a8c75a6b06bec6dcd5d5c0edc9aa80b9003b8e17c088b2f3db327d3e42630d82d20120240c3ba56232280787da4aabbf5bc95a864029f00710e195f2a76460a0317d10b552fe1bea097e41d49756c680a41d6ac186e62169b6b6cd7776ea84618b5b752328a5bacaa10aa122ff9b2698b43efe73d852a899db644863c8c9bc8068ea86ea843fd6fe36272b91cdc5d5317083ef3fd1e5462a0b0d0604dc57b3bbfceb0fca4cd349625dd7b25166af30efe5ee6a0af953a74d65f4736c59918ee55a3b0d9d9d42e04c7f8a77e479109f740e20c464d5d7e3d16805f47b61f403ff7f408c9e850d9baacd8067e544536a4953480b0f9ee9cd45f41ebd67b51f78788a6470cb1e5ca72ca346ce8a50d0ca0c921d5576a4455a1afb6d0bc688004712ee122cacdb29c51e84893324c27fa4a3f1917edf5352272b4c97579a6152e4b77663d0ab532915f2eeb6a862de8b696452321b660c3f2449673d086e95a7af28845a5259b763e0fcd09f72acf7b6c811066263060e5aa5b24658e880a01fd56bda4dad5ab604e129290f7d5489728f2a40968c6168b21cebbbcd11727cc9e9160c4e92e04387d3b0d62aab06a61f26daedd9fed11816ef2180172a47f47184ac4032b88758c98a2e0fb200f70e93ba695f5ebb7a1029610ad360d3b7fa1b4640b9dc674d3625eef786da93dff19bc7991b5d6193a3896664763fde479b5dfc04812111a80782854f2cf68ca7d82765cc9eb40fba4b44640710ed6e653abf9f07b466333f4fd22784d53cf40e17120f42caa841eaa24056b237827b0f47f7257c103c35027e9f503e5acfd023e7357b600d3084d361d5ee65ba319b45c153212a54e6fed85af7e43e0a926ebcbc2edf8de7e2ec9528f00bec262ad04d5c9dafccaea06a24748d28bf1799bae0e895543084539c50b5aaa4fb50d7431d6f0c8cee2a54aaf7ee7919b55bf40adb688632e5dbe273cea09e97b19c3d8e1f4de000deb66fa1942ad03a62d3252f51992244366c156000b49c297167a6cbdedea7ebae139d295f0ad298e0864249b905b7eb812886ec70ecdb286702274b5b8574149bf3866f9e46b997ff5ed622b169a0eb071347f18d530db1663906a28f4544ee4e004ab87b65476af30ede118052ff052b8dc986ca2c93dd5d4943266a579c7698ea014f688b3e8063a107feb162d392e2177b01bff77fb5abe5feebd0607158049a5a093325b7c9ee6b4dfa7a9f65c7c2fb628920d3603a1c2dad979eaa047cd661a268af1078c9788d720e64e4ce9d12e68de1e417ef2f293323681e1071f9220e1ee43d2e29d111b870ce3439f5100ecd4551ab65ee74aa1667e564957e9bc0ae1ea193980da2a0ec2698073388c85bec25ef447f0d5e93a5203fa44dff268e5cb799ed3b66e63d5e07b487e7534f24934c73a62a243e0151843a0fd3807711a101eaa7fc71f0ba68aebb9534d57cba41b094eebfb4c31cca8eddfa426f676aa347be8a7023a4e91ddb154b35cd4d5f7dbc2e5db491de99f33fc2cff2d57029ac950e1ccd681980af6a4e8969dfe39b3c7bfcbcf8fac92f1e6ec9fe572bfa6a7d65860eab2ed10ac01a71290b52e3148e84b7376a8605cd2bb0e8681ffc54691ce087685e33921bd44d36c78291713dce17569570f62137e6904f0d68cf53aa2ec395c389a75141f08114fb293ea63950e4ffee55ec6fc83cf44876b8e7f25cdd393ff87b9eda6eb746085b61a6900de191f0ce2cb388d61ece52e78bc47368194e8e00277e0d1631e6b9d4626ef76f8522582ccd5a40be3febc699bb510acc6271d55ff0f4cf3bb7669855a72efd9ca3e1056a2fe592a5bc877cce2b1f63b58383971da87873d2d1349cf5881242cdce4e7e2c5c514755746a0e0a7c2a6d9701cde005ae3420beb17c379a3516662253554f51f0423bb1844b0b90c54ed8177ceb0e1036a6609d836e748ca06c40ca64befadc6443ec286a0ce464678e8d11eb455f7bb305acebf6cb1f50e394a9bfeb752df1687831bac9cdd811f4f112ef6658d0f8799a866374ff96c5e2b79f30e7a74f8a2bc9ed1f88f01f30e30cb78ffb2bff10108f35e910ee3be4463e9e6f0ed910e8d598326e71dfa2277ffe5579d7fe9b6018bfe295b25219eae07b3b0270665c3fa00c3e0d180812b5cd62925585de84a7c48a9a86dba96544a251654d1966e082432dc85b6149cf21e91a46020ec32b66d28ba3b6a90c0617bc6fdd55aea819af2bcf84864ad60c28fe3c9f8339d0aee68b39d97f63b6e082835d86119cf9b9fdc8b827c847ce40aa10e1577a710132316845e825345e95bdf94d0c66ec65a6c4319fce4792313663b5f7a651a6710783e6ab71608ac6cbbf3af6911adf596ccf7c172b9bd5bceb6db379967b32b143bdd11d2ee12ddf64ecef6391e0f8570e6cddd3db95204919362b89b739fa94e7c1bfde799fd5e22aa25ca6ca42e30c08e23aae2385d99ebab441072a880dcefdab74a4c9bd39d363f6d1933d59400fca161d432aa00f23b1b1c19a154be8989699d549b66d44e39896f5523443bc6ddf4a65e91f1f3fb7b52318869a05856a4fc92f3694c81ed833c972fb918f7e5","cipherparams":{"iv":"8c46d6162cd4c765759aedcbce2a5874"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"82fb6cdc6917609135277badacf15baa31899d08b71a5a0fa33167167c161537"},"mac":"9187b17f7eca48e6b8c586b0cd790dbe0feb876ac8385f93faa7d5e22a3c8fc7"},"id":"92caf6ee-2d43-48c0-859e-ffa1e0e23312","version":3}`
	exampleWalletPassphrase := "QuantumCoinExample123!"
	key, err := ks.DecryptKey([]byte(exampleWalletJSON), exampleWalletPassphrase)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if key == nil {
		t.Fatal("DecryptKey returned nil key")
	}
}

func TestDecryptWalletVer4HybridEds(t *testing.T) {
	const exampleWalletPassphrase = "QuantumCoinExample123!"
	const exampleWalletJSON = "{\"address\":\"3ce22c0e2714196734e42b0d4d5ad11284260502a560e46c2cd857391564142f\",\"crypto\":{\"cipher\":\"aes-256-ctr\",\"ciphertext\":\"436140b5e7756ed556518609da8809a593a48808a3891905b2edee1680907fbfe3832bc2925f85308d02e19d80bc7239c69fe04f825fa5527d24a43a0179b85859c9dfd2d1b68fbb3f2a447b013d0ba36923dc3105db8f72bde463ae2e35ff18825457c88d284fe8018d379b176496282bd6f3eec733e7fbd0584fd7b50c68e2fbdcf62060fc815e2600f5556f6b7a8774f7acb4c51b634d7fd3fcffcf7569a12d292327b2ea670ca0d1ef380c535e58cf2adb9b312e5ede4e14d2b06c343cce33e6b032284d6385f0d4cd70e45b1a49ffe54320387149708f5b4e2bbf5b87a281e6efc1ffa4b55476b26ceacfceb6caeb2b67960d23d8abab323364d7f980cf0b13d8b71463d617a39d0d26df38cef42121ad6932835add5fe8bfed83b19933c9d396c8a4b806c4bc4643697fb5c2df8ef1d36b6e84210153b4e85a9f19c6baebb0bf41904dd1dc323976e0ecd0c41cc103dafc4c7fa3d565ff62eb04d9e778410db443663b9742da592ab74429fe4a04aeba3456c29e4e51c6e4121282147bb20f8b4ad09dab711c7970dc238060372103f3c1da635e5c65606740602f2ca0a29edbc73382934a03becb3351420f1a5475ca830c7d48afb29514a9f05d02fcc7ce146e490810fcf097f60680e5e8008e5068ee59170002ae1d564325985f03fa3091d6bd75c01564600f680e2f9b4421b126ba4329120d876177967289308c18203e861902fc2069be9a9c5662e1f499b37e0f714a74ff67f035921c9127b103b25c02b612e85d8f8fbfd3e7c36e0e9724ff151ba679bc13a8120cbad91cac73a6db48a8e1f312829927084b212ea87153d37edf9644f2b87b0c85b6bea9add51297254543117576f81853e12e87010c69d95a80d0002a68cd76522adc6fa725da98e6f79870e976d7828e1112ba2ccad3be1abfbd5646d34ec44a328aa10d0bfc9b218ae5f48d1eb50aa8f7bef0bc407f1d62ebea0d740c46d4107df2dde7e1f82f66491c0d7db8317983a8acbb3f59069511a1072a35d7436da6ca18d47ae521b2eba4ee81b566ba5fa758f3f77b747dc221180e42a2bf032cbb0da6728bad1fa15f980ec0951e89a9629f931234b93cdda9051190f62e2b1552c74c49c122a547cd91d8799be95cc0af67863e17b70ea3df0e53e3668e35a4619563eef93e3f3dcc1818e7fa4600475213df9f97cbc70e75b2370dce21554fd43709ebfd29001c4bca572f3672bf59a1c643d2b80ef27e759931799c1d57bed9837ea4e0dce4935789c16f540ac97363aee055528e28d0e5953af9dd64497ac2517ad466167def4a524299ef9fd251abb512a2338e12575f4f0d83ef2c6c54ce1ead82414d8116780937aa461486d320517590215c55f030675c50fd07e211dffa52777f6b5f8d60dfbd26f6bd04fa81d2ac5e7ce4ddf3ffc44dd875f91ca6247083ce4b0d34f966cd1f7e7ec5e8472e0391a6de72e9aaef5ca7f684d78e5f162e74da155836d8d142c4692a9462e833b79ec6ab87fe1dfa0e1019d194bcd63e4dd05e90891e1cd0c0976ef5581de552b40b3646d1322000ccc8858073e28a776ef022632fd529983b462fbe006105ad4077cd969f3c33a82903b3754f896b11d242312b963620e8ece7ec6317a083c1082e1ecd4cf5e86336622ed344d3a07a74f3b982dfd27148fd62ac40461602028504f45bb650dcb7238a8538ef95a2cfcc3b6832b1cd0cfcc954899e68fc34be65e4091c352112a53a1cc531319266b7be55e5ee2b396b06b403983353c23c52c9b842d65d3d2b767534ccb1826781218540a3a5ae9e5df39c2840721e9cc77d35f2bc39c8e371c95b02203f3f3f0435ac6634ac23befdadf2e625dd35b474a14a7ca9785adf872fa21dc2cb6fd74c1b7bdd0b6843c1f942916782330feffc1b90f7d49c4f20ab485236ba918a541fbf7570d4426252c75efad94acd8d0802ac73007c9062a2949fcab0c6c5f60aa5545199bcbb1b789759ddc92cbc9c0a584e022f905c98e30a0e56a5a56cdc7ad5d43d298eac4c07c7d003e95fad52b362b249c0b25b16d47886748d4f1e68827f44c2fc1499f3fce29aeb78a1b639f2d43183870f5c983da3401518a45f7fb9b0f8777e12a665f859c46c266a900735a3dac2176388cffe475cba639e0a7cfdc5eec069589f59e125ac2f1856c7108986843cd62ca10292eed3922a898d0455fb49bd23764d3d8d1c40cdb8b0a28600c258e165f5a63636e4a006b6f4f9087ba2a4679a75bf8ec1674c49f0b27cc35047529e6fe0a2701f25e3837e176ec103942351cdef4dd78700c22dfe4fb0232b048f65cacd8397e5eaf4617f9e87706e368e1d81d6c250569bbe9d11b520f67fc46f482086633bf3f4a3ba84ef586bc536439b936777da167684adb0bed53a23374e0fafbd63d5b05c273934ddbd4653d4e7b12fad2047d0c81081584c97bf643ad522a14718efd930da382fda253dd0dbd6de631959cdfda1748e242fd4d76f009123a5028f8ba36ef56432e1643e89783b3b027d294cfe93e91d3cc0db5c38f7a6427c078816bc0ef9136cb318afb5fa6403cce01b48c2425a16729e34417a9e83fe91189771785f736f04b343895be17f135dbe70f402f2cf17fc19b5d3a21c07758e3ec93f25ddc7e0419c34d5b7028401a030b9b12b24fcc661f20d927a3edeb632853bbce1d727060bd5a0ce4f4687d5d8b464d44deab5b15e5965df2004931ca719f869c060cd1690faf333e3d2e1ab618b0802b9adccc48b9b2ee1fd2542e02189c4db6b86836db8638f49198e69b175c1586857a2ab667371c7c0d12b53a7d541e324712629a2f15417dfc1c2fa6a6779f2419e09b301d2caa38d15e5e719da3093788e91a47d95fb002f5cd27bf5643308b91bbd5e433b5b781ba77db7fbd2fa699b8cfdba40eb4c2fc300b0edc8f13f9695b7005cadafb7038f27bca249dc8cc21619551c722aa42e2584d690da94921885f9e7c9e2bf84a400a24c6e6ba9c97e2a494fd4280ca14ead2d3d911b1beab944cc6bfc99ce809f9c1c9f2c849e47b8fbd1cce50c9209e448657dde03c05cdd02aecb24dac778b4818404b447d259a44ce35d17468ebbaa2e054c5c6527267023057064c864bfbc2f025415b28ddff458a5dafeb8bd2417006fbc42d45ea9c32a51eb3b7320953eb7d679966a624e8d45d54aeeebe5ee3fcb3c854df9dd6789d555b8e05146402550522dd0708300ff3c44e6cd1c176ea8433efc9f11563656f943086765430a53e06399374840982ec9865160138d8990e063327f04e4ce76b8809285160bccc2508d841f20e789650e71aed4367e2fb7789d133756555bb5337ba5e3bc84bdd9f00e935efc26e9d3d4071663891b98dc85162e1496fbace127439b0fb1502704556ff8d7033faf61dd6f9ee6e2bb20ebd10b1aa62af3738662a86f35e563083a51f3309f599d03b3395cec716e5714c72e041f3951cfa450b794a85fb25b40cc4db1f47ff7810645e38a473decb5c0a439149ff6bd95d6d83164568eb3759ef46a1cd65eea92afcb7a40a3a616807b45cb1e09e41551713a12349a13ad0ff3ada9baa16cbb6679f5230db6dd9de83277a91875848a4d43fd5ac13326d2faaa56f073fbc5615666b74ca0349bd198eb985b613bd1cc7d2ce4c10cd51541b8bd15e6873e088b34d55d0c14eedaef63a62ad762af809d87331705785c95b31c8461529aef40b83050e272371d8fcc1aa7ff4752ea9489bae71748686018b80740072895231185193ddbd00fab148227c789c9986caeafce3e446dafd5a63d0b05ae40916bacb4fd1001a52511508b235d05ff2c3a4f6972b74907a05265589d1c7e1d30361067f44783bc4437fdffacc870cffa1a74b407ce84ec9671180a33003c9b06c14c5e6426e6ea45198e461589c418332a620e16fadbfb8ca143d9264b659f7ae24e563be11e4a3230210939b59c57170fc780b45c6139d3505c074d746fde5e3639ddd7ba6b1805e691e6ad606f888b3d8fe9d05ec60bcd96a48ca51641c48630a00e17abafcb096b5bef5ea4765ae3b5e4fdd468530266bba5af6d4691cf31cf910d3d0b301635a1e77ec0c08be95570bbc1bdd6efd13a680edfe63efb5485759c0e9826c7b567e69d1ebb529adc1900159da1a86fba46821e4fbeb5983794b0221f4d186f07f92ad86b6e25d1ec4ac0a714aafc35bb7fb5aa16fb671e9c78f9158051e5928d10a0797364ef672812b89cec6feda209030377e4a668a49a866943de6b0132e9c62387e8bd626bcd844d606bd085f70cf04efe140e1d0dc48e1d3fadc9b252ea55f85b7238738f5ccab036037d223d2b2a22d557cd306aac03260827bdc0b73f31919080596331ed4ed215175e08046bc1303689c9e59060f745427492794fc940db77a5cbfcccb146426bca2ea68d75935973ebd6ccd0225b1249c10881691e534c9542ed610fb492368afdd908f901e953e1fb7ffee81cb3a6d1d9b9f61c41b9bfbb157996735f48b41c1b627029f44f5c7aabcee7531d9d787e13563d5a0182cebccede2fefe2b495b1d72fcb001b1f90b31e8c9d1bf2214e8bc7c5798ad64e8341f45a18f209a966b0afc67433c80d5bb690bc08bce4079cde6530336c589fb9243d156efb83822187bc75c90d0e4a732524820991022e398b6f84720d0f3cb1d474331f9870b9c3c4baf8ec92eef9c361dca16196e5218a6a3eeed8b6603e4c56823afabfb1e4cd0d366802b9b27dde9515516614e19940a79b64b83d2d1f6c8a790c5ad045111368784ca8b93af0956a9a0b2412ccf446f7d5107eb078977e94be83ba35cecbe4bfc30d71587ff5c49c67f5396545ae57c08ec3860b7a43c2f2ca6456cf8aab3ba3ba9f0919faeadc0d1870e807531f1e63bff966fa7a224e7e96a7e2767799d0492c6c5a175e3b09930a2adb1882db5498e764a792a4baca6fec377a9f8c0858c9d0b6ed40db006f832a549cc2a7212bba14c8df6f39b768d7c5d38293ecc64ce6e688d01c207d0c30906cafe4fd5f5102f4abc2aff7e0a22f03b395d15a0ee5193c4f3357d907fa694f3dfeb6804ca1637df141602b5a1332b825a06176613a9093e09cd447846b54eb4d033894583b095ff7b1979ea8eb59b84d5d5aede5b6cc6bb70caa8deb63a635b05da7877c02525377b51a9733145d0bde039c76af969bdec5e578ef0b69b17ff02bd063586eff7a31170323ba344af0138efda1fa055a7c1be1b1a9a61b13c52c8675753ae7aa79d585c72516660e08f595e68a0ade7088634e207341dd200ca4b964a9e67b2053f2fb8ac03869034eef05f606e842ca7ca94821b817edcb56234aa376899239bfb6cb4520dc1291967b1faf123220017d665252a29ad00648385ecaadef5c9a324d7083ab3242e6b2655eb739c109e9210b4bce5a4d11388120d57d7c6c9d5e374bc15a9ebc358dfce70a9299f5432deb6962e297fe9b09adaf612f9ae5bddfb380992b6cc901133065b32989e6fbff7aad8eb89b048d54deb9c9883be6ed009f645470b4b7f50548e119da3201046158590b0c117b31fea87601ef01199278e61faf3bfe1a389231d990584dd665398db5ff7761ce3fc59f521424f95c2d9d8ff416f4b3a9472d09eecfbe2ac642e94ded0af9910ccdb9b5d1b55c51efd10d4700a9ac30ef31267f634416dfe867323de1b72596f62890d893f9fee607f2343071bf5dc07db8bd46be2e2c44dfc5d6cd8ddcb4\",\"cipherparams\":{\"iv\":\"baaf756c183b9f0e506cef0699f445d5\"},\"kdf\":\"scrypt\",\"kdfparams\":{\"dklen\":48,\"n\":262144,\"p\":1,\"r\":8,\"salt\":\"9f62e9217e3661aa3c71c86ae3b134ea3b13b098ecb30524a46b75554d92781b\"},\"mac\":\"09c2577b3ce42075e776dfacdf854684ce5ebbaab2fff488bf75c0aa457fdb64\"},\"id\":\"971fde18-c2b7-491d-9a2d-60209e575803\",\"version\":4}"

	key, err := ks.DecryptKey([]byte(exampleWalletJSON), exampleWalletPassphrase)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if key == nil {
		t.Fatal("DecryptKey returned nil key")
	}
}

func TestDecryptWalletVer4Hybrid(t *testing.T) {
	const exampleWalletPassphrase = "QuantumCoinExample123!"
	const exampleWalletJSON = "{\"address\":\"38b12df2d4762a04a183f936c47747a1f13d0b0ba72066b43b4b6d7f776e9e25\",\"crypto\":{\"cipher\":\"aes-256-ctr\",\"ciphertext\":\"473ccf611349ccee6ca59bb50fb8f03c95d8977a3851a585f483389b847b92aed9eb7519461a2b0e89d79ac5c854093b356a0755675646d631bcddddbd995e477ff00eac13e7d34245e1cb6d492fd704a71bb8c28f1535cead22655176e62be529067676ce6d16e0d6714376f298bb787a7ca159e66246a2512321c7b6b1be736055009b7047e4aa22196579be4442a876f091cbe368e54469575c59c927b770dc3a51f447fabcbdcf7ab3b17e85bc2eb59b0c3d77b39ae81119a480717b5303bce9a6c8730c8a1b34995c67426050407e9ee3cbb89ba02423cf309e8bbfe8ff1aaafe18a584e9e9755d6874db4f7374f6a60dbddea73759c7360fff7a8be1b106565aa0aa6f93eb671b8b50042cdc3a15f6e1f21199e3aa6ff5cd5abd9c2add7cbfb083b5fa1dca1227775e1edee04659491d270366851bc2c75a99eb718b321d1e200edb4547b88525ab36f97bf12f92945ec76bbe497640e454168b082b1d8688a3897cfad35a65b4a65c34b4c0c5b03a5d17bac850e5a4cc9cc510ed9e371933613c67bf708205945e050df9be13d00ad8d8021bf857a81cbfcde464f5e33eec773973fcd9df48ad55e2a5eeadcbb2974be860b4679ba071c517581a960fff7ad99abce6fc047895dc5ce05fd3c734caabf181189837f7440f43984b8794d03f1197ee6e3f7602b56fed6970515be1c713664ed4768186bbe960147d74f1ae3e3adb3ccbab7479207939a15f2c9118c0e88a56803d5271559d48ac6e0563ae136df742db0563796b0f8345200633074044d7d064d4b73554988f8303e22e905ee0911049fe50c0d1319240f4c824c3c609904662d11e9158a9123b2ec999f7f5f17931abae9a1403feea3c2a77c2f0ba264a02f442686a308738e1e755d085262f12627b69cb04ac4f0db14159a22abfe820927626545fe652fb326340de821c3a33449cde6a8d7d4d94e09ef7b30c084c43f34919ed2b7147bc95472b1ba7b331963ab5afeed02307c4e72f9c149012312075a38aecabd8ec12867c431cb4e5f8a908ff7a6e9e495720d5af461e72111d36845b01f707b604e13b74defce421c41dbfaa435857ff56705c747138ba5a5e9e100a7ec5055dcac22a12537732d8d38f452e4651aaa3e33a69b9ff793ec344aca87c24ace9fba384a115c3d335b25ecc784428febc40c4182ccde566cbb34c3c3e294c64289baec4173aa6c46d33f6809eb20ed046a5e58e8d2eb5db1e66f4f3411042f9ce948759d4ca833c4d50dcedd331d84c3b6d8377967d23016fccbe300ec26794d604de5711b1c8cbaeae33b2f33d9e7f060e16311638f7a920bcac98d94569272a16db61f3d812d29532c16a55a657064972bc74e914f4d9476ce0aa057c04a5e5038244488d2cfc760ad7245f63167c19c5072b4715b87696c39e89dc873599f95fbfacfa1b05b44cabce771fe8880c8ff9d5e7f864b5ec7ecc770100d82334ae15bc2031e730406b8f212e024db78efc9f216ff54c8c73d5cfd89aacad08d1d853429393af39056c22dd061cd42ce5d865fd6f7cc21b6d8cd4c9b5d9a6a04f4586d145b39c720ed36980ba22976b79037a76595e0453417046e128f0286f9a1623a28d82ebf6efa99c8393b705fb173acdc8313a752724d004fd7b3b652b7dea95eb970db9c239ddb0460d3ab4bda453c3ce28a2d599c8af4e47f44a4ba9d0ccb3569ee208c755e3ecdb37befcb75e7dc147a73a8e40a5a52231e08ab8a98bd68006f21e5c88ae77c7657ad8bc73465da5ddfc87d30a9017bb2e19f637364559764ad30fd4b059b8edd9d2d56cc3594bdfa89ab848efcb18516cfa3f2e2ffdaf8e11a6a1ba7431c5f912b4c39696c3e050853b2f20efe274f2a7a4cda3e8102f6d3fadf2a9d318c09d263f529093c4ad3b88c050080e3bb4ed2418d60fc32a46b17ffd81a0515b8bb88a6ca871b03bd9231c83560a027e476fdf4dc265d3a701d473cf1343f152920c9cc4b0b78257b88f13a117b90e90e50780d5decc294b541c0bca8cf97d9c96a1247c76789d60adb07f732de8b3348713cfcf32df765a0785672a9896e69c044970d07d8020a85946eb0514bc68494ef8c5ba8ded9e8aad6eac6827dc024924171649ce55164b24265daa7ae3122c95ce5cb1c9df7ece5fbf9e92f0c37ec4bbb3f63152fe126ee64db1edb1c045902fb297a2ae193394149a53fd1b0218832c758b00844beb766b103824cee97d3aaed776818f8220b800345e167a03d87756e0661f352bf1c956383a26f77eb681d3ef9c8e21a2e1c1de2c9767f8171dda161928581f69c27139d396adc5a44b862203fb8551b2d3b6c2f17ffce24ff4482691aef65fd3f74e80ddcf905205ba1b10afd93f20fe98e16d491ac02aff5529a0412fb01cd384fee376825db920a95f10d12248fafe7d078d98318c1ff66399357e37c262ac4f314fa6434a03e68e5e01f78b13c2da10a19b20421ef1a6b482b9296c6586760284ad91d5fa8614d1bce0d865e211fe872dc70e500fdef6d3be7e1af303348c53cb665c1827f79ae97c575c476c2e4d6ac2175efedc7e7109990205004b6944fabe419299b782922eb845c66ef7340489760592e326001de112d88af917780d731be23bafb64bc38739f5671cd9cc62f0ca4224ceb887495aff93cb008dce3e644ec3302d304854b0983a8d40c1169a9fe0e15f9465a6c33bf5e9134743468cbd86cdde78b5ba7e5f720f273aea26b8ee9b0655a33c42e8930ce0893d5add1c8e514eee93632771ad98ce2db6d2db8dfa01c50f038ee07db84c42f0d6c72e18b4670e3b361fe9d2170cb6c09eee7b05213de576a7a443c994d9989ba3aeed1db2f4c2e72cc5fa693b2445259b8d6fd22020c44cc3ad5bed07f9683a14bb70a9bdd51990847a94fbdd6ed925856dbdaca8634a994bd6a5e1cbbf18d5ac40918a75d6161fe4bac769bf026642f57e9ded78c71a4e8ad1c57ff1e03aaffdb5fb64866e652d64a753eb3af9559c400189da43c1d83bfd2fa5246cadec8a47bbc13562333fa4042bbf88ecb3bcfd0bc93318c88f31a612dfdfaf42ef4b73acf3a10d97588a33413d59a488358bdd91b3fb1a5e816797bbd84aa3a580c8e0f9716c29b22eae75d0e0037424dc977f49e80762396eb45df38553294b6591372103befa09289e30f02cc0d87f1c9bae9583c16829fc2eb64cff6e985ead98a1b211e40fa0e8c3b3becdee40467127153356290209f4863a3a8ae58ac502227502a683724c4bb5443b61ed2532d4a65b4c2aca6b8fc3202bcb19c5d3745a772cf69823bd7dbf6903b807e14346d4cf6a753f311106a608b65a39d4b10b5233627fdf6ebdf1fa39695ce1f8c87f39e87d67ae3d4a1cdb0669c0febc26fbc08f370f17d20809ca78c904fcebbcbbeffee2c3721a938b2caac509dc39b77e48921b1ddfc3a01c24317650851fd9e9afef6d47ea3cfa1006788ab3f22ee4d2d25f5abad0b220d5789a5bf52b1bc376abf11fb44e0cc26e1f86046cb2339ef8d48e5ffef6f6f62e130f213d086b6fb5c6d48757da1a4810a25bbf17f8d6a2a25772a726ad4a22a270a7e185d53125e451b9306971c8649d2821103035690c890e3f339a9e5d46a6330b61ba91baeaa58f2888e59156db0d498922f6a3b8edac4171849769ce3bf40f2a7cd049e47a3ab4c75ee7d98d7e05fd1db18a31f2baee4089297160861b61d07c8c052dff8068452d484022f8f223cac0b2a38a2deef70817a4d1fea44a6f793121fd229e0cbb79bf910f02eb2801e177fe0e284624355d6fe1a3c7a59a67a04f2f9ddf81f3ed87c4af4902ccd10a75545d5ba3b8d0c077ec416b37ca42f8434288d985a9f49f8467dc7bca6bd3b3ccd17abf1eb39d2b597dc7c06661c21ed7cf83234e52ef6d44f9172c5bd479131d6484982d5a5c22e587bc9d86115a43534492df55c8d43756ceece8d2436c7d14f357be5bd0d6796c14169054b9944109fa4a48bb75a4a76751c7076a8e3e4b52ac1980b137b8879c398c853ba071e3d062f4e47cfea221b7b3aafb21b84536c9170fcecadb9f097673db85782cd7e901e73e789d07e8d34cba72f9d6ff0156fd703c7793df0adb5d95e33295821f7f3133637eb637d6276f3777f1ee267c35b09d3cd92d56e2a308fd2f741fdf1ae3db68315025bc9b596671329a9a43671d769d2ebccb09248eb430718e5caad62836ea0f29cd11bd723d7ba6ff0f4bde48d1571464767c478afb84df1acc03f8001417c86eb004e4268d502dafc2495c520e355501c94923349de5ee56989762655cb8ce102f98569ff70f08f5c5d0079096863983606a1bbc929119706fcbe3ec0c42faa8272fc2331a48e651d14f9a4802a0df1ba34a280190c36533a4c7464e60e707b0c9048260502871a61bacb7e22521b858f135fb124ce20ba85eb9978eb25d4faa8c51d508677f97293a59d811650a259b4d5122b93c8180224413604e157cf6beb06f75c279050876f11defabfefd43ee88df5e200ee95d606090bb86368ac3a7b57cb6e5b9545d211921a2f35ed90c5fd50d8c4409c4223647f9c3d0a9a663355a9f4d032d1aef55c039eda2d81dccb4d2bff2e1345284a76c67e21027720153abdac830754ce5e1d1895854d30c5310e7b97fa6be833ef62668ae3782effafe2ab9cb4a8a055ef10c60b9e51504c0edc77ae00190c3db12334e17358b299378d0b4774b68dc5571133d69cd2085279e0c824bea3367efe3dc026c39c4e8d4e808e321e28dad368170bf43ce0fb095146d3642c3b6263f0a88a770540f2bc6b8ca6ebde62b8dea0ba23ad973d733631b3019f73857d70f03cf779b879169852f738a3ef415210378b0bcf7e72be6f49dc3cc667bc7506bd0d4cef8716f8eb9507fe219ac52b2ef23231983a70a896d6a873e0c3f56bd630712ffdfc81592a91765c8483c332e3e9e08c29056e73585c10274425ed8730db7965bfe0bdb7583ef7572ffd800da44873a27cae462761dba9d5cf02492a59130c587a8ff58d1cd072554df1db5998d2675495da888bbedec6ea0906767d6c14d4d448280ba99ec85cff4b938588576c42a9b61c94561dc74ee971ee4fe68533d39e406bad69dcaed2dd819ee0afe94ffbfd47cf94fdd744d672ac38dfb2f0265abefd5d1cfd450a4db84db7c5dd7619893c5bd0b2ebafa63292f02a6a43002fa3134fbd00dcd8ad559fd26c31c08e68239621b830d0aa9586b77e8021f6c7f87f5a52285bbd246d99b0b29fd0d16dd8baf9cec1f2fbd412fa23b77f266b036ab65da8dd002d52933b3f32f72533adabfa348ebe333c4f006213497b1e8604f081f38cc724240a329b1fa4462e208be3063d919225e12941c4c1574dc790b09c835f88b380344f6830b2c02d1c6d3a723a5126e6d68aae7f3a03e261fae9aab3951a6092a1e723db0903dac41bfe92b29e2b461052f35185d7210a2bc0090ab091becab3f71e7e122611a1db6eae58ca3edeaaf56c9bf1add1ebb2dee64cd3d4a5d3bb50f5ccaf69126357f6305a3e6de8e267aef78db98f4df3933ba02df9e56916e56bf191263dd72781dc3ffde321a96f6cd95278f8cf10f297d80d5d27893e5f20a7cd35296b3c379adbcb81cb1fe4fd2fd5148b701a7e646b388bcd183e31f21e60755eefcf5267fd6c2b7a3a3cceeaa1fdc16301610201cb20aa21e793b2a8ab6a3e9e31e755ec295bbbad195a2574897ed917a628368ce749dcb\",\"cipherparams\":{\"iv\":\"32a0ab5236b63bde3db510c9196d74f2\"},\"kdf\":\"scrypt\",\"kdfparams\":{\"dklen\":48,\"n\":262144,\"p\":1,\"r\":8,\"salt\":\"3275630d42f2d1e90f43cdb7e586f5a36671791a3a69d5e752f15bd41aa0cf1b\"},\"mac\":\"fde3f3875d8c1ef4481b72ffd2644498504aefd8f273c19369c7567c7922d722\"},\"id\":\"8fadd368-07f2-41f6-960d-f04357882906\",\"version\":4}"
	key, err := ks.DecryptKey([]byte(exampleWalletJSON), exampleWalletPassphrase)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if key == nil {
		t.Fatal("DecryptKey returned nil key")
	}
}

func testEncryptPreExpansionSeedRoundTrip(t *testing.T, seedSize int) {
	t.Helper()
	preExpansionSeed := make([]byte, seedSize)
	for i := 0; i < seedSize; i++ {
		preExpansionSeed[i] = byte(i)
	}
	passphrase := "testpassword"

	privKey, err := cryptobase.GenerateKeyFromPreExpansionSeed(preExpansionSeed)
	if err != nil {
		t.Fatal("GenerateKeyFromPreExpansionSeed failed:", err)
	}
	sigAlgPtr, err := cryptobase.GetSigAlgForPrivateKey(privKey.PriData)
	if err != nil {
		t.Fatal(err)
	}
	address, err := (*sigAlgPtr).PublicKeyToAddress(&privKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	id, _ := uuid.NewRandom()
	walletJson, err := ks.EncryptPreExpansionSeed(preExpansionSeed, address, id, passphrase, 2, 1)
	if err != nil {
		t.Fatal("EncryptPreExpansionSeed failed:", err)
	}

	key, err := ks.DecryptKey(walletJson, passphrase)
	if err != nil {
		t.Fatal("DecryptKey failed:", err)
	}
	if key.Address != address {
		t.Fatalf("address mismatch: got %s, want %s", key.Address.Hex(), address.Hex())
	}
}

func TestEncryptPreExpansionSeed_64(t *testing.T) {
	testEncryptPreExpansionSeedRoundTrip(t, cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PreExpansionSeedSize())
}

func TestEncryptPreExpansionSeed_96(t *testing.T) {
	testEncryptPreExpansionSeedRoundTrip(t, cryptobase.SigAlgHybridEds.PreExpansionSeedSize())
}

func TestEncryptPreExpansionSeed_72(t *testing.T) {
	testEncryptPreExpansionSeedRoundTrip(t, cryptobase.SigAlgHybridMlDsaEddsaSlhDsa5.PreExpansionSeedSize())
}

func TestEncryptPreExpansionSeed_InvalidSize(t *testing.T) {
	invalidSeed := make([]byte, 16)
	_, err := cryptobase.GenerateKeyFromPreExpansionSeed(invalidSeed)
	if err == nil {
		t.Fatal("expected error for invalid seed size")
	}
}

func TestDecryptWalletVer4Hybrid5(t *testing.T) {
	const exampleWalletPassphrase = "QuantumCoinExample123!"
	const exampleWalletJSON = "{\"address\":\"030e264c853bd859c53fae3ad6ef0e011dc799685e2b05d5efa7ac50f10ca075\",\"crypto\":{\"cipher\":\"aes-256-ctr\",\"ciphertext\":\"3974d7494284a2bd3b5bc9da58a26add1acd04ce60434b5008e571fdb96107ddfadb97763b804f8da653b5c8fee753b234b120f141065245ff887b7f87ce2952df690722dbb0b21cf76b6c00aead098c8d1659d6d5018469aa42b59276f89df0df46dcb98ebb2151b93ff3c3e2637accf30a013efd28dc1b36a17aa1ed7ab11fdee1d10955ba7a86b3e9a68f5698f2e2fc4ec551b76337ca6b5eadf52898a91cd5adfcf2c9a3079a95730dc474934f20e596c6e0a2568307e6ca4fe3315e8bfc1e0e1554bb798009f36fcf17d74515be848d2ddace9902373a15292ceacee4909f7ff1fb6389e0d4d9faf5a2dbeaf575f6f37ee1670e44f9a065a6ed0188d2303b2dc2e7b98440fa5d4ba109f0b0d70e055c3e6507545df0e8ec7b18d7e3c804ad437e129f251305910733fc7ffa5cfaa11982a5ee402ecef71aceae5df069c3e6b053e4853b96e99ad8bc7bcb4757e5cb22232bbe7501714a60d992f50e937c1f3b260fa1bc12111184e10e43ab2f0d0c52b97d9428e762895de17bf4572b15402b1db6bca35105c49c5eda7a14605818844b8f55dd0e43913c73c4a88469025b4956665f366349e3f7641c483faf8aed3ecd54628c7fa8951683510114d6a4a23dedbf62eb3d9a7fa10d988bbf2d8a47cb31110d9dc56f5bd9fe7236e3f10d5f815396593ff6a860a48d153a13be0c76d139b651ff22affe48e3d03354e630a4efac5eb8ee60cb8a2b6cb02200a00a6d9ca6a33ba34324e7c560202a91642042a707b763f47fc0a434aa23cbeebba8264424c91f84f9c46ba55abdc107d2eb26aa6d87824eaa9c35700080d0962cb9b5bf091d1c76376d73bc606c927aaec07369cdacf1a3f9de788553adda199be21f75200888c67efad1fc5b46ae6dc7ff43aa7ec784e9aa4b67ab61344f889db51910c850d63be9e043a157788adf17acf6fed9151c836f61c042999850077ff47c6dd4abdf246efe3aff37e0cc1a1b15ea3d000541fe6d4dd62c2657cec5b88bd7ad0ea039ce2bb9e82f037dcc321fd53180dc3856ecee5bae2135edb3b4a6edc65883eaab3f47e656d28e51cef5e40aa65776f74bc0ce280e932cffea476499b6efc2ea7f23a4a68278ccb2405cb5e3fb379808631000366bb11e6ada80cd17046f8d319169f6afb90566a96856dcd4c9c3b36a911ddc09742af6c318a5d3810a1009412b0a440d97f5da576fd46e4565301d5e3b6ae901454e1030d55ea034d11faa101f93f451f7ad99b99c606b15fe3131c65840f81c8f100e84e74df555abd1f699ed454a332c31043e32fd9765658c564d74b09c947c1276c32cf1e9dc83f0f9fe674eeeb24f7c28b8218d1188b21d1a3694956325091f87542ccf8f5fc6b81ed5363a18e487c13f35716e1210a7cf5b4da563da822b067b8c4a97edba1629f054bd7cae5f51a993a47032a9ef47c743e65bb7be268fcf9df21129a765505f173af444cd951edfb44b6fe53ee56974004d52198fb6f7113e8169432a97ea1b3e46548370371b565a26944460589f92d5b61ade4aaeaad7a6aca21898579fa91face88ca989b1981e6c10d812b97b86fc008fa0c8d887c9cb9de392365e7673e2fb38de01550ee9de6be705b8ab1b0aea3b2385899615520bd33edf39e062b1f766bea6d53751926f2be5c7d7936c56ad799b013278335648168814c8eeacc8bbc2544630ad6e50da3e22f9a959894f5e21a2e78b43d1fa992fc9d4183044d39889950f96c9d2bbfb7d137e7bf984789af2e0b03a279e12dd410a62ff10424cb8f5a5d7fb7eb6d5980f51a0463ba3be44ccc75b09f41804fadb15cf1d18abcf4213f21e087c2f31e4122d0924611d1ff7f43d4fcc9145d9d106f7785e63de7897ac57b90eabede28c6c4b828ef94425e7d16664632e4acd59b426d632c0e319f69819f91bc1888e298d3fe78a19273c2063c87715cf593ba891e8e4e3a3f99bea42b4d73574c345fe8a4e032e96bdc3c19c86b1a8edbe41fb409f20877006afeaab81ddcbfe43f26568f7478c5d2559e6e766bf534ed581e49226bfb49585f47fc43dceffb58c74f9fd5025195f21f6f770b5fc55f92e0000feda65c1d1b1fc20d9a9b561c94102db3225d65c1ead37267365773ca03888ed96b665465a5b6bbc19575faa4516bf0079aeefe1cc3e359dc962018cdcd776d4e01eeff8a2f5be4630027f3e2e5a21b54e16f6131c4a52bbacf044b9fab5316ddf8d26b7396be7cd908f44b7ade2b8bb459f70aa8ea756a7e578a471ecd5c0b42f1d1c4e843185503c6124d4d9e87dadecf96f39fad1e0e1487c81b497b85d72ad788144a776eeeb9e4344f8fc19df4851abbb5f494f4458cfcd17b872b49ca89923079a8f4afe298df7d402687f6d247377a48ec544754cb79bd522bb4ec2da8c81465b4d4b64dce267a7077fb8cfdb590fe67e5072fc515e8bb23297b669a45bc5a3e7958baa9a8720594218d5de1657ae246e18d2b95d357fe9a0d3a5dc6854afc959b1fc0a5164fae48fb63eb566ffc698651ad133c22319bd0095bebded2e4ede3c3faba75ed855f278d3d0db8e8bcb3ab787a24bbfceacde35fb2ccf3f17d708e189edc23aa650bd3f3847e04a8836abf8ab2af826f025e38044181bd05e11174d9d76958e781df7ca6e1ed542731f9e97d58701f93b31040a3f9992cfe5ca26e2512d5eaf4ee32b7dff5b9ae25d0da53b265c66af691fec963296022b5511a0b0adb45ff78a750a643afdf85a998a1d41f7c8e4fc7e4703470635989f34b4616b77a60909e9a00d126d3afaf4e39fc60b1c7c442c12b746a99612967d387f5c2d89aec84a018242cb23436dd06edb9c8da86bfe0b3a0e84f7dea3fea67e76d7054b1c69ddd9dbb6610f731c045dfd85b1b0f8e46691f315614cdf5964d31063b7f44b398b4b64662ac58100d423e9ce788d6fd58fe8e51131c743ca7c0f53ac5b5239aeb9dc759c0f6a540459c8355baf058dd563080497836fb0439b8962b48bbbc23a5409905e314dc837361af9fcd3ba8cdb2b5b14d40c3328e09706358e3acf4c107c7b17f2fd57fa4f74016a04e7b7318e7ce4293b1c5da46b18552328a07139e58930effe15f940c827dffa676c2bd1caf61de1de86fbb2da8636ecce583a293bc493f004a8ba1349ef2a161e6176f5388eb5e66f06c4a48d67d1c4ed27c00db612da79d62dae28ab1afb3e2205e68739b19abc9684921d0c9b0943dd994cffabafb5e6d2718dbcef1a4f804a01b59cf25966ccde3e4221e08a671a1dc91b549bb510088fead16637d70d2f490aa12a587bdc0e6e8a4126b06396e874aae8211a56babbe054aca22a498e395fd39e12b4ba0b46e3b1b06215cb9f2685762f7ccacb752aac8bb539d21e539225edbe1c632710fdf3ee416db3fb51966da1d7aab6982cf34be0d09fc547578dd875fbb9d44a8d073a47a0ed64371968e248fe9e3c4b23f0c294d4d08bcfcb86419f56b518dfa5f9d1b44c25159ceb57aedc0262a23fd7279f7d9001ce4da0f3273f9e844dec2d06f71656cb7c6095f6b6a14059065e2d576f0b80f3254fd7609dad07851cbc25843016d0b0520a299822ae3a91f890390f09b6d10be15b7e17b22fe6bca9abc05d49bc0d4331d2a4baf28aa29477755d902d28e40076d8d67b48fad71873eb83cd3cf7b3fa307ab433d7f9251ab5835e3d2665f1b25e28bc2f9db2b6781653a067fa8182c60a8b4794a3589c9492d8e4055554788ae18189d28523efa88ce35e3a52e701460005d9bb6e41710fbde5e144331ee0dc2375d8590d2cf403228f925d3ac922718a22b721bb76caac3e160df4254dcb8c67f3d178ed1b7f4cbce3806d5d4b631aa7ba1383e9356b7dd4fba83cf31ff25e78b6c815ae211e7e1ca4279522dba5a768948d75e0ffb8b17b8c4f9aeb6fbc902e786930a9867623ab7b83532c35efb1b5b79f2156e837a996a935c920b0301377bb0fb250bd779a6c5ec540035b76d874b0069a413dcd297e0e1a46a208b212d4f37efc83835b327bc2f00547c37d3463ac56393489820b5f5e9272a1fb566d5381a918103cf925d3beec1cdc0926e8a13b630c65e1af39bb0e7229e4b0bfc045b8706ab6f7f9d11b6d54946c0d46b3730bae9e3d6101f3cb600225e5296779c3d625d2995d2ff765926a4f249c4fa9227420d621de0bc0e2f5d9069a4e40d8dac13cfceea784ab3b3df84c069561d5d63f3a315223d07509c0df832c37316dc376bc0c2b62bbf4de354cc945b9452185c466467607f772e95c9f8701286285a704fd762a41260c1acd45682028bff8e156339456753c74e7aff38e6173de52bbb160d52f4227f0be196cf4f0cdbf5ca8a74fb86ddc10608160a75799891aeb4d0fcb2b1a1596d13225b76d21dd0cb49e0eef9a28796a56062e62597d1868c118db4f191b0ae1d6473f2e6a6b71c221f3377ec8c50aa29a5e011e13263563252f37e343569ca95e7a3758f876aca26c055396d1375d13b9f2defef8ca19ecae2bead72b7f7a4dbb2131862db9933d2ebc9b83d53c3c39c321d727a7fca86235a22392d2a307c24216f55aa3447ca100ad861a45f199d0649fc2e982ed9a6540958fe3947ada64c4a665830a482743126cbde4c45fe0e99560441a7d486cc518da6fa795ca1df851c94042b9cbb3993e3fc20dd73f2b8b7ed9db69810bf81407b8cff7a7131201155939d3892bea67373f3b3eed7b304ae8b02635bf84e0789b8b22f915367d85210d1b96a7bb6c8094eacfaa16a039e9a8fa096add758b49b6b4074947ac16320ef87554c3ebcf1d1a15c2f0aa15365f6fae01c89ef1a5fd5da380ae601115832efdfacc4a8137757c889dbba0b82eabda7946960e9e67134a6f23cdfd6372781de40c55f4ea3a26b740e08132347da92a3eb41a221bf81c472876032568b5875a1a6058ac684d24ecd68357436c8aa50a0981be6a03e8d06b5a46b9c0f8192b977e13657f458d2ca11b581cc5070dc0ebf574ca04b183b3bd6af055e8554725fa113164301c49cf5fb10477472849d0b07bb93a797252d786f17c8b4a53c66c704b47ec1232bb7d4463cedb43d75934fd348f1f7eea0149a937bff306a0daba4e889903843dd38d006db77f3d9c90fd8e5c84a131305cdc54caf885d1b3d7560ba816aadb9ecb2a13a942b72516c37bf5de0fd8026389632f8b2dae366db1fa5e1341a65244718c27b4033a33440d46147cc3aa5b467c189400c3ddbf08016080d8d62a5de6ce574db26cc68634166488acd6b5c416107334c22e77ea57b596f327b7966fac82d4ea37759fb46d56750841f2e1f97b6476792660280f5201406b7b94d3119d713bce25086fd67cc69d2d3edd289f527ba017c6b1550490d9b053ca6eacaacfb2f1c51163ac248afd0f91df2a18ff3d33f65b084ebfbaa70c59121abad909e054bd12ab7a9812b33272f5fc6c5ff70915fb5f00a1e795443e1c0ea1dcd82ed339f61e93a6fc75e826c4c5c198abca3423ed8f73b99e58f58b5b189c7de64700f5d29e035aa8da9bc02c5bdb25b30341c7014bccd5ef7387f145d0f29c23f561f10c70c0529c0a0886361aee68f7a217e6cb160e19f7a2fc5c7bda9347db0592d6d2775fa07c359085da4ca401842f3cd41493cdabe30e4d2e050ee3ba4a5138033acf2bfbb6b05662ca2c12e295545c44bf602b3ca25799e78cdf3d51cf8d5bed9f82eaddb170fd3da4652e87088645455e56382df3c2c4ab9286dc3fcc00d752a2e4bc0e2e100d11fc4f8d34f705657a6b5ab20bd4cea1c025f7f1dee2d4743653864b5fa271631c1e1261a899a3dd5ebd10dd5e2ba1bcf8906a21e6a0d9de1ef7cfe9cd6f1b78952126546ab56e01ab86980789fac1e0646e5eadfb71324b4f5d56e7fa3f026737fa6942154b66ab6046fa27feb276962ae08ccfaece4e9af7ac41ca151d619114b306ccbe1eb2b341501e8a9de50d097624bd1f4db6ef67b968d7ce15c69bb5a0306c918551bb3982191762864785fdcb1259c80ca4763626bcf1c75f3a8b029b25694ba40b39b8ff4b46f18e4bf4a1c476df8a66a4fe36cb16efec1b40ad8e213184a3658370ef8422d8f4450fd1dd48187d2e9bab024db87de298527e5eedb493e007ba9e50389839da2f5c480d09286071252c26458fdc39971df917f4831c6fe672072a8d8833143cdfc91042950ecbbf5c8a1bfe06bde3513330116b703f59ca57014838ab7eae481b2448b2b7ab56ca941402d0f8180153889856cd62f58a77b4f7f88462a7e444906eacd1369b30400e068efac0386c2308997800e2ae4007e251a0f653348ddcb91667d8af481e957ce9fa21cea242cfb5b526cc8161f816866f42a4876d74cb5e488226c563ffc6ad0d56a4de378382089a9754e46a163100a69fae1b13edde476514600947063e867a35945b0bc7728c9400383e8836e87b1307b7c4718d1adfd1237e5f3f204983cf8a59ebae43349fed322ae12e9ec9e6ce757b14f83a3d6dacd8a841ae00f18b34df4bd58f52a0b40a35d326fcb1515fb0e7695603abbb469d3a83c07c9ca7f3d0991df1fcc4ff23901b576b4630d58e4c848596941cbe189dccabbf31ad09e5ac43d463faa8de867e4f84c3e7311fd32e0ce58ecfb0096592b10c9330cfc5e90dfce533b949a27b6877cb055e1277c30648e3d43fc1085d574555c2a94ea6c75c0e3f8c45370932d44d4c11b92992120bd7d5cd8acd5087a672a3ad960502495e492231fbef6b84333b0f5a681599b8fd3df828b2728a9982f0928418271ac5ab44b53cefa00f410ee4dead23c6e71b9b940d2716ba352a4ac3097725d9126306484b83a9492dad1d673a5b10891fb6831eaea4065080f326e6576112678286709dde8ba0e076305a40bb218de058c667daf69c657bd86994ae9590f3d6c393c6822c13497e41ba2ae625132b9b75c1b83a3c3bd69671277756eaa16238a3940a56883ed1dab2a8bb35c56de63f5bb56ddf3320c0d64a8ea49cc94954938804d7ddd18fe6625aa390f54d9bef6585966cd13a652274ddc0aa33d22966a4a1c8ae9a5ae49d4a084e70d27c313ab582759cd7f322d969b02a2c033cffd6e779bd3478ad1b3d3ce78b8a47d2af43c8d6d3921ac2b6123b6cca8760adf6ffb25e1368c5a8860f707829322f329cfe36a54b722f87d2e759f1b893fe55816bcc4fd9ae570f89b3534fe2c1f76d0956dec5df18befe863afcbd1157a5e641a3bd5788a20437cbaf59489978688c1178fe054fca11322262d50b1bc1d1e34e6bb1007d2d16c71b15b88ceecca2f1a09545bc7a9df8fae6c9a1584a26aa507db124d61d2ca0c24cecf80829d04de765f5e2810b9fd046a8195fcb9b9e46101d97ae369c95e1d144cf9c62a7e451e352dbf8f7b539ebc1c7a7b18ee9610a06f6e0a5bed86e89e595127b34c73394dbf1fb7bb5778c1710d362e012b3a7dc80c95aba034c385e6ea07f5ddfe6644568c8b481f1f7f462edaa80572b33c818126b45aa79e12366334cf9aabd6c1a78430f7af55d52ddeaecfb0eddee6a5bdfc8368827414e6336eb4ce6fdb5f3c319d28d55ce76b6d83edf47b8b0063d39a40eeca95b67334a748aff6b596653ab8d4e67e8cb0de6aafacfa25e7033bd6460e244a87c05cd337e70d54e0c12355c74824dd763e2991c14d4a86e368149ad8a19262f84199ad4213913bb47c9e5f3967247251fc1cbe9598e88856f28e2ecb1c6fff5cb44f4e425a05473723514d28e7dce8163e96a88ad4e63a35740749a12be1d5c03d260f8dc5e169e8c215f91dcbc91cec758a81ce870742e3ef2399e5510020a2f7e4461c7b6ffe5c4c1ad276a93c6d494d9c8d730424bb86e33dcd9b06e21cd2b09924e9bfd4518b6f9563ef5d68bab9320eacb581bce5694c1d8679f19ed14ded0753c4e72bedc6fd083cbc5291863fda64dc71dd664ca1ac1adfdde2b8e785d1adbca7df3cfd14689320d49361c95990f4903a5aaf9c99f5a7f2d970870e8ec31e246ffd280d8cbc56c26d844aedf4d2046e23d8683032a757ac6c5857d5e2226510fd9ef8bd4877a1469e35303970408b03769c509be221f39ac193ad55cc531d9d80bac989910ab287f95cfea64259bda74cac691f374b4d463b763b3f3b98db40ad1bc1b69623b9b2b47f5d99490a92afba50477f445a0ff99d1c895409609fc29cf082646db535091ed61483dde4a6581712dcf44ae31aa035da11e4fd20a707a882b2c08f2a387d0678e52465f88f80a7b05dfb42bd0655c15f996a9c8dc8722d97ac251496c067a2d5d950e363aac05a5a862e077d0a672a3524faf7126e54f0048e7b31522e027cee12483135e53eb89ea1381e3ec44d7da6a1806159b124dd55de108ccd3bbccc18a5d9e6b42cdebd535f2395024d809a5fe4ed88a76b488b8ee2984afae5e86b86bf1ee9dbbc66eb282ed366bdb495661d0d5c673208e92cd1895b4ed974f9bd4117bdd3ef0b376f5940100ad9dcbc83a52fa3ff2c8c038e0e8e68c7427324817e184adfcca982910714bd49583fad7a2db6552cf8e68cec590f4d58b318020cdac04d9a1287ddadf4017ed17c279ba4826892ac5fbd62cf71437670e102c667daa25b86483c5e95397fa3a0e2836c2bc6f7b536a1f3619dad08b9abbeee8c7529226e95f45917e0ac5b489f8a5f604594407583fd011a82578dabce46a491e2a34357a38a1152ef6ece2f06cf2b08342fa44effe0694671ce7147de050355bc25246000869df8d2cde1ca3adc3fc9ced541f1a8034c7cef1ff3e4038723149060ddb9e8d7146a1b0f4b06410e7a6a387bc509b97cd19f9f73a7bae3fa7f3d0f534697806c6aa3552e66ba506d35a4d5a23b042317510fd0272c38a74b192222418e90592e58b245aa4f8a4f77c9702b95c4e02faeab79d8a4a6744f56eb65ebfb36adbdbb4bbc6ec6c3871c8f514f023a8e0590f3f1cc213f9ce2a228ea4efca1331b08afab6b7c930b26ed927adc0b03a5de49ecf85662b111a71932b191c4080053bfbb309e654c07dfd2bfe3245123b5345efde2943206e180fa3293d3d31f33ec4f91788f23f4ff1a2220bf657771342e9f162f518c44d0d096e2dd865952237f75ec7b5a642ca0418bffca12f86fce1fd3cc7f55d9166a03f7011724d546ab53680fff82d41e3e4aeb69bb35abad5b59eb82bcfcaca11b0a321d6ffaf4ea84ab2a643f0500e9c81e3efa69a29d6e5a736468d542a3c23aa96c46d7d1491af2d18033630e054276d9d4c07c4ee432c1846402c3cfdfbdf4bacccdc1d924d73cec740048aab82b68f149230883d677cdce0ac2b0eef751be1caa67496605164c5d524a91f110ea611f97119c32b7c150c848bc9a169779fd8dd44f6a06d63e2c11a5a129c6a14b5ab5666f0c657decca7d1f95d099b3d3a652a6f207b7b37df94161f3a1ebebeb3c1f03e8e10f36c9d6f6b2a37a59dd747f2a994bcdfbf204c51a4535acdd256c03b59978c5633d6a4c8d0009dc64a2f95207598114f80e8101581873a8be38f9ce176f82d673410ffd91c8738e8effc6a3dbc285ed2a00b679963035667c45c682c7a7588c2e15ce3745c1ae506bcea574092eb996901d73201104848a7e5b31356d74342bd2307bc8679ea50f307c71e817ce14889477221f90ccaf1cfcf17af069b8ecbc21a76b384b0b296731cf911964d5bdaae5644fa259de99e6c271ffcadbb7d5955df58f59ee10d7cc9b44942066135e98df3211395a3252e12a5a1d58dc7d0e952f2e8f2cdce995e7ca6e34bd3ee4a312f3acfb122d3239074da11f2901e01245eb3b939251a55583fc81986d97799ea64ffe444f7be1785c313e55f86f2554d5253ae92ef0238cc738c7269250ef1db56298c235f1cd8c12dfc0351e54db23167855a992b58ea38bcb2338fe1e567136e8bdbfc13c13982640584d0b0d02993e9459d3cd8a08b4c9a78309ad3008f481c768f77a9bff33c7e5fd39dad1955459b0c7acd0b74f15706b726f15335abd762de0d686d2319e765d59eb3fdac5f599c236007c6494b1dedc029f42c4b78347a72b5172f266418062c9a4c9aef56ede49bbeb4f6e92cce68bc3eaa520dac621475b882fcc0d561e13a7a70348a382ad71508590733e4996a6439a73fce3c96da97360cac60c4bddd79e653b46b5d2316384cd2b99637739fd0b40bdf3361111502d95fd205752a681697bbb1d7cdaee6fb9dc19ce405c80c67bd69d5f41a361fce581a629a48b18c5bc5db0dc0a5df63b8bd89299aabdf9b5fe1023bafefeb9cf0db08608df2904878077524fe4989b41a5db9fe61f18ceb0d714d1b5a2de5ca8c94bf86e05ebc63dff2647ad159659205a2dcc54df4717defa298143e77057711a60381e7e58d2ab2d8026132fe84ff51415edc7fa756e7764df0665b365677ba5209c0caab7fa5979a4405dfc9380a969ada4b1ff92d6561a9414e4e202f0714b637521d6ee8a0055c3412c3eb5d492e43654794603ec26aef005d7f25bfd8c20474c86883a42bf6e7f7fa7b3a9992591ca3caba2c7b54f498815fca2e7fe4f397230ccb155affb1741fac5bf5d22a1f050fff14e7d11e1fc8307334c421f6062a62fa8dc889e8d1ed51ea65a1baf39d9d5df5fe7520897f9352540fbc0152c07998898b0194b96f6633cfb262e645cf7a1c8b9cc9472159287a55a763abbfb707b947a764cfb1a76e607ef9ae8d81e9b97441d6f6220bd74acecd5eb7dc49141ee579a1f819034745851407b867f2f899d4d0b1a0ba7c8544dbc4964aeb8d6330ff821bc22ead3a2792c7238c6fec2a0b20cd52820cd96b683bcb3c1a5e0d74b63466181217f2a34edb004a5b13fa423ec92c8cfa\",\"cipherparams\":{\"iv\":\"fb44e2357661af07225663334e4ff351\"},\"kdf\":\"scrypt\",\"kdfparams\":{\"dklen\":48,\"n\":262144,\"p\":1,\"r\":8,\"salt\":\"35c94dfea1201869893d8e02fa143bc9cfab382a152fda97d910cf5a5f25973a\"},\"mac\":\"a2c01f77bf6acd0e7d7971296627b1ab95bcd4fd6fb1331c8b726505b57a2bde\"},\"id\":\"acd45f9b-81ad-40bb-9a23-595c0462a244\",\"version\":4}"
	key, err := ks.DecryptKey([]byte(exampleWalletJSON), exampleWalletPassphrase)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if key == nil {
		t.Fatal("DecryptKey returned nil key")
	}
}
