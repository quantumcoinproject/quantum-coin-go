package proofofstake

import (
	"bytes"
	"sort"

	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
)

// serializeSelectedValidatorEvaluations builds trace rows for the final selectedValMap, sorted by address (CanPropose from canPropose at serialize time).
func serializeSelectedValidatorEvaluations(selectedValMap map[common.Address]*ValidatorDetailsV2, blockNumber uint64) []backupmanager.BlockProposerV2ValidatorEval {
	addrs := make([]common.Address, 0, len(selectedValMap))
	for valAddr := range selectedValMap {
		addrs = append(addrs, valAddr)
	}
	sort.Slice(addrs, func(i, j int) bool {
		return bytes.Compare(addrs[i][:], addrs[j][:]) < 0
	})
	evals := make([]backupmanager.BlockProposerV2ValidatorEval, 0, len(addrs))
	for _, valAddr := range addrs {
		valDetails := selectedValMap[valAddr]
		canProp, _ := canPropose(valDetails, blockNumber)
		evals = append(evals, backupmanager.BlockProposerV2ValidatorEval{
			ValidatorAddress: valAddr,
			ValidatorDetails: validatorDetailsV2ToBackup(valDetails),
			CanPropose:       canProp,
		})
	}
	return evals
}
