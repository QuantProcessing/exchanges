package bybit

import (
	"context"
	"os"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
)

func setupPerpAdapterWS(t *testing.T) *Adapter {
	t.Helper()
	requireBybitWSTests(t)
	return setupPerpAdapter(t)
}

func setupSpotAdapterWS(t *testing.T) *SpotAdapter {
	t.Helper()
	requireBybitWSTests(t)
	return setupSpotAdapter(t)
}

func requireBybitWSTests(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping: Bybit WS order tests are excluded by -short")
	}
	if os.Getenv("BYBIT_ENABLE_WS_ORDER_TESTS") != "1" {
		t.Skip("Skipping: set BYBIT_ENABLE_WS_ORDER_TESTS=1 to run Bybit trade WS live tests")
	}
}

func TestSpotAdapter_Orders_WS(t *testing.T) {
	adp := setupSpotAdapterWS(t)
	symbol := requireEnvSymbol(t, "BYBIT_SPOT_TEST_SYMBOL")
	updates := testsuite.SetupOrderWatch(t, adp)

	qty, _ := testsuite.SmartQuantity(t, adp, symbol)
	price := testsuite.SmartLimitPrice(t, adp, symbol, exchanges.OrderSideBuy)
	clientID := exchanges.GenerateID()

	err := adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:      symbol,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    qty,
		Price:       price,
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    clientID,
	})
	if err != nil {
		t.Fatalf("PlaceOrderWS failed: %v", err)
	}

	confirmed := testsuite.WaitOrderStatus(t, updates, "", clientID, exchanges.OrderStatusNew, 20*time.Second)
	cancelID := confirmed.OrderID
	if cancelID == "" {
		t.Fatalf("expected order id after ws place, got empty")
	}

	err = adp.CancelOrderWS(context.Background(), cancelID, symbol)
	if err != nil {
		t.Fatalf("CancelOrderWS failed: %v", err)
	}

	cancelled := testsuite.WaitOrderStatus(t, updates, cancelID, clientID, exchanges.OrderStatusCancelled, 20*time.Second)
	if cancelled.Status != exchanges.OrderStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}
}

func TestPerpAdapter_Orders_WS(t *testing.T) {
	adp := setupPerpAdapterWS(t)
	symbol := requireEnvSymbol(t, "BYBIT_PERP_TEST_SYMBOL")
	updates := testsuite.SetupOrderWatch(t, adp)

	qty, _ := testsuite.SmartQuantity(t, adp, symbol)
	price := testsuite.SmartLimitPrice(t, adp, symbol, exchanges.OrderSideBuy)
	clientID := exchanges.GenerateID()

	err := adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:      symbol,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    qty,
		Price:       price,
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    clientID,
	})
	if err != nil {
		t.Fatalf("PlaceOrderWS failed: %v", err)
	}

	confirmed := testsuite.WaitOrderStatus(t, updates, "", clientID, exchanges.OrderStatusNew, 20*time.Second)
	cancelID := confirmed.OrderID
	if cancelID == "" {
		t.Fatalf("expected order id after ws place, got empty")
	}

	err = adp.CancelOrderWS(context.Background(), cancelID, symbol)
	if err != nil {
		t.Fatalf("CancelOrderWS failed: %v", err)
	}

	cancelled := testsuite.WaitOrderStatus(t, updates, cancelID, clientID, exchanges.OrderStatusCancelled, 20*time.Second)
	if cancelled.Status != exchanges.OrderStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}
}

func TestPerpAdapter_ModifyOrder_WS(t *testing.T) {
	adp := setupPerpAdapterWS(t)
	symbol := requireEnvSymbol(t, "BYBIT_PERP_TEST_SYMBOL")
	updates := testsuite.SetupOrderWatch(t, adp)

	qty, _ := testsuite.SmartQuantity(t, adp, symbol)
	price := testsuite.SmartLimitPrice(t, adp, symbol, exchanges.OrderSideBuy)

	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:      symbol,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    qty,
		Price:       price,
		TimeInForce: exchanges.TimeInForceGTC,
	})
	if err != nil {
		t.Fatalf("PlaceOrder failed: %v", err)
	}

	confirmed := testsuite.WaitOrderStatus(t, updates, order.OrderID, order.ClientOrderID, exchanges.OrderStatusNew, 20*time.Second)
	modifiedPrice := price.Add(decimal.NewFromInt(1))

	err = adp.ModifyOrderWS(context.Background(), confirmed.OrderID, symbol, &exchanges.ModifyOrderParams{
		Quantity: confirmed.Quantity,
		Price:    modifiedPrice,
	})
	if err != nil {
		t.Fatalf("ModifyOrderWS failed: %v", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		current, err := adp.FetchOrderByID(context.Background(), confirmed.OrderID, symbol)
		if err == nil && current != nil && current.OrderPrice.Equal(modifiedPrice) {
			goto cancel
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("modified order price was not observed within timeout")

cancel:
	if err := adp.CancelOrder(context.Background(), confirmed.OrderID, symbol); err != nil {
		t.Fatalf("CancelOrder cleanup failed: %v", err)
	}
	_ = testsuite.WaitOrderStatus(t, updates, confirmed.OrderID, confirmed.ClientOrderID, exchanges.OrderStatusCancelled, 20*time.Second)
}
