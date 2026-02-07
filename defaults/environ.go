package defaults

import "os"

var sendStaticNodesOnly = os.Getenv("SEND_STATIC_AND_OUTBOUND_NODES_ONLY")
var skipRebroadcastConsensusPackets = os.Getenv("SKIP_REBROADCAST_CONSENSUS_PACKETS")
var skipPropagateBlock = os.Getenv("SKIP_PROPAGATE_BLOCK")

func SendStaticAndOutboundNodesOnly() bool {
	return sendStaticNodesOnly == "1"
}

func SkipRebroadcastConsensusPackets() bool {
	return skipRebroadcastConsensusPackets == "1"
}

func SkipPropagateBlock() bool {
	return skipPropagateBlock == "1"
}
