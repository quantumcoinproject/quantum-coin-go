package proofofstake

import (
	"errors"
	"math/big"
	"strconv"
	"sync"
	"time"

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

func ParseConsensusPacket(wg *sync.WaitGroup, parentHash common.Hash, packet *eth.ConsensusPacket, filteredValidatorDepositMap map[common.Address]*big.Int,
	blockNumber uint64, validatorDetailsMap *map[common.Address]*ValidatorDetailsV2, consensusContext common.Hash, resultsChan chan *PacketParseResult) {

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

		err = errors.New("validator not part of block")
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

		blockProposer, err := getBlockProposer(parentHash, &filteredValidatorDepositMap, details.Round, validatorDetailsMap, blockNumber, consensusContext)
		if err != nil {
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
	blockNumber uint64, validatorDetailsMap *map[common.Address]*ValidatorDetailsV2, consensusContext common.Hash) (packetRoundMap map[byte]*PacketMap, err error) {
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
		go ParseConsensusPacket(&wg, parentHash, &packet, filteredValidatorDepositMap, blockNumber, validatorDetailsMap, consensusContext, ch)
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

func ValidatePackets(parentHash common.Hash, round byte, packetMap *PacketMap, voteType VoteType,
	filteredValidatorDepositMap *map[common.Address]*big.Int, totalBlockDepositValue *big.Int, minDepositRequired *big.Int, txns []common.Hash, blockNumber uint64, proposedBlockTime uint64) error {
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
		proposalHash.CopyFrom(getNilVoteProposalHash(parentHash, round))
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

	log.Debug("ValidatePackets", "minDepositRequired", minDepositRequired, "okVotesDepositValue", okVotesDepositValue, "nilVotesDepositValue", nilVotesDepositValue,
		"voteType", voteType, "proposalAckDetails", len(packetMap.proposalAckDetailsMap), "txns", len(txns))
	var precommitHash common.Hash
	if voteType == VOTE_TYPE_NIL {
		if okVotesDepositValue.Cmp(minDepositRequired) >= 0 {
			return errors.New("VOTE_TYPE_NIL okVotesDepositValue error")
		}

		if nilVotesDepositValue.Cmp(minDepositRequired) < 0 {
			return errors.New("VOTE_TYPE_NIL nilVotesDepositValue error")
		}

		precommitHash = getNilVotePreCommitHash(parentHash, round)
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
			errors.New("invalid precommithash")
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
			errors.New("invalid commithash")
		}

		commitDepositValue = common.SafeAddBigInt(commitDepositValue, depositValue)
	}

	if commitDepositValue.Cmp(minDepositRequired) < 0 {
		return errors.New("precommit low deposit")
	}

	return nil
}

func ValidateBlockConsensusDataInner(txns []common.Hash, parentHash common.Hash, blockConsensusData *BlockConsensusData, blockAdditionalConsensusData *BlockAdditionalConsensusData,
	validatorDepositMap *map[common.Address]*big.Int, blockNumber uint64, valDetailsMap *map[common.Address]*ValidatorDetailsV2, consensusContext common.Hash) error {
	if blockConsensusData.Round < 1 {
		return errors.New("ValidateBlockConsensusData round min")
	}

	if blockConsensusData.Round >= MAX_ROUND && txns != nil && len(txns) > 0 { //todo: is this valid?
		return errors.New("ValidateBlockConsensusData round max")
	}

	if blockConsensusData.PrecommitHash.IsEqualTo(ZERO_HASH) {
		return errors.New("ValidateBlockConsensusData PrecommitHash zero_hash")
	}

	nilVotedProposers := make(map[common.Address]bool)
	var slashedBlockProposer common.Address
	if blockConsensusData.SlashedBlockProposers != nil {
		for _, proposer := range blockConsensusData.SlashedBlockProposers {
			nilVotedProposers[proposer] = true
			slashedBlockProposer.CopyFrom(proposer)
			log.Debug("ValidateBlockConsensusDataInner proposer slashed", "proposer", proposer)
		}
	}

	valMap := *validatorDepositMap
	filteredValidators, totalBlockDepositValue, minDepositRequired, err := filterValidators(consensusContext, &valMap, blockNumber, valDetailsMap)
	if err != nil {
		return err
	}

	if MIN_BLOCK_DEPOSIT.Cmp(minDepositRequired) > 0 {
		return errors.New("min deposit required error")
	}

	if len(filteredValidators) < MIN_VALIDATORS {
		return errors.New("filteredValidators MIN_VALIDATORS")
	}

	if len(filteredValidators) > MAX_VALIDATORS {
		return errors.New("filteredValidators MAX_VALIDATORS")
	}

	var filteredValidatorDepositMap map[common.Address]*big.Int
	filteredValidatorDepositMap = make(map[common.Address]*big.Int)

	for v, _ := range filteredValidators {
		filteredValidatorDepositMap[v] = valMap[v]
		log.Debug("ValidateBlockConsensusDataInner", "validator", v, "deposit value after filtering", valMap[v], "blockNumber", blockNumber)
	}

	if blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_PROPOSER_NIL_BLOCK_START_BLOCK {
		for valAddr, valDetails := range *valDetailsMap {
			if valDetails.IsValidationPaused { //filteredValidators will already have skipped paused validators, no need to skip again for filteredValidatorDepositMap
				delete(*valDetailsMap, valAddr)
				log.Debug("ValidateBlockConsensusDataInner ValidationPaused remove", "validator", valAddr, "blockNumber", blockNumber)
				continue
			}
			_, ok := filteredValidatorDepositMap[valAddr]
			if ok == false {
				log.Debug("ValidateBlockConsensusDataInner filteredValidatorDepositMap remove", "validator", valAddr, "blockNumber", blockNumber)
				delete(*valDetailsMap, valAddr)
			}
		}

		log.Debug("ValidateBlockConsensusDataInner before getBlockProposer", "len(filteredValidatorDepositMap)",
			len(filteredValidatorDepositMap), "len(valDetailsMap)", len(*valDetailsMap), "blockNumber", blockNumber, "consensusContext", consensusContext)
	}

	roundBlockValidators := make(map[byte]common.Address)
	for r := byte(1); r <= blockConsensusData.Round; r++ {
		roundBlockValidators[r], err = getBlockProposer(parentHash, &filteredValidatorDepositMap, r, valDetailsMap, blockNumber, consensusContext)
		if err != nil {
			return err
		}
		log.Debug("roundBlockValidators[r]", "r", r, "roundBlockValidators[r]", roundBlockValidators[r])
	}

	if blockAdditionalConsensusData.ConsensusPackets == nil {
		return errors.New("nil ConsensusPackets")
	}

	packetRoundMap, err := ParseConsensusPackets(parentHash, &blockAdditionalConsensusData.ConsensusPackets, filteredValidatorDepositMap, blockNumber, valDetailsMap, consensusContext)
	if err != nil {
		return err
	}

	if blockConsensusData.VoteType == VOTE_TYPE_NIL {
		if len(txns) > 0 {
			return errors.New("txns in a NIL block")
		}
		if blockConsensusData.SelectedTransactions != nil && len(blockConsensusData.SelectedTransactions) > 0 {
			return errors.New("SelectedTransactions in a NIL vote")
		}
		if blockConsensusData.BlockProposer.IsEqualTo(ZERO_ADDRESS) == false {
			return errors.New("ValidateBlockConsensusData BlockProposer false")
		}

		if blockConsensusData.ProposalHash.IsEqualTo(getNilVoteProposalHash(parentHash, blockConsensusData.Round)) == false {
			return errors.New("proposal hash check failed")
		}

		precommitHash := getNilVotePreCommitHash(parentHash, blockConsensusData.Round)
		if blockConsensusData.PrecommitHash.IsEqualTo(precommitHash) == false {
			return errors.New("precommitHash hash check failed")
		}

		for r := byte(1); r <= blockConsensusData.Round; r++ {
			if r < MAX_ROUND {
				_, ok := nilVotedProposers[roundBlockValidators[r]]
				if ok == false {
					log.Warn("NilVotesProposer doesn't match expected", "roundBlockValidators[r]", roundBlockValidators[r], "r", r, "parentHash", parentHash, "expected proposer", slashedBlockProposer)
					if ContinueOnProposerCheckError == false {
						return errors.New("nilVotedProposers 1")
					}
				}
			}

			_, ok := packetRoundMap[r]
			if ok == false {
				log.Debug("could not find packetMap for round", "r", r)
				return errors.New("could not find packetMap for round")
			}
		}

		if len(nilVotedProposers) > int(blockConsensusData.Round) {
			return errors.New("unexpected number of nilVotedProposers")
		}

		packetMap := packetRoundMap[blockConsensusData.Round]
		err = ValidatePackets(parentHash, blockConsensusData.Round, packetMap, VOTE_TYPE_NIL, &filteredValidatorDepositMap, totalBlockDepositValue, minDepositRequired, blockConsensusData.SelectedTransactions, blockNumber, blockConsensusData.BlockTime)
		if err != nil {
			return err
		}

		//todo: deep validate block proposers
	} else if blockConsensusData.VoteType == VOTE_TYPE_OK {
		if blockConsensusData.BlockProposer.IsEqualTo(ZERO_ADDRESS) {
			return errors.New("ValidateBlockConsensusData BlockProposer true")
		}

		if blockConsensusData.BlockProposer.IsEqualTo(roundBlockValidators[blockConsensusData.Round]) == false {
			log.Warn("ValidateBlockConsensusData BlockProposer mismatch", "blockConsensusData.BlockProposer", blockConsensusData.BlockProposer,
				"roundBlockValidators[blockConsensusData.Round]", roundBlockValidators[blockConsensusData.Round])
			if ContinueOnProposerCheckError == false {
				return errors.New("ValidateBlockConsensusData BlockProposer true")
			}
		}

		if blockConsensusData.ProposalHash.IsEqualTo(ZERO_HASH) {
			log.Debug("ValidateBlockConsensusData ProposalHash zero_hash")
			return errors.New("ValidateBlockConsensusData ProposalHash zero_hash")
		}

		if blockConsensusData.SelectedTransactions == nil {
			if len(txns) > 0 {
				return errors.New("ValidateBlockConsensusData txns is non-empty but SelectedTransactions is nil")
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
						log.Debug("ValidateBlockConsensusData txn", "txn", txn)
						return errors.New("ValidateBlockConsensusData txns should be a subset of blockConsensusData.SelectedTransactions")
					}
				}
			}
		}

		if blockConsensusData.Round > 1 {
			for r := byte(1); r < blockConsensusData.Round; r++ {
				_, ok := nilVotedProposers[roundBlockValidators[r]]
				if ok == false {
					log.Warn("NilVotesProposer 2", "roundBlockValidators[r]", roundBlockValidators[r], "r", r, "parentHash", parentHash)
					if ContinueOnProposerCheckError == false {
						return errors.New("nilVotedProposers 2")
					}
				}
			}
			if len(blockConsensusData.SlashedBlockProposers) < int(blockConsensusData.Round-1) {
				log.Debug("SlashedBlockProposers", "len(nilVotedProposers)", len(nilVotedProposers), "int(blockConsensusData.Round)", int(blockConsensusData.Round))
				return errors.New("ValidateBlockConsensusData SlashedBlockProposers length")
			}
		}

		packetMap := packetRoundMap[blockConsensusData.Round]
		err = ValidatePackets(parentHash, blockConsensusData.Round, packetMap, VOTE_TYPE_OK, &filteredValidatorDepositMap, totalBlockDepositValue, minDepositRequired, blockConsensusData.SelectedTransactions, blockNumber, blockConsensusData.BlockTime)
		if err != nil {
			return err
		}

	} else {
		log.Debug("ValidateBlockConsensusData unexpected vote type", "vote type", blockConsensusData.VoteType)
		return errors.New("ValidateBlockConsensusData unexpected vote type")
	}

	return nil
}

// In this function, absolute time cannot be validated, since this function can get called at a different time, for example when new node is created and is reading old blocks
// Hence only basic checks are allowed
func ValidateBlockProposalTime(blockNumber uint64, proposedTime uint64) bool {
	if blockNumber == 1 || blockNumber%BLOCK_PERIOD_TIME_CHANGE == 0 || blockNumber >= defaults.DefaultConfig.PosConfig.BLOCK_TIME_ORIG_START_BLOCK {
		if proposedTime == 0 {
			return true
		}

		tm := time.Unix(int64(proposedTime), 0)
		if tm.Second() != 0 || tm.Nanosecond() != 0 { //No granularity at anything other than minute level allowed, to reduce ability to manipulate blockHash
			log.Warn("ValidateBlockProposalTime granularity issue", "second", tm.Second(), "nanosecond", tm.Nanosecond())
			return false
		}
	} else {
		if proposedTime != 0 {
			log.Warn("ValidateBlockProposalTime granularity issue", "proposedTime", proposedTime)
			return false
		}
	}

	return true
}

func ValidateBlockConsensusData(block *types.Block, validatorDepositMap *map[common.Address]*big.Int,
	valDetailsMap *map[common.Address]*ValidatorDetailsV2, getBlockConsensusContext GetBlockConsensusContextFn, getValidatorsFn GetValidatorsFn) error {
	header := block.Header()

	if header.ConsensusData == nil || header.UnhashedConsensusData == nil {
		return errors.New("ValidateBlockConsensusData nil")
	}

	blockConsensusData := &BlockConsensusData{}
	err := rlp.DecodeBytes(header.ConsensusData, blockConsensusData)
	if err != nil {
		return err
	}

	if blockConsensusData.SlashedBlockProposers == nil || blockConsensusData.SelectedTransactions == nil {
		return errors.New("ValidateBlockConsensusData SlashedBlockProposers or SelectedTransactions is nil")
	}

	blockAdditionalConsensusData := &BlockAdditionalConsensusData{}
	err = rlp.DecodeBytes(header.UnhashedConsensusData, blockAdditionalConsensusData)
	if err != nil {
		return err
	}

	if blockAdditionalConsensusData.ConsensusPackets == nil {
		return errors.New("ValidateBlockConsensusData ConsensusPackets is nil")
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

	if ValidateBlockProposalTime(block.Number().Uint64(), blockConsensusData.BlockTime) == false {
		log.Warn("ValidateBlockProposalTime failed", "blockNumber", block.Number().Uint64(), "proposedTime", blockConsensusData.BlockTime)
		return errors.New("ValidateBlockProposalTime failed")
	}

	//Consensus Context
	var consensusContext common.Hash
	blockNumber := header.Number.Uint64()
	if blockNumber >= defaults.DefaultConfig.PosConfig.CONTEXT_BASED_START_BLOCK {
		validators, err := getValidatorsFn(header.ParentHash)
		if err != nil {
			return err
		}

		preFilterValidatorCount := len(validators)

		contextKey, err := GetBlockConsensusContextKeyForBlock(blockNumber)
		if err != nil {
			return err
		}
		blockContext, err := getBlockConsensusContext(contextKey, header.ParentHash)
		if err != nil {
			return err
		}
		consensusContext = crypto.Keccak256Hash(blockContext[:], []byte(strconv.Itoa(preFilterValidatorCount)))
		log.Debug("consensusContext", "blockContext", blockContext, "post consensusContext", consensusContext,
			"preFilterValidatorCount", preFilterValidatorCount, "block", header.Number.Uint64())
	}

	return ValidateBlockConsensusDataInner(txnList, header.ParentHash, blockConsensusData, blockAdditionalConsensusData, validatorDepositMap, header.Number.Uint64(), valDetailsMap, consensusContext)
}
