package lighter

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	testenv.RequireFull(t, "LIGHTER_PRIVATE_KEY", "LIGHTER_ACCOUNT_INDEX", "LIGHTER_KEY_INDEX")
	adp, err := NewAdapter(context.Background(), Options{
		PrivateKey:   os.Getenv("LIGHTER_PRIVATE_KEY"),
		AccountIndex: os.Getenv("LIGHTER_ACCOUNT_INDEX"),
		KeyIndex:     os.Getenv("LIGHTER_KEY_INDEX"),
		RoToken:      os.Getenv("LIGHTER_RO_TOKEN"),
	})
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	return adp
}

func setupSpotAdapter(t *testing.T) *SpotAdapter {
	t.Helper()
	testenv.RequireFull(t, "LIGHTER_PRIVATE_KEY", "LIGHTER_ACCOUNT_INDEX", "LIGHTER_KEY_INDEX")
	adp, err := NewSpotAdapter(context.Background(), Options{
		PrivateKey:   os.Getenv("LIGHTER_PRIVATE_KEY"),
		AccountIndex: os.Getenv("LIGHTER_ACCOUNT_INDEX"),
		KeyIndex:     os.Getenv("LIGHTER_KEY_INDEX"),
		RoToken:      os.Getenv("LIGHTER_RO_TOKEN"),
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

func TestSpotAdapter_Compliance(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, "ETH")
}

func TestSpotAdapter_Orders(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{Symbol: "ETH"})
}

func TestSpotAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 "ETH",
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: false,
		SupportsOrderHistory:   false,
	})
}

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "ETH"})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{Symbol: "ETH"})
}
