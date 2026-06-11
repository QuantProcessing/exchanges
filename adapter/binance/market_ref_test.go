package binance

import (
	"context"
	"net/http"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpAdapterRoutesExplicitQuoteSymbols(t *testing.T) {
	client := newBinancePerpTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/fapi/v1/exchangeInfo", r.URL.Path)
		return binanceJSONHTTPResponse(`{"timezone":"UTC","serverTime":1,"symbols":[
			{"symbol":"BTCUSDT","pair":"BTCUSDT","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT","pricePrecision":1,"quantityPrecision":3,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.1"},{"filterType":"LOT_SIZE","minQty":"0.001","stepSize":"0.001"},{"filterType":"MIN_NOTIONAL","notional":"5"}]},
			{"symbol":"BTCUSDC","pair":"BTCUSDC","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDC","marginAsset":"USDC","pricePrecision":2,"quantityPrecision":4,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"LOT_SIZE","minQty":"0.0001","stepSize":"0.0001"},{"filterType":"MIN_NOTIONAL","notional":"5"}]}
		]}`), nil
	})

	adp, err := newPerpAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	require.Equal(t, "btcusdt", adp.FormatSymbol("BTC"))
	require.Equal(t, "btcusdt", adp.FormatSymbol("BTC/USDT"))
	require.Equal(t, "btcusdc", adp.FormatSymbol("BTC/USDC"))
	require.Equal(t, "BTC/USDT", adp.ExtractSymbol("BTCUSDT"))
	require.Equal(t, "BTC/USDC", adp.ExtractSymbol("BTCUSDC"))

	usdtDetails, err := adp.GetSymbolDetail("BTC/USDT")
	require.NoError(t, err)
	require.Equal(t, int32(1), usdtDetails.PricePrecision)

	usdcDetails, err := adp.GetSymbolDetail("BTC/USDC")
	require.NoError(t, err)
	require.Equal(t, int32(2), usdcDetails.PricePrecision)
}

func TestSpotAdapterRoutesExplicitQuoteSymbols(t *testing.T) {
	client := newBinanceSpotTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v3/exchangeInfo", r.URL.Path)
		return binanceJSONHTTPResponse(`{"timezone":"UTC","serverTime":1,"symbols":[
			{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","baseAssetPrecision":6,"quoteAsset":"USDT","quotePrecision":2,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"LOT_SIZE","minQty":"0.0001","stepSize":"0.0001"}]},
			{"symbol":"BTCUSDC","status":"TRADING","baseAsset":"BTC","baseAssetPrecision":7,"quoteAsset":"USDC","quotePrecision":3,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.001"},{"filterType":"LOT_SIZE","minQty":"0.00001","stepSize":"0.00001"}]}
		]}`), nil
	})

	adp, err := newSpotAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	require.Equal(t, "btcusdt", adp.FormatSymbol("BTC"))
	require.Equal(t, "btcusdt", adp.FormatSymbol("BTC/USDT"))
	require.Equal(t, "btcusdc", adp.FormatSymbol("BTC/USDC"))
	require.Equal(t, "BTC/USDT", adp.ExtractSymbol("BTCUSDT"))
	require.Equal(t, "BTC/USDC", adp.ExtractSymbol("BTCUSDC"))

	usdtDetails, err := adp.GetSymbolDetail("BTC/USDT")
	require.NoError(t, err)
	require.Equal(t, int32(2), usdtDetails.PricePrecision)

	usdcDetails, err := adp.GetSymbolDetail("BTC/USDC")
	require.NoError(t, err)
	require.Equal(t, int32(3), usdcDetails.PricePrecision)
}

func TestPerpAdapterFetchTickerForRoutesMarketRef(t *testing.T) {
	client := newBinancePerpTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/fapi/v1/exchangeInfo":
			return binanceJSONHTTPResponse(`{"timezone":"UTC","serverTime":1,"symbols":[
				{"symbol":"BTCUSDT","pair":"BTCUSDT","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT","pricePrecision":1,"quantityPrecision":3,"filters":[]},
				{"symbol":"BTCUSDC","pair":"BTCUSDC","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDC","marginAsset":"USDC","pricePrecision":2,"quantityPrecision":4,"filters":[]}
			]}`), nil
		case "/fapi/v1/ticker/24hr":
			require.Equal(t, "btcusdc", r.URL.Query().Get("symbol"))
			return binanceJSONHTTPResponse(`{"symbol":"BTCUSDC","lastPrice":"50000","bidPrice":"49999","askPrice":"50001","volume":"10","quoteVolume":"500000","highPrice":"51000","lowPrice":"49000","closeTime":1710000000000}`), nil
		case "/fapi/v1/depth":
			require.Equal(t, "btcusdc", r.URL.Query().Get("symbol"))
			return binanceJSONHTTPResponse(`{"lastUpdateId":1,"bids":[["49999","1"]],"asks":[["50001","1"]]}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newPerpAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	ticker, err := adp.FetchTickerFor(context.Background(), exchanges.ParseMarketRef("BTC/USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp))
	require.NoError(t, err)
	require.Equal(t, "BTC/USDC", ticker.Symbol)
}

func TestSpotAdapterPlaceOrderForCopiesMarketRefIntoParams(t *testing.T) {
	adp := &SpotAdapter{quoteCurrency: "USDT"}
	params := &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("50000"),
	}

	copied := adp.paramsForMarket(exchanges.ParseMarketRef("BTC/USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypeSpot), params)

	require.Equal(t, "BTC", params.Symbol)
	require.Equal(t, exchanges.MarketRef{}, params.Market)
	require.Equal(t, "BTC/USDC", copied.Symbol)
	require.Equal(t, exchanges.QuoteCurrencyUSDC, copied.Market.Quote)
}
