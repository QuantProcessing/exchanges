package bitget

import (
	"context"
	"os"
	"strconv"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

func loadBitgetEnv() {
	for _, path := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	loadBitgetEnv()

	if os.Getenv("BITGET_API_KEY") == "" || os.Getenv("BITGET_SECRET_KEY") == "" || os.Getenv("BITGET_PASSPHRASE") == "" {
		t.Skip("Skipping: BITGET credentials not set")
	}

	opts := Options{
		APIKey:     os.Getenv("BITGET_API_KEY"),
		SecretKey:  os.Getenv("BITGET_SECRET_KEY"),
		Passphrase: os.Getenv("BITGET_PASSPHRASE"),
	}
	if quote := os.Getenv("BITGET_QUOTE_CURRENCY"); quote != "" {
		opts.QuoteCurrency = exchanges.QuoteCurrency(quote)
	}

	adp, err := NewAdapter(context.Background(), opts)
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	configurePerpTestLeverage(t, adp)
	return adp
}

func setupSpotAdapter(t *testing.T) *SpotAdapter {
	t.Helper()
	loadBitgetEnv()

	if os.Getenv("BITGET_API_KEY") == "" || os.Getenv("BITGET_SECRET_KEY") == "" || os.Getenv("BITGET_PASSPHRASE") == "" {
		t.Skip("Skipping: BITGET credentials not set")
	}

	opts := Options{
		APIKey:     os.Getenv("BITGET_API_KEY"),
		SecretKey:  os.Getenv("BITGET_SECRET_KEY"),
		Passphrase: os.Getenv("BITGET_PASSPHRASE"),
	}
	if quote := os.Getenv("BITGET_QUOTE_CURRENCY"); quote != "" {
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

func configurePerpTestLeverage(t *testing.T, adp *Adapter) {
	t.Helper()

	symbol := os.Getenv("BITGET_PERP_TEST_SYMBOL")
	if symbol == "" {
		return
	}

	leverage := 20
	if raw := os.Getenv("BITGET_PERP_TEST_LEVERAGE"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("invalid BITGET_PERP_TEST_LEVERAGE: %v", err)
		}
		leverage = parsed
	}

	if err := adp.SetLeverage(context.Background(), symbol, leverage); err != nil {
		t.Fatalf("SetLeverage(%s,%d) failed: %v", symbol, leverage, err)
	}
}

func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, requireEnvSymbol(t, "BITGET_PERP_TEST_SYMBOL"))
}

func TestPerpAdapter_Orders(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:   requireEnvSymbol(t, "BITGET_PERP_TEST_SYMBOL"),
		Slippage: decimal.NewFromFloat(0.01),
	})
}

func TestPerpAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 requireEnvSymbol(t, "BITGET_PERP_TEST_SYMBOL"),
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: true,
		SupportsOrderHistory:   true,
	})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: requireEnvSymbol(t, "BITGET_PERP_TEST_SYMBOL"),
	})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{
		Symbol: requireEnvSymbol(t, "BITGET_PERP_TEST_SYMBOL"),
	})
}

func TestSpotAdapter_Compliance(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, requireEnvSymbol(t, "BITGET_SPOT_TEST_SYMBOL"))
}

func TestSpotAdapter_Orders(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:   requireEnvSymbol(t, "BITGET_SPOT_TEST_SYMBOL"),
		Slippage: decimal.NewFromFloat(0.01),
	})
}

func TestSpotAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 requireEnvSymbol(t, "BITGET_SPOT_TEST_SYMBOL"),
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: true,
		SupportsOrderHistory:   true,
	})
}

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: requireEnvSymbol(t, "BITGET_SPOT_TEST_SYMBOL"),
	})
}

func TestSpotAdapter_LocalState(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{
		Symbol: requireEnvSymbol(t, "BITGET_SPOT_TEST_SYMBOL"),
	})
}
