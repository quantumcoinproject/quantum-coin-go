------------------------ MODULE MCQuantumCoinConsensus --------------------------
(*
 * Model-checking instantiation of QuantumCoinConsensus.
 *
 * Provides concrete values for all CONSTANTS so that TLC can enumerate
 * the state space.
 *
 * Three configurations are available, selected by changing which MC_Byzantine
 * and MC_Deposits definitions are active:
 *
 *   Safe (default):  4 validators, 1 Byzantine (25% deposit < 33%)
 *   Boundary:        4 validators, 1 Byzantine (33% deposit, at the boundary)
 *   Unsafe:          4 validators, 2 Byzantine (combined 34% deposit, just above 33%)
 *                    (requires MCQuantumCoinConsensusUnsafe.tla which omits the ASSUME)
 *)

EXTENDS QuantumCoinConsensus

\* Fault tolerance: Byzantine validators control < 1/3 of total deposit.
\* This ASSUME is placed here (not in the main spec) so that the unsafe
\* model-checking module can extend the same spec without violating it.
ASSUME ByzantineDeposit * 3 < TotalDeposit

--------------------------------------------------------------------------
(* Safe configuration: 1 of 4 Byzantine, equal deposits (25% < 33%) *)
MC_Validators == {"v1", "v2", "v3", "v4"}

MC_Deposits == [v \in MC_Validators |->
    CASE v = "v1" -> 25
      [] v = "v2" -> 25
      [] v = "v3" -> 25
      [] v = "v4" -> 25]

MC_Proposer == [r \in 1..2 |->
    CASE r = 1 -> "v1"
      [] r = 2 -> "v2"]

MC_MaxRound == 2

MC_Byzantine == {"v4"}

MC_ProposalIds == {"pA", "pB"}

--------------------------------------------------------------------------
(* Boundary configuration: 1 of 4, deposits at the 33% boundary.
   v4 has 33 out of 100 total = exactly 33%.
   ASSUME satisfied: 33*3 = 99 < 100 (strictly less than 1/3). *)

MC_Deposits_Boundary == [v \in MC_Validators |->
    CASE v = "v1" -> 23
      [] v = "v2" -> 22
      [] v = "v3" -> 22
      [] v = "v4" -> 33]

MC_Byzantine_Boundary == {"v4"}

--------------------------------------------------------------------------

MC_Spec == Spec

=============================================================================
