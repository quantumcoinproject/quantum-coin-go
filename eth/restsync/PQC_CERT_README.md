# Enabling Hybrid PQC Certs (pqchelpereddsamldsaslhdsa) for REST Sync TLS

To use the hybrid PQC signing algorithm from `crypto/pqchelpereddsamldsaslhdsa` (Ed25519 + ML-DSA + SLH-DSA) for the REST sync TLS certificate, the following is needed.

## 1. X.509 certificate creation

**Current:** `eth/restsync/cert.go` uses ECDSA P-256 and `x509.CreateCertificate`, which only supports standard key types (RSA, ECDSA, Ed25519).

**To use hybrid PQC:**

- **Option A – Implement `crypto.Signer` for the hybrid key**  
  Add a type that holds the hybrid secret key (from `pqchelpereddsamldsaslhdsa.GenerateKey()`) and implements:
  - `Public() crypto.PublicKey` (return a value that can be encoded in the cert; see below)
  - `Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error)` → call `pqchelpereddsamldsaslhdsa.Sign(secretKey, digest)`

  Even with this, **`x509.CreateCertificate` will not accept it** today: it uses a type switch and only marshals/encodes RSA, ECDSA, and Ed25519 (SubjectPublicKeyInfo and signature algorithm OID). So you also need one of:

  - **Option A1:** Extend Go’s `crypto/x509` (or a fork) to support the hybrid algorithm: define an OID for the hybrid signature algorithm, add encoding of the hybrid public key in SubjectPublicKeyInfo, and in the signing path accept your `crypto.Signer` and use it to produce the cert signature.
  - **Option A2:** Build the certificate manually: construct the TBSCertificate (DER), hash it (e.g. SHA-256), sign with `pqchelpereddsamldsaslhdsa.Sign(secretKey, hash)`, then assemble the Certificate (TBSCertificate + AlgorithmIdentifier + signature). You would need a defined OID and encoding for the hybrid public key in the cert (draft/standard or custom).

- **Option B – Keep ECDSA cert, add PQC elsewhere**  
  Keep the current ECDSA-based REST sync cert and use hybrid PQC only for application-layer or out-of-band authentication, not for the TLS certificate itself.

## 2. TLS stack support

**Current:** `crypto/tls` uses the certificate’s private key to sign the TLS 1.3 **CertificateVerify** message. It only handles **RSA**, **ECDSA**, and **Ed25519** (via type assertions / specific branches). It does not use a generic `crypto.Signer` for unknown key types.

**To use hybrid PQC in TLS:**

- Extend (or fork) Go’s **`crypto/tls`** so that it:
  - Recognizes the hybrid PQC certificate (e.g. by signature algorithm OID or key type).
  - For the server: when building CertificateVerify, uses your hybrid private key (e.g. via a `crypto.Signer` implementation) to sign the handshake transcript.
  - For the client: when verifying the server cert and CertificateVerify, parses the hybrid public key from the cert and verifies the signature with `pqchelpereddsamldsaslhdsa.Verify(pubKey, digest, signature)`.

Until `crypto/tls` is extended this way, a cert that uses the hybrid PQC algorithm cannot be used as the TLS leaf cert for the REST sync server in the standard Go TLS stack.

## 3. Summary checklist

| Component | What’s needed |
|----------|----------------|
| **Key generation** | Use `pqchelpereddsamldsaslhdsa.GenerateKey()` (or `GenerateKeyWithSeed`) instead of ECDSA in `ensureCertKey`; persist the returned `publicKey`/`secretKey` bytes (e.g. in the data dir). |
| **crypto.Signer** | Implement a type that wraps the hybrid secret key and implements `Public()` and `Sign(..., digest, ...)` using `pqchelpereddsamldsaslhdsa.Sign`. |
| **X.509 cert** | Either extend `crypto/x509` (OID + SubjectPublicKeyInfo encoding for hybrid key + use of your Signer) or build the cert manually and sign the TBSCertificate with the hybrid signer. |
| **TLS server** | Extend `crypto/tls` so the server uses your hybrid Signer for the CertificateVerify signature when the selected cert is hybrid PQC. |
| **TLS client** | Extend `crypto/tls` (and possibly `crypto/x509`) so the client can parse the hybrid public key from the cert and verify CertificateVerify with `pqchelpereddsamldsaslhdsa.Verify`. |

## 4. Minimal code hook (cert side)

To prepare for a future hybrid PQC cert in restsync without changing Go’s standard library yet, you can:

1. Add a **`crypto.Signer` implementation** in this repo that wraps `pqchelpereddsamldsaslhdsa` (holds `secretKey []byte`, `Public()` returns a struct holding `publicKey []byte`, `Sign` calls `pqchelpereddsamldsaslhdsa.Sign(secretKey, digest)`).
2. In **`restsync/cert.go`**, add a code path (e.g. behind a build tag or config) that:
   - Generates and stores the hybrid key (e.g. `restsync-pqc.key` / `.crt`) in the data dir.
   - For now, either:
     - Build the cert manually (TBSCertificate + hybrid sign) and write PEM, or  
     - Continue generating an ECDSA cert for TLS and keep the PQC key for non-TLS use only.

Once `crypto/x509` and `crypto/tls` support the hybrid algorithm (upstream or fork), you would switch the REST sync server to load the PQC cert and key and pass the hybrid `crypto.Signer` as the TLS private key.
