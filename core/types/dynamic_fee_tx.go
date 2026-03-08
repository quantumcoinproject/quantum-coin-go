// Copyright 2021 The go-ethereum Authors
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

package types

import (
	"math/big"

	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/wasm/core/types"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

type DynamicFeeTx struct {
	ChainID        *big.Int
	Nonce          uint64
	GasTipCap      *big.Int
	GasFeeCap      *big.Int
	Gas            uint64
	To             *common.Address `rlp:"nil"` // nil means contract creation
	Value          *big.Int
	Data           []byte
	Remarks        []byte
	SigningContext byte
	AccessList     AccessList

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
}

func getGasPrice() *big.Int {
	return big.NewInt(defaults.DEFAULT_PRICE / 10) //100 Q
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *DynamicFeeTx) copy() TxData {
	cpy := &DynamicFeeTx{
		Nonce: tx.Nonce,
		To:    tx.To, // TODO: copy pointed-to address
		Data:  common.CopyBytes(tx.Data),
		Gas:   tx.Gas,
		// These are copied below.
		AccessList:     make(AccessList, len(tx.AccessList)),
		Value:          new(big.Int),
		ChainID:        new(big.Int),
		GasTipCap:      new(big.Int),
		GasFeeCap:      new(big.Int),
		SigningContext: tx.SigningContext,
		V:              new(big.Int),
		R:              new(big.Int),
		S:              new(big.Int),
		Remarks:        common.CopyBytes(tx.Remarks),
	}
	copy(cpy.AccessList, tx.AccessList)
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	if tx.GasTipCap != nil {
		cpy.GasTipCap.Set(tx.GasTipCap)
	}
	if tx.GasFeeCap != nil {
		cpy.GasFeeCap.Set(tx.GasFeeCap)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	return cpy
}

func (tx *DynamicFeeTx) calcGasFee() *big.Int {
	defaultFee := getGasPrice()
	var expectedGasPrice *big.Int
	if tx.SigningContext == byte(crypto.SigningContextDefault) {
		expectedGasPrice = defaultFee
	} else if tx.SigningContext == byte(crypto.SigningContextLevel1) {
		expectedGasPrice = common.SafeMulBigInt(defaultFee, big.NewInt(defaults.SigningContextLevel1Multiplier))
	} else if tx.SigningContext == byte(crypto.SigningContextLevel2) {
		expectedGasPrice = common.SafeMulBigInt(defaultFee, big.NewInt(defaults.SigningContextLevel2Multiplier))
	} else {
		log.Warn("verifyFields", "SigningContext", tx.SigningContext)
		return nil
	}
	return expectedGasPrice
}

// accessors for innerTx.
func (tx *DynamicFeeTx) txType() byte           { return DynamicFeeTxType }
func (tx *DynamicFeeTx) chainID() *big.Int      { return tx.ChainID }
func (tx *DynamicFeeTx) protected() bool        { return true }
func (tx *DynamicFeeTx) accessList() AccessList { return tx.AccessList }
func (tx *DynamicFeeTx) data() []byte           { return tx.Data }
func (tx *DynamicFeeTx) gas() uint64            { return tx.Gas }
func (tx *DynamicFeeTx) gasFeeCap() *big.Int    { return tx.GasFeeCap }
func (tx *DynamicFeeTx) gasTipCap() *big.Int    { return tx.GasTipCap }
func (tx *DynamicFeeTx) gasPrice() *big.Int     { return tx.calcGasFee() }
func (tx *DynamicFeeTx) signingContext() byte {
	return tx.SigningContext
}
func (tx *DynamicFeeTx) maxGasTier() GasTier { return GAS_TIER_DEFAULT }
func (tx *DynamicFeeTx) value() *big.Int     { return tx.Value }
func (tx *DynamicFeeTx) nonce() uint64       { return tx.Nonce }
func (tx *DynamicFeeTx) to() *common.Address { return tx.To }
func (tx *DynamicFeeTx) verifyFields() bool {
	if tx.S != nil {
		signature := tx.S.Bytes()
		if len(signature) > 1 {
			algType := crypto.SignatureAlgorithmType(signature[0])
			if tx.SigningContext == byte(crypto.SigningContextDefault) {
				if algType != crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID && algType != crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID {
					log.Debug("verifyFields signing context algtype mismatch a", "tx.signingContext", tx.SigningContext, "algType", algType)
					return false
				}
			} else if tx.SigningContext == byte(crypto.SigningContextLevel1) {
				if algType != crypto.MLDSA_ED25519_SLHDSA_5_ID {
					log.Debug("verifyFields signing context algtype mismatch b", "tx.signingContext", tx.SigningContext, "algType", algType)
					return false
				}
			} else if tx.SigningContext == byte(crypto.SigningContextLevel2) {
				if algType != crypto.DILITHIUM_ED25519_SPHINCS_FULL_ID && algType != crypto.MLDSA_ED25519_SLHDSA_FULL_ID {
					log.Debug("verifyFields signing context algtype mismatch c", "tx.signingContext", tx.SigningContext, "algType", algType)
					return false
				}
			} else {
				log.Warn("verifyFields", "SigningContext", tx.SigningContext)
				return false
			}
		}
	}

	expectedGasPrice := tx.calcGasFee()
	if expectedGasPrice == nil {
		log.Debug("verifyFields nil expectedGasPrice", "tx.gasPrice()", tx.gasPrice(), "signingContext", tx.SigningContext)
		return false
	}

	if tx.gasPrice().Cmp(expectedGasPrice) != 0 {
		log.Debug("verifyFields", "tx.gasPrice()", tx.gasPrice(), "expectedGasFee", expectedGasPrice, "signingContext", tx.SigningContext)
		return false
	}

	if tx.maxGasTier() != GasTier(types.GAS_TIER_DEFAULT) {
		log.Debug("verifyFields", "tx.maxGasTier()", tx.maxGasTier())
		return false
	}
	return len(tx.Remarks) <= MAX_REMARKS_LENGTH
}

func (tx *DynamicFeeTx) rawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *DynamicFeeTx) setSignatureValues(chainID, v, r, s *big.Int) {
	tx.ChainID, tx.V, tx.R, tx.S = chainID, v, r, s
}

func (tx *DynamicFeeTx) remarks() []byte {
	return tx.Remarks
}

func NewDynamicFeeTransaction(chainId *big.Int, nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, signingContext crypto.SigningContext, data []byte, remarks []byte) *Transaction {
	tx := NewTx(&DynamicFeeTx{
		ChainID:        chainId,
		Nonce:          nonce,
		To:             to,
		Value:          amount,
		Data:           common.CopyBytes(data),
		Remarks:        common.CopyBytes(remarks),
		Gas:            gasLimit,
		SigningContext: byte(signingContext),
	})

	return tx
}
