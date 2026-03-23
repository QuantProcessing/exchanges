package bitget

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBuildMarketCacheSeparatesSpotAndPerp(t *testing.T) {
	instruments := []sdk.Instrument{
		{
			Symbol:    "BTCUSDT",
			Category:  categorySpot,
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
			Status:    "online",
		},
		{
			Symbol:    "BTCUSDT",
			Category:  categoryUSDTFutures,
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
			Status:    "online",
		},
		{
			Symbol:    "BTCUSDC",
			Category:  categoryUSDCFutures,
			BaseCoin:  "BTC",
			QuoteCoin: "USDC",
			Status:    "online",
		},
	}

	cache := buildMarketCache(instruments, exchanges.QuoteCurrencyUSDT)
	require.Equal(t, "BTCUSDT", cache.spotByBase["BTC"].Symbol)
	require.Equal(t, categoryUSDTFutures, cache.perpByBase["BTC"].Category)
}

func TestSymbolDetailsFromInstrumentUsesDeclaredPrecisions(t *testing.T) {
	details, err := symbolDetailsFromInstrument(sdk.Instrument{
		Symbol:            "BTCUSDT",
		Category:          categorySpot,
		BaseCoin:          "BTC",
		QuoteCoin:         "USDT",
		MinOrderQty:       "0.0001",
		MinOrderAmount:    "5",
		PricePrecision:    "2",
		QuantityPrecision: "4",
	})
	require.NoError(t, err)
	require.Equal(t, "BTC", details.Symbol)
	require.EqualValues(t, 2, details.PricePrecision)
	require.EqualValues(t, 4, details.QuantityPrecision)
	require.True(t, details.MinQuantity.Equal(decimal.RequireFromString("0.0001")))
	require.True(t, details.MinNotional.Equal(decimal.RequireFromString("5")))
}

func TestKlineIntervalStringMapsBitgetIntervals(t *testing.T) {
	value, err := klineIntervalString(exchanges.Interval1h)
	require.NoError(t, err)
	require.Equal(t, "1H", value)

	value, err = klineIntervalString(exchanges.Interval1d)
	require.NoError(t, err)
	require.Equal(t, "1D", value)
}
