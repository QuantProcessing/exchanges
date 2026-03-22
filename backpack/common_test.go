package backpack

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBuildMarketCacheSeparatesSpotAndPerp(t *testing.T) {
	markets := []sdk.Market{
		{
			Symbol:      "BTC_USDC",
			BaseSymbol:  "BTC",
			QuoteSymbol: "USDC",
			MarketType:  "SPOT",
			Visible:     true,
		},
		{
			Symbol:      "BTC_USDC_PERP",
			BaseSymbol:  "BTC",
			QuoteSymbol: "USDC",
			MarketType:  "PERP",
			Visible:     true,
		},
	}

	cache, err := buildMarketCache(markets, exchanges.QuoteCurrencyUSDC)
	require.NoError(t, err)
	require.Equal(t, "BTC_USDC", cache.spotByBase["BTC"].Symbol)
	require.Equal(t, "BTC_USDC_PERP", cache.perpByBase["BTC"].Symbol)
}

func TestSymbolDetailsFromMarketUsesTickAndStepPrecision(t *testing.T) {
	market := sdk.Market{
		Symbol:      "BTC_USDC",
		BaseSymbol:  "BTC",
		QuoteSymbol: "USDC",
		MarketType:  "SPOT",
		Filters: sdk.MarketFilters{
			Price: sdk.PriceFilter{
				MinPrice: "10",
				TickSize: "0.10",
			},
			Quantity: sdk.QuantityFilter{
				MinQuantity: "0.001",
				StepSize:    "0.001",
			},
		},
	}

	details, err := symbolDetailsFromMarket(market)
	require.NoError(t, err)
	require.Equal(t, "BTC", details.Symbol)
	require.EqualValues(t, 1, details.PricePrecision)
	require.EqualValues(t, 3, details.QuantityPrecision)
	require.True(t, details.MinQuantity.Equal(decimal.RequireFromString("0.001")))
	require.True(t, details.MinNotional.Equal(decimal.RequireFromString("0.01")))
}

func TestMicrosToMillisPreservesMillisecondsAndConvertsMicros(t *testing.T) {
	require.Equal(t, int64(1710000000000), microsToMillis(1710000000000))
	require.Equal(t, int64(1710000000000), microsToMillis(1710000000000000))
}
