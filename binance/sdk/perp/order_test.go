package perp

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

func TestClient_PlaceOrder(t *testing.T) {
	client := requireBinancePerpLiveWrite(t)
	testenv.RequireLiveCredentials(t, "BINANCE_PERP_TEST_ORDER_QTY", "BINANCE_PERP_TEST_ORDER_PRICE")
	got, err := client.PlaceOrder(context.Background(), PlaceOrderParams{
		Symbol:           envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol),
		Side:             envOrDefault("BINANCE_PERP_TEST_ORDER_SIDE", "BUY"),
		Type:             "LIMIT",
		TimeInForce:      "GTC",
		Quantity:         os.Getenv("BINANCE_PERP_TEST_ORDER_QTY"),
		Price:            os.Getenv("BINANCE_PERP_TEST_ORDER_PRICE"),
		NewClientOrderID: envOrDefault("BINANCE_PERP_TEST_CLIENT_ORDER_ID", "sdk-live-write-test"),
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if got.OrderID == 0 {
		t.Fatalf("unexpected place order response: %+v", got)
	}
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireBinancePerpLiveWrite(t)
	testenv.RequireLiveCredentials(t, "BINANCE_PERP_TEST_CANCEL_ORDER_ID")
	got, err := client.CancelOrder(context.Background(), CancelOrderParams{
		Symbol:  envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol),
		OrderID: os.Getenv("BINANCE_PERP_TEST_CANCEL_ORDER_ID"),
	})
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if got.OrderID == 0 {
		t.Fatalf("unexpected cancel response: %+v", got)
	}
}

func TestClient_ModifyOrder(t *testing.T) {
	client := requireBinancePerpLiveWrite(t)
	testenv.RequireLiveCredentials(t, "BINANCE_PERP_TEST_MODIFY_ORDER_ID", "BINANCE_PERP_TEST_ORDER_QTY", "BINANCE_PERP_TEST_ORDER_PRICE")
	orderID, err := strconv.ParseInt(os.Getenv("BINANCE_PERP_TEST_MODIFY_ORDER_ID"), 10, 64)
	if err != nil {
		t.Fatalf("parse BINANCE_PERP_TEST_MODIFY_ORDER_ID: %v", err)
	}
	got, err := client.ModifyOrder(context.Background(), ModifyOrderParams{
		Symbol:   envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol),
		Side:     envOrDefault("BINANCE_PERP_TEST_ORDER_SIDE", "BUY"),
		OrderID:  orderID,
		Quantity: os.Getenv("BINANCE_PERP_TEST_ORDER_QTY"),
		Price:    os.Getenv("BINANCE_PERP_TEST_ORDER_PRICE"),
	})
	if err != nil {
		t.Fatalf("ModifyOrder: %v", err)
	}
	if got.OrderID == 0 {
		t.Fatalf("unexpected modify response: %+v", got)
	}
}

func TestClient_CancelAllOpenOrders(t *testing.T) {
	err := requireBinancePerpLiveWrite(t).CancelAllOpenOrders(context.Background(), CancelAllOrdersParams{
		Symbol: envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol),
	})
	if err != nil {
		t.Fatalf("CancelAllOpenOrders: %v", err)
	}
}

func TestClient_GetOrder(t *testing.T) {
	client := newLivePrivateClient(t)
	testenv.RequireLiveCredentials(t, "BINANCE_PERP_TEST_ORDER_ID")
	orderID, err := strconv.ParseInt(os.Getenv("BINANCE_PERP_TEST_ORDER_ID"), 10, 64)
	if err != nil {
		t.Fatalf("parse BINANCE_PERP_TEST_ORDER_ID: %v", err)
	}
	got, err := client.GetOrder(context.Background(), envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol), orderID, "")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.OrderID != orderID {
		t.Fatalf("unexpected get order response: %+v", got)
	}
}

func TestClient_GetOpenOrders(t *testing.T) {
	got, err := newLivePrivateClient(t).GetOpenOrders(context.Background(), envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol))
	if err != nil {
		t.Fatalf("GetOpenOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil open orders slice")
	}
}

func TestClient_MyTrades(t *testing.T) {
	got, err := newLivePrivateClient(t).MyTrades(context.Background(), envOrDefault("BINANCE_PERP_TEST_SYMBOL", binancePerpTestSymbol), 5, 0, 0, 0)
	if err != nil {
		t.Fatalf("MyTrades: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil trades slice")
	}
}
