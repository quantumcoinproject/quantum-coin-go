package defaults

const DeepCheckStartBlock = uint64(3000000)
const GasPriceStartBlock = uint64(3000001)
const DefaultGasLimit = 300000000

var DEFAULT_PRICE = int64(47619047619047600)

func GetGasLimit(blockNumber uint64) uint64 {
	if blockNumber < GasPriceStartBlock {
		return DefaultGasLimit
	} else {
		return DefaultGasLimit
	}
}
