--------------------------- MODULE QuantumCoinConsensus ---------------------------
(*
 * TLA+ specification of the QuantumCoin proof-of-stake consensus protocol.
 *
 * Models the 4-phase BFT consensus (PROPOSAL, ACK_PROPOSAL, PRECOMMIT, COMMIT)
 * with deposit-weighted voting, 2-round escalation, and Byzantine faults.
 *
 * See consensus/proofofstake/README.md for the protocol steps this spec models.
 *)

EXTENDS Integers, FiniteSets, Sequences, TLC

CONSTANTS
    Validators,         \* Set of all validator identifiers
    Deposits,           \* Function: Validator -> Nat (staked deposit per validator)
    Proposer,           \* Function: Round -> Validator (proposer for each round)
    MaxRound,           \* Maximum number of rounds (fixed at 2)
    Byzantine           \* Subset of Validators that may act arbitrarily

ASSUME MaxRound = 2
ASSUME Byzantine \subseteq Validators

Honest == Validators \ Byzantine

Rounds == 1..MaxRound

VoteTypes == {"OK", "NIL"}

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
 * An AckVote record: who voted, what type, and for which round.
 * Byzantine validators may send different vote types to different peers,
 * so ackVotes is the set of all votes that have been "observed" globally.
 *)
AckVoteShape == [validator : Validators, voteType : VoteTypes, round : Rounds]
PrecommitVoteShape == [validator : Validators, round : Rounds]
CommitVoteShape == [validator : Validators, round : Rounds]

VARIABLES
    round,              \* Current round number (global, since all honest advance together)
    state,              \* Function: Validator -> States (per-validator state)
    proposed,           \* Function: Round -> BOOLEAN (has proposer sent PROPOSAL this round)
    proposalReceived,   \* Function: Validator -> BOOLEAN (has this validator received the PROPOSAL)
    timedOut,           \* Function: Validator -> BOOLEAN (has this validator's proposal wait timed out)
    ackVotes,           \* Set of AckVote records received so far
    precommitVotes,     \* Set of PrecommitVote records received so far
    commitVotes,        \* Set of CommitVote records received so far
    blockVoteType,      \* Function: Validator -> VoteTypes \cup {"NONE"} (the vote type this validator locked on)
    finalized           \* Function: Validator -> BOOLEAN (has this validator finalized the block)

vars == <<round, state, proposed, proposalReceived, timedOut,
          ackVotes, precommitVotes, commitVotes, blockVoteType, finalized>>

TypeOK ==
    /\ round \in Rounds
    /\ state \in [Validators -> States]
    /\ proposed \in [Rounds -> BOOLEAN]
    /\ proposalReceived \in [Validators -> BOOLEAN]
    /\ timedOut \in [Validators -> BOOLEAN]
    /\ ackVotes \subseteq AckVoteShape
    /\ precommitVotes \subseteq PrecommitVoteShape
    /\ commitVotes \subseteq CommitVoteShape
    /\ blockVoteType \in [Validators -> VoteTypes \cup {"NONE"}]
    /\ finalized \in [Validators -> BOOLEAN]

--------------------------------------------------------------------------
(* Helper: sum of deposits for a set of validators *)
DepositSum[S \in SUBSET Validators] ==
    IF S = {} THEN 0
    ELSE LET v == CHOOSE v \in S : TRUE
         IN Deposits[v] + DepositSum[S \ {v}]

(* Deposit weight of ack votes with a given vote type in a given round *)
AckDepositForType(vt, r) ==
    DepositSum[{vote.validator : vote \in {v \in ackVotes : v.voteType = vt /\ v.round = r}}]

(* Deposit weight of precommit votes in a given round *)
PrecommitDeposit(r) ==
    DepositSum[{vote.validator : vote \in {v \in precommitVotes : v.round = r}}]

(* Deposit weight of commit votes in a given round *)
CommitDeposit(r) ==
    DepositSum[{vote.validator : vote \in {v \in commitVotes : v.round = r}}]

(* Deposit weight of distinct validators that have sent any ack in a given round,
   regardless of vote type. Each validator is counted once even if they equivocated. *)
AckDeposit(r) ==
    DepositSum[{vote.validator : vote \in {v \in ackVotes : v.round = r}}]

(* Deposit weight of validators that have sent any ack in a higher round *)
HigherRoundAckDeposit(r) ==
    IF r >= MaxRound THEN 0
    ELSE DepositSum[{vote.validator : vote \in {v \in ackVotes : v.round > r}}]


--------------------------------------------------------------------------
(* Initial state: Round 1, all validators waiting for proposal *)
Init ==
    /\ round = 1
    /\ state = [v \in Validators |-> "WAITING_FOR_PROPOSAL"]
    /\ proposed = [r \in Rounds |-> FALSE]
    /\ proposalReceived = [v \in Validators |-> FALSE]
    /\ timedOut = [v \in Validators |-> FALSE]
    /\ ackVotes = {}
    /\ precommitVotes = {}
    /\ commitVotes = {}
    /\ blockVoteType = [v \in Validators |-> "NONE"]
    /\ finalized = [v \in Validators |-> FALSE]

--------------------------------------------------------------------------
(* STEP 6.1 + 7: Proposer broadcasts PROPOSAL and implicitly acks it *)
ProposeBlock(v) ==
    /\ v = Proposer[round]
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ proposed[round] = FALSE
    /\ proposed' = [proposed EXCEPT ![round] = TRUE]
    /\ proposalReceived' = [w \in Validators |-> IF w = v THEN TRUE ELSE proposalReceived[w]]
    /\ LET vt == IF round >= MaxRound THEN "NIL" ELSE "OK"
       IN /\ ackVotes' = ackVotes \cup {[validator |-> v, voteType |-> vt, round |-> round]}
          /\ blockVoteType' = [blockVoteType EXCEPT ![v] = vt]
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ UNCHANGED <<round, timedOut, precommitVotes, commitVotes, finalized>>

(* Byzantine proposer: may selectively deliver proposals and equivocate votes *)
ByzantinePropose(v) ==
    /\ v = Proposer[round]
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ proposed[round] = FALSE
    /\ proposed' = [proposed EXCEPT ![round] = TRUE]
    /\ \E subset \in SUBSET Validators :
        proposalReceived' = [w \in Validators |->
            IF w \in subset THEN TRUE ELSE proposalReceived[w]]
    /\ \E S \in SUBSET VoteTypes :
        /\ S /= {}
        /\ ackVotes' = ackVotes \cup {[validator |-> v, voteType |-> vt, round |-> round] : vt \in S}
        /\ blockVoteType' = [blockVoteType EXCEPT ![v] = "NIL"]
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ UNCHANGED <<round, timedOut, precommitVotes, commitVotes, finalized>>

(* Non-proposer receives the proposal *)
ReceiveProposal(v) ==
    /\ v /= Proposer[round]
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ proposed[round] = TRUE
    /\ proposalReceived[v] = FALSE
    /\ proposalReceived' = [proposalReceived EXCEPT ![v] = TRUE]
    /\ UNCHANGED <<round, state, proposed, timedOut, ackVotes, precommitVotes, commitVotes, blockVoteType, finalized>>

(* STEP 7.2: Proposal timeout -- validator has not received proposal *)
ProposalTimeout(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ v /= Proposer[round]
    /\ proposalReceived[v] = FALSE
    /\ timedOut' = [timedOut EXCEPT ![v] = TRUE]
    /\ UNCHANGED <<round, state, proposed, proposalReceived, ackVotes, precommitVotes, commitVotes, blockVoteType, finalized>>

--------------------------------------------------------------------------
(* STEP 7: Send ACK_PROPOSAL vote *)

(* Honest validator sends ACK_PROPOSAL after receiving proposal or timing out *)
SendAckProposal(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ \/ proposalReceived[v] = TRUE
       \/ timedOut[v] = TRUE
    /\ LET vt == IF timedOut[v] THEN "NIL"
                 ELSE IF round >= MaxRound THEN "NIL"
                 ELSE "OK"
       IN /\ ackVotes' = ackVotes \cup {[validator |-> v, voteType |-> vt, round |-> round]}
          /\ blockVoteType' = [blockVoteType EXCEPT ![v] = vt]
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, precommitVotes, commitVotes, finalized>>

(* Byzantine validator sends arbitrary ACK_PROPOSAL votes, including equivocation:
   may send OK to some peers and NIL to others, so both appear globally. *)
ByzantineSendAck(v) ==
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_PROPOSAL"
    /\ \E S \in SUBSET VoteTypes :
        /\ S /= {}
        /\ ackVotes' = ackVotes \cup {[validator |-> v, voteType |-> vt, round |-> round] : vt \in S}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_ACK_PROPOSAL"]
    /\ blockVoteType' = [blockVoteType EXCEPT ![v] = "NIL"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, precommitVotes, commitVotes, finalized>>

--------------------------------------------------------------------------
(* STEP 9: Evaluate ACK_PROPOSAL threshold and send PRECOMMIT *)

SendPrecommit(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_ACK_PROPOSAL"
    /\ \/ AckDepositForType("OK", round) >= Threshold
       \/ AckDepositForType("NIL", round) >= Threshold
    /\ LET vt == IF AckDepositForType("OK", round) >= Threshold
                 THEN "OK"
                 ELSE "NIL"
       IN blockVoteType' = [blockVoteType EXCEPT ![v] = vt]
    /\ precommitVotes' = precommitVotes \cup {[validator |-> v, round |-> round]}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_PRECOMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, commitVotes, finalized>>

(* Byzantine validator sends precommit arbitrarily *)
ByzantineSendPrecommit(v) ==
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_ACK_PROPOSAL"
    /\ precommitVotes' = precommitVotes \cup {[validator |-> v, round |-> round]}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_PRECOMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, commitVotes, blockVoteType, finalized>>

--------------------------------------------------------------------------
(* STEP 11: Evaluate PRECOMMIT threshold and send COMMIT *)

SendCommit(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_PRECOMMIT"
    /\ PrecommitDeposit(round) >= Threshold
    /\ commitVotes' = commitVotes \cup {[validator |-> v, round |-> round]}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_COMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, precommitVotes, blockVoteType, finalized>>

(* Byzantine validator sends commit arbitrarily *)
ByzantineSendCommit(v) ==
    /\ v \in Byzantine
    /\ state[v] = "WAITING_FOR_PRECOMMIT"
    /\ commitVotes' = commitVotes \cup {[validator |-> v, round |-> round]}
    /\ state' = [state EXCEPT ![v] = "WAITING_FOR_COMMIT"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, precommitVotes, blockVoteType, finalized>>

--------------------------------------------------------------------------
(* STEP 13: Evaluate COMMIT threshold -- finalize block *)

FinalizeBlock(v) ==
    /\ v \in Honest
    /\ state[v] = "WAITING_FOR_COMMIT"
    /\ CommitDeposit(round) >= Threshold
    /\ finalized' = [finalized EXCEPT ![v] = TRUE]
    /\ state' = [state EXCEPT ![v] = "RECEIVED_COMMITS"]
    /\ UNCHANGED <<round, proposed, proposalReceived, timedOut, ackVotes, precommitVotes, commitVotes, blockVoteType>>

--------------------------------------------------------------------------
(* STEP 14: Round escalation -- only from Round 1 *)

(* An honest validator stuck in ACK_PROPOSAL phase triggers escalation.
   In the Go code, escalation from ACK can be triggered by timeout alone
   or by evidence that higher-round participants make the current round
   unreachable. AckDeposit counts each validator once even if they equivocated. *)
EscalateFromAck(v) ==
    /\ v \in Honest
    /\ round = 1
    /\ state[v] = "WAITING_FOR_ACK_PROPOSAL"
    /\ AckDepositForType("OK", round) < Threshold
    /\ AckDepositForType("NIL", round) < Threshold
    /\ \/ HigherRoundAckDeposit(round) + AckDeposit(round) >= Threshold
       \/ TRUE  \* nondeterministic timeout
    /\ round' = 2
    /\ state' = [w \in Validators |->
        IF w \in Honest /\ state[w] \in {"WAITING_FOR_PROPOSAL", "WAITING_FOR_ACK_PROPOSAL"}
        THEN "WAITING_FOR_PROPOSAL"
        ELSE state[w]]
    /\ proposed' = [proposed EXCEPT ![2] = FALSE]
    /\ proposalReceived' = [w \in Validators |-> FALSE]
    /\ timedOut' = [w \in Validators |-> FALSE]
    /\ UNCHANGED <<ackVotes, precommitVotes, commitVotes, blockVoteType, finalized>>

(* An honest validator stuck in PRECOMMIT phase triggers escalation.
   In the Go code, escalation from PRECOMMIT requires timeout AND
   higher-round deposit evidence. The TLA+ model uses a global round
   and does not model out-of-order cross-round messages, so the
   higher-round evidence condition cannot be expressed directly.
   Instead, escalation is guarded only by the precommit deficit,
   which is a sound over-approximation: if safety holds with more
   liberal escalation, it holds with the stricter Go implementation. *)
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
    /\ proposalReceived' = [w \in Validators |-> FALSE]
    /\ timedOut' = [w \in Validators |-> FALSE]
    /\ UNCHANGED <<ackVotes, precommitVotes, commitVotes, blockVoteType, finalized>>

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

(* Agreement: No two honest validators finalize with different vote types *)
Agreement ==
    \A v1, v2 \in Honest :
        (finalized[v1] /\ finalized[v2]) =>
            blockVoteType[v1] = blockVoteType[v2]

(* Validity: If all honest validators voted OK, the block is not NIL *)
Validity ==
    (\A v \in Honest : finalized[v] /\ blockVoteType[v] = "OK") =>
        ~(\E w \in Honest : finalized[w] /\ blockVoteType[w] = "NIL")

(* Round 2 Consistency: Any honest validator finalizing in Round 2 has vote type NIL *)
Round2Consistency ==
    \A v \in Honest :
        (finalized[v] /\ round = 2) => blockVoteType[v] = "NIL"

(* No honest validator finalizes without sufficient commit weight *)
CommitIntegrity ==
    \A v \in Honest :
        finalized[v] => CommitDeposit(round) >= Threshold

--------------------------------------------------------------------------
(* LIVENESS PROPERTIES *)

(* Termination: Eventually all honest validators finalize *)
Termination == <>(\A v \in Honest : finalized[v])

(* Round Progress: If round 1 is stuck with no threshold reached, round 2 is eventually entered *)
RoundProgress ==
    (round = 1
     /\ \A v \in Honest : state[v] \in {"WAITING_FOR_ACK_PROPOSAL", "WAITING_FOR_PRECOMMIT"}
     /\ AckDepositForType("OK", 1) < Threshold
     /\ AckDepositForType("NIL", 1) < Threshold)
    ~> (round = 2)

=============================================================================
