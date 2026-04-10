package bybit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bybit/sdk"
	"github.com/stretchr/testify/require"
)

func TestSpotWatchOrdersUsesPrivateWS(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
	})
	require.NoError(t, err)

	updates := make(chan *exchanges.Order, 1)
	adp.privateWS = &stubPrivateWSClient{
		subscribeFn: func(_ context.Context, topic string, handler func(json.RawMessage)) error {
			require.Equal(t, "order.spot", topic)
			handler(json.RawMessage(`{"topic":"order.spot","data":[{"orderId":"1","orderLinkId":"cid-1","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","timeInForce":"GTC","price":"50000","qty":"0.1","cumExecQty":"0","avgPrice":"0","orderStatus":"New","reduceOnly":false,"createdTime":"1710000000000","updatedTime":"1710000000001"}]}`))
			return nil
		},
		unsubscribeFn: func(_ context.Context, topic string) error {
			require.Equal(t, "order.spot", topic)
			return nil
		},
	}

	err = adp.WatchOrders(context.Background(), func(order *exchanges.Order) {
		updates <- order
	})
	require.NoError(t, err)

	select {
	case update := <-updates:
		require.Equal(t, "1", update.OrderID)
		require.Equal(t, exchanges.OrderStatusNew, update.Status)
	case <-time.After(time.Second):
		t.Fatal("expected order update")
	}

	require.NoError(t, adp.StopWatchOrders(context.Background()))
}

func TestPerpWatchPositionsUsesPrivateWS(t *testing.T) {
	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categoryLinear, category)
			return []sdk.Instrument{testLinearInstrument()}, nil
		},
	})
	require.NoError(t, err)

	updates := make(chan *exchanges.Position, 1)
	adp.privateWS = &stubPrivateWSClient{
		subscribeFn: func(_ context.Context, topic string, handler func(json.RawMessage)) error {
			require.Equal(t, "position.linear", topic)
			handler(json.RawMessage(`{"topic":"position.linear","data":[{"symbol":"BTCUSDT","side":"Buy","size":"0.5","avgPrice":"50000","leverage":"10","unrealisedPnl":"100","cumRealisedPnl":"5","liqPrice":"45000"}]}`))
			return nil
		},
		unsubscribeFn: func(_ context.Context, topic string) error {
			require.Equal(t, "position.linear", topic)
			return nil
		},
	}

	err = adp.WatchPositions(context.Background(), func(position *exchanges.Position) {
		updates <- position
	})
	require.NoError(t, err)

	select {
	case update := <-updates:
		require.Equal(t, "BTC", update.Symbol)
		require.Equal(t, exchanges.PositionSideLong, update.Side)
	case <-time.After(time.Second):
		t.Fatal("expected position update")
	}

	require.NoError(t, adp.StopWatchPositions(context.Background()))
}

func TestMapExecutionFillReturnsExecutionDetails(t *testing.T) {
	fill := mapExecutionFill("BTC", sdk.ExecutionRecord{
		ExecID:      "trade-1",
		OrderID:     "order-1",
		OrderLinkID: "cid-1",
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		ExecPrice:   "51000.5",
		ExecQty:     "0.01",
		ExecFee:     "0.18",
		FeeCurrency: "USDT",
		IsMaker:     false,
		ExecTime:    "1703577336606",
	})

	require.Equal(t, "trade-1", fill.TradeID)
	require.Equal(t, "order-1", fill.OrderID)
	require.Equal(t, "cid-1", fill.ClientOrderID)
	require.Equal(t, "BTC", fill.Symbol)
	require.Equal(t, exchanges.OrderSideBuy, fill.Side)
	require.Equal(t, "51000.5", fill.Price.String())
}

type stubPrivateWSClient struct {
	subscribeFn   func(context.Context, string, func(json.RawMessage)) error
	unsubscribeFn func(context.Context, string) error
}

func (c *stubPrivateWSClient) Subscribe(ctx context.Context, topic string, handler func(json.RawMessage)) error {
	if c.subscribeFn == nil {
		panic("unexpected Subscribe call")
	}
	return c.subscribeFn(ctx, topic, handler)
}

func (c *stubPrivateWSClient) Unsubscribe(ctx context.Context, topic string) error {
	if c.unsubscribeFn == nil {
		panic("unexpected Unsubscribe call")
	}
	return c.unsubscribeFn(ctx, topic)
}

func (c *stubPrivateWSClient) Close() error { return nil }
