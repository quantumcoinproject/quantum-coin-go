package handler

import (
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"math/rand"
	"sync"
	"time"
)

const CleanupIntervalSeconds = 1800 * time.Second
const ResendInterval = 6000

type PacketInfo struct {
	packet       *eth.ConsensusPacket
	fromPeerId   string
	sentPeerList map[string]int64
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

func ElapsedMs(startTime int64) int64 {
	end := time.Now().UnixNano() / int64(time.Millisecond)
	start := startTime / int64(time.Millisecond)
	diff := end - start
	return diff
}

func (c *ConsensusPacketHelper) GetPeerListToBroadcast(fromPeerId string, pkt *eth.ConsensusPacket, connectedPeerList []string, rebroadcastCount int) ([]string, error) {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	c.cleanup()

	packetHash := pkt.Hash()
	pktInfo, ok := c.packetMap[packetHash]
	if ok == false {
		pktInfo = &PacketInfo{
			fromPeerId:   fromPeerId,
			sentPeerList: make(map[string]int64),
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

	for i := len(connectedPeerList) - 1; i > 0; i-- { //Fisher Yates shuffle. Send to a random set of peers each time
		minVal := 0
		maxVal := i
		j := rand.Intn(maxVal-minVal) + minVal //non-crypto rand is ok for this purpose
		temp := connectedPeerList[i]
		connectedPeerList[i] = connectedPeerList[j]
		connectedPeerList[j] = temp
	}

	for i, peerId := range connectedPeerList {
		if i >= rebroadcastCount {
			break
		}
		if pktInfo.fromPeerId == peerId {
			continue
		}
		timeSent, ok := pktInfo.sentPeerList[peerId]
		if ok == false || ElapsedMs(timeSent) > ResendInterval {
			broadcastPeerList = append(broadcastPeerList, peerId)
			pktInfo.sentPeerList[peerId] = time.Now().UnixNano()
			log.Trace("broadcastPeerList", "hash", pkt.ParentHash, "broadcastPeer", peerId, "packetHash", packetHash)
		}
	}

	c.packetMap[packetHash] = pktInfo

	return broadcastPeerList, nil
}
