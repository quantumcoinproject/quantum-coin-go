package proofofstake

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

func TestExtraData_basic(t *testing.T) {
	_, err := common.Hex2BytesWithErrorCheck(DefaultExtraDataHex)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("default", common.Hex2Bytes(DefaultExtraDataHex), "len", len(DefaultExtraData))
}

func TestExtraData_encode_decode(t *testing.T) {
	var errorTransactions types.Transactions
	blockNumber := defaults.DefaultConfig.DeepCheckStartBlock

	encoded, err := EncodeBlockExtraData(errorTransactions, DefaultExtraData, blockNumber)
	if err != nil {
		errVal := err.Error()
		t.Fatalf("failed to encode block extra data: %s", errVal)
		return
	}

	decoded, origExtraData, err := DecodeBlockExtraData(encoded, blockNumber)
	if err != nil {
		errVal := err.Error()
		t.Fatalf("failed to encode block extra data: %s", errVal)
		return
	}

	if bytes.Compare(origExtraData, DefaultExtraData) != 0 {
		return
	}

	if decoded.ErrorTransactions.IsEqualTo(errorTransactions) == false {
		t.Fatalf("SkippedTransactions check fail")
		return
	}

	verified, err := VerifyExtraData(blockNumber, encoded)
	if err != nil {
		errVal := err.Error()
		t.Fatalf("failed to encode block extra data: %s", errVal)
		return
	}

	if verified.ErrorTransactions.IsEqualTo(errorTransactions) == false {
		t.Fatalf("SkippedTransactions check fail")
		return
	}
}

func TestExtraData_encode_decode_negative(t *testing.T) {
	var errorTransactions types.Transactions
	blockNumber := defaults.DefaultConfig.DeepCheckStartBlock

	extraDataDummy := []byte{1, 2, 3}

	_, err := EncodeBlockExtraData(errorTransactions, extraDataDummy, blockNumber)
	if err == nil {
		t.Fatalf("EncodeBlockExtraData passed incorrectly")
		return
	}

	_, err = VerifyExtraData(blockNumber, make([]byte, 0))
	if err == nil {
		t.Fatalf("VerifyExtraData incorrectly 1")
		return
	}

	_, err = VerifyExtraData(blockNumber, make([]byte, len(DefaultExtraData)-1))
	if err == nil {
		t.Fatalf("VerifyExtraData incorrectly 2")
		return
	}

	_, err = VerifyExtraData(blockNumber, make([]byte, len(DefaultExtraData)+1))
	if err == nil {
		t.Fatalf("VerifyExtraData incorrectly 2")
		return
	}

	_, err = VerifyExtraData(blockNumber, make([]byte, len(DefaultExtraData)+10000))
	if err == nil {
		t.Fatalf("VerifyExtraData incorrectly 2")
		return
	}
}

func runExtraDataV3RoundTrip(t *testing.T, errorTransactions types.Transactions) {
	blockNumber := defaults.DefaultConfig.PosConfig.ExtraDataV3StartBlock

	encoded, err := EncodeBlockExtraData(errorTransactions, make([]byte, 0), blockNumber)
	if err != nil {
		t.Fatalf("failed to encode v3 block extra data: %s", err.Error())
		return
	}

	// V3 Extra must NOT contain the DefaultExtraData vanity prefix.
	if len(encoded) >= len(DefaultExtraData) {
		t.Fatalf("v3 encoded extra data unexpectedly large: %d", len(encoded))
		return
	}

	decoded, origExtraData, err := DecodeBlockExtraData(encoded, blockNumber)
	if err != nil {
		t.Fatalf("failed to decode v3 block extra data: %s", err.Error())
		return
	}

	if len(origExtraData) != 0 {
		t.Fatalf("v3 origExtraData should be empty, got %d", len(origExtraData))
		return
	}

	if decoded.ErrorTransactions.IsEqualTo(errorTransactions) == false {
		t.Fatalf("v3 ErrorTransactions round-trip check fail")
		return
	}

	verified, err := VerifyExtraData(blockNumber, encoded)
	if err != nil {
		t.Fatalf("failed to verify v3 block extra data: %s", err.Error())
		return
	}

	if verified.ErrorTransactions.IsEqualTo(errorTransactions) == false {
		t.Fatalf("v3 verified ErrorTransactions check fail")
		return
	}
}

func TestExtraData_v3_encode_decode(t *testing.T) {
	var emptyErrorTransactions types.Transactions
	runExtraDataV3RoundTrip(t, emptyErrorTransactions)

	to := common.Address{}
	nonEmptyErrorTransactions := types.Transactions{
		types.NewTransaction(0, to, big.NewInt(100), 21000, big.NewInt(1), nil),
		types.NewTransaction(1, to, big.NewInt(200), 21000, big.NewInt(2), nil),
	}
	runExtraDataV3RoundTrip(t, nonEmptyErrorTransactions)
}

func TestExtraData_v3_encode_decode_negative(t *testing.T) {
	blockNumber := defaults.DefaultConfig.PosConfig.ExtraDataV3StartBlock

	// Non-empty currentExtraData is invalid for v3 (no vanity prefix allowed).
	var errorTransactions types.Transactions
	_, err := EncodeBlockExtraData(errorTransactions, []byte{1, 2, 3}, blockNumber)
	if err == nil {
		t.Fatalf("EncodeBlockExtraData v3 passed incorrectly with non-empty currentExtraData")
		return
	}

	// Garbage bytes are not valid RLP for BlockExtraData.
	garbage := []byte{1, 2, 3}
	_, _, err = DecodeBlockExtraData(garbage, blockNumber)
	if err == nil {
		t.Fatalf("DecodeBlockExtraData v3 passed incorrectly with garbage")
		return
	}

	_, err = VerifyExtraData(blockNumber, garbage)
	if err == nil {
		t.Fatalf("VerifyExtraData v3 passed incorrectly with garbage")
		return
	}

	// A full-length DefaultExtraData-style blob is not valid v3 RLP.
	_, err = VerifyExtraData(blockNumber, DefaultExtraData)
	if err == nil {
		t.Fatalf("VerifyExtraData v3 passed incorrectly with DefaultExtraData blob")
		return
	}
}
