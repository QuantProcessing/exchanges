package okx

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
	if os.Getenv("EXCHANGES_OKX_API_KEY") == "" {
		t.Skip("Skipping: OKX keys not set")
	}
	return NewAdapter(context.Background(), Options{
		APIKey:     os.Getenv("EXCHANGES_OKX_API_KEY"),
		SecretKey:  os.Getenv("EXCHANGES_OKX_SECRET_KEY"),
		Passphrase: os.Getenv("EXCHANGES_OKX_PASSPHRASE"),
	})
}

func setupSpotAdapter(t *testing.T) *SpotAdapter {
	t.Helper()
	_ = godotenv.Load("../../.env")
	if os.Getenv("EXCHANGES_OKX_API_KEY") == "" {
		t.Skip("Skipping: OKX keys not set")
	}
	adp, err := NewSpotAdapter(context.Background(), Options{
		APIKey:     os.Getenv("EXCHANGES_OKX_API_KEY"),
		SecretKey:  os.Getenv("EXCHANGES_OKX_SECRET_KEY"),
		Passphrase: os.Getenv("EXCHANGES_OKX_PASSPHRASE"),
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
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "DOGE"})
}

func TestSpotAdapter_Compliance(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, "BTC")
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "DOGE"})
}

func TestSpotAdapter_Orders(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "DOGE"})
}

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "DOGE"})
}
