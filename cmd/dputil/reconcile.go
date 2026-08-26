package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/accounts"
	"github.com/quantumcoinproject/quantum-coin-go/accounts/abi/bind"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/common/hexutil"
	"github.com/quantumcoinproject/quantum-coin-go/core/types"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/crosssign"
	"github.com/quantumcoinproject/quantum-coin-go/ethclient"
	"github.com/quantumcoinproject/quantum-coin-go/token"
	"github.com/quantumcoinproject/quantum-coin-go/token/tokenconversion"
)

const (
	reconcileDefaultConversionContract = "0x90b6b4e9cf99255a7a527f0e8e5a9a8669af4a8b56b030353127f809292f0632"
	reconcileDefaultHeisenContract     = "0xe8ea8beb86e714ef2bde0afac17d6e45d1c35e48f312d6dc12c4fdb90d9e8a3d"
	reconcileDefaultHolder             = "0x64460985341a560817a086d65330cf8f8acdb2dd3c4ac7748bb2e42f373a7a93"
	reconcileDefaultEthContract        = "0xE7eaec9Bca79d537539C00C58Ae93117fB7280b9"
	reconcileDeadAddressLC             = "0x000000000000000000000000000000000000dead"
	reconcileDefaultReadApi            = "https://app.readrelay.quantumcoinapi.com"
	// event signature topics
	reconcileOnRequestConversionTopic = "0x9ccd4a2af21756c1ba8263d96923711e2a9d7411ef76b11ac11c61229286e96e"
	reconcileTransferTopic            = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
)

var reconcileWeiPerToken = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

type reconcileStringList []string

func (s *reconcileStringList) String() string { return strings.Join(*s, ";") }
func (s *reconcileStringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type reconcileRequest struct {
	Index          uint64
	QuantumAddress common.Address
	EthAddressRaw  string
	EthAddressLC   string
	Signature      string
	TxnHash        string // empty if recovered via view call fallback
	BlockNumber    uint64
	SigValid       bool
	SigVariant     string
	SigErr         string
}

type reconcileBurn struct {
	TxHash    string
	BlockNo   string
	Time      string
	FromLC    string
	AmountWei *big.Int
}

type reconcileSent struct {
	Total    *big.Int
	TxHashes []string
	Amounts  []*big.Int
}

type reconcileRow struct {
	EthAddressLC   string
	QuantumAddress string // HexLower of winning request; "" if none
	Winning        *reconcileRequest
	RequestCount   int
	SnapshotWei    *big.Int
	BurnedWei      *big.Int
	GrossBurnWei   *big.Int
	BurnTxHashes   []string
	AlreadySentWei *big.Int
	SentTxHashes   []string
	PayoutWei      *big.Int
	PendingWei     *big.Int
	Flags          []string
}

// withinWei reports whether a and b differ by at most tol wei.
func withinWei(a *big.Int, b *big.Int, tol *big.Int) bool {
	d := new(big.Int).Sub(a, b)
	return d.Abs(d).Cmp(tol) <= 0
}

func (r *reconcileRow) hasFlag(f string) bool {
	for _, x := range r.Flags {
		if x == f {
			return true
		}
	}
	return false
}

// decimalStrToWei converts a decimal token amount string (optionally with
// thousands separators, up to 18 fractional digits) to exact wei.
func decimalStrToWei(s string) (*big.Int, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	s = strings.TrimPrefix(s, "\uFEFF")
	if len(s) == 0 {
		return nil, errors.New("empty amount")
	}
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if len(intPart) == 0 {
		intPart = "0"
	}
	if len(fracPart) > 18 {
		return nil, fmt.Errorf("more than 18 fractional digits in amount %q", s)
	}
	fracPart = fracPart + strings.Repeat("0", 18-len(fracPart))
	digits := intPart + fracPart
	for _, c := range digits {
		if c < '0' || c > '9' {
			return nil, fmt.Errorf("invalid character in amount %q", s)
		}
	}
	wei, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, fmt.Errorf("cannot parse amount %q", s)
	}
	return wei, nil
}

// weiToTokenStr renders wei as a whole-token decimal string, trailing zeros trimmed.
func weiToTokenStr(wei *big.Int) string {
	if wei == nil {
		return "0"
	}
	q := new(big.Int).Quo(wei, reconcileWeiPerToken)
	r := new(big.Int).Mod(wei, reconcileWeiPerToken)
	if r.Sign() == 0 {
		return q.String()
	}
	frac := fmt.Sprintf("%018s", r.String())
	frac = strings.TrimRight(frac, "0")
	return q.String() + "." + frac
}

// weiToTokenStr18 renders wei as a whole-token decimal string with exactly 18
// fractional digits (the multitransfertokens-compatible form).
func weiToTokenStr18(wei *big.Int) string {
	q := new(big.Int).Quo(wei, reconcileWeiPerToken)
	r := new(big.Int).Mod(wei, reconcileWeiPerToken)
	return q.String() + "." + fmt.Sprintf("%018s", r.String())
}

func reconcileReadCsv(filePath string) ([][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	records, err := rd.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) > 0 && len(records[0]) > 0 {
		records[0][0] = strings.TrimPrefix(records[0][0], "\uFEFF")
	}
	return records, nil
}

type relayTxn struct {
	Hash        string `json:"hash"`
	BlockNumber int64  `json:"blockNumber"`
	From        string `json:"from"`
	To          string `json:"to"`
}

type relayTxnPage struct {
	PageCount int        `json:"pageCount"`
	Items     []relayTxn `json:"items"`
}

// relayAccountTxns pages through the read-relay API's transaction list for an
// account. This avoids scanning the chain: only blocks that actually contain
// transactions touching the account are visited (via their receipts).
func relayAccountTxns(baseURL string, address string) ([]relayTxn, error) {
	var out []relayTxn
	seen := make(map[string]bool)
	page := 1
	for {
		url := fmt.Sprintf("%s/account/%s/transactions/%d", strings.TrimRight(baseURL, "/"), address, page)
		var body []byte
		var err error
		for attempt := 0; attempt < 4; attempt++ {
			var resp *http.Response
			resp, err = http.Get(url)
			if err == nil && resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("http status %d", resp.StatusCode)
			}
			if err == nil {
				body, err = io.ReadAll(resp.Body)
			}
			if resp != nil {
				resp.Body.Close()
			}
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			return nil, fmt.Errorf("read api %s: %v", url, err)
		}
		var p relayTxnPage
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("read api %s: %v", url, err)
		}
		for _, t := range p.Items {
			h := strings.ToLower(t.Hash)
			if h == "" || seen[h] {
				continue
			}
			seen[h] = true
			out = append(out, t)
		}
		if len(p.Items) == 0 || page >= p.PageCount {
			break
		}
		page++
	}
	return out, nil
}

// heisenLogChunk is the block span of a single eth_getLogs query in
// heisenTransfersFrom; ranges are chunked so that a single query never grows
// beyond what the node will serve.
const heisenLogChunk = uint64(50000)

// heisenLogRetryDelay is the pause between failed eth_getLogs attempts (a var so
// tests can zero it).
var heisenLogRetryDelay = 2 * time.Second

// heisenTransfersFrom aggregates, per recipient, every Transfer event of the
// Heisen token whose indexed `from` is holder, over blocks [0, head]. A Transfer
// that cannot be decoded is an error (not a warning): silently dropping one would
// undercount AlreadySent and lead to a duplicate payout.
func heisenTransfersFrom(heisenToken *token.Token, holder common.Address, head uint64) (map[string]*reconcileSent, int, error) {
	sentByQuantum := make(map[string]*reconcileSent)
	transferCount := 0
	for start := uint64(0); start <= head; start += heisenLogChunk {
		end := start + heisenLogChunk - 1
		if end > head || end < start { // end < start only on uint64 wrap
			end = head
		}
		var events []*token.TokenTransfer
		var err error
		for attempt := 0; attempt < 4; attempt++ {
			events, err = heisenTransfersFromRange(heisenToken, holder, start, end)
			if err == nil {
				break
			}
			time.Sleep(heisenLogRetryDelay)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("Transfer logs %d-%d: %v", start, end, err)
		}
		for _, ev := range events {
			if !ev.From.IsEqualTo(holder) {
				return nil, 0, fmt.Errorf("Transfer logs %d-%d: filter returned from=%s (expected %s) in %s", start, end, ev.From.Hex(), holder.Hex(), ev.Raw.TxHash.Hex())
			}
			key := ev.To.HexLower()
			agg, ok := sentByQuantum[key]
			if !ok {
				agg = &reconcileSent{Total: new(big.Int)}
				sentByQuantum[key] = agg
			}
			agg.Total = new(big.Int).Add(agg.Total, ev.Value)
			agg.TxHashes = append(agg.TxHashes, ev.Raw.TxHash.Hex())
			agg.Amounts = append(agg.Amounts, new(big.Int).Set(ev.Value))
			transferCount++
		}
		if end == head || (end-start+1) >= heisenLogChunk && ((end+1)/heisenLogChunk)%10 == 0 {
			fmt.Printf("[transfers] scanned blocks 0-%d of %d, events so far: %d\n", end, head, transferCount)
		}
		if end == head {
			break
		}
	}
	return sentByQuantum, transferCount, nil
}

// heisenTransfersFromRange runs one filtered eth_getLogs query for Transfer
// events with from == holder in [start, end] and decodes every event.
func heisenTransfersFromRange(heisenToken *token.Token, holder common.Address, start, end uint64) ([]*token.TokenTransfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	it, err := heisenToken.FilterTransfer(&bind.FilterOpts{Start: start, End: &end, Context: ctx}, []common.Address{holder}, nil)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var events []*token.TokenTransfer
	for it.Next() {
		if it.Event == nil {
			return nil, errors.New("nil Transfer event from iterator")
		}
		events = append(events, it.Event)
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	return events, nil
}

// receiptLogs fetches a transaction receipt and returns its logs if the
// transaction succeeded (nil logs for failed transactions).
func receiptLogs(client *ethclient.Client, txHash string) ([]*types.Log, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	receipt, err := client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return nil, err
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, nil
	}
	return receipt.Logs, nil
}

// verifyRequestSignature tries the message-template casing variants and
// returns (valid, variantName, err). The recovered-signer comparison inside
// VerifyEthereumAddressAndMessage is case-insensitive on the eth address; the
// variants only change the exact message text that was signed.
func verifyRequestSignature(req *reconcileRequest, quantumContract common.Address, ethContractFlag string) (bool, string, string) {
	sig, err := hexutil.MustDecodeWithError(req.Signature)
	if err != nil {
		return false, "", "bad signature hex: " + err.Error()
	}
	if !common.IsLegacyEthereumHexAddress(req.EthAddressRaw) {
		return false, "", "invalid eth address"
	}

	ethAddrVariants := []struct{ name, val string }{
		{"ethLower", strings.ToLower(req.EthAddressRaw)},
		{"ethRaw", req.EthAddressRaw},
	}
	quantumVariants := []struct{ name, val string }{
		{"qLower", req.QuantumAddress.HexLower()},
		{"qHex", req.QuantumAddress.Hex()},
	}
	qContractVariants := []struct{ name, val string }{
		{"qcLower", quantumContract.HexLower()},
		{"qcHex", quantumContract.Hex()},
	}
	ethContractVariants := []struct{ name, val string }{
		{"ecLower", strings.ToLower(ethContractFlag)},
		{"ecRaw", ethContractFlag},
	}

	var lastErr string
	for _, ea := range ethAddrVariants {
		for _, qa := range quantumVariants {
			for _, qc := range qContractVariants {
				for _, ec := range ethContractVariants {
					message := strings.Replace(crosssign.TokenConversionMessageTemplate, "[ETH_ADDRESS]", ea.val, 1)
					message = strings.Replace(message, "[QUANTUM_ADDRESS]", qa.val, 1)
					message = strings.Replace(message, "[QUANTUM_CONTRACT_ADDRESS]", qc.val, 1)
					message = strings.Replace(message, "[ETH_CONTRACT_ADDRESS]", ec.val, 1)
					digest, _ := accounts.TextAndHash([]byte(message))
					err = crosssign.VerifyEthereumAddressAndMessage(req.EthAddressRaw, digest, sig)
					if err == nil {
						return true, ea.name + "/" + qa.name + "/" + qc.name + "/" + ec.name, ""
					}
					lastErr = err.Error()
				}
			}
		}
	}
	return false, "", lastErr
}

func ReconcileTokenConversions() error {
	fs := flag.NewFlagSet("reconciletokenconversions", flag.ContinueOnError)
	var burnFiles reconcileStringList
	snapshotPath := fs.String("snapshot", "", "path to snapshot csv (Address,TokenBalanceInWei,...)")
	fs.Var(&burnFiles, "burns", "path to an Etherscan token-transfer export csv (repeatable)")
	outFolder := fs.String("out", "", "output folder")
	conversionContractStr := fs.String("conversion-contract", reconcileDefaultConversionContract, "token conversion contract address on QuantumCoin")
	heisenContractStr := fs.String("heisen", reconcileDefaultHeisenContract, "Heisen token contract address on QuantumCoin")
	holderStr := fs.String("holder", reconcileDefaultHolder, "token holder (sender) address on QuantumCoin")
	ethContractStr := fs.String("eth-contract", reconcileDefaultEthContract, "source ERC20 contract address on Ethereum")
	readApi := fs.String("readapi", reconcileDefaultReadApi, "read relay api base url (account transaction lists)")
	payoutPolicy := fs.String("payout-policy", "snapshot", "payout amount policy: snapshot | min | burned")
	dustWeiStr := fs.String("dust-wei", "10000000000000000000", "treat a remaining balance at or below this many wei (default 10 hei) as settled rounding dust")
	burnToleranceBps := fs.Int64("burn-tolerance-bps", 10, "tolerance in basis points when comparing grossed-up burn to snapshot")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	if len(*snapshotPath) == 0 || len(*outFolder) == 0 || len(burnFiles) == 0 {
		fs.Usage()
		return errors.New("required: -snapshot, -out and at least one -burns")
	}
	if *payoutPolicy != "snapshot" && *payoutPolicy != "min" && *payoutPolicy != "burned" {
		return errors.New("invalid -payout-policy, expected snapshot, min or burned")
	}
	for _, a := range []string{*conversionContractStr, *heisenContractStr, *holderStr} {
		if !common.IsHexAddress(a) {
			return errors.New("invalid quantum address " + a)
		}
	}
	if !common.IsLegacyEthereumHexAddress(*ethContractStr) {
		return errors.New("invalid ethereum contract address " + *ethContractStr)
	}
	dustWei, ok := new(big.Int).SetString(*dustWeiStr, 10)
	if !ok || dustWei.Sign() < 0 {
		return errors.New("invalid -dust-wei value")
	}
	if err := os.MkdirAll(*outFolder, 0755); err != nil {
		return err
	}
	conversionContractAddr := common.HexToAddress(*conversionContractStr)
	heisenAddr := common.HexToAddress(*heisenContractStr)
	holderAddr := common.HexToAddress(*holderStr)

	fmt.Println("rpc url:", rawURL)
	client, err := ethclient.Dial(rawURL)
	if err != nil {
		return err
	}
	defer client.Close()

	headCtx, headCancel := context.WithTimeout(context.Background(), 60*time.Second)
	head, err := client.BlockNumber(headCtx)
	headCancel()
	if err != nil {
		return fmt.Errorf("cannot get chain head: %v", err)
	}
	fmt.Println("chain head block:", head)

	// Step 1: conversion requests (event scan for txn hashes + view-call fallback).
	conv, err := tokenconversion.NewTokenconversion(conversionContractAddr, client)
	if err != nil {
		return err
	}
	countBig, err := conv.GetConversionRequestsCount(nil)
	if err != nil {
		return fmt.Errorf("GetConversionRequestsCount failed: %v", err)
	}
	requestCount := countBig.Uint64()
	fmt.Println("on-chain conversion request count:", requestCount)

	// Load every request through the contract's view API (no chain scan).
	requestsByIndex := make(map[uint64]*reconcileRequest)
	for i := uint64(0); i < requestCount; i++ {
		r, err := conv.GetConversionRequest(nil, new(big.Int).SetUint64(i))
		if err != nil {
			return fmt.Errorf("GetConversionRequest(%d) failed: %v", i, err)
		}
		requestsByIndex[i] = &reconcileRequest{
			Index:          i,
			QuantumAddress: r.QuantumAddress,
			EthAddressRaw:  strings.TrimSpace(r.EthAddress),
			EthAddressLC:   strings.ToLower(strings.TrimSpace(r.EthAddress)),
			Signature:      strings.TrimSpace(r.EthSignature),
		}
		if (i+1)%25 == 0 {
			fmt.Printf("[requests] fetched %d of %d\n", i+1, requestCount)
		}
	}
	fmt.Println("requests fetched:", len(requestsByIndex))

	// Recover each request's submission txn hash from the conversion
	// contract's transaction list (read api) + transaction receipts — only the
	// blocks that actually contain conversion requests are touched.
	onRequestTopic := common.HexToHash(reconcileOnRequestConversionTopic)
	convTxns, err := relayAccountTxns(*readApi, conversionContractAddr.HexLower())
	if err != nil {
		return err
	}
	fmt.Println("conversion contract transactions from read api:", len(convTxns))
	txnHashesFound := 0
	for _, t := range convTxns {
		logs, err := receiptLogs(client, t.Hash)
		if err != nil {
			return fmt.Errorf("receipt %s: %v", t.Hash, err)
		}
		for _, lg := range logs {
			if !lg.Address.IsEqualTo(conversionContractAddr) || len(lg.Topics) == 0 || lg.Topics[0] != onRequestTopic {
				continue
			}
			ev, err := conv.ParseOnRequestConversion(*lg)
			if err != nil {
				fmt.Printf("warning: cannot parse OnRequestConversion in %s: %v\n", t.Hash, err)
				continue
			}
			if req, ok := requestsByIndex[ev.Index.Uint64()]; ok {
				req.TxnHash = lg.TxHash.Hex()
				req.BlockNumber = lg.BlockNumber
				txnHashesFound++
			}
		}
	}
	fmt.Printf("request txn hashes recovered: %d of %d\n", txnHashesFound, len(requestsByIndex))

	// Step 2: verify signatures, group per eth address, pick last valid.
	requests := make([]*reconcileRequest, 0, len(requestsByIndex))
	for _, r := range requestsByIndex {
		requests = append(requests, r)
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].Index < requests[j].Index })
	validCount := 0
	for _, r := range requests {
		r.SigValid, r.SigVariant, r.SigErr = verifyRequestSignature(r, heisenAddr, *ethContractStr)
		if r.SigValid {
			validCount++
		}
	}
	fmt.Printf("signatures valid: %d of %d\n", validCount, len(requests))

	requestsByEth := make(map[string][]*reconcileRequest)
	for _, r := range requests {
		requestsByEth[r.EthAddressLC] = append(requestsByEth[r.EthAddressLC], r)
	}
	winningByEth := make(map[string]*reconcileRequest)
	for eth, list := range requestsByEth {
		for _, r := range list { // list is index-ascending
			if r.SigValid {
				winningByEth[eth] = r
			}
		}
	}

	// Step 3: snapshot.
	snapshotWei := make(map[string]*big.Int)
	snapRecords, err := reconcileReadCsv(*snapshotPath)
	if err != nil {
		return fmt.Errorf("cannot read snapshot csv: %v", err)
	}
	for i, rec := range snapRecords {
		if i == 0 && len(rec) > 0 && strings.EqualFold(rec[0], "Address") {
			continue
		}
		if len(rec) < 2 {
			return fmt.Errorf("snapshot row %d has %d columns", i+1, len(rec))
		}
		addr := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(rec[0], "\uFEFF")))
		if !common.IsLegacyEthereumHexAddress(addr) {
			return fmt.Errorf("snapshot row %d: invalid address %q", i+1, rec[0])
		}
		wei, ok := new(big.Int).SetString(strings.TrimSpace(rec[1]), 10)
		if !ok {
			return fmt.Errorf("snapshot row %d: invalid wei %q", i+1, rec[1])
		}
		if wei.Sign() == 0 {
			continue
		}
		if _, dup := snapshotWei[addr]; dup {
			return fmt.Errorf("snapshot row %d: duplicate address %s", i+1, addr)
		}
		snapshotWei[addr] = wei
	}
	fmt.Println("snapshot entries with non-zero balance:", len(snapshotWei))

	// Step 4: burns.
	burnsByEth := make(map[string][]*reconcileBurn)
	seenBurnTx := make(map[string]bool)
	burnRows, dupRows := 0, 0
	for _, bf := range burnFiles {
		records, err := reconcileReadCsv(bf)
		if err != nil {
			return fmt.Errorf("cannot read burns csv %s: %v", bf, err)
		}
		for i, rec := range records {
			if i == 0 && len(rec) > 0 && strings.EqualFold(rec[0], "Transaction Hash") {
				continue
			}
			if len(rec) < 10 {
				return fmt.Errorf("%s row %d has %d columns", bf, i+1, len(rec))
			}
			txHash := strings.ToLower(strings.TrimSpace(rec[0]))
			status := strings.TrimSpace(rec[1])
			from := strings.ToLower(strings.TrimSpace(rec[5]))
			to := strings.ToLower(strings.TrimSpace(rec[7]))
			if !strings.EqualFold(status, "Success") {
				continue
			}
			if to != reconcileDeadAddressLC {
				continue
			}
			if seenBurnTx[txHash] {
				dupRows++
				continue
			}
			seenBurnTx[txHash] = true
			amount, err := decimalStrToWei(rec[9])
			if err != nil {
				return fmt.Errorf("%s row %d: %v", bf, i+1, err)
			}
			burnsByEth[from] = append(burnsByEth[from], &reconcileBurn{
				TxHash:    txHash,
				BlockNo:   strings.TrimSpace(rec[3]),
				Time:      strings.TrimSpace(rec[4]),
				FromLC:    from,
				AmountWei: amount,
			})
			burnRows++
		}
	}
	fmt.Printf("burn transfers loaded: %d (duplicates across files skipped: %d), distinct burners: %d\n", burnRows, dupRows, len(burnsByEth))

	// Step 5: Heisen transfers from holder (already sent). Every Transfer event
	// of the Heisen token with from == holder is fetched with an indexed log
	// filter (eth_getLogs, chunked by block range). This is authoritative: it
	// also catches transfers the holder did not submit itself (e.g. transferFrom
	// by an approved spender), which the read api's per-account transaction list
	// — indexed by transaction from/to only — would miss and thereby undercount
	// AlreadySent (=> duplicate payout).
	heisenToken, err := token.NewToken(heisenAddr, client)
	if err != nil {
		return err
	}
	sentByQuantum, transferCount, err := heisenTransfersFrom(heisenToken, holderAddr, head)
	if err != nil {
		return err
	}
	fmt.Println("holder outbound Heisen Transfer events:", transferCount)
	totalSent := new(big.Int)
	for _, agg := range sentByQuantum {
		totalSent = new(big.Int).Add(totalSent, agg.Total)
	}
	fmt.Printf("holder outbound Heisen transfers: %d recipients, total %s tokens\n", len(sentByQuantum), weiToTokenStr(totalSent))

	// Step 6: reconcile over the union of eth addresses.
	universe := make(map[string]bool)
	for a := range snapshotWei {
		universe[a] = true
	}
	for a := range burnsByEth {
		universe[a] = true
	}
	for a := range requestsByEth {
		universe[a] = true
	}

	tolLow := big.NewInt(10000 - *burnToleranceBps)
	tolHigh := big.NewInt(10000 + *burnToleranceBps)
	tenThousand := big.NewInt(10000)

	rows := make([]*reconcileRow, 0, len(universe))
	rowByEth := make(map[string]*reconcileRow)
	for eth := range universe {
		row := &reconcileRow{
			EthAddressLC:   eth,
			SnapshotWei:    snapshotWei[eth],
			BurnedWei:      new(big.Int),
			GrossBurnWei:   new(big.Int),
			AlreadySentWei: new(big.Int),
			PayoutWei:      new(big.Int),
			PendingWei:     new(big.Int),
			RequestCount:   len(requestsByEth[eth]),
		}
		for _, b := range burnsByEth[eth] {
			row.BurnedWei = new(big.Int).Add(row.BurnedWei, b.AmountWei)
			row.BurnTxHashes = append(row.BurnTxHashes, b.TxHash)
		}
		// Reverse the 0.1% transfer fee: amount received at the dead address is
		// 99.9% of what the holder sent, so gross = burned * 1000 / 999.
		row.GrossBurnWei = new(big.Int).Quo(new(big.Int).Mul(row.BurnedWei, big.NewInt(1000)), big.NewInt(999))

		if w, ok := winningByEth[eth]; ok {
			row.Winning = w
			row.QuantumAddress = w.QuantumAddress.HexLower()
		} else if row.RequestCount > 0 {
			row.Flags = append(row.Flags, "NO_VALID_REQUEST")
		} else {
			row.Flags = append(row.Flags, "NO_REQUEST")
		}
		if row.SnapshotWei == nil {
			row.Flags = append(row.Flags, "NOT_IN_SNAPSHOT")
		}
		if row.BurnedWei.Sign() == 0 {
			row.Flags = append(row.Flags, "NO_BURN_FOUND")
		}

		eligible := row.Winning != nil && row.SnapshotWei != nil && row.BurnedWei.Sign() > 0
		if eligible {
			gross10k := new(big.Int).Mul(row.GrossBurnWei, tenThousand)
			if gross10k.Cmp(new(big.Int).Mul(row.SnapshotWei, tolLow)) < 0 {
				row.Flags = append(row.Flags, "PARTIAL_BURN")
			}
			if gross10k.Cmp(new(big.Int).Mul(row.SnapshotWei, tolHigh)) > 0 {
				row.Flags = append(row.Flags, "AMOUNT_MISMATCH")
			}
			switch {
			case *payoutPolicy == "burned":
				// Matches the operator's observed prior practice: send exactly
				// the amount that arrived at the dead address, but never more
				// than the snapshot entitlement (a wallet can burn tokens
				// acquired after the snapshot).
				if row.BurnedWei.Cmp(row.SnapshotWei) > 0 {
					row.PayoutWei = new(big.Int).Set(row.SnapshotWei)
				} else {
					row.PayoutWei = new(big.Int).Set(row.BurnedWei)
				}
			case *payoutPolicy == "min" && row.GrossBurnWei.Cmp(row.SnapshotWei) < 0:
				row.PayoutWei = new(big.Int).Set(row.GrossBurnWei)
			default:
				row.PayoutWei = new(big.Int).Set(row.SnapshotWei)
			}
		}
		rows = append(rows, row)
		rowByEth[eth] = row
	}

	// Already-sent allocation, grouped by winning quantum address so that a
	// quantum address shared by several eth addresses is never double-counted.
	ethsByQuantum := make(map[string][]string)
	for eth, w := range winningByEth {
		ethsByQuantum[w.QuantumAddress.HexLower()] = append(ethsByQuantum[w.QuantumAddress.HexLower()], eth)
	}
	for quantum, eths := range ethsByQuantum {
		agg := sentByQuantum[quantum]
		var sent *big.Int
		var sentHashes []string
		if agg != nil {
			sent = agg.Total
			sentHashes = agg.TxHashes
		} else {
			sent = new(big.Int)
		}
		if len(eths) == 1 {
			row := rowByEth[eths[0]]
			row.AlreadySentWei = new(big.Int).Set(sent)
			row.SentTxHashes = sentHashes
			continue
		}
		sumPayout := new(big.Int)
		for _, eth := range eths {
			sumPayout = new(big.Int).Add(sumPayout, rowByEth[eth].PayoutWei)
		}
		switch {
		case sent.Sign() == 0:
			// nothing sent yet: everyone in the group is fully pending
		case sent.Cmp(sumPayout) >= 0:
			for _, eth := range eths {
				row := rowByEth[eth]
				row.AlreadySentWei = new(big.Int).Set(row.PayoutWei)
				row.SentTxHashes = sentHashes
				row.Flags = append(row.Flags, "SHARED_QUANTUM_ADDRESS")
			}
		default:
			// Partial coverage of a shared quantum address. Prior sends used the
			// exact burned amount, so allocate each sent transaction to the group
			// member whose burned amount matches it to the wei; never guess beyond
			// exact matches.
			used := make([]bool, len(agg.Amounts))
			consumed := new(big.Int)
			var unresolved []string
			for _, eth := range eths {
				row := rowByEth[eth]
				matched := false
				for i, amt := range agg.Amounts {
					if !used[i] && row.BurnedWei.Sign() > 0 && withinWei(amt, row.BurnedWei, dustWei) {
						used[i] = true
						row.AlreadySentWei = new(big.Int).Set(amt)
						row.SentTxHashes = []string{agg.TxHashes[i]}
						row.Flags = append(row.Flags, "SHARED_QUANTUM_ADDRESS")
						consumed = new(big.Int).Add(consumed, amt)
						matched = true
						break
					}
				}
				if !matched {
					unresolved = append(unresolved, eth)
				}
			}
			leftover := new(big.Int).Sub(sent, consumed)
			for _, eth := range unresolved {
				row := rowByEth[eth]
				row.SentTxHashes = sentHashes
				if leftover.Sign() == 0 {
					// every sent txn is accounted for: this member is fully pending
					row.Flags = append(row.Flags, "SHARED_QUANTUM_ADDRESS")
				} else {
					row.Flags = append(row.Flags, "SHARED_QUANTUM_ADDRESS", "NEEDS_MANUAL_REVIEW")
				}
			}
		}
	}
	for _, row := range rows {
		if row.hasFlag("NEEDS_MANUAL_REVIEW") {
			continue
		}
		pending := new(big.Int).Sub(row.PayoutWei, row.AlreadySentWei)
		if pending.Sign() < 0 {
			pending = new(big.Int)
		}
		row.PendingWei = pending
		if row.PayoutWei.Sign() > 0 {
			if row.PendingWei.Sign() == 0 {
				row.Flags = append(row.Flags, "ALREADY_SENT")
			} else if row.AlreadySentWei.Sign() > 0 && row.PendingWei.Cmp(dustWei) <= 0 {
				// The prior send tool truncated low-order decimals; a dust-level
				// remainder on an otherwise-paid conversion counts as settled.
				row.Flags = append(row.Flags, "ALREADY_SENT", "DUST_REMAINDER")
			} else if row.AlreadySentWei.Sign() > 0 {
				row.Flags = append(row.Flags, "PARTIALLY_SENT")
			}
		}
	}

	// Step 7: burn proofs (secondary cross-check).
	burnProofs, err := listTokenBurnProofs(conversionContractAddr)
	if err != nil {
		fmt.Println("warning: could not fetch burn proofs:", err)
		empty := []string{}
		burnProofs = &empty
	}
	lcProofs := make([]string, len(*burnProofs))
	for i, p := range *burnProofs {
		lcProofs[i] = strings.ToLower(p)
		err = os.WriteFile(filepath.Join(*outFolder, fmt.Sprintf("burnproof-%d.txt", i)), []byte(p), 0644)
		if err != nil {
			return err
		}
	}
	for _, row := range rows {
		if row.PendingWei.Sign() <= 0 {
			continue
		}
		for _, p := range lcProofs {
			if strings.Contains(p, row.EthAddressLC) || (row.QuantumAddress != "" && strings.Contains(p, row.QuantumAddress)) {
				row.Flags = append(row.Flags, "IN_BURN_PROOF")
				break
			}
		}
	}

	// Sort: pending desc, then eth address.
	sort.Slice(rows, func(i, j int) bool {
		c := rows[i].PendingWei.Cmp(rows[j].PendingWei)
		if c != 0 {
			return c > 0
		}
		return rows[i].EthAddressLC < rows[j].EthAddressLC
	})

	// Outputs.
	// 1) pending-sends.csv: the actionable list for manual sending.
	pendingFile, err := os.Create(filepath.Join(*outFolder, "pending-sends.csv"))
	if err != nil {
		return err
	}
	pw := csv.NewWriter(pendingFile)
	pw.Write([]string{"EthAddress", "QuantumAddress", "HeisenTokens", "HeisenWei", "BurnTxnHashes", "RequestTxnHash", "Flags"})
	pendingCount := 0
	totalPending := new(big.Int)
	for _, row := range rows {
		if row.PendingWei.Sign() <= 0 || row.hasFlag("NEEDS_MANUAL_REVIEW") || row.hasFlag("DUST_REMAINDER") {
			continue
		}
		pw.Write([]string{
			row.EthAddressLC,
			row.QuantumAddress,
			weiToTokenStr(row.PendingWei),
			row.PendingWei.String(),
			strings.Join(row.BurnTxHashes, ";"),
			row.Winning.TxnHash,
			strings.Join(row.Flags, "|"),
		})
		pendingCount++
		totalPending = new(big.Int).Add(totalPending, row.PendingWei)
	}
	pw.Flush()
	pendingFile.Close()
	if pw.Error() != nil {
		return pw.Error()
	}

	// 2) Full reconciliation report.
	reportFile, err := os.Create(filepath.Join(*outFolder, "reconciliation-report.csv"))
	if err != nil {
		return err
	}
	rw := csv.NewWriter(reportFile)
	rw.Write([]string{"EthAddress", "QuantumAddress", "RequestIndex", "RequestTxnHash", "RequestCount", "SignatureValid", "SigVariant",
		"SnapshotWei", "BurnedWei", "GrossedUpBurnWei", "BurnTxnCount", "BurnTxnHashes",
		"AlreadySentWei", "SentTxnHashes", "PayoutWei", "PendingWei", "PendingTokens", "Flags"})
	for _, row := range rows {
		reqIndex, reqTxn, sigValid, sigVariant := "", "", "false", ""
		if row.Winning != nil {
			reqIndex = fmt.Sprintf("%d", row.Winning.Index)
			reqTxn = row.Winning.TxnHash
			sigValid = "true"
			sigVariant = row.Winning.SigVariant
		}
		snap := "0"
		if row.SnapshotWei != nil {
			snap = row.SnapshotWei.String()
		}
		rw.Write([]string{
			row.EthAddressLC, row.QuantumAddress, reqIndex, reqTxn, fmt.Sprintf("%d", row.RequestCount), sigValid, sigVariant,
			snap, row.BurnedWei.String(), row.GrossBurnWei.String(), fmt.Sprintf("%d", len(row.BurnTxHashes)), strings.Join(row.BurnTxHashes, ";"),
			row.AlreadySentWei.String(), strings.Join(row.SentTxHashes, ";"), row.PayoutWei.String(), row.PendingWei.String(), weiToTokenStr(row.PendingWei),
			strings.Join(row.Flags, "|"),
		})
	}
	rw.Flush()
	reportFile.Close()
	if rw.Error() != nil {
		return rw.Error()
	}

	// 3) All requests with verification detail (audit artifact).
	reqFile, err := os.Create(filepath.Join(*outFolder, "requests-verified.csv"))
	if err != nil {
		return err
	}
	qw := csv.NewWriter(reqFile)
	qw.Write([]string{"Index", "QuantumAddress", "EthAddress", "TxnHash", "BlockNumber", "SignatureValid", "SigVariant", "SigError", "IsWinning"})
	for _, r := range requests {
		isWinning := "false"
		if w, ok := winningByEth[r.EthAddressLC]; ok && w.Index == r.Index {
			isWinning = "true"
		}
		qw.Write([]string{
			fmt.Sprintf("%d", r.Index), r.QuantumAddress.HexLower(), r.EthAddressLC, r.TxnHash, fmt.Sprintf("%d", r.BlockNumber),
			fmt.Sprintf("%v", r.SigValid), r.SigVariant, r.SigErr, isWinning,
		})
	}
	qw.Flush()
	reqFile.Close()
	if qw.Error() != nil {
		return qw.Error()
	}

	// 4) Holder transfers to addresses that match no winning request.
	winningQuantum := make(map[string]bool)
	for _, w := range winningByEth {
		winningQuantum[w.QuantumAddress.HexLower()] = true
	}
	unmatchedFile, err := os.Create(filepath.Join(*outFolder, "unmatched-transfers.csv"))
	if err != nil {
		return err
	}
	uw := csv.NewWriter(unmatchedFile)
	uw.Write([]string{"QuantumAddress", "TotalWei", "TotalTokens", "TxnHashes"})
	unmatchedCount := 0
	zeroAddressLC := common.Address{}.HexLower()
	feeBurnWei := new(big.Int)
	for quantum, agg := range sentByQuantum {
		if winningQuantum[quantum] {
			continue
		}
		if quantum == zeroAddressLC {
			// The Heisen token burns a 0.1% fee to address(0) on every
			// transfer; these events are fees, not conversion sends.
			feeBurnWei = new(big.Int).Set(agg.Total)
			continue
		}
		uw.Write([]string{quantum, agg.Total.String(), weiToTokenStr(agg.Total), strings.Join(agg.TxHashes, ";")})
		unmatchedCount++
	}
	uw.Flush()
	unmatchedFile.Close()
	if uw.Error() != nil {
		return uw.Error()
	}

	// 5) multitransfertokens-compatible payout csv (headerless: address,amount).
	// Exact 18-dp amounts; assert the payout tool's float path round-trips them.
	payoutFile, err := os.Create(filepath.Join(*outFolder, "payout.csv"))
	if err != nil {
		return err
	}
	yw := csv.NewWriter(payoutFile)
	for _, row := range rows {
		if row.PendingWei.Sign() <= 0 || row.hasFlag("NEEDS_MANUAL_REVIEW") || row.hasFlag("DUST_REMAINDER") {
			continue
		}
		amountStr := weiToTokenStr18(row.PendingWei)
		parsed, err := ParseBigFloat(amountStr)
		if err != nil {
			return fmt.Errorf("payout self-check parse failed for %s: %v", amountStr, err)
		}
		if etherToWeiFloat(parsed).Cmp(row.PendingWei) != 0 {
			return fmt.Errorf("payout self-check round-trip mismatch for %s (eth %s)", amountStr, row.EthAddressLC)
		}
		yw.Write([]string{row.QuantumAddress, amountStr})
	}
	yw.Flush()
	payoutFile.Close()
	if yw.Error() != nil {
		return yw.Error()
	}

	// 6) Summary.
	holderBalance, hbErr := client.GetAccountTokenBalance(heisenAddr, holderAddr)
	var sb strings.Builder
	sb.WriteString("Heisen conversion reconciliation summary\n")
	sb.WriteString(fmt.Sprintf("generated against chain head block %d, rpc %s\n\n", head, rawURL))
	sb.WriteString(fmt.Sprintf("conversion requests on-chain:      %d (signatures valid: %d)\n", len(requests), validCount))
	sb.WriteString(fmt.Sprintf("distinct eth addresses requesting: %d (with a valid winning request: %d)\n", len(requestsByEth), len(winningByEth)))
	sb.WriteString(fmt.Sprintf("snapshot entries (non-zero):       %d\n", len(snapshotWei)))
	sb.WriteString(fmt.Sprintf("burn transfers loaded:             %d from %d files (cross-file duplicates skipped: %d)\n", burnRows, len(burnFiles), dupRows))
	sb.WriteString(fmt.Sprintf("distinct burner eth addresses:     %d\n", len(burnsByEth)))
	sb.WriteString(fmt.Sprintf("holder outbound transfer recipients: %d, total sent: %s hei\n", len(sentByQuantum), weiToTokenStr(totalSent)))
	sb.WriteString(fmt.Sprintf("unmatched holder transfer recipients: %d (see unmatched-transfers.csv)\n", unmatchedCount))
	sb.WriteString(fmt.Sprintf("holder 0.1%% transfer-fee burns to address(0): %s hei\n", weiToTokenStr(feeBurnWei)))
	sb.WriteString(fmt.Sprintf("burn proofs on contract:           %d (dumped as burnproof-N.txt)\n\n", len(lcProofs)))
	sb.WriteString(fmt.Sprintf("PENDING SENDS: %d recipients, total %s hei (%s wei)\n", pendingCount, weiToTokenStr(totalPending), totalPending.String()))
	if hbErr == nil {
		sb.WriteString(fmt.Sprintf("holder current Heisen balance:     %s hei\n", weiToTokenStr(holderBalance)))
		if holderBalance.Cmp(totalPending) < 0 {
			sb.WriteString("WARNING: holder balance is LESS than total pending!\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("holder balance check failed: %v\n", hbErr))
	}
	flagCounts := make(map[string]int)
	for _, row := range rows {
		for _, f := range row.Flags {
			flagCounts[f]++
		}
	}
	flagNames := make([]string, 0, len(flagCounts))
	for f := range flagCounts {
		flagNames = append(flagNames, f)
	}
	sort.Strings(flagNames)
	sb.WriteString("\nflag counts:\n")
	for _, f := range flagNames {
		sb.WriteString(fmt.Sprintf("  %-24s %d\n", f, flagCounts[f]))
	}
	sb.WriteString("\nNOTE: burn data comes from the provided Etherscan export files. Compare the total burned\n")
	sb.WriteString("against the dead address's token balance on Etherscan to confirm the exports are complete.\n")
	err = os.WriteFile(filepath.Join(*outFolder, "summary.txt"), []byte(sb.String()), 0644)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Print(sb.String())
	fmt.Println("\noutput folder:", *outFolder)
	return nil
}
