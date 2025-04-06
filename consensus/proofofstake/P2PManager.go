package proofofstake

import (
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
)

type P2PHandler interface {
	SendConsensusPacket(peerList []string, packet *eth.ConsensusPacket) error
	BroadcastConsensusData(packet *eth.ConsensusPacket) error
	RequestTransactions(txns []common.Hash) error
	RequestConsensusData(packet *eth.RequestConsensusDataPacket) error
	GetLocalPeerId() string
}
