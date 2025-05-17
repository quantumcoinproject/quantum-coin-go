package cachemanager

import "github.com/quantumcoinproject/quantum-coin-go/ethclient"

type AccountDetails struct {
	Address string                `json:"address,omitempty"`
	AccType ethclient.AccountType `json:"accountType,omitempty"`
	Balance string                `json:"balance,omitempty"`
	Nonce   uint64                `json:"nonce,omitempty"`
	Code    string                `json:"code,omitempty"`
}
