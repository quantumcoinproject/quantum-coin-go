package proofofstake

import (
	"fmt"
	"testing"
	"time"
    "flag"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

var runVeryLongTest = flag.Bool("very-long", false, "runs very long tests")

func TestPacketHandler_min_basic_time_hash(t *testing.T) {
	if !*runVeryLongTest {
		t.SkipNow()
	}
	CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER = defaults.DefaultConfig.PosConfig.PROPOSAL_TIME_HASH_START_BLOCK
	numKeys := 4
	_, p2p, valMap, valDetailsMap := NewConsensusTest(numKeys, 1, t.Name())

	parentHash := common.BytesToHash([]byte{1})

	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	proposer, _ := getBlockProposer(parentHash, valMap, 1, valDetailsMap, CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER, common.ZERO_HASH)
	log.Info("=================proposer", "proposer", proposer)

	skipped := false
	c := 0
	skipList := make(map[common.Address]bool)
	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		if h.validator.IsEqualTo(proposer) == false && skipped == false {
			skipped = true
			skipList[h.validator] = true
			continue
		}
		go CurrentConsensusTest.WaitBlockCommit(parentHash, h, t)
		c = c + 1
	}

	fmt.Println("c", c)

	if ValidateTest(valMap, valDetailsMap, startTime, parentHash, p2p, 3, CurrentConsensusTest.MaxWaitCount*2, map[VoteType]bool{VOTE_TYPE_OK: true}, BLOCK_STATE_RECEIVED_COMMITS, t) == false {
		t.Fatalf("failed")
	}

	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		txnList, err := h.consensusHandler.getBlockSelectedTransactions(parentHash)
		if skipList[h.validator] {
			if err == nil {
				t.Fatalf("failed")
			}
		} else {
			if err != nil || txnList == nil || len(txnList) != 0 {
				t.Fatalf("failed")
			}
		}
	}
}

func testPacketHandler_block_proposer_timedout(t *testing.T) {
	if !*runVeryLongTest {
		t.SkipNow()
	}
	numKeys := 4
	_, p2p, valMap, valDetailsMap := NewConsensusTest(numKeys, 1, t.Name())

	parentHash := common.BytesToHash([]byte{1})
	c := 1
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	proposer, _ := getBlockProposer(parentHash, valMap, 1, valDetailsMap, CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER, common.ZERO_HASH)

	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		if h.validator.IsEqualTo(proposer) {
			continue //proposer timeout simulation
		}
		go CurrentConsensusTest.WaitBlockCommit(parentHash, h, t)
		c = c + 1
	}

	if ValidateTest(valMap, valDetailsMap, startTime, parentHash, p2p, 3, CurrentConsensusTest.MaxWaitCount*5, map[VoteType]bool{VOTE_TYPE_NIL: true}, BLOCK_STATE_RECEIVED_COMMITS, t) == false {
		t.Fatalf("failed")
	}

	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		txnList, err := h.consensusHandler.getBlockSelectedTransactions(parentHash)
		if h.validator.IsEqualTo(proposer) {
			if err == nil {
				t.Fatalf("failed")
			}
		} else {
			if err != nil || txnList != nil {
				t.Fatalf("failed")
			}
		}
	}
}

func TestPacketHandler_block_proposer_timedout(t *testing.T) {
	if !*runVeryLongTest {
		t.SkipNow()
	}
	for i := 1; i <= TEST_ITERATIONS; i++ {
		fmt.Println("iteration", i)
		testPacketHandler_block_proposer_timedout(t)
	}
}

func TestPacketHandler_offline_validator_block_full_sign_breakglass(t *testing.T) {
	fmt.Println("TestPacketHandler_basic_various_blocks starting")
	var blockNumbers = []uint64{10000001}
	if err := defaults.SetCryptoBreakGlassBlock(0); err != nil {
		t.Fatalf("failed reset breakglass: %v", err)
	}
	err := defaults.SetCryptoBreakGlassBlock(10000001)
	if err != nil {
		t.Fatalf("failed")
	}

	for _, b := range blockNumbers {
		if shouldSignFull(b) == false {
			t.Fatalf("failed")
		}
		fmt.Println("CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER", b)
		for i := 1; i <= TEST_ITERATIONS; i++ {
			fmt.Println("iteration", i)
			testPacketHandler_basic(4, b, t)
		}
	}
	// Allow async BroadcastConsensusData goroutines to finish before clearing breakglass (same as TestPacketHandler_breakglass).
	time.Sleep(10 * time.Second)
	err = defaults.SetCryptoBreakGlassBlock(0)
	if err != nil {
		t.Fatalf("failed")
	}
	defaults.SetCryptoSigningMode(byte(crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID))
	// Async BroadcastConsensusData may still run; mismatch CurrentConsensusTest so MockP2PHandler drops them before getSigner.
	if CurrentConsensusTest != nil {
		CurrentConsensusTest.TEST_CONSENSUS_BLOCK_NUMBER = 0
	}

	fmt.Println("TestPacketHandler_offline_validator_block_full_sign_breakglass done")
}
