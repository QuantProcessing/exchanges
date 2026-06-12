package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestBinanceIndependentClientConstructors(t *testing.T) {
	ctx := context.Background()

	spotData, err := NewSpotDataClient(ctx, Options{})
	require.NoError(t, err)
	require.NotNil(t, spotData)
	var _ venue.DataClient = spotData

	perpData, err := NewPerpDataClient(ctx, Options{})
	require.NoError(t, err)
	require.NotNil(t, perpData)
	var _ venue.DataClient = perpData

	spotExec, err := NewSpotExecutionClient(ctx, Options{})
	require.NoError(t, err)
	require.NotNil(t, spotExec)
	var _ venue.ExecutionClient = spotExec

	perpExec, err := NewPerpExecutionClient(ctx, Options{})
	require.NoError(t, err)
	require.NotNil(t, perpExec)
	var _ venue.ExecutionClient = perpExec

	_, err = NewSpotExecutionClient(ctx, Options{APIKey: "key-only"})
	require.Error(t, err)
	_, err = NewPerpExecutionClient(ctx, Options{SecretKey: "secret-only"})
	require.Error(t, err)
}

func TestBinanceAdaptersUseIndependentConstructors(t *testing.T) {
	ctx := context.Background()

	spotAdapter, err := NewSpotAdapter(ctx, Options{})
	require.NoError(t, err)
	require.NotNil(t, spotAdapter.MarketData())
	require.NotNil(t, spotAdapter.Execution())

	perpAdapter, err := NewPerpAdapter(ctx, Options{})
	require.NoError(t, err)
	require.NotNil(t, perpAdapter.MarketData())
	require.NotNil(t, perpAdapter.Execution())
}
