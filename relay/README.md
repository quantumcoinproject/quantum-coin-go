## Quantum Coin Relay

Quantum Coin Relay provides a set of simple APIs to interact with the Quantum Coin blockchain.

## API Swagger

Read APIs:
https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/relay/read-api.yaml

Write APIs:
https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/relay/write-api.yaml

### Running A Relay

First, start Quantum Coin blockchain node. Then run relay using the following command:

```relay config.json```

An example relay configuration is given below. 

1) Copy the default genesis file at https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/consensus/proofofstake/genesis/genesis.json to the same folder as relay.
2) Do not change the value of maxSupply for mainnet.
3) Example configuration file is at: https://github.com/quantumcoinproject/quantum-coin-go/blob/dogep/cmd/relay/config.json
4) nodeUrl should point to a QuantumCoin node connected to the blockchain. The node should be started with the following options: ``` --http --http.addr "127.0.0.1" --http.port 8445 --syncmode full --gcmode=archive``` 
5) Do not expose relay directly over a network. If the relay APIs have to be accessed from another machine, then add a TLS layer such a Layer 7 load balancer in front of the relay.  
6) Once the relay is started, the APIs can be accessed following the definitions shared in the yaml files linked above.
7) The enableExtendedApis parameter can be used to control whether APIs such as GetBlockchainDetails, QueryDetails are enabled or not. If not enabled, the response returns a 404.

```
[
  {
    "api": "read",
    "ip": "127.0.0.1",
    "port": "9090",
    "nodeUrl": "http://127.0.0.1:8545",
    "corsAllowedOrigins": "*",
    "enableAuth": false,
    "apiKeys": "",
    "cachePath": "D://cachemanager//",
    "enableExtendedApis": "true",
    "genesisFilePath": "genesis.json",
    "maxSupply": "0x4EE2D6D415B85ACEF8100000000"
  },
  {
    "api": "write",
    "ip": "127.0.0.1",
    "port": "9091",
    "nodeUrl": "http://127.0.0.1:8545",
    "corsAllowedOrigins": "*",
    "enableAuth": false,
    "apiKeys": "",
    "cachePath": "D://cachemanager//",
    "enableExtendedApis": "false",
    "genesisFilePath": "genesis.json"
  }
]
```
