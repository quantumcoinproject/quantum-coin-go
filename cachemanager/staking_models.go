package cachemanager

type StakingReport struct {
	TotalStakedCoins string `json:"totalStakedCoins,omitempty"`
	ReportDate       int64  `json:"reportDate,omitempty"`
}
