abigen
    https://geth.ethereum.org/docs/dapp/native-bindings

    $ abigen --abi stakingContract.abi --pkg main --type StakingContract --out StakingContract.go --bin stakingContract.bin
      abigen --abi stakingContract.abi --pkg main --type StakingContract --out StakingContract.go --bin stakingContract.bin

TestNet
    set DP_URL=http://172.31.34.126:8545
    set DP_ALLOC_ACCOUNT=46f8c16c50b122a568c96fb5e97e44ca9cd205ce
    set DP_ALLOC_ACCOUNT_PASSWORD=dummy
    set TOKENS_INFO=C:\t2build\tokens.json
    set DP_DATA_PATH=/data/
    set DP_ACCOUNT_PASSWORD=dummy

    set DP_URL=\\.\pipe\geth.ipc
    set DP_ALLOC_ACCOUNT=436c1035da1455a2b6490b51de4676ea28a34e32aa68417bd0189a059a244035
    set DP_ALLOC_ACCOUNT_PASSWORD=Test123$$
    set TOKENS_INFO=C:\t4-token\tokens.json
    set DP_DATA_PATH=/data/
    set DP_ACCOUNT_PASSWORD=Test123$$

Testnet Default file
    * tokens.json           -   set TOKENS_INFO=C:\t4-token\tokens.json
    * contract.json         -   create empty json on testnet.exe path folder
    * clientcontract.json   -   create empty json on testnet.exe path folder
    * clientcontract-1.json -   create empty json on testnet.exe path folder

Testnet Working functions
    1) startTestCoinByNewAccount
        * Generate new account (time delay 30 seconds)
        * Transfer coin from primary account  to new account

    2) startTestCoinAccountByAccount
        * Transfer coin account  to  account dynamically (time delay 9 seconds)

    3) startTestAccountByNewToken
        * Select account dynamic
        * Select token name dynamic
        * Create new token contract

    4) startTestNewTokenAccountByAccount
        * Token Send Quantity account by account

    5) startTestTokenByAccount
        * Get token contract
        * sent token from address - dynamic select to address
