// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package crypto

import (
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
	"golang.org/x/crypto/sha3"
)

// SignatureAlgorithmType identifies which hybrid post-quantum (PQC) signature scheme is used.
// QuantumCoin uses NIST-standardized PQC in hybrid mode: Ed25519 + a NIST PQC signature.
// ML-DSA (FIPS 204) and SLH-DSA (FIPS 205) are the primary NIST PQC standards used.
// The Dilithium/SPHINCS+ variants map to the same NIST PQC standardization track.
// This hybrid approach provides quantum-resistance while maintaining classical security.
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

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	h := sha3.NewLegacyKeccak256()
	for _, b := range data {
		h.Write(b)
	}
	return h.Sum(nil)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	h.SetBytes(Keccak256(data...))
	return h
}

// CreateAddress creates an ethereum address given the bytes and the nonce
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce})
	return common.BytesToAddress(Keccak256(data)[:])
}

// CreateAddress2 creates an ethereum address given the address bytes, initial
// contract code hash and a salt.
func CreateAddress2(b common.Address, salt [common.HashLength]byte, inithash []byte) common.Address {
	return common.BytesToAddress(Keccak256([]byte{0xff}, b.Bytes(), salt[:], inithash)[:])
}

func PublicKeyBytesToAddress(pubKey []byte) common.Address {
	var a common.Address
	a.SetBytes(Keccak256(pubKey))
	return a
}
