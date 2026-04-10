package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestTradeWSRequestHonorsCallerContext(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Contains(t, string(payload), `"op":"auth"`)
		require.NoError(t, conn.WriteJSON(map[string]any{"op": "auth", "retCode": 0, "retMsg": "OK"}))

		_, payload, err = conn.ReadMessage()
		require.NoError(t, err)
		require.Contains(t, string(payload), `"op":"order.create"`)
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	client := NewTradeWSClient().WithCredentials("api-key", "secret-key")
	client.url = "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := client.PlaceOrder(ctx, PlaceOrderRequest{
		Category:    "spot",
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		OrderType:   "Limit",
		Qty:         "0.1",
		Price:       "100",
		OrderLinkID: "cid-1",
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestTradeWSPendingRequestUnblocksOnDisconnect(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Contains(t, string(payload), `"op":"auth"`)
		require.NoError(t, conn.WriteJSON(map[string]any{"op": "auth", "retCode": 0, "retMsg": "OK"}))

		_, payload, err = conn.ReadMessage()
		require.NoError(t, err)
		require.Contains(t, string(payload), `"op":"order.cancel"`)
		_ = conn.Close()
	}))
	defer server.Close()

	client := NewTradeWSClient().WithCredentials("api-key", "secret-key")
	client.url = "ws" + strings.TrimPrefix(server.URL, "http")

	start := time.Now()
	err := client.CancelOrder(context.Background(), CancelOrderRequest{
		Category: "spot",
		Symbol:   "BTCUSDT",
		OrderID:  "1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection closed")
	require.Less(t, time.Since(start), 2*time.Second)
}
