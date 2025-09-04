package cachemanager

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/conversion"
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
		log.Error("error Decode", "err", err)
		return
	}
	cnv := Convert{}
	err = contractAbi1.UnpackIntoInterface(&cnv, "OnRequestConversion", data)
	if err != nil {
		log.Error("error OnRequestConversion", "err", err)
		return
	}
	fmt.Println(cnv.EthAddress)
}
