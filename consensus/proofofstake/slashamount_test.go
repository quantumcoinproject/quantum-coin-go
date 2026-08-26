package proofofstake

import (
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// slashScheduleCases lists the block numbers around the SlashV2StartBlock cutoff
// together with the amount Finalize applies there.
func slashScheduleCases(t *testing.T) []struct {
	name  string
	block uint64
	want  *big.Int
} {
	t.Helper()
	cfg := defaults.DefaultConfig.PosConfig
	if cfg.SlashStartBlockNumber >= cfg.SlashV2StartBlock {
		t.Fatalf("schedule sanity: SlashStartBlockNumber %d must precede SlashV2StartBlock %d", cfg.SlashStartBlockNumber, cfg.SlashV2StartBlock)
	}
	if cfg.SLASH_AMOUNT.Cmp(cfg.SLASH_AMOUNT_V2) == 0 {
		t.Fatalf("schedule sanity: SLASH_AMOUNT and SLASH_AMOUNT_V2 must differ for the test to be meaningful")
	}
	return []struct {
		name  string
		block uint64
		want  *big.Int
	}{
		{"SlashStartBlockNumber", cfg.SlashStartBlockNumber, cfg.SLASH_AMOUNT},
		{"SlashV2StartBlockMinusOne", cfg.SlashV2StartBlock - 1, cfg.SLASH_AMOUNT},
		{"SlashV2StartBlock", cfg.SlashV2StartBlock, cfg.SLASH_AMOUNT_V2},
		{"SlashV2StartBlockPlusOne", cfg.SlashV2StartBlock + 1, cfg.SLASH_AMOUNT_V2},
	}
}

// TestGetSlashAmountSchedule pins GetSlashAmount to the SlashV2StartBlock cutoff
// exactly as Finalize applies it: SLASH_AMOUNT strictly below the cutoff,
// SLASH_AMOUNT_V2 from the cutoff onward.
func TestGetSlashAmountSchedule(t *testing.T) {
	for _, tc := range slashScheduleCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetSlashAmount(tc.block); got.Cmp(tc.want) != 0 {
				t.Fatalf("GetSlashAmount(%d) = %s, want %s", tc.block, got, tc.want)
			}
		})
	}
}

// TestGetRewardsSlashingsByVoteUsesSlashSchedule checks the by-vote helper reports
// the same slash amount as GetSlashAmount for round-1 nil votes and nothing otherwise.
func TestGetRewardsSlashingsByVoteUsesSlashSchedule(t *testing.T) {
	for _, tc := range slashScheduleCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			bn := new(big.Int).SetUint64(tc.block)
			rewards, slashed := GetRewardsSlashingsByVote(bn, VOTE_TYPE_NIL, 1)
			if rewards.Sign() != 0 {
				t.Fatalf("nil vote must not reward, got %s", rewards)
			}
			if slashed.Cmp(tc.want) != 0 {
				t.Fatalf("round-1 nil vote at %d slashed %s, want %s", tc.block, slashed, tc.want)
			}
			if _, slashed := GetRewardsSlashingsByVote(bn, VOTE_TYPE_NIL, 2); slashed.Sign() != 0 {
				t.Fatalf("round-2 nil vote must not slash, got %s", slashed)
			}
			if _, slashed := GetRewardsSlashingsByVote(bn, VOTE_TYPE_OK, 1); slashed.Sign() != 0 {
				t.Fatalf("ok vote must not slash, got %s", slashed)
			}
		})
	}
	// Below SlashStartBlockNumber nothing is slashed at all.
	if start := defaults.DefaultConfig.PosConfig.SlashStartBlockNumber; start > 0 {
		if _, slashed := GetRewardsSlashingsByVote(new(big.Int).SetUint64(start-1), VOTE_TYPE_NIL, 1); slashed.Sign() != 0 {
			t.Fatalf("before SlashStartBlockNumber nothing must be slashed, got %s", slashed)
		}
	}
}

// TestParseRewardsInfoSlashAmountMatchesFinalize builds a round-1 nil-vote block on
// either side of SlashV2StartBlock and checks the RPC-reported per-validator and
// total slash amounts equal what Finalize applies (GetSlashAmount). This pins the
// regression where ParseRewardsInfo had the cutoff comparison inverted and reported
// SLASH_AMOUNT where SLASH_AMOUNT_V2 had been applied (and vice versa).
func TestParseRewardsInfoSlashAmountMatchesFinalize(t *testing.T) {
	slashed := []common.Address{randAddress(), randAddress(), randAddress()}
	for _, tc := range slashScheduleCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			consensusData := &BlockConsensusData{
				VoteType:              VOTE_TYPE_NIL,
				Round:                 1,
				SlashedBlockProposers: slashed,
				SelectedTransactions:  make([]common.Hash, 0),
			}
			encoded, err := rlp.EncodeToBytes(consensusData)
			if err != nil {
				t.Fatalf("EncodeToBytes: %v", err)
			}
			header := &types.Header{
				Number:        new(big.Int).SetUint64(tc.block),
				Difficulty:    big.NewInt(1),
				ConsensusData: encoded,
			}
			block := types.NewBlock(header, nil, nil, trie.NewStackTrie(nil))

			info, err := ParseRewardsInfo(block, nil)
			if err != nil {
				t.Fatalf("ParseRewardsInfo: %v", err)
			}
			if got := len(info.SlashedValidators); got != len(slashed) {
				t.Fatalf("SlashedValidators len = %d, want %d", got, len(slashed))
			}
			for i, s := range info.SlashedValidators {
				if !s.SlashedValidator.IsEqualTo(slashed[i]) {
					t.Fatalf("SlashedValidators[%d] = %s, want %s", i, s.SlashedValidator.Hex(), slashed[i].Hex())
				}
				amt, err := hexutil.DecodeBig(s.SlashedAmount)
				if err != nil {
					t.Fatalf("SlashedAmount decode: %v", err)
				}
				if amt.Cmp(tc.want) != 0 {
					t.Fatalf("block %d: reported SlashedAmount %s, Finalize applies %s", tc.block, amt, tc.want)
				}
			}
			total, err := hexutil.DecodeBig(info.SlashAmount)
			if err != nil {
				t.Fatalf("SlashAmount decode: %v", err)
			}
			wantTotal := new(big.Int).Mul(tc.want, big.NewInt(int64(len(slashed))))
			if total.Cmp(wantTotal) != 0 {
				t.Fatalf("block %d: reported total SlashAmount %s, want %s", tc.block, total, wantTotal)
			}
			if rewards, err := hexutil.DecodeBig(info.BlockProposerRewards); err != nil || rewards.Sign() != 0 {
				t.Fatalf("nil-vote block must report zero proposer rewards, got %q (%v)", info.BlockProposerRewards, err)
			}
		})
	}
}

func rewardsInfoForConsensusData(t *testing.T, block uint64, cd *BlockConsensusData) *BlockRewardsInfo {
	t.Helper()
	encoded, err := rlp.EncodeToBytes(cd)
	if err != nil {
		t.Fatalf("EncodeToBytes: %v", err)
	}
	header := &types.Header{Number: new(big.Int).SetUint64(block), Difficulty: big.NewInt(1), ConsensusData: encoded}
	info, err := ParseRewardsInfo(types.NewBlock(header, nil, nil, trie.NewStackTrie(nil)), nil)
	if err != nil {
		t.Fatalf("ParseRewardsInfo: %v", err)
	}
	return info
}

func assertNoSlashReported(t *testing.T, info *BlockRewardsInfo) {
	t.Helper()
	if len(info.SlashedValidators) != 0 {
		t.Fatalf("expected no slashed validators, got %d", len(info.SlashedValidators))
	}
	if info.SlashAmount != "" {
		if amt, err := hexutil.DecodeBig(info.SlashAmount); err != nil || amt.Sign() != 0 {
			t.Fatalf("expected no slash amount, got %q", info.SlashAmount)
		}
	}
}

// TestParseRewardsInfoNoSlashOutsideRoundOneNil: slashing is reported only for round-1
// nil-vote blocks with a non-empty proposer list at or after SlashStartBlockNumber;
// every other shape must report nothing.
func TestParseRewardsInfoNoSlashOutsideRoundOneNil(t *testing.T) {
	cfg := defaults.DefaultConfig.PosConfig
	proposers := []common.Address{randAddress()}
	at := cfg.SlashV2StartBlock + 1
	cases := []struct {
		name  string
		block uint64
		cd    *BlockConsensusData
	}{
		{"round 2 nil vote", at, &BlockConsensusData{VoteType: VOTE_TYPE_NIL, Round: 2, SlashedBlockProposers: proposers, SelectedTransactions: []common.Hash{}}},
		{"round 1 nil vote, empty proposer list", at, &BlockConsensusData{VoteType: VOTE_TYPE_NIL, Round: 1, SlashedBlockProposers: []common.Address{}, SelectedTransactions: []common.Hash{}}},
		{"round 1 nil vote, nil proposer list", at, &BlockConsensusData{VoteType: VOTE_TYPE_NIL, Round: 1, SelectedTransactions: []common.Hash{}}},
		{"ok vote with proposer list", at, &BlockConsensusData{VoteType: VOTE_TYPE_OK, Round: 1, SlashedBlockProposers: proposers, SelectedTransactions: []common.Hash{}}},
	}
	if cfg.SlashStartBlockNumber > 0 {
		cases = append(cases, struct {
			name  string
			block uint64
			cd    *BlockConsensusData
		}{"round 1 nil vote before SlashStartBlockNumber", cfg.SlashStartBlockNumber - 1,
			&BlockConsensusData{VoteType: VOTE_TYPE_NIL, Round: 1, SlashedBlockProposers: proposers, SelectedTransactions: []common.Hash{}}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertNoSlashReported(t, rewardsInfoForConsensusData(t, tc.block, tc.cd))
		})
	}
}

// TestParseRewardsInfoRejectsMalformedConsensusData: undecodable consensus data is an
// error, never an empty (zero-slash) report.
func TestParseRewardsInfoRejectsMalformedConsensusData(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"garbage", []byte{0xff, 0x00, 0x13}},
		{"truncated list", []byte{0xc3, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := &types.Header{Number: new(big.Int).SetUint64(defaults.DefaultConfig.PosConfig.SlashV2StartBlock), Difficulty: big.NewInt(1), ConsensusData: tc.data}
			info, err := ParseRewardsInfo(types.NewBlock(header, nil, nil, trie.NewStackTrie(nil)), nil)
			if err == nil {
				t.Fatalf("expected error for malformed consensus data, got info=%+v", info)
			}
		})
	}
}
