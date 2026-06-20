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
	"container/heap"
	"errors"
	"io"
	"math/big"
	"runtime/debug"
	"sort"
	"sync/atomic"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/log"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

var (
	ErrInvalidSig           = errors.New("transaction invalid v, r, s values")
	ErrUnexpectedProtection = errors.New("transaction type does not supported EIP-155 protected signatures")
	ErrInvalidTxType        = errors.New("transaction type not valid in this context")
	ErrTxTypeNotSupported   = errors.New("transaction type not supported")
	ErrGasFeeCapTooLow      = errors.New("fee cap less than base fee")
	errEmptyTypedTx         = errors.New("empty typed transaction bytes")
)

// Transaction types.
const (
	DefaultFeeTxType = iota
	DynamicFeeTxType = iota
)

// Transaction is an Ethereum transaction.
type Transaction struct {
	inner TxData    // Consensus contents of a transaction
	time  time.Time // Time first seen locally (spam avoidance)

	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

// NewTx creates a new transaction.
func NewTx(inner TxData) *Transaction {
	tx := new(Transaction)
	tx.setDecoded(inner.copy(), 0)
	return tx
}

// TxData is the underlying data of a transaction.
//
// This is implemented by DynamicFeeTx, LegacyTx and AccessListTx.
type TxData interface {
	txType() byte // returns the type ID
	copy() TxData // creates a deep copy and initializes all fields

	chainID() *big.Int
	accessList() AccessList
	data() []byte
	gas() uint64
	gasPrice() *big.Int
	gasTipCap() *big.Int
	gasFeeCap() *big.Int
	maxGasTier() GasTier
	signingContext() byte
	value() *big.Int
	nonce() uint64
	to() *common.Address
	remarks() []byte
	verifyFields() bool

	rawSignatureValues() (v, r, s *big.Int)
	setSignatureValues(chainID, v, r, s *big.Int)
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	// It's an EIP-2718 typed TX envelope.
	buf := encodeBufferPool.Get().(*bytes.Buffer)
	defer encodeBufferPool.Put(buf)
	buf.Reset()
	if err := tx.encodeTyped(buf); err != nil {
		return err
	}
	return rlp.Encode(w, buf.Bytes())
}

// encodeTyped writes the canonical encoding of a typed transaction to w.
func (tx *Transaction) encodeTyped(w *bytes.Buffer) error {
	w.WriteByte(tx.Type())
	return rlp.Encode(w, tx.inner)
}

// RawHash outputs the raw hash
func (tx *Transaction) RawHash() (common.Hash, error) {
	var buff bytes.Buffer
	err := tx.encodeTyped(&buff)
	if err != nil {
		return common.Hash{}, err
	}
	return common.BytesToHash(buff.Bytes()), nil
}

// MarshalBinary returns the canonical encoding of the transaction.
// For legacy transactions, it returns the RLP encoding. For EIP-2718 typed
// transactions, it returns the type and payload.
func (tx *Transaction) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	err := tx.encodeTyped(&buf)
	return buf.Bytes(), err
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	kind, _, err := s.Kind()
	switch {
	case err != nil:
		return err
	case kind == rlp.String:
		// It's an EIP-2718 typed TX envelope.
		var b []byte
		if b, err = s.Bytes(); err != nil {
			return err
		}
		inner, err := tx.decodeTyped(b)
		if err == nil {
			tx.setDecoded(inner, len(b))
		}
		return err
	default:
		return rlp.ErrExpectedList
	}
}

// UnmarshalBinary decodes the canonical encoding of transactions.
// It supports legacy RLP transactions and EIP2718 typed transactions.
func (tx *Transaction) UnmarshalBinary(b []byte) error {
	if len(b) > 0 && b[0] > 0x7f {
		// It's a legacy transaction.
		return errors.New("unsupported txn")
	}
	// It's an EIP2718 typed transaction envelope.
	inner, err := tx.decodeTyped(b)
	if err != nil {
		return err
	}
	tx.setDecoded(inner, len(b))
	return nil
}

// decodeTyped decodes a typed transaction from the canonical format.
func (tx *Transaction) decodeTyped(b []byte) (TxData, error) {
	if len(b) == 0 {
		return nil, errEmptyTypedTx
	}
	switch b[0] {

	case DefaultFeeTxType:
		var inner DefaultFeeTx
		err := rlp.DecodeBytes(b[1:], &inner)
		return &inner, err
	case DynamicFeeTxType:
		var inner DynamicFeeTx
		err := rlp.DecodeBytes(b[1:], &inner)
		return &inner, err
	default:
		return nil, ErrTxTypeNotSupported
	}
}

// setDecoded sets the inner transaction and size after decoding.
func (tx *Transaction) setDecoded(inner TxData, size int) {
	tx.inner = inner
	//tx.time = time.Now()
	tx.time = time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	if size > 0 {
		tx.size.Store(common.StorageSize(size))
	}
}

// Protected says whether the transaction is replay-protected.
func (tx *Transaction) Protected() bool {
	return true
}

// Type returns the transaction type.
func (tx *Transaction) Type() uint8 {
	return tx.inner.txType()
}

// ChainId returns the EIP155 chain ID of the transaction. The return value will always be
// non-nil. For legacy transactions which are not replay-protected, the return value is
// zero.
func (tx *Transaction) ChainId() *big.Int {
	return tx.inner.chainID()
}

// Data returns the input data of the transaction.
func (tx *Transaction) Data() []byte { return tx.inner.data() }

// AccessList returns the access list of the transaction.
func (tx *Transaction) AccessList() AccessList { return tx.inner.accessList() }

// Gas returns the gas limit of the transaction.
func (tx *Transaction) Gas() uint64 { return tx.inner.gas() }

// GasPrice returns the gas price of the transaction.
func (tx *Transaction) GasPrice() *big.Int { return new(big.Int).Set(tx.inner.gasPrice()) }

func (tx *Transaction) MaxGasTier() *big.Int { return new(big.Int).Set(tx.inner.gasPrice()) }

// GasTipCap returns the gasTipCap per gas of the transaction.
func (tx *Transaction) GasTipCap() *big.Int { return new(big.Int).Set(tx.inner.gasTipCap()) }

// GasFeeCap returns the fee cap per gas of the transaction.
func (tx *Transaction) GasFeeCap() *big.Int { return new(big.Int).Set(tx.inner.gasFeeCap()) }

func (tx *Transaction) SigningContext() byte { return tx.inner.signingContext() }

// Value returns the ether amount of the transaction.
func (tx *Transaction) Value() *big.Int { return new(big.Int).Set(tx.inner.value()) }

// Nonce returns the sender account nonce of the transaction.
func (tx *Transaction) Nonce() uint64 { return tx.inner.nonce() }

func (tx *Transaction) Remarks() []byte { return tx.inner.remarks() }

func (tx *Transaction) VerifyFields() bool { return tx.inner.verifyFields() }

// To returns the recipient address of the transaction.
// For contract-creation transactions, To returns nil.
func (tx *Transaction) To() *common.Address {
	// Copy the pointed-to address.
	ito := tx.inner.to()
	if ito == nil {
		return nil
	}
	cpy := *ito
	return &cpy
}

// Cost returns the maximum coins a transaction can spend: gas * maxFeePerGas + value, where
// maxFeePerGas is max(baseFee, gasFeeCap). The base fee is the floor that is always charged; once
// gas tip support is active the sender can additionally be charged up to (gasFeeCap - baseFee) of
// tip per gas, so the cap-based bound reserves enough balance to cover baseFee + effectiveTip.
//
// This is backward compatible before GasTipStartBlock: pre-fork dynamic-fee transactions are
// required to have gasFeeCap == 0 (so max(baseFee, 0) == baseFee) and default-fee transactions have
// gasFeeCap == baseFee, leaving Cost() identical to the legacy gasPrice * gas + value.
func (tx *Transaction) Cost() *big.Int {
	feePerGas := tx.BaseFee()
	if feeCap := tx.GasFeeCap(); feeCap.Cmp(feePerGas) > 0 {
		feePerGas = feeCap
	}
	total := new(big.Int).Mul(feePerGas, new(big.Int).SetUint64(tx.Gas()))
	total.Add(total, tx.Value())
	return total
}

// RawSignatureValues returns the V, R, S signature values of the transaction.
// The return values should not be modified by the caller.
func (tx *Transaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.inner.rawSignatureValues()
}

// Hash returns the transaction hash.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	var h common.Hash
	h = prefixedRlpHash(tx.Type(), tx.inner)
	tx.hash.Store(h)
	return h
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previously cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.inner)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be in the [R || S || V] format where V is 0 or 1.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := tx.inner.copy()
	cpy.setSignatureValues(signer.ChainID(), v, r, s)
	t := time.Date(
		2009, 11, 17, 20, 34, 58, 651387237, time.UTC)

	copiedTxn := &Transaction{inner: cpy, time: t}
	_, err = Sender(signer, copiedTxn)
	if err != nil {
		return nil, err
	}
	return &Transaction{inner: cpy, time: t}, nil
}

func (tx *Transaction) Verify(digestHash []byte) bool {
	v, r, s := tx.RawSignatureValues()
	if v.Uint64() != 1 {
		return false
	}
	isOk, _, _ := cryptobase.DynamicSigVerifier.ValidateSignatureValues(digestHash, byte(v.Uint64()), r, s)
	return isOk
}

// Transactions implements DerivableList for transactions.
type Transactions []*Transaction

func (s Transactions) Less(i, j int) bool { return s[i].Nonce() < s[j].Nonce() }
func (s Transactions) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// EncodeIndex encodes the i'th transaction to w. Note that this does not check for errors
// because we assume that *Transaction will only ever contain valid txns that were either
// constructed by decoding or via public API in this package.
func (s Transactions) EncodeIndex(i int, w *bytes.Buffer) {
	tx := s[i]
	tx.encodeTyped(w)
}

func (s Transactions) IsEqualTo(other Transactions) bool {
	if len(s) != len(other) {
		log.Warn("transactions are not equal", "s len", len(s), "other len", len(other))
		return false
	}

	for i, _ := range other {
		hashA := s[i].Hash()
		hashB := other[i].Hash()
		if hashA.IsEqualTo(hashB) == false {
			log.Warn("transactions are not equal", "i", i, "hashA", hashA.Hex(), "hashB", hashB.Hex())
			return false
		}
	}

	return true
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].Nonce() < s[j].Nonce() }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// WrapperTxn wraps a transaction with its gas price or effective miner gasTipCap
type WrapperTxn struct {
	tx         *Transaction
	sortPrefix []byte
}

// NewWrapperTxn creates a wrapped transaction, calculating the effective
// miner gasTipCap if a base fee is provided.
// Returns error in case of a negative effective miner gasTipCap.
func NewWrapperTxn(tx *Transaction, sortPrefix []byte) (*WrapperTxn, error) {
	return &WrapperTxn{
		tx:         tx,
		sortPrefix: sortPrefix,
	}, nil
}

// TxBySortPrefix implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxBySortPrefix []*WrapperTxn

func (s TxBySortPrefix) Len() int { return len(s) }
func (s TxBySortPrefix) Less(i, j int) bool {
	cmp := bytes.Compare(s[i].sortPrefix, s[j].sortPrefix) < 0
	return cmp
}
func (s TxBySortPrefix) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s *TxBySortPrefix) Push(x interface{}) {
	*s = append(*s, x.(*WrapperTxn))
}

func (s *TxBySortPrefix) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByNonce represents a set of transactions supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByNonce struct {
	txns             map[common.Address]Transactions // Per account nonce-sorted list of transactions
	heads            TxBySortPrefix                  // Next transaction for each unique account (heap)
	signer           Signer                          // Signer for the set of transactions
	parentHash       common.Hash
	orderedAddresses []common.Address
	addressIndex     int
	round            int
}

func flatten(txs map[common.Address]Transactions) Transactions {
	var transactions Transactions
	for _, txnList := range txs {
		for i := 0; i < len(txnList); i++ {
			transactions = append(transactions, txnList[i])
		}
	}

	return transactions
}

func NewTransactionsByNonceFromList(signer Signer, txnList *Transactions, parentHash common.Hash) (*TransactionsByNonce, Transactions, error) {
	var skippedTransactions Transactions
	txs := make(map[common.Address]Transactions)
	for _, txn := range *txnList {
		from, err := Sender(signer, txn)
		if err != nil {
			skippedTransactions = append(skippedTransactions, txn)
			log.Debug("NewTransactionsByNonceFromList", "error", err, "txn", txn.Hash().Hex(), "len skip", len(skippedTransactions))
			continue
		}
		_, ok := txs[from]
		if ok == false {
			txs[from] = make(Transactions, 0)
		}
		txs[from] = append(txs[from], txn)
	}
	txnByNonce, skipped, err := NewTransactionsByNonce(signer, txs, parentHash)
	if err != nil {
		log.Debug("NewTransactionsByNonceFromList NewTransactionsByNonce", "error", err, "skip count", len(skipped))
		return nil, nil, err
	}
	skippedTransactions = append(skippedTransactions, skipped...)
	// Sort by tx hash so that the same input always yields the same order (map iteration in
	// flatten/TxDifference is non-deterministic). Consensus and EncodeBlockExtraData depend on
	// deterministic error transaction order.
	sort.Slice(skippedTransactions, func(i, j int) bool {
		return bytes.Compare(skippedTransactions[i].Hash().Bytes(), skippedTransactions[j].Hash().Bytes()) < 0
	})
	return txnByNonce, skippedTransactions, nil
}

// NewTransactionsByNonce creates a transaction set that can retrieve transactions in a nonce-honouring way.
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByNonce(signer Signer, txs map[common.Address]Transactions, parentHash common.Hash) (*TransactionsByNonce, Transactions, error) {
	before := flatten(txs)

	// Extract keys upfront so deletions don't affect iteration.
	addresses := make([]common.Address, 0, len(txs))
	for from := range txs {
		addresses = append(addresses, from)
	}

	// Initialize a time based heap with the head transactions
	heads := make(TxBySortPrefix, 0, len(txs))
	for _, from := range addresses {
		accTxs := txs[from]
		for i := 0; i < len(accTxs); i++ {
			_, err := Sender(signer, accTxs[i])
			if err != nil {
				delete(txs, from)
				continue
			}
		}
		if len(accTxs) == 0 {
			delete(txs, from)
			continue
		}
		sort.Sort(accTxs)
		prevTxn := accTxs[0]
		for i := 1; i < len(accTxs); i++ {
			if accTxs[i].Nonce() != prevTxn.Nonce()+1 {
				accTxs = accTxs[:i]
				break
			}
			prevTxn = accTxs[i]
		}
		if len(accTxs) == 0 {
			delete(txs, from)
			continue
		}
		acc, err := Sender(signer, accTxs[0])
		if err != nil {
			return nil, nil, err
		}
		sortPrefix := crypto.Keccak256(parentHash.Bytes(), acc.Bytes())
		wrapped, err := NewWrapperTxn(accTxs[0], sortPrefix)
		// Remove transaction if sender doesn't match from, or if wrapping fails.
		if acc != from || err != nil {
			delete(txs, from)
			continue
		}
		heads = append(heads, wrapped)
		txs[from] = accTxs[0:]
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	output := &TransactionsByNonce{
		txns:       txs,
		heads:      heads,
		signer:     signer,
		parentHash: parentHash,
	}
	output.internalSort()
	output.ResetCursor()

	after := flatten(txs)
	skippedTransactions := TxDifference(before, after)

	log.Trace("NewTransactionsByNonce", "before txn count", len(before), "after txn count", len(after))

	return output, skippedTransactions, nil
}

func (t *TransactionsByNonce) GetList() []common.Hash {
	txnList := make([]common.Hash, 0, len(t.txns))

	for _, accTxs := range t.txns {
		for i := 0; i < len(accTxs); i++ {
			txnList = append(txnList, accTxs[i].Hash())
		}
	}

	sort.Slice(txnList, func(i, j int) bool {
		return bytes.Compare(txnList[i].Bytes(), txnList[j].Bytes()) < 0
	})

	return txnList
}

func (t *TransactionsByNonce) GetTotalCount() int {
	count := 0

	for _, accTxs := range t.txns {
		count = count + len(accTxs)
	}

	return count
}

func (t *TransactionsByNonce) GetMap() map[common.Address]Transactions {
	return t.txns
}

// Peek returns the next transaction
func (t *TransactionsByNonce) internalSort() {
	t.orderedAddresses = make([]common.Address, len(t.txns))
	txnIndex := 0
	for from, _ := range t.txns {
		t.orderedAddresses[txnIndex] = from
		txnIndex = txnIndex + 1
	}
	parentHashBytes := t.parentHash.Bytes()
	sort.SliceStable(t.orderedAddresses, func(i, j int) bool {
		sortPrefixI := crypto.Keccak256(parentHashBytes, t.orderedAddresses[i].Bytes())
		sortPrefixJ := crypto.Keccak256(parentHashBytes, t.orderedAddresses[j].Bytes())
		cmp := bytes.Compare(sortPrefixI, sortPrefixJ) < 0
		return cmp
	})
}

func (t *TransactionsByNonce) PeekCursor() *Transaction {
	if t.addressIndex < 0 || len(t.txns) == 0 {
		return nil
	}
	return t.txns[t.orderedAddresses[t.addressIndex]][t.round]
}

func (t *TransactionsByNonce) ResetCursor() {
	t.round = 0
	t.addressIndex = -1
}

func (t *TransactionsByNonce) NextCursor() bool {
	if t.addressIndex == -2 {
		return false
	}
	t.addressIndex = t.addressIndex + 1
	if t.addressIndex >= len(t.orderedAddresses) {
		t.addressIndex = 0
		t.round = t.round + 1
	}

	for i := t.addressIndex; i < len(t.orderedAddresses); i++ {
		if t.addressIndex < 0 {
			debug.PrintStack()
		}
		if t.txns[t.orderedAddresses[i]].Len() > t.round {
			t.addressIndex = i
			return true
		}
	}

	t.round = t.round + 1
	for i := 0; i < t.addressIndex; i++ {
		if t.txns[t.orderedAddresses[i]].Len() > t.round {
			t.addressIndex = i
			return true
		}
	}
	t.addressIndex = -2
	return false
}

// Peek returns the next transaction
func (t *TransactionsByNonce) Peek1() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0].tx
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByNonce) Shift1() {
	acc, _ := Sender(t.signer, t.heads[0].tx)
	if txs, ok := t.txns[acc]; ok && len(txs) > 0 {
		sortPrefix := crypto.Keccak256(t.parentHash.Bytes(), acc.Bytes())
		if wrapped, err := NewWrapperTxn(txs[0], sortPrefix); err == nil {
			t.heads[0], t.txns[acc] = wrapped, txs[1:]
			heap.Fix(&t.heads, 0)
			return
		}
	}
	heap.Pop(&t.heads)
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByNonce) Pop1() {
	heap.Pop(&t.heads)
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to             *common.Address
	from           common.Address
	nonce          uint64
	amount         *big.Int
	gasLimit       uint64
	gasPrice       *big.Int
	signingContext byte
	data           []byte
	accessList     AccessList
	checkNonce     bool
	remarks        []byte
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, accessList AccessList, checkNonce bool, signingContext byte) Message {
	return Message{
		from:           from,
		to:             to,
		nonce:          nonce,
		amount:         amount,
		gasLimit:       gasLimit,
		gasPrice:       gasPrice,
		signingContext: signingContext,
		data:           data,
		accessList:     accessList,
		checkNonce:     checkNonce,
	}
}

// AsMessage returns the transaction as a core.Message.
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:          tx.Nonce(),
		gasLimit:       tx.Gas(),
		gasPrice:       new(big.Int).Set(tx.GasPrice()),
		to:             tx.To(),
		amount:         tx.Value(),
		signingContext: tx.SigningContext(),
		data:           tx.Data(),
		accessList:     tx.AccessList(),
		checkNonce:     true,
		remarks:        tx.Remarks(),
	}
	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

func (m Message) From() common.Address { return m.from }
func (m Message) To() *common.Address  { return m.to }
func (m Message) GasPrice() *big.Int   { return m.gasPrice }
func (m Message) Value() *big.Int      { return m.amount }
func (m Message) Gas() uint64          { return m.gasLimit }
func (m Message) SigningContext() byte { return m.signingContext }

// Tip returns the tip per gas of the transaction.
func (tx *Transaction) Tip() *big.Int { return new(big.Int).Set(tx.inner.gasTipCap()) }

// FeeCap returns the fee cap per gas of the transaction.
func (tx *Transaction) FeeCap() *big.Int { return new(big.Int).Set(tx.inner.gasFeeCap()) }

// BaseFee returns the fixed base fee per gas for the transaction, derived from its
// signing context (DynamicFeeTx) or the fixed default price (DefaultFeeTx). It is the
// same value returned by GasPrice and is the floor that GasFeeCap must cover once gas
// tip support is active.
func (tx *Transaction) BaseFee() *big.Int { return tx.GasPrice() }

// EffectiveGasTip returns the effective miner tip per gas given a base fee:
// min(gasTipCap, gasFeeCap - baseFee). It returns ErrGasFeeCapTooLow when a non-zero fee
// cap cannot cover the base fee. When baseFee is nil it falls back to the raw tip cap.
//
// A zero gasFeeCap means the sender opted out of tips: this is the legacy/default
// DynamicFeeTx case where GasFeeCap/GasTipCap are unset (null) and normalized to zero. Such
// a transaction contributes no tip and is charged only the base fee, preserving backward
// compatibility for existing senders after the fork. Nil inner cap fields are treated as
// zero so pre-tip transactions never panic.
func (tx *Transaction) EffectiveGasTip(baseFee *big.Int) (*big.Int, error) {
	tip := new(big.Int)
	if t := tx.inner.gasTipCap(); t != nil {
		tip.Set(t)
	}
	if baseFee == nil {
		return tip, nil
	}
	feeCap := new(big.Int)
	if f := tx.inner.gasFeeCap(); f != nil {
		feeCap.Set(f)
	}
	if feeCap.Sign() == 0 {
		// Opted out of tips: no tip, base fee only.
		return new(big.Int), nil
	}
	if feeCap.Cmp(baseFee) < 0 {
		return nil, ErrGasFeeCapTooLow
	}
	gap := new(big.Int).Sub(feeCap, baseFee)
	if tip.Cmp(gap) > 0 {
		tip = gap
	}
	return tip, nil
}

func (m Message) Nonce() uint64                   { return m.nonce }
func (m Message) Data() []byte                    { return m.data }
func (m Message) AccessList() AccessList          { return m.accessList }
func (m Message) CheckNonce() bool                { return m.checkNonce }
func (m Message) Remarks() []byte                 { return m.remarks }
func (m Message) OverrideGasPrice(price *big.Int) { m.gasPrice.Set(price) }
