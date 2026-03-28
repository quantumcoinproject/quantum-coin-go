# QuantumCoin Proof-of-Stake Consensus Protocol

> **See also:** [TLA+ Formal Specification](tla/README.md) -- formal model of this protocol for automated verification with the TLC model checker.
>
> **See also:** [TLA+ Verification Report](tla/tla-report.md) -- results of exhaustive model checking, including properties verified and fault tolerance boundary analysis.

This document describes the block-level consensus protocol used by QuantumCoin's proof-of-stake system. It specifies the sequence of message exchanges between validators required to produce each block. The protocol is a stake-weighted, multi-round BFT (Byzantine Fault Tolerant) consensus with four message phases per round: `PROPOSAL`, `ACK_PROPOSAL`, `PRECOMMIT`, and `COMMIT` (4 phases for proposer and 3 phases for non-proposers).

---

## Glossary

| Term | Definition |
|------|-----------|
| **Validator** | A node that actively represents an account that has registered as a validator by staking coins and is eligible to participate in consensus for a given block. The set of validators is deterministically selected and filtered per block. |
| **Proposer** | The single validator deterministically selected (per round) to create and broadcast a block `PROPOSAL`. |
| **Round** | An attempt to reach consensus on a block. Each block allows at most 2 rounds. Round 1 is a normal round. Round 2 is a forced `NIL` round (that creates an empty block with no transactions). |
| **`PROPOSAL`** | A message broadcast by the proposer containing the set of transactions to include in the block, the round number, and the proposed block time. For Round 2, the proposal contain zero transactions. |
| **`ACK_PROPOSAL`** | A vote sent by each validator in response to a `PROPOSAL` (or a timeout). Contains a vote type (`OK` or `NIL`) and the proposal hash. |
| **`PRECOMMIT`** | A vote sent by each validator after 67% of `ACK_PROPOSAL` votes have been collected. Contains a precommit hash derived from the proposal hash and vote type. |
| **`COMMIT`** | A vote sent by each validator after 67% of `PRECOMMIT` votes have been collected. Contains a commit hash derived from the precommit hash. |
| **`OK`** | A vote type indicating the validator accepts the proposed block (with transactions). Only used in rounds where `round < MAX_ROUND`. |
| **`NIL`** | A vote type indicating the validator votes for an empty block. Used when a `PROPOSAL` times out, or unconditionally in Round 2. |
| **Deposit** | The amount of coins staked by a validator. All vote thresholds are weighted by deposit, not by validator count. |
| **67% Threshold** | The minimum weighted deposit required for a phase transition: A phase completes when the sum of deposits of validators who sent the required message meets or exceeds this threshold. The 67% value is rounded up from 2/3 (66.67%). This exact fraction is required so that any two quorums overlap by more than 1/3, which exceeds the maximum Byzantine deposit -- guaranteeing at least one honest validator exists in every quorum intersection. |
| **Timeout** | A duration after which a validator that has not received an expected message proceeds with a `NIL` vote or escalates to the next round. |
| **Block Finalization** | A block is finalized when 67% weighted `COMMIT` votes are collected. The block may contain transactions (`OK` vote) or be empty (`NIL` vote). |
| **Byzantine Validator** | A validator that deviates from the protocol. May send conflicting votes to different peers (equivocation), selectively withhold proposals, vote arbitrarily in any phase, or skip phases entirely. |

---

## Fault Model

The protocol is Byzantine Fault Tolerant (BFT) and tolerates up to **< 1/3** of total weighted deposit controlled by Byzantine validators. This bound derives from the 67% threshold: any two quorums of 67% must overlap by at least 34%, which exceeds the maximum Byzantine deposit (< 33.3%), guaranteeing that any two quorums share at least one honest validator.

Byzantine validators may exhibit the following behaviors:

- **Equivocation**: Send `OK` votes to some peers and `NIL` votes to others for the same `ACK_PROPOSAL`.
- **Selective delivery**: A Byzantine proposer may deliver a `PROPOSAL` to only a subset of validators.
- **Arbitrary voting**: Vote `OK` or `NIL` in any phase regardless of protocol rules.
- **Phase skipping**: Advance to later phases (e.g., `PRECOMMIT`, `COMMIT`) without completing earlier ones.
- **Crash / abstention**: Stop participating entirely (go offline, crash, or withhold all votes). This is a special case of Byzantine behavior -- a Byzantine validator that chooses to do nothing is indistinguishable from a crashed one.

As long as Byzantine validators control less than 1/3 of total deposit, the protocol guarantees:

- **Safety**: No two honest validators finalize with different vote types.
- **Liveness**: All honest validators eventually finalize under partial synchrony (timeouts).

---

## FLP Impossibility and Liveness Assumptions

The FLP impossibility result (Fischer, Lynch, Paterson, 1985) proves that no deterministic consensus protocol can guarantee both safety and liveness in a purely asynchronous network where even a single process may crash. In a fully asynchronous model, messages can be delayed indefinitely, making it impossible to distinguish a crashed node from a slow one.

FLP's impossibility applies to purely asynchronous networks with no timing guarantees. QuantumCoin protocol cannot operate under that model -- it assumes **partial synchrony**. The key assumptions that enable liveness:

1. **Bounded message delay (eventually):** The network is assumed to become synchronous after some unknown Global Stabilization Time (GST). Before GST, messages may be delayed arbitrarily (and liveness is not guaranteed). After GST, all messages between honest validators are delivered within a bounded time.

2. **Timeouts:** Each phase has a timeout duration. If a validator does not receive the expected message within the timeout, it proceeds with a `NIL` vote (for proposals) or escalates to the next round (for `ACK_PROPOSAL` and `PRECOMMIT` phases). Timeouts serve as the synchrony oracle: they translate the bounded-delay assumption into concrete protocol actions.

3. **Round escalation:** If Round 1 stalls (neither `OK` nor `NIL` reaches the 67% threshold), validators escalate to Round 2 -- a forced `NIL` round where all validators vote `NIL` unconditionally. This guarantees that at least one round produces a finalized block, provided the network eventually delivers messages.

4. **Honest majority:** At least 2/3 of total weighted deposit is controlled by honest validators who follow the protocol and whose messages are eventually delivered.

Under these assumptions, the protocol guarantees liveness: every honest validator eventually finalizes a block. Safety (Agreement) holds unconditionally -- even during periods of asynchrony before GST, no two honest validators will finalize with conflicting vote types.

---

## Protocol States (per round)

Each round progresses through these states in order:

| State | Waiting for |
|-------|------------|
| `WAITING_FOR_PROPOSAL` | `PROPOSAL` message from the round's proposer |
| `WAITING_FOR_ACK_PROPOSAL` | `ACK_PROPOSAL` votes reaching 67% threshold |
| `WAITING_FOR_PRECOMMIT` | `PRECOMMIT` votes reaching 67% threshold |
| `WAITING_FOR_COMMIT` | `COMMIT` votes reaching 67% threshold |
| `RECEIVED_COMMITS` | Terminal state; block is finalized |

---

## Consensus Steps

**1) Interrupt:**
At any point, if a valid finalized block is received from the network for the current block number, abort the current consensus and goto step 2 for the next block.

**2) New block begins.**
The consensus handler is invoked with the parent hash of the last finalized block.

**3) Select validator set.**
Deterministically select and filter the set of validators for this block based on on-chain staked deposits.

**4) Initialize Round 1.**
Set round = 1 and state = `WAITING_FOR_PROPOSAL`.

**5) Select proposer.**
Deterministically compute the proposer for the current round from the filtered validator set.

**6) Check proposer role.**
Is this node the selected proposer?

> **6.1) Yes (proposer path):**
>
> > 6.1.1) Construct a `PROPOSAL` message containing selected pending transactions
> > (or zero transactions if round = 2).
> >
> > 6.1.2) Broadcast the `PROPOSAL` to all validators including self.
> >
> > 6.1.3) Goto step 7.
>
> **6.2) No (non-proposer path):**
>
> > 6.2.1) Goto step 7.

**7) Evaluate `PROPOSAL` receipt.**
Was a valid `PROPOSAL` received before the proposal timeout?

> **7.1) Yes, `PROPOSAL` received:**
>
> > If round = 1: broadcast an `ACK_PROPOSAL` with vote type = `OK` to all validators.
> >
> > If round = 2: broadcast an `ACK_PROPOSAL` with vote type = `NIL` to all validators.
>
> **7.2) No, `PROPOSAL` timed out:**
>
> > Broadcast an `ACK_PROPOSAL` with vote type = `NIL` to all validators.

**8) Wait for `ACK_PROPOSAL` threshold.**
Collect `ACK_PROPOSAL` votes from validators until the 67% weighted deposit threshold is reached.

**9) Evaluate `ACK_PROPOSAL` threshold.**
Has the 67% threshold been reached?

> **9.1) Yes:**
>
> > Broadcast a `PRECOMMIT` vote to all validators. Goto step 10.
>
> **9.2) No:**
>
> > If round = 1: if the ack timeout is exceeded, or if validators already
> > participating in Round 2 hold enough deposit that Round 1 can never
> > reach the 67% threshold: goto step 14.
> >
> > If round = 2: remain waiting (no further rounds exist).

**10) Wait for `PRECOMMIT` threshold.**
Collect `PRECOMMIT` votes from validators until the 67% weighted deposit threshold is reached.

**11) Evaluate `PRECOMMIT` threshold.**
Has the 67% threshold been reached?

> **11.1) Yes:**
>
> > Broadcast a `COMMIT` vote to all validators. Goto step 12.
>
> **11.2) No:**
>
> > If round = 1: if the precommit timeout is exceeded, and validators already
> > participating in Round 2 hold enough deposit that Round 1 can never
> > reach the 67% threshold: goto step 14.
> >
> > If round = 2: remain waiting (no further rounds exist).

**12) Wait for `COMMIT` threshold.**
Collect `COMMIT` votes from validators until the 67% weighted deposit threshold is reached.

**13) Evaluate `COMMIT` threshold.**
Has the 67% threshold been reached?

> **13.1) Yes:**
>
> > Block is finalized. Mine the block and goto step 2 for the next block.
>
> **13.2) No:**
>
> > Remain waiting, unless step 1 is triggered.

**14) Round escalation** (only from Round 1; max 2 rounds allowed):

> 14.1) Set round = 2. Round 2 is a forced `NIL` round: the proposer MUST include
> zero transactions in the `PROPOSAL`, and all validators MUST vote `NIL` in their
> `ACK_PROPOSAL`. This produces an empty block, guaranteeing the chain makes progress.
>
> 14.2) Goto step 5 with round = 2.
