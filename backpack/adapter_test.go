package backpack

import (
	"context"
	"os"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	testenv.RequireFull(t, "BACKPACK_API_KEY", "BACKPACK_PRIVATE_KEY")

	opts := Options{
		APIKey:     os.Getenv("BACKPACK_API_KEY"),
		PrivateKey: os.Getenv("BACKPACK_PRIVATE_KEY"),
	}
	if quote := os.Getenv("BACKPACK_QUOTE_CURRENCY"); quote != "" {
		opts.QuoteCurrency = exchanges.QuoteCurrency(quote)
	}

	adp, err := NewAdapter(context.Background(), opts)
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	return adp
}

func setupSpotAdapter(t *testing.T) *SpotAdapter {
	t.Helper()
	testenv.RequireFull(t, "BACKPACK_API_KEY", "BACKPACK_PRIVATE_KEY")

	opts := Options{
		APIKey:     os.Getenv("BACKPACK_API_KEY"),
		PrivateKey: os.Getenv("BACKPACK_PRIVATE_KEY"),
	}
	if quote := os.Getenv("BACKPACK_QUOTE_CURRENCY"); quote != "" {
		opts.QuoteCurrency = exchanges.QuoteCurrency(quote)
	}

	adp, err := NewSpotAdapter(context.Background(), opts)
	if err != nil {
		t.Fatalf("NewSpotAdapter failed: %v", err)
	}
	return adp
}

func requireEnvSymbol(t *testing.T, key string) string {
	t.Helper()
	symbol := os.Getenv(key)
	if symbol == "" {
		t.Skipf("Skipping: %s not set", key)
	}
	return symbol
}

func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, requireEnvSymbol(t, "BACKPACK_PERP_TEST_SYMBOL"))
}

func TestPerpAdapter_Orders(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:   requireEnvSymbol(t, "BACKPACK_PERP_TEST_SYMBOL"),
		Slippage: decimal.NewFromFloat(0.01),
	})
}

func TestPerpAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 requireEnvSymbol(t, "BACKPACK_PERP_TEST_SYMBOL"),
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: false,
		SupportsOrderHistory:   false,
	})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_PERP_TEST_SYMBOL"),
	})
}

func TestPerpAdapter_TradingAccount(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunTradingAccountSuite(t, adp, testsuite.TradingAccountConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_PERP_TEST_SYMBOL"),
	})
}

func TestSpotAdapter_Compliance(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"))
}

func TestSpotAdapter_Orders(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:       requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"),
		SkipSlippage: true,
	})
}

func TestSpotAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"),
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: false,
		SupportsOrderHistory:   false,
	})
}

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"),
	})
}

func TestSpotAdapter_TradingAccount(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunTradingAccountSuite(t, adp, testsuite.TradingAccountConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"),
	})
}
