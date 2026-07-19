QuantumCoin Devnet
===================
This package is a self-contained local development network (devnet). The blockchain data folder is pre-initialized and populated with coins and prefilled wallets, so you can start experimenting with transactions and smart contracts right away.

Network ID: 123123


Running the Devnet
===================
Windows (PowerShell):   .\connectvalidator.ps1
macOS / Ubuntu:         ./connectvalidator.sh

The node unlocks the validator wallet, resumes the pre-initialized chain and starts sealing new blocks.

Warmup time: the node takes about 120 seconds after startup before it begins producing blocks.
During this window the RPC endpoint may already answer queries (e.g. eth_blockNumber), but
transactions will not be mined yet. Wait until the block number has advanced at least once
before sending transactions.


RPC Endpoints
==============
By default the node serves JSON-RPC over IPC only (no TCP port is opened):

Windows:          named pipe  \\.\pipe\geth.ipc
macOS / Ubuntu:   unix socket data/geth.ipc (inside this folder)

To also serve JSON-RPC over HTTP on http://127.0.0.1:<port> (APIs: eth, net, web3, personal), pass a port to the connect script:

Windows (PowerShell):   .\connectvalidator.ps1 -RpcPort 8545
macOS / Ubuntu:         ./connectvalidator.sh 8545

Example HTTP request:
  curl -X POST http://127.0.0.1:8545 -H "Content-Type: application/json" -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}"

When a port is passed, the script also passes --allow-insecure-unlock (the node otherwise refuses to unlock the validator account while HTTP RPC is exposed). This is acceptable on a local devnet only; never do this on mainnet.

If no port is passed, the node starts with IPC only.


Troubleshooting the HTTP RPC endpoint
======================================
If a tool polls http://127.0.0.1:<port> and times out even though the node seems to be running:

1) Your connectvalidator script may be too old to accept a port. Older devnet packages ship a script
   that starts the node IPC-only no matter what arguments you pass; the port is silently ignored and
   nothing ever listens on HTTP. Check the script: the current Windows version starts with
   param([int]$RpcPort = 0). If yours does not, download the latest devnet package, or start the node
   manually with the HTTP flags appended:
     .\dp --datadir data --networkid 123123 --syncmode full --gcmode full --freezermode skipappend --unlock $env:DC_ACC_ADDRESS --mine --http --http.port 8545 --http.api eth,net,web3,personal --allow-insecure-unlock

2) Run the script in a foreground terminal first. Launching it detached or from a wrapper that
   redirects output can fail without leaving anything in the log file. In the foreground the node
   banner appears within a second or two; if you see nothing, the failure is in how the script is
   launched (execution policy, working directory), not in the node. On Windows use:
     powershell -NoProfile -ExecutionPolicy Bypass -File .\connectvalidator.ps1 -RpcPort 8545

3) The node logs to stderr, not stdout. When capturing output to a file, redirect both streams
   (*> node.log in PowerShell, > node.log 2>&1 in sh), otherwise the log file stays empty and the
   node looks hung.

4) Only one node can use the data folder. If a node is already running IPC-only, a second launch
   with an RPC port fails with a datadir lock error. Stop the running dp process first, then
   relaunch with the port.

5) Confirm readiness with a probe, not a delay. Remember the ~120 second warmup before block
   production: poll until the RPC answers AND the block number advances, rather than sleeping
   a fixed time:
     curl -s -X POST http://127.0.0.1:8545 -H "Content-Type: application/json" -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_blockNumber\",\"params\":[],\"id\":1}"
   A JSON response with a result field means the endpoint is up; connection refused means the node
   is not listening on that port (see points 1-4).


Prefilled Wallets
==================
The password for both wallet keystores is: QuantumCoinExample123!

1) Funded account (holds 7,989,494,831.336 coins from the genesis allocation; use this one to send coins):
   Address:  0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6
   Keystore: file named 1a846abe... in this folder.
   To send transactions from this account, copy its keystore file into data/keystore; the running node picks it up automatically within a few seconds.

2) Validator account (unlocked automatically at startup and used for block production):
   Address:  0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71
   Keystore: data/keystore/45dc0028...
   Its 10 trillion coins are locked as a genesis stake deposit in the staking contract, so eth.getBalance on this address returns 0.


Interacting with the Devnet
============================
Attach a console to the running node from a second terminal:

Windows:          .\dp.exe attach \\.\pipe\geth.ipc
macOS / Ubuntu:   ./dp attach data/geth.ipc

Example console commands:

  eth.blockNumber
  eth.getBalance("0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6")
  personal.sendTransaction({from: "0x1a846abe71c8b989e8337c55d608be81c28ab3b2e40c83eaa2a68d516049aec6", to: "0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71", value: web3.toWei(1, "ether")}, "QuantumCoinExample123!")


JavaScript Libraries
=====================
quantumcoin.js (https://github.com/quantumcoinproject/quantumcoin.js) can be used to interact with the blockchain; its API is closely compatible with ethers.js. Point it at the HTTP RPC endpoint (start the node with an RPC port as described above).

quantum-coin-js-sdk (https://github.com/quantumcoinproject/quantum-coin-js-sdk) can be used instead if only lower-level functionality is needed, such as wallet creation and transaction signing. Documentation: https://quantumcoin.org/sdk.html


Warning
========
This devnet is for local development and testing only. Devnet coins have no value. The prefilled wallet keys and passwords are publicly known; never reuse them, or send real funds to them, on mainnet or any public network.

This is blockchain software. Do not use a computer or device with sensitive or important personal data. Use at your own risk.

Goto https://quantumcoin.org for documentation on using this software.
