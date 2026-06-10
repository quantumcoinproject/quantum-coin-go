// Copyright 2019 The go-ethereum Authors
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

package proofofstake

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/core"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/conversion"
	"github.com/quantumcoinproject/quantum-coin-go/systemcontracts/staking"
)

func TestTxnFee(t *testing.T) {

	txnFeeTotal := common.SafeMulBigInt(big.NewInt(defaults.DEFAULT_PRICE), new(big.Int).SetUint64(21000))
	burnAmount, txnFeeRewards := calculateTxnFeeSplitCoins(txnFeeTotal)
	log.Info("TestTxnFee1", "burnAmount", burnAmount, "txnFeeRewards", txnFeeRewards, "txnFeeTotal", txnFeeTotal)

	if burnAmount.String() != "499999999999999800000" {
		t.Fatalf("failed1")
	}

	if txnFeeRewards.String() != "499999999999999800000" {
		t.Fatalf("failed2")
	}

	blockRewards := GetReward(big.NewInt(core.TXN_FEE_CUTTOFF_BLOCK))
	totalRewards := common.SafeAddBigInt(blockRewards, txnFeeRewards)
	log.Info("TestTxnFee2", "blockRewards", blockRewards, "totalRewards", totalRewards, "txnFeeRewards", txnFeeRewards)
	if totalRewards.String() != "951793759512937627532754" {
		t.Fatalf("failed2.1")
	}

	txnFeeTotal = common.SafeMulBigInt(big.NewInt(21000*10), types.GetDefaultGasPrice())
	burnAmount, txnFeeRewards = calculateTxnFeeSplitCoins(txnFeeTotal)
	log.Info("TestTxnFee3", "burnAmount", burnAmount, "txnFeeRewards", txnFeeRewards, "txnFeeTotal", txnFeeTotal)

	if burnAmount.String() != "4999999999999998000000" {
		t.Fatalf("failed3")
	}

	if txnFeeRewards.String() != "4999999999999998000000" {
		t.Fatalf("failed4")
	}

	txnFeeTotal = common.SafeMulBigInt(big.NewInt((21000*4)-1), types.GetDefaultGasPrice())
	burnAmount, txnFeeRewards = calculateTxnFeeSplitCoins(txnFeeTotal)
	log.Info("TestTxnFee4", "burnAmount", burnAmount, "txnFeeRewards", txnFeeRewards, "txnFeeTotal", txnFeeTotal)

	if burnAmount.String() != "1999976190476189676200" {
		t.Fatalf("failed5")
	}

	if txnFeeRewards.String() != "1999976190476189676200" {
		t.Fatalf("failed6")
	}
}

func TestTxnFee_Simple(t *testing.T) {
	txnFeeTotal := common.SafeMulBigInt(big.NewInt(21000*10), types.GetDefaultGasPrice())
	burnAmount, txnFeeRewards := calculateTxnFeeSplitCoins(txnFeeTotal)
	log.Info("TestTxnFee", "burnAmount", burnAmount, "txnFeeRewards", txnFeeRewards, "txnFeeTotal", txnFeeTotal)

	if burnAmount.String() != "4999999999999998000000" {
		t.Fatalf("failed3")
	}

	if txnFeeRewards.String() != "4999999999999998000000" {
		t.Fatalf("failed4")
	}

	txnFeeTotal = common.SafeMulBigInt(big.NewInt(221554), types.GetDefaultGasPrice())
	burnAmount, txnFeeRewards = calculateTxnFeeSplitCoins(txnFeeTotal)
	total := common.SafeAddBigInt(burnAmount, txnFeeRewards)
	log.Info("TestTxnFee", "burnAmount", burnAmount, "txnFeeRewards", txnFeeRewards, "txnFeeTotal", txnFeeTotal, "total", total)
	if total.String() != "10550190476190471970400" {
		t.Fatalf("Failed5")
	}
}

func TestPos_FlattenTxnMap(t *testing.T) {
	txnList, txnAddressMap := flattenTxnMap(nil)
	if txnList != nil && txnAddressMap != nil {
		t.Fatalf("failed")
	}

	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 4)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := types.NewLondonSignerDefaultChain()

	groups := map[common.Address]types.Transactions{}
	txnCount := 0
	overallCount := 0
	for _, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		txnCount = txnCount + 1
		for i := 0; i < txnCount; i++ {
			tx, err := types.SignTx(types.NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			if err != nil {
				fmt.Println("signtx err", err)
				t.Fatalf("failed")
			}
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}

	txnList, txnAddressMap = flattenTxnMap(groups)
	if txnList == nil && txnAddressMap == nil {
		t.Fatalf("failed")
	}

	if len(txnList) != overallCount {
		t.Fatalf("failed")
	}

	if len(txnAddressMap) != overallCount {
		t.Fatalf("failed")
	}

	for addr, txns := range groups {
		for _, txn := range txns {
			addrResult, ok := txnAddressMap[txn.Hash()]
			if ok == false {
				t.Fatalf("failed")
			}
			if addr.IsEqualTo(addrResult) == false {
				t.Fatalf("failed")
			}
		}
	}

	for txnhash, addr := range txnAddressMap {
		addrResult, ok := groups[addr]
		if ok == false {
			t.Fatalf("failed")
		}
		found := false
		for _, t := range addrResult {
			hash := t.Hash()
			if hash.IsEqualTo(txnhash) {
				found = true
				break
			}
		}
		if found == false {
			t.Fatalf("failed")
		}
	}

	resultMap, err := recreateTxnMap(txnList, txnAddressMap, groups)
	if err != nil {
		t.Fatalf("failed")
	}

	for k, v := range groups {
		txns, ok := resultMap[k]
		if ok == false {
			t.Fatalf("failed")
		}

		for _, t1 := range v {
			found := false
			for _, t2 := range txns {
				t2hash := t2.Hash()
				if t2hash.IsEqualTo(t1.Hash()) {
					found = true
					break
				}
			}
			if found == false {
				t.Fatalf("failed")
			}
		}
	}

	for k, v := range resultMap {
		txns, ok := groups[k]
		if ok == false {
			t.Fatalf("failed")
		}

		for _, t1 := range v {
			found := false
			for _, t2 := range txns {
				t2hash := t2.Hash()
				if t2hash.IsEqualTo(t1.Hash()) {
					found = true
					break
				}
			}
			if found == false {
				t.Fatalf("failed")
			}
		}
	}

}

func encCall(abi *abi.ABI, method string, args ...interface{}) ([]byte, error) {
	return abi.Pack(method, args...)
}

func encCallOuter(abi *abi.ABI, method string, args ...interface{}) ([]byte, error) {
	return encCall(abi, method, args...)
}

func TestIsConversionRequestTxn(t *testing.T) {
	abiData, err := conversion.GetConversionContract_ABI()
	if err != nil {
		t.Fatalf("abi error: %v", err)
	}

	// Positive: real requestConversion(string,string) calldata.
	reqData, err := abiData.Pack(conversion.GetContract_Method_requestConversion(),
		"0x9D0bEEc8D63ef6484686d1F8470be62a210B7dBd", "0xdeadbeef")
	if err != nil {
		t.Fatalf("pack requestConversion: %v", err)
	}
	if isConversionRequestTxn(reqData) == false {
		t.Fatalf("expected true for requestConversion calldata")
	}

	// Positive: selector-only (valid prefix, empty body) must still classify as a request,
	// because deeper validation is Convert's job (and such a tx must be skipped, not abort).
	if isConversionRequestTxn(reqData[:4]) == false {
		t.Fatalf("expected true for bare requestConversion selector")
	}

	// Negative: a different conversion-contract method (getAmount).
	getData, err := abiData.Pack(conversion.GetContract_Method_getAmount(), common.Address{1})
	if err != nil {
		t.Fatalf("pack getAmount: %v", err)
	}
	if isConversionRequestTxn(getData) {
		t.Fatalf("expected false for getAmount calldata")
	}

	// Negative: nil, empty, and too-short (<4 bytes) calldata.
	if isConversionRequestTxn(nil) || isConversionRequestTxn([]byte{}) || isConversionRequestTxn([]byte{0x19, 0x47, 0xf4}) {
		t.Fatalf("expected false for nil/empty/short calldata")
	}

	// Negative: correct length but wrong selector (first 4 bytes differ).
	wrong := make([]byte, len(reqData))
	copy(wrong, reqData)
	wrong[0] ^= 0xff
	if isConversionRequestTxn(wrong) {
		t.Fatalf("expected false for corrupted selector")
	}
}

func TestPos_Pack(t *testing.T) {
	method := staking.GetContract_Method_AddDepositorSlashing()
	abiData, err := staking.GetStakingContract_ABI()
	if err != nil {
		fmt.Println("TestPack abi error", err)
		t.Fatalf("failed")
	}

	// call
	slashedAmount := big.NewInt(10)
	_, err = encCallOuter(&abiData, method, ZERO_ADDRESS, slashedAmount)
	if err != nil {
		fmt.Println("Unable to pack TestPack", "error", err)
		t.Fatalf("failed")
	}
}

func TestPos_PackAddress(t *testing.T) {
	fmt.Println(ZERO_ADDRESS)
	method := conversion.GetContract_Method_setConverted()
	abiData, err := conversion.GetConversionContract_ABI()
	if err != nil {
		fmt.Println("TestPackAddress abi error", err)
		t.Fatalf("failed")
	}

	// call
	encoded, err := encCallOuter(&abiData, method, common.Address{1}, common.Address{2})
	if err != nil {
		fmt.Println("Unable to pack TestPackAddress", "error", err)
		t.Fatalf("failed")
	}

	fmt.Println("encoded", encoded)
}

func testGetBlockConsensusContextForBlock(t *testing.T, blockNumber uint64, expectedBlockNumber uint64) {
	expectedKey, err := GetConsensusContextKey(expectedBlockNumber)
	if err != nil {
		fmt.Println("err", err)
		t.Fatalf("failed 1")
		return
	}

	key, err := GetBlockConsensusContextKeyForBlock(blockNumber)
	if err != nil {
		fmt.Println("err", err)
		t.Fatalf("failed 2")
		return
	}

	if key != expectedKey {
		fmt.Println("blockNumber", blockNumber, "expectedKey", expectedKey, "got", key)
		t.Fatalf("failed 3")
		return
	}
}

func Test_GetBlockConsensusContextForBlock(t *testing.T) {
	testGetBlockConsensusContextForBlock(t, uint64(536000), uint64(472000))
	testGetBlockConsensusContextForBlock(t, uint64(536002), uint64(472002))
	testGetBlockConsensusContextForBlock(t, uint64(536003), uint64(472003))

	testGetBlockConsensusContextForBlock(t, uint64(933888), uint64(869888))
	testGetBlockConsensusContextForBlock(t, uint64(933889), uint64(421889))
	testGetBlockConsensusContextForBlock(t, uint64(933890), uint64(421890))
}

func TestTemp(t *testing.T) {
	fmt.Println(common.HexToAddress("0x6e71edae12b1b97f4d1f60370fef10105fa2faae0126114a169c64845d6126c9"))
}

func TestTemp1(t *testing.T) {
	fmt.Println(crypto.Keccak256Hash([]byte("C:/github/quantumswap/main/v3-periphery/contracts/libraries/NFTDescriptor.sol:NFTDescriptor")))
}
