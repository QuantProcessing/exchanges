package standx

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestStandXUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &Adapter{BaseAdapter: exchanges.NewBaseAdapter("STANDX", exchanges.MarketTypePerp, exchanges.NopLogger)}

	_, err := adp.ModifyOrder(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	_, err = adp.FetchKlines(context.Background(), "BTC", exchanges.Interval1m, nil)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	require.ErrorIs(t, adp.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.StopWatchKlines(context.Background(), "BTC", exchanges.Interval1m), exchanges.ErrNotSupported)

	_, err = adp.FetchAllFundingRates(context.Background())
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
