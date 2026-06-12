package okx

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

func TestClient_PlaceOrder(t *testing.T) {
	client := requireOKXLiveWrite(t)
	testenv.RequireLiveCredentials(t, "OKX_TEST_ORDER_SIZE", "OKX_TEST_ORDER_PRICE")
	price := os.Getenv("OKX_TEST_ORDER_PRICE")
	clOrdID := okxEnvOrDefault("OKX_TEST_CLIENT_ORDER_ID", "sdk-live-write-test")
	got, err := client.PlaceOrder(context.Background(), &OrderRequest{
		InstId:  okxEnvOrDefault("OKX_TEST_ORDER_INST_ID", okxSpotInstID),
		TdMode:  okxEnvOrDefault("OKX_TEST_TD_MODE", "cash"),
		ClOrdId: &clOrdID,
		Side:    okxEnvOrDefault("OKX_TEST_ORDER_SIDE", "buy"),
		OrdType: "limit",
		Sz:      os.Getenv("OKX_TEST_ORDER_SIZE"),
		Px:      &price,
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("unexpected place response: %+v", got)
	}
}

func TestClient_ModifyOrder(t *testing.T) {
	client := requireOKXLiveWrite(t)
	testenv.RequireLiveCredentials(t, "OKX_TEST_ORDER_ID", "OKX_TEST_ORDER_SIZE", "OKX_TEST_ORDER_PRICE")
	orderID := os.Getenv("OKX_TEST_ORDER_ID")
	size := os.Getenv("OKX_TEST_ORDER_SIZE")
	price := os.Getenv("OKX_TEST_ORDER_PRICE")
	got, err := client.ModifyOrder(context.Background(), &ModifyOrderRequest{
		InstId: okxEnvOrDefault("OKX_TEST_ORDER_INST_ID", okxSpotInstID),
		OrdId:  &orderID,
		NewSz:  &size,
		NewPx:  &price,
	})
	if err != nil {
		t.Fatalf("ModifyOrder: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("unexpected modify response: %+v", got)
	}
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireOKXLiveWrite(t)
	testenv.RequireLiveCredentials(t, "OKX_TEST_ORDER_ID")
	got, err := client.CancelOrder(context.Background(), okxEnvOrDefault("OKX_TEST_ORDER_INST_ID", okxSpotInstID), os.Getenv("OKX_TEST_ORDER_ID"), "")
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("unexpected cancel response: %+v", got)
	}
}

func TestClient_CancelOrders(t *testing.T) {
	client := requireOKXLiveWrite(t)
	testenv.RequireLiveCredentials(t, "OKX_TEST_ORDER_ID")
	orderID := os.Getenv("OKX_TEST_ORDER_ID")
	got, err := client.CancelOrders(context.Background(), []CancelOrderRequest{{
		InstId: okxEnvOrDefault("OKX_TEST_ORDER_INST_ID", okxSpotInstID),
		OrdId:  &orderID,
	}})
	if err != nil {
		t.Fatalf("CancelOrders: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("unexpected cancel batch response: %+v", got)
	}
}

func TestClient_ClosePosition(t *testing.T) {
	got, err := requireOKXLiveWrite(t).ClosePosition(
		context.Background(),
		okxEnvOrDefault("OKX_TEST_CLOSE_POSITION_INST_ID", okxSwapInstID),
		okxEnvOrDefault("OKX_TEST_MARGIN_MODE", "cross"),
	)
	if err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil close-position response")
	}
}

func TestClient_GetOrder(t *testing.T) {
	client := newLivePrivateClient(t)
	orderID := os.Getenv("OKX_TEST_ORDER_ID")
	clientOrderID := os.Getenv("OKX_TEST_CLIENT_ORDER_ID")
	if orderID == "" && clientOrderID == "" {
		t.Skip("skipping private read: set OKX_TEST_ORDER_ID or OKX_TEST_CLIENT_ORDER_ID to query a real order")
	}
	got, err := client.GetOrder(context.Background(), okxEnvOrDefault("OKX_TEST_ORDER_INST_ID", okxSpotInstID), orderID, clientOrderID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("unexpected order response: %+v", got)
	}
}

func TestClient_GetOrders(t *testing.T) {
	instType := "SPOT"
	instID := okxSpotInstID
	got, err := newLivePrivateClient(t).GetOrders(context.Background(), &instType, &instID)
	if err != nil {
		t.Fatalf("GetOrders: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil pending orders slice")
	}
}
