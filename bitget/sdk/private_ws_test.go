package sdk

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type requestSender interface {
	sendRequest(id string, req any) ([]byte, error)
}

type timedRequestSender interface {
	sendRequestWithTimeout(id string, req any, timeout time.Duration) ([]byte, error)
}

func TestPrivateWSDispatchesTopLevelID(t *testing.T) {
	serverPayload := []byte(`{"event":"trade","id":"req-top","topic":"place-order","category":"spot","args":[{"symbol":"BTCUSDT","orderId":"123","clientOid":"abc"}],"code":"0","msg":"success"}`)
	client := newTestPrivateWSClient(t, func(conn *websocket.Conn) {
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, serverPayload))
	})

	sender, ok := any(client).(requestSender)
	require.True(t, ok, "PrivateWSClient should implement request/response sending")

	resp, err := sender.sendRequest("req-top", map[string]any{"id": "req-top"})
	require.NoError(t, err)
	require.JSONEq(t, string(serverPayload), string(resp))
}

func TestPrivateWSDispatchesClassicNestedID(t *testing.T) {
	serverPayload := []byte(`{"event":"trade","arg":[{"id":"req-classic","instType":"SPOT","channel":"place-order","instId":"BTCUSDT","params":{"orderId":"123","clientOid":"abc"}}],"code":"0","msg":"Success"}`)
	client := newTestPrivateWSClient(t, func(conn *websocket.Conn) {
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, serverPayload))
	})

	sender, ok := any(client).(requestSender)
	require.True(t, ok, "PrivateWSClient should implement request/response sending")

	resp, err := sender.sendRequest("req-classic", map[string]any{"op": "trade"})
	require.NoError(t, err)
	require.JSONEq(t, string(serverPayload), string(resp))
}

func TestPrivateWSRequestTimeoutCleansPending(t *testing.T) {
	client := newTestPrivateWSClient(t, func(conn *websocket.Conn) {
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
	})

	sender, ok := any(client).(timedRequestSender)
	require.True(t, ok, "PrivateWSClient should implement request timeouts")

	_, err := sender.sendRequestWithTimeout("req-timeout", map[string]any{"id": "req-timeout"}, 20*time.Millisecond)
	require.Error(t, err)

	pending := reflect.ValueOf(client).Elem().FieldByName("pendingRequests")
	require.True(t, pending.IsValid(), "PrivateWSClient should track pending requests")
	require.Equal(t, 0, pending.Len(), "timed out request should be removed from pending requests")
}

func newTestPrivateWSClient(t *testing.T, onMessage func(conn *websocket.Conn)) *PrivateWSClient {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		onMessage(conn)
	}))
	t.Cleanup(server.Close)

	url := "ws" + server.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	client := NewPrivateWSClient()
	client.conn = conn
	go client.readLoop(conn)

	t.Cleanup(func() {
		client.closed = true
		_ = conn.Close()
	})

	return client
}
