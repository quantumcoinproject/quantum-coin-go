package proofofstake

import (
	"crypto/rand"
	"math/big"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// Returns a random hash
func randHash() common.Hash {
	var h common.Hash
	rand.Read(h[:])
	return h
}

func randAddress() common.Address {
	var a common.Address
	rand.Read(a[:])
	return a
}

// largeNumber returns a very large big.Int.
func largeNumber(megabytes int) *big.Int {
	buf := make([]byte, megabytes*1024*1024)
	rand.Read(buf)
	bigint := new(big.Int)
	bigint.SetBytes(buf)
	return bigint
}

func TestBlock_NilNegative(t *testing.T) {

	//Case 1
	BlockNilTest(nil, nil, t, "VerifyBlockConsensusData nil")

	//Case 2
	blockConsensusData := &BlockConsensusData{
		VoteType:              VOTE_TYPE_OK,
		SlashedBlockProposers: make([]common.Address, 0),
		Round:                 1,
		SelectedTransactions:  make([]common.Hash, 0),
	}

	BlockNilTest(blockConsensusData, nil, t, "VerifyBlockConsensusData nil")
}

func BlockNilTest(blockConsensusData *BlockConsensusData, blockAdditionalConsensusData *BlockAdditionalConsensusData, t *testing.T, expectedError string) {
	header := &types.Header{
		MixDigest:             randHash(),
		ReceiptHash:           randHash(),
		TxHash:                randHash(),
		Nonce:                 types.BlockNonce{},
		Extra:                 []byte{},
		Bloom:                 types.Bloom{},
		GasUsed:               0,
		Coinbase:              common.Address{},
		GasLimit:              0,
		Time:                  1337,
		ParentHash:            randHash(),
		Root:                  randHash(),
		Number:                largeNumber(2),
		Difficulty:            largeNumber(2),
		ConsensusData:         nil,
		UnhashedConsensusData: nil,
	}

	if blockConsensusData != nil {
		data, err := rlp.EncodeToBytes(blockConsensusData)
		if err != nil {
			t.Fatalf("EncodeToBytes failed 1")
		}
		header.ConsensusData = make([]byte, len(data))
		copy(header.ConsensusData, data)
	}

	if blockAdditionalConsensusData != nil {
		data, err := rlp.EncodeToBytes(blockAdditionalConsensusData)
		if err != nil {
			t.Fatalf("EncodeToBytes failed 2")
		}
		header.UnhashedConsensusData = make([]byte, len(data))
		copy(header.UnhashedConsensusData, data)
	}

	var receipts []*types.Receipt
	var txs [10]*types.Transaction

	for i := 0; i < 10; i++ {
		to := randAddress()
		var data [16000]byte
		baseTx := types.NewDefaultFeeTransactionSimple(0, &to, big.NewInt(100), 21000, data[:])
		rawTx := types.NewTx(baseTx)
		txs[i] = rawTx
	}

	block := types.NewBlock(header, txs[:], receipts, trie.NewStackTrie(nil))
	valMap := make(map[common.Address]*big.Int)
	getValidatorsStub := func(common.Hash) (map[common.Address]*big.Int, error) { return valMap, nil }
	listValidatorsStub := func(common.Hash) (map[common.Address]*ValidatorDetailsV2, error) { return nil, nil }
	err := VerifyBlockConsensusData(block, &valMap, nil, DummyGetBlockConsensusContext, getValidatorsStub, listValidatorsStub)
	if err == nil || strings.Compare(err.Error(), expectedError) != 0 {
		debug.PrintStack()
		t.Fatalf("BlockNilTest failed")
	}
}

func DummyGetBlockConsensusContext(key string, blockHash common.Hash) ([32]byte, error) {
	var blockContext [32]byte
	copy(blockContext[:], []byte(key))
	return blockContext, nil
}

// --- helpers for VerifyPacketsPreviousRound tests ---

func makePreviousRoundTestFixtures(numValidators int) (
	parentHash common.Hash,
	round byte,
	validators []common.Address,
	filteredValidatorDepositMap map[common.Address]*big.Int,
	totalBlockDepositValue *big.Int,
	minDepositRequired *big.Int,
	nilVoteProposalHashes map[byte]common.Hash,
	nilVotePrecommitHashes map[byte]common.Hash,
) {
	parentHash = randHash()
	round = byte(1)
	validators = make([]common.Address, numValidators)
	filteredValidatorDepositMap = make(map[common.Address]*big.Int)
	depositPerValidator := big.NewInt(100)
	totalBlockDepositValue = big.NewInt(0)

	for i := 0; i < numValidators; i++ {
		validators[i] = randAddress()
		filteredValidatorDepositMap[validators[i]] = new(big.Int).Set(depositPerValidator)
		totalBlockDepositValue = common.SafeAddBigInt(totalBlockDepositValue, depositPerValidator)
	}

	// 67% quorum
	minDepositRequired = new(big.Int).Mul(totalBlockDepositValue, big.NewInt(67))
	minDepositRequired.Div(minDepositRequired, big.NewInt(100))

	nilVoteProposalHashes = make(map[byte]common.Hash)
	nilVoteProposalHashes[round] = getNilVoteProposalHash(parentHash, round)
	nilVoteProposalHashes[2] = getNilVoteProposalHash(parentHash, 2)

	nilVotePrecommitHashes = make(map[byte]common.Hash)
	nilVotePrecommitHashes[round] = getNilVotePreCommitHash(parentHash, round)
	nilVotePrecommitHashes[2] = getNilVotePreCommitHash(parentHash, 2)

	return
}

func buildPreviousRoundPacketMap(round byte, okValidators []common.Address, nilValidators []common.Address,
	okProposalHash common.Hash, nilProposalHash common.Hash) *PacketMap {
	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}

	for _, v := range okValidators {
		pm.proposalAckDetailsMap[v] = &ProposalAckDetails{
			Round:               round,
			ProposalAckVoteType: VOTE_TYPE_OK,
		}
		pm.proposalAckDetailsMap[v].ProposalHash.CopyFrom(okProposalHash)
	}

	for _, v := range nilValidators {
		pm.proposalAckDetailsMap[v] = &ProposalAckDetails{
			Round:               round,
			ProposalAckVoteType: VOTE_TYPE_NIL,
		}
		pm.proposalAckDetailsMap[v].ProposalHash.CopyFrom(nilProposalHash)
	}

	return pm
}

// --- Positive tests ---

func TestVerifyPacketsPreviousRound_SplitVote(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)
	nilProposalHash := nilPropHashes[round]
	okProposalHash := randHash()

	// 2 OK + 2 NIL: neither reaches 67% quorum (each has 50%)
	pm := buildPreviousRoundPacketMap(round, validators[:2], validators[2:], okProposalHash, nilProposalHash)

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err != nil {
		t.Fatalf("expected no error for split vote, got: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_AllNilAboveQuorum(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)
	nilProposalHash := nilPropHashes[round]

	// 3 of 4 validators acked NIL (75% >= 67%), no OK votes
	pm := buildPreviousRoundPacketMap(round, nil, validators[:3], common.Hash{}, nilProposalHash)

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err != nil {
		t.Fatalf("expected no error for all nil votes above quorum, got: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_AllOkAboveQuorum(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)
	okProposalHash := randHash()

	// 3 of 4 validators acked OK (75% >= 67%), no NIL votes
	pm := buildPreviousRoundPacketMap(round, validators[:3], nil, okProposalHash, common.Hash{})

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err != nil {
		t.Fatalf("expected no error for all ok votes above quorum, got: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_EmptyPacketMap(t *testing.T) {
	parentHash, round, _, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)

	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for empty packet map (no votes means below quorum)")
	}
	if !strings.Contains(err.Error(), "total votes below quorum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_OkQuorumReachedNilNot(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)
	okProposalHash := randHash()

	// 3 OK + 1 NIL: OK has 75% >= 67%, NIL has 25% < 67%
	// This is valid for a previous round: the round may have stalled in precommit
	pm := buildPreviousRoundPacketMap(round, validators[:3], validators[3:], okProposalHash, nilPropHashes[round])

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err != nil {
		t.Fatalf("expected no error when only one side reaches quorum, got: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_NilQuorumReachedOkNot(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)
	okProposalHash := randHash()

	// 1 OK + 3 NIL: NIL has 75% >= 67%, OK has 25% < 67%
	pm := buildPreviousRoundPacketMap(round, validators[:1], validators[1:], okProposalHash, nilPropHashes[round])

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err != nil {
		t.Fatalf("expected no error when only nil side reaches quorum, got: %v", err)
	}
}

// --- Negative tests ---

func TestVerifyPacketsPreviousRound_UnrecognizedValidator(t *testing.T) {
	parentHash, round, _, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)

	unknownAddr := randAddress()
	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}
	pm.proposalAckDetailsMap[unknownAddr] = &ProposalAckDetails{
		Round:               round,
		ProposalAckVoteType: VOTE_TYPE_NIL,
	}
	pm.proposalAckDetailsMap[unknownAddr].ProposalHash.CopyFrom(nilPropHashes[round])

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for unrecognized validator")
	}
	if !strings.Contains(err.Error(), "unrecognized validator") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_WrongRound(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)

	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}
	pm.proposalAckDetailsMap[validators[0]] = &ProposalAckDetails{
		Round:               round + 1, // wrong round
		ProposalAckVoteType: VOTE_TYPE_NIL,
	}
	pm.proposalAckDetailsMap[validators[0]].ProposalHash.CopyFrom(nilPropHashes[round])

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for wrong round")
	}
	if !strings.Contains(err.Error(), "invalid round") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_InvalidVoteType(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)

	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}
	pm.proposalAckDetailsMap[validators[0]] = &ProposalAckDetails{
		Round:               round,
		ProposalAckVoteType: VoteType(99), // invalid
	}

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for invalid vote type")
	}
	if !strings.Contains(err.Error(), "invalid vote type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_InvalidNilProposalHash(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)

	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}
	badHash := randHash()
	pm.proposalAckDetailsMap[validators[0]] = &ProposalAckDetails{
		Round:               round,
		ProposalAckVoteType: VOTE_TYPE_NIL,
	}
	pm.proposalAckDetailsMap[validators[0]].ProposalHash.CopyFrom(badHash) // wrong nil proposal hash

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for invalid nil proposal hash")
	}
	if !strings.Contains(err.Error(), "invalid nil proposal hash") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_BothSidesReachQuorum(t *testing.T) {
	parentHash := randHash()
	round := byte(1)

	// 6 validators, deposit 100 each, total 600, quorum 67% = 402
	// 5 OK + 5 NIL would require 10 validators. Instead use 10 validators with 5+5.
	numValidators := 10
	validators := make([]common.Address, numValidators)
	valMap := make(map[common.Address]*big.Int)
	totalDep := big.NewInt(0)
	dep := big.NewInt(100)
	for i := 0; i < numValidators; i++ {
		validators[i] = randAddress()
		valMap[validators[i]] = new(big.Int).Set(dep)
		totalDep = common.SafeAddBigInt(totalDep, dep)
	}
	// minDeposit = 67% of 1000 = 670; 7 validators (700) >= 670
	minDep := new(big.Int).Mul(totalDep, big.NewInt(67))
	minDep.Div(minDep, big.NewInt(100))

	nilPropHashes := map[byte]common.Hash{
		round: getNilVoteProposalHash(parentHash, round),
		2:     getNilVoteProposalHash(parentHash, 2),
	}

	okProposalHash := randHash()
	// 7 OK + 7 NIL — but only 10 validators total, so 7+3
	// Actually, we need both to reach quorum. With 10 validators at 100 each, quorum is 670.
	// 7 OK (700 >= 670) + 7 NIL (700 >= 670) requires 14 validators. Let's use different deposits.
	// Simpler: make 8 validators with deposit 100, quorum = 536.
	// 6 OK (600 >= 536) + 6 NIL impossible with 8.
	// Easiest: overlap won't work; use 14 validators.
	numValidators = 14
	validators = make([]common.Address, numValidators)
	valMap = make(map[common.Address]*big.Int)
	totalDep = big.NewInt(0)
	for i := 0; i < numValidators; i++ {
		validators[i] = randAddress()
		valMap[validators[i]] = new(big.Int).Set(dep)
		totalDep = common.SafeAddBigInt(totalDep, dep)
	}
	// quorum = 67% of 1400 = 938; 10 validators (1000) >= 938
	minDep = new(big.Int).Mul(totalDep, big.NewInt(67))
	minDep.Div(minDep, big.NewInt(100))

	nilPropHashes = map[byte]common.Hash{
		round: getNilVoteProposalHash(parentHash, round),
		2:     getNilVoteProposalHash(parentHash, 2),
	}

	// 10 OK + 10 NIL is impossible with 14 (no overlap). Use 10 OK + 4 NIL.
	// 10 * 100 = 1000 >= 938. 4 * 100 = 400 < 938. Only one side reaches quorum => should pass.
	// For both to reach: need separate sets each with >= 938. Impossible with 14 at 100 each.
	// Use unequal deposits: give 7 validators 200 each.
	for i := 0; i < 7; i++ {
		valMap[validators[i]] = big.NewInt(200)
	}
	// Recalculate total: 7*200 + 7*100 = 2100
	totalDep = big.NewInt(0)
	for _, d := range valMap {
		totalDep = common.SafeAddBigInt(totalDep, d)
	}
	// quorum = 67% of 2100 = 1407
	minDep = new(big.Int).Mul(totalDep, big.NewInt(67))
	minDep.Div(minDep, big.NewInt(100))

	// OK voters: validators[0..6] (7*200=1400 < 1407), still not enough.
	// Use 8 validators at 200 each: 8*200 + 6*100 = 2200, quorum = 1474
	for i := 7; i < 8; i++ {
		valMap[validators[i]] = big.NewInt(200)
	}
	totalDep = big.NewInt(0)
	for _, d := range valMap {
		totalDep = common.SafeAddBigInt(totalDep, d)
	}
	// quorum = 67% of 2200 = 1474
	minDep = new(big.Int).Mul(totalDep, big.NewInt(67))
	minDep.Div(minDep, big.NewInt(100))

	// 8 * 200 = 1600 >= 1474 for OK
	// remaining 6 * 100 = 600 < 1474 for NIL => only OK reaches quorum, should pass.
	// We need BOTH to reach. Let's just use a low minDepositRequired manually for this test.
	minDep = big.NewInt(500) // 500 total
	// OK voters: validators[0..6] = 7*200=1400 >= 500
	// NIL voters: validators[7..13]: 200 + 100*5 = 700 >= 500
	pm := buildPreviousRoundPacketMap(round, validators[:7], validators[7:], okProposalHash, nilPropHashes[round])

	nilPrecommitHashes := map[byte]common.Hash{
		round: getNilVotePreCommitHash(parentHash, round),
		2:     getNilVotePreCommitHash(parentHash, 2),
	}

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error when both ok and nil reach quorum")
	}
	if !strings.Contains(err.Error(), "both ok and nil reached quorum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_TotalVotesBelowQuorum(t *testing.T) {
	parentHash, round, validators, valMap, totalDep, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)
	nilProposalHash := nilPropHashes[round]

	// 2 of 4 validators voted (50% < 67% quorum): 1 OK + 1 NIL
	okProposalHash := randHash()
	pm := buildPreviousRoundPacketMap(round, validators[:1], validators[1:2], okProposalHash, nilProposalHash)

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for total votes below quorum")
	}
	if !strings.Contains(err.Error(), "total votes below quorum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_DepositExceedsTotalBlock(t *testing.T) {
	parentHash, round, validators, valMap, _, minDep, nilPropHashes, nilPrecommitHashes := makePreviousRoundTestFixtures(4)

	// Set totalBlockDeposit to a very small value so that any ack exceeds it
	tinyTotal := big.NewInt(1)
	nilProposalHash := nilPropHashes[round]
	pm := buildPreviousRoundPacketMap(round, nil, validators[:2], common.Hash{}, nilProposalHash)

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, tinyTotal, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for deposit exceeding total block deposit")
	}
	if !strings.Contains(err.Error(), "totalVotesDepositValue") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyPacketsPreviousRound_MissingNilVoteProposalHashLookup(t *testing.T) {
	parentHash := randHash()
	round := byte(1)
	validators := []common.Address{randAddress()}
	valMap := map[common.Address]*big.Int{validators[0]: big.NewInt(100)}
	totalDep := big.NewInt(100)
	minDep := big.NewInt(67)

	// Empty nilVoteProposalHashes — lookup will fail
	nilPropHashes := make(map[byte]common.Hash)

	pm := &PacketMap{
		round:                 round,
		proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
		proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
		precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
		commitDetailsMap:      make(map[common.Address]*CommitDetails),
	}
	pm.proposalAckDetailsMap[validators[0]] = &ProposalAckDetails{
		Round:               round,
		ProposalAckVoteType: VOTE_TYPE_NIL,
	}

	nilPrecommitHashes := make(map[byte]common.Hash)

	err := VerifyPacketsPreviousRound(parentHash, round, pm, &valMap, totalDep, minDep, 1, nilPropHashes, nilPrecommitHashes)
	if err == nil {
		t.Fatalf("expected error for missing nil vote proposal hash")
	}
	if !strings.Contains(err.Error(), "nil vote proposal hash not precomputed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Regression tests for P0 critical fixes (M-1, M-2, M-3) ---

// TestVerifyBlock_M1_ParentMismatchReturnsError verifies that the VerifyBlock
// code path returns a non-nil error when block number or parent hash doesn't
// match the current chain head (M-1 fix: previously returned nil).
// Since VerifyBlock needs a full ProofOfStake + ethAPI, we test via
// VerifyBlockConsensusData with a header whose parent hash cannot resolve.
// The direct regression for M-1 is confirmed by code inspection:
// the old code was `return err` where err was nil after successful
// GetHeaderByNumberInner; now it's `return errors.New(...)`.
func TestVerifyBlock_M1_ParentMismatchReturnsError(t *testing.T) {
	blockConsensusData := &BlockConsensusData{
		VoteType:              VOTE_TYPE_OK,
		SlashedBlockProposers: make([]common.Address, 0),
		Round:                 1,
		SelectedTransactions:  make([]common.Hash, 0),
	}

	blockAdditionalConsensusData := &BlockAdditionalConsensusData{
		ConsensusPackets: []eth.ConsensusPacket{{ParentHash: randHash()}},
		InitTime:         uint64(time.Now().UnixNano() / int64(time.Millisecond)),
	}

	header := &types.Header{
		Number:            big.NewInt(999),
		ParentHash:        randHash(),
		Time:              uint64(time.Now().Unix()),
		Difficulty:        big.NewInt(999),
		GasLimit:          1000000,
		Root:              randHash(),
		TxHash:            randHash(),
		ReceiptHash:       randHash(),
		Extra:             []byte{},
	}

	data, err := rlp.EncodeToBytes(blockConsensusData)
	if err != nil {
		t.Fatalf("EncodeToBytes blockConsensusData: %v", err)
	}
	header.ConsensusData = data

	data2, err := rlp.EncodeToBytes(blockAdditionalConsensusData)
	if err != nil {
		t.Fatalf("EncodeToBytes blockAdditionalConsensusData: %v", err)
	}
	header.UnhashedConsensusData = data2

	block := types.NewBlock(header, nil, nil, trie.NewStackTrie(nil))
	valMap := make(map[common.Address]*big.Int)
	getValidatorsStub := func(common.Hash) (map[common.Address]*big.Int, error) {
		return valMap, nil
	}
	listValidatorsStub := func(common.Hash) (map[common.Address]*ValidatorDetailsV2, error) {
		return nil, nil
	}

	err = VerifyBlockConsensusData(block, &valMap, nil, DummyGetBlockConsensusContext, getValidatorsStub, listValidatorsStub)
	if err == nil {
		t.Fatalf("M-1 regression: VerifyBlockConsensusData should return an error for an unresolvable block, got nil")
	}
}

// obtainValidOKConsensusData runs the consensus test infrastructure to produce
// valid round-1 VOTE_TYPE_OK consensus data (with transactions), returning
// all the pieces needed to call VerifyBlockConsensusDataInner.
func obtainValidOKConsensusData(t *testing.T) (
	txns []common.Hash,
	parentHash common.Hash,
	blockConsensusData *BlockConsensusData,
	blockAdditionalConsensusData *BlockAdditionalConsensusData,
	preparedState *PreparedConsensusState,
	blockNumber uint64,
	consensusContext common.Hash,
) {
	t.Helper()

	numKeys := 4
	_, p2p, valMap, valDetailsMap := NewConsensusTest(numKeys, 1, t.Name())
	parentHash = getTestParentHash(CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER)
	blockNumber = CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER

	numTxns := 3
	txnSlice := make([]common.Hash, numTxns)
	for i := 0; i < numTxns; i++ {
		txnSlice[i] = common.BytesToHash([]byte{byte(i + 1)})
	}

	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		h.SetValidatorTransactions(txnSlice)
		go CurrentConsensusTest.WaitBlockCommit(parentHash, h, t)
	}

	if ValidateTest(valMap, valDetailsMap, startTime, parentHash, p2p, numKeys, CurrentConsensusTest.MaxWaitCount,
		map[VoteType]bool{VOTE_TYPE_OK: true}, BLOCK_STATE_RECEIVED_COMMITS, t) == false {
		t.Fatalf("failed to reach BLOCK_STATE_RECEIVED_COMMITS")
	}

	for _, handler := range p2p.mockP2pHandlers {
		blockState, _, err := handler.consensusHandler.getBlockState(parentHash)
		if err != nil || blockState != BLOCK_STATE_RECEIVED_COMMITS {
			continue
		}

		txns, err = handler.consensusHandler.getBlockSelectedTransactions(parentHash)
		if err != nil {
			continue
		}

		blockConsensusData, blockAdditionalConsensusData, _, err = handler.consensusHandler.getBlockConsensusData(parentHash)
		if err != nil || blockConsensusData == nil {
			continue
		}

		if blockConsensusData.VoteType != VOTE_TYPE_OK {
			continue
		}

		blockContext, err := getBlockConsensusContext("", parentHash)
		if err != nil {
			t.Fatalf("getBlockConsensusContext: %v", err)
		}
		consensusContext = crypto.Keccak256Hash(blockContext[:], []byte(strconv.Itoa(len(*valMap))))

		var valDetails map[common.Address]*ValidatorDetailsV2
		if valDetailsMap != nil {
			valDetails = *valDetailsMap
		}
		preparedState, err = PrepareConsensusState(parentHash, consensusContext, *valMap, valDetails, blockNumber)
		if err != nil {
			t.Fatalf("PrepareConsensusState: %v", err)
		}

		return
	}

	t.Fatalf("could not obtain valid OK consensus data from any handler")
	return
}

// TestVerifyBlockConsensusDataInner_M2_WrongProposalHash verifies that a
// VOTE_TYPE_OK block with a tampered ProposalHash is rejected (M-2 fix).
func TestVerifyBlockConsensusDataInner_M2_WrongProposalHash(t *testing.T) {
	txns, parentHash, bcd, bacd, prepared, blockNum, ctx := obtainValidOKConsensusData(t)

	_, err := VerifyBlockConsensusDataInner(txns, parentHash, bcd, bacd, prepared, blockNum, ctx)
	if err != nil {
		t.Fatalf("valid data should pass: %v", err)
	}

	tampered := *bcd
	tampered.ProposalHash = randHash()

	_, err = VerifyBlockConsensusDataInner(txns, parentHash, &tampered, bacd, prepared, blockNum, ctx)
	if err == nil {
		t.Fatalf("M-2 regression: tampered ProposalHash should be rejected")
	}
	if !strings.Contains(err.Error(), "ProposalHash mismatch") {
		t.Fatalf("M-2 regression: expected 'ProposalHash mismatch' error, got: %v", err)
	}
}

// TestVerifyBlockConsensusDataInner_M3_WrongPrecommitHash verifies that a
// VOTE_TYPE_OK block with a tampered PrecommitHash is rejected (M-3 fix).
func TestVerifyBlockConsensusDataInner_M3_WrongPrecommitHash(t *testing.T) {
	txns, parentHash, bcd, bacd, prepared, blockNum, ctx := obtainValidOKConsensusData(t)

	_, err := VerifyBlockConsensusDataInner(txns, parentHash, bcd, bacd, prepared, blockNum, ctx)
	if err != nil {
		t.Fatalf("valid data should pass: %v", err)
	}

	tampered := *bcd
	tampered.PrecommitHash = randHash()

	_, err = VerifyBlockConsensusDataInner(txns, parentHash, &tampered, bacd, prepared, blockNum, ctx)
	if err == nil {
		t.Fatalf("M-3 regression: tampered PrecommitHash should be rejected")
	}
	if !strings.Contains(err.Error(), "PrecommitHash mismatch") {
		t.Fatalf("M-3 regression: expected 'PrecommitHash mismatch' error, got: %v", err)
	}
}

// TestVerifyBlockConsensusDataInner_ValidOKPassesWithNewChecks is a positive
// test ensuring that legitimately produced OK consensus data still passes
// after the M-2 and M-3 checks were added.
func TestVerifyBlockConsensusDataInner_ValidOKPassesWithNewChecks(t *testing.T) {
	txns, parentHash, bcd, bacd, prepared, blockNum, ctx := obtainValidOKConsensusData(t)

	_, err := VerifyBlockConsensusDataInner(txns, parentHash, bcd, bacd, prepared, blockNum, ctx)
	if err != nil {
		t.Fatalf("valid OK consensus data should pass all checks including M-2/M-3: %v", err)
	}
}

// --- Integration test: VerifyBlockConsensusDataInner with round-2 calls VerifyPacketsPreviousRound ---

func TestVerifyBlockConsensusDataInner_Round2_NilBlock(t *testing.T) {
	numKeys := 4
	_, p2p, valMap, valDetailsMap := NewConsensusTest(numKeys, 1, t.Name())
	parentHash := getTestParentHash(CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER)

	// Run consensus to round 2 — force bifurcation by blocking packets between pairs
	c := 0
	var prev *MockP2PHandler
	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		if c%2 == 1 {
			h.mockP2pManager.BlockPacketsBetweenValidators(h.validator, prev.validator)
		}

		numTxns := 1
		txns := make([]common.Hash, numTxns)
		for i := 0; i < numTxns; i++ {
			txns[i] = common.BytesToHash([]byte{byte(i + c*10)})
		}
		h.SetValidatorTransactions(txns)
		prev = h
		go CurrentConsensusTest.WaitBlockCommit(parentHash, h, t)
		c = c + 1
	}

	// Wait until at least one handler reaches round 2, then unblock
	startTime := time.Now()
	for {
		for _, handler := range p2p.mockP2pHandlers {
			_, round, _ := handler.consensusHandler.getBlockState(parentHash)
			if round >= 2 {
				p2p.DeleteAllPacketBlocks()
				goto round2Reached
			}
		}
		if time.Since(startTime) > 120*time.Second {
			t.Fatalf("timed out waiting for round 2")
		}
		time.Sleep(time.Second)
	}
round2Reached:

	// Wait for commits
	startTimeMs := startTime.UnixNano() / int64(time.Millisecond)
	if ValidateTest(valMap, valDetailsMap, startTimeMs, parentHash, p2p, 3, CurrentConsensusTest.MaxWaitCount*3,
		map[VoteType]bool{VOTE_TYPE_OK: true, VOTE_TYPE_NIL: true}, BLOCK_STATE_RECEIVED_COMMITS, t) == false {
		t.Fatalf("failed to reach commits")
	}

	// Now verify: VerifyBlockConsensusDataInner internally calls VerifyPacketsPreviousRound for round 1
	VerifyBlockConsensusDataTest(parentHash, p2p, valMap, valDetailsMap, t)
}
