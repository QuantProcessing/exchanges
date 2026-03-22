package aster

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/testsuite"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	_ = godotenv.Load("../../.env")
	if os.Getenv("ASTER_API_KEY") == "" {
		t.Skip("Skipping: ASTER keys not set")
	}
	adp, err := NewAdapter(context.Background(), Options{
		APIKey:    os.Getenv("ASTER_API_KEY"),
		SecretKey: os.Getenv("ASTER_SECRET_KEY"),
	})
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	return adp
}

func setupSpotAdapter(t *testing.T) *SpotAdapter {
	t.Helper()
	_ = godotenv.Load("../../.env")
	if os.Getenv("ASTER_API_KEY") == "" {
		t.Skip("Skipping: ASTER keys not set")
	}
	adp, err := NewSpotAdapter(context.Background(), Options{
		APIKey:    os.Getenv("ASTER_API_KEY"),
		SecretKey: os.Getenv("ASTER_SECRET_KEY"),
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
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:   "DOGE",
		Slippage: decimal.NewFromFloat(0.015), // Aster caps limit price at exactly 2% above market
	})
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
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol:       "ASTER",
		SkipSlippage: true, // Spot market orders don't support slippage
	})
}

func TestSpotAdapter_OrderQuerySemantics(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunOrderQuerySemanticsSuite(t, adp, testsuite.OrderQueryConfig{
		Symbol:                 "ASTER",
		SupportsOpenOrders:     true,
		SupportsTerminalLookup: true,
		SupportsOrderHistory:   false,
	})
}

func TestSpotAdapter_Lifecycle(t *testing.T) {
	adp := setupSpotAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{Symbol: "ASTER"})
}

func TestPerpAdapter_LocalState(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLocalStateSuite(t, adp, testsuite.LocalStateConfig{Symbol: "DOGE"})
}
