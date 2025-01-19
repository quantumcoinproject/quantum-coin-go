package proofofstake

import (
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/log"
	"os"
	"testing"
	"time"
)

func TestPacketHandler_min_basic_time_hash(t *testing.T) {
	if os.Getenv("EXTENDED_TESTS") == "" {
		t.Skip("skipped")
	}
	TEST_CONSENSUS_BLOCK_NUMBER = PROPOSAL_TIME_HASH_START_BLOCK
	numKeys := 4
	_, p2p, valMap, valDetailsMap := Initialize(numKeys)

	parentHash := common.BytesToHash([]byte{1})

	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	proposer, _ := getBlockProposer(parentHash, valMap, 1, valDetailsMap, TEST_CONSENSUS_BLOCK_NUMBER, common.ZERO_HASH)
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
		go WaitBlockCommit(parentHash, h, t)
		c = c + 1
	}

	fmt.Println("c", c)

	if ValidateTest(valMap, valDetailsMap, startTime, parentHash, p2p, 3, DefaultMaxWaitCount*2, map[VoteType]bool{VOTE_TYPE_OK: true}, BLOCK_STATE_RECEIVED_COMMITS, t) == false {
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

	TEST_CONSENSUS_BLOCK_NUMBER = uint64(1)
}

func testPacketHandler_block_proposer_timedout(t *testing.T) {
	if os.Getenv("EXTENDED_TESTS") == "" {
		t.Skip("skipped")
	}
	numKeys := 4
	_, p2p, valMap, valDetailsMap := Initialize(numKeys)

	parentHash := common.BytesToHash([]byte{1})
	c := 1
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	proposer, _ := getBlockProposer(parentHash, valMap, 1, valDetailsMap, TEST_CONSENSUS_BLOCK_NUMBER, common.ZERO_HASH)

	for _, handler := range p2p.mockP2pHandlers {
		h := handler
		if h.validator.IsEqualTo(proposer) {
			continue //proposer timeout simulation
		}
		go WaitBlockCommit(parentHash, h, t)
		c = c + 1
	}

	if ValidateTest(valMap, valDetailsMap, startTime, parentHash, p2p, 3, DefaultMaxWaitCount*5, map[VoteType]bool{VOTE_TYPE_NIL: true}, BLOCK_STATE_RECEIVED_COMMITS, t) == false {
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
	if os.Getenv("EXTENDED_TESTS") == "" {
		t.Skip("skipped")
	}
	for i := 1; i <= TEST_ITERATIONS; i++ {
		fmt.Println("iteration", i)
		testPacketHandler_block_proposer_timedout(t)
	}
}
