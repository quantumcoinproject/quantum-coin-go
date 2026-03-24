# RLPx V2 Transport Protocol — Cryptographic Specification

## 1. Overview

RLPx V2 is a post-quantum hybrid authenticated key establishment and encrypted transport protocol for peer-to-peer communication. It provides:

- **IND-CCA2 key encapsulation** via a hybrid X25519 + ML-KEM-768 KEM.
- **Post-quantum signature authentication** via a hybrid Ed25519 + ML-DSA-44 signature scheme (with SLH-DSA-SHAKE-256f public key binding).
- **Authenticated encryption** via AES-256-GCM with HKDF-derived directional keys.
- **Explicit key confirmation** via HMAC-SHA3-256 Finished messages (TLS 1.3 style).
- **Forward secrecy** from ephemeral KEM key pairs generated per session.

The protocol structure closely follows the TLS 1.3 key schedule (RFC 8446), adapted to use post-quantum KEM for key establishment in place of (EC)DHE, and post-quantum hybrid signatures in place of classical signature schemes.

## 2. Cryptographic Primitives

### 2.1 Key Encapsulation Mechanism (KEM)

| Property | Value |
|---|---|
| Scheme | **X25519 + ML-KEM-768** (hybrid) |
| Implementation | Cloudflare CIRCL `kem/hybrid.X25519MLKEM768()` |
| Classical component | X25519 (Curve25519 ECDH) |
| Post-quantum component | ML-KEM-768 (FIPS 203, formerly CRYSTALS-Kyber-768) |
| IND-CCA2 security | 192-bit post-quantum, 128-bit classical |
| Shared secret size | 64 bytes (32 bytes X25519 ∥ 32 bytes ML-KEM-768, combined by CIRCL) |

The hybrid construction runs both X25519 and ML-KEM-768 in parallel and combines their shared secrets internally. An attacker must break **both** X25519 **and** ML-KEM-768 to recover the shared secret, providing security against both classical and quantum adversaries.

### 2.2 Signature Algorithm

| Property | Value |
|---|---|
| Scheme | **Hybrid Ed25519 + ML-DSA-44** (compact mode) |
| Implementation | CIRCL `sign/hybrideds`, via `cryptobase.SigAlg` |
| Classical component | Ed25519 |
| Lattice PQ component | ML-DSA-44 (FIPS 204, formerly CRYSTALS-Dilithium2) |
| Hash-based PQ component | SLH-DSA-SHAKE-256f (FIPS 205, key bound into signature hash) |
| Public key size | 1408 bytes (32 Ed25519 + 1312 ML-DSA-44 + 64 SLH-DSA) |
| Compact signature size | 2558 bytes |
| Combined signature (sig + pubkey) | 3970 bytes |

**Compact signing process:** A 40-byte random nonce is generated, then the hybrid hash `SHA3-512(nonce ∥ message ∥ SLH-DSA-pubkey)` is computed. Both Ed25519 and ML-DSA-44 sign this hash independently. The SLH-DSA public key is cryptographically bound into the signed data (enabling future "break-glass" full signatures on the same key material) but does not produce a signature in compact mode.

**Combined signature format:** The protocol uses a length-prefixed two-part envelope: `[2-byte total_len][2-byte sig_len][compact_signature][public_key]`. Verification extracts both parts, checks the embedded public key matches the expected key, then verifies both Ed25519 and ML-DSA-44 signatures.

### 2.3 Hash Functions

| Usage | Algorithm |
|---|---|
| Transcript hashing | **SHA3-256** (`golang.org/x/crypto/sha3`) |
| HKDF-Extract / HKDF-Expand | **HKDF with SHA3-256** (`golang.org/x/crypto/hkdf`) |
| Finished message MAC | **HMAC-SHA3-256** (`crypto/hmac` with `sha3.New256`) |
| Compact signature internal hash | **SHA3-512** (inside CIRCL `hybrideds`) |

### 2.4 Symmetric Encryption

| Property | Value |
|---|---|
| Algorithm | **AES-256-GCM** |
| Key size | 256 bits (32 bytes) |
| IV size | 96 bits (12 bytes) |
| Authentication tag | 128 bits (16 bytes, GCM default) |
| Nonce construction | IV ⊕ 64-bit sequence counter (RFC 8446 §5.3) |

## 3. Key Schedule

The key schedule follows TLS 1.3 (RFC 8446 §7.1), replacing `(EC)DHE` with the hybrid KEM shared secret and replacing SHA-256 with SHA3-256 throughout.

### 3.1 HKDF-Expand-Label

Labels are encoded as:

```
struct {
    uint16 length;
    uint8  label_length;
    opaque label<1..255>;     // "pqkem " + label_name
    uint8  hash_length;
    opaque hash_value<0..255>;
} HkdfLabel;
```

The label prefix `"pqkem "` is prepended to all label names (e.g., `"pqkem c hs traffic"`).

### 3.2 Derivation Steps

```
                          0 (32 zero bytes)
                               |
                               v
                    HKDF-Extract(key=0, salt=nil)
                               |
                               v
                         early_secret
                               |
                     Expand-Label("derived", SHA3-256(""), 32)
                               |
                               v
                         derived_secret
                               |
                               v
              HKDF-Extract(key=KEM_shared_secret, salt=derived_secret)
                               |
                               v
                       handshake_secret ──────────────────────────────────┐
                               |                                         |
           ┌───────────────────┴────────────────────┐                    |
           |                                        |                    |
  Expand-Label(                            Expand-Label(                 |
  "c hs traffic",                          "s hs traffic",              |
   transcript_hash, 32)                     transcript_hash, 32)        |
           |                                        |                    |
           v                                        v                    |
  client_hs_traffic_secret             server_hs_traffic_secret          |
           |                                        |                    |
     ┌─────┴─────┐                           ┌─────┴─────┐              |
     |           |                           |           |              |
 Expand(       Expand(                   Expand(       Expand(          |
 "key",32)     "iv",12)                  "key",32)     "iv",12)         |
     |           |                           |           |              |
     v           v                           v           v              |
 client_hs   client_hs                  server_hs   server_hs          |
   _key        _iv                        _key        _iv              |
                                                                        |
                                                                        |
                       Expand-Label("derived", SHA3-256(""), 32)  ◄─────┘
                               |
                               v
              HKDF-Extract(key=0, salt=derived_secret_2)
                               |
                               v
                        master_secret ─────────────────────────────────┐
                               |                                       |
           ┌───────────────────┴────────────────────┐                  |
           |                                        |                  |
  Expand-Label(                            Expand-Label(               |
  "c ap traffic",                          "s ap traffic",             |
   transcript_hash_2, 32)                   transcript_hash_2, 32)     |
           |                                        |                  |
           v                                        v                  |
  client_ap_traffic_secret             server_ap_traffic_secret        |
           |                                        |                  |
     ┌─────┴─────┐                           ┌─────┴─────┐            |
     |           |                           |           |            |
 Expand(       Expand(                   Expand(       Expand(        |
 "key",32)     "iv",12)                  "key",32)     "iv",12)       |
     |           |                           |           |            |
     v           v                           v           v            |
 client_ap   client_ap                  server_ap   server_ap         |
   _key        _iv                        _key        _iv
```

**`transcript_hash`** at handshake key derivation covers `ClientHello ∥ ServerHello`.  
**`transcript_hash_2`** at application key derivation covers `ClientHello ∥ ServerHello ∥ ServerVerify ∥ ClientVerify`.

The KEM shared secret is consumed immediately after `HKDF-Extract` and is zeroed from memory.

## 4. Handshake Protocol

The V2 handshake consists of six messages exchanged over five round trips:

```
Client                                                Server
──────                                                ──────

(1) ClientHello          ─────────────────────►
    {Version, KEM_PublicKey, Random[32]}
                                                      (2) ServerHello
                         ◄─────────────────────
                         {Version, KEM_Ciphertext, Random[32]}

    ── Derive handshake keys (both sides) ──
    transcript = ClientHello ∥ ServerHello (wire bytes)

                                                      (3) ServerVerify [encrypted]
                         ◄─────────────────────
                         {Sig_server(transcript_hash)}

(4) ClientVerify [encrypted]  ────────────────►
    {Sig_client(transcript_hash')}

    ── Derive application keys (both sides) ──
    transcript' = ClientHello ∥ ... ∥ ClientVerify

                                                      (5) ServerFinished [encrypted]
                         ◄─────────────────────
                         {HMAC(finished_key_s, transcript_hash'')}

(6) ClientFinished [encrypted]  ──────────────►
    {HMAC(finished_key_c, transcript_hash''')}

    ── Handshake complete: application data ──
```

### 4.1 ClientHello (message 1, plaintext)

The client:
1. Generates an **ephemeral** X25519 + ML-KEM-768 key pair via `GenerateKemKeyPair()`.
2. Fills 32 bytes of `ClientHelloRandomData` from `crypto/rand`.
3. Sends `{Version=1, ClientKemPublicKey, ClientHelloRandomData}` RLP-encoded with a 2-byte big-endian length prefix and 100–199 bytes of random padding.

### 4.2 ServerHello (message 2, plaintext)

The server:
1. Validates the client's KEM public key length against `scheme.PublicKeySize()`.
2. Calls `EncapsulateSecret(ClientKemPublicKey)` → `(ciphertext, shared_secret)`.
3. Validates that `shared_secret` has the expected length and is not all-zero.
4. Fills 32 bytes of `ServerHelloRandomData` from `crypto/rand`.
5. Sends `{Version=1, CipherText, ServerHelloRandomData}` with random padding.

### 4.3 Handshake Key Derivation

Both sides:
1. Compute `transcript = ClientHello_wire_bytes ∥ ServerHello_wire_bytes` (raw wire bytes excluding the 2-byte length prefix, to prevent RLP padding malleability).
2. Compute `transcript_hash = SHA3-256(transcript)`.
3. Derive handshake traffic secrets and AES-256-GCM keys/IVs via the key schedule (§3.2) using `transcript_hash` and the KEM `shared_secret`.
4. Zero the KEM shared secret, ephemeral private key bytes, and KEM state.

The client decapsulates: `shared_secret = DecapsulateSecret(ciphertext)`, with ciphertext length validation and all-zero shared secret rejection.

### 4.4 ServerVerify (message 3, encrypted under handshake keys)

The server:
1. Signs `transcript_hash` using its long-term signing private key: `Sig_server = Sign(SHA3-256(transcript), server_private_key)`.
2. The signature is the hybrid Ed25519 + ML-DSA-44 compact combined format (3970 bytes: signature + embedded public key).
3. Sends `ServerVerifyMessage{Signature, SignatureLen}` encrypted with the server handshake key.

The client:
1. Decrypts the message using the server handshake cipher.
2. Rejects empty signatures (`SignatureLen == 0`) and invalid lengths.
3. Extracts the server's public key from the combined signature via `PublicKeyBytesFromSignature`.
4. Compares the extracted public key against the expected `serverSigningPublicKey` (known a priori for the target node).
5. Verifies the signature via `SigAlg.Verify(pubkey, transcript_hash, signature)`.

### 4.5 ClientVerify (message 4, encrypted under handshake keys)

The client:
1. Appends the deterministic serialization of `ServerVerifyMessage` to the transcript.
2. Recomputes `transcript_hash' = SHA3-256(transcript')`.
3. Signs `transcript_hash'` using its long-term signing private key.
4. Sends `ClientVerifyMessage{Signature, SignatureLen}` encrypted with the client handshake key.

The server:
1. Decrypts and verifies the client's signature (same validation as §4.4).
2. Recovers the client's public key from the signature. The server does **not** check the recovered key against a pre-known identity at the RLPx layer; identity binding is performed by the upper p2p layer (see §7).
3. Derives application traffic secrets using `transcript_hash` over `ClientHello ∥ ServerHello ∥ ServerVerify ∥ ClientVerify`.

### 4.6 Finished Messages (messages 5–6, encrypted under handshake keys)

Finished messages provide explicit key confirmation, following TLS 1.3 (RFC 8446 §4.4.4):

**ServerFinished (message 5):**
1. Derive `finished_key_s = HKDF-Expand-Label(server_hs_traffic_secret, "s finished", "", 32)`.
2. Compute `verify_data = HMAC-SHA3-256(finished_key_s, transcript_hash)` where the transcript covers `ClientHello` through `ClientVerify`.
3. Send `FinishedMessage{VerifyData}` encrypted with the server handshake key.
4. The client verifies using `crypto/subtle.ConstantTimeCompare`.

**ClientFinished (message 6):**
1. The transcript is extended to include the `ServerFinished` message, so the client Finished cryptographically binds to the server Finished.
2. Derive `finished_key_c = HKDF-Expand-Label(client_hs_traffic_secret, "c finished", "", 32)`.
3. Compute `verify_data = HMAC-SHA3-256(finished_key_c, transcript_hash''')`.
4. Send `FinishedMessage{VerifyData}` encrypted with the client handshake key.
5. The server verifies using constant-time comparison.

After both Finished messages are verified, the handshake is complete. All handshake key material (handshake keys, IVs, traffic secrets, master secret, finished keys, transcript hash) is zeroed. Only the application AES-256-GCM cipher objects and application IVs remain live.

## 5. Record Layer (Application Data)

### 5.1 Record Format

Each record on the wire consists of:

```
┌──────────────────────┬──────────────────────────────────┐
│   Header (RLP)       │   AEAD Ciphertext                │
├──────────────────────┼──────────────────────────────────┤
│ 2-byte len prefix    │ Encrypted(EncryptedPayload) ∥ Tag│
│ MinorVersion = 2     │                                  │
│ RecordLength         │                                  │
│ AdditionalData[32]   │                                  │
└──────────────────────┴──────────────────────────────────┘
```

**Header fields** (RLP-encoded with 2-byte big-endian length prefix):
- `MinorVersion`: must be `2`.
- `RecordLength`: byte length of the AEAD ciphertext (plaintext + 16-byte GCM tag).
- `AdditionalData`: 32-byte deterministic AAD (see §5.2). The `Rest` field must be empty; trailing header data is rejected.

**EncryptedPayload** (RLP-encoded inside the AEAD plaintext):
- `PacketType`: `23` for application data, `21` for handshake.
- `Context`: application-level message code (e.g., devp2p message ID).
- `Fragment`: the actual message payload.
- `Rest`: must be empty; trailing data is rejected.

### 5.2 AAD Construction

AAD is deterministically constructed from the header fields and verified on decryption:

```
AAD[0..3] = MinorVersion (big-endian uint32)
AAD[4]    = 23 (PacketTypeApplicationData, fixed — does not leak whether the
                record is handshake or application data)
AAD[5..8] = RecordLength (big-endian uint32)
AAD[9..31] = 0x00
```

The receiver reconstructs AAD from the received `MinorVersion` and `RecordLength`, then compares it against the header's `AdditionalData`. Any mismatch aborts the connection.

### 5.3 Nonce Construction

Per RFC 8446 §5.3:

```
nonce[0..11] = IV[0..11] ⊕ (0x0000_0000 ∥ sequence_number[0..7])
```

The 64-bit sequence number is XORed into the last 8 bytes of the 12-byte IV. Sequence numbers start at 0 and increment monotonically per direction per epoch (handshake / application). A nonce reuse guard rejects `sequence_number == 2^64 - 1`.

Separate sequence counters and IVs are maintained for:
- Client → Server handshake
- Server → Client handshake
- Client → Server application
- Server → Client application

### 5.4 Record Size Limits

| Record type | Maximum ciphertext size |
|---|---|
| Handshake | 16 KiB (16,384 bytes) |
| Application data | 64 MiB (67,108,864 bytes) |

These limits are enforced **before** reading the ciphertext from the network, preventing allocation-based denial-of-service before AEAD authentication.

### 5.5 Compression

The V2 record layer does **not** apply compression (neither gzip nor Snappy) at the transport level. The `EncryptedPayload.Fragment` is encrypted as-is.

The `Conn.SetSnappy` API exists for the devp2p protocol layer above RLPx but is orthogonal to the V2 record encryption. Legacy (V1) read paths include gzip decompression with a size limit for backward compatibility; this is not part of the V2 protocol.

The `common.go` file defines `decompressWithLimit` and `maybeDecompress` with size caps (96 MiB for V2, 128 MiB for legacy) that reject decompressed output exceeding the configured maximum, providing protection against decompression bombs (zip bombs). These functions are available for use by legacy code paths; V2 does not invoke them in the standard record layer.

## 6. Serialization

### 6.1 RLP Framing

All handshake messages and record headers are RLP-encoded (Recursive Length Prefix). Each serialized message is preceded by a 2-byte big-endian length prefix indicating the RLP payload size.

### 6.2 Random Padding

`Serialize()` appends 100–199 bytes of random padding (sampled uniformly via `crypto/rand`) to each message. This provides traffic analysis resistance by obscuring true message sizes.

### 6.3 Deterministic Serialization

`SerializeDeterministic()` (with `padLen=0`) is used for transcript entries to ensure both sides compute identical transcript hashes. This prevents padding malleability from causing transcript divergence.

### 6.4 Transcript Construction

The transcript is built by concatenation of **raw wire bytes** (excluding the 2-byte length prefix for the messages sent by the local party, including them for received messages). This prevents RLP re-serialization differences from causing transcript divergence. Specifically:
- `ClientHello` wire bytes are taken excluding the length prefix on the sending side, and as-is from deserialization on the receiving side.
- `ServerHello` wire bytes follow the same convention.
- `ServerVerify` and `ClientVerify` use deterministic serialization (zero padding) for the transcript.
- `ServerFinished` uses deterministic serialization for the transcript.

## 7. Identity Binding

The RLPx V2 layer is intentionally **identity-agnostic** — it authenticates the peer's signing key but does not enforce identity policy. Identity binding is performed by the p2p layer above:

1. **Key recovery:** After the handshake, the server obtains the client's public key via `ClientSigningPublicKey()`. The client's expected server key is provided at connection establishment.
2. **Node ID computation:** The upper layer computes `node_id = Keccak-256(SerializePublicKey(recovered_key))`.
3. **Protocol handshake:** Over the now-encrypted channel, the peer sends its public key bytes.
4. **Identity check:** The upper layer verifies `Keccak-256(claimed_pubkey) == node_id`. Failure results in connection rejection.

This two-layer design keeps the transport layer generic while the p2p layer enforces that only the holder of the private key corresponding to a given node ID can claim that identity.

## 8. Secret Zeroization

The protocol implements defense-in-depth memory hygiene:

| Material | Zeroed when |
|---|---|
| KEM shared secret | Immediately after HKDF-Extract into handshake secret |
| Ephemeral KEM private key | Immediately after key derivation; also zeroed on handshake failure |
| KEM state (CIRCL internal) | `Clean()` called after key derivation |
| Handshake traffic secrets | After Finished exchange (`ZeroPostHandshakeKeyMaterial`) |
| Handshake keys and IVs | After Finished exchange |
| Handshake cipher objects | Set to nil after Finished exchange |
| Master secret | After Finished exchange |
| Application key byte slices | Zeroed after AES cipher objects are instantiated |
| Transcript hash | After Finished exchange |
| Finished keys | Zeroed via `defer` immediately after use |
| All secrets (on failure) | `ZeroSecrets()` called in deferred error handler |

**Known limitation:** Go's `crypto/aes` stores an expanded AES key schedule internally with no API to zero it. Setting cipher fields to nil drops the reference, but the 176-byte expanded key persists on the heap until garbage collection. The raw key slices are properly zeroed.

## 9. Concurrency Model

- **Handshake:** Protected by a mutex (`mutex`); `PerformHandshake()` is not reentrant. The `handshakeDone` atomic flag prevents duplicate handshakes.
- **Writes:** A `writeMutex` is acquired unconditionally for every `WriteEncrypted` call, regardless of packet type, preventing nonce reuse from concurrent writes.
- **Reads:** A `readMutex` is acquired for application data reads. Handshake reads are single-threaded (called only from within `PerformHandshake`).
- **Application data gating:** `WriteEncrypted` and `ReadAndDecrypt` for `PacketTypeApplicationData` reject calls before the handshake is complete.

## 10. Security Properties

| Property | Mechanism |
|---|---|
| Quantum-resistant key establishment | Hybrid X25519 + ML-KEM-768 (IND-CCA2) |
| Quantum-resistant authentication | Hybrid Ed25519 + ML-DSA-44 signatures |
| Forward secrecy | Ephemeral KEM key pair per session |
| Mutual authentication | Both parties sign the transcript |
| Key confirmation | HMAC-SHA3-256 Finished messages |
| Replay protection | Monotonic per-direction sequence numbers |
| Nonce reuse prevention | Write mutex + IV⊕counter construction + overflow guard |
| Record integrity | AES-256-GCM AEAD (128-bit tag) |
| Record confidentiality | AES-256-GCM |
| Transcript binding | All signatures and Finished MACs cover the running transcript hash |
| Ciphertext/key length validation | Explicit checks before KEM operations |
| All-zero shared secret rejection | Constant-time check post-KEM |
| Traffic analysis resistance | Random padding on serialized messages |
| Allocation DoS mitigation | Record size limits enforced before reading ciphertext |
| AAD determinism | AAD is reconstructed and compared, not trusted from wire |
| Packet type concealment | AAD uses fixed content type regardless of actual packet type |
| Empty/malformed signature rejection | Explicit `SignatureLen == 0` and bounds checks |
| Trailing data rejection | `Rest` field checks on Header, EncryptedPayload, and FinishedMessage |
| Constant-time Finished comparison | `crypto/subtle.ConstantTimeCompare` |
