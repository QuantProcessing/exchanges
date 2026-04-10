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

func TestPrivateWSReconnectsAndResubscribes(t *testing.T) {
	var connections atomic.Int32
	var subscribes atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Contains(t, string(payload), `"op":"auth"`)
		require.NoError(t, conn.WriteJSON(map[string]any{"op": "auth", "success": true}))

		switch connections.Add(1) {
		case 1:
			_, payload, err = conn.ReadMessage()
			require.NoError(t, err)
			require.Contains(t, string(payload), `"op":"subscribe"`)
			subscribes.Add(1)
			_ = conn.Close()
		default:
			_, payload, err = conn.ReadMessage()
			require.NoError(t, err)
			require.Contains(t, string(payload), `"op":"subscribe"`)
			subscribes.Add(1)
			require.NoError(t, conn.WriteJSON(struct {
				Topic string            `json:"topic"`
				Data  []ExecutionRecord `json:"data"`
			}{
				Topic: "execution.spot",
				Data: []ExecutionRecord{{
					ExecID:   "1",
					Symbol:   "BTCUSDT",
					ExecTime: "1",
				}},
			}))
		}
	}))
	defer server.Close()

	client := NewPrivateWSClient().WithCredentials("api-key", "secret-key")
	client.url = "ws" + strings.TrimPrefix(server.URL, "http")
	client.ctx, client.cancel = context.WithCancel(context.Background())
	defer client.cancel()

	got := make(chan json.RawMessage, 1)
	err := client.Subscribe(context.Background(), "execution.spot", func(payload json.RawMessage) {
		got <- payload
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return subscribes.Load() >= 2
	}, 5*time.Second, 100*time.Millisecond)

	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("expected payload after reconnect")
	}
}
