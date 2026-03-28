# QuantumCoin Consensus TLA+ Specification

> **See also:** [Consensus Protocol Description](../README.md) -- human-readable step-by-step specification of the protocol that this TLA+ model formalizes.
>
> **See also:** [TLA+ Verification Report](tla-report.md) -- results of exhaustive model checking, including all properties verified and fault tolerance boundary analysis.

Formal TLA+ specification of the QuantumCoin proof-of-stake consensus protocol,
suitable for model checking with TLC.

## Files

| File | Description |
|------|-------------|
| `QuantumCoinConsensus.tla` | Main specification: models the 4-phase BFT protocol with Byzantine faults (including equivocation), deposit-weighted voting, two-round escalation, and OK/NIL vote types. Defines `ByzantineDeposit` for use in the fault tolerance `ASSUME`. |
| `MCQuantumCoinConsensus.tla` | Model-checking module: instantiates the spec with 4 validators and concrete constants. Contains Safe and Boundary configurations. |
| `MCQuantumCoinConsensusUnsafe.tla` | Unsafe model-checking module: 2 Byzantine validators (combined 34% deposit), bypasses the < 33% `ASSUME` to demonstrate safety violations just above the fault tolerance threshold. |
| `QuantumCoinConsensus.cfg` | TLC configuration for the Safe model (1 Byzantine, 25% deposit). |
| `QuantumCoinConsensusBoundary.cfg` | TLC configuration for the Boundary model (1 Byzantine, 33% deposit, at the limit). |
| `QuantumCoinConsensusUnsafe.cfg` | TLC configuration for the Unsafe model (2 Byzantine, combined 34% deposit). Expected to find violations. |
| `tla_test.go` | Go test harness: invokes TLC from `go test` and fails on any counterexamples or invariant violations. |

## Fault Tolerance

The model-checking wrapper (`MCQuantumCoinConsensus.tla`) formally asserts that Byzantine validators control less than 1/3 of total deposit:

```
ASSUME ByzantineDeposit * 3 < TotalDeposit
```

This ASSUME is placed in the model-checking module (not the main spec) so that `MCQuantumCoinConsensusUnsafe.tla` can extend the same spec without violating the assumption. The main spec defines the `ByzantineDeposit` operator used by this ASSUME.

This mirrors the standard BFT requirement: with a 67% threshold, any two quorums overlap by at least 34%, which exceeds the maximum Byzantine deposit (< 33%). This guarantees that any two quorums share at least one honest validator, preventing conflicting finalizations.

### Byzantine Behaviors Modeled

The spec models the following Byzantine behaviors, all of which are explored nondeterministically by TLC:

- **Equivocation**: A Byzantine validator sends both `OK` and `NIL` `ACK_PROPOSAL` votes for the same round, so different honest peers observe different vote types.
- **Selective proposal delivery**: A Byzantine proposer delivers the `PROPOSAL` to an arbitrary subset of validators (including possibly none).
- **Arbitrary voting**: Byzantine validators can send `ACK_PROPOSAL`, `PRECOMMIT`, and `COMMIT` votes regardless of preconditions.
- **Phase skipping**: Byzantine validators advance through protocol phases without waiting for thresholds.

## Properties Verified

### Safety (checked as invariants)

- **TypeOK** -- All variables remain within their declared types.
- **Agreement** -- No two honest validators finalize with different vote types.
- **Validity** -- If all honest validators voted OK, no honest validator finalizes as NIL.
- **Round2Consistency** -- Any honest finalization in Round 2 has vote type NIL.
- **CommitIntegrity** -- No honest validator finalizes without >= 67% commit deposit weight.

### Liveness (checked as temporal properties)

- **Termination** -- All honest validators eventually finalize.

## FLP Impossibility and Liveness in the TLA+ Model

The FLP impossibility result (Fischer, Lynch, Paterson, 1985) proves that no deterministic consensus protocol can guarantee both safety and liveness in a purely asynchronous system. This specification passes both safety and liveness checks because the TLA+ model does **not** assume a purely asynchronous network -- it encodes partial synchrony assumptions that the real protocol relies on.

### Network Assumptions

FLP assumes a purely asynchronous network with no timing guarantees whatsoever. The QuantumCoin consensus protocol cannot operate under that model. Instead, it requires **partial synchrony**: the network eventually delivers messages within bounded time after some unknown Global Stabilization Time (GST). Before GST, safety still holds but liveness is not guaranteed. After GST, timeouts and round escalation ensure progress.

### How the TLA+ spec encodes partial synchrony

The spec uses **weak fairness** to model the eventual message delivery assumption:

```
Fairness == \A v \in Honest : WF_vars(HonestActions(v))
Spec == Init /\ [][Next]_vars /\ Fairness
```

`WF_vars(HonestActions(v))` means: if the actions of honest validator `v` are continuously enabled, then `v` must eventually take a step. This prevents infinite starvation of any honest validator and is the TLA+ analog of "messages are eventually delivered."

Concretely, this fairness condition guarantees:

| Protocol mechanism | TLA+ encoding | What it ensures |
|---|---|---|
| Proposal delivery or timeout | `WF` on `ReceiveProposal(v)` and `ProposalTimeout(v)` | Every honest non-proposer eventually receives the proposal or times out |
| ACK_PROPOSAL voting | `WF` on `SendAckProposal(v)` | Every honest validator that received a proposal or timed out eventually sends its ACK |
| Phase transitions | `WF` on `SendPrecommit(v)`, `SendCommit(v)`, `FinalizeBlock(v)` | Once thresholds are met, honest validators eventually advance |
| Round escalation | `WF` on `EscalateFromAck(v)`, `EscalateFromPrecommit(v)` | If Round 1 is stuck, honest validators eventually escalate to Round 2 |

Timeouts are modeled nondeterministically (`\/ TRUE` in escalation guards and `ProposalTimeout`) rather than with explicit clocks. This is a sound over-approximation: any behavior possible under concrete timeouts is also possible under nondeterministic timeouts.

### Relationship to FLP

FLP applies to purely asynchronous systems with **no** timing or fairness assumptions. The `WF_vars` fairness condition is precisely the assumption that places this protocol outside FLP's scope -- it guarantees that enabled actions eventually occur, which corresponds to the partial synchrony assumption that messages are eventually delivered. Under partial synchrony, liveness is achievable alongside safety, as shown by Dwork, Lynch, and Stockmeyer (1988). The protocol does not "solve" or circumvent FLP; it simply operates under stronger assumptions than those FLP considers.

## Model Configurations

### Safe (default): `QuantumCoinConsensus.cfg`

| Parameter | Value |
|-----------|-------|
| Validators | v1, v2, v3, v4 |
| Deposits | 25 each (total = 100) |
| Byzantine | v4 (25% deposit) |
| Proposers | v1 (Round 1), v2 (Round 2) |

All properties should **pass**.

### Boundary: `QuantumCoinConsensusBoundary.cfg`

| Parameter | Value |
|-----------|-------|
| Validators | v1, v2, v3, v4 |
| Deposits | v1=23, v2=22, v3=22, v4=33 (total = 100) |
| Byzantine | v4 (33% deposit, at the limit) |
| Proposers | v1 (Round 1), v2 (Round 2) |

All properties should **pass**, demonstrating safety holds at exactly 33%.

### Unsafe: `QuantumCoinConsensusUnsafe.cfg`

| Parameter | Value |
|-----------|-------|
| Validators | v1, v2, v3, v4 |
| Deposits | v1=33, v2=33, v3=17, v4=17 (total = 100) |
| Byzantine | v3, v4 (combined 34% deposit, just above 33%) |
| Proposers | v1 (Round 1), v2 (Round 2) |

Safety properties are expected to **fail**, demonstrating that the protocol cannot tolerate > 33% Byzantine deposit. The violation occurs at just 34% -- only 1% above the passing boundary.

## Prerequisites

1. **Java** (JDK 11+) -- required to run TLC.
2. **TLA+ Tools** (`tla2tools.jar`) -- download from
   [https://github.com/tlaplus/tlaplus/releases](https://github.com/tlaplus/tlaplus/releases).

Place `tla2tools.jar` in this directory, or set the `TLA_TOOLS_JAR` environment
variable to its absolute path.

## Running the Model Checker

### Safe configuration

```bash
cd consensus/proofofstake/tla
java -jar tla2tools.jar -config QuantumCoinConsensus.cfg -workers auto -deadlock MCQuantumCoinConsensus
```

### Boundary configuration

```bash
java -jar tla2tools.jar -config QuantumCoinConsensusBoundary.cfg -workers auto -deadlock MCQuantumCoinConsensus
```

### Unsafe configuration (expected to find violations)

```bash
java -jar tla2tools.jar -config QuantumCoinConsensusUnsafe.cfg -workers auto -deadlock MCQuantumCoinConsensusUnsafe
```

### Via Go test harness

```bash
cd consensus/proofofstake/tla
go test -v -run TestTLCModelCheck -count=1
```

Use `-short` to skip TLC tests during normal development:

```bash
go test -short ./consensus/proofofstake/tla/...
```

## Protocol Reference

See [../README.md](../README.md) for the full 14-step consensus protocol
description that this specification models.
