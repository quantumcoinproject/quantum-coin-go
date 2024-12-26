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
	"encoding/json"
	"errors"
	"math/big"

	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
)

// txJSON is the JSON representation of transactions.
type txJSON struct {
	Type hexutil.Uint64 `json:"type"`

	// Common transaction fields:
	Nonce      *hexutil.Uint64 `json:"nonce"`
	GasPrice   *hexutil.Big    `json:"gasPrice"`
	Gas        *hexutil.Uint64 `json:"gas"`
	MaxGasTier *hexutil.Uint64 `json:"maxGasTier"`
	Value      *hexutil.Big    `json:"value"`
	Data       *hexutil.Bytes  `json:"input"`
	V          *hexutil.Big    `json:"v"`
	R          *hexutil.Big    `json:"r"`
	S          *hexutil.Big    `json:"s"`
	To         *common.Address `json:"to"`
	Remarks    *hexutil.Bytes  `json:"remarks"`

	// Access list transaction fields:
	ChainID    *hexutil.Big `json:"chainId,omitempty"`
	AccessList *AccessList  `json:"accessList,omitempty"`

	// Only used for encoding:
	Hash common.Hash `json:"hash"`

	VBlob []byte `json:"vBlob"`
	RBlob []byte `json:"rBlob"`
	SBlob []byte `json:"sBlob"`
}

type txJSONinner struct {
	Type hexutil.Uint64 `json:"type"`

	// Common transaction fields:
	Nonce      *hexutil.Uint64 `json:"nonce"`
	GasPrice   *hexutil.Big    `json:"gasPrice"`
	Gas        *hexutil.Uint64 `json:"gas"`
	MaxGasTier *hexutil.Uint64 `json:"maxGasTier"`
	Value      *hexutil.Big    `json:"value"`
	Data       *hexutil.Bytes  `json:"input"`
	To         *common.Address `json:"to"`
	Remarks    *hexutil.Bytes  `json:"remarks"`

	// Access list transaction fields:
	ChainID    *hexutil.Big `json:"chainId,omitempty"`
	AccessList *AccessList  `json:"accessList,omitempty"`

	// Only used for encoding:
	Hash common.Hash `json:"hash"`

	VBlob []byte `json:"vBlob"`
	RBlob []byte `json:"rBlob"`
	SBlob []byte `json:"sBlob"`
}

// MarshalJSON marshals as JSON with a hash.
func (t *Transaction) MarshalJSON() ([]byte, error) {
	var enc txJSON
	// These are set for all tx types.
	enc.Hash = t.Hash()
	enc.Type = hexutil.Uint64(t.Type())

	// Other fields are set conditionally depending on tx type.
	switch tx := t.inner.(type) {
	case *DefaultFeeTx:
		if tx.verifyFields() == false {
			return nil, errors.New("verify fields failed")
		}
		enc.ChainID = (*hexutil.Big)(tx.ChainID)
		enc.AccessList = &tx.AccessList
		enc.Nonce = (*hexutil.Uint64)(&tx.Nonce)
		enc.Gas = (*hexutil.Uint64)(&tx.Gas)
		enc.MaxGasTier = (*hexutil.Uint64)(&tx.MaxGasTier)
		enc.Value = (*hexutil.Big)(tx.Value)
		enc.Data = (*hexutil.Bytes)(&tx.Data)
		enc.Remarks = (*hexutil.Bytes)(&tx.Remarks)
		enc.To = t.To()
		enc.V = (*hexutil.Big)(tx.V)
		enc.R = (*hexutil.Big)(tx.R)
		enc.S = (*hexutil.Big)(tx.S)
		enc.VBlob = tx.V.Bytes()
		enc.RBlob = tx.R.Bytes()
		enc.SBlob = tx.S.Bytes()
	}
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (t *Transaction) UnmarshalJSON(input []byte) error {
	var dec txJSONinner
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	// Decode / verify fields according to transaction type.
	var inner TxData
	switch dec.Type {
	case DefaultFeeTxType:
		var itx DefaultFeeTx

		inner = &itx

		// Now set the inner transaction.
		t.setDecoded(inner, 0)

		// Access list is optional for now.
		if dec.AccessList != nil {
			itx.AccessList = *dec.AccessList
		}

		if dec.ChainID == nil {
			return errors.New("missing required field 'chainId' in transaction")
		}
		itx.ChainID = (*big.Int)(dec.ChainID)

		if dec.To != nil {
			itx.To = dec.To
		}

		if dec.Nonce == nil {
			return errors.New("missing required field 'nonce' in transaction")
		}
		itx.Nonce = uint64(*dec.Nonce)

		if dec.Gas == nil {
			return errors.New("missing required field 'gas' for txdata")
		}
		itx.Gas = uint64(*dec.Gas)

		itx.Value = (*big.Int)(dec.Value)
		if dec.Data == nil {
			return errors.New("missing required field 'input' in transaction")
		}
		itx.Data = *dec.Data
		if dec.Remarks != nil {
			itx.Remarks = *dec.Data
			if len(itx.Remarks) > MAX_REMARKS_LENGTH {
				return errors.New("verify remarks failed")
			}
		}

		if dec.MaxGasTier == nil {
			return errors.New("missing required field 'maxGasTier' in transaction") //todo: fill
		}
		maxGasTier := int64(*dec.MaxGasTier)

		if big.NewInt(maxGasTier).Cmp(GAS_TIER_DEFAULT_PRICE) == 0 {
			itx.MaxGasTier = GAS_TIER_DEFAULT
		} else if big.NewInt(maxGasTier).Cmp(GAS_TIER_2x_PRICE) == 0 {
			itx.MaxGasTier = GAS_TIER_2X
		} else if big.NewInt(maxGasTier).Cmp(GAS_TIER_5x_PRICE) == 0 {
			itx.MaxGasTier = GAS_TIER_5X
		} else if big.NewInt(maxGasTier).Cmp(GAS_TIER_10x_PRICE) == 0 {
			itx.MaxGasTier = GAS_TIER_10X
		} else {
			return errors.New("invalid max gas tier")
		}

		if dec.VBlob == nil {
			return errors.New("missing required field 'VBlob' in transaction")
		} else {
			itx.V = big.NewInt(1).SetBytes(dec.VBlob)
		}

		if dec.RBlob == nil {
			return errors.New("missing required field 'RBlob' in transaction")
		} else {
			itx.R = big.NewInt(1).SetBytes(dec.RBlob)
		}

		if dec.SBlob == nil {
			return errors.New("missing required field 'SBlob' in transaction")
		} else {
			itx.S = big.NewInt(1).SetBytes(dec.SBlob)
		}
	default:
		return ErrTxTypeNotSupported
	}

	// TODO: check hash here?
	return nil
}
