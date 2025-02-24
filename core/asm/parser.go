package asm

import (
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/core/vm"
	"github.com/QuantumCoinProject/qc/log"
)

const FunctionSignatureLength = 4
const PlainHex = "0xffffffff"

var Erc20Methods = []string{"0x18160ddd", "0x70a08231", "0xdd62ed3e", "0xa9059cbb", "0x095ea7b3", "0x23b872dd"}

func ParseMethodList(runtimeBinCode string) (map[string]bool, error) {

	methodList := make(map[string]bool)

	byteCodeArr, err := hexutil.Decode(runtimeBinCode)
	if err != nil {
		return nil, err
	}

	looper := NewInstructionIterator(byteCodeArr)
	for looper.Next() {
		if looper.arg == nil || len(looper.arg) != FunctionSignatureLength || looper.Op() != vm.PUSH4 {
			continue
		}
		encodedArg := hexutil.Encode(looper.arg)
		if encodedArg == PlainHex {
			continue
		}

		methodList[encodedArg] = true
	}

	return methodList, nil
}

func IsErc20(runtimeBinCode string) bool {
	methodList, err := ParseMethodList(runtimeBinCode)
	if err != nil {
		log.Debug("ParseMethodList", "error", err)
		return false
	}
	for _, method := range Erc20Methods {
		_, ok := methodList[method]
		if ok == false {
			return false
		}
	}

	return true
}
