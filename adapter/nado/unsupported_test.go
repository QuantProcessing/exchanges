package nado

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPerpUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &Adapter{}

	_, err := adp.ModifyOrder(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.SetLeverage(context.Background(), "BTC", 2), exchanges.ErrNotSupported)
}

func TestSpotUnsupportedPathsUseSentinelErrors(t *testing.T) {
	adp := &SpotAdapter{}

	require.ErrorIs(t, adp.TransferAsset(context.Background(), &exchanges.TransferParams{}), exchanges.ErrNotSupported)
	_, err := adp.ModifyOrder(context.Background(), "1", "BTC", &exchanges.ModifyOrderParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.SetLeverage(context.Background(), "BTC", 2), exchanges.ErrNotSupported)
	require.ErrorIs(t, adp.WatchKlines(context.Background(), "BTC", exchanges.Interval1m, nil), exchanges.ErrNotSupported)
}
