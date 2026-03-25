package decibel

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestDecibelUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &Adapter{BaseAdapter: exchanges.NewBaseAdapter("DECIBEL", exchanges.MarketTypePerp, exchanges.NopLogger)}
	adp.initRuntimeState()

	_, err := adp.ModifyOrder(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	require.ErrorIs(t, adp.SetLeverage(context.Background(), "BTC", 5), exchanges.ErrNotSupported)

	_, err = adp.FetchFundingRate(context.Background(), "BTC")
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	_, err = adp.FetchAllFundingRates(context.Background())
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	_, err = adp.FetchTrades(context.Background(), "BTC", 10)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	_, err = adp.FetchKlines(context.Background(), "BTC", exchanges.Interval1m, nil)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	require.ErrorIs(t, adp.WatchTicker(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchTrades(context.Background(), "BTC", nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTicker(context.Background(), "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchTrades(context.Background(), "BTC"), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchPositions(context.Background(), nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.CancelAllOrders(context.Background(), "BTC"), exchanges.ErrNotSupported)
}

func TestDecibelStopMethodsAreIdempotent(t *testing.T) {
	adp := &Adapter{BaseAdapter: exchanges.NewBaseAdapter("DECIBEL", exchanges.MarketTypePerp, exchanges.NopLogger)}
	adp.initRuntimeState()

	require.NoError(t, adp.StopWatchOrderBook(context.Background(), "BTC"))
	require.NoError(t, adp.StopWatchOrders(context.Background()))
	require.NoError(t, adp.Close())
}
