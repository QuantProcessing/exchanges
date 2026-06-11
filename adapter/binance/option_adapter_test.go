package binance

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
)

func setupOptionAdapter(t *testing.T) *OptionAdapter {
	t.Helper()
	testenv.RequireFull(t, "BINANCE_OPTION_API_KEY", "BINANCE_OPTION_SECRET_KEY")
	adp, err := NewOptionAdapter(context.Background(), Options{
		APIKey:    os.Getenv("BINANCE_OPTION_API_KEY"),
		SecretKey: os.Getenv("BINANCE_OPTION_SECRET_KEY"),
	})
	if err != nil {
		t.Fatalf("NewOptionAdapter failed: %v", err)
	}
	return adp
}

func TestOptionAdapter_Compliance(t *testing.T) {
	adp := setupOptionAdapter(t)
	defer adp.Close()

	underlying := os.Getenv("BINANCE_OPTION_TEST_UNDERLYING")
	if underlying == "" {
		underlying = "BTC"
	}
	cfg := testsuite.OptionTradingAccountConfig{
		Underlying: underlying,
	}

	if instrumentID := os.Getenv("BINANCE_OPTION_TEST_INSTRUMENT"); instrumentID != "" {
		inst, err := adp.ParseInstrument(instrumentID)
		requireNoError(t, err, "BINANCE_OPTION_TEST_INSTRUMENT")

		qty := requireDecimalEnv(t, "BINANCE_OPTION_TEST_QTY")
		premium := requireDecimalEnv(t, "BINANCE_OPTION_TEST_LIMIT_PREMIUM")
		cfg.LiveTestInstrument = inst
		cfg.PassiveLimitQty = qty
		cfg.PassiveLimitPremium = premium
	}

	testsuite.RunOptionTradingAccountSuite(t, adp, cfg)
}

func requireNoError(t *testing.T, err error, field string) {
	t.Helper()
	if err != nil {
		t.Fatalf("invalid %s: %v", field, err)
	}
}

func requireDecimalEnv(t *testing.T, key string) decimal.Decimal {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s must be set when BINANCE_OPTION_TEST_INSTRUMENT is set", key)
	}
	parsed, err := decimal.NewFromString(value)
	requireNoError(t, err, key)
	if !parsed.IsPositive() {
		t.Fatalf("%s must be positive, got %s", key, parsed)
	}
	return parsed
}
