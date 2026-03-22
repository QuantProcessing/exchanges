package backpack

import (
	"context"
	"os"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

func loadBackpackEnv() {
	for _, path := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	loadBackpackEnv()

	if os.Getenv("BACKPACK_API_KEY") == "" || os.Getenv("BACKPACK_PRIVATE_KEY") == "" {
		t.Skip("Skipping: BACKPACK credentials not set")
	}

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
	loadBackpackEnv()

	if os.Getenv("BACKPACK_API_KEY") == "" || os.Getenv("BACKPACK_PRIVATE_KEY") == "" {
		t.Skip("Skipping: BACKPACK credentials not set")
	}

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

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_PERP_TEST_SYMBOL"),
	})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{
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

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"),
	})
}

func TestSpotAdapter_LocalState(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{
		Symbol: requireEnvSymbol(t, "BACKPACK_SPOT_TEST_SYMBOL"),
	})
}
