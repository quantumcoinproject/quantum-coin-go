// Copyright 2016 The go-ethereum Authors
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

package abi

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
)

var (
	errBadBool = errors.New("abi: improperly encoded boolean value")
	// Upstream cefc0fa00: integer decoding is stringent — an encoded word that
	// does not fit the declared type is rejected instead of silently truncated.
	errBadUint8  = errors.New("abi: improperly encoded uint8 value")
	errBadUint16 = errors.New("abi: improperly encoded uint16 value")
	errBadUint32 = errors.New("abi: improperly encoded uint32 value")
	errBadUint64 = errors.New("abi: improperly encoded uint64 value")
	errBadInt8   = errors.New("abi: improperly encoded int8 value")
	errBadInt16  = errors.New("abi: improperly encoded int16 value")
	errBadInt32  = errors.New("abi: improperly encoded int32 value")
	errBadInt64  = errors.New("abi: improperly encoded int64 value")
)

// formatSliceString formats the reflection kind with the given slice size
// and returns a formatted string representation.
func formatSliceString(kind reflect.Kind, sliceSize int) string {
	if sliceSize == -1 {
		return fmt.Sprintf("[]%v", kind)
	}
	return fmt.Sprintf("[%d]%v", sliceSize, kind)
}

// sliceTypeCheck checks that the given slice can by assigned to the reflection
// type in t.
func sliceTypeCheck(t Type, val reflect.Value) error {
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return typeErr(formatSliceString(t.GetType().Kind(), t.Size), val.Type())
	}

	if t.T == ArrayTy && val.Len() != t.Size {
		return typeErr(formatSliceString(t.Elem.GetType().Kind(), t.Size), formatSliceString(val.Type().Elem().Kind(), val.Len()))
	}

	if t.Elem.T == SliceTy || t.Elem.T == ArrayTy {
		if val.Len() > 0 {
			return sliceTypeCheck(*t.Elem, val.Index(0))
		}
	}

	if val.Type().Elem().Kind() != t.Elem.GetType().Kind() {
		return typeErr(formatSliceString(t.Elem.GetType().Kind(), t.Size), val.Type())
	}
	return nil
}

// typeCheck checks that the given reflection value can be assigned to the reflection
// type in t.
func typeCheck(t Type, value reflect.Value) error {
	if t.T == SliceTy || t.T == ArrayTy {
		return sliceTypeCheck(t, value)
	}

	// Check base type validity. Element types will be checked later on.
	if t.GetType().Kind() != value.Kind() {
		return typeErr(t.GetType().Kind(), value.Kind())
	} else if t.T == FixedBytesTy && t.Size != value.Len() {
		return typeErr(t.GetType(), value.Type())
	} else {
		return nil
	}

}

// typeErr returns a formatted type casting error.
func typeErr(expected, got interface{}) error {
	return fmt.Errorf("abi: cannot use %v as type %v as argument", got, expected)
}

type Error struct {
	Name   string
	Inputs Arguments
	str    string

	// Sig contains the string signature according to the ABI spec.
	// e.g. error foo(uint32 a, int b) = "foo(uint32,int256)"
	// Please note that "int" is substitute for its canonical representation "int256"
	Sig string

	// ID returns the canonical representation of the error's signature used by the
	// abi definition to identify event names and types.
	ID common.Hash
}

func NewError(name string, inputs Arguments) Error {
	// sanitize inputs to remove inputs without names
	// and precompute string and sig representation.
	names := make([]string, len(inputs))
	types := make([]string, len(inputs))
	for i, input := range inputs {
		if input.Name == "" {
			inputs[i] = Argument{
				Name:    fmt.Sprintf("arg%d", i),
				Indexed: input.Indexed,
				Type:    input.Type,
			}
		} else {
			inputs[i] = input
		}
		// string representation
		names[i] = fmt.Sprintf("%v %v", input.Type, inputs[i].Name)
		if input.Indexed {
			names[i] = fmt.Sprintf("%v indexed %v", input.Type, inputs[i].Name)
		}
		// sig representation
		types[i] = input.Type.String()
	}

	str := fmt.Sprintf("error %v(%v)", name, strings.Join(names, ", "))
	sig := fmt.Sprintf("%v(%v)", name, strings.Join(types, ","))
	id := common.BytesToHash(crypto.Keccak256([]byte(sig)))

	return Error{
		Name:   name,
		Inputs: inputs,
		str:    str,
		Sig:    sig,
		ID:     id,
	}
}

func (e *Error) String() string {
	return e.str
}

func (e *Error) Unpack(data []byte) (interface{}, error) {
	if len(data) < 4 {
		return "", errors.New("invalid data for unpacking")
	}
	if !bytes.Equal(data[:4], e.ID[:4]) {
		return "", errors.New("invalid data for unpacking")
	}
	return e.Inputs.Unpack(data[4:])
}
