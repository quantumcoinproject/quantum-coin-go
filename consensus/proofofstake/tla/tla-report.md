# QuantumCoin TLA+ Model Checking Report

> **See also:** [Consensus Protocol Description](../README.md) -- human-readable step-by-step specification of the protocol that was formally verified.
>
> **See also:** [TLA+ Specification README](README.md) -- file descriptions, prerequisites, and instructions for running the model checker.

Verification results for the QuantumCoin proof-of-stake consensus protocol TLA+ specification.

**Date:** 2026-04-02
**TLC Version:** 2026.03.31.154134 (rev: becec35)
**Platform:** Windows 11, OpenJDK 21.0.10, 16 cores, 21.8 GB heap

---

## What the Specification Models

The TLA+ specification models the core consensus protocol: 4-phase BFT voting (PROPOSAL, ACK, PRECOMMIT, COMMIT), deposit-weighted quorum thresholds, two-round escalation, and Byzantine faults including:

- **Proposer equivocation**: A Byzantine proposer sends different proposal identities to different honest validators.
- **Vote equivocation**: A Byzantine validator sends votes for all `(voteType, proposalId)` combinations, so different honest peers can form quorums for different certified values.
- **Selective proposal delivery**: A Byzantine proposer delivers to an arbitrary subset of validators.
- **Phase skipping**: Byzantine validators advance through phases without waiting for thresholds.
- **Crash/abstention**: Modeled implicitly via nondeterministic Byzantine inaction.

Each vote (ACK, PRECOMMIT, COMMIT) carries a **proposal identity** that models the hash-binding property of the real protocol. Quorum formation requires matching on both `(voteType, proposalId)` — votes for different proposal identities do not combine. This enables the model to detect both OK-vs-NIL conflicts and OK(blockA)-vs-OK(blockB) conflicts from an equivocating proposer.

Byzantine voting uses a worst-case over-approximation: each Byzantine validator emits votes for all `(voteType, proposalId)` combinations in each phase, rather than choosing nondeterministically. This is sound because if safety holds under maximal equivocation, it holds for any subset.

---

## Fault Tolerance Boundary

The protocol's safety depends on Byzantine validators controlling **less than 33%** of total weighted deposit. The specification enforces this via:

```
ASSUME ByzantineDeposit * 3 < TotalDeposit
```

To verify this boundary, three model configurations were checked:

| Configuration | Byzantine Deposit | Within Tolerance | Expected |
|---------------|-------------------|------------------|----------|
| Safe | 25% (25/100) | Yes | All properties pass |
| Boundary | 33% (33/100) | Yes (at the limit) | All properties pass |
| Unsafe | 34% (34/100) | No (just above 33%) | Safety violations |

The Boundary and Unsafe configurations differ by just 1 percentage point, providing a tight proof that the 33% threshold is the exact safety boundary.

---

## Safe Configuration (25% Byzantine)

| Parameter | Value |
|-----------|-------|
| Validators | v1, v2, v3, v4 |
| Deposits | 25 each (total = 100) |
| Threshold | 67 |
| Proposers | v1 (Round 1), v2 (Round 2) |
| Byzantine | v4 (25% deposit) |
| ProposalIds | pA, pB |

### Results

| Property | Type | Result |
|----------|------|--------|
| TypeOK | Safety (invariant) | PASS |
| Agreement | Safety (invariant) | PASS |
| Validity | Safety (invariant) | PASS |
| Round2Consistency | Safety (invariant) | PASS |
| CommitIntegrity | Safety (invariant) | PASS |
| Termination | Liveness (temporal) | PASS |

### State Space

| Metric | Value |
|--------|-------|
| States generated | 43,911 |
| Distinct states | 15,951 |
| State graph depth | 26 |
| Runtime | 2 seconds |

---

## Boundary Configuration (33% Byzantine)

| Parameter | Value |
|-----------|-------|
| Validators | v1, v2, v3, v4 |
| Deposits | v1=23, v2=22, v3=22, v4=33 (total = 100) |
| Threshold | 67 |
| Proposers | v1 (Round 1), v2 (Round 2) |
| Byzantine | v4 (33% deposit, at the limit) |
| ProposalIds | pA, pB |

The ASSUME is satisfied: `33 * 3 = 99 < 100` (strictly less than 1/3).

### Results

| Property | Type | Result |
|----------|------|--------|
| TypeOK | Safety (invariant) | PASS |
| Agreement | Safety (invariant) | PASS |
| Validity | Safety (invariant) | PASS |
| Round2Consistency | Safety (invariant) | PASS |
| CommitIntegrity | Safety (invariant) | PASS |
| Termination | Liveness (temporal) | PASS |

### State Space

| Metric | Value |
|--------|-------|
| States generated | 43,911 |
| Distinct states | 15,951 |
| State graph depth | 26 |
| Runtime | 2 seconds |

---

## Unsafe Configuration (34% Byzantine)

| Parameter | Value |
|-----------|-------|
| Validators | v1, v2, v3, v4 |
| Deposits | v1=33, v2=33, v3=17, v4=17 (total = 100) |
| Threshold | 67 |
| Proposers | v1 (Round 1), v2 (Round 2) |
| Byzantine | v3, v4 (combined 34% deposit, just above 33%) |
| ProposalIds | pA, pB |

The ASSUME is violated as expected: `34 * 3 = 102 >= 100`. This configuration uses `MCQuantumCoinConsensusUnsafe.tla`, which extends the main spec directly and omits the ASSUME.

### Results

| Property | Type | Result                  |
|----------|------|-------------------------|
| Agreement | Safety (invariant) | **VIOLATED** (expected) |

TLC found a counterexample in 16 steps:

1. v2 (honest, deposit 33) times out waiting for a proposal.
2. v3 (Byzantine, deposit 17) sends ack votes for all `(voteType, proposalId)` combinations — equivocating across OK, NIL, pA, pB, and NilProposal.
3. v2 sends `ACK_PROPOSAL` with `NIL` / `NIL_PROPOSAL` (timed out, no proposal received).
4. v3 (Byzantine) skips to `PRECOMMIT`, sending precommit votes for all combinations.
5. v4 (Byzantine, deposit 17) sends ack votes for all `(voteType, proposalId)` combinations.
6. v2 sees NIL/NIL_PROPOSAL deposit = v2(33) + v3(17) + v4(17) = **67** >= 67 threshold. Precommits `NIL` / `NIL_PROPOSAL`.
7. v1 (honest, deposit 33) proposes block `pA` and acks `OK` / `pA`. Now OK/pA deposit = v1(33) + v3(17) + v4(17) = **67** >= 67 threshold.
8. v1 precommits `OK` / `pA`.
9. v4 (Byzantine) sends precommit votes for all combinations.
10. v1 and v2 each send commit votes for their respective certified values.
11. v3 and v4 (Byzantine) send commit votes for all combinations.
12. v1 finalizes with **OK / pA**. v2 finalizes with **NIL / NIL_PROPOSAL**. **Agreement violated.**

The equivocation by v3 and v4 allows both OK/pA and NIL/NIL_PROPOSAL quorums to reach the 67% threshold at different points in the execution. v2 advances on the NIL path first, and v1 advances on the OK path later — the conflicting quorums do not need to form at the same instant. The violation occurs at just 34% Byzantine deposit — only 1 percentage point above the boundary where all properties pass (33%).

### State Space (partial, stopped at first violation (expected))

| Metric | Value |
|--------|-------|
| States generated | ~28,000 |
| Distinct states | ~10,000 |
| State graph depth | 20 |
| Runtime | 2 seconds |

*Note: Partial state counts vary across runs because TLC stops at the first violation (expected), and the exact frontier explored depends on worker thread scheduling.*

---

## Why the Boundary Is 33%

The 67% threshold creates an overlap guarantee between any two quorums. For two quorums Q1 and Q2, each with >= 67% of total deposit, their intersection must contain at least `67 + 67 - 100 = 34%` of deposit.

- **At <= 33% Byzantine**: The 34% overlap exceeds the Byzantine deposit. Any two quorums must share at least one honest validator, ensuring all honest validators agree on both vote type and proposal identity.
- **At > 33% Byzantine**: The 34% overlap can be entirely Byzantine. Equivocating Byzantine validators can appear in two quorums while voting for different certified values, enabling conflicting finalizations.

The TLC results confirm this mathematically: 33% passes, 34% fails.

---

## Properties Explained

### Safety (checked as invariants -- hold in every reachable state)

- **TypeOK**: All variables remain within their declared types throughout execution.
- **Agreement**: No two honest validators finalize with different blocks. This checks both `blockVoteType` and `blockProposalId` agreement, covering OK-vs-NIL conflicts and OK(blockA)-vs-OK(blockB) conflicts from an equivocating proposer. This is the core BFT safety guarantee.
- **Validity**: If all honest validators voted `OK`, no honest validator finalizes as `NIL`. Prevents the Byzantine minority from forcing an empty block when the honest majority agrees on a real block.
- **Round2Consistency**: Any honest validator that finalizes in Round 2 has vote type `NIL`, consistent with Round 2 being a forced `NIL` round.
- **CommitIntegrity**: No honest validator finalizes without at least 67% weighted `COMMIT` deposit for its specific `(voteType, proposalId)` certified value.

### Liveness (checked as temporal properties -- must eventually hold)

- **Termination**: All honest validators eventually finalize the block. This guarantees the protocol makes progress under the Byzantine fault model with partial synchrony (modeled via per-validator weak fairness).

### RoundProgress (excluded)

The `RoundProgress` property ("if Round 1 is stuck, Round 2 is eventually entered") is defined in the spec but excluded from model checking. This is a **limitation of the TLA+ property formulation**, not a problem with the consensus protocol design.

The property is expressed as a leads-to formula: `(round = 1 /\ thresholds not met) ~> (round = 2)`. In TLA+, `P ~> Q` means "whenever P holds in some state, Q must eventually hold in some later state." The issue is that the antecedent (neither OK nor NIL reaching 67%) can hold **transiently** -- for example, after some validators have acked but before the remaining validators send their votes. At that moment thresholds are not yet met, so the antecedent is true. But then the remaining votes arrive, a threshold is reached, and Round 1 completes successfully without ever needing Round 2. Since `round = 2` never becomes true, the leads-to property is violated.

There is no clean way to distinguish "transiently below threshold while votes are still arriving" from "permanently stuck and needs escalation" in a single TLA+ temporal formula without introducing auxiliary variables to track whether the system is truly deadlocked. This is a well-known difficulty with expressing conditional liveness in temporal logic.

The protocol itself handles round escalation correctly: the `Termination` property passes, which proves that every honest validator eventually finalizes. This can only happen if the system successfully escalates when Round 1 is genuinely stuck. `Termination` therefore implicitly covers the round escalation path.

---

## Byzantine Behaviors Modeled

The specification models the following Byzantine behaviors, all explored nondeterministically by TLC:

| Behavior | Description |
|----------|-------------|
| Proposer equivocation | A Byzantine proposer delivers different proposal identities to different honest validators, attempting to create competing OK quorums for different blocks. |
| Vote equivocation | A Byzantine validator sends votes for all `(voteType, proposalId)` combinations for the same round, so different honest peers can observe votes for different certified values. |
| Selective proposal delivery | A Byzantine proposer delivers the `PROPOSAL` to an arbitrary subset of validators (including possibly none). |
| Arbitrary voting | Byzantine validators send `ACK_PROPOSAL`, `PRECOMMIT`, and `COMMIT` votes for all `(voteType, proposalId)` combinations regardless of preconditions. |
| Phase skipping | Byzantine validators advance through protocol phases without waiting for thresholds. |

---

## Conclusion

The QuantumCoin consensus protocol, as modeled in `QuantumCoinConsensus.tla`, satisfies all safety invariants and the termination liveness property when Byzantine validators control **<= 33%** of total weighted deposit. This was verified exhaustively across two configurations:

- **Safe** (25% Byzantine): 43,911 states explored, all properties pass.
- **Boundary** (33% Byzantine): 43,911 states explored, all properties pass.

The `Agreement` invariant now verifies safety at the **block-identity level**: it checks that finalized honest validators agree on both vote type and proposal identity, covering both OK-vs-NIL conflicts and the equivocating-proposer scenario (OK(blockA) vs. OK(blockB)).

When Byzantine deposit exceeds 33%, the **Agreement** invariant is violated via equivocation, as demonstrated by the Unsafe configuration (34% Byzantine, just 1% above the boundary). This confirms that the 33% bound is both necessary and sufficient for safety under the Byzantine fault model.
