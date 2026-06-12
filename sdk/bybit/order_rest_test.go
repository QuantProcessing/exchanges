package sdk

import (
	"context"
	"os"
	"testing"
)

func TestClient_PlaceOrder(t *testing.T) {
	client := requireBybitLiveWrite(t, "BYBIT_TEST_ORDER_QTY", "BYBIT_TEST_ORDER_PRICE")
	symbol := bybitEnvOrDefault("BYBIT_TEST_SYMBOL", bybitLinearSymbol)

	got, err := client.PlaceOrder(context.Background(), PlaceOrderRequest{
		Category:    "linear",
		Symbol:      symbol,
		Side:        bybitEnvOrDefault("BYBIT_TEST_ORDER_SIDE", "Buy"),
		OrderType:   "Limit",
		Qty:         os.Getenv("BYBIT_TEST_ORDER_QTY"),
		Price:       os.Getenv("BYBIT_TEST_ORDER_PRICE"),
		TimeInForce: "GTC",
		OrderLinkID: bybitEnvOrDefault("BYBIT_TEST_ORDER_LINK_ID", ""),
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected order response")
	}
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireBybitLiveWrite(t, "BYBIT_TEST_ORDER_ID")
	symbol := bybitEnvOrDefault("BYBIT_TEST_SYMBOL", bybitLinearSymbol)

	got, err := client.CancelOrder(context.Background(), CancelOrderRequest{
		Category: "linear",
		Symbol:   symbol,
		OrderID:  os.Getenv("BYBIT_TEST_ORDER_ID"),
	})
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected cancel response")
	}
}

func TestClient_CancelAllOrders(t *testing.T) {
	client := requireBybitLiveWrite(t)
	symbol := bybitEnvOrDefault("BYBIT_TEST_SYMBOL", bybitLinearSymbol)

	err := client.CancelAllOrders(context.Background(), CancelAllOrdersRequest{
		Category: "linear",
		Symbol:   symbol,
	})
	if err != nil {
		t.Fatalf("CancelAllOrders: %v", err)
	}
}

func TestClient_AmendOrder(t *testing.T) {
	client := requireBybitLiveWrite(t, "BYBIT_TEST_ORDER_ID", "BYBIT_TEST_ORDER_QTY", "BYBIT_TEST_ORDER_PRICE")
	symbol := bybitEnvOrDefault("BYBIT_TEST_SYMBOL", bybitLinearSymbol)

	got, err := client.AmendOrder(context.Background(), AmendOrderRequest{
		Category: "linear",
		Symbol:   symbol,
		OrderID:  os.Getenv("BYBIT_TEST_ORDER_ID"),
		Qty:      os.Getenv("BYBIT_TEST_ORDER_QTY"),
		Price:    os.Getenv("BYBIT_TEST_ORDER_PRICE"),
	})
	if err != nil {
		t.Fatalf("AmendOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected amend response")
	}
}

func TestClient_GetOpenOrders(t *testing.T) {
	got, err := newLivePrivateClient(t).GetOpenOrders(context.Background(), "linear", bybitLinearSymbol)
	if err != nil {
		t.Fatalf("GetOpenOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected open orders slice")
	}
}

func TestClient_GetOrderHistory(t *testing.T) {
	got, err := newLivePrivateClient(t).GetOrderHistory(context.Background(), "linear", bybitLinearSymbol)
	if err != nil {
		t.Fatalf("GetOrderHistory: %v", err)
	}
	if got == nil {
		t.Fatal("expected order history slice")
	}
}

func TestClient_GetOrderHistoryFiltered(t *testing.T) {
	client := newLivePrivateClient(t)
	orderID := os.Getenv("BYBIT_TEST_ORDER_ID")
	if orderID == "" {
		t.Skip("BYBIT_TEST_ORDER_ID is required for filtered order history live test")
	}

	got, err := client.GetOrderHistoryFiltered(context.Background(), "linear", bybitLinearSymbol, orderID, "")
	if err != nil {
		t.Fatalf("GetOrderHistoryFiltered: %v", err)
	}
	if got == nil {
		t.Fatal("expected filtered order history slice")
	}
}

func TestClient_GetRealtimeOrders(t *testing.T) {
	got, err := newLivePrivateClient(t).GetRealtimeOrders(context.Background(), "linear", bybitLinearSymbol, "", "", "", 0)
	if err != nil {
		t.Fatalf("GetRealtimeOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected realtime orders slice")
	}
}
