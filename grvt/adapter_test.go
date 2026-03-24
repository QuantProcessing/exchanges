package grvt

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	testenv.RequireFull(t, "GRVT_API_KEY", "GRVT_SUB_ACCOUNT_ID", "GRVT_PRIVATE_KEY")
	adp, err := NewAdapter(context.Background(), Options{
		APIKey:       os.Getenv("GRVT_API_KEY"),
		SubAccountID: os.Getenv("GRVT_SUB_ACCOUNT_ID"),
		PrivateKey:   os.Getenv("GRVT_PRIVATE_KEY"),
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

func TestPerpAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 "ETH",
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: false,
		SupportsOrderHistory:   false,
	})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "ETH"})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{Symbol: "ETH"})
}
