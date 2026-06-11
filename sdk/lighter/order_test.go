package lighter

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestClient_PlaceOrder(t *testing.T) {
	client := requireLighterLiveWrite(t, "LIGHTER_TEST_ORDER_PRICE", "LIGHTER_TEST_ORDER_BASE_AMOUNT")
	got, err := client.PlaceOrder(context.Background(), CreateOrderRequest{
		MarketId:      lighterMarketID(t),
		Price:         uint32(lighterIntEnv(t, "LIGHTER_TEST_ORDER_PRICE", 0)),
		BaseAmount:    lighterInt64Env(t, "LIGHTER_TEST_ORDER_BASE_AMOUNT", 0),
		IsAsk:         uint32(lighterIntEnv(t, "LIGHTER_TEST_ORDER_IS_ASK", 0)),
		OrderType:     OrderTypeLimit,
		TimeInForce:   OrderTimeInForcePostOnly,
		ClientOrderId: lighterInt64Env(t, "LIGHTER_TEST_CLIENT_ORDER_ID", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected create order response")
	}
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireLighterLiveWrite(t, "LIGHTER_TEST_ORDER_ID")
	got, err := client.CancelOrder(context.Background(), CancelOrderRequest{
		MarketId: lighterMarketID(t),
		OrderId:  lighterInt64Env(t, "LIGHTER_TEST_ORDER_ID", 0),
	})
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected cancel order response")
	}
}

func TestClient_ModifyOrder(t *testing.T) {
	client := requireLighterLiveWrite(t, "LIGHTER_TEST_ORDER_ID", "LIGHTER_TEST_ORDER_PRICE", "LIGHTER_TEST_ORDER_BASE_AMOUNT")
	got, err := client.ModifyOrder(context.Background(), ModifyOrderRequest{
		MarketId:   lighterMarketID(t),
		OrderIndex: lighterInt64Env(t, "LIGHTER_TEST_ORDER_ID", 0),
		BaseAmount: lighterInt64Env(t, "LIGHTER_TEST_ORDER_BASE_AMOUNT", 0),
		Price:      uint32(lighterIntEnv(t, "LIGHTER_TEST_ORDER_PRICE", 0)),
	})
	if err != nil {
		t.Fatalf("ModifyOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected modify order response")
	}
}

func TestClient_SendTxBatch(t *testing.T) {
	client := requireLighterLiveWrite(t, "LIGHTER_TEST_BATCH_TX_TYPE", "LIGHTER_TEST_BATCH_TX_INFO")
	got, err := client.SendTxBatch(context.Background(), []map[string]string{{
		"tx_type": os.Getenv("LIGHTER_TEST_BATCH_TX_TYPE"),
		"tx_info": os.Getenv("LIGHTER_TEST_BATCH_TX_INFO"),
	}})
	if err != nil {
		t.Fatalf("SendTxBatch: %v", err)
	}
	if got == nil {
		t.Fatal("expected batch response")
	}
}
