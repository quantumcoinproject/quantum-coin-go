package proofofstake

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"testing"
)

func TestExtraData_basic(t *testing.T) {
	_, err := common.Hex2BytesWithErrorCheck(DefaultExtraDataHex)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("default", common.Hex2Bytes(DefaultExtraDataHex), "len", len(DefaultExtraData))
}
