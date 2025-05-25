package proofofstake

import (
	"errors"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

type BlockExtraData struct {
	SkippedTransactions types.Transactions `json:"skippedTransactions" gencodec:"required"`
	ErrorTransactions   types.Transactions `json:"errorTransactions" gencodec:"required"`
	ExtraData           []byte             `json:"extraData" gencodec:"required"`
}

func EncodeBlockExtraData(skippedTransactions types.Transactions, errorTransactions types.Transactions, currentExtraData []byte) ([]byte, error) {
	blockExtraData := BlockExtraData{
		SkippedTransactions: skippedTransactions,
		ExtraData:           make([]byte, 0),
		ErrorTransactions:   errorTransactions,
	}

	data, err := rlp.EncodeToBytes(&blockExtraData)
	if err != nil {
		log.Error("EncodeBlockExtraData", "error", err)
		return nil, err
	}

	return append(currentExtraData, data...), nil
}

func DecodeBlockExtraData(extraData []byte) (*BlockExtraData, error) {
	if len(extraData) < extraDataBaseLen+1 {
		log.Error("DecodeBlockExtraData", "extraData length invalid", len(extraData))
		return nil, errors.New("invalid ExtraData")
	}
	blockExtraData := BlockExtraData{}

	data := extraData[extraDataBaseLen+1:]

	err := rlp.DecodeBytes(data, &blockExtraData)
	if err != nil {
		log.Error("DecodeBlockExtraData", "error", err)
		return nil, err
	}

	return &blockExtraData, nil
}
