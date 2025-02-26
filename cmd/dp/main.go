// Copyright 2014 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

// geth is the official command-line client for Ethereum.
package main

import (
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/accounts"
	"github.com/QuantumCoinProject/qc/accounts/keystore"
	"github.com/QuantumCoinProject/qc/cmd/dp/profiling"
	"github.com/QuantumCoinProject/qc/cmd/utils"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/console/prompt"
	"github.com/QuantumCoinProject/qc/eth"
	"github.com/QuantumCoinProject/qc/eth/downloader"
	"github.com/QuantumCoinProject/qc/ethclient"
	"github.com/QuantumCoinProject/qc/internal/debug"
	"github.com/QuantumCoinProject/qc/internal/ethapi"
	"github.com/QuantumCoinProject/qc/internal/flags"
	"github.com/QuantumCoinProject/qc/log"
	"github.com/QuantumCoinProject/qc/metrics"
	"github.com/QuantumCoinProject/qc/node"
	"github.com/bt51/ntpclient"
	"gopkg.in/urfave/cli.v1"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	clientIdentifier = "geth" // Client identifier to advertise over the network
)

const VERSION_NUMBER = "v2.0.62"

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit = ""
	gitDate   = ""
	// The app that holds all commands and flags.
	app = flags.NewApp(gitCommit, gitDate, "the go-ethereum command line interface")
	// flags that configure the node
	nodeFlags = []cli.Flag{
		utils.IdentityFlag,
		utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.DataDirFlag,
		utils.AncientFlag,
		utils.MinFreeDiskSpaceFlag,
		utils.KeyStoreDirFlag,
		utils.ExternalSignerFlag,
		utils.NoUSBFlag,
		utils.USBFlag,
		utils.SmartCardDaemonPathFlag,
		utils.OverrideLondonFlag,
		utils.TxPoolLocalsFlag,
		utils.TxPoolNoLocalsFlag,
		utils.TxPoolJournalFlag,
		utils.TxPoolRejournalFlag,
		utils.TxPoolPriceLimitFlag,
		utils.TxPoolPriceBumpFlag,
		utils.TxPoolAccountSlotsFlag,
		utils.TxPoolGlobalSlotsFlag,
		utils.TxPoolAccountQueueFlag,
		utils.TxPoolGlobalQueueFlag,
		utils.TxPoolLifetimeFlag,
		utils.SyncModeFlag,
		utils.ExitWhenSyncedFlag,
		utils.GCModeFlag,
		utils.SnapshotFlag,
		utils.TxLookupLimitFlag,
		utils.LightServeFlag,
		utils.LightIngressFlag,
		utils.LightEgressFlag,
		utils.LightMaxPeersFlag,
		utils.LightNoPruneFlag,
		utils.LightKDFFlag,
		utils.UltraLightServersFlag,
		utils.UltraLightFractionFlag,
		utils.UltraLightOnlyAnnounceFlag,
		utils.LightNoSyncServeFlag,
		utils.WhitelistFlag,
		utils.BloomFilterSizeFlag,
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheTrieFlag,
		utils.CacheTrieJournalFlag,
		utils.CacheTrieRejournalFlag,
		utils.CacheGCFlag,
		utils.CacheSnapshotFlag,
		utils.CacheNoPrefetchFlag,
		utils.CachePreimagesFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.MiningEnabledFlag,
		utils.MinerThreadsFlag,
		utils.MinerNotifyFlag,
		utils.MinerGasTargetFlag,
		utils.MinerGasLimitFlag,
		utils.MinerGasPriceFlag,
		utils.MinerEtherbaseFlag,
		utils.MinerExtraDataFlag,
		utils.MinerRecommitIntervalFlag,
		utils.MinerNoVerfiyFlag,
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyPasswordFlag,
		utils.NodeKeyHexFlag,
		utils.DNSDiscoveryFlag,
		utils.MainnetFlag,
		utils.DeveloperFlag,
		utils.DeveloperPeriodFlag,
		utils.RopstenFlag,
		utils.RinkebyFlag,
		utils.GoerliFlag,
		utils.CalaverasFlag,
		utils.VMEnableDebugFlag,
		utils.NetworkIdFlag,
		utils.EthStatsURLFlag,
		utils.FakePoWFlag,
		utils.NoCompactionFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.GpoMaxGasPriceFlag,
		utils.GpoIgnoreGasPriceFlag,
		utils.MinerNotifyFullFlag,
		configFileFlag,
		utils.CatalystFlag,
		utils.EnableBackupsFlag,
		utils.RebroadcastCountFlag,
		utils.ProfPortFlag,
		utils.FreezerModeFlag,
	}

	rpcFlags = []cli.Flag{
		utils.HTTPEnabledFlag,
		utils.HTTPListenAddrFlag,
		utils.HTTPPortFlag,
		utils.HTTPCORSDomainFlag,
		utils.HTTPVirtualHostsFlag,
		utils.LegacyRPCEnabledFlag,
		utils.LegacyRPCListenAddrFlag,
		utils.LegacyRPCPortFlag,
		utils.LegacyRPCCORSDomainFlag,
		utils.LegacyRPCVirtualHostsFlag,
		utils.LegacyRPCApiFlag,
		utils.GraphQLEnabledFlag,
		utils.GraphQLCORSDomainFlag,
		utils.GraphQLVirtualHostsFlag,
		utils.HTTPApiFlag,
		utils.HTTPPathPrefixFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.WSPathPrefixFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.InsecureUnlockAllowedFlag,
		utils.RPCGlobalGasCapFlag,
		utils.RPCGlobalTxFeeCapFlag,
		utils.AllowUnprotectedTxs,
	}

	metricsFlags = []cli.Flag{
		utils.MetricsEnabledFlag,
		utils.MetricsEnabledExpensiveFlag,
		utils.MetricsHTTPFlag,
		utils.MetricsPortFlag,
		utils.MetricsEnableInfluxDBFlag,
		utils.MetricsInfluxDBEndpointFlag,
		utils.MetricsInfluxDBDatabaseFlag,
		utils.MetricsInfluxDBUsernameFlag,
		utils.MetricsInfluxDBPasswordFlag,
		utils.MetricsInfluxDBTagsFlag,
	}
)

func init() {
	// Initialize the CLI app and start Geth
	app.Action = geth
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2013-2021 The go-ethereum Authors"
	app.Commands = []cli.Command{
		// See chaincmd.go:
		initCommand,
		importCommand,
		exportCommand,
		importPreimagesCommand,
		exportPreimagesCommand,
		removedbCommand,
		dumpCommand,
		dumpGenesisCommand,
		// See accountcmd.go:
		accountCommand,
		walletCommand,
		// See consolecmd.go:
		consoleCommand,
		attachCommand,
		javascriptCommand,
		// See misccmd.go:
		makedagCommand,
		versionCommand,
		versionCheckCommand,
		licenseCommand,
		// See config.go
		dumpConfigCommand,
		// see dbcmd.go
		dbCommand,
		// See cmd/utils/flags_legacy.go
		utils.ShowDeprecated,
		// See snapshot.go
		snapshotCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = append(app.Flags, nodeFlags...)
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, consoleFlags...)
	app.Flags = append(app.Flags, debug.Flags...)
	app.Flags = append(app.Flags, metricsFlags...)

	app.Before = func(ctx *cli.Context) error {
		return debug.Setup(ctx)
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		prompt.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

func main() {
	log.Info("=======================================")
	log.Info("||        Quantum Coin               ||")
	log.Info("||              Q                    ||")
	log.Info("||  Quantum Resistant Blockchain     ||")
	log.Info("=======================================")
	fmt.Println("VERSION_NUMBER", VERSION_NUMBER)

	if len(os.Args) < 2 {
		os.Exit(-1)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prepare manipulates memory cache allowance and setups metric system.
// This function should be called before launching devp2p stack.
func prepare(ctx *cli.Context) {
	// If we're running a known preset, log it for convenience.
	switch {
	case ctx.GlobalIsSet(utils.RopstenFlag.Name):
		log.Info("Starting Geth on Ropsten testnet...")

	case ctx.GlobalIsSet(utils.RinkebyFlag.Name):
		log.Info("Starting Geth on Rinkeby testnet...")

	case ctx.GlobalIsSet(utils.GoerliFlag.Name):
		log.Info("Starting Geth on Görli testnet...")

	case ctx.GlobalIsSet(utils.CalaverasFlag.Name):
		log.Info("Starting Geth on Calaveras testnet...")

	case ctx.GlobalIsSet(utils.DeveloperFlag.Name):
		log.Info("Starting Geth in ephemeral dev mode...")

	case !ctx.GlobalIsSet(utils.NetworkIdFlag.Name):
		log.Info("Starting Geth on Ethereum mainnet...")
	}
	// If we're a full node on mainnet without --cache specified, bump default cache allowance
	if ctx.GlobalString(utils.SyncModeFlag.Name) != "light" && !ctx.GlobalIsSet(utils.CacheFlag.Name) && !ctx.GlobalIsSet(utils.NetworkIdFlag.Name) {
		// Make sure we're not on any supported preconfigured testnet either
		if !ctx.GlobalIsSet(utils.RopstenFlag.Name) && !ctx.GlobalIsSet(utils.RinkebyFlag.Name) && !ctx.GlobalIsSet(utils.GoerliFlag.Name) && !ctx.GlobalIsSet(utils.DeveloperFlag.Name) {
			// Nope, we're really on mainnet. Bump that cache up!
			log.Info("Bumping default cache on mainnet", "provided", ctx.GlobalInt(utils.CacheFlag.Name), "updated", 4096)
			ctx.GlobalSet(utils.CacheFlag.Name, strconv.Itoa(4096))
		}
	}
	// If we're running a light client on any network, drop the cache to some meaningfully low amount
	if ctx.GlobalString(utils.SyncModeFlag.Name) == "light" && !ctx.GlobalIsSet(utils.CacheFlag.Name) {
		log.Info("Dropping default light client cache", "provided", ctx.GlobalInt(utils.CacheFlag.Name), "updated", 128)
		ctx.GlobalSet(utils.CacheFlag.Name, strconv.Itoa(128))
	}

	// Start metrics export if enabled
	utils.SetupMetrics(ctx)

	// Start system runtime metrics collection
	go metrics.CollectProcessMetrics(3 * time.Second)
}

// geth is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func geth(ctx *cli.Context) error {
	if args := ctx.Args(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}

	checkTimeSync()

	prepare(ctx)

	//Ask for passphrase rightaway if unlock account was specified
	passphrase := ""
	inputs := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	if len(inputs) > 0 && len(inputs[0]) > 0 {
		passphrase = os.Getenv("DP_ACC_PWD")
		if len(passphrase) == 0 {
			passphrase = utils.GetPassPhrase("Enter the passphrase for decrypting the wallet:", false)
		}

		if len(passphrase) == 0 {
			utils.Fatalf("Cannot unlock account without password")
		}
	}

	stack, backend := makeFullNode(ctx)
	defer stack.Close()

	startNode(ctx, stack, backend, passphrase)
	stack.Wait()
	return nil
}

// startNode boots up the system node and all registered protocols, after which
// it unlocks any requested accounts, and starts the RPC/IPC interfaces and the
// miner.
func startNode(ctx *cli.Context, stack *node.Node, backend ethapi.Backend, passphrase string) {
	if ctx.IsSet(utils.ProfPortFlag.Name) {
		profPort := ctx.GlobalInt(utils.ProfPortFlag.Name)
		go profiling.StartProfiling(profPort)
	}

	// Unlock any account specifically requested
	_, err := unlockAccounts(ctx, stack, passphrase)
	if err != nil {
		utils.Fatalf("Could not unlock the account: %v", err)
	}

	// Start up the node itself
	utils.StartNode(ctx, stack)

	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	// Create a client to interact with local geth node.
	rpcClient, err := stack.Attach()
	if err != nil {
		utils.Fatalf("Failed to attach to self: %v", err)
	}
	ethClient := ethclient.NewClient(rpcClient)

	go func() {
		// Open any wallets already attached
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				log.Warn("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		// Listen for wallet event till termination
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					log.Warn("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				log.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)

				var derivationPaths []accounts.DerivationPath
				if event.Wallet.URL().Scheme == "ledger" {
					derivationPaths = append(derivationPaths, accounts.LegacyLedgerBaseDerivationPath)
				}
				derivationPaths = append(derivationPaths, accounts.DefaultBaseDerivationPath)

				event.Wallet.SelfDerive(derivationPaths, ethClient)

			case accounts.WalletDropped:
				log.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()

	// Spawn a standalone goroutine for status synchronization monitoring,
	// close the node when synchronization is complete if user required.
	if ctx.GlobalBool(utils.ExitWhenSyncedFlag.Name) {
		go func() {
			sub := stack.EventMux().Subscribe(downloader.DoneEvent{})
			defer sub.Unsubscribe()
			for {
				event := <-sub.Chan()
				if event == nil {
					continue
				}
				done, ok := event.Data.(downloader.DoneEvent)
				if !ok {
					continue
				}
				if timestamp := time.Unix(int64(done.Latest.Time), 0); time.Since(timestamp) < 10*time.Minute {
					log.Info("Synchronisation completed", "latestnum", done.Latest.Number, "latesthash", done.Latest.Hash(),
						"age", common.PrettyAge(timestamp))
					stack.Close()
				}
			}
		}()
	}

	// Start auxiliary services if enabled
	if ctx.GlobalBool(utils.MiningEnabledFlag.Name) || ctx.GlobalBool(utils.DeveloperFlag.Name) {
		// Mining only makes sense if a full Ethereum node is running
		if ctx.GlobalString(utils.SyncModeFlag.Name) == "light" {
			utils.Fatalf("Light clients do not support mining")
		}
		ethBackend, ok := backend.(*eth.EthAPIBackend)
		if !ok {
			utils.Fatalf("Ethereum service not running: %v", err)
		}
		// Set the gas price to the limits from the CLI and start mining
		gasprice := utils.GlobalBig(ctx, utils.MinerGasPriceFlag.Name)
		ethBackend.TxPool().SetGasPrice(gasprice)
		// start mining
		threads := ctx.GlobalInt(utils.MinerThreadsFlag.Name)
		if err := ethBackend.StartMining(threads); err != nil {
			utils.Fatalf("Failed to start mining: %v", err)
		}
	}
}

// unlockAccounts unlocks any account specifically requested.
func unlockAccounts(ctx *cli.Context, stack *node.Node, passphrase string) (accounts.Account, error) {
	var unlocks []string
	inputs := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	for _, input := range inputs {
		if trimmed := strings.TrimSpace(input); trimmed != "" {
			unlocks = append(unlocks, trimmed)
		}
	}

	// Short circuit if there is no account to unlock.
	if len(unlocks) == 0 {
		return accounts.Account{}, nil
	}

	if len(unlocks) > 1 {
		return accounts.Account{}, errors.New("only one account unlock is supported")
	}

	// If insecure account unlocking is not allowed if node's APIs are exposed to external.
	// Print warning log to user and skip unlocking.
	if !stack.Config().InsecureUnlockAllowed && stack.Config().ExtRPCEnabled() {
		utils.Fatalf("Account unlock with HTTP access is forbidden!")
	}
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)

	if len(passphrase) == 0 {
		passwords := utils.MakePasswordList(ctx)
		for i, account := range unlocks {
			account, _, err := unlockAccount(ks, account, i, passwords)

			return account, err
		}
	} else {
		log.Info("Password passed")
		for i, account := range unlocks {
			account, _, err := unlockAccount(ks, account, i, []string{passphrase})
			return account, err
		}
	}

	return accounts.Account{}, errors.New("no account exists to unlock")
}

func checkTimeSync() {
	skipTimeCheck := os.Getenv("SKIP_TIME_CHECK")
	if len(skipTimeCheck) > 0 && skipTimeCheck == "1" {
		return
	}

	log.Info("Checking time. Please wait....")
	ntpTime, err := ntpclient.GetNetworkTime("pool.ntp.org", 123)
	if err != nil {
		log.Warn("Error trying to check time", "err", err)
		return
	}

	const ALLOWED_TIME_SKEW_MINUTES = 2

	ntpTimeUtc := ntpTime.UTC()
	currTimeUtc := time.Now().UTC()

	if currTimeUtc.Before(ntpTimeUtc) {
		difference := ntpTimeUtc.Sub(currTimeUtc)
		if difference.Minutes() > ALLOWED_TIME_SKEW_MINUTES {
			log.Error("There is clock skew in your device. This can affect functionality. Please set the correct time.", "ntp time", ntpTime.Format(time.UnixDate), "device time", currTimeUtc.Format(time.UnixDate))
			log.Info("To disable this check, set the environment variable SKIP_TIME_CHECK to 1")
			os.Exit(-1)
		}
	} else if currTimeUtc.After(ntpTimeUtc) {
		difference := currTimeUtc.Sub(ntpTimeUtc)
		if difference.Minutes() > ALLOWED_TIME_SKEW_MINUTES {
			log.Error("There is clock skew in your device. This can affect functionality. Please set the correct time.", "ntp time", ntpTime.Format(time.UnixDate), "device time", currTimeUtc.Format(time.UnixDate))
			log.Info("To disable this time check (not recommended), set the environment variable SKIP_TIME_CHECK to 1")
			os.Exit(-1)
		}
	}

	log.Info("Completed time check.")
}
