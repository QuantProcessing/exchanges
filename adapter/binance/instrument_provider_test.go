package binance

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestInstrumentProviderCachesSpotAndPerpSeparately(t *testing.T) {
	provider := newInstrumentProviderForTest([]instrumentSeed{
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintSpot, Base: model.BTC, Quote: model.USDT},
		{RawSymbol: "BTCUSDT", Product: venue.ProductHintPerp, Base: model.BTC, Quote: model.USDT},
	})

	require.NoError(t, provider.LoadAll(context.Background()))
	require.Len(t, provider.List(), 2)

	spot, ok := provider.Get(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"))
	require.True(t, ok)
	require.Equal(t, model.InstrumentTypeCurrencyPair, spot.Type)

	perp, ok := provider.Get(model.MustInstrumentID("BTC-USDT-PERP.BINANCE"))
	require.True(t, ok)
	require.Equal(t, model.InstrumentTypeCryptoPerp, perp.Type)
}

func TestInstrumentProviderLoadReturnsInstrumentNotLoaded(t *testing.T) {
	provider := newInstrumentProviderForTest(nil)
	_, err := provider.Load(context.Background(), model.MustInstrumentID("ETH-USDT-PERP.BINANCE"))
	require.ErrorIs(t, err, model.ErrInstrumentNotLoaded)
}
