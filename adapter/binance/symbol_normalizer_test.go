package binance

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestSymbolNormalizerSpot(t *testing.T) {
	n := symbolNormalizer{}
	got, err := n.ToInstrumentID("BTCUSDT", venue.ProductHintSpot)
	require.NoError(t, err)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), got)
	raw, err := n.ToVenueSymbol(got)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", raw)
}

func TestSymbolNormalizerPerp(t *testing.T) {
	n := symbolNormalizer{}
	got, err := n.ToInstrumentID("BTCUSDT", venue.ProductHintPerp)
	require.NoError(t, err)
	require.Equal(t, model.MustInstrumentID("BTC-USDT-PERP.BINANCE"), got)
	raw, err := n.ToVenueSymbol(got)
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", raw)
}
