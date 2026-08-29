package token

import (
	"math/big"
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

// Wrapped-Q wraps/unwraps emit Deposit/Withdrawal, never Transfer. They must
// surface as mint/burn legs so the account's holding is refreshed; ordinary
// Transfer parsing must be untouched.
func TestParseTokenTransactionWQEvents(t *testing.T) {
	wq := common.HexToAddress("0x" + strings.Repeat("ab", 32))
	account := common.HexToAddress("0x" + strings.Repeat("11", 32))
	amount := big.NewInt(3_000_000_000_000_000_000) // 3 Q in wei
	word := make([]byte, 32)
	amount.FillBytes(word)

	tx := types.NewTransaction(0, wq, big.NewInt(0), 21000, big.NewInt(1), nil)
	mkLog := func(sig string) *types.Log {
		return &types.Log{
			Address: wq,
			Topics:  []common.Hash{crypto.Keccak256Hash([]byte(sig)), common.HexToHash(account.Hex())},
			Data:    word,
		}
	}
	receipt := &types.Receipt{TxHash: tx.Hash(), Logs: []*types.Log{
		mkLog("Deposit(address,uint256)"),
		mkLog("Withdrawal(address,uint256)"),
	}}

	transfers, approvals, err := ParseTokenTransaction(tx, receipt)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(approvals) != 0 {
		t.Errorf("no approvals expected, got %d", len(approvals))
	}
	if len(transfers) != 2 {
		t.Fatalf("want 2 synthetic legs, got %d", len(transfers))
	}
	zero := common.Address{}
	mint, burn := transfers[0], transfers[1]
	if !mint.ContractAddress.IsEqualTo(wq) || !mint.From.IsEqualTo(zero) || !mint.To.IsEqualTo(account) || mint.Tokens.Cmp(amount) != 0 {
		t.Errorf("wrap leg wrong: %+v", mint)
	}
	if !burn.ContractAddress.IsEqualTo(wq) || !burn.From.IsEqualTo(account) || !burn.To.IsEqualTo(zero) || burn.Tokens.Cmp(amount) != 0 {
		t.Errorf("unwrap leg wrong: %+v", burn)
	}

	// A malformed Deposit (no indexed account) is ignored, not mis-parsed.
	bad := &types.Receipt{TxHash: tx.Hash(), Logs: []*types.Log{{
		Address: wq, Topics: []common.Hash{crypto.Keccak256Hash([]byte("Deposit(address,uint256)"))}, Data: word,
	}}}
	transfers, _, err = ParseTokenTransaction(tx, bad)
	if err != nil || len(transfers) != 0 {
		t.Errorf("malformed deposit: want 0 legs, got %d (err %v)", len(transfers), err)
	}
}

// Plain Transfer parsing is unchanged by the WQ cases.
func TestParseTokenTransactionTransferUnchanged(t *testing.T) {
	tok := common.HexToAddress("0x" + strings.Repeat("cd", 32))
	from := common.HexToAddress("0x" + strings.Repeat("11", 32))
	to := common.HexToAddress("0x" + strings.Repeat("22", 32))
	amount := big.NewInt(42)
	word := make([]byte, 32)
	amount.FillBytes(word)
	tx := types.NewTransaction(0, tok, big.NewInt(0), 21000, big.NewInt(1), nil)
	receipt := &types.Receipt{TxHash: tx.Hash(), Logs: []*types.Log{{
		Address: tok,
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")), common.HexToHash(from.Hex()), common.HexToHash(to.Hex())},
		Data:    word,
	}}}
	transfers, _, err := ParseTokenTransaction(tx, receipt)
	if err != nil || len(transfers) != 1 {
		t.Fatalf("want 1 transfer, got %d (err %v)", len(transfers), err)
	}
	if !transfers[0].From.IsEqualTo(from) || !transfers[0].To.IsEqualTo(to) || transfers[0].Tokens.Cmp(amount) != 0 {
		t.Errorf("transfer leg wrong: %+v", transfers[0])
	}
}
