package main

import (
	"math/big"
	"testing"
)

func mustWei(t *testing.T, s string) *big.Int {
	t.Helper()
	w, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad wei literal %q", s)
	}
	return w
}

func TestDecimalStrToWei(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"988,020,990,000", "988020990000000000000000000000", false},
		{"4,149,334,042.861292810722256057", "4149334042861292810722256057", false},
		{"939,228,519.45683502", "939228519456835020000000000", false},
		{"0", "0", false},
		{"1.5", "1500000000000000000", false},
		{".5", "500000000000000000", false},
		{"1.1234567890123456789", "", true},  // >18 fractional digits
		{"12a", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := decimalStrToWei(c.in)
		if c.err {
			if err == nil {
				t.Errorf("decimalStrToWei(%q): expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("decimalStrToWei(%q): unexpected error %v", c.in, err)
			continue
		}
		if got.Cmp(mustWei(t, c.want)) != 0 {
			t.Errorf("decimalStrToWei(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestWeiToTokenStr(t *testing.T) {
	cases := []struct {
		wei  string
		want string
	}{
		{"988020990000000000000000000000", "988020990000"},
		{"4149334042861292810722256057", "4149334042.861292810722256057"},
		{"1500000000000000000", "1.5"},
		{"500000000000000000", "0.5"},
		{"0", "0"},
	}
	for _, c := range cases {
		if got := weiToTokenStr(mustWei(t, c.wei)); got != c.want {
			t.Errorf("weiToTokenStr(%s) = %q, want %q", c.wei, got, c.want)
		}
	}
}

// The payout csv feeds dputil multitransfertokens, whose float path must
// round-trip our exact 18-dp amount strings back to the identical wei value.
func TestPayoutRoundTrip(t *testing.T) {
	weis := []string{
		"988020990000000000000000000000",
		"4149334042861292810722256057",
		"1298123816311818117513535232418",
		"1",
		"999999999999999999",
		"651505538054017931787953440772",
	}
	for _, w := range weis {
		wei := mustWei(t, w)
		s := weiToTokenStr18(wei)
		parsed, err := ParseBigFloat(s)
		if err != nil {
			t.Fatalf("ParseBigFloat(%q): %v", s, err)
		}
		back := etherToWeiFloat(parsed)
		if back.Cmp(wei) != 0 {
			t.Errorf("round-trip %s -> %q -> %s", w, s, back)
		}
	}
}

// Observed real pair: snapshot 989,010,000,000 tokens; the dead address
// received 988,020,990,000 (exactly 99.9%, DogeP's 0.1% transfer fee). The
// grossed-up burn must fall inside the default 10 bps tolerance band.
func TestFeeGrossUpTolerance(t *testing.T) {
	snapshot := mustWei(t, "989010000000000000000000000000")
	burned := mustWei(t, "988020990000000000000000000000")
	gross := new(big.Int).Quo(new(big.Int).Mul(burned, big.NewInt(1000)), big.NewInt(999))

	tol := int64(10)
	gross10k := new(big.Int).Mul(gross, big.NewInt(10000))
	low := new(big.Int).Mul(snapshot, big.NewInt(10000-tol))
	high := new(big.Int).Mul(snapshot, big.NewInt(10000+tol))
	if gross10k.Cmp(low) < 0 {
		t.Errorf("gross %s below tolerance vs snapshot %s (would flag PARTIAL_BURN)", gross, snapshot)
	}
	if gross10k.Cmp(high) > 0 {
		t.Errorf("gross %s above tolerance vs snapshot %s (would flag AMOUNT_MISMATCH)", gross, snapshot)
	}
}
