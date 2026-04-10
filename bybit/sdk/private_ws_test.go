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

func TestPrivateWSAuthHonorsCallerContext(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Contains(t, string(payload), `"op":"auth"`)
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	client := NewPrivateWSClient().WithCredentials("api-key", "secret-key")
	client.url = "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := client.Connect(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 2*time.Second)
}
