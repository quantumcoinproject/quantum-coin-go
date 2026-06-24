package proofofstake

import (
	"bytes"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/accounts"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"github.com/quantumcoinproject/quantum-coin-go/trie"
)

// setMalleabilityGate overrides the ConsensusMalleabilityV1 activation height for a
// test and returns a restore function. Tests using it must not run in parallel since
// it mutates global config.
func setMalleabilityGate(t *testing.T, height uint64) {
	t.Helper()
	orig := defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = height
	t.Cleanup(func() { defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = orig })
}

// TestVerifyErrorTransactions checks per-tx field/signature validation of the
// transactions carried in a header's extra-data. This is semantic validation the
// RLP decoder cannot perform, so it is genuinely additive.
func TestVerifyErrorTransactions(t *testing.T) {
	c := &ProofOfStake{signer: types.NewLondonSignerDefaultChain()}

	t.Run("empty_ok", func(t *testing.T) {
		if err := c.verifyErrorTransactions(nil); err != nil {
			t.Fatalf("expected nil for empty list, got %v", err)
		}
	})

	t.Run("valid_signed_ok", func(t *testing.T) {
		key, err := cryptobase.SigAlg.GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		signer := types.NewLondonSignerDefaultChain()
		tx, err := types.SignTx(types.NewTransaction(0, common.Address{}, big.NewInt(100), 21000, big.NewInt(1), nil), signer, key)
		if err != nil {
			t.Fatalf("SignTx: %v", err)
		}
		if err := c.verifyErrorTransactions(types.Transactions{tx}); err != nil {
			t.Fatalf("expected valid signed tx to verify, got %v", err)
		}
	})

	t.Run("unsigned_rejected", func(t *testing.T) {
		tx := types.NewTransaction(0, common.Address{}, big.NewInt(100), 21000, big.NewInt(1), nil)
		if err := c.verifyErrorTransactions(types.Transactions{tx}); err == nil {
			t.Fatalf("expected unsigned tx to be rejected")
		}
	})
}

// TestVerifyExtraData_v3_strictDecoder confirms canonical v3 extra-data verifies and
// that trailing/non-canonical bytes are rejected by the strict RLP decoder (no
// separate re-encode check is needed for encoding-level malleability).
func TestVerifyExtraData_v3_strictDecoder(t *testing.T) {
	blockNumber := defaults.DefaultConfig.PosConfig.ExtraDataV3StartBlock

	var errorTransactions types.Transactions
	encoded, err := EncodeBlockExtraData(errorTransactions, make([]byte, 0), blockNumber)
	if err != nil {
		t.Fatalf("EncodeBlockExtraData: %v", err)
	}

	if _, err := VerifyExtraData(blockNumber, encoded); err != nil {
		t.Fatalf("expected canonical v3 extra to verify, got %v", err)
	}

	// Trailing byte: rejected by rlp.DecodeBytes ("input contains more than one value").
	nonCanonical := append(append([]byte{}, encoded...), 0x00)
	if _, err := VerifyExtraData(blockNumber, nonCanonical); err == nil {
		t.Fatalf("expected non-canonical v3 extra (trailing byte) to be rejected")
	}
}

// TestVerifyExtraData_deepcheck_strictDecoder confirms canonical pre-v3 (DeepCheck)
// extra-data verifies and that trailing bytes after the canonical suffix are
// rejected by the strict RLP decoder.
func TestVerifyExtraData_deepcheck_strictDecoder(t *testing.T) {
	blockNumber := defaults.DefaultConfig.DeepCheckStartBlock

	var errorTransactions types.Transactions
	encoded, err := EncodeBlockExtraData(errorTransactions, DefaultExtraData, blockNumber)
	if err != nil {
		t.Fatalf("EncodeBlockExtraData: %v", err)
	}

	if _, err := VerifyExtraData(blockNumber, encoded); err != nil {
		t.Fatalf("expected canonical DeepCheck extra to verify, got %v", err)
	}

	nonCanonical := append(append([]byte{}, encoded...), 0x00)
	if _, err := VerifyExtraData(blockNumber, nonCanonical); err == nil {
		t.Fatalf("expected non-canonical DeepCheck extra (trailing byte) to be rejected")
	}
}

// TestRLPDecodeRejectsNonCanonical documents the property the implementation relies
// on instead of an explicit re-encode check: the RLP decoder rejects non-canonical
// integer encodings, so a successful decode pins the bytes to their canonical form.
func TestRLPDecodeRejectsNonCanonical(t *testing.T) {
	// Canonical encoding of uint64(5) is 0x05; 0x8105 is a non-canonical size form.
	var v uint64
	if err := rlp.DecodeBytes([]byte{0x05}, &v); err != nil || v != 5 {
		t.Fatalf("expected canonical 0x05 to decode to 5, got v=%d err=%v", v, err)
	}
	if err := rlp.DecodeBytes([]byte{0x81, 0x05}, &v); err == nil {
		t.Fatalf("expected non-canonical size 0x8105 to be rejected by the decoder")
	}
	// Leading-zero integers are also rejected.
	if err := rlp.DecodeBytes([]byte{0x00}, &v); err == nil {
		t.Fatalf("expected leading-zero integer 0x00 to be rejected by the decoder")
	}
}

// TestVerifyExtraData_roundTripStable asserts decode -> re-encode is byte-stable
// for the consensus extra-data structures (sanity check on encoder determinism).
func TestVerifyExtraData_roundTripStable(t *testing.T) {
	blockNumber := defaults.DefaultConfig.PosConfig.ExtraDataV3StartBlock

	to := common.Address{}
	errorTransactions := types.Transactions{
		types.NewTransaction(0, to, big.NewInt(100), 21000, big.NewInt(1), nil),
		types.NewTransaction(1, to, big.NewInt(200), 21000, big.NewInt(2), nil),
	}

	encoded, err := EncodeBlockExtraData(errorTransactions, make([]byte, 0), blockNumber)
	if err != nil {
		t.Fatalf("EncodeBlockExtraData: %v", err)
	}

	decoded, _, err := DecodeBlockExtraData(encoded, blockNumber)
	if err != nil {
		t.Fatalf("DecodeBlockExtraData: %v", err)
	}

	reEncoded, err := rlp.EncodeToBytes(decoded)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}

	if bytes.Equal(reEncoded, encoded) == false {
		t.Fatalf("decode -> re-encode not byte-stable")
	}
}

// TestIsConsensusMalleabilityV1 verifies the fork-gate helper boundary behaviour.
func TestIsConsensusMalleabilityV1(t *testing.T) {
	setMalleabilityGate(t, 1000)
	if defaults.IsConsensusMalleabilityV1(999) {
		t.Fatalf("expected inactive below activation height")
	}
	if defaults.IsConsensusMalleabilityV1(1000) == false {
		t.Fatalf("expected active at activation height")
	}
	if defaults.IsConsensusMalleabilityV1(1001) == false {
		t.Fatalf("expected active above activation height")
	}
}

// decodeProposalBlockTime returns the BlockTime carried by the proposal packet for the
// given round (used to assert gap #2 round-trips it once gated).
func decodeProposalBlockTime(packets []eth.ConsensusPacket, round byte) (uint64, bool) {
	for i := range packets {
		data := packets[i].ConsensusData
		if len(data) < 2 {
			continue
		}
		var si int
		if data[0] >= MinConsensusNetworkProtocolVersion {
			si = 2
		} else {
			si = 1
		}
		if ConsensusPacketType(data[si-1]) != CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK {
			continue
		}
		var pd ProposalDetails
		if rlp.DecodeBytes(data[si:], &pd) != nil {
			continue
		}
		if pd.Round == round {
			return pd.BlockTime, true
		}
	}
	return 0, false
}

// TestConsensusMalleabilityV1_DecidingRound covers gap #1 (deciding-round proposal
// packet cross-check) and gap #2 (BlockTime preservation in ParseConsensusPackets),
// including the below-activation regression that historical behaviour is unchanged.
func TestConsensusMalleabilityV1_DecidingRound(t *testing.T) {
	txns, parentHash, bcd, bacd, prepared, blockNum, ctx := obtainValidOKConsensusData(t)
	setMalleabilityGate(t, blockNum) // active for this block

	// Gap #1 positive: valid OK data still passes with the cross-check active.
	if _, err := VerifyBlockConsensusDataInner(txns, parentHash, bcd, bacd, prepared, blockNum, ctx); err != nil {
		t.Fatalf("gap#1: valid OK data should pass with cross-check active: %v", err)
	}

	// Gap #1 negative: make the header internally consistent for a different txn set,
	// so it disagrees only with the proposer's (unchanged) signed proposal packet.
	tampered := *bcd
	newTxns := append(append([]common.Hash{}, bcd.SelectedTransactions...), randHash())
	tampered.SelectedTransactions = newTxns
	if blockNum >= defaults.DefaultConfig.PosConfig.PROPOSAL_TIME_HASH_START_BLOCK {
		tampered.ProposalHash = GetCombinedTxnHashWithTime(parentHash, tampered.Round, newTxns, tampered.BlockTime)
	} else {
		tampered.ProposalHash = GetCombinedTxnHash(parentHash, tampered.Round, newTxns)
	}
	tampered.PrecommitHash = getOkVotePreCommitHash(parentHash, tampered.ProposalHash, tampered.Round)

	_, err := VerifyBlockConsensusDataInner(txns, parentHash, &tampered, bacd, prepared, blockNum, ctx)
	if err == nil || strings.Contains(err.Error(), "proposal packet disagrees with BlockConsensusData") == false {
		t.Fatalf("gap#1: expected 'proposal packet disagrees with BlockConsensusData', got %v", err)
	}

	// Gap #2: the proposal BlockTime is preserved by ParseConsensusPackets when gated,
	// and zeroed (historical behaviour) when not.
	origBT, found := decodeProposalBlockTime(bacd.ConsensusPackets, bcd.Round)
	if found == false {
		t.Fatalf("gap#2: no proposal packet found for round %d", bcd.Round)
	}
	valDetailsMap := prepared.ValidatorDetailsMap

	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = blockNum // active
	prm, err := ParseConsensusPackets(parentHash, &bacd.ConsensusPackets, prepared.FilteredValidatorsDepositMap, blockNum, &valDetailsMap, ctx, prepared.RoundProposers)
	if err != nil {
		t.Fatalf("gap#2 active parse: %v", err)
	}
	pd := prm[bcd.Round].proposalDetailsMap[bcd.BlockProposer]
	if pd == nil {
		t.Fatalf("gap#2: missing proposer proposal in parsed map (active)")
	}
	if pd.BlockTime != origBT {
		t.Fatalf("gap#2: active parse BlockTime=%d, want %d", pd.BlockTime, origBT)
	}

	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = ^uint64(0) // inactive
	prm2, err := ParseConsensusPackets(parentHash, &bacd.ConsensusPackets, prepared.FilteredValidatorsDepositMap, blockNum, &valDetailsMap, ctx, prepared.RoundProposers)
	if err != nil {
		t.Fatalf("gap#2 inactive parse: %v", err)
	}
	pd2 := prm2[bcd.Round].proposalDetailsMap[bcd.BlockProposer]
	if pd2 == nil {
		t.Fatalf("gap#2: missing proposer proposal in parsed map (inactive)")
	}
	if pd2.BlockTime != 0 {
		t.Fatalf("gap#2: inactive parse BlockTime=%d, want 0 (historical behaviour)", pd2.BlockTime)
	}
}

// TestParseConsensusPacket_RejectsUnknownVersionGated covers gap #4: at/after the
// activation height a consensus packet whose version byte is >= the minimum but not
// the current version is rejected; below the height the old behaviour is preserved.
func TestParseConsensusPacket_RejectsUnknownVersionGated(t *testing.T) {
	setMalleabilityGate(t, 0)

	parentHash := randHash()
	valMap := map[common.Address]*big.Int{}
	valDetails := map[common.Address]*ValidatorDetailsV2{}

	mkPkt := func(version byte) *eth.ConsensusPacket {
		return &eth.ConsensusPacket{
			ParentHash:    parentHash,
			Signature:     make([]byte, 100),
			ConsensusData: []byte{version, byte(CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK)},
		}
	}
	run := func(pkt *eth.ConsensusPacket, blockNumber uint64) error {
		var wg sync.WaitGroup
		ch := make(chan *PacketParseResult, 1)
		wg.Add(1)
		go ParseConsensusPacket(&wg, parentHash, pkt, valMap, blockNumber, &valDetails, common.Hash{}, map[byte]common.Address{}, ch)
		res := <-ch
		wg.Wait()
		return res.err
	}

	// Active: a future/unknown version (6) is rejected for the version reason.
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = 0
	if err := run(mkPkt(byte(6)), 100); err == nil || err.Error() != "unsupported consensus protocol version" {
		t.Fatalf("gap#4: expected 'unsupported consensus protocol version', got %v", err)
	}

	// Inactive: version 6 is NOT rejected for the version reason (it instead fails later
	// on signature verification), i.e. historical behaviour is unchanged.
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = ^uint64(0)
	if err := run(mkPkt(byte(6)), 100); err != nil && err.Error() == "unsupported consensus protocol version" {
		t.Fatalf("gap#4 regression: version should not be rejected below activation height")
	}
}

// TestConsensusMalleabilityV1_InitTimeBound covers gap #5: a non-zero InitTime that is
// implausibly far after the block timestamp is rejected once gated, and is not checked
// below the activation height.
func TestConsensusMalleabilityV1_InitTimeBound(t *testing.T) {
	setMalleabilityGate(t, 0)

	buildBlock := func(initTime uint64) *types.Block {
		bcd := &BlockConsensusData{
			VoteType:              VOTE_TYPE_OK,
			SlashedBlockProposers: make([]common.Address, 0),
			Round:                 1,
			SelectedTransactions:  make([]common.Hash, 0),
			BlockTime:             0,
		}
		bacd := &BlockAdditionalConsensusData{
			ConsensusPackets: []eth.ConsensusPacket{{ParentHash: randHash()}},
			InitTime:         initTime,
		}
		header := &types.Header{
			Number:      big.NewInt(5),
			ParentHash:  randHash(),
			Time:        1000,
			Difficulty:  big.NewInt(5),
			GasLimit:    1000000,
			Root:        randHash(),
			TxHash:      randHash(),
			ReceiptHash: randHash(),
			Extra:       []byte{},
		}
		d1, _ := rlp.EncodeToBytes(bcd)
		header.ConsensusData = d1
		d2, _ := rlp.EncodeToBytes(bacd)
		header.UnhashedConsensusData = d2
		return types.NewBlock(header, nil, nil, trie.NewStackTrie(nil))
	}

	valMap := map[common.Address]*big.Int{}
	getV := func(common.Hash) (map[common.Address]*big.Int, error) { return valMap, nil }
	listV := func(common.Hash) (map[common.Address]*ValidatorDetailsV2, error) { return nil, nil }

	// header.Time = 1000s; allowance is 1 day. InitTime is in ms, so this is just over the bound.
	hugeInit := (uint64(1000)+uint64(86400))*1000 + 1

	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = 0 // active
	err := VerifyBlockConsensusData(buildBlock(hugeInit), &valMap, nil, DummyGetBlockConsensusContext, getV, listV)
	if err == nil || strings.Contains(err.Error(), "InitTime after block time") == false {
		t.Fatalf("gap#5: expected 'InitTime after block time', got %v", err)
	}

	// A plausible InitTime must not be rejected on InitTime grounds (it may fail later).
	err = VerifyBlockConsensusData(buildBlock(uint64(1000)*1000), &valMap, nil, DummyGetBlockConsensusContext, getV, listV)
	if err != nil && strings.Contains(err.Error(), "InitTime after block time") {
		t.Fatalf("gap#5: plausible InitTime wrongly rejected: %v", err)
	}

	// Inactive: the huge InitTime is not checked below the activation height.
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = ^uint64(0)
	err = VerifyBlockConsensusData(buildBlock(hugeInit), &valMap, nil, DummyGetBlockConsensusContext, getV, listV)
	if err != nil && strings.Contains(err.Error(), "InitTime after block time") {
		t.Fatalf("gap#5 regression: InitTime should not be checked below activation height: %v", err)
	}
}

// TestConsensusMalleabilityV1_BlockTimeMonotonic covers gap #7: header.Time must
// strictly exceed parent.Time once gated; below the activation height it is not checked.
func TestConsensusMalleabilityV1_BlockTimeMonotonic(t *testing.T) {
	setMalleabilityGate(t, 0)

	c := &ProofOfStake{}
	parent := &types.Header{
		Number:      big.NewInt(10),
		Time:        1000,
		Difficulty:  big.NewInt(10),
		GasLimit:    1000000,
		Root:        randHash(),
		TxHash:      randHash(),
		ReceiptHash: randHash(),
		Extra:       []byte{},
	}
	mkHeader := func(headerTime uint64) *types.Header {
		return &types.Header{
			Number:      big.NewInt(11),
			ParentHash:  parent.Hash(),
			Time:        headerTime,
			Difficulty:  big.NewInt(11),
			GasLimit:    1000000,
			GasUsed:     0,
			Root:        randHash(),
			TxHash:      randHash(),
			ReceiptHash: randHash(),
			Extra:       []byte{},
		}
	}

	// Active, equal time: rejected as non-monotonic.
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = 0
	if err := c.verifyCascadingFields(nil, mkHeader(1000), []*types.Header{parent}); err != errNonMonotonicBlockTime {
		t.Fatalf("gap#7: expected errNonMonotonicBlockTime for equal time, got %v", err)
	}
	// Active, earlier time: rejected as non-monotonic.
	if err := c.verifyCascadingFields(nil, mkHeader(999), []*types.Header{parent}); err != errNonMonotonicBlockTime {
		t.Fatalf("gap#7: expected errNonMonotonicBlockTime for earlier time, got %v", err)
	}
	// Active, greater time: passes the monotonic check (fails later in verifySeal, but
	// not with the monotonic error).
	if err := c.verifyCascadingFields(nil, mkHeader(1001), []*types.Header{parent}); err == errNonMonotonicBlockTime {
		t.Fatalf("gap#7: monotonic check should pass for greater time")
	}

	// Inactive: equal time is not rejected on monotonicity grounds.
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = ^uint64(0)
	if err := c.verifyCascadingFields(nil, mkHeader(1000), []*types.Header{parent}); err == errNonMonotonicBlockTime {
		t.Fatalf("gap#7 regression: monotonicity should not be checked below activation height")
	}
}

// TestConsensusMalleabilityV1_AckEquivocation covers gap #3: the live ack handler
// rejects a conflicting ack from a validator in the same round (and ignores an
// identical re-broadcast) once gated, while below the activation height it overwrites
// as before.
func TestConsensusMalleabilityV1_AckEquivocation(t *testing.T) {
	setMalleabilityGate(t, 0)

	parentHash := randHash()
	self := randAddress()
	validator := randAddress()
	hashA := randHash()
	hashB := randHash()

	mkHandler := func() *ConsensusHandler {
		existing := &ProposalAckDetails{Round: 1, ProposalAckVoteType: VOTE_TYPE_OK}
		existing.ProposalHash.CopyFrom(hashA)
		brd := &BlockRoundDetails{
			Round:                 1,
			state:                 BLOCK_STATE_WAITING_FOR_PROPOSAL_ACKS,
			validatorProposalAcks: map[common.Address]*ProposalAckDetails{validator: existing},
			proposalAckPackets:    map[common.Address]*eth.ConsensusPacket{},
		}
		bsd := &BlockStateDetails{
			parentHash:                   parentHash,
			currentRound:                 1,
			blockNumber:                  1000,
			blockRoundMap:                map[byte]*BlockRoundDetails{1: brd},
			filteredValidatorsDepositMap: map[common.Address]*big.Int{self: big.NewInt(100), validator: big.NewInt(100)},
		}
		return &ConsensusHandler{
			account:              accounts.Account{Address: self},
			blockStateDetailsMap: map[common.Hash]*BlockStateDetails{parentHash: bsd},
		}
	}
	mkAckPacket := func(vt VoteType, ph common.Hash) *eth.ConsensusPacket {
		ad := &ProposalAckDetails{Round: 1, ProposalAckVoteType: vt}
		ad.ProposalHash.CopyFrom(ph)
		enc, err := rlp.EncodeToBytes(ad)
		if err != nil {
			t.Fatalf("encode ack: %v", err)
		}
		data := append([]byte{ConsensusNetworkProtocolVersion, byte(CONSENSUS_PACKET_TYPE_ACK_BLOCK_PROPOSAL)}, enc...)
		return &eth.ConsensusPacket{ParentHash: parentHash, ConsensusData: data, Signature: []byte{0x01}}
	}

	// Active: conflicting ack (different proposal hash) is rejected as equivocation.
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = 0
	cph := mkHandler()
	if err := cph.handleAckBlockProposalPacket(validator, mkAckPacket(VOTE_TYPE_OK, hashB)); err == nil || strings.Contains(err.Error(), "conflicting ack") == false {
		t.Fatalf("gap#3: expected conflicting-ack equivocation error, got %v", err)
	}

	// Active: an identical re-broadcast is ignored (no error, not overwritten).
	cph = mkHandler()
	if err := cph.handleAckBlockProposalPacket(validator, mkAckPacket(VOTE_TYPE_OK, hashA)); err != nil {
		t.Fatalf("gap#3: identical ack re-broadcast should be ignored, got %v", err)
	}

	// Inactive: a conflicting ack overwrites without error (historical behaviour).
	defaults.DefaultConfig.PosConfig.ConsensusMalleabilityV1StartBlock = ^uint64(0)
	cph = mkHandler()
	if err := cph.handleAckBlockProposalPacket(validator, mkAckPacket(VOTE_TYPE_OK, hashB)); err != nil {
		t.Fatalf("gap#3 regression: below activation height a duplicate ack should overwrite without error, got %v", err)
	}
}
