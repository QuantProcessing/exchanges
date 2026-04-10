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

func TestSpotWatchOrderBookUsesSnapshotAndWSUpdates(t *testing.T) {
	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, &bybitStubClient{
		getInstrumentsFn: func(_ context.Context, category string) ([]sdk.Instrument, error) {
			require.Equal(t, categorySpot, category)
			return []sdk.Instrument{testSpotInstrument()}, nil
		},
		getOrderBookFn: func(_ context.Context, category, symbol string, limit int) (*sdk.OrderBook, error) {
			require.Equal(t, categorySpot, category)
			require.Equal(t, "BTCUSDT", symbol)
			return &sdk.OrderBook{
				Symbol: "BTCUSDT",
				Bids:   [][]sdk.NumberString{{"49998", "0.5"}},
				Asks:   [][]sdk.NumberString{{"50002", "0.4"}},
				TS:     1710000000000,
			}, nil
		},
	})
	require.NoError(t, err)

	adp.publicWS = &stubPublicWSClient{
		subscribeFn: func(_ context.Context, topic string, handler func(json.RawMessage)) error {
			require.Equal(t, "orderbook.50.BTCUSDT", topic)
			payload := json.RawMessage(`{"topic":"orderbook.50.BTCUSDT","type":"snapshot","ts":1710000000001,"data":{"s":"BTCUSDT","b":[["49999","0.8"]],"a":[["50001","1.2"]],"u":11,"seq":12,"cts":1710000000001}}`)
			handler(payload)
			return nil
		},
		unsubscribeFn: func(_ context.Context, topic string) error {
			require.Equal(t, "orderbook.50.BTCUSDT", topic)
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = adp.WatchOrderBook(ctx, "BTC", 10, nil)
	require.NoError(t, err)

	book := adp.GetLocalOrderBook("BTC", 10)
	require.NotNil(t, book)
	require.Len(t, book.Bids, 1)
	require.Equal(t, "49999", book.Bids[0].Price.String())

	require.NoError(t, adp.StopWatchOrderBook(context.Background(), "BTC"))
	require.Nil(t, adp.GetLocalOrderBook("BTC", 10))
}

type stubPublicWSClient struct {
	subscribeFn   func(context.Context, string, func(json.RawMessage)) error
	unsubscribeFn func(context.Context, string) error
}

func (c *stubPublicWSClient) Subscribe(ctx context.Context, topic string, handler func(json.RawMessage)) error {
	if c.subscribeFn == nil {
		panic("unexpected Subscribe call")
	}
	return c.subscribeFn(ctx, topic, handler)
}

func (c *stubPublicWSClient) Unsubscribe(ctx context.Context, topic string) error {
	if c.unsubscribeFn == nil {
		panic("unexpected Unsubscribe call")
	}
	return c.unsubscribeFn(ctx, topic)
}

func (c *stubPublicWSClient) Close() error { return nil }
