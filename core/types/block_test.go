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

package types

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/hashingalgorithm"
	"github.com/QuantumCoinProject/qc/log"
	"hash"
	"math/big"
	"reflect"
	"testing"

	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/math"
	"github.com/QuantumCoinProject/qc/params"
	"github.com/QuantumCoinProject/qc/rlp"
)

var (
	privtestkey1, _ = cryptobase.SigAlg.GenerateKey()
	hextestkey1, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey1)
	sigtest1, _     = cryptobase.SigAlg.Sign([]byte("This is test programThis is test"), privtestkey)
	hexsigtest1     = hex.EncodeToString(sigtest)
)

func TestCreateBlock(t *testing.T) {
	hash := common.HexToHash("0x123123")
	var blockNonce BlockNonce
	blockNonce[0] = 1

	header := &Header{
		Coinbase:    common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"),
		Number:      big.NewInt(int64(1)),
		ParentHash:  hash,
		Difficulty:  big.NewInt(131072),
		GasLimit:    uint64(3141592),
		TxHash:      EmptyRootHash,
		ReceiptHash: EmptyRootHash,
		MixDigest:   common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"),
		Root:        common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"),
		Nonce:       blockNonce,
		Time:        1426516743,
		GasUsed:     21000,
	}
	hash = header.Hash()

	block1 := NewBlockWithHeader(header)

	buff := new(bytes.Buffer)
	err := rlp.Encode(buff, block1)
	if err != nil {
		t.Fatal("Encode error: ", err)
	}
	buffBlob := buff.Bytes()

	var block2 Block

	if err := rlp.DecodeBytes(buffBlob, &block2); err != nil {
		t.Fatal("decode error: ", err)
	}

	if block1.header.Coinbase.IsEqualTo(block2.header.Coinbase) == false {
		t.Fatalf("failed Coinbase")
	}
	if block1.header.Number.Cmp(block2.header.Number) != 0 {
		t.Fatalf("failed Number")
	}
	if block1.header.ParentHash.IsEqualTo(block2.header.ParentHash) == false {
		t.Fatalf("failed ParentHash")
	}
	if block1.header.Difficulty.Cmp(block2.header.Difficulty) != 0 {
		t.Fatalf("failed Difficulty")
	}
	if block1.header.TxHash.IsEqualTo(block2.header.TxHash) == false {
		t.Fatalf("failed TxHash")
	}
	if block1.header.ReceiptHash.IsEqualTo(block2.header.ReceiptHash) == false {
		t.Fatalf("failed ReceiptHash")
	}

	hash1 := block1.Hash()
	hash2 := block2.Hash()
	if hash1.IsEqualTo(hash2) == false {
		t.Fatalf("failed block hash")
	}

	if len(block1.transactions) != len(block2.transactions) {
		log.Info("txn count", "b1", len(block1.transactions), "b2", len(block2.transactions))
		t.Fatalf("txn count")
	}

	for i, _ := range block1.transactions {
		h1 := block1.transactions[i].Hash()
		h2 := block2.transactions[i].Hash()

		if h1.IsEqualTo(h2) == false {
			t.Fatalf("failed txn hash")
		}
	}

	fmt.Println(block1.Hash())
	fmt.Println(common.Bytes2Hex(buffBlob))
}

// from bcValidBlockTest.json, "SimpleTx"
func TestBlockEncoding(t *testing.T) {

	blockEnc := common.FromHex("f9020bf90207a00000000000000000000000000000000000000000000000000000000000123123a00000000000000000000000008888f1f195afa192cfee860698584c030f4c9db1a0ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b90100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008302000001832fefd8825208845506eb0780a0000000000000000000000000000000000000000000000000000000000000000080a0bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff49888010000000000000080c0")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	check("Difficulty", block.Difficulty(), big.NewInt(131072))
	check("GasLimit", block.GasLimit(), uint64(3141592))
	check("GasUsed", block.GasUsed(), uint64(21000))
	check("Coinbase", block.Coinbase(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	check("MixDigest", block.MixDigest(), common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"))
	check("Root", block.Root(), common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"))
	check("Hash", block.Hash(), common.HexToHash("0xc47617147c52fc49f5b5bdb3ab971c7ff227adb915734ef9339f0236373d2899"))
	check("Nonce", block.Nonce(), uint64(72057594037927936))
	check("Time", block.Time(), uint64(1426516743))
	check("Size", block.Size(), common.StorageSize(len(blockEnc)))

	ourBlockEnc, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}
	if !bytes.Equal(ourBlockEnc, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", ourBlockEnc, blockEnc)
	}
}

var benchBuffer = bytes.NewBuffer(make([]byte, 0, 32000))

func BenchmarkEncodeBlock(b *testing.B) {
	block := makeBenchBlock()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		benchBuffer.Reset()
		if err := rlp.Encode(benchBuffer, block); err != nil {
			b.Fatal(err)
		}
	}
}

// testHasher is the helper tool for transaction/receipt list hashing.
// The original hasher is trie, in order to get rid of import cycle,
// use the testing hasher instead.
type testHasher struct {
	hasher hash.Hash
}

func newHasher() *testHasher {
	return &testHasher{hasher: hashingalgorithm.NewHashState()}
}

func (h *testHasher) Reset() {
	h.hasher.Reset()
}

func (h *testHasher) Update(key, val []byte) {
	h.hasher.Write(key)
	h.hasher.Write(val)
}

func (h *testHasher) Hash() common.Hash {
	return common.BytesToHash(h.hasher.Sum(nil))
}

func makeBenchBlock() *Block {
	var (
		key, _   = cryptobase.SigAlg.GenerateKey()
		txs      = make([]*Transaction, 70)
		receipts = make([]*Receipt, len(txs))
		signer   = LatestSigner(params.TestChainConfig)
	)
	header := &Header{
		Difficulty: math.BigPow(11, 11),
		Number:     math.BigPow(2, 9),
		GasLimit:   12345678,
		GasUsed:    1476322,
		Time:       9876543,
		Extra:      []byte("coolest block on chain"),
	}
	for i := range txs {
		amount := math.BigPow(2, int64(i))
		price := big.NewInt(300000)
		data := make([]byte, 100)
		tx := NewTransaction(uint64(i), common.Address{}, amount, 123457, price, data)
		signedTx, err := SignTx(tx, signer, key)
		if err != nil {
			panic(err)
		}
		txs[i] = signedTx
		receipts[i] = NewReceipt(make([]byte, 32), false, tx.Gas())
	}
	return NewBlock(header, txs, receipts, newHasher())
}
