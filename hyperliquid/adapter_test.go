package hyperliquid

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/testsuite"

	"github.com/joho/godotenv"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	_ = godotenv.Load("../../.env")
	if os.Getenv("HYPERLIQUID_PRIVATE_KEY") == "" {
		t.Skip("Skipping: HYPERLIQUID keys not set")
	}
	adp, err := NewAdapter(context.Background(), Options{
		PrivateKey:  os.Getenv("HYPERLIQUID_PRIVATE_KEY"),
		AccountAddr: os.Getenv("HYPERLIQUID_ACCOUNT_ADDR"),
	})
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	return adp
}

func setupSpotAdapter(t *testing.T) *SpotAdapter {
	t.Helper()
	_ = godotenv.Load("../../.env")
	if os.Getenv("HYPERLIQUID_PRIVATE_KEY") == "" {
		t.Skip("Skipping: HYPERLIQUID keys not set")
	}
	adp, err := NewSpotAdapter(context.Background(), Options{
		PrivateKey:  os.Getenv("HYPERLIQUID_PRIVATE_KEY"),
		AccountAddr: os.Getenv("HYPERLIQUID_ACCOUNT_ADDR"),
	})
	if err != nil {
		t.Fatalf("NewSpotAdapter failed: %v", err)
	}
	return adp
}

func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, "BTC")
}

func TestPerpAdapter_Orders(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "HYPE"})
}

func TestSpotAdapter_Compliance(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, "BTC")
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "HYPE"})
}

func TestSpotAdapter_Orders(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:       "HYPE",
		SkipSlippage: true,
	})
}

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "HYPE"})
}
