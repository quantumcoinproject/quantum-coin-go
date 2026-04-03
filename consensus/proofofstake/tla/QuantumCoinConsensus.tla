--------------------------- MODULE QuantumCoinConsensus ---------------------------
(*
 * TLA+ specification of the QuantumCoin proof-of-stake consensus protocol.
 *
 * Models the 4-phase BFT consensus (PROPOSAL, ACK_PROPOSAL, PRECOMMIT, COMMIT)
 * with deposit-weighted voting, 2-round escalation, and Byzantine faults
 * including proposer equivocation (sending different proposals to different
 * validators).
 *
 * Proposal identity: Each proposal carries an abstract identifier from the
 * finite set ProposalIds. A Byzantine proposer may send different proposal
 * identities to different validators (equivocating proposer). Honest
 * validators bind their votes to the specific proposal identity they
 * received, and quorum formation requires matching on both vote type and
 * proposal identity. This models the hash-binding property of the real
 * protocol where ACK, PRECOMMIT, and COMMIT messages are chained to a
 * specific proposal hash.
 *
 * See consensus/proofofstake/README.md for the protocol steps this spec models.
 *)

EXTENDS Integers, FiniteSets, Sequences, TLC

CONSTANTS
    Validators,         \* Set of all validator identifiers
    Deposits,           \* Function: Validator -> Nat (staked deposit per validator)
    Proposer,           \* Function: Round -> Validator (proposer for each round)
    MaxRound,           \* Maximum number of rounds (fixed at 2)
    Byzantine,          \* Subset of Validators that may act arbitrarily
    ProposalIds         \* Finite set of abstract proposal identifiers (e.g. {"pA", "pB"})

ASSUME MaxRound = 2
ASSUME Byzantine \subseteq Validators
ASSUME ProposalIds /= {}

Honest == Validators \ Byzantine

Rounds == 1..MaxRound

VoteTypes == {"OK", "NIL"}

\* NilProposal is the deterministic nil-proposal identity used for NIL votes.
\* It must not be a member of ProposalIds (which represent real proposals).
NilProposal == "NIL_PROPOSAL"

\* The full set of proposal identifiers that can appear in votes.
AllProposalIds == ProposalIds \cup {NilProposal}

TotalDeposit == LET S == {Deposits[v] : v \in Validators}
                IN LET Sum[ss \in SUBSET Validators] ==
                       IF ss = {} THEN 0
                       ELSE LET x == CHOOSE v \in ss : TRUE
                            IN Deposits[x] + Sum[ss \ {x}]
                   IN Sum[Validators]

\* 67% threshold: smallest integer >= 67/100 * TotalDeposit
Threshold == ((TotalDeposit * 67) + 99) \div 100

\* Total deposit held by Byzantine validators
ByzantineDeposit == LET Sum[ss \in SUBSET Byzantine] ==
                        IF ss = {} THEN 0
                        ELSE LET x == CHOOSE v \in ss : TRUE
                             IN Deposits[x] + Sum[ss \ {x}]
                    IN Sum[Byzantine]

States == {
    "WAITING_FOR_PROPOSAL",
    "WAITING_FOR_ACK_PROPOSAL",
    "WAITING_FOR_PRECOMMIT",
    "WAITING_FOR_COMMIT",
    "RECEIVED_COMMITS"
}

(*
 * Vote records now carry a proposalId field, modeling the hash-binding
 * property of the real protocol. An honest validator's ACK, PRECOMMIT,
 * and COMMIT are all bound to the same (voteType, proposalId) pair.
 * Byzantine validators may send votes with arbitrary proposalId values.
 *)
AckVoteShape == [validator : Validators, voteType : VoteTypes, proposalId : AllProposalIds, round : Rounds]
PrecommitVoteShape == [validator : Validators, voteType : VoteTypes, proposalId : AllProposalIds, round : Rounds]
CommitVoteShape == [validator : Validators, voteType : VoteTypes, proposalId : AllProposalIds, round : Rounds]

VARIABLES
    round,              \* Current round number (global, since all honest advance together)
    state,              \* Function: Validator -> States (per-validator state)
    proposed,           \* Function: Round -> BOOLEAN (has proposer sent PROPOSAL this round)
    proposalReceived,   \* Function: Validator -> AllProposalIds \cup {"NONE"} (which proposal this validator received)
    timedOut,           \* Function: Validator -> BOOLEAN (has this validator's proposal wait timed out)
    ackVotes,           \* Set of AckVote records received so far
    precommitVotes,     \* Set of PrecommitVote records received so far
    commitVotes,        \* Set of CommitVote records received so far
    blockVoteType,      \* Function: Validator -> VoteTypes \cup {"NONE"} (the vote type this validator locked on)
    blockProposalId,    \* Function: Validator -> AllProposalIds \cup {"NONE"} (the proposal id this validator locked on)
    finalized           \* Function: Validator -> BOOLEAN (has this validator finalized the block)

vars == <<round, state, proposed, proposalReceived, timedOut,
          ackVotes, precommitVotes, commitVotes, blockVoteType, blockProposalId, finalized>>

TypeOK ==
    /\ round \in Rounds
    /\ state \in [Validators -> States]
    /\ proposed \in [Rounds -> BOOLEAN]
    /\ proposalReceived \in [Validators -> AllProposalIds \cup {"NONE"}]
    /\ timedOut \in [Validators -> BOOLEAN]
    /\ ackVotes \subseteq AckVoteShape
    /\ precommitVotes \subseteq PrecommitVoteShape
    /\ commitVotes \subseteq CommitVoteShape
    /\ blockVoteType \in [Validators -> VoteTypes \cup {"NONE"}]
    /\ blockProposalId \in [Validators -> AllProposalIds \cup {"NONE"}]
    /\ finalized \in [Validators -> BOOLEAN]

--------------------------------------------------------------------------
(* Helper: sum of deposits for a set of validators *)
DepositSum[S \in SUBSET Validators] ==
    IF S = {} THEN 0
    ELSE LET v == CHOOSE v \in S : TRUE
         IN Deposits[v] + DepositSum[S \ {v}]

(* Deposit weight of ack votes with a given vote type AND proposal id in a given round *)
AckDepositForTypeAndProposal(vt, pid, r) ==
    DepositSum[{vote.validator : vote \in {v \in ackVotes : v.voteType = vt /\ v.proposalId = pid /\ v.round = r}}]

(* Deposit weight of precommit votes with a given vote type and proposal id in a given round *)
PrecommitDepositForTypeAndProposal(vt, pid, r) ==
    DepositSum[{vote.validator : vote \in {v \in precommitVotes : v.voteType = vt /\ v.proposalId = pid /\ v.round = r}}]

(* Deposit weight of commit votes with a given vote type and proposal id in a given round *)
CommitDepositForTypeAndProposal(vt, pid, r) ==
    DepositSum[{vote.validator : vote \in {v \in commitVotes : v.voteType = vt /\ v.proposalId = pid /\ v.round = r}}]

(* Deposit weight of distinct validators that have sent any ack in a given round,
   regardless of vote type or proposal id. Each validator is counted once. *)
AckDeposit(r) ==
    DepositSum[{vote.validator : vote \in {v \in ackVotes : v.round = r}}]

(* Whether the ack threshold has been reached for ANY (voteType, proposalId) pair *)
AckThresholdReached(r) ==
    \E vt \in VoteTypes : \E pid \in AllProposalIds :
        AckDepositForTypeAndProposal(vt, pid, r) >= Threshold

(* The (voteType, proposalId) pair that reached ack threshold, if any.
   Used by honest validators to determine their precommit value. *)
AckThresholdVoteType(r) ==
    CHOOSE vt \in VoteTypes : \E pid \in AllProposalIds :
        AckDepositForTypeAndProposal(vt, pid, r) >= Threshold

AckThresholdProposalId(r) ==
    LET vt == AckThresholdVoteType(r)
    IN CHOOSE pid \in AllProposalIds :
        AckDepositForTypeAndProposal(vt, pid, r) >= Threshold

(* Whether the precommit threshold has been reached for ANY (voteType, proposalId) pair *)
PrecommitThresholdReached(r) ==
    \E vt \in VoteTypes : \E pid \in AllProposalIds :
        PrecommitDepositForTypeAndProposal(vt, pid, r) >= Threshold

(* Whether the commit threshold has been reached for ANY (voteType, proposalId) pair *)
CommitThresholdReached(r) ==
    \E vt \in VoteTypes : \E pid \in AllProposalIds :
        CommitDepositForTypeAndProposal(vt, pid, r) >= Threshold

(* Deposit weight of validators that have sent any ack in a higher round *)
HigherRoundAckDeposit(r) ==
    IF r >= MaxRound THEN 0
    ELSE DepositSum[{vote.validator : vote \in {v \in ackVotes : v.round > r}}]

(* Whether the ack threshold has been reached for any single vote type
   (across all proposal ids) -- used for escalation guards *)
AckDepositForType(vt, r) ==
    DepositSum[{vote.validator : vote \in {v \in ackVotes : v.voteType = vt /\ v.round = r}}]

(* Total precommit deposit regardless of type/proposal -- used for escalation guard *)
PrecommitDeposit(r) ==
    DepositSum[{vote.validator : vote \in {v \in precommitVotes : v.round = r}}]


--------------------------------------------------------------------------
(* Initial state: Round 1, all validators waiting for proposal *)
Init ==
    /\ round = 1
    /\ state = [v \in Validators |-> "WAITING_FOR_PROPOSAL"]
    /\ proposed = [r \in Rounds |-> FALSE]
    /\ proposalReceived = [v \in Validators |-> "NONE"]
    /\ timedOut = [v \in Validators |-> FALSE]
    /\ ackVotes = {}
    /\ precommitVotes = {}
    /\ commitVotes = {}
    /\ blockVoteType = [v \in Validators |-> "NONE"]
    /\ blockProposalId = [v \in Validators |-> "NONE"]
    /\ finalized = [v \in Validators |-> FALSE]

--------------------------------------------------------------------------
(* STEP 6.1 + 7: Honest proposer broadcasts PROPOSAL and implicitly acks it.
   An honest proposer sends the same proposal to all validators. *)
ProposeBlock(v) ==
    /\ v = Proposer[round]
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ proposed[round] = FALSE
    /\ \E pid \in ProposalIds :
        /\ proposed' = [proposed EXCEPT ![round] = TRUE]
        /\ proposalReceived' = [w \in Validators |-> IF w = v THEN pid ELSE proposalReceived[w]]
        /\ LET vt == IF round >= MaxRound THEN "NIL" ELSE "OK"
               ackPid == IF round >= MaxRound THEN NilProposal ELSE pid
           IN /\ ackVotes' = ackVotes \cup {[validator |-> v, voteType |-> vt, proposalId |-> ackPid, round |-> round]}
              /\ blockVoteType' = [blockVoteType EXCEPT ![v] = vt]
              /\ blockProposalId' = [blockProposalId EXCEPT ![v] = ackPid]
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ UNCHANGED <<round, timedOut, precommitVotes, commitVotes, finalized>>

(* Byzantine proposer: may send DIFFERENT proposals to different validators
   (equivocating proposer) and may equivocate its own votes.
   Each honest validator independently receives one of the proposal ids or
   nothing; Byzantine validators always receive nothing (they vote via
   ByzantineSendAck regardless). The proposer emits a single ack vote
   here; additional equivocating acks are emitted via ByzantineSendAck. *)
ByzantinePropose(v) ==
    /\ v = Proposer[round]
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ proposed[round] = FALSE
    /\ proposed' = [proposed EXCEPT ![round] = TRUE]
    /\ \E deliveryFn \in [Honest -> ProposalIds \cup {"NONE"}] :
        proposalReceived' = [w \in Validators |->
            IF w \in Honest /\ deliveryFn[w] /= "NONE" THEN deliveryFn[w] ELSE proposalReceived[w]]
    /\ ackVotes' = ackVotes \cup
        {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round] : vt \in VoteTypes, pid \in AllProposalIds}
    /\ blockVoteType' = [blockVoteType EXCEPT ![v] = "NIL"]
    /\ blockProposalId' = [blockProposalId EXCEPT ![v] = NilProposal]
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ UNCHANGED <<round, timedOut, precommitVotes, commitVotes, finalized>>

(* Non-proposer receives the proposal.
   The proposal identity delivered to this validator was set by the proposer action. *)
ReceiveProposal(v) ==
    /\ v /= Proposer[round]
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ proposed[round] = TRUE
    /\ proposalReceived[v] /= "NONE"
    /\ UNCHANGED <<round, state, proposed, proposalReceived, timedOut,
                   ackVotes, precommitVotes, commitVotes, blockVoteType, blockProposalId, finalized>>

(* STEP 7.2: Proposal timeout -- validator has not received proposal *)
ProposalTimeout(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ v /= Proposer[round]
    /\ proposalReceived[v] = "NONE"
    /\ timedOut' = [timedOut EXCEPT ![v] = TRUE]
    /\ UNCHANGED <<round, state, proposed, proposalReceived, ackVotes, precommitVotes, commitVotes, blockVoteType, blockProposalId, finalized>>

--------------------------------------------------------------------------
(* STEP 7: Send ACK_PROPOSAL vote.
   An honest validator binds its ack to the specific proposal it received.
   A validator accepts at most one proposal per round (first valid proposal rule). *)

SendAckProposal(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ \/ proposalReceived[v] /= "NONE"
       \/ timedOut[v] = TRUE
    /\ LET vt == IF timedOut[v] THEN "NIL"
                 ELSE IF round >= MaxRound THEN "NIL"
                 ELSE "OK"
           pid == IF vt = "NIL" THEN NilProposal ELSE proposalReceived[v]
       IN /\ ackVotes' = ackVotes \cup {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round]}
          /\ blockVoteType' = [blockVoteType EXCEPT ![v] = vt]
          /\ blockProposalId' = [blockProposalId EXCEPT ![v] = pid]
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, precommitVotes, commitVotes, finalized>>

(* Byzantine validator sends arbitrary ACK_PROPOSAL votes. Emits votes for
   all (voteType, proposalId) combinations at once, modeling the worst case
   where the Byzantine validator equivocates across both vote types and
   both proposal identities simultaneously. *)
ByzantineSendAck(v) ==
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ ackVotes' = ackVotes \cup
        {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round] : vt \in VoteTypes, pid \in AllProposalIds}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ blockVoteType' = [blockVoteType EXCEPT ![v] = "NIL"]
    /\ blockProposalId' = [blockProposalId EXCEPT ![v] = NilProposal]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, precommitVotes, commitVotes, finalized>>

--------------------------------------------------------------------------
(* STEP 9: Evaluate ACK_PROPOSAL threshold and send PRECOMMIT.
   An honest validator's precommit is bound to the (voteType, proposalId)
   pair that reached the ack threshold. *)

SendPrecommit(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_ACK_PROPOSAL"
    /\ AckThresholdReached(round)
    /\ LET vt == AckThresholdVoteType(round)
           pid == AckThresholdProposalId(round)
       IN /\ blockVoteType' = [blockVoteType EXCEPT ![v] = vt]
          /\ blockProposalId' = [blockProposalId EXCEPT ![v] = pid]
          /\ precommitVotes' = precommitVotes \cup {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round]}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_PRECOMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, commitVotes, finalized>>

(* Byzantine validator sends precommit votes for all (voteType, proposalId)
   combinations. Sound over-approximation: worst-case equivocation. *)
ByzantineSendPrecommit(v) ==
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_ACK_PROPOSAL"
    /\ precommitVotes' = precommitVotes \cup
        {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round] : vt \in VoteTypes, pid \in AllProposalIds}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_PRECOMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, commitVotes, blockVoteType, blockProposalId, finalized>>

--------------------------------------------------------------------------
(* STEP 11: Evaluate PRECOMMIT threshold and send COMMIT.
   An honest validator's commit is bound to the (voteType, proposalId)
   pair that reached the precommit threshold. *)

SendCommit(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PRECOMMIT"
    /\ PrecommitThresholdReached(round)
    /\ LET vt == blockVoteType[v]
           pid == blockProposalId[v]
       IN commitVotes' = commitVotes \cup {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round]}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_COMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, precommitVotes, blockVoteType, blockProposalId, finalized>>

(* Byzantine validator sends commit votes for all (voteType, proposalId)
   combinations. Sound over-approximation: worst-case equivocation. *)
ByzantineSendCommit(v) ==
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_PRECOMMIT"
    /\ commitVotes' = commitVotes \cup
        {[validator |-> v, voteType |-> vt, proposalId |-> pid, round |-> round] : vt \in VoteTypes, pid \in AllProposalIds}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_COMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, precommitVotes, blockVoteType, blockProposalId, finalized>>

--------------------------------------------------------------------------
(* STEP 13: Evaluate COMMIT threshold -- finalize block *)

FinalizeBlock(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_COMMIT"
    /\ CommitThresholdReached(round)
    /\ finalized' = [finalized EXCEPT ![v] = TRUE]
    /\ state' = [state EXCEPT ![v] = "RECEIVED_COMMITS"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, precommitVotes, commitVotes, blockVoteType, blockProposalId>>

--------------------------------------------------------------------------
(* STEP 14: Round escalation -- only from Round 1 *)

EscalateFromAck(v) ==
    /\ v \in Honest
    /\ round = 1
    /\ state[v] = "WAITING_FOR_ACK_PROPOSAL"
    /\ ~AckThresholdReached(round)
    /\ \/ HigherRoundAckDeposit(round) + AckDeposit(round) >= Threshold
       \/ TRUE  \* nondeterministic timeout
    /\ round' = 2
    /\ state' = [w \in Validators |->
        IF w \in Honest /\ state[w] \in {"WAITING_FOR_PROPOSAL", "WAITING_FOR_ACK_PROPOSAL"}
        THEN "WAITING_FOR_PROPOSAL"
        ELSE state[w]]
    /\ proposed' = [proposed EXCEPT ![2] = FALSE]
    /\ proposalReceived' = [w \in Validators |-> "NONE"]
    /\ timedOut' = [w \in Validators |-> FALSE]
    /\ UNCHANGED <<ackVotes, precommitVotes, commitVotes, blockVoteType, blockProposalId, finalized>>

EscalateFromPrecommit(v) ==
    /\ v \in Honest
    /\ round = 1
    /\ state[v] = "WAITING_FOR_PRECOMMIT"
    /\ PrecommitDeposit(round) < Threshold
    /\ round' = 2
    /\ state' = [w \in Validators |->
        IF w \in Honest /\ state[w] \in {"WAITING_FOR_PROPOSAL", "WAITING_FOR_ACK_PROPOSAL", "WAITING_FOR_PRECOMMIT"}
        THEN "WAITING_FOR_PROPOSAL"
        ELSE state[w]]
    /\ proposed' = [proposed EXCEPT ![2] = FALSE]
    /\ proposalReceived' = [w \in Validators |-> "NONE"]
    /\ timedOut' = [w \in Validators |-> FALSE]
    /\ UNCHANGED <<ackVotes, precommitVotes, commitVotes, blockVoteType, blockProposalId, finalized>>

--------------------------------------------------------------------------
(* Next-state relation *)
Next ==
    \/ \E v \in Validators :
        \/ ProposeBlock(v)
        \/ ByzantinePropose(v)
        \/ ReceiveProposal(v)
        \/ ProposalTimeout(v)
        \/ SendAckProposal(v)
        \/ ByzantineSendAck(v)
        \/ SendPrecommit(v)
        \/ ByzantineSendPrecommit(v)
        \/ SendCommit(v)
        \/ ByzantineSendCommit(v)
        \/ FinalizeBlock(v)
        \/ EscalateFromAck(v)
        \/ EscalateFromPrecommit(v)

HonestActions(v) ==
    \/ ProposeBlock(v)
    \/ ReceiveProposal(v)
    \/ ProposalTimeout(v)
    \/ SendAckProposal(v)
    \/ SendPrecommit(v)
    \/ SendCommit(v)
    \/ FinalizeBlock(v)
    \/ EscalateFromAck(v)
    \/ EscalateFromPrecommit(v)

Fairness == \A v \in Honest : WF_vars(HonestActions(v))

Spec == Init /\ [][Next]_vars /\ Fairness

--------------------------------------------------------------------------
(* SAFETY PROPERTIES *)

(* Agreement: No two honest validators finalize with different blocks.
   This covers both OK-vs-NIL conflicts and OK(blockA)-vs-OK(blockB) conflicts
   from an equivocating proposer. Two validators agree iff they locked on the
   same vote type AND the same proposal identity. *)
Agreement ==
    \A v1, v2 \in Honest :
        (finalized[v1] /\ finalized[v2]) =>
            /\ blockVoteType[v1] = blockVoteType[v2]
            /\ blockProposalId[v1] = blockProposalId[v2]

(* Validity: If all honest validators voted OK, no honest validator finalizes as NIL *)
Validity ==
    (\A v \in Honest : finalized[v] /\ blockVoteType[v] = "OK") =>
        ~(\E w \in Honest : finalized[w] /\ blockVoteType[w] = "NIL")

(* Round 2 Consistency: Any honest validator finalizing in Round 2 has vote type NIL *)
Round2Consistency ==
    \A v \in Honest :
        (finalized[v] /\ round = 2) => blockVoteType[v] = "NIL"

(* No honest validator finalizes without sufficient commit weight for its certified value *)
CommitIntegrity ==
    \A v \in Honest :
        finalized[v] => CommitDepositForTypeAndProposal(blockVoteType[v], blockProposalId[v], round) >= Threshold

--------------------------------------------------------------------------
(* LIVENESS PROPERTIES *)

(* Termination: Eventually all honest validators finalize *)
Termination == <>(\A v \in Honest : finalized[v])

(* Round Progress: If round 1 is stuck with no threshold reached, round 2 is eventually entered *)
RoundProgress ==
    (round = 1
     /\ \A v \in Honest : state[v] \in {"WAITING_FOR_ACK_PROPOSAL", "WAITING_FOR_PRECOMMIT"}
     /\ ~AckThresholdReached(1))
    ~> (round = 2)

=============================================================================
