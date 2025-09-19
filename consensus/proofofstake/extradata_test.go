package proofofstake

import (
	"bytes"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"testing"
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
		t.Fatalf(err.Error())
		return
	}

	decoded, origExtraData, err := DecodeBlockExtraData(encoded, blockNumber)
	if err != nil {
		t.Fatalf(err.Error())
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
		t.Fatalf(err.Error())
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
