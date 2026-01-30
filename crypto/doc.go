// Package crypto provides cryptographic primitives and signature algorithms for QuantumCoin.
//
// QuantumCoin is a quantum-resistant blockchain that uses post-quantum cryptography (PQC) 
// in hybrid mode. All production signatures combine a classical algorithm (Ed25519) 
// with NIST-standardized post-quantum algorithms to ensure long-term security against 
// both classical and quantum computers (Shor's algorithm).
//
// Supported NIST PQC Standards:
//   - ML-DSA (Module-Lattice-Based Digital Signature Standard, FIPS 204)
//   - SLH-DSA (Stateless Hash-Based Digital Signature Standard, FIPS 205)
//   - Dilithium and SPHINCS+ (NIST PQC Standardization Round 3/4)
//
// The hybrid approach (e.g., Ed25519 + ML-DSA) provides defense-in-depth: the system 
// remains secure as long as at least one of the combined algorithms is not broken.
//
// Subpackages:
//   - hybrideds, hybridedsfull: Ed25519 + Dilithium + SPHINCS+ hybrid
//   - hybrideddsamldsaslhdsa, hybrideddsamldsaslhdsafull, hybrideddsamldsaslhdsa5: Ed25519 + ML-DSA + SLH-DSA hybrid
//   - cryptobase: Centralized routing for algorithm selection, signing, and verification.
package crypto
