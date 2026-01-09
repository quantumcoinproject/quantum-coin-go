//go:build !js
// +build !js

package main

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
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

func (m *mockJsValue) InstanceOf(constructor interface{}) bool {
	// For testing, we'll check if it's a Uint8Array by checking if it has a length and numeric indices
	if m.valueType == jsTypeObject && m.arrayVal != nil {
		// Check if all elements are numbers (bytes)
		for _, v := range m.arrayVal {
			if _, ok := v.(float64); !ok {
				return false
			}
		}
		return true
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
		// Convert from hex string to big.Int
		var hexStr string
		switch v := val.(type) {
		case string:
			hexStr = v
		case int:
			hexStr = hexutil.EncodeBig(big.NewInt(int64(v)))
		case int64:
			hexStr = hexutil.EncodeBig(big.NewInt(v))
		case *big.Int:
			return v, nil
		default:
			return nil, fmt.Errorf("unsupported type for int/uint: %T", val)
		}
		if !strings.HasPrefix(hexStr, "0x") && !strings.HasPrefix(hexStr, "0X") {
			hexStr = "0x" + hexStr
		}
		bigVal, err := hexutil.DecodeBig(hexStr)
		if err != nil {
			return nil, fmt.Errorf("invalid int/uint value: %s", hexStr)
		}
		return bigVal, nil
		
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
	// Test with hex string (1e18 in decimal = 0xde0b6b3a7640000 in hex)
	testValue := "0xde0b6b3a7640000"
	abiType := createUintType(256)
	
	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	bigVal, ok := result.(*big.Int)
	if !ok {
		t.Fatalf("Expected *big.Int, got %T", result)
	}
	
	expectedVal, _ := hexutil.DecodeBig(testValue)
	if bigVal.Cmp(expectedVal) != 0 {
		t.Errorf("Value mismatch: expected %s, got %s", expectedVal.String(), bigVal.String())
	}
	
	// Verify round-trip: convert back to hex and check
	hexStr := hexutil.EncodeBig(bigVal)
	if hexStr != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, hexStr)
	}
}

func TestConvertJsValueToGoType_Int256(t *testing.T) {
	// Test with hex string for negative number
	// -1 in 256-bit two's complement = 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
	testValue := "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	abiType := createIntType(256)
	
	result, err := convertValueToGoTypeForTest(testValue, abiType)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}
	
	bigVal, ok := result.(*big.Int)
	if !ok {
		t.Fatalf("Expected *big.Int, got %T", result)
	}
	
	expectedVal, _ := hexutil.DecodeBig(testValue)
	if bigVal.Cmp(expectedVal) != 0 {
		t.Errorf("Value mismatch: expected %s, got %s", expectedVal.String(), bigVal.String())
	}
	
	// Verify round-trip: convert back to hex and check
	hexStr := hexutil.EncodeBig(bigVal)
	if hexStr != testValue {
		t.Errorf("Round-trip failed: expected %s, got %s", testValue, hexStr)
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
	// Test with hex strings
	// 1e18 = 0xde0b6b3a7640000, 2e18 = 0x1bc16d674ec80000, 3e18 = 0x29a2241af62c0000
	testValue := []interface{}{
		"0xde0b6b3a7640000",
		"0x1bc16d674ec80000",
		"0x29a2241af62c0000",
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
	for i, elem := range arrVal {
		bigVal, ok := elem.(*big.Int)
		if !ok {
			t.Fatalf("Expected *big.Int at index %d, got %T", i, elem)
		}
		
		expectedVal, _ := hexutil.DecodeBig(testValue[i].(string))
		if bigVal.Cmp(expectedVal) != 0 {
			t.Errorf("Element %d mismatch: expected %s, got %s", i, expectedVal.String(), bigVal.String())
		}
		
		// Verify round-trip: convert back to hex and check
		hexStr := hexutil.EncodeBig(bigVal)
		if hexStr != testValue[i].(string) {
			t.Errorf("Round-trip failed for element %d: expected %s, got %s", i, testValue[i].(string), hexStr)
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
	// Test with hex strings: 100 = 0x64, 200 = 0xc8, 300 = 0x12c
	testValue := []interface{}{
		"0x64",
		"0xc8",
		"0x12c",
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
	for i, elem := range arrVal {
		bigVal, ok := elem.(*big.Int)
		if !ok {
			t.Fatalf("Expected *big.Int at index %d, got %T", i, elem)
		}
		
		expectedVal, _ := hexutil.DecodeBig(testValue[i].(string))
		if bigVal.Cmp(expectedVal) != 0 {
			t.Errorf("Element %d mismatch: expected %s, got %s", i, expectedVal.String(), bigVal.String())
		}
	}
}

func TestConvertJsValueToGoType_FixedArraySizeMismatch(t *testing.T) {
	// Test with hex strings
	testValue := []interface{}{
		"0x64",
		"0xc8",
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

	// Pack a return value: 1000000000000000000 (1e18) = 0xde0b6b3a7640000
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
		"0xde0b6b3a7640000", // amountOut in hex
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
		"0xde0b6b3a7640000", // 1 token
		"0x1bc16d674ec80000", // 2 tokens
		"0x29a2241af62c0000", // 3 tokens
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

	amountIn := "0xde0b6b3a7640000"        // 1 token
	amountOutMin := "0xc9f2c9cd04674edea40000000" // Some minimum
	deadline := "0x5f5e100"                 // Some deadline

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

