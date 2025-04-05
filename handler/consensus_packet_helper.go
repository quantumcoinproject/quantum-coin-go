package handler

import (
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core"
	"github.com/QuantumCoinProject/qc/eth/protocols/eth"
	"github.com/QuantumCoinProject/qc/log"
	"sync"
	"time"
)

const CleanupIntervalSeconds = 1800 * time.Second

type PacketInfo struct {
	packet       *eth.ConsensusPacket
	fromPeerId   string
	sentPeerList map[string]bool
	timeAdded    int64
}

type ConsensusPacketHelper struct {
	chain           *core.BlockChain
	packetMap       map[common.Hash]*PacketInfo
	mapLock         sync.Mutex
	lastCleanupTime int64
}

func NewConsensusPacketHelper(chain *core.BlockChain) *ConsensusPacketHelper {
	return &ConsensusPacketHelper{
		chain:           chain,
		packetMap:       make(map[common.Hash]*PacketInfo),
		lastCleanupTime: time.Now().UnixNano(),
	}
}

func (c *ConsensusPacketHelper) cleanup() {
	currentTime := time.Now().UnixNano()
	if currentTime-c.lastCleanupTime < CleanupIntervalSeconds.Nanoseconds() {
		log.Trace("ConsensusPacketHelper cleanup skipped")
		return
	}

	c.lastCleanupTime = currentTime
	currentParentHash := c.chain.CurrentHeader().ParentHash
	for hash, pktInfo := range c.packetMap {
		if currentTime-pktInfo.timeAdded > CleanupIntervalSeconds.Nanoseconds() {
			if currentParentHash.IsEqualTo(pktInfo.packet.ParentHash) == false {
				log.Trace("ConsensusPacketHelper cleanup packet", "hash", hash.Hex(), "timeAdded", pktInfo.timeAdded, "ParentHash", pktInfo.packet.ParentHash)
				delete(c.packetMap, hash)
			} else {
				log.Trace("ConsensusPacketHelper skipping cleanup packet parent hash match", "hash", hash.Hex(),
					"timeAdded", pktInfo.timeAdded, "ParentHash", pktInfo.packet.ParentHash)
			}
		}
	}
}

func (c *ConsensusPacketHelper) GetPeerListToBroadcast(fromPeerId string, pkt *eth.ConsensusPacket, connectedPeerList []string) ([]string, error) {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	c.cleanup()

	packetHash := pkt.Hash()
	pktInfo, ok := c.packetMap[packetHash]
	if ok == false {
		pktInfo = &PacketInfo{
			fromPeerId:   fromPeerId,
			sentPeerList: make(map[string]bool),
			timeAdded:    time.Now().UnixNano(),
		}
		pktInfo.packet = &eth.ConsensusPacket{}
		pktInfo.packet.CopyFrom(pkt)
		log.Trace("GetPeerListToBroadcast new packet", "ParentHash", pkt.ParentHash, "packetHash", packetHash)
	} else {
		log.Trace("GetPeerListToBroadcast existing packet", "ParentHash", pkt.ParentHash, "packetHash", packetHash)
	}

	broadcastPeerList := make([]string, 0)
	if connectedPeerList == nil || len(connectedPeerList) == 0 {
		log.Trace("GetPeerListToBroadcast empty connectedPeerList", "ParentHash", pkt.ParentHash, "packetHash", packetHash)
		return broadcastPeerList, nil
	}

	for _, peerId := range connectedPeerList {
		if pktInfo.fromPeerId == peerId {
			continue
		}
		if _, ok := pktInfo.sentPeerList[peerId]; ok == false {
			broadcastPeerList = append(broadcastPeerList, peerId)
			pktInfo.sentPeerList[peerId] = true
			log.Trace("broadcastPeerList", "hash", pkt.ParentHash, "broadcastPeer", peerId, "packetHash", packetHash)
		}
	}

	c.packetMap[packetHash] = pktInfo

	return broadcastPeerList, nil
}
