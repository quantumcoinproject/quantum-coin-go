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
	"encoding/json"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/hashingalgorithm"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"hash"
	"math/big"
	"reflect"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/math"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

var (
	privtestkey1, _ = cryptobase.SigAlg.GenerateKey()
	hextestkey1, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey1)
	sigtest1, _     = cryptobase.DynamicSign.Sign([]byte("This is test programThis is test"), privtestkey)
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

func TestBlockTransactionsRoot(t *testing.T) {
	keypair, err := cryptobase.SigAlg.DeserializePrivateKey(common.Hex2Bytes("c74f5b4a077825afce8bb6af94a17658c0ebc19d27bcc86b2e2503719fe760482e66ab980429a49024d2ed146de28a307ec163498946a9dd327162524197e3045e2247d4a0443ae4aae18d3cf8b3669c08d4ad340408f02fecbb9d5cbed819a5a39ab5f2e219c29d768db87c77ef658c6ce33049cdd64002968c2666034dbcac81828a4821cb06cbde12f409de86fab226a40c4624ece2fc6e4ffb49cc689868b1d656079f1b18adbe66a6a56d56b1ef8812d0fb1d9f21815127600fc0c2779ce294000a2182cc9844a1404502194e13a340d91466c20005123082db1645c106849cc44d11279192c8842413221b4821443629090749a0208a21960440228c50c271da4862d38868a2a040d234920bc731db0061a2008223a428029431123168c93081e1824cc44068d2c8040218098022111804400c02728ac471d2a670d2942d011824e114208c182881268a0bc62521966924082a54c081ca128211480619014e14804142180a08248e90440649406e11b3280109459402260a954d44428e1830880a4622dc189109414d20126589c43002b391e31250832452c4b031cc3009c9183053c22d89146c01478688866820301003136a0126240c04211ac18c92284264460d53228e1cc801648648a0402ed08231d2228003b248e3442864406d24b10401c02d12040c02932801b3484a160c21b62da4323224c000cab4849bb4601a338a09062a8a160d9832601bc44849828410426c5a36484a1609c2262913476941b08061404004190089c0608c288c08c98cc3266c43242a033510da28400c11820bb74c22354c211909a10830cc060a048751cb300612390d42b4311b2984a2901008a36982246e8cc66c8b92054a2661c3c811d34840213681d4160103c26009c7609c0641c444121a3468189981100029534005e41211602064034231e00005988421811809a11041883630529205a30430c8c065cbc460cc944d4216601048894036911b360020100ca2c64114438e11c109220124c23010d3920cd8308a80300619364e1297699a829049128009c1281a324484103220410c19a9401c394e14b60511a00c5b922d1933469280514cc28802b48061c440da108809b05019935088c82c1994301324315912059a18725824318024240a166a5b4886131750db304ec8066d00c2601449424b98804840921b268804c851d2c8852443621ab9110130280a49209a082698406163168c4204024a082224998d03810ca3862409968d1b065219904921b341dac48113974c029741831222e13002932241039864c186851945914ca28880b8250c352dd3342918b5841b435289a660d4220988c5ff369f66a0cbbdd4df9a6f643513cd64ad88b1d02b02d90db66d433657f0f6e997915a4faf9ac5b23e85c535f92261f74f851a80de60f712bd8cc84ed586d640997acbde89051e3fe2d814f3222a938fe12c62e7e491e808500050faa0675c3747fd540074a5859ae6d3891a920d43395004e0108d98206e0ef85807ae28605b3bdbd08d65b9f7c3f883fa4ea011820c876e267ec511ceb508f2c5916916b161dba59c1f36a1cb6f50279cbe0f905a0f02a9109d8b7549800c629f906d5ebb0cd1f42c71e7ab9da83a449b94045b3b77c88bbc5bdff731956b2c2e55a8b1eb15bde9d4e07fbc1235946702dd1bae658e7b35b7049cca9026973e3466188e51c858256dd6d9a37cc58e9543c58c4d7c5fdb2583def57784c6e0bb676eba7372b5083a27e88c68ade97295f064b6139833cb7e6729a412dc2282684ff104e6ed7405f0f2fe24f1eb81be6951a6832aa5e3e41367715edc3a366491d720f29b867638c65b78be1aec8b55b8be15220f836bcdf39cc1b6c82c972db5baf1ba3ade74dafbd4e59ff7287e953e462d2db9093b90b97c42303f6fa394195b385893e4752f6a8ba7db2a8b6a2f48fad7f8db5c0236fe7b95f64b4e7ea35a26261d988bcc3e7e3ca28635fab521399a21a0bde79146cfa747c914f90d25dbdb1a4e7d62e9e47fa8a1d44d23567fb0fd4dfeb153ee4f5939cb5fa1d6c9133e29509d05a3ff1e8bfca50909d1da82649d817c5d148442a63cde32cf8112e2895e9a0338831c04c6c82f44f59bca7842e2445b0aac6fa947633f9e10c9a783c8dc4ab4cea5977846dec19b30da64a07ed2aa32bfa34165912a50bb85c1a1c5d8778463e0f022461bf01ecd15d9c0b07296b86ba187476a43d93d1da954f8a0c3e57fa9b367273a2b05d0244af44324407c3f73b8416ccf341f667ed8db0552ed8015a8f248ac54b1d03008cf46e081f2f0208fdb05d69144a8f89f8762b1ee836c2d91699166093354fd0171445a2fc7bda34b16c86c415a54f88a873efe12256943855235449c622d806512cfa0c3d2f7e8232c9cdd76adbcdfeedd5ec0c36ad05a55344a6682280f75bd30ce06e08e8484e443fbedd8765fc60e6422d7c33d823e7b5022f896fa9164c0f3d73fee28c65cd49afc1271b72d93931c8a527a657bb64a74ec4b0ee1a9a522446000b8d0f1de6c86ef5a78bd7f55d5f8bb4e78c44c640539ff6cd787742056bdfbcf9e917e469a0e2f48a50cb67fab85eafc3da58d246e2ea5d80ca93664a8cb3df20a753ab5f2a6fb9b467d3d24b4bf678f3905080882d29211df75081a910b9bc10a998400a09aca649b874d4c1c7df9dcec34312e0e373e9025073c77da1df56f340bb66a7621dd4aa4188d37bbdd71f16202c5bd03dbad5c0bcda8b1fd9351ec9bf558ee02f9ca41dbcc641d467d76adae691a4f49b562c7efc61a0fb5301f3cec548af46cc238387d8e5cc5d18038ed4ad48ac65d5a1e02480cfad3093264aa77b6aefc5c09edbb31a354cafebed7fa170f3c7f1bd4af5b3318399ba778e4dd77f8ab586676971780505c189c814faad495e998768e01f9fcd22d702bd030c9afd3fa6910a51f73b1b994815611ceb1607b60eee4bc96bcf193add7821a46084870a2a0ef280c99269b2bda21301d6f1ff3ad9b1c254dabb16357ecb2e697582ab2cee0ac22400a4e4c4a5a8504276f918eaf5de16e1d70fcbebf0796fc8eae2ed4a4ce243f0ed9c76e7a2464662b5757261ad0a407f79eda860c81ea71e4587ed6f4d217dcbded2769f9ae2468df802b70464194fef1d7fdcf5205af26ab74600ae363e99f22ab92cf8f520be168552b5d3ae472c4ddea50f56c39ea3749ac9f36632b47b07741a159d227ede749997d95b50bbe180a58c176734845fd91bbc8164d926745bbdbeb6cb540bfade28ee1b43a542f1d3ce1f04fa41957b37539f32d1479f2880f99d8d1c6b6d57e194cba2e9a5a9173ebd8af33d771529bcb256767ad8b1cd873099e263c4e6d64e6ee981f4dbe6cc1fcf2b7d93a71a053c9250dd50c6e83deeb63d5eed1bddb53de54fa87b9140390ce39909236355746a359a262d9f2cbad718778ed1253195774c2fbfb1768cacfa98264f3711082c1e92b8e3b4901c93714520eec83b8023a1422c31b4aaa917517b6fb515c8fa3a77ed8bbc25ca4a73a99982fdf04f70c4c9fc9b34f756b87e82166b36852aab3e26b93056b1cc54fc4830963265f6ca98486ff08deb954206e9c1e12bc5e7d9bf530b7fb88798213a2f4b1f9a5c97d93e7025045fc22b651da1f1e577aee02bb072678ffe93c24d4efb43f8c89d6059359a8f504baf6f80ef75e2247d4a0443ae4aae18d3cf8b3669c08d4ad340408f02fecbb9d5cbed819a5d33a5f09438f6f86b82bf4b791ff1e46ba11668d45f8df96a0f6eed068e33a96d728ed77319882a22ad04aa7446231bbabfcdd515f41bfba215d777f2b8bfd1f2d1fefabbac319ed8a1474c635d7ee4296b4b61a327d1daf3f5ac50ad484a8ffdfedfb58fcc11fb4882bb20a8dffdb68aa9035e81d0e1297759f875b76c67d870bdf2d7b1c195ffe89b3e4cb31c834a40e0de20c0e2adda29ed77e041f1dab1e4ae83ea320b01498634c227b5a2b4c36e7911eaefa40920de760c98c47e38a1d157afd4bff9771b80453ea95a985f6367274d2922aeb37c5b7b85fb1681a7958a65d10178871d631c879ded24cfccc6a252f42f33c5cdfca421187a32e286c5ce6a708c8a4599ebd1ed326cdcb547323674ee6363e36d060070d207be3274b61c180c96e5fdd36a877842b7b443697dc3392ea793f8e2af0063b355287bb65602b5f68a99194620789396dcef3e066dfd09076601832f758fe0d3a08b93adff9963ecd06917eee2d6669b134eb14d6da7aa49ed90c34a3a58e5bda1de193ff8c016079ceba2cda2419ac4bc0b10b69411e644092052a1630ab58d7f8197d998a3ccc7c0f5895c2f2c0c2bc920b727926cc16558d6b880e2d4708070ad46b88befc5525602a7c499dcd4c778990999f96cac53f4a9a199cc689d93b35fb0e1f5fa8a6928ff9ff8d86ad8bcc303318058c9ae30beecacf5aaeac675b00138bfee1dcb41a3e9b4de2bb94dd94d0bc1ab428615d58792ee6e5d16fd68897d05f0f1c8060ff4de53f11bdf64e0045d02f33859e02e02935450ad9da50baacaf3f1934358acbbc013d471cb0a0cbd5d3d297ef5299887767e830b5655a7787d5f8420b49f1792321309b34b4cdb959f91c30a91a3fcfb446f04850a735676ba43c9b18982cc902fdfd63ff1d7c52e698f57796e6166abe4339e43c550869b86b139b993f1f998166dc74e8821fc52873d0c7f113be219668c706bea4774b42a48b489c25847e6c555b06fe47dda0df2931078510516c8ad229962145f60faaad907f7e8719c52e288d5e27bc280959e84a8ace092abbf1078207437fbcec9f1755e2000fff2db748aa853b454318ed5b8bd1fe30cc80fd430aa38451e655f2f4e7722bc5407692e708948bd970508821358c9bc285a37a16d686f06a9a373317e8eb7dd762f052013d7f9fe34fbe881b30c5954cdb7bb08b974cd17335907a37a5302f1c9de49ce10c598f8ffcd9495487f9122367d4a82fc93f06fbff88b9cf326cb498f5a38538dba1f8a7abd4b0998e3a67381fa3dc1fe65e827d30fdd1dd72163e387ada4fe3bd1513d147454061a3c77aba5a00bf35eb7a5880d7ce0ac9bf5427407e9fbf7507d25e145a742c00d68c6217617e0b5c4eb15ccba8fc74512b17de199773917e78ec70649877de66192f8bdfc2450b7a0ecc1cdee4ae54af4e69e25837339b5f318d4bc2f473d3d5cb96268a5b7169c02b441c014c05fc5f84bf8baf0dd325f714b32e15884e591c187b2f242ee2799f7f35f8bf4304e15309e39e9f1d92f59a3098481de06469ce1249722f1d1b432cbef6501996e578924592479ea871b2a8bc0acca96f682d460524a6b28c5a7a5d44c69cddf99bdde19617f27e35b9a25c2790ed0357bbf81a598c8d9c388dcdb66f6da04ec3b4a01dbacad7e5bdb165884c296e33082dac0f4ae2f2c84eff33ccd0bb32b69bfccd8eb4726aecfab24a684c73eb0dacb08883a20adb05645d2c63740ef51c9de20f706d00b209640a7f9ed7dda936305593b09e3b9ec3f2b875ec0fb5d3547abcf9235a1f5d45cd00a72cbcb58b1da407a6b443cfe481b1d9c9a99d69b629b22e2ea96129ae94d44a8503603732c67f37de63aa96b7729c5ac57735c5b269be7d018cf3ea4fc7feb761ac4255f66cbc896253b6ef3ce63aac42ac6080b7ece76e6ad84f839e10c69bf0c7ff313ef5469205ae8f45ea118248d14c43780c9fc5ff35674016b7"))
	if err != nil {
		t.Fatalf("failed")
	}

	var (
		txs      = make([]*Transaction, 1)
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
	i := 0
	amount := math.BigPow(2, int64(i))
	price := big.NewInt(300000)
	data := make([]byte, 100)
	tx := NewTransaction(uint64(i), common.Address{}, amount, 123457, price, data)
	signedTx1, err := SignTx(tx, signer, keypair)
	if err != nil {
		t.Fatalf("failed")
	}
	txs[i] = signedTx1
	receipts[i] = NewReceipt(make([]byte, 32), false, tx.Gas())
	block1 := NewBlock(header, txs, receipts, newHasher())
	block2 := NewBlock(header, txs, receipts, newHasher())
	txnRoot1 := block1.TxHash()
	txnRoot2 := block2.TxHash()
	txnRoot3 := block1.TxHash()
	if txnRoot1.IsEqualTo(txnRoot2) != true {
		t.Fatalf("failed")
	}
	if txnRoot1.IsEqualTo(txnRoot3) != true {
		t.Fatalf("failed")
	}

	signedTx1.RawSignatureValues()
	rawHash, _ := signedTx1.RawHash()
	fmt.Println("block", "block transactions root", txnRoot1, "Hash", signedTx1.Hash(), "rawHash", rawHash)

	signedTx2, err := SignTx(tx, signer, keypair)
	if err != nil {
		t.Fatalf("failed")
	}
	txs[i] = signedTx2
	block3 := NewBlock(header, txs, receipts, newHasher())
	rawHash, _ = signedTx2.RawHash()
	fmt.Println("block", "block transactions root", block3.TxHash(), "Hash", signedTx2.Hash(), "rawHash", rawHash)
}

// headerTestBaseline returns a fully populated header for Hash / IsEqualTo tests.
func headerTestBaseline() *Header {
	var bloom Bloom
	bloom[0] = 0xab
	bloom[len(bloom)-1] = 0xcd
	return &Header{
		ParentHash:            common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
		Coinbase:              common.HexToAddress("0x2222222222222222222222222222222222222222"),
		Root:                  common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"),
		TxHash:                common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444"),
		ReceiptHash:           common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555"),
		Bloom:                 bloom,
		Difficulty:            big.NewInt(1001),
		Number:                big.NewInt(2002),
		GasLimit:              3003,
		GasUsed:               4004,
		Time:                  5005,
		Extra:                 []byte{0x01, 0x02},
		Author:                common.HexToHash("0x6666666666666666666666666666666666666666666666666666666666666666"),
		ConsensusData:         []byte{0xde, 0xad},
		MixDigest:             common.HexToHash("0x7777777777777777777777777777777777777777777777777777777777777777"),
		Nonce:                 EncodeNonce(0x8888888800000000),
		UnhashedConsensusData: []byte{0xca, 0xfe},
	}
}

func TestHeaderJSONRoundTripPreservesHash(t *testing.T) {
	h := headerTestBaseline()
	want := h.Hash()
	blob, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var out Header
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatal(err)
	}
	got := out.Hash()
	if !got.IsEqualTo(want) {
		t.Fatalf("Hash() changed after JSON round-trip: got %v want %v\njson=%s", got, want, blob)
	}
}

func TestHeaderJSONRoundTripPreservesHash_emptySlices(t *testing.T) {
	h := headerTestBaseline()
	h.Extra = []byte{}
	h.ConsensusData = []byte{}
	h.UnhashedConsensusData = []byte{}
	blob, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var out Header
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal: %v\njson=%s", err, blob)
	}
	want := h.Hash()
	got := out.Hash()
	if !got.IsEqualTo(want) {
		t.Fatalf("Hash() changed: got %v want %v\njson=%s", got, want, blob)
	}
}

func TestHeaderJSONRoundTripPreservesHash_nilByteSlices(t *testing.T) {
	h := headerTestBaseline()
	h.Extra = nil
	h.ConsensusData = nil
	h.UnhashedConsensusData = nil
	blob, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var out Header
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal: %v\njson=%s", err, blob)
	}
	want := h.Hash()
	got := out.Hash()
	if !got.IsEqualTo(want) {
		t.Fatalf("Hash() changed: got %v want %v\njson=%s", got, want, blob)
	}
}

// TestHeaderHashFieldSensitivity checks that Header.Hash() (RLP-based) changes when any
// hashed field changes, and stays the same when only UnhashedConsensusData changes.
func TestHeaderHashFieldSensitivity(t *testing.T) {
	baseline := headerTestBaseline()

	baseHash := baseline.Hash()
	if baseHash.IsEqualTo(common.Hash{}) {
		t.Fatal("baseline header hash is zero")
	}
	if h2 := baseline.Hash(); !h2.IsEqualTo(baseHash) {
		t.Fatalf("Hash() not stable: %v vs %v", baseHash, h2)
	}

	assertHashChanges := func(name string, mutate func(*Header)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			h := CopyHeader(baseline)
			mutate(h)
			if got := h.Hash(); got.IsEqualTo(baseHash) {
				t.Fatalf("hash unchanged after mutating %s: %v", name, got)
			}
		})
	}

	assertHashSame := func(name string, mutate func(*Header)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			h := CopyHeader(baseline)
			mutate(h)
			if got := h.Hash(); !got.IsEqualTo(baseHash) {
				t.Fatalf("hash changed after mutating %s: got %v want %v", name, got, baseHash)
			}
		})
	}

	assertHashChanges("ParentHash", func(h *Header) {
		h.ParentHash = common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	})
	assertHashChanges("Coinbase", func(h *Header) {
		h.Coinbase = common.HexToAddress("0xffffffffffffffffffffffffffffffffffffffff")
	})
	assertHashChanges("Root", func(h *Header) {
		h.Root = common.HexToHash("0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	})
	assertHashChanges("TxHash", func(h *Header) {
		h.TxHash = common.HexToHash("0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	})
	assertHashChanges("ReceiptHash", func(h *Header) {
		h.ReceiptHash = common.HexToHash("0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	})
	assertHashChanges("Bloom", func(h *Header) {
		h.Bloom[0] ^= 0xff
	})
	assertHashChanges("Difficulty", func(h *Header) {
		h.Difficulty = new(big.Int).Add(h.Difficulty, big.NewInt(1))
	})
	assertHashChanges("Number", func(h *Header) {
		h.Number = new(big.Int).Add(h.Number, big.NewInt(1))
	})
	assertHashChanges("GasLimit", func(h *Header) { h.GasLimit++ })
	assertHashChanges("GasUsed", func(h *Header) { h.GasUsed++ })
	assertHashChanges("Time", func(h *Header) { h.Time++ })
	assertHashChanges("Extra", func(h *Header) {
		h.Extra = append([]byte(nil), append(h.Extra, 0xff)...)
	})
	assertHashChanges("Author", func(h *Header) {
		h.Author = common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	})
	assertHashChanges("ConsensusData", func(h *Header) {
		h.ConsensusData = append([]byte(nil), append(h.ConsensusData, 0xff)...)
	})
	assertHashChanges("MixDigest", func(h *Header) {
		h.MixDigest = common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	})
	assertHashChanges("Nonce", func(h *Header) {
		h.Nonce = EncodeNonce(1)
	})

	assertHashSame("UnhashedConsensusData", func(h *Header) {
		h.UnhashedConsensusData = []byte{0x00, 0x11, 0x22, 0x33}
	})
	assertHashSame("UnhashedConsensusData_nil", func(h *Header) {
		h.UnhashedConsensusData = nil
	})
}

// TestHeaderIsEqualTo checks Header.IsEqualTo: true for identical headers (including
// UnhashedConsensusData), false when any hashed field differs, and false when only
// UnhashedConsensusData differs (hashed payload still matches).
func TestHeaderIsEqualTo(t *testing.T) {
	baseline := headerTestBaseline()

	t.Run("identical_copy", func(t *testing.T) {
		other := CopyHeader(baseline)
		if !baseline.IsEqualTo(other) {
			t.Fatal("expected baseline equal to deep copy")
		}
		if !other.IsEqualTo(baseline) {
			t.Fatal("expected symmetry with deep copy")
		}
	})

	t.Run("self", func(t *testing.T) {
		if !baseline.IsEqualTo(baseline) {
			t.Fatal("expected self equal")
		}
	})

	t.Run("both_nil_unhashed_still_equal_when_hashed_match", func(t *testing.T) {
		a := CopyHeader(baseline)
		b := CopyHeader(baseline)
		a.UnhashedConsensusData = nil
		b.UnhashedConsensusData = nil
		if !a.IsEqualTo(b) {
			t.Fatal("expected equal when both unhashed are nil and rest matches")
		}
	})

	assertNotEqual := func(name string, mutate func(*Header)) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			other := CopyHeader(baseline)
			mutate(other)
			if baseline.IsEqualTo(other) {
				t.Fatalf("expected not equal after mutating %s on other", name)
			}
			if other.IsEqualTo(baseline) {
				t.Fatalf("expected not equal (symmetric) after mutating %s on other", name)
			}
		})
	}

	assertNotEqual("ParentHash", func(h *Header) {
		h.ParentHash = common.HexToHash("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	})
	assertNotEqual("Coinbase", func(h *Header) {
		h.Coinbase = common.HexToAddress("0xffffffffffffffffffffffffffffffffffffffff")
	})
	assertNotEqual("Root", func(h *Header) {
		h.Root = common.HexToHash("0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	})
	assertNotEqual("TxHash", func(h *Header) {
		h.TxHash = common.HexToHash("0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	})
	assertNotEqual("ReceiptHash", func(h *Header) {
		h.ReceiptHash = common.HexToHash("0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	})
	assertNotEqual("Bloom", func(h *Header) {
		h.Bloom[0] ^= 0xff
	})
	assertNotEqual("Difficulty", func(h *Header) {
		h.Difficulty = new(big.Int).Add(h.Difficulty, big.NewInt(1))
	})
	assertNotEqual("Number", func(h *Header) {
		h.Number = new(big.Int).Add(h.Number, big.NewInt(1))
	})
	assertNotEqual("GasLimit", func(h *Header) { h.GasLimit++ })
	assertNotEqual("GasUsed", func(h *Header) { h.GasUsed++ })
	assertNotEqual("Time", func(h *Header) { h.Time++ })
	assertNotEqual("Extra", func(h *Header) {
		h.Extra = append([]byte(nil), append(h.Extra, 0xff)...)
	})
	assertNotEqual("Author", func(h *Header) {
		h.Author = common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	})
	assertNotEqual("ConsensusData", func(h *Header) {
		h.ConsensusData = append([]byte(nil), append(h.ConsensusData, 0xff)...)
	})
	assertNotEqual("MixDigest", func(h *Header) {
		h.MixDigest = common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	})
	assertNotEqual("Nonce", func(h *Header) {
		h.Nonce = EncodeNonce(1)
	})

	t.Run("UnhashedConsensusData_differs_same_Hash", func(t *testing.T) {
		other := CopyHeader(baseline)
		other.UnhashedConsensusData = []byte{0x01, 0x02, 0x03}
		h1 := baseline.Hash()
		h2 := other.Hash()
		if !h1.IsEqualTo(h2) {
			t.Fatal("sanity: Hash() should match when only UnhashedConsensusData differs")
		}
		if baseline.IsEqualTo(other) {
			t.Fatal("expected not equal when UnhashedConsensusData differs")
		}
		if other.IsEqualTo(baseline) {
			t.Fatal("expected not equal (symmetric) when UnhashedConsensusData differs")
		}
	})

	t.Run("UnhashedConsensusData_nil_vs_nonempty", func(t *testing.T) {
		a := CopyHeader(baseline)
		b := CopyHeader(baseline)
		a.UnhashedConsensusData = nil
		b.UnhashedConsensusData = []byte{0xca, 0xfe}
		if a.IsEqualTo(b) {
			t.Fatal("expected not equal: nil vs non-nil unhashed payload")
		}
	})
}
