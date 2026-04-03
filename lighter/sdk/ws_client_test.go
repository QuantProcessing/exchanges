package lighter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

func TestWSClientHandleIncomingFrameDispatchesJSONTextOrderBook(t *testing.T) {
	client := NewWebsocketClient(context.Background())

	var got WsOrderBookEvent
	require.NoError(t, client.registerTypedSubscription("order_book/7", nil, func(env *Envelope) error {
		return env.Unmarshal(&got)
	}))

	msg := []byte(`{"channel":"order_book:7","type":"update/order_book","timestamp":1700000000000,"order_book":{"nonce":9,"begin_nonce":8,"asks":[{"price":"1.2","size":"3"}],"bids":[]}}`)
	require.NoError(t, client.handleIncomingFrame(websocket.TextMessage, msg))

	require.Equal(t, "order_book:7", got.Channel)
	require.Equal(t, int64(9), got.OrderBook.Nonce)
	require.Equal(t, int64(8), got.OrderBook.BeginNonce)
	require.Len(t, got.OrderBook.Asks, 1)
}

func TestWSClientHandleIncomingFrameDispatchesMsgpackBinaryTrade(t *testing.T) {
	client := NewWebsocketClient(context.Background())

	var got WsTradeEvent
	require.NoError(t, client.registerTypedSubscription("trade/4", nil, func(env *Envelope) error {
		return env.Unmarshal(&got)
	}))

	raw, err := msgpack.Marshal(map[string]any{
		"channel": "trade:4",
		"type":    "update/trade",
		"nonce":   int64(5),
		"trades": []map[string]any{{
			"trade_id": int64(12),
			"price":    "2000.5",
			"size":     "0.3",
		}},
	})
	require.NoError(t, err)

	require.NoError(t, client.handleIncomingFrame(websocket.BinaryMessage, raw))
	require.Len(t, got.Trades, 1)
	require.Equal(t, int64(12), got.Trades[0].TradeId)
	require.Equal(t, "2000.5", got.Trades[0].Price)
}

func TestWSClientRawCallbackReceivesNormalizedJSONAfterMsgpackDecode(t *testing.T) {
	client := NewWebsocketClient(context.Background())

	var got map[string]any
	require.NoError(t, client.registerRawSubscription("height", nil, func(data []byte) {
		require.NoError(t, json.Unmarshal(data, &got))
	}))

	raw, err := msgpack.Marshal(map[string]any{
		"channel": "height",
		"type":    "update/height",
		"height":  int64(123),
	})
	require.NoError(t, err)

	require.NoError(t, client.handleIncomingFrame(websocket.BinaryMessage, raw))
	require.Equal(t, "height", got["channel"])
	require.Equal(t, "update/height", got["type"])
	require.Equal(t, float64(123), got["height"])
}

func TestWSClientBuildURLIncludesReadonlyAndEncoding(t *testing.T) {
	client := NewWebsocketClientWithConfig(context.Background(), WSConfig{
		URL:      MainnetWSURL,
		ReadOnly: true,
		Encoding: WSEncodingMsgpack,
	})

	got, err := client.buildURL()
	require.NoError(t, err)
	require.Equal(t, "wss://mainnet.zklighter.elliot.ai/stream?encoding=msgpack&readonly=true", got)
}

func TestWSClientPingLoopSendsControlPing(t *testing.T) {
	client := NewWebsocketClientWithConfig(context.Background(), WSConfig{
		KeepaliveInterval: 5 * time.Millisecond,
	})
	rec := &recordingConn{}
	client.conn = rec

	go client.pingLoop()
	time.Sleep(20 * time.Millisecond)
	client.Close()

	require.GreaterOrEqual(t, rec.pingCount, 1)
	require.Zero(t, rec.jsonCount)
}

type recordingConn struct {
	pingCount int
	jsonCount int
}

func (c *recordingConn) ReadMessage() (int, []byte, error) {
	return 0, nil, errors.New("not implemented")
}

func (c *recordingConn) WriteJSON(v interface{}) error {
	c.jsonCount++
	return nil
}

func (c *recordingConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	if messageType == websocket.PingMessage {
		c.pingCount++
	}
	return nil
}

func (c *recordingConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *recordingConn) Close() error {
	return nil
}
