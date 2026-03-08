package types

type SignatureAlgorithmType byte

const DILITHIUM_ED25519_SPHINCS_COMPACT_ID SignatureAlgorithmType = 1 // Hybrid: Ed25519 + NIST Dilithium + SPHINCS+ (compact)

const DILITHIUM_ED25519_SPHINCS_FULL_ID SignatureAlgorithmType = 2 // Hybrid: Ed25519 + NIST Dilithium + SPHINCS+ (full)

const MLDSA_ED25519_SLHDSA_COMPACT_ID SignatureAlgorithmType = 3 // Hybrid: Ed25519 + NIST ML-DSA (FIPS 204) + SLH-DSA (FIPS 205) compact

const MLDSA_ED25519_SLHDSA_FULL_ID SignatureAlgorithmType = 4 // Hybrid: Ed25519 + NIST ML-DSA (FIPS 204) + SLH-DSA (FIPS 205) full

const MLDSA_ED25519_SLHDSA_5_ID SignatureAlgorithmType = 5 // Hybrid: Ed25519 + NIST ML-DSA (FIPS 204) + SLH-DSA (FIPS 205) level 5 (maximum security)

type SigningContext byte

const SigningContextDefault SigningContext = 0 //DILITHIUM_ED25519_SPHINCS_COMPACT_ID, MLDSA_ED25519_SLHDSA_COMPACT_ID
const SigningContextLevel1 SigningContext = 1  //MLDSA_ED25519_SLHDSA_FULL_ID
const SigningContextLevel2 SigningContext = 2  //MLDSA_ED25519_SLHDSA_FULL_ID
