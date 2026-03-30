package proofofstake

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/backupmanager"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/eth/protocols/eth"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

type PacketMap struct {
	round                 byte
	proposalDetailsMap    map[common.Address]*ProposalDetails
	proposalAckDetailsMap map[common.Address]*ProposalAckDetails
	precommitDetailsMap   map[common.Address]*PreCommitDetails
	commitDetailsMap      map[common.Address]*CommitDetails
}

type PacketParseResult struct {
	round              byte
	packetType         ConsensusPacketType
	proposalDetails    *ProposalDetails
	proposalAckDetails *ProposalAckDetails
	precommitDetails   *PreCommitDetails
	commitDetails      *CommitDetails
	validator          common.Address
	err                error
}

var MAX_PACKETS_SAFETY_LIMIT = (MAX_VALIDATORS * 3 * int(MAX_ROUND+1)) + 2 //number 3 is the three phases of BFT, number 2 is proposals for each round and MAX_ROUND+1 is to account for any unknowns, instead of just using MAX_ROUND
var ContinueOnProposerCheckError = !defaults.EnableProposerCheck()
var validatorError = "validator not part of block"

func ParseConsensusPacket(wg *sync.WaitGroup, parentHash common.Hash, packet *eth.ConsensusPacket, filteredValidatorDepositMap map[common.Address]*big.Int,
	blockNumber uint64, validatorDetailsMap *map[common.Address]*ValidatorDetailsV2, consensusContext common.Hash, roundProposers map[byte]common.Address, resultsChan chan *PacketParseResult) {

	defer wg.Done()

	var err error
	var validator common.Address

	if packet.ParentHash.IsEqualTo(parentHash) == false {
		err = errors.New("unexpected parenthash")
		resultsChan <- &PacketParseResult{err: err}
		return
	}

	if packet.Signature == nil || packet.ConsensusData == nil || len(packet.Signature) == 0 || len(packet.ConsensusData) == 0 {
		err = errors.New("invalid consensus packet, nil data")
		resultsChan <- &PacketParseResult{err: err}
		return
	}

	dataToVerify := append(packet.ParentHash.Bytes(), packet.ConsensusData...)
	digestHash := crypto.Keccak256(dataToVerify)
	var pubKey *signaturealgorithm.PublicKey

	var startIndex int
	if packet.ConsensusData[0] >= MinConsensusNetworkProtocolVersion {
		startIndex = 2
	} else {
		startIndex = 1
	}

	isBreakGlass := defaults.IsCryptoBreakglassMode(blockNumber)
	if isBreakGlass && len(packet.Signature) != cryptobase.SigAlgHybridEdsFull.SignatureWithPublicKeyLength() {
		err = errors.New("invalid breakglass signature length")
		resultsChan <- &PacketParseResult{err: err}
	}

	sigAlg := cryptobase.GetSigAlgForValidation(blockNumber)

	packetType := ConsensusPacketType(packet.ConsensusData[startIndex-1])
	if isBreakGlass || (packetType == CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK && len(packet.Signature) != sigAlg.SignatureWithPublicKeyLength()) { //for verify, it is ok not to check the blockNumber for full
		log.Info("ParseConsensusPacket shouldSignFull", "sigAlg", sigAlg.SignatureName(), "IsCryptoBreakglassMode", isBreakGlass,
			"len(packet.Signature)", len(packet.Signature), "sigAlg.SignatureWithPublicKeyLength()", sigAlg.SignatureWithPublicKeyLength(), "name", sigAlg.SignatureName())
		var signContext []byte
		if blockNumber < defaults.DefaultConfig.PosConfig.SigAlgSwitchBlock {
			signContext = FULL_SIGN_CONTEXT
		} else {
			signContext = FULL_SIGN_CONTEXT_V2
		}
		pubKey, err = sigAlg.PublicKeyFromSignatureWithContext(digestHash, packet.Signature, signContext)
		if err != nil {
			err = InvalidPacketErr
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		if sigAlg.VerifyWithContext(pubKey.PubData, digestHash, packet.Signature, signContext) == false {
			err = InvalidPacketErr
			resultsChan <- &PacketParseResult{err: err}
			return
		}
	} else {
		pubKey, err = sigAlg.PublicKeyFromSignature(digestHash, packet.Signature)
		if err != nil {
			resultsChan <- &PacketParseResult{err: err}
			return
		}
		if sigAlg.Verify(pubKey.PubData, digestHash, packet.Signature) == false {
			err = InvalidPacketErr
			resultsChan <- &PacketParseResult{err: err}
			return
		}
	}

	validator, err = sigAlg.PublicKeyToAddress(pubKey)
	if err != nil {
		log.Debug("invalid 3", "err", err)
		resultsChan <- &PacketParseResult{err: err}
		return
	}

	_, ok := filteredValidatorDepositMap[validator]
	if ok == false {
		log.Debug("ParseConsensusPacket validator not part of block", "validator", validator, "count", len(filteredValidatorDepositMap))
		err = errors.New(validatorError)
		resultsChan <- &PacketParseResult{err: err}
		return
	}

	if packetType == CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK {
		details := ProposalDetails{}

		err = rlp.DecodeBytes(packet.ConsensusData[startIndex:], &details)
		if err != nil {
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		if details.Round < byte(1) || details.Round > MAX_ROUND {
			err = errors.New("invalid round a1")
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		blockProposer, ok := roundProposers[details.Round]
		if !ok {
			err = errors.New("round proposer not precomputed")
			resultsChan <- &PacketParseResult{err: err}
			return
		}
		if blockProposer.IsEqualTo(validator) == false {
			log.Warn("invalid block proposer", "expected", blockProposer, "actual", validator)
			if ContinueOnProposerCheckError == false {
				err = errors.New("invalid block proposer")
				resultsChan <- &PacketParseResult{err: err}
				return
			}
		}
		log.Debug("parseconsensuspackets propose", "details.Round", details.Round)
		packetDetail := &PacketParseResult{
			packetType:      packetType,
			proposalDetails: &details,
			validator:       validator,
			round:           details.Round,
		}
		resultsChan <- packetDetail
		return
	} else if packetType == CONSENSUS_PACKET_TYPE_ACK_BLOCK_PROPOSAL {
		details := ProposalAckDetails{}

		err = rlp.DecodeBytes(packet.ConsensusData[startIndex:], &details)
		if err != nil {
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		if details.Round < byte(1) || details.Round > MAX_ROUND {
			err = errors.New("invalid round a2")
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		packetDetail := &PacketParseResult{
			packetType:         packetType,
			proposalAckDetails: &details,
			validator:          validator,
			round:              details.Round,
		}
		resultsChan <- packetDetail
		return
	} else if packetType == CONSENSUS_PACKET_TYPE_PRECOMMIT_BLOCK {
		details := PreCommitDetails{}

		err = rlp.DecodeBytes(packet.ConsensusData[startIndex:], &details)
		if err != nil {
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		if details.Round < byte(1) || details.Round > MAX_ROUND {
			err = errors.New("invalid round a3")
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		packetDetail := &PacketParseResult{
			packetType:       packetType,
			precommitDetails: &details,
			validator:        validator,
			round:            details.Round,
		}
		resultsChan <- packetDetail
		return
	} else if packetType == CONSENSUS_PACKET_TYPE_COMMIT_BLOCK {
		details := CommitDetails{}

		err = rlp.DecodeBytes(packet.ConsensusData[startIndex:], &details)
		if err != nil {
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		if details.Round < byte(1) || details.Round > MAX_ROUND {
			err = errors.New("invalid round a4")
			resultsChan <- &PacketParseResult{err: err}
			return
		}

		packetDetail := &PacketParseResult{
			packetType:    packetType,
			commitDetails: &details,
			validator:     validator,
			round:         details.Round,
		}
		resultsChan <- packetDetail
		return
	} else {
		resultsChan <- &PacketParseResult{err: UnknownPacketTypeErr}
		return
	}
}

func ParseConsensusPackets(parentHash common.Hash, consensusPackets *[]eth.ConsensusPacket, filteredValidatorDepositMap map[common.Address]*big.Int,
	blockNumber uint64, validatorDetailsMap *map[common.Address]*ValidatorDetailsV2, consensusContext common.Hash, roundProposers map[byte]common.Address) (packetRoundMap map[byte]*PacketMap, err error) {
	packetRoundMap = make(map[byte]*PacketMap)

	packets := *consensusPackets
	if len(packets) > MAX_PACKETS_SAFETY_LIMIT {
		log.Warn("ParseConsensusPackets safety limit", "ParseConsensusPackets", ParseConsensusPackets, "actual count", len(packets))
		return nil, PacketsOverLimitErr
	}

	startTime := time.Now()
	var wg sync.WaitGroup
	ch := make(chan *PacketParseResult)

	for _, packet := range packets {
		wg.Add(1)
		go ParseConsensusPacket(&wg, parentHash, &packet, filteredValidatorDepositMap, blockNumber, validatorDetailsMap, consensusContext, roundProposers, ch)
	}
	results := make([]*PacketParseResult, len(packets))

	i := 0

	for packetParseResult := range ch {

		results[i] = packetParseResult
		i = i + 1
		if i == len(packets) {
			break
		}
	}

	close(ch)

	wg.Wait()
	log.Debug("ParseConsensusPacket list time taken", "elapsed", time.Since(startTime))

	for index, packetParseResult := range results {
		if packetParseResult.err != nil {
			log.Debug("ParseConsensusPackets", "err", packetParseResult.err)
			if packetParseResult.err.Error() == validatorError {
				continue
			}
			return nil, packetParseResult.err
		}

		_, ok := packetRoundMap[packetParseResult.round]
		if ok == false {
			packetRoundMap[packetParseResult.round] = &PacketMap{
				round:                 packetParseResult.round,
				proposalDetailsMap:    make(map[common.Address]*ProposalDetails),
				proposalAckDetailsMap: make(map[common.Address]*ProposalAckDetails),
				precommitDetailsMap:   make(map[common.Address]*PreCommitDetails),
				commitDetailsMap:      make(map[common.Address]*CommitDetails),
			}
		}

		if packetParseResult.packetType == CONSENSUS_PACKET_TYPE_PROPOSE_BLOCK {
			details := packetParseResult.proposalDetails
			log.Debug("parseconsensuspackets propose", "details.Round", details.Round)

			packetMap := packetRoundMap[details.Round]
			pktTest, ok := packetMap.proposalDetailsMap[packetParseResult.validator]
			if ok == true {
				log.Debug("duplicate proposal packet", "validator", packetParseResult.validator, "details.Round", details.Round,
					"txn count", len(details.Txns), "pktTest.Round", pktTest.Round, "len(pktTest.Txns)", len(pktTest.Txns), "index", index, "len(*consensusPackets)", len(*consensusPackets))
				return nil, errors.New("duplicate proposal packet")
			} else {
				log.Debug("proposal packet", "validator", packetParseResult.validator, "Round", details.Round, "count", len(details.Txns), "index", index)
			}
			proposalDetails := &ProposalDetails{
				Round: details.Round,
				Txns:  make([]common.Hash, len(details.Txns)),
			}
			for i, txn := range details.Txns {
				proposalDetails.Txns[i].CopyFrom(txn)
			}
			packetMap.proposalDetailsMap[packetParseResult.validator] = proposalDetails
			packetRoundMap[details.Round] = packetMap
		} else if packetParseResult.packetType == CONSENSUS_PACKET_TYPE_ACK_BLOCK_PROPOSAL {
			details := packetParseResult.proposalAckDetails
			packetMap := packetRoundMap[details.Round]
			_, ok := packetMap.proposalAckDetailsMap[packetParseResult.validator]
			if ok == true {
				log.Warn("duplicate ack proposal packet", "validator", packetParseResult.validator)
				return nil, errors.New("duplicate ack proposal packet")
			}
			proposalAckDetails := &ProposalAckDetails{
				Round:               details.Round,
				ProposalAckVoteType: details.ProposalAckVoteType,
			}
			proposalAckDetails.ProposalHash.CopyFrom(details.ProposalHash)
			if proposalAckDetails.ProposalAckVoteType != VOTE_TYPE_NIL && proposalAckDetails.ProposalAckVoteType != VOTE_TYPE_OK {
				log.Debug("proposalAckDetails.ProposalAckVoteType", "ProposalAckVoteType", proposalAckDetails.ProposalAckVoteType)
				return nil, errors.New("invalid vote type a")
			}

			if details.Round == MAX_ROUND && proposalAckDetails.ProposalAckVoteType != VOTE_TYPE_NIL {
				log.Debug("proposalAckDetails.ProposalAckVoteType", "ProposalAckVoteType", proposalAckDetails.ProposalAckVoteType)
				return nil, errors.New("invalid vote type expecting nil")
			}

			packetMap.proposalAckDetailsMap[packetParseResult.validator] = proposalAckDetails
			packetRoundMap[details.Round] = packetMap
		} else if packetParseResult.packetType == CONSENSUS_PACKET_TYPE_PRECOMMIT_BLOCK {
			details := packetParseResult.precommitDetails

			packetMap := packetRoundMap[details.Round]
			_, ok := packetMap.precommitDetailsMap[packetParseResult.validator]
			if ok == true {
				log.Warn("duplicate precommit packet", "validator", packetParseResult.validator)
				return nil, errors.New("duplicate precommit packet")
			}
			precommitDetails := &PreCommitDetails{
				Round: details.Round,
			}
			precommitDetails.PrecommitHash.CopyFrom(details.PrecommitHash)

			packetMap.precommitDetailsMap[packetParseResult.validator] = precommitDetails
			packetRoundMap[details.Round] = packetMap
		} else if packetParseResult.packetType == CONSENSUS_PACKET_TYPE_COMMIT_BLOCK {
			details := packetParseResult.commitDetails

			packetMap := packetRoundMap[details.Round]
			_, ok := packetMap.commitDetailsMap[packetParseResult.validator]
			if ok == true {
				log.Warn("duplicate commit packet", "validator", packetParseResult.validator)
				return nil, errors.New("duplicate commit packet")
			}
			commitDetails := &CommitDetails{
				Round: details.Round,
			}
			commitDetails.CommitHash.CopyFrom(details.CommitHash)

			packetMap.commitDetailsMap[packetParseResult.validator] = commitDetails
			packetRoundMap[details.Round] = packetMap
		} else {
			return nil, UnknownPacketTypeErr
		}
	}

	return packetRoundMap, nil
}

func VerifyPackets(parentHash common.Hash, round byte, packetMap *PacketMap, voteType VoteType,
	filteredValidatorDepositMap *map[common.Address]*big.Int, totalBlockDepositValue *big.Int, minDepositRequired *big.Int, txns []common.Hash, blockNumber uint64, proposedBlockTime uint64,
	nilVoteProposalHashes map[byte]common.Hash, nilVotePrecommitHashes map[byte]common.Hash) error {
	valMap := *filteredValidatorDepositMap

	okVotesDepositValue := big.NewInt(0)
	nilVotesDepositValue := big.NewInt(0)

	var proposalHash common.Hash
	if voteType == VOTE_TYPE_OK {
		log.Debug("GetCombinedTxnHash a", "parentHash", parentHash, "round", round, "count", len(txns))
		if blockNumber >= defaults.DefaultConfig.PosConfig.PROPOSAL_TIME_HASH_START_BLOCK {
			proposalHash = GetCombinedTxnHashWithTime(parentHash, round, txns, proposedBlockTime)
		} else {
			proposalHash = GetCombinedTxnHash(parentHash, round, txns)
		}
	} else {
		log.Debug("GetCombinedTxnHash b", "parentHash", parentHash, "round", round)
		ph, err := lookupNilVoteProposalHash(nilVoteProposalHashes, round)
		if err != nil {
			return err
		}
		proposalHash.CopyFrom(ph)
		if txns != nil && len(txns) > 0 {
			return errors.New("invalid transactions with nil vote")
		}
	}

	for v, proposalAckDetails := range packetMap.proposalAckDetailsMap {
		depositValue, ok := valMap[v]
		if ok == false {
			return errors.New("unrecognized validator")
		}

		if proposalAckDetails.Round != round {
			return errors.New("invalid round f")
		}
		log.Debug("val dep", "val", v, "depositValue", depositValue, "ProposalAckVoteType", proposalAckDetails.ProposalAckVoteType, "ProposalHash", proposalAckDetails.ProposalHash)

		if proposalAckDetails.ProposalAckVoteType == VOTE_TYPE_NIL {
			if proposalAckDetails.ProposalHash.IsEqualTo(proposalHash) == false { //can be OK VOTE as well
				if voteType != VOTE_TYPE_OK { //can be ok VOTE as well
					log.Debug("proposal hash 2", "proposalHash", proposalHash, "proposalAckDetails.ProposalHash", proposalAckDetails.ProposalHash)
					return errors.New("invalid proposal hash")
				}
				continue
			}
			nilVotesDepositValue = common.SafeAddBigInt(nilVotesDepositValue, depositValue)
		} else if proposalAckDetails.ProposalAckVoteType == VOTE_TYPE_OK {
			if proposalAckDetails.ProposalHash.IsEqualTo(proposalHash) == false {
				if voteType != VOTE_TYPE_NIL { //can be NIL VOTE as well
					log.Debug("proposal hash 1", "proposalHash", proposalHash, "proposalAckDetails.ProposalHash", proposalAckDetails.ProposalHash, "voteType", voteType)
				}
				continue
			}
			okVotesDepositValue = common.SafeAddBigInt(okVotesDepositValue, depositValue)
		} else {
			return errors.New("invalid vote type b")
		}

		totalVotesDepositValue := common.SafeAddBigInt(nilVotesDepositValue, okVotesDepositValue)
		if totalVotesDepositValue.Cmp(totalBlockDepositValue) > 0 {
			return errors.New("invalid totalVotesDepositValue")
		}
	}

	log.Debug("VerifyPackets", "minDepositRequired", minDepositRequired, "okVotesDepositValue", okVotesDepositValue, "nilVotesDepositValue", nilVotesDepositValue,
		"voteType", voteType, "proposalAckDetails", len(packetMap.proposalAckDetailsMap), "txns", len(txns))
	var precommitHash common.Hash
	if voteType == VOTE_TYPE_NIL {
		if okVotesDepositValue.Cmp(minDepositRequired) >= 0 {
			return errors.New("VOTE_TYPE_NIL okVotesDepositValue error")
		}

		if nilVotesDepositValue.Cmp(minDepositRequired) < 0 {
			return errors.New("VOTE_TYPE_NIL nilVotesDepositValue error")
		}

		var err error
		precommitHash, err = lookupNilVotePrecommitHash(nilVotePrecommitHashes, round)
		if err != nil {
			return err
		}
	} else {
		if okVotesDepositValue.Cmp(minDepositRequired) < 0 {
			return errors.New("VOTE_TYPE_OK okVotesDepositValue error")
		}

		if nilVotesDepositValue.Cmp(minDepositRequired) >= 0 {
			return errors.New("VOTE_TYPE_OK nilVotesDepositValue error")
		}

		precommitHash = getOkVotePreCommitHash(parentHash, proposalHash, round)
	}
	commitHash := getCommitHash(precommitHash)
	precommitDepositValue := big.NewInt(0)
	for v, precommitDetails := range packetMap.precommitDetailsMap {
		depositValue, ok := valMap[v]
		if ok == false {
			return errors.New("unrecognized validator")
		}

		if precommitDetails.PrecommitHash.IsEqualTo(precommitHash) == false {
			return errors.New("invalid precommithash")
		}

		precommitDepositValue = common.SafeAddBigInt(precommitDepositValue, depositValue)
	}

	if precommitDepositValue.Cmp(minDepositRequired) < 0 {
		return errors.New("precommit low deposit")
	}

	commitDepositValue := big.NewInt(0)
	for v, commitDetails := range packetMap.commitDetailsMap {
		depositValue, ok := valMap[v]
		if ok == false {
			return errors.New("unrecognized validator")
		}

		if commitDetails.CommitHash.IsEqualTo(commitHash) == false {
			return errors.New("invalid commithash")
		}

		commitDepositValue = common.SafeAddBigInt(commitDepositValue, depositValue)
	}

	if commitDepositValue.Cmp(minDepositRequired) < 0 {
		return errors.New("precommit low deposit")
	}

	return nil
}

func VerifyBlockConsensusDataInner(txns []common.Hash, parentHash common.Hash, blockConsensusData *BlockConsensusData, blockAdditionalConsensusData *BlockAdditionalConsensusData,
	preparedState *PreparedConsensusState, blockNumber uint64, consensusContext common.Hash) (*backupmanager.BlockExtendedDetails, error) {
	if blockConsensusData.Round < 1 {
		return nil, errors.New("VerifyBlockConsensusData round min")
	}

	if blockConsensusData.Round >= MAX_ROUND && txns != nil && len(txns) > 0 { //todo: is this valid?
		return nil, errors.New("VerifyBlockConsensusData round max")
	}

	if blockConsensusData.PrecommitHash.IsEqualTo(ZERO_HASH) {
		return nil, errors.New("VerifyBlockConsensusData PrecommitHash zero_hash")
	}

	nilVotedProposers := make(map[common.Address]bool)
	var slashedBlockProposer common.Address
	if blockConsensusData.SlashedBlockProposers != nil {
		for _, proposer := range blockConsensusData.SlashedBlockProposers {
			nilVotedProposers[proposer] = true
			slashedBlockProposer.CopyFrom(proposer)
			log.Debug("VerifyBlockConsensusDataInner proposer slashed", "proposer", proposer)
		}
	}

	if preparedState == nil {
		return nil, errors.New("VerifyBlockConsensusDataInner preparedState nil")
	}

	filteredValidatorDepositMap := preparedState.FilteredValidatorsDepositMap
	totalBlockDepositValue := preparedState.TotalBlockDepositValue
	minDepositRequired := preparedState.MinDepositRequired
	preparedValDetailsMap := preparedState.ValidatorDetailsMap

	blockExtendedDetails := backupmanager.BlockExtendedDetails{
		BlockNumber:      big.NewInt(int64(blockNumber)),
		ParentHash:       parentHash,
		FilteredDeposits: make([]backupmanager.ValidatorDeposit, len(filteredValidatorDepositMap)),
	}
	blockExtendedDetails.ConsensusContext.CopyFrom(consensusContext)

	valIndex := 0
	for v, dep := range filteredValidatorDepositMap {
		blockExtendedDetails.FilteredDeposits[valIndex] = backupmanager.ValidatorDeposit{
			ValidatorAddress:  v,
			PostFilterDeposit: dep,
		}
		valIndex++
	}

	if blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK {
		blockExtendedDetails.StakingValidatorDetails = make([]backupmanager.ValidatorDetailsV2, len(preparedValDetailsMap))
		valDetailsIndex := 0
		for _, valDetails := range preparedValDetailsMap {
			blockExtendedDetails.StakingValidatorDetails[valDetailsIndex] = (backupmanager.ValidatorDetailsV2)(*valDetails)
			valDetailsIndex++
		}
	}

	var err error
	roundBlockProposers := make(map[byte]common.Address)
	for r := byte(1); r <= blockConsensusData.Round; r++ {
		roundBlockProposers[r], err = lookupRoundProposer(preparedState.RoundProposers, r)
		if err != nil {
			return nil, err
		}
		log.Debug("roundBlockProposers[r]", "r", r, "roundBlockProposers[r]", roundBlockProposers[r])
	}

	if blockAdditionalConsensusData.ConsensusPackets == nil {
		return nil, errors.New("nil ConsensusPackets")
	}

	packetRoundMap, err := ParseConsensusPackets(parentHash, &blockAdditionalConsensusData.ConsensusPackets, filteredValidatorDepositMap, blockNumber, &preparedValDetailsMap, consensusContext, preparedState.RoundProposers)
	if err != nil {
		return nil, err
	}

	if blockConsensusData.VoteType == VOTE_TYPE_NIL {
		if len(txns) > 0 {
			return nil, errors.New("txns in a NIL block")
		}
		if blockConsensusData.SelectedTransactions != nil && len(blockConsensusData.SelectedTransactions) > 0 {
			return nil, errors.New("SelectedTransactions in a NIL vote")
		}
		if blockConsensusData.BlockProposer.IsEqualTo(ZERO_ADDRESS) == false {
			return nil, errors.New("VerifyBlockConsensusData BlockProposer false")
		}

		expProp, err := lookupNilVoteProposalHash(preparedState.NilVoteProposalHashes, blockConsensusData.Round)
		if err != nil {
			return nil, err
		}
		if blockConsensusData.ProposalHash.IsEqualTo(expProp) == false {
			return nil, errors.New("proposal hash check failed")
		}

		precommitHash, err := lookupNilVotePrecommitHash(preparedState.NilVotePrecommitHashes, blockConsensusData.Round)
		if err != nil {
			return nil, err
		}
		if blockConsensusData.PrecommitHash.IsEqualTo(precommitHash) == false {
			return nil, errors.New("precommitHash hash check failed")
		}

		for r := byte(1); r <= blockConsensusData.Round; r++ {
			if r < MAX_ROUND {
				_, ok := nilVotedProposers[roundBlockProposers[r]]
				if ok == false {
					log.Warn("NilVotesProposer doesn't match", "blockNumber", blockNumber, "roundBlockProposers[r]", roundBlockProposers[r], "r", r, "parentHash", parentHash, "slashedBlockProposer", slashedBlockProposer, "ContinueOnProposerCheckError", ContinueOnProposerCheckError)
					if ContinueOnProposerCheckError == false {
						return &blockExtendedDetails, errors.New("nilVotedProposers 1")
					}
				}
			}

			_, ok := packetRoundMap[r]
			if ok == false {
				log.Debug("could not find packetMap for round", "r", r)
				return nil, errors.New("could not find packetMap for round")
			}
		}

		if len(nilVotedProposers) > int(blockConsensusData.Round) {
			return nil, errors.New("unexpected number of nilVotedProposers")
		}

		packetMap := packetRoundMap[blockConsensusData.Round]
		err = VerifyPackets(parentHash, blockConsensusData.Round, packetMap, VOTE_TYPE_NIL, &filteredValidatorDepositMap, totalBlockDepositValue, minDepositRequired, blockConsensusData.SelectedTransactions, blockNumber, blockConsensusData.BlockTime,
			preparedState.NilVoteProposalHashes, preparedState.NilVotePrecommitHashes)
		if err != nil {
			return nil, err
		}

		//todo: deep validate block proposers
	} else if blockConsensusData.VoteType == VOTE_TYPE_OK {
		if blockConsensusData.BlockProposer.IsEqualTo(ZERO_ADDRESS) {
			return nil, errors.New("VerifyBlockConsensusData BlockProposer true")
		}

		if blockConsensusData.BlockProposer.IsEqualTo(roundBlockProposers[blockConsensusData.Round]) == false {
			log.Warn("VerifyBlockConsensusData BlockProposer mismatch", "blockConsensusData.BlockProposer", blockConsensusData.BlockProposer,
				"roundBlockProposers[blockConsensusData.Round]", roundBlockProposers[blockConsensusData.Round])
			if ContinueOnProposerCheckError == false {
				return &blockExtendedDetails, errors.New("VerifyBlockConsensusData BlockProposer true")
			}
		}

		if blockConsensusData.ProposalHash.IsEqualTo(ZERO_HASH) {
			log.Debug("VerifyBlockConsensusData ProposalHash zero_hash")
			return nil, errors.New("VerifyBlockConsensusData ProposalHash zero_hash")
		}

		if blockConsensusData.SelectedTransactions == nil {
			if len(txns) > 0 {
				return nil, errors.New("VerifyBlockConsensusData txns is non-empty but SelectedTransactions is nil")
			}
		} else {
			var selectedTxnsMap map[common.Hash]bool
			selectedTxnsMap = make(map[common.Hash]bool)
			for _, txn := range blockConsensusData.SelectedTransactions {
				selectedTxnsMap[txn] = true
			}
			if txns != nil {
				for _, txn := range txns {
					_, ok := selectedTxnsMap[txn]
					if ok == false {
						log.Debug("VerifyBlockConsensusData txn", "txn", txn)
						return nil, errors.New("VerifyBlockConsensusData txns should be a subset of blockConsensusData.SelectedTransactions")
					}
				}
			}
		}

		if blockConsensusData.Round > 1 {
			for r := byte(1); r < blockConsensusData.Round; r++ {
				_, ok := nilVotedProposers[roundBlockProposers[r]]
				if ok == false {
					log.Warn("NilVotesProposer 2", "roundBlockProposers[r]", roundBlockProposers[r], "r", r, "parentHash", parentHash)
					if ContinueOnProposerCheckError == false {
						return &blockExtendedDetails, errors.New("nilVotedProposers 2")
					}
				}
			}
			if len(blockConsensusData.SlashedBlockProposers) < int(blockConsensusData.Round-1) {
				log.Debug("SlashedBlockProposers", "len(nilVotedProposers)", len(nilVotedProposers), "int(blockConsensusData.Round)", int(blockConsensusData.Round))
				return nil, errors.New("VerifyBlockConsensusData SlashedBlockProposers length")
			}
		}

		packetMap := packetRoundMap[blockConsensusData.Round]
		err = VerifyPackets(parentHash, blockConsensusData.Round, packetMap, VOTE_TYPE_OK, &filteredValidatorDepositMap, totalBlockDepositValue, minDepositRequired, blockConsensusData.SelectedTransactions, blockNumber, blockConsensusData.BlockTime,
			preparedState.NilVoteProposalHashes, preparedState.NilVotePrecommitHashes)
		if err != nil {
			return nil, err
		}

	} else {
		log.Debug("VerifyBlockConsensusData unexpected vote type", "vote type", blockConsensusData.VoteType)
		return nil, errors.New("VerifyBlockConsensusData unexpected vote type")
	}

	return &blockExtendedDetails, nil
}

// In this function, absolute time cannot be validated, since this function can get called at a different time, for example when new node is created and is reading old blocks
// Hence only basic checks are allowed
func VerifyBlockProposalTime(blockNumber uint64, proposedTime uint64) bool {
	if blockNumber == 1 || blockNumber%BLOCK_PERIOD_TIME_CHANGE == 0 || blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_TIME_ORIG_START_BLOCK {
		if proposedTime == 0 {
			return true
		}

		tm := time.Unix(int64(proposedTime), 0)
		if tm.Second() != 0 || tm.Nanosecond() != 0 { //No granularity at anything other than minute level allowed, to reduce ability to manipulate blockHash
			log.Warn("VerifyBlockProposalTime granularity issue", "second", tm.Second(), "nanosecond", tm.Nanosecond())
			return false
		}
	} else {
		if proposedTime != 0 {
			log.Warn("VerifyBlockProposalTime granularity issue", "proposedTime", proposedTime)
			return false
		}
	}

	return true
}

// VerifyBlockConsensusData verifies consensus fields on a block. validatorDepositMap and valDetailsMap are
// deprecated and ignored; validator state is loaded via getValidatorsFn and listValidatorsFn.
func VerifyBlockConsensusData(block *types.Block, validatorDepositMap *map[common.Address]*big.Int,
	valDetailsMap *map[common.Address]*ValidatorDetailsV2, getBlockConsensusContext GetBlockConsensusContextFn, getValidatorsFn GetValidatorsFn, listValidatorsFn ListValidatorsAsMapFn) error {
	_ = validatorDepositMap
	_ = valDetailsMap
	header := block.Header()

	if header.ConsensusData == nil || header.UnhashedConsensusData == nil {
		return errors.New("VerifyBlockConsensusData nil")
	}

	blockConsensusData := &BlockConsensusData{}
	err := rlp.DecodeBytes(header.ConsensusData, blockConsensusData)
	if err != nil {
		return err
	}

	if blockConsensusData.SlashedBlockProposers == nil || blockConsensusData.SelectedTransactions == nil {
		return errors.New("VerifyBlockConsensusData SlashedBlockProposers or SelectedTransactions is nil")
	}

	blockAdditionalConsensusData := &BlockAdditionalConsensusData{}
	err = rlp.DecodeBytes(header.UnhashedConsensusData, blockAdditionalConsensusData)
	if err != nil {
		return err
	}

	if blockAdditionalConsensusData.ConsensusPackets == nil {
		return errors.New("VerifyBlockConsensusData ConsensusPackets is nil")
	}

	txns := block.Transactions()
	var txnList []common.Hash
	if txns != nil {
		txnList = make([]common.Hash, len(txns))
		for i, t := range txns {
			txnList[i].CopyFrom(t.Hash())
		}
	} else {
		txnList = make([]common.Hash, 0)
	}

	if VerifyBlockProposalTime(block.Number().Uint64(), blockConsensusData.BlockTime) == false {
		log.Warn("VerifyBlockProposalTime failed", "blockNumber", block.Number().Uint64(), "proposedTime", blockConsensusData.BlockTime)
		return errors.New("VerifyBlockProposalTime failed")
	}

	blockNumber := header.Number.Uint64()
	preparedData, err := PrepareConsensusData(header.ParentHash, blockNumber, getValidatorsFn, getBlockConsensusContext, listValidatorsFn, common.Hash{})
	if err != nil {
		return err
	}
	consensusContext := preparedData.ConsensusContext
	preFilterValidatorCount := preparedData.PreFilterValidatorCount

	blockExtendedDetails, err := VerifyBlockConsensusDataInner(txnList, header.ParentHash, blockConsensusData, blockAdditionalConsensusData, preparedData.Prepared, blockNumber, consensusContext)
	if blockExtendedDetails != nil && backupmanager.GetConsensusInstance() != nil { //save even if error
		blockExtendedDetails.PreFilterValidatorCount = big.NewInt(int64(preFilterValidatorCount))
		blockExtendedDetails.PreparedConsensusState = preparedConsensusStateToBackup(preparedData.Prepared)
		if preparedData.Prepared.BlockProposerV2Traces != nil {
			blockExtendedDetails.BlockProposerV2Traces = make([]*backupmanager.BlockProposerV2RoundTrace, len(preparedData.Prepared.BlockProposerV2Traces))
			copy(blockExtendedDetails.BlockProposerV2Traces, preparedData.Prepared.BlockProposerV2Traces)
		}
		blockExtendedContext := backupmanager.BlockExtendedContextBlockVerify
		if err != nil {
			blockExtendedContext = backupmanager.BlockExtendedContextBlockVerifyError
		}
		errBackup := backupmanager.GetConsensusInstance().BackupBlockExtendedDetails(blockExtendedDetails, blockExtendedContext)
		if errBackup != nil {
			log.Warn("VerifyBlockConsensusDataInner backup consensus", "errBackup", errBackup)
		}
	}

	return err
}
