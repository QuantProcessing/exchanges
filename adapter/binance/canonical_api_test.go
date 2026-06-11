package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestCanonicalVenueAdapterAPIUsesPrimaryNames(t *testing.T) {
	adapter, err := NewVenueAdapter(context.Background(), VenueOptions{AccountID: "acct"})
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, adapter.Venue())
	require.Equal(t, model.AccountID("acct"), adapter.Execution().AccountID())

	caps := DeclaredCapabilities()
	require.Equal(t, model.VenueBinance, caps.Venue)
	require.True(t, caps.MarketData.Ticker)
}
