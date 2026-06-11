package bitget

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/sdk/bitget"
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
	require.Equal(t, "BTCUSDT", cache.spotByBase["BTC/USDT"].Symbol)
	require.Equal(t, "BTCUSDC", cache.perpByBase["BTC/USDC"].Symbol)
	require.Equal(t, categoryUSDTFutures, cache.perpByBase["BTC"].Category)
}

func TestBuildMarketCacheSupportsMultipleQuotes(t *testing.T) {
	instruments := []sdk.Instrument{
		{
			Symbol:    "BTCUSDT",
			Category:  categorySpot,
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
			Status:    "online",
		},
		{
			Symbol:    "BTCUSDC",
			Category:  categorySpot,
			BaseCoin:  "BTC",
			QuoteCoin: "USDC",
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

	usdtCache := buildMarketCache(instruments, exchanges.QuoteCurrencyUSDT)
	require.Equal(t, "BTCUSDT", usdtCache.spotByBase["BTC"].Symbol)
	require.Equal(t, "BTCUSDT", usdtCache.spotByBase["BTC/USDT"].Symbol)
	require.Equal(t, "BTCUSDC", usdtCache.spotByBase["BTC/USDC"].Symbol)
	require.Equal(t, categoryUSDTFutures, usdtCache.perpByBase["BTC"].Category)
	require.Equal(t, categoryUSDCFutures, usdtCache.perpByBase["BTC/USDC"].Category)
	require.Contains(t, usdtCache.bySymbol, "BTCUSDC")

	usdcCache := buildMarketCache(instruments, exchanges.QuoteCurrencyUSDC)
	require.Equal(t, "BTCUSDC", usdcCache.spotByBase["BTC"].Symbol)
	require.Equal(t, "BTCUSDT", usdcCache.spotByBase["BTC/USDT"].Symbol)
	require.Equal(t, "BTCUSDC", usdcCache.spotByBase["BTC/USDC"].Symbol)
	require.Equal(t, categoryUSDCFutures, usdcCache.perpByBase["BTC"].Category)
	require.Equal(t, categoryUSDTFutures, usdcCache.perpByBase["BTC/USDT"].Category)
	require.Contains(t, usdcCache.bySymbol, "BTCUSDT")
}

func TestMarketCatalogFormatsAndExtractsInstrumentSymbols(t *testing.T) {
	catalog := buildMarketCache([]sdk.Instrument{
		{
			Symbol:    "BTCUSDT",
			Category:  categorySpot,
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
			Status:    "online",
		},
		{
			Symbol:    "BTCUSDC",
			Category:  categorySpot,
			BaseCoin:  "BTC",
			QuoteCoin: "USDC",
			Status:    "online",
		},
		{
			Symbol:    "ETHUSDC",
			Category:  categoryUSDCFutures,
			BaseCoin:  "ETH",
			QuoteCoin: "USDC",
			Status:    "online",
		},
	}, exchanges.QuoteCurrencyUSDT)

	require.Equal(t, "BTCUSDT", catalog.FormatSymbol("BTC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypeSpot))
	require.Equal(t, "BTCUSDC", catalog.FormatSymbol("BTC/USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypeSpot))
	require.Equal(t, "ETHUSDC", catalog.FormatSymbol("ETH/USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp))
	require.Equal(t, "BTC/USDT", catalog.ExtractSymbol("BTCUSDT", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypeSpot))
	require.Equal(t, "ETH/USDC", catalog.ExtractSymbol("ETHUSDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp))
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
