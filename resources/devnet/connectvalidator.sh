#!/bin/sh
# Optional first argument: HTTP RPC port. If set (e.g. ./connectvalidator.sh 8545),
# the node also serves JSON-RPC over HTTP on http://127.0.0.1:<port>. By default
# only the IPC unix socket data/geth.ipc is available.
export DC_ACC_ADDRESS="0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71"
export DP_ACC_PWD="QuantumCoinExample123!"
export MIN_VALIDATORS="1"
export Q_DEFAULT_CONFIG="1"
export SKIP_STARTUP_DELAY="1"
# Records the per-block consensus backup that proofofstake_getBlockExtendedDetails
# serves. Without it that API returns "GetConsensusInstance is nil" and block
# validator details are unavailable to explorers/indexers.
export BLOCK_EXTENDED_SAVE="1"
RPC_ARGS=""
if [ -n "$1" ]; then
  # --allow-insecure-unlock is required because the validator account is unlocked
  # while HTTP RPC is exposed. Safe here: devnet only, keys are publicly known.
  RPC_ARGS="--http --http.port $1 --http.api eth,net,web3,personal,proofofstake,txpool,tracer --allow-insecure-unlock"
fi
./dp --datadir data --networkid 123123 --syncmode full --gcmode full --freezermode skipappend --unlock $DC_ACC_ADDRESS --miner.etherbase $DC_ACC_ADDRESS --mine $RPC_ARGS
