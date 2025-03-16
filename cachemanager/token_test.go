package cachemanager

import (
	"fmt"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/systemcontracts/conversion"
	"strings"
	"testing"
)

func TestHex(t *testing.T) {
	fmt.Println(logTransferSigHash)
	fmt.Println(logApprovalSigHash)
}

type Convert struct {
	EthAddress        string
	EthereumSignature string
}

var contractAbi1, _ = abi.JSON(strings.NewReader(string(conversion.ConversionMetaData.ABI)))

func TestConv(t *testing.T) {
	//Enter hex data from getTransactionReceipt below
	data, err := hexutil.Decode("")
	if err != nil {
		t.Fatalf("failed a")
	}
	cnv := Convert{}
	err = contractAbi1.UnpackIntoInterface(&cnv, "OnRequestConversion", data)
	if err != nil {
		fmt.Println(err)
		t.Fatalf("failed b")
	}
	fmt.Println(cnv.EthAddress)
}
