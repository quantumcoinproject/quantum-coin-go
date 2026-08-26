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

package common

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"math/big"
)

// Common big integers often used
var (
	Big1   = big.NewInt(1)
	Big2   = big.NewInt(2)
	Big3   = big.NewInt(3)
	Big0   = big.NewInt(0)
	Big32  = big.NewInt(32)
	Big256 = big.NewInt(256)
	Big257 = big.NewInt(257)
)

func SafeAddBigInt(x, y *big.Int) *big.Int {
	result := big.NewInt(0)
	result.Add(x, y)
	return result
}

// SafeSubBigInt returns x - y in a freshly allocated big.Int (x and y are never
// aliased or modified). It is a SIGNED subtraction: the result may be negative.
// For balances, deposits, gas or any other non-negative amount use
// SafeSubBigIntNonNegative instead, which refuses to underflow.
func SafeSubBigInt(x, y *big.Int) *big.Int {
	result := big.NewInt(0)
	result.Sub(x, y)
	return result
}

// SafeSubBigIntNonNegative returns x - y in a freshly allocated big.Int and panics if
// the result would be negative. It is meant for amounts that are non-negative by
// definition (balances, deposits, rewards, block counts): every caller is expected to
// have established x >= y already, so a panic here means state accounting has gone
// wrong and must not be silently persisted as a negative value.
func SafeSubBigIntNonNegative(x, y *big.Int) *big.Int {
	if x.Cmp(y) < 0 {
		panic(fmt.Sprintf("big.Int underflow: have=%s sub=%s", x.String(), y.String()))
	}
	result := big.NewInt(0)
	result.Sub(x, y)
	return result
}

func SafeMulBigInt(x, y *big.Int) *big.Int {
	result := big.NewInt(0)
	result.Mul(x, y)
	return result
}

func SafeDivBigInt(x, y *big.Int) *big.Int {
	result := big.NewInt(0)
	result.Div(x, y)
	return result
}

func SafeDivBigFloat(x, y *big.Float) *big.Float {
	result := big.NewFloat(0)
	result.Quo(x, y)
	return result
}

// SafePercentageOfBigInt returns what percentage of Y is X. Example, if y=1000 and x=100, then 10 is returned
func SafePercentageOfBigInt(x, y *big.Int) *big.Int {
	hundred := big.NewInt(100)
	return SafeDivBigInt(SafeMulBigInt(hundred, x), y)
}

// SafeRelativePercentageBigInt returns the proportionate percentage of the total value. For example, if total = 1000 and percentage = 10, then 100 is returned
func SafeRelativePercentageBigInt(total, percentage *big.Int) *big.Int {
	hundred := big.NewInt(100)
	return SafeDivBigInt(SafeMulBigInt(total, percentage), hundred)
}

func BigIntToHexString(val *big.Int) string {
	hexVal := (*hexutil.Big)(val)
	return hexVal.String()
}
