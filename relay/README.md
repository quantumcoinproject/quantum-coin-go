## Quantum Coin Relay

Quantum Coin Relay provides a set of simple APIs to interact with the Quantum Coin blockchain. The advantage of using a relay as opposed to using the node APIs directly are:

1) Performance and Security Isolation. 
2) Scale the Relay linearly for Read APIs, since it caches information and doesn't have to query the blockchain node for every request.
3) Suitable for Exchanges, Block Explorers and client side applications such as Wallet Apps; data is aggregated and summarized.

The Relay APIs can be programmatically accessed using SDK at https://github.com/quantumcoinproject/quantum-coin-js-sdk

## API Swagger

Read APIs:
https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/relay/read-api.yaml

Write APIs:
https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/relay/write-api.yaml

## Running A Relay

1) First, start Quantum Coin blockchain node. See steps to start the node: https://quantumcoin.org/connecting-to-mainnet-snapshot.html
2) Open Terminal (Linux) or Command Prompt (Windows) and switch to the folder where the relay and other files are located.
3) Run the following command to start the relay:

#### Windows:

```relay.exe config.json```

#### Linux:

Replace $HOME/dp with the directory path where the relay file is located.

```
export LD_LIBRARY_PATH=$HOME/dp
./relay ./config.json
```

An example relay configuration is given below. 

1) Copy the default genesis file at https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/consensus/proofofstake/genesis/genesis.json to the same folder as relay.
2) Do not change the value of `maxSupply` for mainnet.
3) Example configuration file is at: https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/cmd/relay/config.json
4) `nodeUrl` should point to a QuantumCoin node connected to the blockchain. The node should be started with the following options: ``` --http --http.addr "127.0.0.1" --http.port 8445 --syncmode full --gcmode=archive```
5) Alternatively, if the relay is running in the same machine as the QuantumCoin node, you may also specify the IPC endpoint for `nodeUrl` instead. Windows: ```\\.\pipe\geth.ipc``` Linux: ```.\data\geth.ipc```
5) Do not expose relay directly over a network. If the relay APIs have to be accessed from another machine, then add a TLS layer such a Layer 7 load balancer in front of the relay.  
6) Once the relay is started, the APIs can be accessed following the definitions shared in the yaml files linked above.
7) The `enableExtendedApis` parameter can be used to control whether APIs such as GetBlockchainDetails, QueryDetails are enabled or not. If not enabled, the response returns a 404.

#### Example Linux Configuration
```
[
  {
    "api": "read",
    "ip": "127.0.0.1",
    "port": "9090",
    "nodeUrl": "./data/geth.ipc",
    "corsAllowedOrigins": "*",
    "enableAuth": false,
    "apiKeys": "",
    "cachePath": ".",
    "enableExtendedApis": false,
    "genesisFilePath": "genesis.json",
    "maxSupply": "0x4EE2D6D415B85ACEF8100000000"
  },
  {
    "api": "write",
    "ip": "127.0.0.1",
    "port": "9091",
    "nodeUrl": "./data/geth.ipc",
    "corsAllowedOrigins": "*",
    "enableAuth": false,
    "apiKeys": "",
    "cachePath": ".",
    "enableExtendedApis": false,
    "genesisFilePath": "genesis.json"
  }
]
```

#### Example Windows Configuration

```
[
  {
    "api": "read",
    "ip": "127.0.0.1",
    "port": "9090",
    "nodeUrl": "//./pipe/geth.ipc",
    "corsAllowedOrigins": "*",
    "enableAuth": false,
    "apiKeys": "",
    "cachePath": ".",
    "enableExtendedApis": false,
    "genesisFilePath": "genesis.json",
    "maxSupply": "0x4EE2D6D415B85ACEF8100000000"
  },
  {
    "api": "write",
    "ip": "127.0.0.1",
    "port": "9091",
    "nodeUrl": "//./pipe/geth.ipc",
    "corsAllowedOrigins": "*",
    "enableAuth": false,
    "apiKeys": "",
    "cachePath": ".",
    "enableExtendedApis": false,
    "genesisFilePath": "genesis.json"
  }
]
```
