package okx

import (
	"context"
	"net/http"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestPerpAdapterRoutesExplicitQuoteSymbols(t *testing.T) {
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v5/public/instruments", r.URL.Path)
		require.Equal(t, "SWAP", r.URL.Query().Get("instType"))
		return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[
			{"instId":"BTC-USDT-SWAP","instIdCode":123456,"baseCcy":"BTC","quoteCcy":"USDT","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.1","lotSz":"1","minSz":"1","instType":"SWAP","state":"live"},
			{"instId":"BTC-USDC-SWAP","instIdCode":123457,"baseCcy":"BTC","quoteCcy":"USDC","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.01","lotSz":"0.1","minSz":"0.1","instType":"SWAP","state":"live"}
		]}`), nil
	})

	adp, err := newPerpAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	require.Equal(t, "BTC-USDT-SWAP", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC-USDT-SWAP", adp.FormatSymbol("BTC/USDT"))
	require.Equal(t, "BTC-USDC-SWAP", adp.FormatSymbol("BTC/USDC"))
	require.Equal(t, "BTC/USDT", adp.ExtractSymbol("BTC-USDT-SWAP"))
	require.Equal(t, "BTC/USDC", adp.ExtractSymbol("BTC-USDC-SWAP"))

	usdtDetails, err := adp.GetSymbolDetail("BTC/USDT")
	require.NoError(t, err)
	require.Equal(t, int32(1), usdtDetails.PricePrecision)

	usdcDetails, err := adp.GetSymbolDetail("BTC/USDC")
	require.NoError(t, err)
	require.Equal(t, int32(2), usdcDetails.PricePrecision)
}

func TestSpotAdapterRoutesExplicitQuoteSymbols(t *testing.T) {
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v5/public/instruments", r.URL.Path)
		require.Equal(t, "SPOT", r.URL.Query().Get("instType"))
		return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[
			{"instId":"BTC-USDT","instIdCode":654321,"baseCcy":"BTC","quoteCcy":"USDT","tickSz":"0.01","lotSz":"0.0001","minSz":"0.0001","instType":"SPOT","state":"live"},
			{"instId":"BTC-USDC","instIdCode":654322,"baseCcy":"BTC","quoteCcy":"USDC","tickSz":"0.001","lotSz":"0.00001","minSz":"0.00001","instType":"SPOT","state":"live"}
		]}`), nil
	})

	adp, err := newSpotAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	require.Equal(t, "BTC-USDT", adp.FormatSymbol("BTC"))
	require.Equal(t, "BTC-USDT", adp.FormatSymbol("BTC/USDT"))
	require.Equal(t, "BTC-USDC", adp.FormatSymbol("BTC/USDC"))
	require.Equal(t, "BTC/USDT", adp.ExtractSymbol("BTC-USDT"))
	require.Equal(t, "BTC/USDC", adp.ExtractSymbol("BTC-USDC"))

	usdtDetails, err := adp.GetSymbolDetail("BTC/USDT")
	require.NoError(t, err)
	require.Equal(t, int32(2), usdtDetails.PricePrecision)

	usdcDetails, err := adp.GetSymbolDetail("BTC/USDC")
	require.NoError(t, err)
	require.Equal(t, int32(3), usdcDetails.PricePrecision)
}

func TestPerpAdapterFetchOrderBookForRoutesMarketRef(t *testing.T) {
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			require.Equal(t, "SWAP", r.URL.Query().Get("instType"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[
				{"instId":"BTC-USDT-SWAP","instIdCode":123456,"baseCcy":"BTC","quoteCcy":"USDT","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.1","lotSz":"1","minSz":"1","instType":"SWAP","state":"live"},
				{"instId":"BTC-USDC-SWAP","instIdCode":123457,"baseCcy":"BTC","quoteCcy":"USDC","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.01","lotSz":"0.1","minSz":"0.1","instType":"SWAP","state":"live"}
			]}`), nil
		case "/api/v5/market/books":
			require.Equal(t, "BTC-USDC-SWAP", r.URL.Query().Get("instId"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"asks":[["50001","1","0","1"]],"bids":[["49999","2","0","1"]],"ts":"1710000000000"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newPerpAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	book, err := adp.FetchOrderBookFor(context.Background(), exchanges.ParseMarketRef("BTC/USDC", exchanges.QuoteCurrencyUSDT, exchanges.MarketTypePerp), 5)
	require.NoError(t, err)
	require.Equal(t, "BTC/USDC", book.Symbol)
	require.Len(t, book.Bids, 1)
	require.Len(t, book.Asks, 1)
}
