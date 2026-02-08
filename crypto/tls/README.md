# crypto/tls — PQC TLS support

This package provides **hybrid PQC** (Ed25519 + ML-DSA + SLH-DSA) support for X.509 certificates and TLS 1.3 CertificateVerify.

## Contents

1. **Signer and X.509 (PQC cert)**  
   - `HybridSigner` implements `crypto.Signer` and can sign certificate TBSCertificate and TLS 1.3 CertificateVerify messages.  
   - `PublicKey` holds the raw hybrid public key.  
   - `CreateCertificate` builds a self-signed X.509 certificate (custom OID and SubjectPublicKeyInfo).  
   - `ParseCertificatePQC` / `VerifyCertificatePQC` parse and verify PQC certificates.

2. **TLS 1.3 support**  
   - **In this package:** Helpers for the CertificateVerify message format (`SignedMessageTLS13`, `TranscriptHashSHA256`). The hybrid signer signs a **32-byte digest**: use SHA-256 of the TLS signed message, then `Sign(rand, digest, nil)`.  
   - **Full TLS stack:** The Go standard library `crypto/tls` does not support this key type (`signatureSchemesForPublicKey` returns `nil`). To run a TLS 1.3 server or client with a PQC certificate you need a **fork** of `crypto/tls` that adds a `SignatureScheme` for the hybrid algorithm and handles `*PublicKey` in auth and handshake.  
   - E2E test `TestE2E_StdlibTLSRejectsPQCKey` confirms that the stdlib handshake fails with our cert; `TestE2E_TLS13CertificateVerifySignAndVerify` validates sign/verify of the CertificateVerify payload (hash-then-sign with SHA-256).

## Usage

- **Generate key and signer:** use `pqchelpereddsamldsaslhdsa.GenerateKey()`, then `NewHybridSigner(secretKey)`.  
- **Create cert:** `CreateCertificate(DefaultCertTemplate(validity), signer)`.  
- **TLS CertificateVerify:** build the signed message with `SignedMessageTLS13`, hash with SHA-256, then `signer.Sign(rand, digest[:], nil)`; verify with `Verify(pub, digest, sig)`.

## Tests

- Unit: signer (Sign/Verify, sizes), cert (create, parse, verify, PEM).  
- E2E: full cert create/verify, TLS 1.3 CertificateVerify sign/verify, and stdlib TLS with PQC cert (expect handshake failure unless the stack is patched).
