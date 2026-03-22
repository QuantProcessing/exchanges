package standx

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
	if os.Getenv("STANDX_PRIVATE_KEY") == "" {
		t.Skip("Skipping: STANDX keys not set")
	}
	adp, err := NewAdapter(context.Background(), Options{
		PrivateKey: os.Getenv("STANDX_PRIVATE_KEY"),
	})
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	return adp.(*Adapter)
}

func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, "BTC")
}

func TestPerpAdapter_Orders(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "ETH"})
}

func TestPerpAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 "ETH",
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: false,
		SupportsOrderHistory:   false,
	})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{Symbol: "ETH"})
}
