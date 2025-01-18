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

package core

import (
	"github.com/QuantumCoinProject/qc/consensus/mockconsensus"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/hashingalgorithm"
	"math/big"
	"testing"

	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/params"
	"github.com/QuantumCoinProject/qc/rlp"
)

var (
	privtestkey1, _ = cryptobase.SigAlg.GenerateKey()
	hextestkey1, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey1)
)

func getBlock(transactions int, dataSize int) *types.Block {
	var (
		aa = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		// Generate a canonical chain to act as the main dataset
		engine = mockconsensus.NewMockConsensus()
		db     = rawdb.NewMemoryDatabase()
		// A sender who makes transactions, has some funds
		key, _  = cryptobase.SigAlg.HexToPrivateKey(hextestkey1)
		address = cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		funds   = big.NewInt(1000000000000000)
		gspec   = &Genesis{
			Config: params.TestChainConfig,
			Alloc:  GenesisAlloc{address: {Balance: funds}},
		}
		genesis = gspec.MustCommit(db)
	)

	// We need to generate as many blocks +1 as uncles
	blocks, _ := GenerateChain(params.TestChainConfig, genesis, engine, db, 1,
		func(n int, b *BlockGen) {
			if n == 1 {
				// Add transactions and stuff on the last block
				for i := 0; i < transactions; i++ {
					tx, _ := types.SignTx(types.NewTransaction(uint64(i), aa,
						big.NewInt(0), 50000, nil, make([]byte, dataSize)), types.NewLondonSignerDefaultChain(), key)
					b.AddTx(tx)
				}

			}
		})
	block := blocks[len(blocks)-1]
	return block
}

// BenchmarkHashing compares the speeds of hashing a rlp raw data directly
// without the unmarshalling/marshalling step
func BenchmarkHashing(b *testing.B) {
	// Make a pretty fat block
	var (
		bodyRlp  []byte
		blockRlp []byte
	)
	{
		block := getBlock(200, 50)
		bodyRlp, _ = rlp.EncodeToBytes(block.Body())
		blockRlp, _ = rlp.EncodeToBytes(block)
	}
	var got common.Hash
	var hasher = hashingalgorithm.NewHashState()
	b.Run("iteratorhashing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var hash common.Hash
			it, err := rlp.NewListIterator(bodyRlp)
			if err != nil {
				b.Fatal(err)
			}
			it.Next()
			txs := it.Value()
			txIt, err := rlp.NewListIterator(txs)
			if err != nil {
				b.Fatal(err)
			}
			for txIt.Next() {
				hasher.Reset()
				hasher.Write(txIt.Value())
				hasher.Sum(hash[:0])
				got = hash
			}
		}
	})
	var exp common.Hash
	b.Run("fullbodyhashing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var body types.Body
			rlp.DecodeBytes(bodyRlp, &body)
			for _, tx := range body.Transactions {
				exp = tx.Hash()
			}
		}
	})
	b.Run("fullblockhashing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var block types.Block
			rlp.DecodeBytes(blockRlp, &block)
			for _, tx := range block.Transactions() {
				tx.Hash()
			}
		}
	})
	if got != exp {
		b.Fatalf("hash wrong, got %x exp %x", got, exp)
	}
}
