package backpack

import (
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/stretchr/testify/require"
)

func TestPerpFormatSymbolUsesMarketCache(t *testing.T) {
	adp := &Adapter{
		BaseAdapter: exchanges.NewBaseAdapter("BACKPACK", exchanges.MarketTypePerp, exchanges.NopLogger),
		quote:       exchanges.QuoteCurrencyUSDC,
		markets: &marketCache{
			perpByBase: map[string]sdk.Market{"BTC": {Symbol: "BTC_USDC_PERP"}},
			bySymbol:   map[string]sdk.Market{"BTC_USDC_PERP": {BaseSymbol: "BTC"}},
		},
	}

	require.Equal(t, "BTC_USDC_PERP", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC", adp.ExtractSymbol("BTC_USDC_PERP"))
}

func TestSpotFormatSymbolUsesMarketCache(t *testing.T) {
	adp := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("BACKPACK", exchanges.MarketTypeSpot, exchanges.NopLogger),
		quote:       exchanges.QuoteCurrencyUSDC,
		markets: &marketCache{
			spotByBase: map[string]sdk.Market{"BTC": {Symbol: "BTC_USDC"}},
			bySymbol:   map[string]sdk.Market{"BTC_USDC": {BaseSymbol: "BTC"}},
		},
	}

	require.Equal(t, "BTC_USDC", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC", adp.ExtractSymbol("BTC_USDC"))
}

func TestToTickerSetsTimestampWhenExchangeDoesNotProvideOne(t *testing.T) {
	before := time.Now().UnixMilli()
	got := toTicker("BTC", &sdk.Ticker{
		LastPrice:   "50000",
		High:        "51000",
		Low:         "49000",
		Volume:      "10",
		QuoteVolume: "500000",
	})
	after := time.Now().UnixMilli()

	require.Equal(t, "BTC", got.Symbol)
	require.True(t, got.LastPrice.IsPositive())
	require.GreaterOrEqual(t, got.Timestamp, before)
	require.LessOrEqual(t, got.Timestamp, after)
}
