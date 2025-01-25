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
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/rlp"
)

// The values in those tests are from the Transaction Tests
// at github.com/ethereum/tests.

var defaultSigner = londonSigner{
	chainId: big.NewInt(DEFAULT_CHAIN_ID),
}
var (
	baseTx = NewTransaction(
		3,
		testAddr,
		big.NewInt(10),
		2000,
		big.NewInt(1),
		common.FromHex("5544"),
	)

	privtestkey, _ = cryptobase.SigAlg.DeserializePrivateKey(common.Hex2Bytes("16294761a8120e1670789f88b8326b26484b5c543207137deb4988d55b53684d76da102d3eacb9a92b07e7927b1d0f6cbca1ee11a468b5e356a86a8c9167bd13fb979fdf3ad71722435cd0d50dd75371a5e046e053b5e7c3768a339760930622d7050ab8a9b2b3e88d519bf51c8b485530e7e149d39ae694324ea3710cff06a625103769c50a32f4b37cbd9fef0d8c6f416ce4169414188a9a4041a49cae46105638f2926e99f885ed3dc96008d57fd7e99f63c5a77164a150bcda82f79eab2410814d0a16802387618818725b364a82b66d9a384620c50941406e42968102b10503b929c3b82d0b144e48420420c640e22062999265034546d34072d014448b3609a3200220c8454a446908910923408980c86403002410128d58220961048d99066121464c198261233046a408215b1826920446cc184e20296910b271cab404e22462e32421cb820d0a446800116a4386659b268cd1b24403816c58323094c825601285dab850c38865e3105122436191268ed9b0841836480bc34d99988551046c03918191486c8432620ba47140880c2041105b4422191449420665232121d94072d2b62888a0450929861038041447295016919b4622c8128221344d2219650436689bb4600a942c1c0864c8348e52028de1060e18b84813319293244691b4281a3525c3a210d34488039310590222ca466123012c14076d18154623c8880c172913266cd8188a5044528b469053246c14419152044d4c924194904c9a186a912824cc300d4138461b381062466d1447665800510819851b138944b268234210db8288540621c1424c0a367012004920356c42008a11362402b62d60347002418e20268c612001da804910094208482298b64163000e02156503394e19446541a00858a88561148959064e9b0201a2b26d60a444c3b624a2862504422e5a186d430032e020209c905003216892c04981b884caa62c59026d21c40189346110185023c83018341004126203a02120286da3182c23a69108926cda3644c4b025c0846411276e0398699330410118401ca9301c05484a32251b8111248645db206d8204269a946c92429254c88513b409240788d9c820cc8448cc2429232548cca80413b02122374ae3c4284ab2815a342e88868563244194406522986d81064e103660021401d1946cd2b24444084a91968c11982ce31249943412123220db047210c4042033094242621bb024c4921003a628c9a205081461c19245514228901810024109180072189488200952a4a491c9244d60140e04a770da040813470699c04c58189123c0480206624836800cc3610393891b037122140553880d7d50f162169b80aba1a4cfa71c3e3e4f38c617af25541c087a7edd5a97c96559ade41ddeb8ba956597fa2b331880de9bc79a3740298700172f7a598b8342f0ce2e6ec68da7be48fec51d610158d5eda747d3f213f429d19d2862ce034a701be209e9f5c6f7a7c4d9acbb1c33dca8bb0d3b84459886e91d50cddf837f5d601fdcc49d0157c15ccb76b0013b225da1a37f2a3e0c5e08f1bd4a630eaf60fce88fc68d8977b55aa79329e374200a7defdf8f82cb46919d8a16dcdb97b455fcd9d866a1bf97c51f1084b5be94c1b27a3067bb499b8dbb10507806405ea3f54f2fff1e10e4d8fba5f01535c10e302d212ff9c1b5194dadfc2a2fadb171b8229d45b5f928b44d3aadc7389d547b4316618b3b7f85f16b44a2f560c359e541a28d0423b0df5a73ffea0ce91d08c9b5f71ef3333d59cff7d005fbe076d5101ba3107b4819f888a70fa4710a75191eb0eeee6386d1144dc7f99efd41f1de84ab9b08c9c8f02afcb8e990ea2fbfabbe04ed4222047fa62fb0be68ca1812eaf780a045796c9dde641f617cd1bb079f8fdb022519cafabdef55433745592b1d990a4af61cb529760a5f09a0aecdfe7dced2ad070048e4cb0f52bb019d9ce8b0e26f85ac70a0f57a32d27daa4fa87d60a4ce4449b9798ea53b23803b221c262ec980908f731935d88d972249d86363fbad626a8526ab0fcdd33fdf83752568799e67927ed0787a8f3ff83790043b84e4b3127d1fbc2fd6b5afde87506edc7a8d58c6db84cb40146ee48d3606f14d0e37b17b449e6f148e84540083f0ce7655fb45d1a12c1b18baf23747d163d5f6338a11e82c77f02210e261a0902918b777b53483bee21182dc2dca5f91927735e0a46ef85b8de77e023e142c78ae927e573710298ac15aa8f82f6536212927b12b993ec35557c49ae8cd87f9e595c7c6276ec46b1cd31286bfd6351dcea738d59c6cb39dbb63c7ed0eace41ef4d6c0e77cc6a2b3b5505eb44a7f2e4df3a6e5195993a77394516a63646a7505577a7d41714033ec9730ac584ba82e2c1ff43274a6ba2ae0f273a4a316832790a91bb755565844c8d95148097d898a25a908fc29acff609c95525ede6b71579f6ed0452783d1560d8534cc5501d6cd0cbe0228e2344688a50cb4dad7596233946c93f21394722357b2fec78b5f9eb7062aaf197e60a5a8d56b28df1c8fd156927962540c1bdc2b5e97b6bcece4496406db6e86ff2d184ca70d948f9974684ff8d0e7e8de150d996ebabf3898365a5754493c3ff1c31cb4fc5cfa19355f7056d857f972910668d36e15093fbf780671b4a1d301266fbe99ec4871671258bd257444c8bd174b05dc123606c45528eac994d8fc64f87da8a9a3c3e5f19ac1dedcaa69be3cf870040208f0877f4a57d278f244abc922a1f0e23a3aad3acbfaadfcdbddec6d3e2908ff8d1fcc1709f5f868823a3c317b99f5f7ae1873567f41a1806c76c8e1bf1d93db3d6dc60f451443f0e015f5bdf2ab00ac4ac995e4ec81f9ffad16a4b4ac0c31cc0e0c58c27317b2e6c779322f01ab6504fc830ac1032154c0453b26953481cc7cef76a4e6186dcfde0438610196d3317ec615068287123b74f5f142c9bbc81ab7f86c2c5f98ddea34670b7e09a7c8956fb709968c0cbeb4f4aa049ade992241c3aa06b7b5ad0726a787dead4278e50203cd4cc4224af9c4eca7322c76ef9c8fc316ce2156c2fb40225515e9fb3fee0835e28625451013487c3ca822f319e7306ac441dab4d9010c4c5bbe99a632830d39455a402706475157d1f26c48fccab707a6c133f12f5e0e763e83a36ffe44dce0b9dcd342ef1027f043ca682a18636ff64c0530549b87104e421a0d38af526fe6237acc9b14a3057063a8821b39f0c6a389f220c0b68ca8075d0198535a5b5c2d9eee2d765814924eac8903a8f4e9c0a4577d248bbf1b81d1bea57bedda36e8d43b8730fb150e33bb24b20e1d879f0ff8eac5a463f6c3a89701a2c6267b034c4d5f9ab8ae203cc405b1751a9ffa04d8f16329a9ed1fbb8ab2929550c8be134b806f8a31112f636dafa55d05e03f35f6283dd67c0f54be17e52be6d0232ce47043eca40c9456e90fcdfdba710deb7428a16e80ac7d350122f9f90fc94c9a529896acbfca9eb13987c7591f77042aa970515a0caed0ac20aa6114bd6c610c7459f5ce68f3a0bb39b9240f809d521b9307378f3575006d85fdfca8b7d0c6bff145a51bad1c69105ec83997f83f2c0bb16933e9d1ea920df5ce7df97da968b14ff0a73f4774b60f33f091ed0e7719b8ee7ef880c440ed8338856b03111b7ebaabc19ffeec96a291ff0b66e38bf501477a3cfe951bbb159d7bfb979fdf3ad71722435cd0d50dd75371a5e046e053b5e7c3768a3397609306227b80f7b166795bb2fbfea48bd383c1fe885325a5d342631551e308eec28811803a8b7111cc1bfda888851194dced49f0db9b5fcdac21d5ddc1243d383b2e27ce693aecd9c0a2f2e9d61c7a871b111b5f32bed1d4177c389ed7ef7169eb3606d1147fdde900695ee02222e6500940872b92074b7f2ea5a98f1993b60695d6703369354f2847a66dfb7d0e759db222bcf046069a4d9afc31a2b6408699705b2e551130fe134f9e4a321a893cf07f205ce481e4af507ba49fab1a646304fc3a6821473a003e2326d3390003657b2b9414457313efd579917d9271854aea8311963d89b658dec2382241f4e76874c2c261831f66304cd86b1cc183301c914b01ea5a2fc2ad5aec9776c3fc28da53b084ec72b97604ca1160db67702d8d63262e0357994e048508ec853ecdb1a854421a7cbb45f1c0b6ed73644bdcdf30c4702a981ffcb671c1c14f962ffd23df25df0301684962dc94cdd2e2297ca5891a88492b3c07a9b7d2560d6e100538a900d0033f8d965186b7aafc5b2bd1cedb9b32051ca241619bf7b2762f0d0041b8387e0864183b70d29692b396f0328eab9898c6858ea78ff18333d6509985d14db872122d3d274a3eb3f94f07144048db83e092cd0f2d03cc834ac3e6977eba79db7edac057ce52483b498cc56e71db5e0ab06f059f007305aaa442b19f7d9bd9dd4975f6ddf34717218326d72b7158e9937d9bdd7765c65570827c6e63846e5f335808b68fb73bee818719f1573d79ca9cf72942a8167bca457f06668df2e4db6dc7532f3298e331d4a8f4fa8db74e8983f4b2804650fa173f874d6f5eca7a4da2e93d3e23190fb180b96212f127e99dc0f713ce0043b97980dd875735ec112ac581caa975a4310369303a7f3c92c5cee4f2240d6281d801173558ca3882d63d89643cf61a1314b9990ba8393df5defc90f244b5c4ddb274f70bc63075574a1b98075ee4ea4d54d6d013f98716a2ad19ea50ea9cd5f3686831dbc1873b1f8beb73c8c237730686703de1b298b81d5fa494d9419f52bd9634f07a6becef44eac6176544ee7a5e9e2fe5259961fdcd6ef997488ba94e030f59a73ece7be01cdfb3993afdef989a0cbffbc45f0aa1ea087583455da30cd074957c7960359bf92365a5d0cf4b540aefe68c6008f29919bf39dee86da0f41e33817567103e33572fc3e7885e7e719a53e5a4a9c0598b48066bec565203da29c9847684e27b6b513657a55e7523b940af1c3433282324ddb9577409edc42edb93ad2768c10d39ba4b08ba12fb1ef62bb5613fc1a9393f5b394022f24d5388a9ef4e5fcc0320885a1193c923c34a4673061e391c97aeecfe4a9d8f82f53876573f0f2b2dd7829e024c87bf2a28c61fff826efd6e79a1cd67bde93e15d3cc95981c58f3345afcd649fb16954bd5ef41811273c69f87cd8d39cde8f2c334993819b7e7bd48be9ffbf56e778795f5323b6fdf456e0f1fb3944c1ebd77438751df4e3605d909af261d6d3be39a4cd53defe6662d4460c5415ab769ca81e430d269eb13d5ee1b1bc75f5334fa42804a6579af373a8ecdce83bf72e873928683169901d93b64d24138634f89f12afe5c5843b04781aee69c53a7613d7cea5b43c8f41401c1a62e971e16c3de2eb8f7ab8c5bf644c4e279e0fa093031bebb1ed62e179373e36f9de0b1b12755cf466c97a24cf42aabf4836136374f361671d17fefee278485a3f61fc230e5a3e6f5e36906bd3e87512df79369f557c23b9b12f1f3351afac7305f7fcb439370f4b4b89f5f732a4b47585a775353610a05ff3e0aee0658f7acc084ecafce29fbedcc61a374235e088c89fa5b81e2d63ecaaf4de3a475823992ce1ecba709968f4bdfedc872c1cb90e4a79ca169f66251845411ff68df2cab62f15193201d85a16a5dc9f0ff52056c23150094090a143e3b749f8195c50abc34b0d9f85b09debe31478892b9e1e3de582c5ca139d426a897185a71fd88"))
	hextestkey, _  = cryptobase.SigAlg.PrivateKeyToHex(privtestkey)
	hash1, _       = defaultSigner.Hash(baseTx)
	sigtest, _     = cryptobase.SigAlg.Sign(hash1.Bytes(), privtestkey)
	hexsigtest     = hex.EncodeToString(sigtest)
	parentHash     = common.HexToHash("0xabcdbaea6a6c7c4c2dfeb977efac326af552d87")

	testAddr = common.HexToAddress("b94f5374fce5edbc8e2a8697c15331677e6ebf0b")

	emptyTx = NewTransaction(
		0,
		common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"),
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)

	rightvrsTx, _ = baseTx.WithSignature(
		NewLondonSignerDefaultChain(),
		common.Hex2Bytes(hexsigtest),
	)

	emptyEip2718Tx = NewTx(&DefaultFeeTx{
		ChainID:    big.NewInt(1),
		Nonce:      3,
		To:         &testAddr,
		Value:      big.NewInt(10),
		Gas:        25000,
		MaxGasTier: GAS_TIER_DEFAULT,
		Data:       common.FromHex("5544"),
	})

	eipSigner  = NewLondonSignerDefaultChain()
	hash2, err = eipSigner.Hash(emptyEip2718Tx)

	sigtest2, _ = cryptobase.SigAlg.Sign(hash2.Bytes(), privtestkey)
	hexsigtest2 = hex.EncodeToString(sigtest2)

	signedEip2718Tx, _ = emptyEip2718Tx.WithSignature(
		eipSigner,
		common.Hex2Bytes(hexsigtest2),
	)
)

func TestDecodeEmptyTypedTx(t *testing.T) {
	input := []byte{0x80}
	var tx Transaction
	err := rlp.DecodeBytes(input, &tx)
	if err != errEmptyTypedTx {
		t.Fatal("wrong error:", err)
	}
}

func TestTransactionSigHash(t *testing.T) {
	homestead := NewLondonSignerDefaultChain()
	hash, err := homestead.Hash(emptyTx)
	if err != nil {
		t.Fatalf("failed")
	}
	if hash != common.HexToHash("aa0954bae3882a5702a5532365f31e1083aadbfb4f18b3588f995e46cab6b969") {
		t.Errorf("empty transaction hash mismatch, got %x", hash)
	}
	hash, err = homestead.Hash(rightvrsTx)
	if err != nil {
		t.Fatalf("failed")
	}
	if hash != common.HexToHash("23abf201dcf94bd17d44522c0c1a4168e76740441073eccffb65cb37e8a1496f") {
		t.Errorf("RightVRS transaction hash mismatch, got %x", hash)
	}
}

func TestTransactionEncode(t *testing.T) {
	txb, err := rlp.EncodeToBytes(rightvrsTx)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	should := common.FromHex(hex.EncodeToString(txb))
	if !bytes.Equal(txb, should) {
		t.Errorf("encoded RLP mismatch, got %x", txb)
	}
}

func decodeTx(data []byte) (*Transaction, error) {
	var tx Transaction
	t, err := &tx, rlp.Decode(bytes.NewReader(data), &tx)
	return t, err
}

func encodeTx(tx *Transaction) error {
	buff := new(bytes.Buffer)
	err := rlp.Encode(buff, tx)
	if err != nil {
		return err
	}
	fmt.Println("encodedtx", common.Bytes2Hex(buff.Bytes()))
	return nil
}

func defaultTestKey() (*signaturealgorithm.PrivateKey, common.Address) {
	key, _ := cryptobase.SigAlg.HexToPrivateKey(hextestkey)
	addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
	return key, addr
}

func TestRecipientEmpty(t *testing.T) {
	signer := NewLondonSignerDefaultChain()
	key, addr := defaultTestKey()

	tx := NewTransaction(uint64(0), common.Address{}, big.NewInt(10), 123457, big.NewInt(1), []byte{})
	signedTx, err := SignTx(tx, signer, key)
	if err != nil {
		panic(err)
	}
	err = encodeTx(signedTx)
	if err != nil {
		t.Fatal(err)
	}

	hb, _ := cryptobase.SigAlg.SerializePrivateKey(privtestkey)
	fmt.Println("hb", common.Bytes2Hex(hb))

	tx, err = decodeTx(common.Hex2Bytes("b90fb800f90fb48301e0f3808301e24101a000000000000000000000000000000000000000000000000000000000000000000a8080c001b9058076da102d3eacb9a92b07e7927b1d0f6cbca1ee11a468b5e356a86a8c9167bd13fb979fdf3ad71722435cd0d50dd75371a5e046e053b5e7c3768a3397609306227b80f7b166795bb2fbfea48bd383c1fe885325a5d342631551e308eec28811803a8b7111cc1bfda888851194dced49f0db9b5fcdac21d5ddc1243d383b2e27ce693aecd9c0a2f2e9d61c7a871b111b5f32bed1d4177c389ed7ef7169eb3606d1147fdde900695ee02222e6500940872b92074b7f2ea5a98f1993b60695d6703369354f2847a66dfb7d0e759db222bcf046069a4d9afc31a2b6408699705b2e551130fe134f9e4a321a893cf07f205ce481e4af507ba49fab1a646304fc3a6821473a003e2326d3390003657b2b9414457313efd579917d9271854aea8311963d89b658dec2382241f4e76874c2c261831f66304cd86b1cc183301c914b01ea5a2fc2ad5aec9776c3fc28da53b084ec72b97604ca1160db67702d8d63262e0357994e048508ec853ecdb1a854421a7cbb45f1c0b6ed73644bdcdf30c4702a981ffcb671c1c14f962ffd23df25df0301684962dc94cdd2e2297ca5891a88492b3c07a9b7d2560d6e100538a900d0033f8d965186b7aafc5b2bd1cedb9b32051ca241619bf7b2762f0d0041b8387e0864183b70d29692b396f0328eab9898c6858ea78ff18333d6509985d14db872122d3d274a3eb3f94f07144048db83e092cd0f2d03cc834ac3e6977eba79db7edac057ce52483b498cc56e71db5e0ab06f059f007305aaa442b19f7d9bd9dd4975f6ddf34717218326d72b7158e9937d9bdd7765c65570827c6e63846e5f335808b68fb73bee818719f1573d79ca9cf72942a8167bca457f06668df2e4db6dc7532f3298e331d4a8f4fa8db74e8983f4b2804650fa173f874d6f5eca7a4da2e93d3e23190fb180b96212f127e99dc0f713ce0043b97980dd875735ec112ac581caa975a4310369303a7f3c92c5cee4f2240d6281d801173558ca3882d63d89643cf61a1314b9990ba8393df5defc90f244b5c4ddb274f70bc63075574a1b98075ee4ea4d54d6d013f98716a2ad19ea50ea9cd5f3686831dbc1873b1f8beb73c8c237730686703de1b298b81d5fa494d9419f52bd9634f07a6becef44eac6176544ee7a5e9e2fe5259961fdcd6ef997488ba94e030f59a73ece7be01cdfb3993afdef989a0cbffbc45f0aa1ea087583455da30cd074957c7960359bf92365a5d0cf4b540aefe68c6008f29919bf39dee86da0f41e33817567103e33572fc3e7885e7e719a53e5a4a9c0598b48066bec565203da29c9847684e27b6b513657a55e7523b940af1c3433282324ddb9577409edc42edb93ad2768c10d39ba4b08ba12fb1ef62bb5613fc1a9393f5b394022f24d5388a9ef4e5fcc0320885a1193c923c34a4673061e391c97aeecfe4a9d8f82f53876573f0f2b2dd7829e024c87bf2a28c61fff826efd6e79a1cd67bde93e15d3cc95981c58f3345afcd649fb16954bd5ef41811273c69f87cd8d39cde8f2c334993819b7e7bd48be9ffbf56e778795f5323b6fdf456e0f1fb3944c1ebd77438751df4e3605d909af261d6d3be39a4cd53defe6662d4460c5415ab769ca81e430d269eb13d5ee1b1bc75f5334fa42804a6579af373a8ecdce83bf72e873928683169901d93b64d24138634f89f12afe5c5843b04781aee69c53a7613d7cea5b43c8f41401c1a62e971e16c3de2eb8f7ab8c5bf644c4e279e0fa093031bebb1ed62e179373e36f9de0b1b12755cf466c97a24cf42aabf4836136374f361671d17fefee278485a3f61fc230e5a3e6f5e36906bd3e87512df79369f557c23b9b12f1f3351afac7305f7fcb439370f4b4b89f5f732a4b47585a775353610a05ff3e0aee062cab62f15193201d85a16a5dc9f0ff52056c23150094090a143e3b749f8195c50abc34b0d9f85b09debe31478892b9e1e3de582c5ca139d426a897185a71fd88b909fe012009d78ccf5ab0b8313a255f404cf02219d53740d589ffc8775f7516da6642453a5be81cb07508691307c4495f258304a41eeb9d2af6e5177f223039ca03aaf909d4eea7b195eb55991b724162e3215742808822eae0bbf40e170aaf04e9c1542c553398a05050163dd4c5f074d63c780e4ef87c9a209a26e82e2305ea2837c61ce67fb6e7321af467d2fc6f260f6ab66837155ee5f29c3e32f52e142640306c22abf8ba5eea4d6fa464bf58a5f33a69a16ab7f3b19151f16e97369b62f8adb6832e90012868905504025066ed11c025f35407022b26aecd611b735761c3aff8380839c791e1deaaf6f46f4189cf936420717c09d35ff736ea694f3f819abe6c427bcc6bc6e6b1304faa3f956eccbb4db4f417e8ac0df3e489ad18b9c3b6e83a51817df3d670aabfaff7da7fc164a9dbe3dda029296d454ab20a2ede6fc3b4d8f60b847855b53dbdfc3680e192ad096e2a6d31b7c1bee37a4c42831cdc166f1c0c62707803d041341c30f7d6f020ce0c820930604e4000e33e0076165a825fd6515817114bd56344e942fa47ef76f785848e206094316fba926f8c8adaadd8f32798120cf807f7233bc1fa0ad40ae3a908af60f21b2a9bbc210ce23ea011ba8069ccd4477bbef66bac4f67ef5cf76c25bf08cba0b4f4bce2a8bdcbfcb5d04d8eb08cd7b94f7fda56a201ad4a257f711a3bad6bbe3a7283afd094110cee9b0bebe4202710408eb9d453383a1012c67335a270c6d350a2e73a751c2afe3f58824dbb28e686ede2bfd1f849bf764b92b4acfec64d535fa5e3e0abd7710498d225827ef8580356ddd66284784768a78f20541e7b449efc7a737c0ea07c5d326ee442f8b904bb31cb48bdc6253d4195e3852b6681f71587c215a9c25923273f30cea8ee437bb4c2be8b476d202e5961113fac24bdfa7e8a5853fa0a58566e54ee4f27da469d74f2f7830e77384be84bd87910513a19503e4015d39595be7126d420cfd154d179a99fd9cf571b732d1f4d81e722aa002e041daef726da950280a7cd0717064a4cd0fbf628aa268ef273d78cfc24079b66e28113db1af91ebab0126007dd9cf10b34324363ef03f6cd109346a1ae690e37ab0977c73e7f4589a7e27ed11e5c47b8fd2d94e3a1d3e371ab799c3c338a45882ca93752b11d1c9093464a66f98eaac2aa9eedde0e0dd2a5a9d9ee8fc7ab65817e885e3e0f0fca752e6f8ce5851442d3668d13b08b531559ec028b73cc6c6b8ec291b1385de28d49f3abe53dd8dda9fa458d14e24a6e4eed23bc7a03a53cf3fb58bf731f206a79cca196b70e4e0f5dcccaabc6861ceef3b2f6fdce7fd03af7de20568df7060ff6bd0907051a32c0eeec9063e5e56e2d2ae650f0c9c95ffa70409ef9a68251597d0dc9d852c986daba92b5cc5f8a2e8e5484f9867a0f73c5c6f71dc258fa559a12c2dc50d181e455a73ccb3d7b384866865740202258b20f00a968690b7b3e591c24346d5b443663fb3d03615db5711f94b24ad4c2418caaac6d974e1c02a8927372fd69b8603f3e0fc7a810ad6b4e14729aed5161cd19c2efb8bb3b71b4230f1505a5655464d525df9c115f5b1808262d4cb0ad4f4f015450e477276d10b743085883aec4d15f18c55e8b2f00cc476d95513adab52a107af848877a25e7119438869bc48b34638aee8452406d300a57f8071b24d25132ddd63746a3e136a9c147990f7baeb5f49273a54ceec9e48dac908cdf404501de9605dbbec533e64f183f7f14da3729d73162a686b56b2e0a8756ff8236dfbf105aac00356cdc6ffe1ee573f55b7c154aa2b75793063eed752734e904270347e3317c2597cb74c40cc071fe0b7c21550501f5c98a7fde8e9e20504406db6c68063cbcfbce75ba04fa8ff16892347939ab027e75b53e888eb203da64954e64e4e6434f3acb041a83670a76ffe9f8c271378348253afc6c8ceace58a8175fef1ea963527774c7945285f8ec33975765080a5418d7e0dec40067fba7d41ad6314d00a52b900f634e0de6a9960b39472b11fc629fbed78f28810df2ae3d7a6898fff401d0462bacb2ed24ea0b689669083b855b6133e6a9703dbcf22b42635f7afac5acf85088399b3f7cf760fb231dc9d01520482121dc18b99b917482cd03aee22cbedf11f5cf4b859a73f7db0e8302f693f6c082da741f38c0de0fe2083f3641d3ce1c832bc6b600af6dc2fb203f2ebe0c2d4ab6aec095c6c97b2a3e065ea9648d904ddb08bf9e683e8428e4fcc38e49aac56846c5329c5049436bcb21bc9f1b5ef425d1822edea68a49a731e3bde9798ecbf667041de72bd148140f8b9f77811840d24588639f12c2f56ea0abb5622add01b1d2e26dfa70a37d8672209d92cf0a7be5b82879317b3ce942b04db32c37705ce1062d966b8eda079fd57a362742c5133a66f233a9288c95c963e12170d05e0006be6e1e4e2e44a8fb44132312bb47d8c0c42618280b1bee95ab6cf8bd1439ada90222557b9e2fa8feaff0a40f8b61747fffaf81789b6790ec53f4029249c494b748547c14829c0237a312d6d4397d9721b2f96944cd60740e21e194392918d6d5a09f027a87d08f639d40df9360e4bcd199b5d7e9b0f0ca5ba3955b363c19324c111bfb1f6620b16b9863e9cef500e65fe94472d22649a15f554fde2d7d79ab2a2d45a9c49279851860525045de5de61f63180e2daa51ff8e3e15ed64d0a6afc69cc945242b763db72c4a30a5b3a53c6639ec20a864caf364856ffd3d906b3195104b950614c634271e1e3feb14db2ab67972d3bee3bb0ab02d480efe6c1a71da4009d926671fc5a9dc2dff722f3f8166fe983da015229fde6c7f669cb79bdeb7f2c184603c0082f3825fbab6e07fe5a254797bb0da7ae2fbc1e2eeee4a9ad91dce2374df053c6c830ee3a0ee7807f5a21df8d760b96a04f08ac3b1fff18ea41508dde66b3242c2311b9c5847e61fa0eca2cfbf8e8aa37486816af3173ed41bf91f85da8c79e36735fa9832bc63a8771a5d621ce3474663e6b49e7d39c6581eccae1cdf87c0d89beb5ac6e828e198811ff27bbaf9a08c03d43b4bace137b9372831ab4b83ca04f69e5712b57afee09bcd7af1a476a18afa5a09ba11d7d3b3c612678f2f8ce670ddad1ae23c9bf99b98090d292644cedadd02305099351708987789ced7be90821674e8714a8b9283a05a536533e956c33464c1130d2f533d013949cd0341bc764babf5fac02b1c4c10bcf9a6c6c2634c1f8918078922ea441997bf84efd4e5929c43ed29fa49d702dbe83f83cdced6e4657576828e27b927d2f6eb59296ae6c54afa34c4924e51d3f68d2c2fdab5caeb8ac1024f6a1fb9791b0beff3567e9e54f144ab2d9aeb98af065f31ce238e81a871cdcd8106d2de3c0110232a3a414b515f858d9095a5d0d3d8d9dbed01040c1019262732394c696e72a9b2b5c0c4d6e7111c1f4254aec6d8dfeef1f5fa242d393a3d4970737a7c8892939abcc6c9cfd4e1e3edeef8ff00001428354e1084941162fa94b0e61f31b57c6a4aab4398303b3deb798c2fdd92b2d20d8f83dd96841420cfb61483d3cc56929e3a52d31ae4dc09344b4abff0f12fbc1fc1f04d2624b2956a1b50"))
	if err != nil {
		t.Fatal(err)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if addr != from {
		fmt.Println("address got", from, "address want", addr)
		t.Fatal("derived address doesn't match")
	}
}

func TestRecipientNormal(t *testing.T) {
	_, addr := defaultTestKey()

	tx, err := decodeTx(common.Hex2Bytes("b90fb800f90fb48301e0f3808301e24101a000000000000000000000000000000000000000000000000000000000000000000a8080c001b9058076da102d3eacb9a92b07e7927b1d0f6cbca1ee11a468b5e356a86a8c9167bd13fb979fdf3ad71722435cd0d50dd75371a5e046e053b5e7c3768a3397609306227b80f7b166795bb2fbfea48bd383c1fe885325a5d342631551e308eec28811803a8b7111cc1bfda888851194dced49f0db9b5fcdac21d5ddc1243d383b2e27ce693aecd9c0a2f2e9d61c7a871b111b5f32bed1d4177c389ed7ef7169eb3606d1147fdde900695ee02222e6500940872b92074b7f2ea5a98f1993b60695d6703369354f2847a66dfb7d0e759db222bcf046069a4d9afc31a2b6408699705b2e551130fe134f9e4a321a893cf07f205ce481e4af507ba49fab1a646304fc3a6821473a003e2326d3390003657b2b9414457313efd579917d9271854aea8311963d89b658dec2382241f4e76874c2c261831f66304cd86b1cc183301c914b01ea5a2fc2ad5aec9776c3fc28da53b084ec72b97604ca1160db67702d8d63262e0357994e048508ec853ecdb1a854421a7cbb45f1c0b6ed73644bdcdf30c4702a981ffcb671c1c14f962ffd23df25df0301684962dc94cdd2e2297ca5891a88492b3c07a9b7d2560d6e100538a900d0033f8d965186b7aafc5b2bd1cedb9b32051ca241619bf7b2762f0d0041b8387e0864183b70d29692b396f0328eab9898c6858ea78ff18333d6509985d14db872122d3d274a3eb3f94f07144048db83e092cd0f2d03cc834ac3e6977eba79db7edac057ce52483b498cc56e71db5e0ab06f059f007305aaa442b19f7d9bd9dd4975f6ddf34717218326d72b7158e9937d9bdd7765c65570827c6e63846e5f335808b68fb73bee818719f1573d79ca9cf72942a8167bca457f06668df2e4db6dc7532f3298e331d4a8f4fa8db74e8983f4b2804650fa173f874d6f5eca7a4da2e93d3e23190fb180b96212f127e99dc0f713ce0043b97980dd875735ec112ac581caa975a4310369303a7f3c92c5cee4f2240d6281d801173558ca3882d63d89643cf61a1314b9990ba8393df5defc90f244b5c4ddb274f70bc63075574a1b98075ee4ea4d54d6d013f98716a2ad19ea50ea9cd5f3686831dbc1873b1f8beb73c8c237730686703de1b298b81d5fa494d9419f52bd9634f07a6becef44eac6176544ee7a5e9e2fe5259961fdcd6ef997488ba94e030f59a73ece7be01cdfb3993afdef989a0cbffbc45f0aa1ea087583455da30cd074957c7960359bf92365a5d0cf4b540aefe68c6008f29919bf39dee86da0f41e33817567103e33572fc3e7885e7e719a53e5a4a9c0598b48066bec565203da29c9847684e27b6b513657a55e7523b940af1c3433282324ddb9577409edc42edb93ad2768c10d39ba4b08ba12fb1ef62bb5613fc1a9393f5b394022f24d5388a9ef4e5fcc0320885a1193c923c34a4673061e391c97aeecfe4a9d8f82f53876573f0f2b2dd7829e024c87bf2a28c61fff826efd6e79a1cd67bde93e15d3cc95981c58f3345afcd649fb16954bd5ef41811273c69f87cd8d39cde8f2c334993819b7e7bd48be9ffbf56e778795f5323b6fdf456e0f1fb3944c1ebd77438751df4e3605d909af261d6d3be39a4cd53defe6662d4460c5415ab769ca81e430d269eb13d5ee1b1bc75f5334fa42804a6579af373a8ecdce83bf72e873928683169901d93b64d24138634f89f12afe5c5843b04781aee69c53a7613d7cea5b43c8f41401c1a62e971e16c3de2eb8f7ab8c5bf644c4e279e0fa093031bebb1ed62e179373e36f9de0b1b12755cf466c97a24cf42aabf4836136374f361671d17fefee278485a3f61fc230e5a3e6f5e36906bd3e87512df79369f557c23b9b12f1f3351afac7305f7fcb439370f4b4b89f5f732a4b47585a775353610a05ff3e0aee062cab62f15193201d85a16a5dc9f0ff52056c23150094090a143e3b749f8195c50abc34b0d9f85b09debe31478892b9e1e3de582c5ca139d426a897185a71fd88b909fe012009d78ccf5ab0b8313a255f404cf02219d53740d589ffc8775f7516da6642453a5be81cb07508691307c4495f258304a41eeb9d2af6e5177f223039ca03aaf909d4eea7b195eb55991b724162e3215742808822eae0bbf40e170aaf04e9c1542c553398a05050163dd4c5f074d63c780e4ef87c9a209a26e82e2305ea2837c61ce67fb6e7321af467d2fc6f260f6ab66837155ee5f29c3e32f52e142640306c22abf8ba5eea4d6fa464bf58a5f33a69a16ab7f3b19151f16e97369b62f8adb6832e90012868905504025066ed11c025f35407022b26aecd611b735761c3aff8380839c791e1deaaf6f46f4189cf936420717c09d35ff736ea694f3f819abe6c427bcc6bc6e6b1304faa3f956eccbb4db4f417e8ac0df3e489ad18b9c3b6e83a51817df3d670aabfaff7da7fc164a9dbe3dda029296d454ab20a2ede6fc3b4d8f60b847855b53dbdfc3680e192ad096e2a6d31b7c1bee37a4c42831cdc166f1c0c62707803d041341c30f7d6f020ce0c820930604e4000e33e0076165a825fd6515817114bd56344e942fa47ef76f785848e206094316fba926f8c8adaadd8f32798120cf807f7233bc1fa0ad40ae3a908af60f21b2a9bbc210ce23ea011ba8069ccd4477bbef66bac4f67ef5cf76c25bf08cba0b4f4bce2a8bdcbfcb5d04d8eb08cd7b94f7fda56a201ad4a257f711a3bad6bbe3a7283afd094110cee9b0bebe4202710408eb9d453383a1012c67335a270c6d350a2e73a751c2afe3f58824dbb28e686ede2bfd1f849bf764b92b4acfec64d535fa5e3e0abd7710498d225827ef8580356ddd66284784768a78f20541e7b449efc7a737c0ea07c5d326ee442f8b904bb31cb48bdc6253d4195e3852b6681f71587c215a9c25923273f30cea8ee437bb4c2be8b476d202e5961113fac24bdfa7e8a5853fa0a58566e54ee4f27da469d74f2f7830e77384be84bd87910513a19503e4015d39595be7126d420cfd154d179a99fd9cf571b732d1f4d81e722aa002e041daef726da950280a7cd0717064a4cd0fbf628aa268ef273d78cfc24079b66e28113db1af91ebab0126007dd9cf10b34324363ef03f6cd109346a1ae690e37ab0977c73e7f4589a7e27ed11e5c47b8fd2d94e3a1d3e371ab799c3c338a45882ca93752b11d1c9093464a66f98eaac2aa9eedde0e0dd2a5a9d9ee8fc7ab65817e885e3e0f0fca752e6f8ce5851442d3668d13b08b531559ec028b73cc6c6b8ec291b1385de28d49f3abe53dd8dda9fa458d14e24a6e4eed23bc7a03a53cf3fb58bf731f206a79cca196b70e4e0f5dcccaabc6861ceef3b2f6fdce7fd03af7de20568df7060ff6bd0907051a32c0eeec9063e5e56e2d2ae650f0c9c95ffa70409ef9a68251597d0dc9d852c986daba92b5cc5f8a2e8e5484f9867a0f73c5c6f71dc258fa559a12c2dc50d181e455a73ccb3d7b384866865740202258b20f00a968690b7b3e591c24346d5b443663fb3d03615db5711f94b24ad4c2418caaac6d974e1c02a8927372fd69b8603f3e0fc7a810ad6b4e14729aed5161cd19c2efb8bb3b71b4230f1505a5655464d525df9c115f5b1808262d4cb0ad4f4f015450e477276d10b743085883aec4d15f18c55e8b2f00cc476d95513adab52a107af848877a25e7119438869bc48b34638aee8452406d300a57f8071b24d25132ddd63746a3e136a9c147990f7baeb5f49273a54ceec9e48dac908cdf404501de9605dbbec533e64f183f7f14da3729d73162a686b56b2e0a8756ff8236dfbf105aac00356cdc6ffe1ee573f55b7c154aa2b75793063eed752734e904270347e3317c2597cb74c40cc071fe0b7c21550501f5c98a7fde8e9e20504406db6c68063cbcfbce75ba04fa8ff16892347939ab027e75b53e888eb203da64954e64e4e6434f3acb041a83670a76ffe9f8c271378348253afc6c8ceace58a8175fef1ea963527774c7945285f8ec33975765080a5418d7e0dec40067fba7d41ad6314d00a52b900f634e0de6a9960b39472b11fc629fbed78f28810df2ae3d7a6898fff401d0462bacb2ed24ea0b689669083b855b6133e6a9703dbcf22b42635f7afac5acf85088399b3f7cf760fb231dc9d01520482121dc18b99b917482cd03aee22cbedf11f5cf4b859a73f7db0e8302f693f6c082da741f38c0de0fe2083f3641d3ce1c832bc6b600af6dc2fb203f2ebe0c2d4ab6aec095c6c97b2a3e065ea9648d904ddb08bf9e683e8428e4fcc38e49aac56846c5329c5049436bcb21bc9f1b5ef425d1822edea68a49a731e3bde9798ecbf667041de72bd148140f8b9f77811840d24588639f12c2f56ea0abb5622add01b1d2e26dfa70a37d8672209d92cf0a7be5b82879317b3ce942b04db32c37705ce1062d966b8eda079fd57a362742c5133a66f233a9288c95c963e12170d05e0006be6e1e4e2e44a8fb44132312bb47d8c0c42618280b1bee95ab6cf8bd1439ada90222557b9e2fa8feaff0a40f8b61747fffaf81789b6790ec53f4029249c494b748547c14829c0237a312d6d4397d9721b2f96944cd60740e21e194392918d6d5a09f027a87d08f639d40df9360e4bcd199b5d7e9b0f0ca5ba3955b363c19324c111bfb1f6620b16b9863e9cef500e65fe94472d22649a15f554fde2d7d79ab2a2d45a9c49279851860525045de5de61f63180e2daa51ff8e3e15ed64d0a6afc69cc945242b763db72c4a30a5b3a53c6639ec20a864caf364856ffd3d906b3195104b950614c634271e1e3feb14db2ab67972d3bee3bb0ab02d480efe6c1a71da4009d926671fc5a9dc2dff722f3f8166fe983da015229fde6c7f669cb79bdeb7f2c184603c0082f3825fbab6e07fe5a254797bb0da7ae2fbc1e2eeee4a9ad91dce2374df053c6c830ee3a0ee7807f5a21df8d760b96a04f08ac3b1fff18ea41508dde66b3242c2311b9c5847e61fa0eca2cfbf8e8aa37486816af3173ed41bf91f85da8c79e36735fa9832bc63a8771a5d621ce3474663e6b49e7d39c6581eccae1cdf87c0d89beb5ac6e828e198811ff27bbaf9a08c03d43b4bace137b9372831ab4b83ca04f69e5712b57afee09bcd7af1a476a18afa5a09ba11d7d3b3c612678f2f8ce670ddad1ae23c9bf99b98090d292644cedadd02305099351708987789ced7be90821674e8714a8b9283a05a536533e956c33464c1130d2f533d013949cd0341bc764babf5fac02b1c4c10bcf9a6c6c2634c1f8918078922ea441997bf84efd4e5929c43ed29fa49d702dbe83f83cdced6e4657576828e27b927d2f6eb59296ae6c54afa34c4924e51d3f68d2c2fdab5caeb8ac1024f6a1fb9791b0beff3567e9e54f144ab2d9aeb98af065f31ce238e81a871cdcd8106d2de3c0110232a3a414b515f858d9095a5d0d3d8d9dbed01040c1019262732394c696e72a9b2b5c0c4d6e7111c1f4254aec6d8dfeef1f5fa242d393a3d4970737a7c8892939abcc6c9cfd4e1e3edeef8ff00001428354e1084941162fa94b0e61f31b57c6a4aab4398303b3deb798c2fdd92b2d20d8f83dd96841420cfb61483d3cc56929e3a52d31ae4dc09344b4abff0f12fbc1fc1f04d2624b2956a1b50"))
	if err != nil {
		t.Fatal(err)
	}

	from, err := Sender(NewLondonSignerDefaultChain(), tx)
	if err != nil {
		t.Fatal(err)
	}
	if addr != from {
		t.Fatal("derived address doesn't match")
	}
}

// Tests that if multiple transactions have the same price, the ones seen earlier
// are prioritized to avoid network spam attacks aiming for a specific ordering.
func TestTransactionSort(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 5)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)

		for i := 0; i < 5; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionSortIncreasing(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 4)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	txnCount := 0
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		txnCount = txnCount + 1
		for i := 0; i < txnCount; i++ {
			tx, err := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			if err != nil {
				fmt.Println(err)
				t.Fatalf("failed")
			}
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionSortDecreasing(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 4)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	txnCount := 4
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		txnCount = txnCount - 1
		for i := 0; i < txnCount; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionSortIncreaseDecrease(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 6)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	txnCount := 0
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		if txnCount == 2 {
			txnCount = txnCount - 1
		} else {
			txnCount = txnCount + 1
		}
		for i := 0; i < txnCount; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionSortSingle(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 1)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		for i := 0; i < 1; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionSortSingleAccount(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 1)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	txnCount := 5
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)
		for i := 0; i < txnCount; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			groups[addr] = append(groups[addr], tx)
			fmt.Println("txhash", tx.Hash(), addr)
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionSortNoTxns(t *testing.T) {
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}

	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	overallCount := 0
	ok := txset.NextCursor()
	for ok == true {
		txn := txset.PeekCursor()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce())
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func testTransactionNonceOrder_byCount(txnCount int, t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 1)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)

		txnList := make([]*Transaction, 0)
		for i := 0; i < txnCount; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			txnList = append(txnList, tx)
			//groups[addr] = append(groups[addr], tx)
			//fmt.Println("txhash", tx.Hash(), addr)
		}
		for j := len(txnList) - 1; j >= 0; j-- {
			groups[addr] = append(groups[addr], txnList[j])
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	prevNonce := uint64(0)
	for ok == true {
		txn := txset.PeekCursor()
		if txn.Nonce() < prevNonce {
			fmt.Println("failed", txn.Hash(), txn.Nonce(), prevNonce)
			t.Errorf("failed")
			t.Fatalf("failed")
		}
		prevNonce = txn.Nonce()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce(), prevNonce)
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != overallCount {
		t.Errorf("test count failed")
	}
	fmt.Println("count", count)
}

func TestTransactionNonceOrder(t *testing.T) {
	testTransactionNonceOrder_byCount(10, t)
	testTransactionNonceOrder_byCount(1, t)
	testTransactionNonceOrder_byCount(2, t)
}

func testTransactionNonceOrder_skip_byCount(txnCount int, skipMap map[int]bool, outputCount int, t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*signaturealgorithm.PrivateKey, 1)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = cryptobase.SigAlg.GenerateKey()
	}
	signer := NewLondonSignerDefaultChain()

	// Generate a batch of transactions with overlapping prices, but different creation times
	groups := map[common.Address]Transactions{}
	overallCount := 0
	for start, key := range keys {
		addr := cryptobase.SigAlg.PublicKeyToAddressNoError(&key.PublicKey)

		txnList := make([]*Transaction, 0)
		for i := 0; i < txnCount; i++ {
			tx, _ := SignTx(NewTransaction(uint64(i), common.Address{}, big.NewInt(100), 100, big.NewInt(1), nil), signer, key)
			tx.time = time.Unix(0, int64(len(keys)-start))
			overallCount = overallCount + 1
			txnList = append(txnList, tx)
			//groups[addr] = append(groups[addr], tx)
			//fmt.Println("txhash", tx.Hash(), addr)
		}
		for j := len(txnList) - 1; j >= 0; j = j - 1 {
			_, ok := skipMap[j]
			if ok {
				continue
			}
			groups[addr] = append(groups[addr], txnList[j])
		}
	}
	// Sort the transactions and cross check the nonce ordering
	parentHash := common.BytesToHash([]byte("test parent hash"))
	txset := NewTransactionsByNonce(signer, groups, parentHash)

	count := 0
	ok := txset.NextCursor()
	prevNonce := uint64(0)
	for ok == true {
		txn := txset.PeekCursor()
		if txn.Nonce() < prevNonce {
			fmt.Println("failed", txn.Hash(), txn.Nonce(), prevNonce)
			t.Errorf("failed")
			t.Fatalf("failed")
		}
		prevNonce = txn.Nonce()
		from, _ := Sender(signer, txn)
		fmt.Println("Cursor", txn.Hash(), from, txn.Nonce(), prevNonce)
		ok = txset.NextCursor()
		count = count + 1
	}
	if count != outputCount {
		fmt.Println("count", count, outputCount)
		t.Errorf("test count failed")
	}
}

func TestTransactionNonceOrderSkip(t *testing.T) {
	testTransactionNonceOrder_skip_byCount(10, map[int]bool{1: true}, 1, t)
	testTransactionNonceOrder_skip_byCount(10, map[int]bool{5: true}, 5, t)
	testTransactionNonceOrder_skip_byCount(10, map[int]bool{0: true}, 9, t)
	testTransactionNonceOrder_skip_byCount(10, map[int]bool{9: true}, 9, t)
	testTransactionNonceOrder_skip_byCount(0, map[int]bool{0: true}, 0, t)
	testTransactionNonceOrder_skip_byCount(1, map[int]bool{0: true}, 0, t)
}

// TestTransactionCoding tests serializing/de-serializing to/from rlp and JSON.
func TestTransactionCoding(t *testing.T) {
	key, err := cryptobase.SigAlg.GenerateKey()
	if err != nil {
		t.Fatalf("could not generate key: %v", err)
	}
	var (
		signer    = NewLondonSigner(big.NewInt(DEFAULT_CHAIN_ID))
		addr      = common.HexToAddress("0x0000000000000000000000000000000000000001")
		recipient = common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
		accesses  = AccessList{{Address: addr, StorageKeys: []common.Hash{{0}}}}
	)
	for i := uint64(0); i < 500; i++ {
		var txdata TxData

		// Legacy tx.
		txdata = &DefaultFeeTx{
			ChainID:    big.NewInt(DEFAULT_CHAIN_ID),
			Nonce:      i,
			To:         &recipient,
			Gas:        1,
			MaxGasTier: GAS_TIER_DEFAULT,
			AccessList: accesses,
			Data:       []byte("abcdef"),
		}

		tx, err := SignNewTx(key, signer, txdata)
		if err != nil {
			t.Fatalf("could not sign transaction: %v", err)
		}
		// RLP
		parsedTx, err := encodeDecodeBinary(tx)
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(parsedTx, tx)

		// JSON
		parsedTx, err = encodeDecodeJSON(tx)
		if err != nil {
			t.Fatal(err)
		}
		assertEqual(parsedTx, tx)
	}
}

func encodeDecodeJSON(tx *Transaction) (*Transaction, error) {
	data, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("json encoding failed: %v", err)
	}
	var parsedTx = &Transaction{}
	if err := json.Unmarshal(data, &parsedTx); err != nil {
		return nil, fmt.Errorf("json decoding failed: %v", err)
	}
	return parsedTx, nil
}

func encodeDecodeBinary(tx *Transaction) (*Transaction, error) {
	data, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("rlp encoding failed: %v", err)
	}
	var parsedTx = &Transaction{}
	if err := parsedTx.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("rlp decoding failed: %v", err)
	}
	return parsedTx, nil
}

func assertEqual(orig *Transaction, cpy *Transaction) error {
	// compare nonce, price, gaslimit, recipient, amount, payload, V, R, S
	if want, got := orig.Hash(), cpy.Hash(); want != got {
		return fmt.Errorf("parsed tx differs from original tx, want %v, got %v", want, got)
	}
	if want, got := orig.ChainId(), cpy.ChainId(); want.Cmp(got) != 0 {
		return fmt.Errorf("invalid chain id, want %d, got %d", want, got)
	}
	if orig.AccessList() != nil {
		if !reflect.DeepEqual(orig.AccessList(), cpy.AccessList()) {
			return fmt.Errorf("access list wrong!")
		}
	}
	return nil
}
