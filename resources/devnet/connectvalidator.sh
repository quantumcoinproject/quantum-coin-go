export DC_ACC_ADDRESS="0x45dc00282f80628911a15775cd821a3747d2c62e14e54d1339f057c9e921be71"
export DP_ACC_PWD="QuantumCoinExample123!"
export MIN_VALIDATORS="1"
export Q_DEFAULT_CONFIG="1"
export SKIP_STARTUP_DELAY="1"
./dp --datadir data --networkid 123123 --syncmode full --gcmode full --freezermode skipappend --unlock $env:DC_ACC_ADDRESS --mine

