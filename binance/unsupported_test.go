package binance

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestBinanceSpotUnsupportedPathsUseSentinelErrors(t *testing.T) {
	spot := &SpotAdapter{}
	margin := &MarginAdapter{}

	require.ErrorIs(t, spot.TransferAsset(context.Background(), &exchanges.TransferParams{}), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.WatchPositions(context.Background(), nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.StopWatchPositions(context.Background()), exchanges.ErrNotSupported)

	require.ErrorIs(t, margin.WatchOrders(context.Background(), nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.WatchPositions(context.Background(), nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.WatchTicker(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.WatchOrderBook(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.WatchTrades(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.StopWatchOrders(context.Background()), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.StopWatchPositions(context.Background()), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.StopWatchTicker(context.Background(), "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.StopWatchOrderBook(context.Background(), "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.StopWatchTrades(context.Background(), "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, margin.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
}
