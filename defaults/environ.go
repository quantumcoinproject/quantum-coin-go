package defaults

import (
	"os"
	"strconv"

	"github.com/quantumcoinproject/quantum-coin-go/log"
)

var sendStaticNodesOnly = os.Getenv("SEND_STATIC_AND_OUTBOUND_NODES_ONLY")
var skipRebroadcastConsensusPackets = os.Getenv("SKIP_REBROADCAST_CONSENSUS_PACKETS")
var skipPropagateBlock = os.Getenv("SKIP_PROPAGATE_BLOCK")
var enableProposerCheck = os.Getenv("ENABLE_PROPOSER_CHECK")
var skipStartupDelay = os.Getenv("SKIP_STARTUP_DELAY")
var skipDeepBlockCheck = os.Getenv("SKIP_BLOCK_DEEP_CHECK")
var skipMissingTxn = os.Getenv("SKIP_MISSING_TXN")
var skipConsensusStartupHashCheck = os.Getenv("SKIP_CONSENSUS_STARTUP_HASH_CHECK")
var skipProposalTimeDiffCheck = os.Getenv("SKIP_PROPOSAL_TIME_DIFF_CHECK")
var failExperimentalReorg = os.Getenv("EXPERIMENTAL_FAIL_REORG")
var isConsensusRelay = os.Getenv("IS_CONSENSUS_RELAY")
var testHookEndpoint = os.Getenv("TEST_HOOK_ENDPOINT")
var enableBlockExtendedSave = os.Getenv("BLOCK_EXTENDED_SAVE")

func SendStaticAndOutboundNodesOnly() bool {
	return sendStaticNodesOnly == "1"
}

func SkipRebroadcastConsensusPackets() bool {
	return skipRebroadcastConsensusPackets == "1"
}

func SkipStartupDelay() bool {
	return skipStartupDelay == "1"
}

func SkipPropagateBlock() bool {
	return skipPropagateBlock == "1"
}

func EnableProposerCheck() bool {
	return enableProposerCheck == "1"
}

func EnableBlockExtendedSave() bool {
	return enableBlockExtendedSave == "1"
}

func SkipDeepBlockCheck() bool {
	return skipDeepBlockCheck == "1"
}

func SkipMissingTxn() bool {
	return skipMissingTxn == "1"
}

func SkipConsensusStartupHashCheck() bool {
	return skipConsensusStartupHashCheck == "1"
}

func SkipProposalTimeDiffCheck() bool {
	return skipProposalTimeDiffCheck == "1"
}

func FailExperimentalReorg() bool {
	return failExperimentalReorg == "1"
}

func IsConsensusRelay() bool {
	return isConsensusRelay == "1"
}

func GetMinValidatorsOverride() int {
	minValStr := os.Getenv("MIN_VALIDATORS")
	var minVal int
	if len(minValStr) > 0 {
		var err error
		minVal, err = strconv.Atoi(minValStr)
		if err != nil {
			log.Error("Error parsing MIN_VALIDATORS environment variable")
			panic(err)
		}
		if minVal < 1 || minVal > minVal {
			log.Error("Invalid MIN_VALIDATORS", "MIN_VALIDATORS", minVal)
			panic("Invalid MIN_VALIDATORS")
		}
	}

	return minVal
}

func GetTestHookEndpoint() string {
	return testHookEndpoint
}
