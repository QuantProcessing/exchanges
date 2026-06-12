package bybit

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/sdk/bybit"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBuildMarketCacheFiltersByQuoteAndTradingStatus(t *testing.T) {
	instruments := []sdk.Instrument{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
			Status:    instrumentStatusTrading,
		},
		{
			Symbol:    "BTCUSDC",
			BaseCoin:  "BTC",
			QuoteCoin: "USDC",
			Status:    instrumentStatusTrading,
		},
		{
			Symbol:    "ETHUSDT",
			BaseCoin:  "ETH",
			QuoteCoin: "USDT",
			Status:    "PreLaunch",
		},
	}

	cache := buildMarketCache(instruments, exchanges.QuoteCurrencyUSDT)
	require.Equal(t, "BTCUSDT", cache.byBase["BTC"].Symbol)
	require.Equal(t, "BTCUSDT", cache.byBase["BTC/USDT"].Symbol)
	require.Equal(t, "BTCUSDC", cache.byBase["BTC/USDC"].Symbol)
	_, hasETH := cache.byBase["ETH"]
	require.False(t, hasETH)
	require.Contains(t, cache.bySymbol, "BTCUSDC")
}

func TestBuildMarketCacheSupportsMultipleQuotes(t *testing.T) {
	instruments := []sdk.Instrument{
		{
			Symbol:    "BTCUSDT",
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
			Status:    instrumentStatusTrading,
		},
		{
			Symbol:    "BTCUSDC",
			BaseCoin:  "BTC",
			QuoteCoin: "USDC",
			Status:    instrumentStatusTrading,
		},
	}

	usdtCache := buildMarketCache(instruments, exchanges.QuoteCurrencyUSDT)
	require.Equal(t, "BTCUSDT", usdtCache.byBase["BTC"].Symbol)
	require.Equal(t, "BTCUSDT", usdtCache.byBase["BTC/USDT"].Symbol)
	require.Equal(t, "BTCUSDC", usdtCache.byBase["BTC/USDC"].Symbol)
	require.Contains(t, usdtCache.bySymbol, "BTCUSDC")

	usdcCache := buildMarketCache(instruments, exchanges.QuoteCurrencyUSDC)
	require.Equal(t, "BTCUSDC", usdcCache.byBase["BTC"].Symbol)
	require.Equal(t, "BTCUSDT", usdcCache.byBase["BTC/USDT"].Symbol)
	require.Equal(t, "BTCUSDC", usdcCache.byBase["BTC/USDC"].Symbol)
	require.Contains(t, usdcCache.bySymbol, "BTCUSDT")
}

func TestSymbolDetailsFromInstrumentUsesTickSizeAndQtyStep(t *testing.T) {
	details, err := symbolDetailsFromInstrument(sdk.Instrument{
		Symbol:     "BTCUSDT",
		BaseCoin:   "BTC",
		QuoteCoin:  "USDT",
		PriceScale: "2",
		PriceFilter: sdk.PriceFilter{
			TickSize: "0.01",
		},
		LotSizeFilter: sdk.LotSizeFilter{
			BasePrecision:    "0.001",
			MinOrderQty:      "0.001",
			MinNotionalValue: "5",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "BTC", details.Symbol)
	require.EqualValues(t, 2, details.PricePrecision)
	require.EqualValues(t, 3, details.QuantityPrecision)
	require.True(t, details.MinQuantity.Equal(decimal.RequireFromString("0.001")))
	require.True(t, details.MinNotional.Equal(decimal.RequireFromString("5")))
}

func TestSymbolDetailsFromSpotInstrumentUsesMinOrderAmt(t *testing.T) {
	details, err := symbolDetailsFromInstrument(sdk.Instrument{
		Symbol:     "SOLUSDT",
		BaseCoin:   "SOL",
		QuoteCoin:  "USDT",
		PriceScale: "0",
		PriceFilter: sdk.PriceFilter{
			TickSize: "0.01",
		},
		LotSizeFilter: sdk.LotSizeFilter{
			BasePrecision: "0.0001",
			MinOrderQty:   "0.0001",
			MinOrderAmt:   "5",
		},
	})
	require.NoError(t, err)
	require.True(t, details.MinNotional.Equal(decimal.RequireFromString("5")))
}
