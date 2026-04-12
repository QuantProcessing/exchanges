package edgex

import (
	"context"
	"os"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
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

func TestPerpAdapter_TradingAccount(t *testing.T) {
	adp := setupPerpAdapter(t)
	testsuite.RunTradingAccountSuite(t, adp, testsuite.TradingAccountConfig{Symbol: "DOGE"})
}

func TestPerpAdapter_TradingAccountPostOnlyTopOfBookTerminalRefusal(t *testing.T) {
	adp := setupPerpAdapter(t)
	defer adp.Close()

	ctx := context.Background()
	acct := account.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(ctx))
	defer acct.Close()

	time.Sleep(2 * time.Second)

	qty, _ := testsuite.SmartQuantity(t, adp, "DOGE")
	ob, err := adp.FetchOrderBook(ctx, "DOGE", 1)
	require.NoError(t, err)
	require.NotEmpty(t, ob.Asks, "expected top-of-book ask for post-only rejection test")

	details, err := adp.FetchSymbolDetails(ctx, "DOGE")
	require.NoError(t, err)

	rejectPrice := ob.Asks[0].Price
	tick := decimal.New(1, -details.PricePrecision)
	rejectPrice = exchanges.RoundToPrecision(rejectPrice.Add(tick), details.PricePrecision)

	flow, err := acct.Place(ctx, &exchanges.OrderParams{
		Symbol:      "DOGE",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		TimeInForce: exchanges.TimeInForcePO,
		Price:       rejectPrice,
		Quantity:    qty,
	})
	require.NoError(t, err)
	defer flow.Close()

	initial := flow.Latest()
	require.NotNil(t, initial, "post-only reject flow should expose an initial snapshot")
	require.NotEmpty(t, initial.ClientOrderID, "TradingAccount should pre-bind a client order id before ack")

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	terminal, err := flow.Wait(waitCtx, func(o *exchanges.Order) bool {
		return o != nil && (o.Status == exchanges.OrderStatusRejected || o.Status == exchanges.OrderStatusCancelled)
	})
	require.NoError(t, err)
	require.Contains(t,
		[]exchanges.OrderStatus{exchanges.OrderStatusRejected, exchanges.OrderStatusCancelled},
		terminal.Status,
	)
	require.Equal(t, initial.ClientOrderID, terminal.ClientOrderID)
}
