package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestPublicWSReconnectsAndResubscribes(t *testing.T) {
	var connections atomic.Int32
	var subscribes atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		switch connections.Add(1) {
		case 1:
			_, payload, err := conn.ReadMessage()
			require.NoError(t, err)
			require.Contains(t, string(payload), `"op":"subscribe"`)
			subscribes.Add(1)
			_ = conn.Close()
		default:
			_, payload, err := conn.ReadMessage()
			require.NoError(t, err)
			require.Contains(t, string(payload), `"op":"subscribe"`)
			subscribes.Add(1)
		}
	}))
	defer server.Close()

	client := NewPublicWSClient("spot")
	client.url = "ws" + strings.TrimPrefix(server.URL, "http")
	client.ctx, client.cancel = context.WithCancel(context.Background())
	defer client.cancel()

	err := client.Subscribe(context.Background(), "orderbook.50.BTCUSDT", func(_ json.RawMessage) {})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return subscribes.Load() >= 2
	}, 5*time.Second, 100*time.Millisecond)
}
