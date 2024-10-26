package proofofstake

import (
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/eth/protocols/eth"
)

type P2PHandler interface {
	SendConsensusPacket(peerList []string, packet *eth.ConsensusPacket) error
	BroadcastConsensusData(packet *eth.ConsensusPacket) error
	RequestTransactions(txns []common.Hash) error
	RequestConsensusData(packet *eth.RequestConsensusDataPacket) error
	GetLocalPeerId() string
}
