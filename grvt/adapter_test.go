//go:build grvt

package grvt

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
	if os.Getenv("EXCHANGES_GRVT_API_KEY") == "" {
		t.Skip("Skipping: GRVT keys not set")
	}
	adp, err := NewAdapter(context.Background(), Options{
		APIKey:       os.Getenv("EXCHANGES_GRVT_API_KEY"),
		SubAccountID: os.Getenv("EXCHANGES_GRVT_SUB_ACCOUNT_ID"),
		PrivateKey:   os.Getenv("EXCHANGES_GRVT_PRIVATE_KEY"),
	})
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	return adp
}

func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, "BTC")
}

func TestPerpAdapter_Orders(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "ETH"})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "ETH"})
}
