package defaults

import "errors"

const DeepCheckStartBlock = uint64(3000000)
const GasPriceStartBlock = uint64(3000001)
const DefaultGasLimit = 300000000

var DEFAULT_PRICE = int64(47619047619047600)
var cryptoBreakglassBlock uint64 = 0
var signingMode byte = 1 //crypto.DILITHIUM_ED25519_SPHINCS_COMPACT_ID)

func GetGasLimit(blockNumber uint64) uint64 {
	if blockNumber < GasPriceStartBlock {
		return DefaultGasLimit
	} else {
		return DefaultGasLimit
	}
}

func SetCryptoBreakGlassBlock(blockNumber uint64) error {
	if cryptoBreakglassBlock > 0 && blockNumber != 0 {
		return errors.New("SetCryptoBreakGlassBlock already set")
	}
	cryptoBreakglassBlock = blockNumber
	return nil
}

func IsCryptoBreakglassMode(blockNumber uint64) bool {
	return cryptoBreakglassBlock != 0 && blockNumber >= cryptoBreakglassBlock
}

func SetCryptoSigningMode(signMode byte) {
	signingMode = signMode
}

func GetSigningMode() byte {
	return signingMode
}
