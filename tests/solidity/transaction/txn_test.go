package transaction

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/consensus"
	"github.com/QuantumCoinProject/qc/consensus/mockconsensus"
	"github.com/QuantumCoinProject/qc/core"
	"github.com/QuantumCoinProject/qc/core/rawdb"
	"github.com/QuantumCoinProject/qc/core/state"
	"github.com/QuantumCoinProject/qc/core/types"
	"github.com/QuantumCoinProject/qc/core/vm"
	"github.com/QuantumCoinProject/qc/internal/ethapi"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/params"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

/*
1) solc.exe  --bin --bin-runtime --abi c:\github\quantum-coin-go\tests\solidity\transaction\txn.sol -o c:\github\quantum-coin-go\tests\solidity\transaction
2) Copy contents of TxnContract.bin-runtime into TxnContractRuntimeBin constant below
3) abigen --bin=c:\github\quantum-coin-go\tests\solidity\transaction\TxnContract.bin -abi=c:\github\quantum-coin-go\tests\solidity\transaction\TxnContract.abi --pkg=transaction --out=c:\github\quantum-coin-go\tests\solidity\transaction\txn.go
*/

const TxnContractRuntimeBin = "608060405260043610601c5760003560e01c8063d0e30db0146021575b600080fd5b6027603d565b6040518082815260200191505060405180910390f35b600080349050605681600054606590919063ffffffff16565b60008190555060005491505090565b600080828401905083811015607657fe5b809150509291505056fea2646970667358221220f2a9c298fd8e42f28b414deb38136c23f2479330a95a99542b014e0456fcd44464736f6c637827302e372e362d646576656c6f702e323032352e322e31332b636f6d6d69742e34663635373333610058"
const GENESIS_BLOCK_HASH = "0x2c8127f13d50434052128a88c9c9f79a27d44a1145e51f6fd250b6e247369e99"

var txnContractAddress = common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000003000")
var messageSenderAddress = common.HexToAddress("0x0000000000000000000000000000000000000000000000000000000000005000")

func newStateDb() *state.StateDB {
	dname, _ := os.MkdirTemp("", "txntest")
	fmt.Println("txntest db path", dname)
	db, _ := rawdb.NewLevelDBDatabase(dname, 0, 0, "", false)
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(db), nil)

	statedb.CreateAccount(txnContractAddress)
	statedb.SetCode(txnContractAddress, common.FromHex(TxnContractRuntimeBin))

	for i := 0; i <= 9999999; i++ {
		statedb.AddBalance(messageSenderAddress, big.NewInt(9000000000000000000))
		//statedb.AddBalance(runnerContractAddress, big.NewInt(9000000000000000000))
		//statedb.AddBalance(messageSenderAddress, big.NewInt(9000000000000000000))
	}

	statedb.Finalise(true) // Push the state into the "original" slot

	return statedb
}

// largeNumber returns a very large big.Int.
func largeNumber(megabytes int) *big.Int {
	buf := make([]byte, megabytes*1024*1024)
	rand.Read(buf)
	bigint := new(big.Int)
	bigint.SetBytes(buf)
	return bigint
}

type TestChainContext struct {
	Eng consensus.Engine
}

func (tcc *TestChainContext) Engine() consensus.Engine {
	return tcc.Eng
}

func (tcc *TestChainContext) GetHeader(lastKnownHash common.Hash, lastKnownNumber uint64) *types.Header {
	hash := common.BytesToHash([]byte(strconv.FormatUint(lastKnownNumber+1, 10)))
	blockNumber := big.NewInt(int64(lastKnownNumber + 1))

	header := &types.Header{
		MixDigest:   hash,
		ReceiptHash: hash,
		TxHash:      hash,
		Nonce:       types.BlockNonce{},
		Extra:       []byte{},
		Bloom:       types.Bloom{},
		GasUsed:     0,
		Coinbase:    common.Address{},
		GasLimit:    0,
		Time:        1337,
		ParentHash:  lastKnownHash,
		Root:        hash,
		Number:      blockNumber,
		Difficulty:  largeNumber(2),
	}

	return header
}

var chainConfig = &params.ChainConfig{
	ChainID:             big.NewInt(1),
	HomesteadBlock:      new(big.Int),
	EIP155Block:         new(big.Int),
	EIP150Block:         new(big.Int),
	EIP158Block:         new(big.Int),
	ByzantiumBlock:      new(big.Int),
	ConstantinopleBlock: new(big.Int),
	PetersburgBlock:     new(big.Int),
	IstanbulBlock:       new(big.Int),
	BerlinBlock:         new(big.Int),
	LondonBlock:         new(big.Int),
}

var engine = mockconsensus.New(chainConfig, nil, common.HexToHash(GENESIS_BLOCK_HASH))

var tcc = &TestChainContext{Eng: engine}

func execute(tcc *TestChainContext, data []byte, from common.Address, state *state.StateDB, header *types.Header, value *big.Int) (hexutil.Bytes, error) {
	msgData := (hexutil.Bytes)(data)

	args := ethapi.TransactionArgs{
		From:  &from,
		To:    &txnContractAddress,
		Data:  &msgData,
		Value: (*hexutil.Big)(value),
	}

	msg, err := args.ToMessage(math.MaxUint64)
	if err != nil {
		return nil, err
	}

	vmError := func() error { return nil }
	vmConfig := &vm.Config{OverrideGasFailure: true}

	txContext := core.NewEVMTxContext(msg)
	context := core.NewEVMBlockContext(header, tcc, nil)
	evm := vm.NewEVM(context, txContext, state, chainConfig, *vmConfig)

	gp := new(core.GasPool).AddGas(math.MaxUint64)
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	if err = vmError(); err != nil {
		return nil, err
	}

	if result == nil {
		return nil, errors.New("result is nil")
	}

	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, core.NewRevertError(result)
	}
	return result.Return(), result.Err
}

func encodeCall(abi *abi.ABI, method string, args ...interface{}) ([]byte, error) {
	return abi.Pack(method, args...)
}

func GetTxnContract_ABI() (abi.ABI, error) {
	s := TransactionMetaData.ABI
	abi, err := abi.JSON(strings.NewReader(s))
	return abi, err
}

func executeTxn(state *state.StateDB, iterations uint64) error {
	method := "deposit"
	abiData, err := GetTxnContract_ABI()
	if err != nil {
		log.Error("deposit abi error", "err", err)
		return err
	}
	// call
	data, err := encodeCall(&abiData, method)
	if err != nil {
		log.Error("Unable to pack deposit", "error", err)
		return err
	}

	header := tcc.GetHeader(common.ZERO_HASH, uint64(1))
	value := big.NewInt(1)

	var result hexutil.Bytes

	startTime := time.Now()
	for i := uint64(0); i < iterations; i++ {
		result, err = execute(tcc, data, messageSenderAddress, state, header, value)
		if err != nil {
			log.Error("Unable to execute", "error", err)
			return err
		}

		if len(result) == 0 {
			return errors.New("deposit result is 0")
		}
	}
	timeTaken := time.Since(startTime)
	fmt.Println("deposit", "iterations", iterations, "time taken", timeTaken)

	var (
		totalDeposited = big.NewInt(0)
	)

	if err = abiData.UnpackIntoInterface(&totalDeposited, method, result); err != nil {
		log.Error("deposit UnpackIntoInterface", "err", err, "totalDeposited", totalDeposited)
	}

	fmt.Println("deposit", "totalDeposited", totalDeposited)

	return nil
}

func TestTxnSingle(t *testing.T) {
	var err error

	sdb := newStateDb()

	err = executeTxn(sdb, 1)
	if err != nil {
		fmt.Println("error a", err)
		t.Fatalf("failed a")
	}

}

func TestTxnTen(t *testing.T) {
	var err error

	sdb := newStateDb()

	err = executeTxn(sdb, 1)
	if err != nil {
		fmt.Println("error a", err)
		t.Fatalf("failed a")
	}

}

func TestTxnHundred(t *testing.T) {
	var err error

	sdb := newStateDb()

	err = executeTxn(sdb, 100)
	if err != nil {
		fmt.Println("error a", err)
		t.Fatalf("failed a")
	}

}

func TestTxnThousand(t *testing.T) {
	var err error

	sdb := newStateDb()

	err = executeTxn(sdb, 1000)
	if err != nil {
		fmt.Println("error a", err)
		t.Fatalf("failed a")
	}

}

func TestTxnTenThousand(t *testing.T) {
	var err error

	sdb := newStateDb()

	err = executeTxn(sdb, 10000)
	if err != nil {
		fmt.Println("error a", err)
		t.Fatalf("failed a")
	}

}

func TestTxnHundredThousand(t *testing.T) {
	var err error

	sdb := newStateDb()

	err = executeTxn(sdb, 1000000)
	if err != nil {
		fmt.Println("error a", err)
		t.Fatalf("failed a")
	}

}
