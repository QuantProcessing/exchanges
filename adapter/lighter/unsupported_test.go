package lighter

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPerpUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &Adapter{}

	require.ErrorIs(t, adp.SetLeverage(context.Background(), "BTC", 2), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
}

func TestSpotUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &SpotAdapter{}

	_, err := adp.FetchKlines(context.Background(), "BTC", exchanges.Interval1m, nil)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchPositions(context.Background(), nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchPositions(context.Background()), exchanges.ErrNotSupported)
}
