package sdk

import (
	"encoding/json"
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

func TestPrivateWSReturnsErrorFrameWithoutIDWhenSingleRequestPending(t *testing.T) {
	client := newTestPrivateWSClient(t, func(conn *websocket.Conn) {
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"error","code":40026,"msg":"User is disabled","ts":1774230104229}`)))
	})

	sender, ok := any(client).(timedRequestSender)
	require.True(t, ok, "PrivateWSClient should implement request timeouts")

	_, err := sender.sendRequestWithTimeout("req-error", map[string]any{"id": "req-error"}, 50*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "40026")
	require.Contains(t, err.Error(), "User is disabled")
}

func TestPlaceClassicPerpOrderWSOmitsFalseReduceOnly(t *testing.T) {
	client := newTestPrivateWSClient(t, func(conn *websocket.Conn) {
		_, payload, err := conn.ReadMessage()
		require.NoError(t, err)

		var req struct {
			Args []struct {
				ID     string         `json:"id"`
				Params map[string]any `json:"params"`
			} `json:"args"`
		}
		require.NoError(t, json.Unmarshal(payload, &req))
		require.NotEmpty(t, req.Args)
		_, exists := req.Args[0].Params["reduceOnly"]
		require.False(t, exists, "classic perp ws request should omit reduceOnly when false")

		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"trade","arg":[{"id":"`+req.Args[0].ID+`","instType":"USDT-FUTURES","channel":"place-order","instId":"BTCPERP","params":{"orderId":"123","clientOid":"abc"}}],"code":0,"msg":"Success"}`)))
	})

	resp, err := client.PlaceClassicPerpOrderWS(&PlaceOrderRequest{
		Symbol:     "BTCPERP",
		Qty:        "0.001",
		Side:       "buy",
		OrderType:  "market",
		ClientOID:  "abc",
		MarginCoin: "USDC",
		MarginMode: "crossed",
		TradeSide:  "open",
		ReduceOnly: "no",
	}, "USDT-FUTURES", "USDC")
	require.NoError(t, err)
	require.Equal(t, "123", resp.OrderID)
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
