package proofofstake

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

// The consensus amount subtractions were switched to common.SafeSubBigIntNonNegative,
// which panics on underflow. These tests drive each call site's guard to its boundary
// and past it to show the guard, not the panic, handles the edge.

func TestOfflineValidatorPenaltyNeverUnderflows(t *testing.T) {
	v4 := defaults.DefaultConfig.PosConfig.OfflineValidatorV4StartBlock
	deposit := big.NewInt(1000)
	cases := []struct {
		name       string
		nilBlocks  int64
		block      uint64
		wantAmount int64
	}{
		{"before V4 start block: unchanged", 49, v4 - 1, 1000},
		{"nil count 2: no penalty", 2, v4, 1000},
		{"nil count 3: 6%", 3, v4, 940},
		{"nil count 49: 98%", 49, v4, 20},
		{"nil count 50: exactly 100% -> zero, no underflow", 50, v4, 0},
		{"nil count 51: over 100% -> zero, no underflow", 51, v4, 0},
		{"nil count 1000: far over -> zero, no underflow", 1000, v4 + 10, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if v4 == 0 && tc.block == v4-1 {
				t.Skip("V4 active from genesis in this config")
			}
			val := &ValidatorDetailsV2{Validator: common.Address{}, NilBlockCount: big.NewInt(tc.nilBlocks)}
			got := getOfflineValidatorDepositAfterPenalty(val, tc.block, deposit)
			if got.Int64() != tc.wantAmount {
				t.Fatalf("deposit after penalty = %s, want %d", got, tc.wantAmount)
			}
			if got.Sign() < 0 {
				t.Fatalf("deposit went negative: %s", got)
			}
			if deposit.Int64() != 1000 {
				t.Fatalf("input deposit was mutated: %s", deposit)
			}
		})
	}
}

func TestCalculateTxnFeeSplitCoinsNeverUnderflows(t *testing.T) {
	pct := defaults.DefaultConfig.PosConfig.TxnFeeRewardsPercentage
	if pct < 0 || pct > 100 {
		t.Fatalf("TxnFeeRewardsPercentage %d outside [0,100]; burn = total - rewards would underflow", pct)
	}
	huge, _ := new(big.Int).SetString("1000000000000000000000007", 10)
	for _, total := range []*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(2), big.NewInt(99), big.NewInt(100), big.NewInt(101), huge} {
		burn, rewards := calculateTxnFeeSplitCoins(total)
		if burn.Sign() < 0 || rewards.Sign() < 0 {
			t.Fatalf("total %s: negative split burn=%s rewards=%s", total, burn, rewards)
		}
		if sum := new(big.Int).Add(burn, rewards); sum.Cmp(total) != 0 {
			t.Fatalf("total %s: burn %s + rewards %s != total", total, burn, rewards)
		}
		wantRewards := new(big.Int).Div(new(big.Int).Mul(total, big.NewInt(pct)), big.NewInt(100))
		if rewards.Cmp(wantRewards) != 0 {
			t.Fatalf("total %s: rewards %s, want %s", total, rewards, wantRewards)
		}
	}
}

func TestGetRewardBeforeRewardStartBlockIsZero(t *testing.T) {
	start := defaults.DefaultConfig.PosConfig.RewardStartBlockNumber
	if start == 0 {
		t.Skip("rewards start at genesis in this config")
	}
	// blockNumber < rewardStartBlock must short-circuit before the (guarded) subtraction.
	for _, bn := range []uint64{0, 1, start - 1} {
		if r := GetReward(new(big.Int).SetUint64(bn)); r.Sign() != 0 {
			t.Fatalf("GetReward(%d) = %s before RewardStartBlockNumber %d, want 0", bn, r, start)
		}
	}
	if r := GetReward(new(big.Int).SetUint64(start)); r.Sign() <= 0 {
		t.Fatalf("GetReward(%d) = %s at RewardStartBlockNumber, want > 0", start, r)
	}
}
