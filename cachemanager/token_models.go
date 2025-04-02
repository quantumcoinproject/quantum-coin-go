package cachemanager

type TokenDetails struct {
	ContractAddress        string `json:"contractAddress,omitempty"`
	CreatorAddress         string `json:"creatorAddress,omitempty"`
	CreatedBlockNumber     uint64 `json:"createdBlockNumber,omitempty"`
	CreatedTransactionHash string `json:"createdTransactionHash,omitempty"`
	Name                   string `json:"name,omitempty"`
	Symbol                 string `json:"symbol,omitempty"`
	TotalSupply            string `json:"totalSupply,omitempty"`
	Decimals               string `json:"decimals,omitempty"`
}

type GetTokenDetailsResponse struct {
	Result TokenDetails `json:"result,omitempty"`
}

type AccountTokenSummary struct {
	AccountAddress  string `json:"accountAddress,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"`
	Name            string `json:"name,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	TokenBalance    string `json:"tokenBalance,omitempty"`
}

type AccountTokenList struct {
	Address string                `json:"address"`
	Tokens  []AccountTokenSummary `json:"tokens"`
}

type ListAccountTokensResponse struct {
	PageCount      uint64                `json:"pageCount"`
	AccountAddress string                `json:"accountAddress"`
	Items          []AccountTokenSummary `json:"items"`
}

type AccountTokenTransactionList struct {
	Address      string                      `json:"address"`
	Transactions []AccountTransactionCompact `json:"transactions"`
}

type ListAccountTokenTransactionsResponse struct {
	PageCount uint64                      `json:"pageCount"`
	Items     []AccountTransactionCompact `json:"items"`
}
