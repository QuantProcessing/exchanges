package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestVenueRegistryOpensBinanceAdapter(t *testing.T) {
	got, err := venue.Open(context.Background(), model.VenueBinance, map[string]string{
		"account_id":     "binance-main",
		"quote_currency": "USDT",
	})
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, got.Venue())
	require.Equal(t, model.AccountID("binance-main"), got.Execution().AccountID())
	require.Equal(t, model.VenueBinance, got.Capabilities().Venue)
}
