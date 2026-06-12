package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestCanonicalBinanceAdaptersAreAccountTypeSpecific(t *testing.T) {
	spotAdapter, err := NewSpotAdapter(context.Background(), Options{})
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, spotAdapter.Venue())
	require.Equal(t, defaultSpotAccountID, spotAdapter.Execution().AccountID())
	require.Contains(t, spotAdapter.Capabilities().InstrumentTypes, model.InstrumentTypeCurrencyPair)
	require.True(t, spotAdapter.Capabilities().MarketData.StreamTicker)
	require.True(t, spotAdapter.Capabilities().MarketData.StreamOrderBook)
	require.True(t, spotAdapter.Capabilities().MarketData.StreamTrades)
	require.True(t, spotAdapter.Capabilities().MarketData.StreamBars)

	perpAdapter, err := NewPerpAdapter(context.Background(), Options{})
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, perpAdapter.Venue())
	require.Equal(t, defaultPerpAccountID, perpAdapter.Execution().AccountID())
	require.Contains(t, perpAdapter.Capabilities().InstrumentTypes, model.InstrumentTypeCryptoPerp)
	require.True(t, perpAdapter.Capabilities().MarketData.StreamTicker)
	require.True(t, perpAdapter.Capabilities().MarketData.StreamOrderBook)
	require.True(t, perpAdapter.Capabilities().MarketData.StreamTrades)
	require.True(t, perpAdapter.Capabilities().MarketData.StreamBars)

	var _ venue.Adapter = spotAdapter
	var _ venue.Adapter = perpAdapter
}

func TestVenueRegistrySelectsBinanceAccountType(t *testing.T) {
	spotAdapter, err := venue.Open(context.Background(), model.VenueBinance, map[string]string{
		"account_type": "spot",
		"account_id":   "spot-acct",
	})
	require.NoError(t, err)
	require.Equal(t, model.AccountID("spot-acct"), spotAdapter.Execution().AccountID())
	require.Contains(t, spotAdapter.Capabilities().InstrumentTypes, model.InstrumentTypeCurrencyPair)

	perpAdapter, err := venue.Open(context.Background(), model.VenueBinance, map[string]string{
		"account_type": "usdt_futures",
		"account_id":   "perp-acct",
	})
	require.NoError(t, err)
	require.Equal(t, model.AccountID("perp-acct"), perpAdapter.Execution().AccountID())
	require.Contains(t, perpAdapter.Capabilities().InstrumentTypes, model.InstrumentTypeCryptoPerp)
}
