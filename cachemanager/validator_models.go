package cachemanager

import "github.com/quantumcoinproject/quantum-coin-go/consensus/proofofstake"

type ValidatorCompact struct {
	Depositor               string `json:"depositor"     gencodec:"required"`
	Validator               string `json:"validator"     gencodec:"required"`
	Balance                 string `json:"balance"       gencodec:"required"`
	NetBalance              string `json:"netBalance"    gencodec:"required"`
	BlockRewards            string `json:"blockRewards"  gencodec:"required"`
	Slashings               string `json:"slashings"  gencodec:"required"`
	IsValidationPaused      bool   `json:"isValidationPaused"  gencodec:"required"`
	WithdrawalBlock         string `json:"withdrawalBlock"  gencodec:"required"`
	WithdrawalAmount        string `json:"withdrawalAmount"  gencodec:"required"`
	LastNiLBlock            string `json:"lastNiLBlock" gencodec:"required"`
	NilBlockCount           string `json:"nilBlockCount" gencodec:"required"`
	BlockProposerResetBlock string `json:"blockProposerResetBlock" gencodec:"required"`
	ValidatorResetBlock     string `json:"validatorResetBlock" gencodec:"required"`
}

func fromValidatorDetails(validatorDetails *proofofstake.ValidatorDetails) *ValidatorCompact {
	return &ValidatorCompact{
		Depositor:               validatorDetails.Depositor.HexLower(),
		Validator:               validatorDetails.Validator.HexLower(),
		Balance:                 validatorDetails.Balance,
		NetBalance:              validatorDetails.NetBalance,
		BlockRewards:            validatorDetails.BlockRewards,
		Slashings:               validatorDetails.Slashings,
		IsValidationPaused:      validatorDetails.IsValidationPaused,
		WithdrawalBlock:         validatorDetails.WithdrawalBlock,
		WithdrawalAmount:        validatorDetails.WithdrawalAmount,
		LastNiLBlock:            validatorDetails.LastNiLBlock,
		NilBlockCount:           validatorDetails.NilBlockCount,
		BlockProposerResetBlock: validatorDetails.BlockProposerResetBlock,
		ValidatorResetBlock:     validatorDetails.ValidatorResetBlock,
	}
}

type ValidatorList struct {
	Validators []ValidatorCompact `json:"validators"`
}

type ListValidatorsResponse struct {
	PageCount uint64             `json:"pageCount"`
	Items     []ValidatorCompact `json:"items"`
}

type ValidatorReport struct {
	TotalValidators uint64 `json:"totalValidators,omitempty"`
	ReportDate      int64  `json:"reportDate,omitempty"`
}
