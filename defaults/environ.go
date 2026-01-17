package defaults

import "os"

var sendStaticNodesOnly = os.Getenv("SEND_STATIC_NODES_ONLY")

func SendStaticNodesOnly() bool {
	return sendStaticNodesOnly == "1"
}
