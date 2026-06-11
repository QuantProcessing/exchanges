package okx

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestOKXUnsupportedPathsUseSentinelErrors(t *testing.T) {
	spot := &SpotAdapter{}
	perp := &Adapter{}

	require.ErrorIs(t, spot.TransferAsset(context.Background(), &exchanges.TransferParams{}), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.WatchPositions(context.Background(), nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.StopWatchPositions(context.Background()), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.WatchTrades(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, spot.StopWatchTrades(context.Background(), "BTC"), exchanges.ErrNotSupported)

	require.ErrorIs(t, perp.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, perp.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
	require.ErrorIs(t, perp.WatchTrades(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, perp.StopWatchTrades(context.Background(), "BTC"), exchanges.ErrNotSupported)
}
