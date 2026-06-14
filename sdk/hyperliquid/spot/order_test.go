package spot

import (
	"context"
	"os"
	"strconv"
	"testing"

	hyperliquid "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
)

func TestClient_UserOpenOrders(t *testing.T) {
	account := os.Getenv("HYPERLIQUID_ACCOUNT_ADDR")
	orders, err := newLivePrivateClient(t).UserOpenOrders(context.Background(), account)
	if err != nil {
		t.Fatalf("UserOpenOrders: %v", err)
	}
	if orders == nil {
		t.Fatal("expected orders slice")
	}
}

func TestClient_PlaceOrder(t *testing.T) {
	client := requireHyperliquidLiveWrite(t, "HYPERLIQUID_SPOT_TEST_ASSET_ID", "HYPERLIQUID_TEST_ORDER_PRICE", "HYPERLIQUID_TEST_ORDER_SIZE")
	assetID := hyperliquidSpotAssetID(t)
	price := hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_PRICE")
	size := hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_SIZE")

	status, err := client.PlaceOrder(context.Background(), PlaceOrderRequest{
		AssetID: assetID,
		IsBuy:   hyperliquidEnvOrDefault("HYPERLIQUID_TEST_ORDER_SIDE", "buy") == "buy",
		Price:   price,
		Size:    size,
		OrderType: OrderType{Limit: &OrderTypeLimit{
			Tif: hyperliquid.TifGtc,
		}},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if status == nil {
		t.Fatal("expected order status")
	}
}

func TestClient_ModifyOrder(t *testing.T) {
	client := requireHyperliquidLiveWrite(t, "HYPERLIQUID_SPOT_TEST_ASSET_ID", "HYPERLIQUID_TEST_ORDER_ID", "HYPERLIQUID_TEST_ORDER_PRICE", "HYPERLIQUID_TEST_ORDER_SIZE")
	oid := hyperliquidInt64Env(t, "HYPERLIQUID_TEST_ORDER_ID")

	status, err := client.ModifyOrder(context.Background(), ModifyOrderRequest{
		Oid: &oid,
		Order: PlaceOrderRequest{
			AssetID: hyperliquidSpotAssetID(t),
			IsBuy:   hyperliquidEnvOrDefault("HYPERLIQUID_TEST_ORDER_SIDE", "buy") == "buy",
			Price:   hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_PRICE"),
			Size:    hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_SIZE"),
			OrderType: OrderType{Limit: &OrderTypeLimit{
				Tif: hyperliquid.TifGtc,
			}},
		},
	})
	if err != nil {
		t.Fatalf("ModifyOrder: %v", err)
	}
	if status == nil {
		t.Fatal("expected modify status")
	}
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireHyperliquidLiveWrite(t, "HYPERLIQUID_SPOT_TEST_ASSET_ID", "HYPERLIQUID_TEST_ORDER_ID")

	status, err := client.CancelOrder(context.Background(), CancelOrderRequest{
		AssetID: hyperliquidSpotAssetID(t),
		OrderID: hyperliquidInt64Env(t, "HYPERLIQUID_TEST_ORDER_ID"),
	})
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if status == nil {
		t.Fatal("expected cancel status")
	}
}

func hyperliquidSpotAssetID(t *testing.T) int {
	t.Helper()
	raw := os.Getenv("HYPERLIQUID_SPOT_TEST_ASSET_ID")
	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse HYPERLIQUID_SPOT_TEST_ASSET_ID: %v", err)
	}
	return value
}

func hyperliquidFloatEnv(t *testing.T, key string) float64 {
	t.Helper()
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		t.Fatalf("parse %s: %v", key, err)
	}
	return value
}

func hyperliquidInt64Env(t *testing.T, key string) int64 {
	t.Helper()
	value, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil {
		t.Fatalf("parse %s: %v", key, err)
	}
	return value
}
