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
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
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
		"1000000000000000000", // amountOut as number string
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

	amountIn := "1000000000000000000"        // 1 token
	amountOutMin := "1000000000000000000000000000" // Some minimum
	deadline := "100000000"                 // Some deadline

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
		nonce  uint64
		desc   string
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
