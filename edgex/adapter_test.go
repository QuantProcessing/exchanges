
package edgex

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
	if os.Getenv("EXCHANGES_EDGEX_PRIVATE_KEY") == "" {
		t.Skip("Skipping: EDGEX keys not set")
	}
	adp, err := NewAdapter(context.Background(), Options{
		PrivateKey: os.Getenv("EXCHANGES_EDGEX_PRIVATE_KEY"),
		AccountID:  os.Getenv("EXCHANGES_EDGEX_ACCOUNT_ID"),
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
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "DOGE"})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "DOGE"})
}
