package sdk

import (
	"context"
	"os"
	"testing"
)

func TestClient_PlaceOrder(t *testing.T) {
	client := requireBitgetLiveWrite(t, "BITGET_TEST_ORDER_QTY", "BITGET_TEST_ORDER_PRICE")
	symbol := bitgetEnvOrDefault("BITGET_TEST_SYMBOL", bitgetSpotSymbol)

	got, err := client.PlaceOrder(context.Background(), &PlaceOrderRequest{
		Category:    bitgetSpotCategory,
		Symbol:      symbol,
		Qty:         os.Getenv("BITGET_TEST_ORDER_QTY"),
		Price:       os.Getenv("BITGET_TEST_ORDER_PRICE"),
		Side:        bitgetEnvOrDefault("BITGET_TEST_ORDER_SIDE", "buy"),
		OrderType:   "limit",
		TimeInForce: "gtc",
		ClientOID:   bitgetEnvOrDefault("BITGET_TEST_CLIENT_ORDER_ID", ""),
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected place order response")
	}
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireBitgetLiveWrite(t, "BITGET_TEST_ORDER_ID")
	symbol := bitgetEnvOrDefault("BITGET_TEST_SYMBOL", bitgetSpotSymbol)

	got, err := client.CancelOrder(context.Background(), &CancelOrderRequest{
		Category: bitgetSpotCategory,
		Symbol:   symbol,
		OrderID:  os.Getenv("BITGET_TEST_ORDER_ID"),
	})
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected cancel order response")
	}
}

func TestClient_CancelAllOrders(t *testing.T) {
	client := requireBitgetLiveWrite(t)
	symbol := bitgetEnvOrDefault("BITGET_TEST_SYMBOL", bitgetSpotSymbol)

	if err := client.CancelAllOrders(context.Background(), &CancelAllOrdersRequest{Category: bitgetSpotCategory, Symbol: symbol}); err != nil {
		t.Fatalf("CancelAllOrders: %v", err)
	}
}

func TestClient_ModifyOrder(t *testing.T) {
	client := requireBitgetLiveWrite(t, "BITGET_TEST_ORDER_ID", "BITGET_TEST_ORDER_PRICE")
	symbol := bitgetEnvOrDefault("BITGET_TEST_SYMBOL", bitgetSpotSymbol)

	got, err := client.ModifyOrder(context.Background(), &ModifyOrderRequest{
		Category: bitgetSpotCategory,
		Symbol:   symbol,
		OrderID:  os.Getenv("BITGET_TEST_ORDER_ID"),
		NewPrice: os.Getenv("BITGET_TEST_ORDER_PRICE"),
		NewQty:   os.Getenv("BITGET_TEST_ORDER_QTY"),
	})
	if err != nil {
		t.Fatalf("ModifyOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected modify order response")
	}
}

func TestClient_GetOrder(t *testing.T) {
	client := newLivePrivateClient(t)
	orderID := os.Getenv("BITGET_TEST_ORDER_ID")
	if orderID == "" {
		t.Skip("BITGET_TEST_ORDER_ID is required for GetOrder live test")
	}

	got, err := client.GetOrder(context.Background(), bitgetSpotCategory, bitgetSpotSymbol, orderID, "")
	if err != nil {
		skipIfBitgetAccountModeMismatch(t, err)
		t.Fatalf("GetOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected order record")
	}
}

func TestClient_GetOpenOrders(t *testing.T) {
	got, err := newLivePrivateClient(t).GetOpenOrders(context.Background(), bitgetSpotCategory, bitgetSpotSymbol)
	if err != nil {
		skipIfBitgetAccountModeMismatch(t, err)
		t.Fatalf("GetOpenOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected open orders slice")
	}
}

func TestClient_GetOrderHistory(t *testing.T) {
	got, err := newLivePrivateClient(t).GetOrderHistory(context.Background(), bitgetSpotCategory, bitgetSpotSymbol)
	if err != nil {
		skipIfBitgetAccountModeMismatch(t, err)
		t.Fatalf("GetOrderHistory: %v", err)
	}
	if got == nil {
		t.Fatal("expected order history slice")
	}
}

func TestClient_GetAccountAssets(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountAssets(context.Background())
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA account assets endpoint")
		t.Fatalf("GetAccountAssets: %v", err)
	}
	if got == nil {
		t.Fatal("expected account assets")
	}
}

func TestClient_GetAccountInfo(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountInfo(context.Background())
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA account info endpoint")
		t.Fatalf("GetAccountInfo: %v", err)
	}
	if got == nil {
		t.Fatal("expected account info")
	}
}

func TestClient_GetFundingAssets(t *testing.T) {
	got, err := newLivePrivateClient(t).GetFundingAssets(context.Background(), os.Getenv("BITGET_TEST_COIN"))
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA funding assets endpoint")
		t.Fatalf("GetFundingAssets: %v", err)
	}
	if got == nil {
		t.Fatal("expected funding assets slice")
	}
}

func TestClient_GetFinancialRecords(t *testing.T) {
	got, err := newLivePrivateClient(t).GetFinancialRecords(context.Background(), FinancialRecordsRequest{
		Category: bitgetPerpCategory,
		Coin:     os.Getenv("BITGET_TEST_COIN"),
		Limit:    "10",
	})
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA financial records endpoint")
		t.Fatalf("GetFinancialRecords: %v", err)
	}
	if got == nil {
		t.Fatal("expected financial records")
	}
}

func TestClient_GetAccountFeeRate(t *testing.T) {
	got, err := newLivePrivateClient(t).GetAccountFeeRate(context.Background(), bitgetSpotCategory, bitgetSpotSymbol)
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA account fee rate endpoint")
		t.Fatalf("GetAccountFeeRate: %v", err)
	}
	if got == nil {
		t.Fatal("expected account fee rate")
	}
}

func TestClient_GetSwitchStatus(t *testing.T) {
	got, err := newLivePrivateClient(t).GetSwitchStatus(context.Background())
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA switch status endpoint")
		t.Fatalf("GetSwitchStatus: %v", err)
	}
	if got == nil {
		t.Fatal("expected switch status")
	}
}

func TestClient_GetMaxTransferable(t *testing.T) {
	got, err := newLivePrivateClient(t).GetMaxTransferable(context.Background(), bitgetEnvOrDefault("BITGET_TEST_COIN", "USDT"))
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA max transferable endpoint")
		t.Fatalf("GetMaxTransferable: %v", err)
	}
	if got == nil {
		t.Fatal("expected max transferable")
	}
}

func TestClient_GetOpenInterestLimit(t *testing.T) {
	got, err := newLivePrivateClient(t).GetOpenInterestLimit(context.Background(), bitgetPerpCategory, bitgetPerpSymbol)
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA open interest limit endpoint")
		t.Fatalf("GetOpenInterestLimit: %v", err)
	}
	if got == nil {
		t.Fatal("expected open interest limit")
	}
}

func TestClient_GetCurrentPositions(t *testing.T) {
	got, err := newLivePrivateClient(t).GetCurrentPositions(context.Background(), bitgetPerpCategory, bitgetPerpSymbol)
	if err != nil {
		skipIfBitgetPrivateReadUnavailable(t, err, "Bitget UTA current positions endpoint")
		t.Fatalf("GetCurrentPositions: %v", err)
	}
	if got == nil {
		t.Fatal("expected current positions slice")
	}
}

func TestClient_SetLeverage(t *testing.T) {
	client := requireBitgetLiveWrite(t)
	symbol := bitgetEnvOrDefault("BITGET_TEST_SYMBOL", bitgetPerpSymbol)
	leverage := bitgetEnvOrDefault("BITGET_TEST_LEVERAGE", "2")

	if err := client.SetLeverage(context.Background(), &SetLeverageRequest{Category: bitgetPerpCategory, Symbol: symbol, Leverage: leverage}); err != nil {
		t.Fatalf("SetLeverage: %v", err)
	}
}

func TestClient_SetHoldMode(t *testing.T) {
	client := requireBitgetLiveWrite(t)
	mode := bitgetEnvOrDefault("BITGET_TEST_HOLD_MODE", "one_way_mode")

	if err := client.SetHoldMode(context.Background(), mode); err != nil {
		t.Fatalf("SetHoldMode: %v", err)
	}
}
