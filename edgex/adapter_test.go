package edgex

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	testenv.RequireFull(t, "EDGEX_STARK_PRIVATE_KEY", "EDGEX_ACCOUNT_ID")
	adp, err := NewAdapter(context.Background(), Options{
		PrivateKey: os.Getenv("EDGEX_STARK_PRIVATE_KEY"),
		AccountID:  os.Getenv("EDGEX_ACCOUNT_ID"),
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

func TestPerpAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 "DOGE",
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: true,
		SupportsOrderHistory:   false,
	})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "DOGE"})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{Symbol: "DOGE"})
}
