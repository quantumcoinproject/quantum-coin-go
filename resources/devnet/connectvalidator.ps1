param(
    # Optional. If set (e.g. .\connectvalidator.ps1 -RpcPort 8545), the node also
    # serves JSON-RPC over HTTP on http://127.0.0.1:<RpcPort>. By default only the
    # IPC named pipe \\.\pipe\geth.ipc is available.
    [int]$RpcPort = 0
)
$env:DC_ACC_ADDRESS="0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71"
$env:DP_ACC_PWD="QuantumCoinExample123!"
$env:MIN_VALIDATORS="1"
$env:Q_DEFAULT_CONFIG="1"
$env:SKIP_STARTUP_DELAY="1"
# Records the per-block consensus backup that proofofstake_getBlockExtendedDetails
# serves. Without it that API returns "GetConsensusInstance is nil" and block
# validator details are unavailable to explorers/indexers.
$env:BLOCK_EXTENDED_SAVE="1"
$rpcArgs = @()
if ($RpcPort -gt 0) {
    # --allow-insecure-unlock is required because the validator account is unlocked
    # while HTTP RPC is exposed. Safe here: devnet only, keys are publicly known.
    $rpcArgs = @("--http", "--http.port", "$RpcPort", "--http.api", "eth,net,web3,personal,proofofstake,txpool,tracer", "--allow-insecure-unlock")
}
.\dp --datadir data --networkid 123123 --syncmode full --gcmode full --freezermode skipappend --unlock $env:DC_ACC_ADDRESS --miner.etherbase $env:DC_ACC_ADDRESS --mine @rpcArgs
