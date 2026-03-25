package decibel

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
)

func setupPerpAdapter(t *testing.T) *Adapter {
	t.Helper()
	testenv.RequireFull(
		t,
		"DECIBEL_API_KEY",
		"DECIBEL_PRIVATE_KEY",
		"DECIBEL_SUBACCOUNT_ADDR",
		"DECIBEL_PERP_TEST_SYMBOL",
	)

	adp, err := NewAdapter(context.Background(), Options{
		APIKey:         os.Getenv("DECIBEL_API_KEY"),
		PrivateKey:     os.Getenv("DECIBEL_PRIVATE_KEY"),
		SubaccountAddr: os.Getenv("DECIBEL_SUBACCOUNT_ADDR"),
	})
	if err != nil {
		t.Fatalf("NewAdapter failed: %v", err)
	}
	t.Cleanup(func() {
		_ = adp.Close()
	})
	return adp
}

func TestPerpAdapter_Compliance(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunAdapterComplianceTests(t, adp, os.Getenv("DECIBEL_PERP_TEST_SYMBOL"))
}

func TestPerpAdapter_Orders(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunOrderSuite(t, adp, testsuite.OrderSuiteConfig{
		Symbol: os.Getenv("DECIBEL_PERP_TEST_SYMBOL"),
	})
}

func TestPerpAdapter_Lifecycle(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunLifecycleSuite(t, adp, testsuite.LifecycleConfig{
		Symbol: os.Getenv("DECIBEL_PERP_TEST_SYMBOL"),
	})
}
