package proofofstake

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/accounts"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hybrideds"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// shortConsensusDataCases returns ConsensusData values that are too short to carry a
// packet type byte. The 1-byte >= MinConsensusNetworkProtocolVersion case is the
// dangerous one: it selects the 2-byte header form (startIndex=2) and then indexes
// ConsensusData[1], which does not exist.
func shortConsensusDataCases() map[string][]byte {
	return map[string][]byte{
		"one_byte_version_current": {ConsensusNetworkProtocolVersion},
		"one_byte_version_above":   {MinConsensusNetworkProtocolVersion + 1},
		"one_byte_version_max":     {0xff},
	}
}

// A single legacy-form byte (< MinConsensusNetworkProtocolVersion) selects
// startIndex=1 and therefore reads index 0, which exists. It is well-formed framing
// carrying a valid packet type, so it must NOT be rejected as too short -- included
// here to pin that the bounds fix did not over-reject the legacy wire format.
func TestLegacyOneByteConsensusDataStillParses(t *testing.T) {
	pkt := &eth.ConsensusPacket{
		ParentHash:    common.Hash{},
		ConsensusData: []byte{byte(CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK)},
		Signature:     longSignature(),
	}
	if got := getPacketType(pkt); got != CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK {
		t.Errorf("getPacketType = %v, want CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK", got)
	}
	cph := &ConsensusHandler{}
	if !cph.ShouldRebroadCast(pkt, "peer") {
		t.Errorf("ShouldRebroadCast rejected a well-formed legacy packet")
	}
}

func longSignature() []byte {
	return make([]byte, hybrideds.CRYPTO_SIGNATURE_BYTES+2048)
}

// A malformed packet from any peer must be rejected, never panic. Before the fix a
// 1-byte ConsensusData of 0x05 panicked with "index out of range [1] with length 1"
// at four sites in this file, reachable pre-signature-verification.
func TestShortConsensusDataDoesNotPanic_ConsensusHandler(t *testing.T) {
	for name, data := range shortConsensusDataCases() {
		data := data
		t.Run(name, func(t *testing.T) {
			pkt := &eth.ConsensusPacket{
				ParentHash:    common.Hash{},
				ConsensusData: data,
				Signature:     longSignature(),
			}

			cph := &ConsensusHandler{}
			if cph.ShouldRebroadCast(pkt, "peer") {
				t.Errorf("ShouldRebroadCast returned true for malformed packet")
			}

			if got := getPacketType(pkt); got != CONSENSUS_PACKET_TYPE_INVALID {
				t.Errorf("getPacketType = %v, want CONSENSUS_PACKET_TYPE_INVALID", got)
			}

			if err := cph.processPacket(pkt, 1); err == nil {
				t.Errorf("processPacket returned nil error for malformed packet")
			}

			if _, _, _, _, err := parsePacketInternal(pkt, 1, false); err == nil {
				t.Errorf("parsePacketInternal returned nil error for malformed packet")
			}
		})
	}
}

// The same malformed packet arriving through the real remote entry point must be
// rejected rather than crashing the process.
func TestShortConsensusDataDoesNotPanic_HandleConsensusPacket(t *testing.T) {
	for name, data := range shortConsensusDataCases() {
		data := data
		t.Run(name, func(t *testing.T) {
			cph := &ConsensusHandler{
				initialized: true,
				initTime:    time.Now().Add(-24 * time.Hour),
			}
			pkt := &eth.ConsensusPacket{
				ParentHash:    common.Hash{},
				ConsensusData: data,
				Signature:     longSignature(),
			}
			// signFn nil short-circuits before processPacket, so set it to reach the
			// parsing path that used to panic.
			cph.signFn = func(account accounts.Account, mimeType string, message []byte, sigAlg byte) ([]byte, error) {
				return nil, nil
			}
			_ = cph.HandleConsensusPacket(pkt, "attacker-peer")
		})
	}
}

// Packets carried inside a block's extra data are attacker-controlled too, and
// ParseConsensusPacket runs in a bare goroutine with no recover(), so a panic there
// kills the node during block verification.
func TestShortConsensusDataDoesNotPanic_ParseConsensusPacket(t *testing.T) {
	for name, data := range shortConsensusDataCases() {
		data := data
		t.Run(name, func(t *testing.T) {
			parentHash := common.Hash{}
			pkt := eth.ConsensusPacket{
				ParentHash:    parentHash,
				ConsensusData: data,
				Signature:     longSignature(),
			}
			var wg sync.WaitGroup
			wg.Add(1)
			ch := make(chan *PacketParseResult, 1)
			depMap := make(map[common.Address]*big.Int)
			valDetails := make(map[common.Address]*ValidatorDetailsV2)
			proposers := make(map[byte]common.Address)

			ParseConsensusPacket(&wg, parentHash, &pkt, depMap, 1, &valDetails, common.Hash{}, proposers, ch)

			select {
			case res := <-ch:
				if res.err == nil {
					t.Errorf("ParseConsensusPacket accepted malformed packet")
				}
			default:
				t.Errorf("ParseConsensusPacket produced no result")
			}
		})
	}
}

// An empty (non-nil) packet list must not hang. RLP decodes an empty list to an
// empty non-nil slice, so the callers' nil checks do not catch it; before the fix
// ParseConsensusPackets blocked forever on a channel nobody closed, halting block
// import while holding the chain mutex.
func TestParseConsensusPacketsEmptyListDoesNotDeadlock(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		empty := []eth.ConsensusPacket{}
		depMap := make(map[common.Address]*big.Int)
		valDetails := make(map[common.Address]*ValidatorDetailsV2)
		proposers := make(map[byte]common.Address)
		_, err := ParseConsensusPackets(common.Hash{}, &empty, depMap, 1, &valDetails, common.Hash{}, proposers)
		if err == nil {
			t.Errorf("expected an error for an empty packet list")
		}
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("ParseConsensusPackets deadlocked on an empty packet list")
	}
}

// Guards the assumption the fix relies on: an empty RLP list decodes to an empty
// non-nil slice, which is why the callers' `== nil` checks were insufficient.
func TestEmptyConsensusPacketsRLPDecodesNonNil(t *testing.T) {
	encoded, err := rlp.EncodeToBytes(&BlockAdditionalConsensusData{
		ConsensusPackets: []eth.ConsensusPacket{},
		InitTime:         12345,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded BlockAdditionalConsensusData
	if err := rlp.DecodeBytes(encoded, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.ConsensusPackets == nil {
		t.Fatal("expected empty non-nil slice; a nil check would then have sufficed")
	}
	if len(decoded.ConsensusPackets) != 0 {
		t.Fatalf("expected length 0, got %d", len(decoded.ConsensusPackets))
	}
}
