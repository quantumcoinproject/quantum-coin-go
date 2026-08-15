package defaults

import (
	"testing"
)

// TestGetMaxGasLimitGasV3 verifies the GasV3StartBlock switch of the normal gas-limit
// ceiling: DefaultGasLimit below the fork, DefaultGasLimitV2 from the fork onward.
func TestGetMaxGasLimitGasV3(t *testing.T) {
	forkBlock := DefaultConfig.PosConfig.GasV3StartBlock

	if got := GetMaxGasLimit(forkBlock - 1); got != DefaultConfig.DefaultGasLimit {
		t.Fatalf("pre-fork max gas limit: got %d, want %d", got, DefaultConfig.DefaultGasLimit)
	}
	if got := GetMaxGasLimit(forkBlock); got != DefaultConfig.DefaultGasLimitV2 {
		t.Fatalf("at-fork max gas limit: got %d, want %d", got, DefaultConfig.DefaultGasLimitV2)
	}
	if got := GetMaxGasLimit(forkBlock + 1); got != DefaultConfig.DefaultGasLimitV2 {
		t.Fatalf("post-fork max gas limit: got %d, want %d", got, DefaultConfig.DefaultGasLimitV2)
	}
}

// TestGetMaxGasLimitGasV3Breakglass verifies the GasV3StartBlock switch of the
// breakglass ceiling: BreakglassDefaultGasLimit below the fork,
// BreakglassDefaultGasLimitV2 from the fork onward.
func TestGetMaxGasLimitGasV3Breakglass(t *testing.T) {
	forkBlock := DefaultConfig.PosConfig.GasV3StartBlock

	if err := SetCryptoBreakGlassBlock(1); err != nil {
		t.Fatalf("SetCryptoBreakGlassBlock: %v", err)
	}
	t.Cleanup(func() {
		if err := SetCryptoBreakGlassBlock(0); err != nil {
			t.Errorf("reset SetCryptoBreakGlassBlock: %v", err)
		}
	})

	if got := GetMaxGasLimit(forkBlock - 1); got != DefaultConfig.BreakglassDefaultGasLimit {
		t.Fatalf("pre-fork breakglass max gas limit: got %d, want %d", got, DefaultConfig.BreakglassDefaultGasLimit)
	}
	if got := GetMaxGasLimit(forkBlock); got != DefaultConfig.BreakglassDefaultGasLimitV2 {
		t.Fatalf("at-fork breakglass max gas limit: got %d, want %d", got, DefaultConfig.BreakglassDefaultGasLimitV2)
	}
	if got := GetMaxGasLimit(forkBlock + 1); got != DefaultConfig.BreakglassDefaultGasLimitV2 {
		t.Fatalf("post-fork breakglass max gas limit: got %d, want %d", got, DefaultConfig.BreakglassDefaultGasLimitV2)
	}
}

// TestGetMaxTransactionsForBlockGasV3 verifies the max-transactions-per-block bound
// drops proportionally with the gas-limit ceiling at GasV3StartBlock.
func TestGetMaxTransactionsForBlockGasV3(t *testing.T) {
	forkBlock := DefaultConfig.PosConfig.GasV3StartBlock

	preWant := int(DefaultConfig.DefaultGasLimit / BASIC_TXN_GAS)
	postWant := int(DefaultConfig.DefaultGasLimitV2 / BASIC_TXN_GAS)

	if got := GetMaxTransactionsForBlock(forkBlock - 1); got != preWant {
		t.Fatalf("pre-fork max transactions: got %d, want %d", got, preWant)
	}
	if got := GetMaxTransactionsForBlock(forkBlock); got != postWant {
		t.Fatalf("at-fork max transactions: got %d, want %d", got, postWant)
	}
	if got := GetMaxTransactionsForBlock(forkBlock + 1); got != postWant {
		t.Fatalf("post-fork max transactions: got %d, want %d", got, postWant)
	}
}

// TestGasV3Schedule freezes the GasV3 activation heights and ceiling values on both
// networks, and asserts the ordering invariants the dynamic gas-limit scheme
// (ComputeBlockGasLimit) relies on.
func TestGasV3Schedule(t *testing.T) {
	testCases := []struct {
		name           string
		config         *Config
		wantStartBlock uint64
	}{
		{"mainnet", MainnetConfig, 5319280},
		{"devnet", DevnetConfig, 82},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pos := tc.config.PosConfig
			if pos.GasV3StartBlock != tc.wantStartBlock {
				t.Errorf("GasV3StartBlock: got %d, want %d", pos.GasV3StartBlock, tc.wantStartBlock)
			}
			if pos.GasV3StartBlock <= pos.BlockTimeBindingV1StartBlock {
				t.Errorf("GasV3StartBlock (%d) must be after BlockTimeBindingV1StartBlock (%d)",
					pos.GasV3StartBlock, pos.BlockTimeBindingV1StartBlock)
			}
			if tc.config.DefaultGasLimitV2 != 45000000 {
				t.Errorf("DefaultGasLimitV2: got %d, want 45000000", tc.config.DefaultGasLimitV2)
			}
			if tc.config.BreakglassDefaultGasLimitV2 != 9000000 {
				t.Errorf("BreakglassDefaultGasLimitV2: got %d, want 9000000", tc.config.BreakglassDefaultGasLimitV2)
			}
			// ComputeBlockGasLimit requires maxGas > minGas for every active ceiling,
			// and the reduced ceilings must not exceed the ones they replace.
			if MIN_DYNAMIC_GAS_LIMIT >= tc.config.BreakglassDefaultGasLimitV2 {
				t.Errorf("MIN_DYNAMIC_GAS_LIMIT (%d) must be below BreakglassDefaultGasLimitV2 (%d)",
					MIN_DYNAMIC_GAS_LIMIT, tc.config.BreakglassDefaultGasLimitV2)
			}
			if tc.config.BreakglassDefaultGasLimitV2 >= tc.config.DefaultGasLimitV2 {
				t.Errorf("BreakglassDefaultGasLimitV2 (%d) must be below DefaultGasLimitV2 (%d)",
					tc.config.BreakglassDefaultGasLimitV2, tc.config.DefaultGasLimitV2)
			}
			if tc.config.DefaultGasLimitV2 > tc.config.DefaultGasLimit {
				t.Errorf("DefaultGasLimitV2 (%d) must not exceed DefaultGasLimit (%d)",
					tc.config.DefaultGasLimitV2, tc.config.DefaultGasLimit)
			}
		})
	}
}
