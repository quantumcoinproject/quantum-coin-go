package main

import (
	"math/big"
	"strings"
	"testing"
)

func coins(whole int64, fracWei int64) *big.Int {
	w := new(big.Int).Mul(big.NewInt(whole), reconcileWeiPerToken)
	return w.Add(w, big.NewInt(fracWei))
}

func TestNetWithdrawableWeiKeepsFractionalCoins(t *testing.T) {
	half := new(big.Int).Div(reconcileWeiPerToken, big.NewInt(2)) // 0.5 coin
	cases := []struct {
		name     string
		rewards  *big.Int
		slash    *big.Int
		wantWei  *big.Int
		wantNote string
	}{
		// Regression: the old code truncated both sides to whole coins first, so 0.5 coin of
		// rewards became 0 and was rejected as "invalid depositor amount".
		{"sub-coin rewards are withdrawable", half, big.NewInt(0), half, ""},
		{"1 wei of rewards is withdrawable", big.NewInt(1), big.NewInt(0), big.NewInt(1), ""},
		// Regression: 2.9 - 1.1 used to yield floor(2.9) - floor(1.1) = 1 coin; it is 1.8.
		{"fractional difference is exact", coins(2, 900_000_000_000_000_000), coins(1, 100_000_000_000_000_000), coins(1, 800_000_000_000_000_000), ""},
		{"whole coins unchanged", coins(10, 0), coins(3, 0), coins(7, 0), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := netWithdrawableWei(tc.rewards, tc.slash)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Cmp(tc.wantWei) != 0 {
				t.Fatalf("net = %s, want %s", got, tc.wantWei)
			}
		})
	}
}

func TestNetWithdrawableWeiRejectsNothingToWithdraw(t *testing.T) {
	cases := []struct {
		name    string
		rewards *big.Int
		slash   *big.Int
		wantErr string
	}{
		{"slashings equal rewards", coins(5, 0), coins(5, 0), "no rewards available"},
		{"slashings exceed rewards", coins(5, 0), coins(5, 1), "no rewards available"},
		{"slashings exceed sub-coin rewards", big.NewInt(3), big.NewInt(4), "no rewards available"},
		{"zero rewards", big.NewInt(0), big.NewInt(0), "no rewards available"},
		{"nil rewards", nil, big.NewInt(0), "invalid depositor amount"},
		{"nil slashings", coins(1, 0), nil, "invalid depositor amount"},
		{"negative rewards", big.NewInt(-1), big.NewInt(0), "invalid depositor amount"},
		{"negative slashings", coins(1, 0), big.NewInt(-1), "invalid depositor amount"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := netWithdrawableWei(tc.rewards, tc.slash)
			if err == nil {
				t.Fatalf("expected error, got %s", got)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// initiatePartialWithdrawalWei must refuse a non-positive amount before it touches the
// network (no key or RPC endpoint is needed to reach the check).
func TestInitiatePartialWithdrawalWeiRejectsNonPositive(t *testing.T) {
	for _, amt := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1)} {
		err := initiatePartialWithdrawalWei(nil, amt)
		if err == nil || !strings.Contains(err.Error(), "invalid withdrawal amount") {
			t.Fatalf("amount %v: expected 'invalid withdrawal amount', got %v", amt, err)
		}
	}
}
