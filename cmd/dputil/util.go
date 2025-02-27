package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/accounts"
	"github.com/QuantumCoinProject/qc/accounts/abi/bind"
	"github.com/QuantumCoinProject/qc/accounts/keystore"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/consensus/proofofstake"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/params"
	"github.com/QuantumCoinProject/qc/systemcontracts/conversion"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking/stakingv1"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking/stakingv2"
	"github.com/QuantumCoinProject/qc/token"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const GAS_LIMIT_ENV = "GAS_LIMIT"
const DEFAULT_GAS_LIMIT = uint64(210000)

type KeyStore struct {
	Handle *keystore.KeyStore
}

type BalanceData struct {
	Result struct {
		Balance string `json:"_balance"`
		Nonce   string `json:"nonce"`
	}
}

func etherToWei(val *big.Int) *big.Int {
	return new(big.Int).Mul(val, big.NewInt(params.Ether))
}

func weiToEther(val *big.Int) *big.Int {
	return new(big.Int).Div(val, big.NewInt(params.Ether))
}

func etherToWeiFloat(eth *big.Float) *big.Int {
	truncInt, _ := eth.Int(nil)
	truncInt = new(big.Int).Mul(truncInt, big.NewInt(params.Ether))
	fracStr := strings.Split(fmt.Sprintf("%.18f", eth), ".")[1]
	fracStr += strings.Repeat("0", 18-len(fracStr))
	fracInt, _ := new(big.Int).SetString(fracStr, 10)
	wei := new(big.Int).Add(truncInt, fracInt)
	return wei
}

func getBalance(address string) (ethBalance string, weiBalance string, err error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return "", "", err
	}
	balance, err := client.BalanceAt(context.Background(), common.HexToAddress(address), nil)
	if err != nil {
		return "", "", err
	}
	return weiToEther(balance).String(), balance.String(), nil
}

func requestGetBalance(address string) (ethBalance string, weiBalance string, nonce string, err error) {
	request, err := http.NewRequest("GET", READ_API_URL+"/api/accounts/"+address+"/balance", nil)
	if err != nil {
		return "", "", "", err
	}
	request.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", "", "", err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", "", "", err
	}

	var balanceData BalanceData
	err = json.Unmarshal(body, &balanceData)
	if err != nil {
		return "", "", "", err
	}

	if len(balanceData.Result.Balance) == 0 {
		balanceData.Result.Balance = "0"
	}
	if len(balanceData.Result.Nonce) == 0 {
		balanceData.Result.Nonce = "0"
	}

	balance := new(big.Int)
	_, err = fmt.Sscan(balanceData.Result.Balance, balance)
	if err != nil {
		return "", "", "", err
	}

	return weiToEther(balance).String(), balanceData.Result.Balance, balanceData.Result.Nonce, nil
}

func findAllAddresses() ([]string, error) {
	keyfileDir := os.Getenv("DP_KEY_FILE_DIR")
	if len(keyfileDir) == 0 {
		return nil, errors.New("Both DP_KEY_FILE and DP_KEY_FILE_DIR environment variables not set")
	}

	files, err := ioutil.ReadDir(keyfileDir)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	var addresses []string
	addresses = make([]string, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		columns := strings.Split(file.Name(), "--")
		if len(columns) != 3 {
			continue
		}
		addresses = append(addresses, columns[2])
	}

	return addresses, nil
}

func findKeyFile(keyAddress string) (string, error) {
	keyfile := os.Getenv("DP_KEY_FILE")
	if len(keyfile) > 0 {
		return keyfile, nil
	}

	keyfileDir := os.Getenv("DP_KEY_FILE_DIR")
	if len(keyfileDir) == 0 {
		return "", errors.New("Both DP_KEY_FILE and DP_KEY_FILE_DIR environment variables not set")
	}

	files, err := ioutil.ReadDir(keyfileDir)
	if err != nil {
		log.Fatal(err)
		return "", err
	}

	addr := strings.ToLower(strings.Replace(keyAddress, "0x", "", 1))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.Contains(strings.ToLower(file.Name()), addr) {
			return filepath.Join(keyfileDir, file.Name()), nil
		}
	}

	return "", errors.New("could not find key file")
}

type ConnectionContext struct {
	From   string
	Client *ethclient.Client
	Key    *keystore.Key
}

func GetKeyFromFile(keyFile string, accPwd string) (*signaturealgorithm.PrivateKey, error) {
	secretKey, err := ReadDataFile(keyFile)
	if err != nil {
		return nil, err
	}

	password := accPwd
	key, err := keystore.DecryptKey(secretKey, password)
	if err != nil {
		return nil, err
	}

	return key.PrivateKey, nil
}

func GetConnectionContext(from string) (*ConnectionContext, error) {
	keyFile, err := findKeyFile(from)
	if err != nil {
		return nil, err
	}

	secretKey, err := ReadDataFile(keyFile)
	if err != nil {
		return nil, err
	}

	password := os.Getenv("DP_ACC_PWD")
	key, err := keystore.DecryptKey(secretKey, password)
	if err != nil {
		return nil, err
	}

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return nil, err
	}

	return &ConnectionContext{
		From:   from,
		Client: client,
		Key:    key,
	}, nil
}

func sendVia(connectionContext *ConnectionContext, to string, quantity string, nonce uint64) (string, uint64, error) {
	if connectionContext == nil {
		return "", 0, errors.New("nil")
	}
	fromAddress := common.HexToAddress(connectionContext.From)
	toAddress := common.HexToAddress(to)

	if nonce == 0 {
		nonceTmp, err := connectionContext.Client.PendingNonceAt(context.Background(), fromAddress)
		if err != nil {
			return "", 0, err
		}
		nonce = nonceTmp
	}

	chainID, err := connectionContext.Client.NetworkID(context.Background())
	if err != nil {
		return "", 0, err
	}
	gasLimit := uint64(21000)
	gasPrice, err := connectionContext.Client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", 0, err
	}

	v, err := ParseBigFloat(quantity)
	if err != nil {
		return "", 0, err
	}

	value := etherToWeiFloat(v)

	var data []byte
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), connectionContext.Key.PrivateKey)
	if err != nil {
		return "", 0, err
	}
	err = connectionContext.Client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", 0, err
	}

	fmt.Println("Sent Transaction", "from", fromAddress, "to", toAddress, "quantity", quantity, "Transaction", signedTx.Hash().Hex())
	return signedTx.Hash().Hex(), nonce, nil
}

func send(from string, to string, quantity string) (string, error) {
	keyFile, err := findKeyFile(from)
	if err != nil {
		return "", err
	}

	fmt.Println("keyFile", keyFile)
	secretKey, err := ReadDataFile(keyFile)
	if err != nil {
		return "", err
	}
	password := os.Getenv("DP_ACC_PWD")
	key, err := keystore.DecryptKey(secretKey, password)
	if err != nil {
		return "", err
	}

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return "", err
	}

	fromAddress := common.HexToAddress(from)
	toAddress := common.HexToAddress(to)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return "", err
	}

	envNonce := os.Getenv("NONCE_VALUE")
	if len(envNonce) > 0 {
		nonceVal, err := strconv.ParseInt(envNonce, 10, 64)
		if err == nil {
			nonce = uint64(nonceVal)
			fmt.Println("Using nonce passed from environment variable NONCE_VALUE: ", nonceVal)
		} else {
			fmt.Println("Not Using nonce passed from environment variable NONCE_VALUE due to error: ", err)
			return "", err
		}
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return "", err
	}
	gasLimit := uint64(21000)

	v, err := ParseBigFloat(quantity)
	if err != nil {
		return "", err
	}

	value := etherToWeiFloat(v)

	var data []byte
	tx := types.NewDefaultFeeTransaction(chainID, nonce, &toAddress, value, gasLimit, types.GAS_TIER_DEFAULT, data)
	fmt.Println("chainID", chainID)

	signedTx, err := types.SignTx(tx, types.NewLondonSigner(chainID), key.PrivateKey)
	if err != nil {
		fmt.Println("signedTx err", err)
		return "", err
	}
	fmt.Println("signedTx ok")
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}

	fmt.Println("Sent Transaction", "from", fromAddress, "to", toAddress, "quantity", quantity, "Transaction", signedTx.Hash().Hex())
	return signedTx.Hash().Hex(), nil
}

func GetTransaction(txnHash string) (string, error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return "", err
	}
	hash := common.HexToHash(txnHash)
	fmt.Println("hash", hash)
	return client.RawTransactionByHash(context.Background(), hash)
}

// ParseBigFloat parse string value to big.Float
func ParseBigFloat(value string) (*big.Float, error) {
	f := new(big.Float)
	f.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(value, f)
	return f, err
}

func ReadDataFile(filename string) ([]byte, error) {
	// Open our jsonFile
	jsonFile, err := os.Open(filename)
	// if we os.Open returns an error then handle it
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}

	fmt.Println("Successfully Opened ", filename)
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	return byteValue, nil
}

func (ks *KeyStore) CreateNewKeys(password string) accounts.Account {
	account, err := ks.Handle.NewAccount(password)
	if err != nil {
		log.Println(err.Error())
	}
	return account
}

func (ks *KeyStore) GetKeysByAddress(address string) (accounts.Account, error) {
	var account accounts.Account
	var err error
	if ks.Handle.HasAddress(common.HexToAddress(address)) {
		if account, err = ks.Handle.Find(accounts.Account{Address: common.HexToAddress(address)}); err != nil {
			return accounts.Account{}, err
		}
	}
	return account, nil
}

func (ks *KeyStore) GetAllKeys() []accounts.Account {
	return ks.Handle.Accounts()
}

func SetUpKeyStore() *KeyStore {
	dataDir := os.Getenv("DP_DATA_PATH")
	if dataDir == "" {
		dataDir = "data"
	}

	ks := &KeyStore{}
	ks.Handle = keystore.NewKeyStore(dataDir, keystore.LightScryptN, keystore.LightScryptP)
	return ks
}

func convertCoins(ethAddress string, ethSignature string, key *signaturealgorithm.PrivateKey) error {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}
	contractAddress := common.HexToAddress(conversion.CONVERSION_CONTRACT)

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return err
	}
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		return err
	}
	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = DEFAULT_GAS_LIMIT

	contract, err := conversion.NewConversion(contractAddress, client)
	if err != nil {
		return err
	}

	tx, err := contract.RequestConversion(txnOpts, ethAddress, ethSignature)
	if err != nil {
		return err
	}

	fmt.Println("Your request to get the quantum coins has been added to the queue for processing. Please check your account balance after 10 minutes.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println("Your can you use the following command to check your account balance: ")
	fmt.Println("dputil balance [YOUR_QUANTUM_ADDRESS]")
	fmt.Println("Do double check that you have backed up your quantum wallet safely in multiple devices and offline backups. And remember your password!")
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func requestConvertCoins(ethAddress string, ethSignature string, key *signaturealgorithm.PrivateKey) error {

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}
	_, _, n, err := requestGetBalance(fromAddress.String())
	if err != nil {
		return err
	}

	var nonce uint64
	fmt.Sscan(n, &nonce)

	contractAddress := common.HexToAddress(conversion.CONVERSION_CONTRACT)

	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = DEFAULT_GAS_LIMIT

	method := conversion.GetContract_Method_requestConversion()
	abiData, err := conversion.GetConversionContract_ABI()
	if err != nil {
		return err
	}

	input, err := abiData.Pack(method, ethAddress, ethSignature)
	if err != nil {
		return err
	}

	baseTx := types.NewDefaultFeeTransactionSimple(nonce, &contractAddress, txnOpts.Value,
		txnOpts.GasLimit, input)

	var rawTx *types.Transaction
	rawTx = types.NewTx(baseTx)

	if txnOpts.Signer == nil {
		return errors.New("no signer to authorize the transaction with")
	}

	signTx, err := txnOpts.Signer(txnOpts.From, rawTx)
	if err != nil {
		return err
	}

	signTxBinary, err := signTx.MarshalBinary()
	if err != nil {
		return err
	}

	tx := signTx
	txData := hexutil.Encode(signTxBinary)

	var jsonStr = []byte(`{"txnData" : "` + txData + `"}`)

	request, err := http.NewRequest("POST", WRITE_API_URL+"/api/transactions", bytes.NewBuffer(jsonStr))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	fmt.Println("Your request to get the quantum dp coins has been added to the queue for processing. Please check your account balance after 10 minutes.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println("Your can you use the following command to check your account balance: ")
	fmt.Println("dputil balance [YOUR_QUANTUM_ADDRESS]")
	fmt.Println("Do double check that you have backed up your quantum wallet safely in multiple devices and offline backups. And remember your password!")
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func newDeposit(validatorAddress string, depositAmount string, key *signaturealgorithm.PrivateKey) error {

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(250000)

	val, _ := ParseBigFloat(depositAmount)
	txnOpts.Value = etherToWeiFloat(val)

	blockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		return err
	}

	var tx *types.Transaction
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contract, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.NewDeposit(txnOpts, common.HexToAddress(validatorAddress))
		if err != nil {
			return err
		}
	} else {
		contract, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.NewDeposit(txnOpts, common.HexToAddress(validatorAddress))
		if err != nil {
			return err
		}
	}

	fmt.Println("Your request to deposit has been added to the queue for processing. Please check your account balance after 10 minutes.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func initiateWithdrawal(key *signaturealgorithm.PrivateKey) error {

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = DEFAULT_GAS_LIMIT

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	var tx *types.Transaction
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contract, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.InitiateWithdrawal(txnOpts)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Operation not supported. Use partial withdraw instead")
		return nil
	}

	fmt.Println("Your request to initiate withdrawal has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func getGasLimit() (uint64, error) {
	gasLimitEnv := os.Getenv(GAS_LIMIT_ENV)
	if len(gasLimitEnv) > 0 {
		gasLimit, err := strconv.ParseUint(gasLimitEnv, 10, 64)
		if err != nil {
			fmt.Println("Error parsing gas limit, err")
			return gasLimit, err
		}
		fmt.Println("Using gas limit passed using environment variable", gasLimit)
		return gasLimit, nil
	} else {
		return DEFAULT_GAS_LIMIT, nil
	}
}

func completeWithdrawal(key *signaturealgorithm.PrivateKey) error {

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit, err = getGasLimit()
	if err != nil {
		return err
	}

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	var tx *types.Transaction
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contract, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.CompleteWithdrawal(txnOpts)
		if err != nil {
			return err
		}
	} else {
		contract, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.CompleteWithdrawal(txnOpts)
		if err != nil {
			return err
		}
	}

	fmt.Println("Your request to complete withdrawal has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func getBalanceOfDepositor(dep string) (*big.Int, error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return nil, err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)

	var depositorBalance *big.Int
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {

		instance, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorBalance, err = instance.GetBalanceOfDepositor(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorBalance, err = instance.GetBalanceOfDepositor(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("StakingBalance", "Address", dep, "coins", weiToEther(depositorBalance).String(), "wei", depositorBalance)

	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return depositorBalance, nil
}

func getNetBalanceOfDepositor(dep string) (*big.Int, error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return nil, err
	}

	var depositorBalance *big.Int
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorBalance, err = instance.GetNetBalanceOfDepositor(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorBalance, err = instance.GetNetBalanceOfDepositor(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("StakingNetBalance", "Address", dep, "coins", weiToEther(depositorBalance).String(), "wei", depositorBalance)

	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return depositorBalance, nil
}

func getDepositorOfValidator(val string) (common.Address, error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return common.ZERO_ADDRESS, err
	}

	var depositor common.Address
	var validator common.Address
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return common.ZERO_ADDRESS, err
		}

		validator = common.HexToAddress(val)
		depositor, err = instance.GetDepositorOfValidator(nil, validator)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return common.ZERO_ADDRESS, err
		}

		validator = common.HexToAddress(val)
		depositor, err = instance.GetDepositorOfValidator(nil, validator)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("Depositor", depositor, "validator", validator)
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return depositor, err
}

func getDepositorBlockRewards(dep string) (*big.Int, error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return nil, err
	}

	var depositorBalance *big.Int
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorBalance, err = instance.GetDepositorRewards(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorBalance, err = instance.GetDepositorRewards(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("BlockRewards", "Depositor", dep, "coins", weiToEther(depositorBalance).String(), "wei", depositorBalance)
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return depositorBalance, nil
}

func getDepositorSlashings(dep string) (*big.Int, error) {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return nil, err
	}

	var depositorSlashing *big.Int
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorSlashing, err = instance.GetDepositorSlashings(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return nil, err
		}

		depositor := common.HexToAddress(dep)
		depositorSlashing, err = instance.GetDepositorSlashings(nil, depositor)
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("Slashing", "Depositor", dep, "coins", weiToEther(depositorSlashing).String(), "wei", depositorSlashing)
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return depositorSlashing, nil
}

type ValidatorDetails struct {
	Depositor          common.Address `json:"depositor"     gencodec:"required"`
	Validator          common.Address `json:"validator"     gencodec:"required"`
	Balance            string         `json:"balance"       gencodec:"required"`
	NetBalance         string         `json:"netBalance"    gencodec:"required"`
	BlockRewards       string         `json:"blockRewards"  gencodec:"required"`
	Slashings          string         `json:"slashings"  gencodec:"required"`
	IsValidationPaused bool           `json:"isValidationPaused"  gencodec:"required"`
	WithdrawalBlock    string         `json:"withdrawalBlock"  gencodec:"required"`
	WithdrawalAmount   string         `json:"withdrawalAmount"  gencodec:"required"`
	LastNiLBlock       string         `json:"lastNiLBlock" gencodec:"required"`
	NilBlockCount      string         `json:"nilBlockCount" gencodec:"required"`
}

func listValidators() error {
	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
	}

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	var validatorList []common.Address

	blockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		return err
	}

	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		instance, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		validatorList, err = instance.ListValidators(nil)
		if err != nil {
			return err
		}
	} else {
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		validatorList, err = instance.ListValidators(nil)
		if err != nil {
			return err
		}
	}

	totalDepositedBalance := big.NewInt(int64(0))

	var validatorDetails *ValidatorDetails
	var validatorDetailsList []*ValidatorDetails

	for i := 0; i < len(validatorList); i++ {
		depositor, err := getDepositorOfValidator(validatorList[i].String())
		if err != nil {
			return err
		}

		if depositor.IsEqualTo(common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000000000")) {
			continue
		}

		balanceVal, err := getBalanceOfDepositor(depositor.String())
		if err != nil {
			return err
		}

		netBalance, err := getNetBalanceOfDepositor(depositor.String())
		if err != nil {
			return err
		}

		blockrewards, err := getDepositorBlockRewards(depositor.String())
		if err != nil {
			return err
		}

		blockslashing, err := getDepositorSlashings(depositor.String())
		if err != nil {
			return err
		}

		validatorDetails = &ValidatorDetails{
			Depositor:    depositor,
			Validator:    validatorList[i],
			Balance:      hexutil.EncodeBig(balanceVal),
			NetBalance:   hexutil.EncodeBig(netBalance),
			BlockRewards: hexutil.EncodeBig(blockrewards),
			Slashings:    hexutil.EncodeBig(blockslashing),
		}
		validatorDetailsList = append(validatorDetailsList, validatorDetails)

		totalDepositedBalance = totalDepositedBalance.Add(totalDepositedBalance, balanceVal)

	}

	for i := 0; i < len(validatorDetailsList); i++ {
		validatorDetails := validatorDetailsList[i]

		balance, _ := hexutil.DecodeBig(validatorDetails.Balance)
		netBalance, _ := hexutil.DecodeBig(validatorDetails.NetBalance)
		blockRewards, _ := hexutil.DecodeBig(validatorDetails.BlockRewards)
		slashing, _ := hexutil.DecodeBig(validatorDetails.Slashings)

		fmt.Println("Depositor ", validatorDetails.Depositor, "Validator ", validatorDetails.Validator, "Balance coins", weiToEther(balance).String(),
			"NetBalance coins", weiToEther(netBalance).String(), "Block Rewards coins", weiToEther(blockRewards).String(), "Slashing Coins", weiToEther(slashing).String())
	}

	fmt.Println("Total validators", len(validatorDetailsList), "totalDepositedBalance", weiToEther(totalDepositedBalance).String())

	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func initiatePartialWithdrawal(key *signaturealgorithm.PrivateKey, amount string) error {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(100000)

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	contract, err := stakingv2.NewStaking(contractAddress, client)
	if err != nil {
		return err
	}

	amountFlt, err := ParseBigFloat(amount)
	if err != nil {
		return err
	}
	amountWei := etherToWeiFloat(amountFlt)

	tx, err := contract.InitiatePartialWithdrawal(txnOpts, amountWei)
	if err != nil {
		return err
	}

	fmt.Println("Your request to initiate rewards withdrawal has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func completePartialWithdrawal(key *signaturealgorithm.PrivateKey) error {

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(50000)

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	var tx *types.Transaction
	contract, err := stakingv2.NewStaking(contractAddress, client)
	if err != nil {
		return err
	}

	tx, err = contract.CompletePartialWithdrawal(txnOpts)
	if err != nil {
		return err
	}

	fmt.Println("Your request to complete rewards withdrawal has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func increaseDeposit(key *signaturealgorithm.PrivateKey, additionalAmount string) error {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(65000)

	val, _ := ParseBigFloat(additionalAmount)
	txnOpts.Value = etherToWeiFloat(val)

	contract, err := stakingv2.NewStaking(contractAddress, client)
	if err != nil {
		return err
	}

	tx, err := contract.IncreaseDeposit(txnOpts)
	if err != nil {
		return err
	}

	fmt.Println("Your request to initiate rewards withdrawal has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func changeValidator(key *signaturealgorithm.PrivateKey, newValidatorAddress common.Address) error {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(175000)

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	contract, err := stakingv2.NewStaking(contractAddress, client)
	if err != nil {
		return err
	}

	tx, err := contract.ChangeValidator(txnOpts, newValidatorAddress)
	if err != nil {
		return err
	}

	fmt.Println("Your request to change the validator has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func getStakingDetails(validatorAddress common.Address) error {
	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
	}

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)

	blockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		return err
	}

	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		fmt.Println(nil)
	} else {
		instance, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		stakingDetails, err := instance.GetStakingDetails(nil, validatorAddress)

		if stakingDetails.Depositor.IsEqualTo(common.ZERO_ADDRESS) {
			return nil
		}

		fmt.Println("Depositor ", stakingDetails.Depositor, " Validator ", stakingDetails.Validator)
		fmt.Println("Last NiL Block ", stakingDetails.LastNilBlockNumber.String(), " Nil Block Count ", stakingDetails.NilBlockCount.String())
		fmt.Println("Withdrawal Block ", stakingDetails.WithdrawalBlock.String())
		fmt.Println("Withdrawal coins ", weiToEther(stakingDetails.WithdrawalAmount).String())
		fmt.Println("Slashing coins", weiToEther(stakingDetails.Slashings).String())
		fmt.Println("Rewards coins ", weiToEther(stakingDetails.BlockRewards).String())
		fmt.Println("Staking Balance coins ", weiToEther(stakingDetails.Balance).String())
		fmt.Println("Net Balance coins ", weiToEther(stakingDetails.NetBalance).String())
	}

	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func pauseValidation(key *signaturealgorithm.PrivateKey) error {

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(100000)

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	var tx *types.Transaction
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contract, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.PauseValidation(txnOpts)
		if err != nil {
			return err
		}
	} else {
		contract, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.PauseValidation(txnOpts)
		if err != nil {
			return err
		}
	}

	fmt.Println("Your request to pause validation has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func resumeValidation(key *signaturealgorithm.PrivateKey) error {

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	contractAddress := common.HexToAddress(staking.STAKING_CONTRACT)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(100000)

	val, _ := ParseBigFloat("0")
	txnOpts.Value = etherToWeiFloat(val)

	var tx *types.Transaction
	var blockNumber uint64
	if blockNumber < proofofstake.STAKING_CONTRACT_V2_CUTOFF_BLOCK {
		contract, err := stakingv1.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.ResumeValidation(txnOpts)
		if err != nil {
			return err
		}
	} else {
		contract, err := stakingv2.NewStaking(contractAddress, client)
		if err != nil {
			return err
		}

		tx, err = contract.PauseValidation(txnOpts)
		if err != nil {
			return err
		}
	}

	fmt.Println("Your request to resume validation has been added to the queue for processing.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}

func transferTokens(contractAddr string, toAddr string, tokenTransferAmount *big.Int, key *signaturealgorithm.PrivateKey) error {
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}

	fromAddress, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)

	if err != nil {
		return err
	}

	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return err
	}

	toAddress := common.HexToAddress(toAddr)

	contractAddress := common.HexToAddress(contractAddr)
	txnOpts, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(123123))

	if err != nil {
		return err
	}

	txnOpts.From = fromAddress
	txnOpts.Nonce = big.NewInt(int64(nonce))
	txnOpts.GasLimit = uint64(2500000)
	txnOpts.Value = big.NewInt(0)

	var tx *types.Transaction
	contract, err := token.NewToken(contractAddress, client)
	if err != nil {
		return err
	}

	tx, err = contract.Transfer(txnOpts, toAddress, tokenTransferAmount)
	if err != nil {
		return err
	}

	fmt.Println("Your request to transfer tokens has been added to the queue for processing. Please check your account balance after 10 minutes.")
	fmt.Println("The transaction hash for tracking this request is: ", tx.Hash())
	fmt.Println()

	time.Sleep(1000 * time.Millisecond)

	return nil
}
