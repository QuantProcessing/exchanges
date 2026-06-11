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
			perpByBase: map[string]sdk.Instrument{
				"BTC":      {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categoryUSDTFutures},
				"BTC/USDT": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categoryUSDTFutures},
				"BTC/USDC": {Symbol: "BTCUSDC", BaseCoin: "BTC", QuoteCoin: "USDC", Category: categoryUSDCFutures},
			},
			bySymbol: map[string]sdk.Instrument{
				"BTCUSDT": {BaseCoin: "BTC", QuoteCoin: "USDT", Category: categoryUSDTFutures},
				"BTCUSDC": {BaseCoin: "BTC", QuoteCoin: "USDC", Category: categoryUSDCFutures},
			},
		},
	}

	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC/USDT"))
	require.Equal(t, "BTCUSDC", adp.FormatSymbol("BTC/USDC"))
	require.Equal(t, "BTC/USDT", adp.ExtractSymbol("BTCUSDT"))
	require.Equal(t, "BTC/USDC", adp.ExtractSymbol("BTCUSDC"))
}

func TestSpotFormatSymbolUsesMarketCache(t *testing.T) {
	adp := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeSpot, exchanges.NopLogger),
		quote:       exchanges.QuoteCurrencyUSDT,
		markets: &marketCache{
			spotByBase: map[string]sdk.Instrument{
				"BTC":      {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot},
				"BTC/USDT": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot},
				"BTC/USDC": {Symbol: "BTCUSDC", BaseCoin: "BTC", QuoteCoin: "USDC", Category: categorySpot},
			},
			bySymbol: map[string]sdk.Instrument{
				"BTCUSDT": {BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot},
				"BTCUSDC": {BaseCoin: "BTC", QuoteCoin: "USDC", Category: categorySpot},
			},
		},
	}

	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTCUSDT", adp.FormatSymbol("BTC/USDT"))
	require.Equal(t, "BTCUSDC", adp.FormatSymbol("BTC/USDC"))
	require.Equal(t, "BTC/USDT", adp.ExtractSymbol("BTCUSDT"))
	require.Equal(t, "BTC/USDC", adp.ExtractSymbol("BTCUSDC"))
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
