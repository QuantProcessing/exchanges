package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestNewV2AdapterWiresVenueClients(t *testing.T) {
	adapter, err := NewV2Adapter(context.Background(), V2Options{AccountID: "acct"})
	require.NoError(t, err)
	require.Equal(t, model.VenueBinance, adapter.Venue())
	require.NotNil(t, adapter.Instruments())
	require.NotNil(t, adapter.MarketData())
	require.NotNil(t, adapter.Execution())
	require.Equal(t, model.AccountID("acct"), adapter.Execution().AccountID())

	var _ venue.Adapter = adapter
}

func TestV2DeclaredCapabilitiesSeparateCertifiedLifecycleFromBasicStartup(t *testing.T) {
	caps := V2DeclaredCapabilities()
	require.Equal(t, model.VenueBinance, caps.Venue)
	require.Contains(t, caps.InstrumentTypes, model.InstrumentTypeCurrencyPair)
	require.Contains(t, caps.InstrumentTypes, model.InstrumentTypeCryptoPerp)
	require.True(t, caps.MarketData.Ticker)
	require.True(t, caps.MarketData.OrderBook)
	require.False(t, caps.MarketData.PrivateData)
	require.True(t, caps.Execution.Submit)
	require.True(t, caps.Execution.OrderReports)
	require.True(t, caps.Execution.FillReports)
	require.True(t, caps.AccountState.Snapshot)
	require.True(t, caps.Reconciliation.Startup)
	require.False(t, caps.Reconciliation.Reconnect)
}
