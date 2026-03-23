package bitget

import (
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/stretchr/testify/require"
)

func TestPerpFormatSymbolUsesMarketCache(t *testing.T) {
	adp := &Adapter{
		BaseAdapter:  exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypePerp, exchanges.NopLogger),
		quote:        exchanges.QuoteCurrencyUSDT,
		perpCategory: categoryUSDTFutures,
		markets: &marketCache{
			perpByBase: map[string]sdk.Instrument{"BTC": {Symbol: "BTCUSDT", BaseCoin: "BTC", Category: categoryUSDTFutures}},
			bySymbol:   map[string]sdk.Instrument{"BTCUSDT": {BaseCoin: "BTC", Category: categoryUSDTFutures}},
		},
	}

	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC", adp.ExtractSymbol("BTCUSDT"))
}

func TestSpotFormatSymbolUsesMarketCache(t *testing.T) {
	adp := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeSpot, exchanges.NopLogger),
		quote:       exchanges.QuoteCurrencyUSDT,
		markets: &marketCache{
			spotByBase: map[string]sdk.Instrument{"BTC": {Symbol: "BTCUSDT", BaseCoin: "BTC", Category: categorySpot}},
			bySymbol:   map[string]sdk.Instrument{"BTCUSDT": {BaseCoin: "BTC", Category: categorySpot}},
		},
	}

	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC", adp.ExtractSymbol("BTCUSDT"))
}

func TestToTickerMapsBidAskAndMid(t *testing.T) {
	ticker := toTicker("BTC", &sdk.Ticker{
		Timestamp:    "1710000000000",
		LastPrice:    "50000",
		Bid1Price:    "49999",
		Ask1Price:    "50001",
		Volume24h:    "10",
		Turnover24h:  "500000",
		HighPrice24h: "51000",
		LowPrice24h:  "49000",
		IndexPrice:   "50002",
		MarkPrice:    "50003",
	})

	require.Equal(t, "BTC", ticker.Symbol)
	require.Equal(t, "50000", ticker.LastPrice.String())
	require.Equal(t, "50000", ticker.MidPrice.String())
	require.Equal(t, int64(1710000000000), ticker.Timestamp)
}
