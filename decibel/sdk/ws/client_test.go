package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type fakeConn struct {
	mu     sync.Mutex
	reads  chan any
	writes []any
	closed bool
	readsN int
}

func newFakeConn() *fakeConn {
	return &fakeConn{
		reads: make(chan any, 8),
	}
}

func (c *fakeConn) ReadJSON(v any) error {
	c.mu.Lock()
	c.readsN++
	c.mu.Unlock()

	msg, ok := <-c.reads
	if !ok {
		return errors.New("closed")
	}
	if err, ok := msg.(error); ok {
		return err
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func (c *fakeConn) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, v)
	return nil
}

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.reads)
	}
	return nil
}

func (c *fakeConn) snapshotWrites() []any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]any, len(c.writes))
	copy(out, c.writes)
	return out
}

func (c *fakeConn) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readsN
}

func TestDecibelWSConnectUsesExpectedURLAndHeaders(t *testing.T) {
	var (
		gotURL    string
		gotHeader http.Header
	)

	conn := newFakeConn()
	client := NewClient(context.Background(), "test-api-key")
	client.URL = TestnetWSURL
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		gotURL = url
		gotHeader = header.Clone()
		return conn, nil
	}

	require.NoError(t, client.Connect())
	require.Equal(t, TestnetWSURL, gotURL)
	require.Equal(t, "Bearer test-api-key", gotHeader.Get("Authorization"))
	require.NotEmpty(t, gotHeader.Get("Origin"))
}

func TestDecibelWSConnectIsIdempotentForExistingConnection(t *testing.T) {
	conn := newFakeConn()
	var dialCount int

	client := NewClient(context.Background(), "test-api-key")
	client.PingInterval = 0
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		dialCount++
		return conn, nil
	}

	require.NoError(t, client.Connect())
	require.Eventually(t, func() bool {
		return conn.readCount() == 1
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, client.Connect())
	time.Sleep(50 * time.Millisecond)

	require.Equal(t, 1, dialCount)
	require.Equal(t, 1, conn.readCount())
}

func TestDecibelWSReplaysStoredSubscriptionsOnFirstConnect(t *testing.T) {
	conn := newFakeConn()

	client := NewClient(context.Background(), "test-api-key")
	client.PingInterval = 0
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		return conn, nil
	}

	require.NoError(t, client.Subscribe("depth:0xbtc:1", func(MarketDepthMessage) {}))
	require.NoError(t, client.SubscribeUserPositions("0xuser", func(UserPositionsMessage) {}))
	require.NoError(t, client.Connect())

	require.Eventually(t, func() bool {
		return len(conn.snapshotWrites()) == 2
	}, time.Second, 10*time.Millisecond)

	writes := conn.snapshotWrites()
	require.Len(t, writes, 2)
	topics := make(map[string]bool, len(writes))
	for _, write := range writes {
		msg, ok := write.(subscriptionRequest)
		require.True(t, ok)
		topics[msg.Subscribe.Topic] = true
	}
	require.True(t, topics["depth:0xbtc:1"])
	require.True(t, topics["user_positions:0xuser"])
}

func TestDecibelWSReplaysSubscriptionsAfterReconnect(t *testing.T) {
	conn1 := newFakeConn()
	conn2 := newFakeConn()

	var dialCount int
	client := NewClient(context.Background(), "test-api-key")
	client.ReconnectWait = 0
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		dialCount++
		if dialCount == 1 {
			return conn1, nil
		}
		return conn2, nil
	}

	require.NoError(t, client.Connect())
	require.NoError(t, client.Subscribe("depth:0xbtc:1", func(MarketDepthMessage) {}))

	conn1.reads <- errors.New("boom")

	require.Eventually(t, func() bool {
		return dialCount >= 2
	}, 2*time.Second, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		return len(conn2.snapshotWrites()) >= 1
	}, 2*time.Second, 20*time.Millisecond)

	writes := conn2.snapshotWrites()
	require.Len(t, writes, 1)

	msg, ok := writes[0].(subscriptionRequest)
	require.True(t, ok)
	require.Equal(t, "depth:0xbtc:1", msg.Subscribe.Topic)
}

func TestDecibelWSDispatchesTypedUserPositionsEvents(t *testing.T) {
	conn := newFakeConn()
	client := NewClient(context.Background(), "test-api-key")
	client.PingInterval = 0
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		return conn, nil
	}

	events := make(chan UserPositionsMessage, 1)

	require.NoError(t, client.Connect())
	require.NoError(t, client.SubscribeUserPositions("0xuser", func(msg UserPositionsMessage) {
		events <- msg
	}))

	conn.reads <- UserPositionsMessage{
		Topic: "user_positions:0xuser",
		Positions: []Position{
			{
				Market:     "0xbtc",
				Size:       decimal.RequireFromString("1.5"),
				EntryPrice: decimal.RequireFromString("50000"),
			},
		},
	}

	select {
	case msg := <-events:
		require.Equal(t, "user_positions:0xuser", msg.Topic)
		require.Len(t, msg.Positions, 1)
		require.Equal(t, "0xbtc", msg.Positions[0].Market)
		require.True(t, decimal.RequireFromString("1.5").Equal(msg.Positions[0].Size))
		require.True(t, decimal.RequireFromString("50000").Equal(msg.Positions[0].EntryPrice))
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for positions event")
	}
}

func TestDecibelWSDispatchesNormalizedOrderHistoryStatuses(t *testing.T) {
	conn := newFakeConn()
	client := NewClient(context.Background(), "test-api-key")
	client.PingInterval = 0
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		return conn, nil
	}

	events := make(chan UserOrderHistoryMessage, 1)

	require.NoError(t, client.Connect())
	require.NoError(t, client.SubscribeUserOrderHistory("0xuser", func(msg UserOrderHistoryMessage) {
		events <- msg
	}))

	conn.reads <- UserOrderHistoryMessage{
		Topic: "user_order_history:0xuser",
		Orders: []OrderHistoryItem{
			{OrderID: "1", Status: "Filled"},
			{OrderID: "2", Status: "Rejected"},
			{OrderID: "3", Status: "mystery"},
		},
	}

	select {
	case msg := <-events:
		require.Len(t, msg.Orders, 3)
		require.Equal(t, exchanges.OrderStatusFilled, msg.Orders[0].NormalizedStatus)
		require.Equal(t, exchanges.OrderStatusRejected, msg.Orders[1].NormalizedStatus)
		require.Equal(t, exchanges.OrderStatusUnknown, msg.Orders[2].NormalizedStatus)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for order history event")
	}
}

func TestDecibelWSDispatchesNormalizedOrderUpdates(t *testing.T) {
	conn := newFakeConn()
	client := NewClient(context.Background(), "test-api-key")
	client.PingInterval = 0
	client.dial = func(ctx context.Context, url string, header http.Header) (wsConn, error) {
		return conn, nil
	}

	events := make(chan OrderUpdateMessage, 1)

	require.NoError(t, client.Connect())
	require.NoError(t, client.SubscribeOrderUpdates("0xuser", func(msg OrderUpdateMessage) {
		events <- msg
	}))

	conn.reads <- OrderUpdateMessage{
		Topic: "order_updates:0xuser",
		Order: OrderUpdateRecord{
			Status: "Open",
			Order: OrderUpdateItem{
				OrderID:       "1",
				ClientOrderID: "cli-1",
				Market:        "0xbtc",
				OrderType:     "limit",
				Status:        "Open",
				IsBuy:         true,
			},
		},
	}

	select {
	case msg := <-events:
		require.Equal(t, exchanges.OrderStatusNew, msg.Order.NormalizedStatus)
		require.Equal(t, "1", msg.Order.Order.OrderID)
		require.Equal(t, "cli-1", msg.Order.Order.ClientOrderID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for order update event")
	}
}
