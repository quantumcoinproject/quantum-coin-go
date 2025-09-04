package types

import (
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"math/big"
	"testing"
)

func TestGas(t *testing.T) {
	a := big.NewInt(defaults.DEFAULT_PRICE)
	fmt.Println(a)
}
