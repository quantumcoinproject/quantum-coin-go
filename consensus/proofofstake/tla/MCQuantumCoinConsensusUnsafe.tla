--------------------- MODULE MCQuantumCoinConsensusUnsafe ----------------------
(*
 * Unsafe model-checking instantiation of QuantumCoinConsensus.
 *
 * 4 validators, 2 Byzantine with combined 34% deposit (just above 33%).
 * Extends the main spec directly (not MCQuantumCoinConsensus) to avoid
 * the < 33% fault tolerance ASSUME.
 *
 * With 2 Byzantine validators, equivocation can create two conflicting
 * quorums that each exceed 67%, breaking Agreement.
 *
 * Purpose: demonstrate that safety properties FAIL when Byzantine deposit
 * exceeds 33%, proving the boundary is tight.
 *)

EXTENDS QuantumCoinConsensus

MC_Validators == {"v1", "v2", "v3", "v4"}

MC_Deposits == [v \in MC_Validators |->
    CASE v = "v1" -> 33
      [] v = "v2" -> 33
      [] v = "v3" -> 17
      [] v = "v4" -> 17]

MC_Proposer == [r \in 1..2 |->
    CASE r = 1 -> "v1"
      [] r = 2 -> "v2"]

MC_MaxRound == 2

MC_Byzantine == {"v3", "v4"}

MC_ProposalIds == {"pA", "pB"}

MC_Spec == Spec

=============================================================================
