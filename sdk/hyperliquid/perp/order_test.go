package perp

import (
	"context"
	"os"
	"strconv"
	"testing"

	hyperliquid "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
	"github.com/stretchr/testify/require"
)

func TestClient_UserOpenOrders(t *testing.T) {
	account := os.Getenv("HYPERLIQUID_ACCOUNT_ADDR")
	orders, err := newLivePrivateClient(t).UserOpenOrders(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, orders)
}

func TestClient_OrderStatus(t *testing.T) {
	client := newLivePrivateClient(t)
	orderID := os.Getenv("HYPERLIQUID_TEST_ORDER_ID")
	if orderID == "" {
		t.Skip("HYPERLIQUID_TEST_ORDER_ID is required for OrderStatus live test")
	}

	status, err := client.OrderStatus(context.Background(), os.Getenv("HYPERLIQUID_ACCOUNT_ADDR"), hyperliquidInt64Env(t, "HYPERLIQUID_TEST_ORDER_ID"))
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestClient_PlaceOrder(t *testing.T) {
	client := requireHyperliquidLiveWrite(t, "HYPERLIQUID_PERP_TEST_ASSET_ID", "HYPERLIQUID_TEST_ORDER_PRICE", "HYPERLIQUID_TEST_ORDER_SIZE")

	status, err := client.PlaceOrder(context.Background(), PlaceOrderRequest{
		AssetID: hyperliquidPerpAssetID(t),
		IsBuy:   hyperliquidBoolEnv("HYPERLIQUID_TEST_ORDER_IS_BUY", true),
		Price:   hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_PRICE"),
		Size:    hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_SIZE"),
		OrderType: OrderType{Limit: &OrderTypeLimit{
			Tif: hyperliquid.TifGtc,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestClient_ModifyOrder(t *testing.T) {
	client := requireHyperliquidLiveWrite(t, "HYPERLIQUID_PERP_TEST_ASSET_ID", "HYPERLIQUID_TEST_ORDER_ID", "HYPERLIQUID_TEST_ORDER_PRICE", "HYPERLIQUID_TEST_ORDER_SIZE")
	oid := hyperliquidInt64Env(t, "HYPERLIQUID_TEST_ORDER_ID")

	status, err := client.ModifyOrder(context.Background(), ModifyOrderRequest{
		Oid: &oid,
		Order: PlaceOrderRequest{
			AssetID: hyperliquidPerpAssetID(t),
			IsBuy:   hyperliquidBoolEnv("HYPERLIQUID_TEST_ORDER_IS_BUY", true),
			Price:   hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_PRICE"),
			Size:    hyperliquidFloatEnv(t, "HYPERLIQUID_TEST_ORDER_SIZE"),
			OrderType: OrderType{Limit: &OrderTypeLimit{
				Tif: hyperliquid.TifGtc,
			}},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestClient_CancelOrder(t *testing.T) {
	client := requireHyperliquidLiveWrite(t, "HYPERLIQUID_PERP_TEST_ASSET_ID", "HYPERLIQUID_TEST_ORDER_ID")

	status, err := client.CancelOrder(context.Background(), CancelOrderRequest{
		AssetID: hyperliquidPerpAssetID(t),
		OrderID: hyperliquidInt64Env(t, "HYPERLIQUID_TEST_ORDER_ID"),
	})
	require.NoError(t, err)
	require.NotNil(t, status)
}

func hyperliquidFloatEnv(t *testing.T, key string) float64 {
	t.Helper()
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	require.NoError(t, err)
	return value
}

func hyperliquidInt64Env(t *testing.T, key string) int64 {
	t.Helper()
	value, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	require.NoError(t, err)
	return value
}
