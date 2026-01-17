package defaults

import "os"

var sendStaticNodesOnly = os.Getenv("SEND_STATIC_AND_OUTBOUND_NODES_ONLY")

func SendStaticAndOutboundNodesOnly() bool {
	return sendStaticNodesOnly == "1"
}
