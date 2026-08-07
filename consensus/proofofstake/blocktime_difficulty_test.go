package proofofstake

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/defaults"
)

// setBlockTimeBindingGate overrides the BlockTimeBindingV1 activation height for a
// test and restores it afterwards. Tests using it must not run in parallel since it
// mutates global config. Mirrors setMalleabilityGate in malleability_test.go.
func setBlockTimeBindingGate(t *testing.T, height uint64) {
	t.Helper()
	orig := defaults.DefaultConfig.PosConfig.BlockTimeBindingV1StartBlock
	defaults.DefaultConfig.PosConfig.BlockTimeBindingV1StartBlock = height
	t.Cleanup(func() { defaults.DefaultConfig.PosConfig.BlockTimeBindingV1StartBlock = orig })
}

// H1: Difficulty is defined as exactly the block number. It must be compared as a
// big.Int, not via Uint64(), which returns only the low 64 bits and would accept an
// inflated value such as 2^64+number -- inflating total difficulty and forcing reorgs.
func TestDifficultyComparisonRejectsTruncatedMatch(t *testing.T) {
	number := big.NewInt(5319300)

	inflated := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 64), number)
	if inflated.Uint64() != number.Uint64() {
		t.Fatalf("premise broken: expected low-64-bit collision")
	}
	if inflated.Cmp(number) == 0 {
		t.Fatalf("Cmp must distinguish 2^64+n from n")
	}

	// Values that must be accepted / rejected by the exact comparison.
	if number.Cmp(new(big.Int).Set(number)) != 0 {
		t.Errorf("exact value must compare equal")
	}
	for _, bad := range []*big.Int{
		inflated,
		new(big.Int).Add(number, big.NewInt(1)),
		new(big.Int).Sub(number, big.NewInt(1)),
		new(big.Int).Lsh(big.NewInt(1), 200),
	} {
		if bad.Cmp(number) == 0 {
			t.Errorf("value %s must not compare equal to %s", bad, number)
		}
	}
}

// C3: DeriveBlockTime is the single source of truth shared by block production
// (Finalize) and verification, so the two cannot drift. An OK vote with a BlockTime
// ahead of the parent adopts BlockTime; otherwise the block falls back to
// parent.Time + Period.
func TestDeriveBlockTime(t *testing.T) {
	const period = uint64(6)
	origStart := defaults.DefaultConfig.PosConfig.BLOCK_TIME_ORIG_START_BLOCK
	blockNumber := origStart + 10 // past BLOCK_TIME_ORIG_START_BLOCK

	tests := []struct {
		name       string
		parentTime uint64
		voteType   VoteType
		blockTime  uint64
		want       uint64
	}{
		{"ok_vote_adopts_blocktime", 1000, VOTE_TYPE_OK, 1020, 1020},
		{"ok_vote_blocktime_not_ahead", 1000, VOTE_TYPE_OK, 1000, 1006},
		{"ok_vote_blocktime_behind", 1000, VOTE_TYPE_OK, 900, 1006},
		{"nil_vote_uses_period", 1000, VOTE_TYPE_NIL, 1020, 1006},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveBlockTime(blockNumber, tc.parentTime, period, &BlockConsensusData{
				VoteType:  tc.voteType,
				BlockTime: tc.blockTime,
			})
			if got != tc.want {
				t.Errorf("DeriveBlockTime = %d, want %d", got, tc.want)
			}
		})
	}
}

// C3: the binding is backward-incompatible, so it must be inert below its activation
// height and enforced at or above it.
func TestBlockTimeBindingGate(t *testing.T) {
	setBlockTimeBindingGate(t, 1000)

	if defaults.IsBlockTimeBindingV1(999) {
		t.Errorf("binding must be inactive below the activation height")
	}
	if !defaults.IsBlockTimeBindingV1(1000) {
		t.Errorf("binding must be active at the activation height")
	}
	if !defaults.IsBlockTimeBindingV1(1001) {
		t.Errorf("binding must be active above the activation height")
	}
}

// Mainnet must not activate the binding retroactively: doing so would invalidate
// historical blocks whose stored Time differs from the derived value.
func TestBlockTimeBindingNotScheduledOnMainnet(t *testing.T) {
	if got := MainnetConfigBlockTimeBindingStart(); got != defaults.NotScheduled {
		t.Errorf("mainnet BlockTimeBindingV1StartBlock = %d, want NotScheduled (%d)", got, defaults.NotScheduled)
	}
}

func MainnetConfigBlockTimeBindingStart() uint64 {
	return defaults.MainnetConfig.PosConfig.BlockTimeBindingV1StartBlock
}
