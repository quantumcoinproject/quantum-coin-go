package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/keystore"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/console/prompt"
	"github.com/quantumcoinproject/quantum-coin-go/conversionutil"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/crosssign"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/params"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const READ_API_URL = "https://scan.dpapi.org"
const WRITE_API_URL = "https://txn.dpapi.org"

func printHelp() {
	fmt.Println("===========")
	fmt.Println(" dputil ")
	fmt.Println("      Set a default environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil genesis-sign ETH_ADDRESS DEPOSITOR_QUANTUM_ADDRESS VALIDATOR_QUANTUM_ADDRESS AMOUNT")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_KEY_FILE_DIR, DP_DEPOSITOR_ACC_PWD, DP_VALIDATOR_ACC_PWD")
	fmt.Println("===========")
	fmt.Println("dputil genesis-verify JSON_FILE_NAME")
	fmt.Println("===========")
	fmt.Println("dputil getconversionmessage ETH_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_KEY_FILE")
	fmt.Println("===========")
	fmt.Println("dputil getcoinsfortokens ETH_ADDRESS ETH_SIGNATURE")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_KEY_FILE")
	fmt.Println("===========")
	fmt.Println("dputil balance ACCOUNT_ADDRESS")
	fmt.Println("===========")
	fmt.Println("dputil stakingdeposit DEPOSITOR_ADDRESS VALIDATOR_ADDRESS DEPOSITOR_AMOUNT")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil stakingbalance DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("===========")
	fmt.Println("dputil listvalidators")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("===========")
	fmt.Println("dputil blockrewards DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("===========")
	fmt.Println("dputil initiatewithdrawalrewards DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("===========")
	fmt.Println("dputil completewithdrawalrewards DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil completewithdrawal DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil initiatepartialwithdrawal DEPOSITOR_ADDRESS amount")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil completepartialwithdrawal DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil increasedeposit DEPOSITOR_ADDRESS ADDITIONAL_DEPOSIT_AMOUNT")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil changevalidator DEPOSITOR_ADDRESS NEW_VALIDATOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("===========")
	fmt.Println("dputil getstakingdetails VALIDATOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil pausevalidation DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil resumevalidation DEPOSITOR_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil transfercoins FROM_ADDRESS TO_ADDRESS AMOUNT")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("dputil transfertokens CONTRACT_ADDRESS FROM_ADDRESS TO_ADDRESS amount")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("dputil renouncetokenownership CONTRACT_ADDRESS FROM_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("dputil createtoken FROM_ADDRESS TOKEN_NAME TOKEN_SYMBOL TOTAL_SUPPLY")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("dputil tokenbalance CONTRACT_ADDRESS ACCOUNT_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil multitransfertokens CONTRACT_ADDRESS FROM_ADDRESS CSV_FILE")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("      CSV file format example (with no column header):")
	fmt.Println("           toAddress1, amount1")
	fmt.Println("           toAddress2, amount2")
	fmt.Println("dputil txn TXN_HASH")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil createtokenconversioncontract FROM_ADDRESS")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("dputil submittokenburnproof BURN_PROOF_CONTRACT_ADDRESS FROM_ADDRESS BURN_PROOF_FILE")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL, DP_KEY_FILE_DIR")
	fmt.Println("dputil listtokenconversions BURN_PROOF_CONTRACT_ADDRESS OUTPUT_FOLDER")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil listaddresstokenconversions QUANTUM_WALLET_ADDRESS BURN_PROOF_CONTRACT_ADDRESS QUANTUM_CONTRACT_ADDRESS ETHEREUM_CONTRACT_ADDRESS OUTPUT_FOLDER")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("dputil sethead BLOCK_NUMBER")
	fmt.Println("      Set the following environment variables:")
	fmt.Println("           DP_RAW_URL")
	fmt.Println("===========")
	fmt.Println("===========")
}

var rawURL string
var wg sync.WaitGroup

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}
	defaults.LoadDefaultConfig()
	signMode := os.Getenv("SIGN_MODE")
	if signMode == "4" {
		defaults.SetCryptoSigningMode(4)
	} else if signMode == "3" {
		defaults.SetCryptoSigningMode(3)
	} else if signMode == "2" {
		defaults.SetCryptoSigningMode(2)
	} else if signMode == "1" {
		defaults.SetCryptoSigningMode(1)
	} else if len(signMode) > 0 {
		fmt.Println("Unknown value for environment variable SIGN_MODE")
		return
	}

	rawURL = os.Getenv("DP_RAW_URL")
	if len(rawURL) == 0 {
		runtimeOS := strings.ToLower(runtime.GOOS)
		if runtimeOS == "windows" {
			rawURL = "\\\\.\\pipe\\geth.ipc"
		} else {
			rawURL = "data/geth.ipc"
		}
	}

	if os.Args[1] == "balance" {
		balance()
	} else if os.Args[1] == "transfercoins" {
		sendTxn()
	} else if os.Args[1] == "txn" {
		getTxn()
	} else if os.Args[1] == "genesis-sign" {
		GenesisSign()
	} else if os.Args[1] == "genesis-verify" {
		GenesisVerify()
	} else if os.Args[1] == "getconversionmessage" {
		err := GetConversionMessage()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "getcoinsfortokens" {
		err := ConvertToCoins()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "stakingdeposit" {
		err := Deposit()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "stakingbalance" {
		err := DepositorBalance()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "listvalidators" {
		err := listValidators()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "blockrewards" {
		err := DepositorBlockRewards()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "initiatewithdrawalrewards" {
		err := InitiateWithdrawalRewards()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "completewithdrawalrewards" {
		err := CompletePartialWithdrawal()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "initiatepartialwithdrawal" {
		err := InitiatePartialWithdrawal()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "completepartialwithdrawal" {
		err := CompletePartialWithdrawal()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "increasedeposit" {
		err := IncreaseDeposit()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "changevalidator" {
		err := ChangeValidator()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "getstakingdetails" {
		err := GetStakingDetails()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "pausevalidation" {
		err := PauseValidation()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "resumevalidation" {
		err := ResumeValidation()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "initiatewithdrawal" {
		err := InitiateWithdrawal()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "completewithdrawal" {
		err := CompleteWithdrawal()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "transfertokens" {
		err := TransferTokens()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "renouncetokenownership" {
		err := RenounceTokenOwnerShip()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "createtoken" {
		err := CreateToken()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "tokenbalance" {
		err := TokenBalance()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "multitransfertokens" {
		err := MultiTransferTokens()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "accountlist" {
		err := AccountList()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "createtokenconversioncontract" {
		err := CreateTokenConversionContract()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "submittokenburnproof" {
		err := SubmitTokenBurnProof()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "listtokenconversions" {
		err := ListTokenConversions()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "listaddresstokenconversions" {
		err := ListAddressTokenConversions()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "sethead" {
		err := SetHead()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else if os.Args[1] == "listcoinconversions" {
		err := ListCoinConversions()
		if err != nil {
			fmt.Println("Error", err)
		}
	} else {
		printHelp()
	}
}

func GenesisSign() {
	if len(os.Args) < 6 {
		printHelp()
		return
	}
	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		fmt.Println("Set the keyfile directory environment variable DP_KEY_FILE_DIR")
		return
	}
	if len(os.Getenv("DP_DEPOSITOR_ACC_PWD")) == 0 {
		fmt.Println("Set the depositor password environment variable DP_DEPOSITOR_ACC_PWD")
		return
	}
	if len(os.Getenv("DP_VALIDATOR_ACC_PWD")) == 0 {
		fmt.Println("Set the validator password environment variable DP_VALIDATOR_ACC_PWD")
		return
	}

	ethAddr := os.Args[2]
	depositorAddr := os.Args[3]
	validatorAddr := os.Args[4]
	amount := os.Args[5]

	if common.IsLegacyEthereumHexAddress(ethAddr) == false {
		fmt.Println("Invalid eth address", ethAddr)
		return
	}

	if common.IsHexAddress(depositorAddr) == false {
		fmt.Println("Invalid depositor address", depositorAddr)
		return
	}

	if common.IsHexAddress(validatorAddr) == false {
		fmt.Println("Invalid validator address", validatorAddr)
		return
	}

	_, err := ParseBigFloat(amount)
	if err != nil {
		fmt.Println(err)
		return
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		fmt.Println("Error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR", err)
		return
	}
	depositorKey, err := ReadDataFile(depositorKeyFile)
	if err != nil {
		fmt.Println("Error loading depositor key file", err)
		return
	}
	depPassword := os.Getenv("DP_DEPOSITOR_ACC_PWD")
	depKey, err := keystore.DecryptKey(depositorKey, depPassword)
	if err != nil {
		fmt.Println("Error decrypting depositor key using DP_DEPOSITOR_ACC_PWD", err)
		return
	}

	validatorKeyFile, err := findKeyFile(validatorAddr)
	if err != nil {
		fmt.Println("Error finding VALIDATOR_ADDRESS in DP_KEY_FILE_DIR", err)
		return
	}
	validatorKey, err := ReadDataFile(validatorKeyFile)
	if err != nil {
		fmt.Println("Error loading validator key file", err)
		return
	}
	valPassword := os.Getenv("DP_VALIDATOR_ACC_PWD")
	valKey, err := keystore.DecryptKey(validatorKey, valPassword)
	if err != nil {
		fmt.Println("Error decrypting depositor key using DP_VALIDATOR_ACC_PWD", err)
		return
	}

	details, err := crosssign.SignGenesis(depKey.PrivateKey, valKey.PrivateKey, ethAddr, amount)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Signed the genesis validator message!")

	marshalled, err := json.Marshal(details)
	if err != nil {
		fmt.Println(err)
		return
	}

	fileName := "cross-sign-" + depositorAddr + ".json"
	err = ioutil.WriteFile(fileName, marshalled, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Successfully created cross-sign file", fileName)

	return
}

func GenesisVerify() {
	if len(os.Args) < 3 {
		printHelp()
		return
	}

	jsonFile := os.Args[2]

	jsonString, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		fmt.Println("error opening json file", jsonFile, err)
		return
	}

	jsonBytes := []byte(jsonString)

	details := crosssign.GenesisCrossSignDetails{}
	err = json.Unmarshal(jsonBytes, &details)
	if err != nil {
		fmt.Println("error reading json", jsonFile, err)
		return
	}

	_, err = crosssign.VerifyGenesis(&details)
	if err != nil {
		fmt.Println("verify failed", err)
		return
	}

	fmt.Println("Verify succeeded!")
}

func balance() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	addr := os.Args[2]

	if common.IsHexAddress(addr) == false {
		fmt.Println("Invalid address", addr)
		return
	}

	if strings.HasPrefix(addr, "0x") == false {
		addr = "0x" + addr
	}

	if len(rawURL) == 0 {
		ethBalance, weiBalance, nonce, err := requestGetBalance(addr)
		if err != nil {
			fmt.Println("Error", err)
		}
		fmt.Println("Address", addr, "coins", ethBalance, "wei", weiBalance, "nonce", nonce)
	} else {
		ethBalance, weiBalance, err := getBalance(addr)
		if err != nil {
			fmt.Println("Error", err)
		}
		fmt.Println("Address", addr, "coins", ethBalance, "wei", weiBalance)
	}
}

type Txn struct {
	FromAddress string
	ToAddress   string
	Quantity    string
	Count       int
}

func sendTxn() {
	if len(os.Args) < 5 {
		printHelp()
		return
	}

	from := os.Args[2]
	to := os.Args[3]
	quantity := os.Args[4]
	shouldConfirm := os.Getenv("SHOULD_CONFIRM")

	if common.IsHexAddress(from) == false {
		fmt.Println("Invalid address", from)
		return
	}

	if common.IsHexAddress(to) == false {
		fmt.Println("Invalid address", to)
		return
	}

	flt, err := ParseBigFloat(quantity)
	if err != nil {
		fmt.Println(err)
		return
	}

	wei := etherToWeiFloat(flt)
	coins := weiToEther(wei)

	fmt.Println("Send", "from", from, "to", to, "quantity", quantity, "coins", coins)

	if shouldConfirm != "no" {
		ethConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you want to send %v coins from address %s to address %s?", coins, from, to))
		if err != nil {
			log.Error("error", err)
			return
		}
		if ethConfirm != true {
			log.Error("confirmation not made")
			return
		}
	}
	fmt.Println()

	txHash, err := send(from, to, quantity)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("TxnHash", txHash)
}

func getTxn() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	hash := os.Args[2]

	txnJson, receipt, err := GetTransaction(hash)
	if err != nil {
		fmt.Println("GetTransaction Error", err)
		return
	}
	jsonVal, err := Prettify(txnJson)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(jsonVal)
	if receipt != nil {
		receiptJson, err := json.Marshal(receipt)
		if err != nil {
			fmt.Printf("%+v\n", receipt)
			return
		}
		fmt.Println("======================Receipt======================")
		prettyReceipt, err := Prettify(string(receiptJson))
		if err != nil {
			fmt.Printf("%+v\n", receipt)
			return
		}
		fmt.Printf(prettyReceipt)
	} else {
		fmt.Println("receipt is nil")
	}
}

func Prettify(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

func GetConversionMessage() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	ethAddress := os.Args[2]
	if common.IsLegacyEthereumHexAddress(ethAddress) == false {
		return errors.New("invalid EthAddress")
	}

	keyFile := os.Getenv("DP_KEY_FILE")
	if len(keyFile) == 0 {
		return errors.New("DP_KEY_FILE environment variable is not set")
	}

	fmt.Println(fmt.Sprintf("Quantum wallet address %s", keyFile))
	accPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the quantum wallet password : "))
	if err != nil {
		return err
	}
	if len(accPwd) == 0 {
		return errors.New("password is not set")
	}

	key, err := GetKeyFromFile(keyFile, accPwd)
	if err != nil {
		return err
	}

	qAddr, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}

	quantumAddress := qAddr.Hex()

	message := strings.Replace(crosssign.ConversionMessageTemplate, "[ETH_ADDRESS]", strings.ToLower(ethAddress), 1)
	message = strings.Replace(message, "[QUANTUM_ADDRESS]", strings.ToLower(quantumAddress), 1)

	fmt.Println("Message is: ")
	fmt.Println(message)

	return nil
}

func ConvertToCoins() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	ethAddress := os.Args[2]
	if common.IsLegacyEthereumHexAddress(ethAddress) == false {
		return errors.New("invalid EthAddress")
	}

	_, ok := conversionutil.SnapshotMap[strings.ToLower(ethAddress)]

	if ok == false {
		log.Trace("IsGasExemptTxn address not in snapshot", "ethAddress", ethAddress)
		return errors.New("unidentified eth address")
	}

	ethConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you confirm that your ETH ADDRESS having the Dogep tokens is %s ?", ethAddress))
	if err != nil {
		return err
	}
	if ethConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	ethSignature := os.Args[3]

	keyFile := os.Getenv("DP_KEY_FILE")
	if len(keyFile) == 0 {
		return errors.New("DP_KEY_FILE environment variable is not set")
	}

	fmt.Println(fmt.Sprintf("Quantum wallet addres %s", keyFile))
	accPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the quantum wallet password : "))
	if err != nil {
		return err
	}
	if len(accPwd) == 0 {
		return errors.New("password is not set")
	}
	fmt.Println()

	backupConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you confirm that you have backed up your quantum wallet located at %s ?", keyFile))
	if err != nil {
		return err
	}
	if backupConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	passwordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the wallet password will always be required to use the quantum wallet at %s?", keyFile))
	if err != nil {
		return err
	}
	if passwordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	key, err := GetKeyFromFile(keyFile, accPwd)
	if err != nil {
		return err
	}

	qAddr, err := cryptobase.SigAlg.PublicKeyToAddress(&key.PublicKey)
	if err != nil {
		return err
	}

	quantumAddress := qAddr.Hex()

	time.Sleep(500 * time.Millisecond)

	fmt.Println()
	quantumConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you confirm that you want the coins deposited to QUANTUM ADDRESS %s ?", quantumAddress))
	if err != nil {
		return err
	}
	if quantumConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	crossSignDetails := &crosssign.ConversionSignDetails{
		EthAddress:        strings.ToLower(ethAddress),
		EthereumSignature: ethSignature,
		QuantumAddress:    strings.ToLower(quantumAddress),
	}

	_, err = crosssign.VerifyConversion(crossSignDetails)
	if err != nil {
		fmt.Println("An error occurred while verifying the ethereum signature.")
		return err
	}

	time.Sleep(3000 * time.Millisecond)
	fmt.Println("Final confirmation!!!")
	time.Sleep(3000 * time.Millisecond)
	fmt.Println("Verify your message...")
	time.Sleep(3000 * time.Millisecond)

	message := strings.Replace(crosssign.ConversionMessageTemplate, "[ETH_ADDRESS]", strings.ToLower(ethAddress), 1)
	message = strings.Replace(message, "[QUANTUM_ADDRESS]", strings.ToLower(quantumAddress), 1)

	finalConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("%s", message))
	if err != nil {
		return err
	}
	if finalConfirm != true {
		return errors.New("confirmation not made")
	}

	if len(rawURL) == 0 {
		return requestConvertCoins(ethAddress, ethSignature, key)
	} else {
		return convertCoins(ethAddress, ethSignature, key)
	}
}

func Deposit() error {
	if len(os.Args) < 5 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]
	validatorAddr := os.Args[3]
	depositorAmount := os.Args[4]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	if common.IsHexAddress(validatorAddr) == false {
		return errors.New("invalid validator address " + validatorAddr)
	}

	_, err := ParseBigFloat(depositorAmount)
	if err != nil {
		return err
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	validatorKeyFile, err := findKeyFile(validatorAddr)
	if err != nil {
		return errors.New("error finding VALIDATOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Validator wallet addres %s", validatorKeyFile))
	validatorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the validator wallet password : "))
	if err != nil {
		return err
	}
	if len(validatorPwd) == 0 {
		return errors.New("validator password is not set")
	}
	fmt.Println()

	validatorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the validator password will always be required to use the quantum validator wallet at %s?", validatorKeyFile))
	if err != nil {
		return err
	}
	if validatorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	valKey, err := GetKeyFromFile(validatorKeyFile, validatorPwd)
	if err != nil {
		return errors.New("error decrypting validator key " + err.Error())
	}

	valAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&valKey.PublicKey)
	if err != nil {
		return errors.New("validator PublicKeyToAddress " + err.Error())
	}

	if !valAddressFromKey.IsEqualTo(common.HexToAddress(validatorAddr)) {
		return errors.New("validator key address check failed")
	}

	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
		//return requestNewDeposit(validatorAddr, depositorAmount, depKey)
	} else {
		return newDeposit(validatorAddr, depositorAmount, depKey)
	}
}

func InitiateWithdrawal() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return initiateWithdrawal(depKey)
}

func CompleteWithdrawal() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return completeWithdrawal(depKey)
}

func DepositorBalance() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
	} else {
		_, err := getBalanceOfDepositor(depositorAddr)
		return err
	}
}

func DepositorBlockRewards() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
	} else {
		_, err := getDepositorBlockRewards(depositorAddr)
		return err
	}
}

func InitiateWithdrawalRewards() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("nvalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	depositorReward, err := getDepositorBlockRewards(depositorAddr)
	if err != nil {
		return errors.New("depositor reward " + err.Error())
	}

	if depositorReward.Cmp(big.NewInt(0)) == 0 {
		return errors.New("there are no rewards available to withdraw")
	}

	depositorSlashings, err := getDepositorSlashings(depositorAddr)
	if err != nil {
		return errors.New("depositor slashings " + err.Error())
	}

	if depositorSlashings.Cmp(depositorReward) >= 0 {
		return errors.New("there are no rewards available to withdraw")
	}

	amount := big.NewInt(0)
	amount = amount.Sub(weiToEther(depositorReward), weiToEther(depositorSlashings))

	depositorRewardConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("The following amount will be withdrawn. Please confirm if you are ok : %d?", amount))
	if err != nil {
		return err
	}

	if depositorRewardConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	if amount.Int64() > 0 {
		return initiatePartialWithdrawal(depKey, amount.String())
	} else {
		return errors.New("invalid depositor amount")
	}

}

func InitiatePartialWithdrawal() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]
	amount := os.Args[3]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return initiatePartialWithdrawal(depKey, amount)
}

func CompletePartialWithdrawal() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return completePartialWithdrawal(depKey)
}

func IncreaseDeposit() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]
	depositAmount := os.Args[3]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return increaseDeposit(depKey, depositAmount)
}

func ChangeValidator() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]
	newValidatorAddr := os.Args[3]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	if common.IsHexAddress(newValidatorAddr) == false {
		return errors.New("invalid validator address " + newValidatorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	validatorKeyFile, err := findKeyFile(newValidatorAddr)
	if err != nil {
		return errors.New("error finding VALIDATOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Validator wallet addres %s", validatorKeyFile))
	validatorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the validator wallet password : "))
	if err != nil {
		return err
	}
	if len(validatorPwd) == 0 {
		return errors.New("validator password is not set")
	}
	fmt.Println()

	validatorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the validator password will always be required to use the quantum validator wallet at %s?", validatorKeyFile))
	if err != nil {
		return err
	}
	if validatorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	valKey, err := GetKeyFromFile(validatorKeyFile, validatorPwd)
	if err != nil {
		return errors.New("error decrypting validator key " + err.Error())
	}

	valAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&valKey.PublicKey)
	if err != nil {
		return errors.New("validator PublicKeyToAddress " + err.Error())
	}

	if !valAddressFromKey.IsEqualTo(common.HexToAddress(newValidatorAddr)) {
		return errors.New("validator key address check failed")
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return changeValidator(depKey, common.HexToAddress(newValidatorAddr))
}

func GetStakingDetails() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	validatorAddr := os.Args[2]

	if common.IsHexAddress(validatorAddr) == false {
		return errors.New("invalid validator address " + validatorAddr)
	}
	return getStakingDetails(common.HexToAddress(validatorAddr))
}

func PauseValidation() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return pauseValidation(depKey)
}

func ResumeValidation() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	depositorAddr := os.Args[2]

	if common.IsHexAddress(depositorAddr) == false {
		return errors.New("invalid depositor address " + depositorAddr)
	}

	depositorKeyFile, err := findKeyFile(depositorAddr)
	if err != nil {
		return errors.New("error finding DEPOSITOR_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("Depositor wallet address %s", depositorKeyFile))
	depositorPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the depositor wallet password : "))
	if err != nil {
		return err
	}
	if len(depositorPwd) == 0 {
		return errors.New("depositor password is not set")
	}

	depKey, err := GetKeyFromFile(depositorKeyFile, depositorPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	depositorPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you understand that the depositor password will always be required to use the quantum depositor wallet at %s?", depositorKeyFile))
	if err != nil {
		return err
	}
	if depositorPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	depAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&depKey.PublicKey)
	if err != nil {
		return errors.New("depositor public key to address " + err.Error())
	}

	if !depAddressFromKey.IsEqualTo(common.HexToAddress(depositorAddr)) {
		return errors.New("depositor key address check failed")
	}

	return resumeValidation(depKey)
}

func TransferTokens() error {
	if len(os.Args) < 6 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	contractAddr := os.Args[2]
	fromAddr := os.Args[3]
	toAddr := os.Args[4]
	transferAmt := os.Args[5]

	if common.IsHexAddress(contractAddr) == false {
		return errors.New("invalid contract address " + contractAddr)
	}

	if common.IsHexAddress(fromAddr) == false {
		return errors.New("invalid from address " + fromAddr)
	}

	if common.IsHexAddress(toAddr) == false {
		return errors.New("invalid to address " + toAddr)
	}

	val, err := ParseBigFloat(transferAmt)
	if err != nil {
		return err
	}
	tokenTransferAmount := etherToWeiFloat(val)

	fromAccountKeyFile, err := findKeyFile(fromAddr)
	if err != nil {
		return errors.New("error finding FROM_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("From account wallet address %s", fromAccountKeyFile))
	fromAccountPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the wallet password : "))
	if err != nil {
		return err
	}
	if len(fromAccountPwd) == 0 {
		return errors.New("from account password is not set")
	}

	fromKey, err := GetKeyFromFile(fromAccountKeyFile, fromAccountPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	fromAccountPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you want to transfer %s tokens for contract %s from %s to %s?",
		transferAmt, contractAddr, fromAddr, toAddr))
	if err != nil {
		return err
	}
	if fromAccountPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	fromAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&fromKey.PublicKey)
	if err != nil {
		return errors.New("from account public key to address " + err.Error())
	}

	if !fromAddressFromKey.IsEqualTo(common.HexToAddress(fromAddr)) {
		return errors.New("from account key address check failed " + fromAddressFromKey.Hex() + " " + fromAddr)
	}

	return transferTokens(contractAddr, toAddr, tokenTransferAmount, fromKey)
}

func CreateToken() error {
	if len(os.Args) < 6 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	fromAddr := os.Args[2]
	tokenName := os.Args[3]
	tokenSymbol := os.Args[4]
	tokenTotalSupply := os.Args[5]

	if len(tokenName) == 0 || len(tokenName) > 100 {
		return errors.New("invalid tokenName " + tokenName)
	}

	if len(tokenSymbol) == 0 || len(tokenSymbol) > 10 {
		return errors.New("invalid tokenSymbol " + tokenSymbol)
	}

	fltTotalSupply, err := ParseBigFloat(tokenTotalSupply)
	if err != nil {
		return err
	}
	tokenTotalSupplyWei := etherToWeiFloat(fltTotalSupply)
	baseBurnPercentDivisor := big.NewInt(100000)

	// Parse the string as an unsigned integer with base 10 and 8-bit size
	totalDecimals := uint8(18)

	fromAccountKeyFile, err := findKeyFile(fromAddr)
	if err != nil {
		return errors.New("error finding FROM_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("From account wallet address %s", fromAccountKeyFile))
	fromAccountPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the wallet password : "))
	if err != nil {
		return err
	}
	if len(fromAccountPwd) == 0 {
		return errors.New("from account password is not set")
	}

	fromKey, err := GetKeyFromFile(fromAccountKeyFile, fromAccountPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	fromAccountPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you want to create a token for with symbol `%s` ,  name `%s`? This transaction requires gas fees.", tokenName, tokenSymbol))
	if err != nil {
		return err
	}
	if fromAccountPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	fromAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&fromKey.PublicKey)
	if err != nil {
		return errors.New("from account public key to address " + err.Error())
	}

	if !fromAddressFromKey.IsEqualTo(common.HexToAddress(fromAddr)) {
		return errors.New("from account key address check failed " + fromAddressFromKey.Hex() + " " + fromAddr)
	}

	return createToken(tokenName, tokenSymbol, tokenTotalSupplyWei, baseBurnPercentDivisor, totalDecimals, fromKey)
}

func TokenBalance() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	contractAddr := os.Args[2]
	if common.IsHexAddress(contractAddr) == false {
		return errors.New("invalid contract address " + contractAddr)
	}

	accountAddr := os.Args[3]
	if common.IsHexAddress(accountAddr) == false {
		return errors.New("invalid account address " + accountAddr)
	}

	_, err := getTokenBalance(common.HexToAddress(accountAddr), common.HexToAddress(contractAddr))
	return err
}

func MultiTransferTokens() error {
	if len(os.Args) < 5 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	contractAddr := os.Args[2]
	fromAddr := os.Args[3]
	csvFile := os.Args[4]

	if common.IsHexAddress(contractAddr) == false {
		return errors.New("invalid contract address " + contractAddr)
	}

	if common.IsHexAddress(fromAddr) == false {
		return errors.New("invalid from address " + fromAddr)
	}

	fromAccountKeyFile, err := findKeyFile(fromAddr)
	if err != nil {
		return errors.New("error finding FROM_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("From account wallet address %s", fromAccountKeyFile))
	fromAccountPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the wallet password : "))
	if err != nil {
		return err
	}
	if len(fromAccountPwd) == 0 {
		return errors.New("from account password is not set")
	}

	fromKey, err := GetKeyFromFile(fromAccountKeyFile, fromAccountPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	fromAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&fromKey.PublicKey)
	if err != nil {
		return errors.New("from account public key to address " + err.Error())
	}

	if !fromAddressFromKey.IsEqualTo(common.HexToAddress(fromAddr)) {
		return errors.New("from account key address check failed " + fromAddressFromKey.Hex() + " " + fromAddr)
	}

	return multiTransferTokens(contractAddr, csvFile, fromKey)
}

type Transaction struct {
	FromAddress string `json:"fromAddress,omitempty"`
	ToAddress   string `json:"toAddress,omitempty"`
}

type TransactionList struct {
	PageCount int           `json:"pageCount,omitempty"`
	Result    []Transaction `json:"result,omitempty"`
}

func getTxnPage(pageNumber int, dpApiUrl string) (*TransactionList, error) {
	url := dpApiUrl + "/api/dogep/transactions/page/" + strconv.Itoa(pageNumber)
	fmt.Println("getting url " + url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("http.Get error", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	txnList := TransactionList{}

	err = json.Unmarshal(body, &txnList)
	if err != nil {
		fmt.Println("Unmarshal error", err)
		return nil, err
	}

	return &txnList, nil
}

func AccountList() error {
	dpApiUrl := os.Getenv("DP_API_URL")
	if len(dpApiUrl) == 0 {
		fmt.Println("set DP_API_URL")
		return errors.New("DP_API_URL not set")
	}

	accountList := make(map[string]bool)
	pageNumber := 1
	for {
		txnList, err := getTxnPage(pageNumber, dpApiUrl)
		if err != nil {
			return err
		}
		if txnList.Result != nil && len(txnList.Result) > 0 {
			for _, txn := range txnList.Result {
				accountList[strings.ToLower(txn.ToAddress)] = true
				accountList[strings.ToLower(txn.FromAddress)] = true
			}
		} else {
			break
		}
		if pageNumber < txnList.PageCount {
			pageNumber = pageNumber + 1
		} else {
			break
		}
	}
	for addr, _ := range accountList {
		fmt.Println(addr)
	}
	return nil
}

func CreateTokenConversionContract() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	fromAddr := os.Args[2]

	fromAccountKeyFile, err := findKeyFile(fromAddr)
	if err != nil {
		return errors.New("error finding FROM_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("From account wallet address %s", fromAccountKeyFile))
	fromAccountPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the wallet password : "))
	if err != nil {
		return err
	}
	if len(fromAccountPwd) == 0 {
		return errors.New("from account password is not set")
	}

	fromKey, err := GetKeyFromFile(fromAccountKeyFile, fromAccountPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	fromAccountPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you want to create a token conversion contract? This transaction requires gas fees."))
	if err != nil {
		return err
	}
	if fromAccountPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	fromAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&fromKey.PublicKey)
	if err != nil {
		return errors.New("from account public key to address " + err.Error())
	}

	if !fromAddressFromKey.IsEqualTo(common.HexToAddress(fromAddr)) {
		return errors.New("from account key address check failed " + fromAddressFromKey.Hex() + " " + fromAddr)
	}

	return createtokenconversioncontract(fromKey)
}

func SubmitTokenBurnProof() error {
	if len(os.Args) < 5 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	contractAddr := os.Args[2]
	fromAddr := os.Args[3]
	burnProofFile := os.Args[4]

	if common.IsHexAddress(contractAddr) == false {
		return errors.New("invalid contract address " + contractAddr)
	}

	if common.IsHexAddress(fromAddr) == false {
		return errors.New("invalid from address " + fromAddr)
	}

	fromAccountKeyFile, err := findKeyFile(fromAddr)
	if err != nil {
		return errors.New("error finding FROM_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("From account wallet address %s", fromAccountKeyFile))
	fromAccountPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the wallet password : "))
	if err != nil {
		return err
	}
	if len(fromAccountPwd) == 0 {
		return errors.New("from account password is not set")
	}

	fromKey, err := GetKeyFromFile(fromAccountKeyFile, fromAccountPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	fromAccountPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you want to burn proof tokens to contract %s from %s? A lot of coins will be required for gas fee.",
		contractAddr, fromAddr))
	if err != nil {
		return err
	}
	if fromAccountPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	fromAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&fromKey.PublicKey)
	if err != nil {
		return errors.New("from account public key to address " + err.Error())
	}

	if !fromAddressFromKey.IsEqualTo(common.HexToAddress(fromAddr)) {
		return errors.New("from account key address check failed " + fromAddressFromKey.Hex() + " " + fromAddr)
	}

	file, err := os.Open(burnProofFile)
	if err != nil {
		fmt.Println("error opening file", err, burnProofFile)
	}
	defer func() {
		if err = file.Close(); err != nil {
			fmt.Println("error closing file", err, burnProofFile)
		}
	}()

	fileBytes, err := io.ReadAll(file)
	if len(fileBytes) > 1024*2 {
		fmt.Println("too long burn proof")
		return nil
	}

	return submitBurnProof(contractAddr, string(fileBytes), fromKey)
}

func ListCoinConversions() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage. enter the output folder")
	}

	outputFolder := os.Args[2]
	_, err := os.Stat(outputFolder)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("folder doesn't exist")
		} else {
			fmt.Println(err)
		}
		return err
	}

	summary, err := listConversionDetails()
	if err != nil {
		fmt.Println(err)
		return err
	}

	outputR := "QuantumAddress,EthAddress,IsConverted,Coins, Wei\n"
	for _, r := range summary.ConversionList {
		outputR = outputR + fmt.Sprintf("%s,%s,%v,%v,%v\n", r.QuantumAddress, r.EthAddress, r.IsConverted, params.WeiToEther(r.Coins), r.Coins)
	}
	err = os.WriteFile(path.Join(outputFolder, "coin-conversions.csv"), []byte(outputR), 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}

	totalConvertedCoins, err := hexutil.DecodeBig(summary.ConvertedCoins)
	if err != nil {
		fmt.Println(err)
		return err
	}
	totalCoins, err := hexutil.DecodeBig(summary.TotalCoins)
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("Coin Conversion Summary", "Total", summary.Total, "TotalConverted", summary.TotalConverted,
		"TotalNotConverted", summary.TotalNotConverted, "ConvertedCoins", params.WeiToEther(totalConvertedCoins), "TotalCoins", params.WeiToEther(totalCoins))
	fmt.Println("finished writing", "output folder", outputFolder)
	fmt.Println("!!!!!!!!!!!!!!!!!!!Warning: none of the burn proofs or conversion are verified by this tool!!!!!!!!!!!!!!!!!!!")

	return nil
}

func ListTokenConversions() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	burnProofContractAddr := os.Args[2]
	if common.IsHexAddress(burnProofContractAddr) == false {
		return errors.New("invalid burn proof contract address " + burnProofContractAddr)
	}
	burnProofContractAddress := common.HexToAddress(burnProofContractAddr)

	outputFolder := os.Args[3]
	_, err := os.Stat(outputFolder)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("folder doesn't exist")
		} else {
			fmt.Println(err)
		}
		return err
	}

	requests, err := listTokenConversionRequests(burnProofContractAddress)
	if err != nil {
		fmt.Println(err)
		return err
	}

	outputR := "QuantumAddress,EthAddress,EthSignature\n"
	for _, r := range *requests {
		outputR = outputR + fmt.Sprintf("%s,%s,%s\n", r.QuantumAddress, r.EthAddress, r.EthSignature)
	}
	err = os.WriteFile(path.Join(outputFolder, "token-conversion-requests.csv"), []byte(outputR), 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}

	burnProofs, err := listTokenBurnProofs(burnProofContractAddress)
	if err != nil {
		fmt.Println(err)
		return err
	}

	for i, b := range *burnProofs {
		err = os.WriteFile(path.Join(outputFolder, "burnproof-"+strconv.Itoa(i)+".csv"), []byte(b), 0644)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	fmt.Println("finished writing", "output folder", outputFolder, "request count", len(*requests), "burnproofs count", len(*burnProofs))
	fmt.Println("!!!!!!!!!!!!!!!!!!!Warning: none of the burn proofs or conversion are verified by this tool!!!!!!!!!!!!!!!!!!!")

	return err
}

func ListAddressTokenConversions() error {
	if len(os.Args) < 7 {
		printHelp()
		return errors.New("incorrect usage")
	}

	quantumWalletAddress := os.Args[2]
	if common.IsHexAddress(quantumWalletAddress) == false {
		return errors.New("invalid quantum wallet address " + quantumWalletAddress)
	}
	quantumWalletAddr := common.HexToAddress(quantumWalletAddress)

	burnProofContractAddress := os.Args[3]
	if common.IsHexAddress(burnProofContractAddress) == false {
		return errors.New("invalid burn proof contract address " + burnProofContractAddress)
	}
	burnProofContractAddr := common.HexToAddress(burnProofContractAddress)

	quantumContractAddress := os.Args[4]
	if common.IsHexAddress(quantumContractAddress) == false {
		return errors.New("invalid token quantum contract address " + quantumContractAddress)
	}
	quantumContractAddr := common.HexToAddress(quantumContractAddress)

	eContractAddr := os.Args[5]

	outputFolder := os.Args[6]

	fmt.Println("quantum wallet address", quantumWalletAddr)
	fmt.Println("burn proof contract address", burnProofContractAddress)
	fmt.Println("quantum token contract address", quantumContractAddress)
	fmt.Println("ethereum token contract address", eContractAddr)
	fmt.Println("output folder", outputFolder)

	_, err := os.Stat(outputFolder)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("folder doesn't exist")
		} else {
			fmt.Println(err)
		}
		return err
	}

	requests, err := listTokenConversionRequests(burnProofContractAddr)
	if err != nil {
		fmt.Println(err)
		return err
	}

	outputR := "QuantumAddress,EthAddress,EthSignature,CrossSignVerified\n"
	for _, r := range *requests {
		if r.QuantumAddress.HexLower() == quantumWalletAddr.HexLower() {
			crossSignDetails := &crosssign.TokenConversionSignDetails{
				EthAddress:              strings.ToLower(r.EthAddress),
				EthereumSignature:       r.EthSignature,
				QuantumAddress:          r.QuantumAddress.HexLower(),
				QuantumContractAddress:  quantumContractAddr.HexLower(),
				EthereumContractAddress: eContractAddr,
			}
			_, err = crosssign.VerifyConversionToken(crossSignDetails)
			if err != nil {
				fmt.Println("Failed verify", "ethereum address",
					"quantum wallet address", r.QuantumAddress, "quantum token contract address", quantumContractAddr,
					"ethereum contract address", eContractAddr, r.EthAddress, "ethereum signature")
			}
			verified := err == nil

			outputR = outputR + fmt.Sprintf("%s,%s,%s,%v\n", r.QuantumAddress, r.EthAddress, r.EthSignature, verified)
		}
	}
	err = os.WriteFile(path.Join(outputFolder, "address-token-conversion-requests.csv"), []byte(outputR), 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}

	burnProofs, err := listTokenBurnProofs(burnProofContractAddr)
	if err != nil {
		fmt.Println(err)
		return err
	}

	for i, b := range *burnProofs {
		err = os.WriteFile(path.Join(outputFolder, "burnproof-"+strconv.Itoa(i)+".csv"), []byte(b), 0644)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	fmt.Println("finished writing", "output folder", outputFolder, "conversion request count", len(*requests), "burnproofs count", len(*burnProofs))
	fmt.Println("!!!!!!!!!!!!!!!!!!!Warning: none of the burn proofs or conversions are verified by this tool!!!!!!!!!!!!!!!!!!!")

	return err
}

func RenounceTokenOwnerShip() error {
	if len(os.Args) < 4 {
		printHelp()
		return errors.New("incorrect usage")
	}

	if len(os.Getenv("DP_KEY_FILE_DIR")) == 0 {
		return errors.New("set the keyfile directory environment variable DP_KEY_FILE_DIR")
	}

	contractAddr := os.Args[2]
	fromAddr := os.Args[3]

	if common.IsHexAddress(contractAddr) == false {
		return errors.New("invalid contract address " + contractAddr)
	}

	if common.IsHexAddress(fromAddr) == false {
		return errors.New("invalid from address " + fromAddr)
	}

	fromAccountKeyFile, err := findKeyFile(fromAddr)
	if err != nil {
		return errors.New("error finding FROM_ADDRESS in DP_KEY_FILE_DIR " + err.Error())
	}

	fmt.Println(fmt.Sprintf("From account wallet address %s", fromAccountKeyFile))
	fromAccountPwd, err := prompt.Stdin.PromptPassword(fmt.Sprintf("Enter the wallet password : "))
	if err != nil {
		return err
	}
	if len(fromAccountPwd) == 0 {
		return errors.New("from account password is not set")
	}

	fromKey, err := GetKeyFromFile(fromAccountKeyFile, fromAccountPwd)
	if err != nil {
		return errors.New("error decrypting depositor key " + err.Error())
	}

	fmt.Println()

	fromAccountPasswordConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you want to renounce ownership of contract %s from %s?",
		contractAddr, fromAddr))
	if err != nil {
		return err
	}
	if fromAccountPasswordConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	fromAddressFromKey, err := cryptobase.SigAlg.PublicKeyToAddress(&fromKey.PublicKey)
	if err != nil {
		return errors.New("from account public key to address " + err.Error())
	}

	if !fromAddressFromKey.IsEqualTo(common.HexToAddress(fromAddr)) {
		return errors.New("from account key address check failed " + fromAddressFromKey.Hex() + " " + fromAddr)
	}

	return renounceTokenOwnership(contractAddr, fromKey)
}

func SetHead() error {
	if len(os.Args) < 3 {
		printHelp()
		return errors.New("incorrect usage")
	}

	blockNum := os.Args[2]

	blockNumUint64, err := strconv.ParseUint(blockNum, 10, 64)
	if err != nil {
		fmt.Println("Enter block number correctly: ", blockNum)
		return err
	}

	ethConfirm, err := prompt.Stdin.PromptConfirm(fmt.Sprintf("Do you confirm setting the block head to number %d ?", blockNumUint64))
	if err != nil {
		return err
	}
	if ethConfirm != true {
		return errors.New("confirmation not made")
	}
	fmt.Println()

	if len(rawURL) == 0 {
		return errors.New("DP_RAW_URL environment variable not specified")
	}

	client, err := ethclient.Dial(rawURL)
	if err != nil {
		fmt.Println("Dial failed, ensure DP_RAW_URL is set correctly", err)
		return err
	}

	err = client.SetHead(context.Background(), hexutil.EncodeUint64(blockNumUint64))
	if err != nil {
		fmt.Println("SetHead failed", err, blockNumUint64)
		return err
	}

	fmt.Println("SetHead succeeded")

	return nil
}
