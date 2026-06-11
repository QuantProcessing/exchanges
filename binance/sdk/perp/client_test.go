package perp

import (
	"context"
	"os"
	"testing"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

const (
	binancePerpLiveWriteFlag = "BINANCE_PERP_ENABLE_LIVE_WRITE_TESTS"
	binancePerpTestSymbol    = "BTCUSDT"
)

func newLiveClient() *Client {
	return NewClient()
}

func newLivePrivateClient(t *testing.T) *Client {
	t.Helper()
	testenv.RequireLiveCredentials(t, "BINANCE_API_KEY", "BINANCE_SECRET_KEY")
	return NewClient().WithCredentials(os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_SECRET_KEY"))
}

func requireBinancePerpLiveWrite(t *testing.T) *Client {
	t.Helper()
	testenv.RequireLiveWrite(t, binancePerpLiveWriteFlag, "BINANCE_API_KEY", "BINANCE_SECRET_KEY")
	return newLivePrivateClient(t)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func TestClient_WithCredentials(t *testing.T) {
	client := NewClient().WithCredentials("key", "secret")

	if client.APIKey != "key" || client.SecretKey != "secret" {
		t.Fatalf("unexpected credentials: %+v", client)
	}
}

func TestClient_Get(t *testing.T) {
	var out struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := newLiveClient().Get(context.Background(), "/fapi/v1/time", nil, false, &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if out.ServerTime == 0 {
		t.Fatalf("unexpected server time response: %+v", out)
	}
}

func TestClient_Post(t *testing.T) {
	client := requireBinancePerpLiveWrite(t)
	if _, err := client.CreateListenKey(context.Background()); err != nil {
		t.Fatalf("Post via CreateListenKey: %v", err)
	}
}

func TestClient_Delete(t *testing.T) {
	client := requireBinancePerpLiveWrite(t)
	if err := client.CloseListenKey(context.Background()); err != nil {
		t.Fatalf("Delete via CloseListenKey: %v", err)
	}
}

func TestClient_Put(t *testing.T) {
	client := requireBinancePerpLiveWrite(t)
	if err := client.KeepAliveListenKey(context.Background()); err != nil {
		t.Fatalf("Put via KeepAliveListenKey: %v", err)
	}
}
