# TLA+ & Consensus Review Findings (Master List)

Consolidated findings from three independent AI reviews of the QuantumCoin consensus protocol documentation, TLA+ specification, and model checking results.

**Sources:**

- **Review A** (`CONSENSUS_REVIEW.md`)
- **Review B** (`ANALYSIS.md`)
- **Review C** (`consensus-review-report.md`)

---

## Summary

All three reviews confirm that:
- The TLA+ verification results are correct: Safe (25%) and Boundary (33%) pass; Unsafe (34%) violates Agreement.
- The TLA+ specification captures the high-level protocol structure faithfully.
- The 33% fault tolerance boundary is correctly demonstrated.

They disagree on severity of some findings and on whether certain abstractions constitute real issues.

---

## Findings

### F1. README step 11.2: "or" should be "and" (precommit escalation condition)

| | |
|---|---|
| **Severity** | Design documentation bug |
| **Reported by** | Review B |
| **Status** | Open |

The README says:

> If round = 1: if the precommit timeout is exceeded, **or** if validators already participating in Round 2 hold enough deposit...

The Go implementation (`shouldMoveToNextRoundPrecommit`) requires the timeout to have elapsed **before** checking the higher-round deposit condition. The timeout is a hard prerequisite, not an alternative. The correct word is **and**.

Step 9.2 (ACK phase) correctly uses "or" because `shouldMoveToNextRoundProposalAcks` can trigger escalation from the higher-round deposit condition alone, without a timeout gate.

---

### F2. TLA+ double-counting of equivocating validator deposits in `EscalateFromAck`

| | |
|---|---|
| **Severity** | TLA+ modeling inaccuracy (does not affect safety conclusions) |
| **Reported by** | Review A |
| **Status** | Open |

In `EscalateFromAck(v)`, the condition:

```
AckDepositForType("OK", round) + AckDepositForType("NIL", round) >= Threshold
```

double-counts a Byzantine validator's deposit if it sent both OK and NIL (equivocation). In the Go code, `validatorProposalAcks` is a map keyed by validator, so each validator's deposit is counted only once regardless of equivocation.

**Impact:** Makes escalation slightly easier to trigger in the TLA+ model than in the real code. This is a sound over-approximation for safety proofs (if safety holds with more liberal escalation, it holds with stricter escalation), but it is an inaccuracy relative to the Go implementation.

---

### F3. README oversimplifies ACK threshold semantics

| | |
|---|---|
| **Severity** | Design documentation imprecision |
| **Reported by** | Review C |
| **Status** | Open |

The README says the phase transition happens when 67% weighted `ACK_PROPOSAL` votes are collected. The implementation is stricter:
- Transition on `OK` requires 67% of `OK` votes matching the proposal hash, **and** the local validator must have also voted `OK`.
- Transition on `NIL` requires 67% of `NIL` votes.

The "local validator must have voted OK" gate (`selfAckProposalVoteType == VOTE_TYPE_OK`) is a consistency guarantee not mentioned in the README. Review B identified this same detail but classified it as "correctly omitted implementation detail" rather than a documentation gap.

---

### F4. TLA+ model uses global state vs. validator-local views

| | |
|---|---|
| **Severity** | Known abstraction (not a bug) |
| **Reported by** | Review C |
| **Status** | Acknowledged |

The TLA+ model uses a single global `round` and global vote sets (`ackVotes`, `precommitVotes`, `commitVotes`), rather than per-validator local message views. Review C argues this means the equivocation modeling is not as precise as the documentation claims -- Byzantine OK and NIL votes coexist globally, but the model doesn't explicitly represent different honest validators holding different local views.

Reviews A and B consider this a sound over-approximation: if safety holds when all votes are globally visible (worst case for the adversary), it holds when honest validators have partial views.

---

### F5. Config file comments are stale

| | |
|---|---|
| **Severity** | Cosmetic |
| **Reported by** | Review C |
| **Status** | Open |

- `QuantumCoinConsensusBoundary.cfg` comment says "just under 33%" but the configured deposit is exactly 33/100.
- `QuantumCoinConsensusUnsafe.cfg` comment says "50%" but the configured deposit is 17+17 = 34%.

---

### F6. TLA README imprecise about ASSUME placement

| | |
|---|---|
| **Severity** | Cosmetic |
| **Reported by** | Reviews B, C |
| **Status** | Open |

The TLA README says "The specification formally asserts `ASSUME ByzantineDeposit * 3 < TotalDeposit`." This ASSUME lives in `MCQuantumCoinConsensus.tla` (the model-checking wrapper), not in `QuantumCoinConsensus.tla` (the main spec). The main spec intentionally omits it so that the unsafe module can extend it. Correct in substance, imprecise in location.

---

### F7. Unsafe configuration partial state-space counts are non-reproducible

| | |
|---|---|
| **Severity** | Informational |
| **Reported by** | Reviews B, C |
| **Status** | Expected behavior |

The tla-report says 283,054 states generated for the Unsafe run. Actual runs produce different numbers (244,187 per Review B; 280,828 per Review C). This is expected: TLC stops at the first violation, and with 16 parallel workers, which states are explored before the violation is found depends on thread scheduling. The violation, the property broken, and the counterexample structure are all consistent.

---

### F8. Unsafe counterexample narrative slightly imprecise

| | |
|---|---|
| **Severity** | Cosmetic |
| **Reported by** | Review C |
| **Status** | Open |

The tla-report describes the unsafe violation as if both OK and NIL thresholds are met "simultaneously." The actual TLC trace shows a sequential interleaving: v2 advances on the NIL path first, then v1 proposes and advances on the OK path later. The high-level conclusion (equivocation enables conflicting quorums) is correct, but "simultaneously" is an imprecise description of the trace.

---

## Cross-Review Agreement Matrix

| Finding | Review A | Review B | Review C |
|---------|----------|----------|----------|
| F1. Step 11.2 "or" → "and" | Not mentioned | **Reported** | Not mentioned |
| F2. Double-counting in EscalateFromAck | **Reported** | Not mentioned | Not mentioned |
| F3. ACK threshold oversimplified | Not mentioned | Noted as "correctly omitted" | **Reported** |
| F4. Global vs. local state abstraction | Not mentioned | Noted as "sound" | **Reported** |
| F5. Stale config comments | Not mentioned | Not mentioned | **Reported** |
| F6. ASSUME placement imprecision | Not mentioned | **Reported** | **Reported** |
| F7. Non-reproducible unsafe stats | Not mentioned | **Reported** | **Reported** |
| F8. Counterexample narrative imprecise | Not mentioned | Not mentioned | **Reported** |
| README-to-code alignment | Confirmed | 1 issue found | 2 issues found |
| TLA+-to-README alignment | Confirmed | Confirmed | Mostly confirmed |
| TLC results confirmed | Yes | Yes | Yes |
