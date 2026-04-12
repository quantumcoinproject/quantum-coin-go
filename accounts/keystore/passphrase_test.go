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

package keystore

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/quantumcoinproject/quantum-coin-go/accounts"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

// Tests that a json key file can be decrypted and encrypted in multiple rounds.
func TestKeyEncryptDecrypt(t *testing.T) {

	// Do a few rounds of decryption and encryption
	for i := 0; i < 3; i++ {
		dir, ks := tmpKeyStore(t, true)
		defer os.RemoveAll(dir)

		pass := strconv.Itoa(i)
		a1, err := ks.NewAccount(pass)
		if err != nil {
			t.Fatal(err)
		}
		if err := ks.Unlock(a1, pass); err != nil {
			t.Fatal(err)
		}
		if _, err := ks.SignHash(accounts.Account{Address: a1.Address}, testSigData); err != nil {
			t.Fatal(err)
		}
	}

}

func makePreExpansionSeed(size int) []byte {
	preExpansionSeed := make([]byte, size)
	for i := 0; i < size; i++ {
		preExpansionSeed[i] = byte(i)
	}
	return preExpansionSeed
}

func testEncryptDecryptPreExpansionSeed(t *testing.T, seedSize int) {
	t.Helper()
	preExpansionSeed := makePreExpansionSeed(seedSize)
	passphrase := "testpassword"

	privKey, err := cryptobase.GenerateKeyFromPreExpansionSeed(preExpansionSeed)
	if err != nil {
		t.Fatal("GenerateKeyFromPreExpansionSeed failed:", err)
	}
	sigAlgPtr, err := cryptobase.GetSigAlgForPrivateKey(privKey.PriData)
	if err != nil {
		t.Fatal("GetSigAlgForPrivateKey failed:", err)
	}
	sigAlg := *sigAlgPtr
	address, err := sigAlg.PublicKeyToAddress(&privKey.PublicKey)
	if err != nil {
		t.Fatal("PublicKeyToAddress failed:", err)
	}

	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatal("uuid failed:", err)
	}

	walletJson, err := EncryptPreExpansionSeed(preExpansionSeed, address, id, passphrase, veryLightScryptN, veryLightScryptP)
	if err != nil {
		t.Fatal("EncryptPreExpansionSeed failed:", err)
	}

	key, err := DecryptKey(walletJson, passphrase)
	if err != nil {
		t.Fatal("DecryptKey failed:", err)
	}

	if key.Address != address {
		t.Fatalf("address mismatch: got %s, want %s", key.Address.Hex(), address.Hex())
	}
	if len(key.PrivateKey.PriData) == 0 {
		t.Fatal("private key is empty")
	}
	if len(key.PrivateKey.PubData) == 0 {
		t.Fatal("public key is empty")
	}
}

func TestEncryptDecryptPreExpansionSeed_64(t *testing.T) {
	testEncryptDecryptPreExpansionSeed(t, cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PreExpansionSeedSize())
}

func TestEncryptDecryptPreExpansionSeed_96(t *testing.T) {
	testEncryptDecryptPreExpansionSeed(t, cryptobase.SigAlgHybridEds.PreExpansionSeedSize())
}

func TestEncryptDecryptPreExpansionSeed_72(t *testing.T) {
	testEncryptDecryptPreExpansionSeed(t, cryptobase.SigAlgHybridMlDsaEddsaSlhDsa5.PreExpansionSeedSize())
}

func TestDecryptKeyV5_WrongPassword(t *testing.T) {
	preExpansionSeed := makePreExpansionSeed(cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PreExpansionSeedSize())
	privKey, err := cryptobase.GenerateKeyFromPreExpansionSeed(preExpansionSeed)
	if err != nil {
		t.Fatal(err)
	}
	sigAlgPtr, err := cryptobase.GetSigAlgForPrivateKey(privKey.PriData)
	if err != nil {
		t.Fatal(err)
	}
	address, err := (*sigAlgPtr).PublicKeyToAddress(&privKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := uuid.NewRandom()

	walletJson, err := EncryptPreExpansionSeed(preExpansionSeed, address, id, "correctpassword", veryLightScryptN, veryLightScryptP)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptKey(walletJson, "wrongpassword")
	if err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt, got: %v", err)
	}
}

func TestDecryptKeyV5_InvalidSeedSize(t *testing.T) {
	invalidSeed := make([]byte, 16)
	passphrase := "testpassword"

	cryptoStruct, err := EncryptDataV4(invalidSeed, []byte(passphrase), veryLightScryptN, veryLightScryptP)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := uuid.NewRandom()
	encryptedKeyJson := encryptedKeyJSONV4{
		hex.EncodeToString(make([]byte, 32)),
		cryptoStruct,
		id.String(),
		5,
	}
	walletJson, err := json.Marshal(encryptedKeyJson)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptKey(walletJson, passphrase)
	if err == nil {
		t.Fatal("expected error for invalid seed size")
	}
}

func TestDecryptKeyV5_TamperedAddress(t *testing.T) {
	preExpansionSeed := makePreExpansionSeed(cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PreExpansionSeedSize())
	passphrase := "testpassword"

	cryptoStruct, err := EncryptDataV4(preExpansionSeed, []byte(passphrase), veryLightScryptN, veryLightScryptP)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := uuid.NewRandom()
	tamperedAddress := hex.EncodeToString(make([]byte, 32))
	encryptedKeyJson := encryptedKeyJSONV4{
		tamperedAddress,
		cryptoStruct,
		id.String(),
		5,
	}
	walletJson, err := json.Marshal(encryptedKeyJson)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecryptKey(walletJson, passphrase)
	if err == nil {
		t.Fatal("expected error for tampered address, got nil")
	}
	if !strings.Contains(err.Error(), "address mismatch") {
		t.Fatalf("expected address mismatch error, got: %v", err)
	}
}

func TestDecryptV5Fixture_Seed64(t *testing.T) {
	testDecryptV5Fixture(t, "v5-seed64-wallet.json", cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PreExpansionSeedSize())
}

func TestDecryptV5Fixture_Seed96(t *testing.T) {
	testDecryptV5Fixture(t, "v5-seed96-wallet.json", cryptobase.SigAlgHybridEds.PreExpansionSeedSize())
}

func TestDecryptV5Fixture_Seed72(t *testing.T) {
	testDecryptV5Fixture(t, "v5-seed72-wallet.json", cryptobase.SigAlgHybridMlDsaEddsaSlhDsa5.PreExpansionSeedSize())
}

func testDecryptV5Fixture(t *testing.T, filename string, seedSize int) {
	t.Helper()
	fixtureFile := filepath.Join("testdata", "keystore", filename)
	walletJson, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("could not read fixture %s: %v", filename, err)
	}

	key, err := DecryptKey(walletJson, "testpassword")
	if err != nil {
		t.Fatalf("DecryptKey failed for %s: %v", filename, err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(walletJson, &m); err != nil {
		t.Fatal(err)
	}
	jsonAddress := m["address"].(string)
	keyAddressHex := hex.EncodeToString(key.Address[:])
	if jsonAddress != keyAddressHex {
		t.Fatalf("address mismatch in %s: json=%s, derived=%s", filename, jsonAddress, keyAddressHex)
	}
}

func TestGenerateV5Fixtures(t *testing.T) {
	if os.Getenv("GENERATE_V5_FIXTURES") == "" {
		t.Skip("Set GENERATE_V5_FIXTURES=1 to regenerate V5 fixture files")
	}
	seedSizes := []struct {
		size     int
		filename string
	}{
		{cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PreExpansionSeedSize(), "v5-seed64-wallet.json"},
		{cryptobase.SigAlgHybridEds.PreExpansionSeedSize(), "v5-seed96-wallet.json"},
		{cryptobase.SigAlgHybridMlDsaEddsaSlhDsa5.PreExpansionSeedSize(), "v5-seed72-wallet.json"},
	}
	for _, ss := range seedSizes {
		preExpansionSeed := makePreExpansionSeed(ss.size)
		privKey, err := cryptobase.GenerateKeyFromPreExpansionSeed(preExpansionSeed)
		if err != nil {
			t.Fatal(err)
		}
		sigAlgPtr, err := cryptobase.GetSigAlgForPrivateKey(privKey.PriData)
		if err != nil {
			t.Fatal(err)
		}
		address, err := (*sigAlgPtr).PublicKeyToAddress(&privKey.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := uuid.NewRandom()
		walletJson, err := EncryptPreExpansionSeed(preExpansionSeed, address, id, "testpassword", veryLightScryptN, veryLightScryptP)
		if err != nil {
			t.Fatal(err)
		}
		outPath := filepath.Join("testdata", "keystore", ss.filename)
		if err := os.WriteFile(outPath, walletJson, 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("Generated %s (address: %s)", ss.filename, address.Hex())
	}
}

func TestDecryptV5Fixture_WrongPassword(t *testing.T) {
	fixtureFile := filepath.Join("testdata", "keystore", "v5-seed64-wallet.json")
	walletJson, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatalf("could not read fixture: %v", err)
	}

	_, err = DecryptKey(walletJson, "wrongpassword")
	if err != ErrDecrypt {
		t.Fatalf("expected ErrDecrypt, got: %v", err)
	}
}
