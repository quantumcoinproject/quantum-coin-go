# QuantumCoin (Q) - Go Implementation

**QuantumCoin (Q) is a quantum-resistant blockchain** that implements **NIST post-quantum cryptography (PQC)** algorithms for digital signatures and node-to-node key establishment. This repository (`quantum-coin-go`) is the Go implementation of the QuantumCoin node client, forked from [go-ethereum](https://github.com/ethereum/go-ethereum).

[![Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/bbbMPyzJTM)

---

### TL;DR - Post-Quantum Cryptography Summary

| Component | PQC Algorithm | NIST Standard | Classical Algorithm | Hybrid |
|-----------|---------------|---------------|---------------------|--------|
| **Signatures** | ML-DSA-44/87, SLH-DSA-SHAKE-256f/s | FIPS 204, FIPS 205 | ed25519 | Yes |
| **Key Establishment** | ML-KEM-768 | FIPS 203 | X25519 | Yes |

All **PQC-signature operations** use hybrid constructions combining post-quantum and classical algorithms, ensuring security against both quantum and classical attackers.

---

## Post-Quantum Cryptography

QuantumCoin uses **hybrid constructions** (PQC + classical) so the system remains secure against both classical attackers and quantum-capable adversaries. Hybrid mode also provides a hedge if any single primitive is weakened. See: [NIST PQC FAQs](https://csrc.nist.gov/projects/post-quantum-cryptography/faqs).

**Algorithms used (NIST standard names):**
- **Signatures:** **ML-DSA** (Module-Lattice Digital Signature Algorithm; standardized from Dilithium; [FIPS 204](https://csrc.nist.gov/pubs/fips/204/final)), **SLH-DSA** (Stateless Hash-Based Digital Signature Algorithm; standardized from SPHINCS+; [FIPS 205](https://csrc.nist.gov/pubs/fips/205/final)), plus **ed25519** in a hybrid combiner.
- **Key Establishment:** **ML-KEM** (Module-Lattice Key Encapsulation Mechanism; standardized from Kyber; [FIPS 203](https://csrc.nist.gov/pubs/fips/203/final)) in hybrid with **X25519** (time-gated; see below).

### Post-Quantum Digital Signature Schemes

QuantumCoin implements multiple **hybrid signature** modes. At a high level, a hybrid signature is built from:

- **PQC signature** (e.g., ML-DSA and/or SLH-DSA), plus
- **ed25519** (classical),

combined so that **verification must succeed for all component signatures** required by that mode. This means an attacker must break *both* the post-quantum and classical algorithms to forge a signature.

Two families of hybrid signatures exist in the codebase:

- **Dilithium + ed25519 + SPHINCS+** (legacy naming in code: `DILITHIUM_ED25519_SPHINCS_*`)
- **ML-DSA + ed25519 + SLH-DSA** (FIPS-aligned naming in code: `MLDSA_ED25519_SLHDSA_*`)

QuantumCoin supports a **compact vs full** approach:

- **Compact mode**: Uses ML-DSA + ed25519 for smaller on-chain size (default for most operations).
- **Full mode** (break-glass): Adds SLH-DSA (hash-based signatures) for defense-in-depth. Hash-based signatures like SLH-DSA provide security based on the well-understood properties of hash functions, offering an additional layer of protection even if lattice-based assumptions are compromised.

The post-quantum cryptography code lives under [`./crypto`](./crypto). QuantumCoin uses a fork of Cloudflare's CIRCL library for PQC/hybrid primitives (see dependency `github.com/quantumcoinproject/circl` in [`go.mod`](./go.mod)).

#### Usage of PQC Signature Schemes

1. **Transactions** are signed with one of the hybrid PQC signature schemes. The gas price of the transaction varies based on the scheme to compensate for larger signature and public key sizes.
2. **Node keys** use post-quantum cryptography to provide blockchain node identifiers via signed packets.
3. **Validator consensus messages** use the same hybrid post-quantum signature approach. Messages are typically signed in **compact mode**, while the proposal message of every 4,096th block is signed in **full mode** (adding the extra PQC component) to maintain smaller average block sizes.

The hybrid schemes exposed at the protocol level include:

| Scheme | ML-DSA Variant | SLH-DSA Variant | Classical | NIST Security Level |
|--------|----------------|-----------------|-----------|---------------------|
| Scheme 1 | ML-DSA-44 | SLH-DSA-SHAKE-256f | ed25519 | Level 1 / Level 5 |
| Scheme 2 | ML-DSA-87 | SLH-DSA-SHAKE-256s | ed25519 | Level 5 / Level 5 |

To verify use of the PQC signature schemes, inspect any transaction or validator message and verify the signature fields with a standard PQC library, using the algorithm identifier to select the correct hybrid mode.

### Post-Quantum Key Establishment

Node-to-node sessions use a **PQC-capable KEM** from the CIRCL fork. The KEM selection is time-gated via `defaults.DefaultConfig.KemSwitchTime`:

- **Before `KemSwitchTime`**: `Kyber512` (PQC KEM) via `circl/kem/kyber/kyber512`
- **After `KemSwitchTime`**: hybrid **X25519 + ML-KEM-768** via `circl/kem/hybrid.X25519MLKEM768()`

These are used to **establish** a secure session between blockchain nodes in the rewritten RLPx handshake.

Note: the default mainnet config sets `KemSwitchTime` to **Feb 1, 2026 00:00:00 UTC** (see `defaults/config.go`).

- **KEM selection logic**: [`./crypto/keyestablishmentalgorithm/kem.go`](./crypto/keyestablishmentalgorithm/kem.go)
- **Handshake (client)**: [`./p2p/rlpx/client.go`](./p2p/rlpx/client.go)
- **Handshake (server)**: [`./p2p/rlpx/server.go`](./p2p/rlpx/server.go)
- **Network switch time**: [`./defaults/config.go`](./defaults/config.go)

### Evidence (Quick Pointers for Reviewers)

| What | File Path | Details |
|------|-----------|---------|
| Hybrid signature algorithm IDs | [`./crypto/crypto.go`](./crypto/crypto.go) | `DILITHIUM_ED25519_SPHINCS_*`, `MLDSA_ED25519_SLHDSA_*` |
| Signature selection/verification | [`./crypto/cryptobase/cryptobase.go`](./crypto/cryptobase/cryptobase.go) | Wiring for hybrid signatures |
| KEM selection and session logic | [`./crypto/keyestablishmentalgorithm/kem.go`](./crypto/keyestablishmentalgorithm/kem.go) | Kyber512 / X25519+ML-KEM-768 |
| Protocol-level switches | [`./defaults/config.go`](./defaults/config.go) | `SigAlgSwitchBlock`, `KemSwitchTime` |
| CIRCL hybrid signature bindings | `./crypto/*` | Imports `github.com/quantumcoinproject/circl/sign/...` |

### Quick Verification (Locally)

Run unit tests for the PQC/hybrid components:

```bash
go test ./crypto/...
go test ./p2p/rlpx
```

### Prerequisites

Requires a Go toolchain compatible with the version declared in [`go.mod`](./go.mod).

#### Building

Build the node binary:

```bash
go build -o ./build ./...
```

### Running the Node

Check the [documentation](https://quantumcoin.org) portal for information on running the blockchain node client.

### Other Changes from Ethereum

1. **32-byte addresses**: Addresses are 32 bytes instead of 20 bytes in Ethereum, for increased security.

2. **Rewritten RLPx protocol**: The RLPx protocol has been completely rewritten and modularized to use post-quantum cryptography. The final client and server encryption keys are derived similarly to TLS 1.3 as detailed in RFC 8446. A PQC-capable KEM is used for key exchange, and the resulting key material is used as input to HMAC HKDF functions (RFC 5869). However, unlike TLS, instead of trusting a certificate, the node's identity is verified via its hybrid PQC key pair. The private key corresponds to the hybrid PQC key pair used to secure the account using digital signatures. These changes are in the [`./p2p/rlpx`](./p2p/rlpx) package.

3. **New consensus engine**: A new consensus engine (Proof-of-Stake) has been added. It uses 3-phase BFT consensus for immediate deterministic finality. Timeout values are adjusted to improve liveness within the bounds of the **FLP theorem**.

## Known Issues

1. Commits to fix tests are pending sanitization before merge.
2. The transaction metadata contains values named `v`, `r`, and `s`, which are legacy fields from Ethereum. In Ethereum, these values are used for public key recovery; however, QuantumCoin's hybrid PQC signatures use a different mechanism for identity verification.

## Addendum

"Quantum Coin" ("QuantumCoin") and "Quantum Coin Community" were previously known under the monikers "Doge Protocol" and "Doge Protocol Community" respectively.

## Contributing

Thank you for considering helping out with the source code! We welcome contributions from anyone on the internet, and are grateful for even the smallest of fixes!

If you'd like to contribute to quantum-coin-go, please fork, fix, commit and send a pull request to review and merge into the main code base. If you wish to submit more complex changes though, please check up first in [our community Discord Server](https://discord.gg/bbbMPyzJTM) to ensure those changes are in line with the general philosophy of the project and/or get some early feedback which can make both your efforts much lighter as well as our review and merge procedures quicker and simpler.

Please make sure your contributions adhere to our coding guidelines:

- Code must adhere to the official Go [formatting](https://golang.org/doc/effective_go.html#formatting) guidelines (i.e. use [gofmt](https://golang.org/cmd/gofmt/)).
- Code must be documented according to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary) guidelines.
- Pull requests need to be based on and opened against the `dogep` branch.
- Commit messages should be prefixed with the package(s) they modify.
  - E.g. "eth, rpc: make trace configs optional"

## License

The quantum-coin-go library maintains the same licensing model of go-ethereum. The library (i.e. all code outside of the `cmd` directory) is licensed under the [GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also included in our repository in the `COPYING.LESSER` file.

The binaries (i.e. all code inside of the `cmd` directory) are licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also included in our repository in the `COPYING` file.
