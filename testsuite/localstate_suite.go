package testsuite

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
)

// LocalStateConfig configures the LocalState integration test.
type LocalStateConfig struct {
	Symbol string // Required: e.g. "DOGE"
}

func RunLocalStateSuite(t *testing.T, adp exchanges.Exchange, cfg LocalStateConfig) {
	RunTradingAccountSuite(t, adp, TradingAccountConfig{Symbol: cfg.Symbol})
}
