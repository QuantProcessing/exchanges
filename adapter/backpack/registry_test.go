package backpack

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestLookupConstructorBuildsBackpackSpotAndPerp(t *testing.T) {
	ctor, err := exchanges.LookupConstructor("BACKPACK")
	require.NoError(t, err)
	require.NotNil(t, ctor)

	_, err = ctor(context.Background(), exchanges.MarketType("margin"), map[string]string{
		"quote_currency": "USDC",
	})
	require.Error(t, err)
}

func TestUnsupportedMethodsReturnErrNotSupported(t *testing.T) {
	adp := &SpotAdapter{}
	err := adp.TransferAsset(context.Background(), &exchanges.TransferParams{})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}

func TestSpotWatchPositionsReturnErrNotSupported(t *testing.T) {
	adp := &SpotAdapter{}
	err := adp.WatchPositions(context.Background(), func(*exchanges.Position) {})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)

	err = adp.StopWatchPositions(context.Background())
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}
