package relay

import "errors"

var (
	InfoTitleLatestBlockDetails      = "Get latest block details"
	InfoTitleAccountDetails          = "Get account details"
	InfoTitleTransaction             = "Get Transaction"
	InfoTitleSendTransaction         = "Send Transaction"
	InfoTitleListAccountTransactions = "List Account Transactions"
	InfoTitleGetBlockchainDetails    = "Get Blockchain details"
)

var (
	MsgDial               = "Dial"
	MsgAddress            = "Address"
	MsgBalance            = "Balance"
	MsgNonce              = "Nonce"
	MsgBlockNumber        = "Block number"
	MsgHash               = "Hash"
	MsgTransaction        = "Transaction"
	MsgTransactionReceipt = "Transaction receipt"
	MsgSend               = "Send"
	MsgRawRawTxHex        = "Raw tx hex"
	MsgRawTxData          = "Raw tx data"
	MsgTimeDuration       = "Time Duration"
	MsgStatus             = "Status"
	MsgError              = "Error"
)

var (
	ErrEmptyAddress   = errors.New("empty address")
	ErrInvalidAddress = errors.New("invalid address")
	ErrEmptyHash      = errors.New("empty hash")
	ErrInvalidHash    = errors.New("invalid hash")
	ErrEmptyRawTxHex  = errors.New("empty raw tx")
)

type RelayConfig struct {
	Api                string `json:"api"`
	Ip                 string `json:"ip"`
	Port               string `json:"port"`
	NodeUrl            string `json:"nodeUrl"`
	CorsAllowedOrigins string `json:"corsAllowedOrigins"`
	EnableAuth         bool   `json:"enableAuth"`
	ApiKeys            string `json:"apiKeys"`
	CachePath          string `json:"cachePath"`
	EnableExtendedApis bool   `json:"enableExtendedApis"`
	GenesisFilePath    string `json:"genesisFilePath"`
	MaxSupply          string `json:"maxSupply"`
}
