# QuantumCoin (Q) - Go Implementation

**QuantumCoin (Q) is a quantum-resistant blockchain** that implements **NIST post-quantum cryptography (PQC)** algorithms for digital signatures and node-to-node key establishment. This repository (`quantum-coin-go`) is the Go implementation of the QuantumCoin node client, forked from [go-ethereum](https://github.com/ethereum/go-ethereum).

[![Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/bbbMPyzJTM)

## Building

Requires a Go toolchain compatible with the version declared in [`go.mod`](./go.mod).

Build the node binary:

```bash
go build -o ./build ./...
```

### Running the Node

Check the [documentation](https://quantumcoin.org/connecting-to-mainnet.html) portal for information on running the blockchain node client.

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
- **Key Establishment:** **ML-KEM** (Module-Lattice Key Encapsulation Mechanism; standardized from Kyber; [FIPS 203](https://csrc.nist.gov/pubs/fips/203/final)) in hybrid with **X25519**.

### Post-Quantum Digital Signature Schemes

QuantumCoin implements multiple **hybrid signature** modes. At a high level, a hybrid signature is built from:

- **PQC signature** (e.g., ML-DSA and/or SLH-DSA), plus
- **ed25519** (classical),

combined so that **verification must succeed for all component signatures** required by that mode. This means an attacker must break *both* the post-quantum and classical algorithms to forge a signature.

Two families of hybrid signatures exist in the codebase:

- **Dilithium + ed25519 + SPHINCS+** (legacy naming in code: `DILITHIUM_ED25519_SPHINCS_*`)
- **ML-DSA + ed25519 + SLH-DSA** (FIPS-aligned naming in code: `MLDSA_ED25519_SLHDSA_*`)

QuantumCoin supports a **compact vs full** approach:

- **Compact mode**: Signs with Ed25519 + ML-DSA only, for smaller on-chain size (default for most operations). The SLH-DSA **public key is still embedded** in the compact signature, so the hash-based component can be verified retroactively if a full signature is later produced for the same key pair.
- **Full mode** (break-glass): Signs with all three components (Ed25519 + ML-DSA + SLH-DSA) for defense-in-depth. Hash-based signatures like SLH-DSA provide security based on the well-understood properties of hash functions, offering an additional layer of protection even if lattice-based assumptions are compromised.

The post-quantum cryptography code lives under [`./crypto`](./crypto). QuantumCoin uses a fork of Cloudflare's CIRCL library for PQC/hybrid primitives (see dependency `github.com/quantumcoinproject/circl` in [`go.mod`](./go.mod)).

#### Usage of PQC Signature Schemes

1. **Transactions** are signed with one of the hybrid PQC signature schemes. The gas price of the transaction varies based on the scheme to compensate for larger signature and public key sizes.
2. **Node keys** use post-quantum cryptography to provide blockchain node identifiers via signed packets.
3. **Validator consensus messages** use the same hybrid post-quantum signature approach. Messages are typically signed in **compact mode**, while the proposal message of every 4,096th block is signed in **full mode** (adding the extra PQC component) to maintain smaller average block sizes.

The hybrid schemes exposed at the protocol level include:

| Algorithm ID | Code Constant | Components | Mode | Notes |
|:---:|---|---|---|---|
| 1 | `DILITHIUM_ED25519_SPHINCS_COMPACT_ID` | Ed25519 + Dilithium + SPHINCS+ | Compact | Legacy (pre-`SigAlgSwitchBlock`) |
| 2 | `DILITHIUM_ED25519_SPHINCS_FULL_ID` | Ed25519 + Dilithium + SPHINCS+ | Full | Legacy (pre-`SigAlgSwitchBlock`) |
| 3 | `MLDSA_ED25519_SLHDSA_COMPACT_ID` | Ed25519 + ML-DSA + SLH-DSA | Compact | FIPS-aligned (post-`SigAlgSwitchBlock`) |
| 4 | `MLDSA_ED25519_SLHDSA_FULL_ID` | Ed25519 + ML-DSA + SLH-DSA | Full | FIPS-aligned (post-`SigAlgSwitchBlock`) |
| 5 | `MLDSA_ED25519_SLHDSA_5_ID` | Ed25519 + ML-DSA + SLH-DSA | Full | NIST Security Level 5 for all components |

The specific ML-DSA and SLH-DSA parameter sets (e.g., ML-DSA-44 vs ML-DSA-87, SLH-DSA-SHAKE-256f vs 256s) are determined by the CIRCL library implementations imported in each scheme package (see `circl/sign/hybridedmldsaslhdsa` for IDs 3–4 and `circl/sign/hybridedmldsaslhdsa5` for ID 5 in the [`quantumcoinproject/circl`](https://github.com/quantumcoinproject/circl) dependency).

To verify use of the PQC signature schemes, inspect any transaction or validator message and verify the signature fields with a standard PQC library, using the algorithm identifier to select the correct hybrid mode.

### Post-Quantum Key Establishment

Node-to-node sessions use hybrid **X25519 + ML-KEM-768** (via `circl/kem/hybrid.X25519MLKEM768()`) to **establish** a secure session between blockchain nodes in the rewritten RLPx handshake. The KEM is unconditional: every session uses this hybrid construction.

The RLPx protocol has two versions, selected at runtime by `defaults.DefaultConfig.KemSwitchTime` (mainnet: **Aug 21, 2026 00:00:00 UTC**; see `defaults/config.go`). Both versions use the same hybrid KEM; the V2 protocol adds encrypted headers, a fixed HKDF label encoding, and a new frame format.

- **KEM selection logic**: [`./crypto/keyestablishmentalgorithm/kem.go`](./crypto/keyestablishmentalgorithm/kem.go)
- **Handshake V1 (legacy, client)**: [`./p2p/rlpx/client.go`](./p2p/rlpx/client.go)
- **Handshake V1 (legacy, server)**: [`./p2p/rlpx/server.go`](./p2p/rlpx/server.go)
- **Handshake V2 (client)**: [`./p2p/rlpx/clientv2.go`](./p2p/rlpx/clientv2.go)
- **Handshake V2 (server)**: [`./p2p/rlpx/serverv2.go`](./p2p/rlpx/serverv2.go)
- **Key derivation (HKDF)**: [`./p2p/rlpx/secret.go`](./p2p/rlpx/secret.go)
- **Protocol switch time**: [`./defaults/config.go`](./defaults/config.go)

### Evidence (Quick Pointers for Reviewers)

| What | File Path | Details |
|------|-----------|---------|
| Hybrid signature algorithm IDs | [`./crypto/crypto.go`](./crypto/crypto.go) | `DILITHIUM_ED25519_SPHINCS_*`, `MLDSA_ED25519_SLHDSA_*` |
| Signature selection/verification | [`./crypto/cryptobase/cryptobase.go`](./crypto/cryptobase/cryptobase.go) | Wiring for hybrid signatures |
| KEM selection | [`./crypto/keyestablishmentalgorithm/kem.go`](./crypto/keyestablishmentalgorithm/kem.go) | X25519+ML-KEM-768 |
| RLPx handshake (V2) | [`./p2p/rlpx/clientv2.go`](./p2p/rlpx/clientv2.go), [`./p2p/rlpx/serverv2.go`](./p2p/rlpx/serverv2.go) | Active handshake after `KemSwitchTime` |
| HKDF key derivation | [`./p2p/rlpx/secret.go`](./p2p/rlpx/secret.go) | TLS 1.3-style key schedule |
| Protocol-level switches | [`./defaults/config.go`](./defaults/config.go) | `SigAlgSwitchBlock`, `KemSwitchTime` |
| CIRCL hybrid signature bindings | `./crypto/*` | Imports `github.com/quantumcoinproject/circl/sign/...` |

### Audit and independent verification

This section is intended for **cryptographers and auditors** who need to verify that QuantumCoin uses NIST-specified post-quantum and classical signature algorithms correctly, and to perform independent per-component verification (e.g. cross-checking against [PQClean](https://github.com/PQClean/PQClean), [liboqs](https://github.com/open-quantum-safe/liboqs), or other reference implementations).

**Underlying primitives.** The hybrid signature schemes do not modify any underlying cryptographic primitive; each component (Ed25519, ML-DSA, SLH-DSA, or the legacy Dilithium/SPHINCS+ in schemes 1–2) is invoked as specified by the relevant NIST standard (FIPS 186-5, FIPS 204, FIPS 205) or draft. This aligns with NIST guidance on combining NIST-approved and post-quantum algorithms ([NIST IR 8547](https://csrc.nist.gov/pubs/ir/8547/ipd)).

**Audit tooling (CIRCL).** For **audit, validation, and independent per-component verification** of hybrid signatures, the dependency [**quantumcoinproject/circl**](https://github.com/quantumcoinproject/circl) provides the **[hybridparser](https://github.com/quantumcoinproject/circl/tree/main/sign/hybridparser)** package. It offers:

- **ParseHybrid**: verify a hybrid signature and extract per-component public keys and signatures (hex-encoded) for re-verification with external DSA implementations.
- **CheckHybrid**: reconstruct and verify using both the composite hybrid verifier and each component’s verifier.

The **[hybridparser README](https://github.com/quantumcoinproject/circl/blob/main/sign/hybridparser/README.md)** describes NIST alignment (FIPS 204, FIPS 205, FIPS 186-5), the `HybridSignature` struct (SchemeID, Message, PublicKeys, Signatures, Context, AdditionalData), and how to decode hex components and pass them to external implementations for conformance audits. **Use hybridparser for audit and tooling only; production verification in this node uses the hybrid scheme APIs directly.**

**Obtaining data to audit in QuantumCoin:**

| Target | How to obtain | Notes |
|--------|----------------|-------|
| **Transaction signatures** | **(a)** Attach to a node and call **`eth.getTransactionSignature(txHash)`** (e.g. `dp attach IPC_ENDPOINT`, then `eth.getTransactionSignature("0x...")`). **(b)** Run **`dputil txnsig TXN_HASH`** with `DP_RAW_URL` set to a public RPC endpoint (e.g. `https://public.rpc.quantumcoinapi.com`). | Returns transaction hash, public key hex, signature hex, and a **hybridSignature** object (SchemeID, Message, PublicKeys, Signatures, Context, AdditionalData). The **Message** field is the signing hash (digest) of the transaction; use it with the extracted components for per-component verification as in the hybridparser README. |
| **Consensus messages** | Call **`proofofstake.getBlockConsensusDataWithSignatures(blockNumberInHex)`** (e.g. from `dp attach` or any RPC client). | Returns block consensus data including validator signatures for that block; these can be parsed and verified per component using the same hybridparser workflow. |

**Consensus signature mode.** The consensus **proposal packet (packet type 0)** uses the following signing rule: one in every 4,096 blocks starting from block **421,888** is signed in **Full** signature mode (all three components: Ed25519 + ML-DSA + SLH-DSA); all other proposal packets are signed in **Compact** mode (Ed25519 + ML-DSA only; SLH-DSA public key present but not signed). All other consensus packet types use Compact mode. Auditors can use `getBlockConsensusDataWithSignatures` and the block number to determine which mode was used for a given block’s proposal.

**Example: auditing transaction signatures with dputil**

Set `DP_RAW_URL` to the public RPC endpoint, then run `dputil.exe txnsig` with the transaction hash. Example transaction IDs you can use:

```bash
# Windows (cmd)
set DP_RAW_URL=https://public.rpc.quantumcoinapi.com
dputil.exe txnsig 0x5419cd6244d236c1e08bb2274122a3d10b4555c27d16977a68dc2d0b3d6d901a
dputil.exe txnsig 0x292a2405b2253989e99d4e9c7ee20975fa586a3f5d9fba90617398b93fb55734
dputil.exe txnsig 0x25cd8b25103f8d6b889afbe57d1cfd8473843c744a0a38e36a7549a75a9fd498
```

```powershell
# Windows (PowerShell)
$env:DP_RAW_URL = "https://public.rpc.quantumcoinapi.com"
.\dputil.exe txnsig 0x5419cd6244d236c1e08bb2274122a3d10b4555c27d16977a68dc2d0b3d6d901a
.\dputil.exe txnsig 0x292a2405b2253989e99d4e9c7ee20975fa586a3f5d9fba90617398b93fb55734
.\dputil.exe txnsig 0x25cd8b25103f8d6b889afbe57d1cfd8473843c744a0a38e36a7549a75a9fd498
```

```bash
# Linux/macOS
export DP_RAW_URL=https://public.rpc.quantumcoinapi.com
./dputil txnsig 0x5419cd6244d236c1e08bb2274122a3d10b4555c27d16977a68dc2d0b3d6d901a
./dputil txnsig 0x292a2405b2253989e99d4e9c7ee20975fa586a3f5d9fba90617398b93fb55734
./dputil txnsig 0x25cd8b25103f8d6b889afbe57d1cfd8473843c744a0a38e36a7549a75a9fd498
```

Each command prints the full transaction signature result (including `hybridSignature`) as JSON to the console for audit.

Using the above, auditors can obtain raw signature material (message, public keys, component signatures) and verify each component against FIPS 204, FIPS 205, and FIPS 186-5 (or the applicable pre-final drafts for schemes 1–2) with their chosen tooling.


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
