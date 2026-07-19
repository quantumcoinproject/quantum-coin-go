# QuantumCoin Devnet

Run a local, single-node QuantumCoin development network (devnet) on your own machine. The devnet package is fully self-contained: it comes **populated with coins and prefilled wallets**, so you can start experimenting with transactions and smart contracts right away — no setup, syncing, or faucet needed.

## Download

Go to the [releases page](https://github.com/quantumcoinproject/quantum-coin-go/releases) and, from the **latest release**, download the devnet package for your platform:

| Platform | File to download |
|----------|------------------|
| Windows | `windows-devnet.zip` |
| macOS | `mac-devnet.tar.gz` |
| Ubuntu | `ubuntu-devnet.tar.gz` |

## Running the Devnet

1. Extract the downloaded archive.

   - **Windows:** right-click `windows-devnet.zip` and select *Extract All*, or run:

     ```powershell
     Expand-Archive windows-devnet.zip
     ```

   - **macOS / Ubuntu:**

     ```bash
     tar -xzf mac-devnet.tar.gz      # macOS
     tar -xzf ubuntu-devnet.tar.gz   # Ubuntu
     ```

2. From the extracted `devnet` folder, run the connect script:

   - **Windows** (PowerShell):

     ```powershell
     .\connectvalidator.ps1
     ```

   - **macOS / Ubuntu:**

     ```bash
     ./connectvalidator.sh
     ```

That's it — the node unlocks the validator wallet, resumes the pre-initialized chain, and starts sealing new blocks within a few seconds. The devnet runs with network ID `123123`.

## RPC Endpoints

By default, the node serves JSON-RPC over **IPC only** — a named pipe on Windows and a unix domain socket on macOS/Ubuntu. No TCP port is opened by default.

| OS | Default RPC transport | Endpoint |
|----|----------------------|----------|
| Windows | Named pipe (IPC) | `\\.\pipe\geth.ipc` |
| macOS / Ubuntu | Unix domain socket (IPC) | `data/geth.ipc` (inside the `devnet` folder) |

### Optional: HTTP RPC port

To also serve JSON-RPC over HTTP (useful for tools that cannot use IPC, such as web dashboards, MetaMask-style wallets, or remote scripts), pass a port to the connect script. The node then listens on `http://127.0.0.1:<port>` with the `eth`, `net`, `web3`, and `personal` APIs enabled. In this mode the script also passes `--allow-insecure-unlock`, because the node refuses to unlock the validator account while HTTP RPC is exposed otherwise — this is acceptable on a local devnet where the keys are publicly known anyway, but is exactly why you should never do this on mainnet.

- **Windows** (PowerShell):

  ```powershell
  .\connectvalidator.ps1 -RpcPort 8545
  ```

- **macOS / Ubuntu:**

  ```bash
  ./connectvalidator.sh 8545
  ```

Example request against the HTTP endpoint:

```bash
curl -X POST http://127.0.0.1:8545 -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

If no port is passed, the scripts start the node with IPC only, exactly as before.

## Prefilled Wallets

The package ships with two wallets, prefilled in the genesis block (verified against the devnet genesis and a running node). The password for both keystores is:

```text
QuantumCoinExample123!
```

### Funded account (use this one to send coins)

```text
0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6
```

- **Spendable balance:** 7,989,494,831.336 coins (`eth.getBalance` returns `7.989494831336e+27` wei)
- **Keystore:** file named `1a846abe...` in the package root folder

### Validator account (produces blocks)

```text
0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71
```

- **Spendable balance:** 0 — its 10,000,000,000,000 (10 trillion) coins are locked as a genesis stake deposit in the staking contract, so `eth.getBalance` on this address returns `0`
- **Keystore:** `data/keystore/45dc0028...`; unlocked automatically at startup by the connect script and used for block production

To send transactions from the funded account, copy its keystore file into the `data/keystore` folder (the running node picks it up automatically within a few seconds):

```powershell
# Windows
copy 1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6 data\keystore\
```

```bash
# macOS / Ubuntu
cp 1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6 data/keystore/
```

## Interacting with the Devnet

Attach a console to the running node (from the `devnet` folder, in a second terminal), using the default IPC endpoint for your OS:

```powershell
# Windows
.\dp.exe attach \\.\pipe\geth.ipc
```

```bash
# macOS / Ubuntu
./dp attach data/geth.ipc
```

If you started the node with an HTTP RPC port, you can also attach over HTTP:

```bash
./dp attach http://127.0.0.1:8545
```

Example console commands:

```javascript
eth.blockNumber
eth.getBalance("0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6")
personal.sendTransaction({from: "0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6", to: "0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71", value: web3.toWei(1, "ether")}, "QuantumCoinExample123!")
```

## What's Inside

- **Prefilled wallets** populated with coins in the genesis block (see above).
- **Pre-initialized blockchain data**: the `data` folder already contains an initialized chain, so the node starts producing blocks right away.
- **Node binaries**: the `dp` node client, the `dputil` utility, and the `relay`.
- **Genesis and configuration files** for the devnet network.

## Warning

- The devnet is for **local development and testing only**. Coins on the devnet have no value.
- The prefilled wallet keys and passwords are **publicly known**. Never reuse them, or send real funds to them, on mainnet or any public network.
- This is blockchain software. Do not use a computer or device with sensitive or important personal data. Use at your own risk.

## More Information

- Main repository: [quantum-coin-go](https://github.com/quantumcoinproject/quantum-coin-go)
- Documentation: [https://quantumcoin.org](https://quantumcoin.org)
- Community: [Discord](https://discord.gg/bbbMPyzJTM)
