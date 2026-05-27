package okx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var _ exchanges.OptionExchange = (*OptionAdapter)(nil)

func TestOKXOptionAdapterListsContracts(t *testing.T) {
	ctx := context.Background()
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			require.Equal(t, "OPTION", r.URL.Query().Get("instType"))
			return okxOptionInstrumentHTTPResponse(), nil
		case "/api/v5/market/ticker":
			require.Equal(t, "BTC-USD-260514-75000-C", r.URL.Query().Get("instId"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instType":"OPTION","instId":"BTC-USD-260514-75000-C","last":"2.5","bidPx":"2.4","askPx":"2.6","high24h":"3","low24h":"2","vol24h":"20","volCcy24h":"50","ts":"1778745600000"}]}`), nil
		case "/api/v5/market/books":
			require.Equal(t, "BTC-USD-260514-75000-C", r.URL.Query().Get("instId"))
			require.Equal(t, "2", r.URL.Query().Get("sz"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"bids":[["2.4","3","0","1"]],"asks":[["2.6","5","0","1"]],"ts":"1778745600000"}]}`), nil
		case "/api/v5/market/trades":
			require.Equal(t, "BTC-USD-260514-75000-C", r.URL.Query().Get("instId"))
			require.Equal(t, "1", r.URL.Query().Get("limit"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instId":"BTC-USD-260514-75000-C","tradeId":"trade-1","px":"2.5","sz":"3","side":"sell","ts":"1778745600000"}]}`), nil
		case "/api/v5/market/candles":
			require.Equal(t, "BTC-USD-260514-75000-C", r.URL.Query().Get("instId"))
			require.Equal(t, "1H", r.URL.Query().Get("bar"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[["1778745600000","2","3","1.5","2.5","20","0.2","50","1"]]}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newOptionAdapterWithClient(ctx, Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.Contains(t, adp.ListSymbols(), "BTC-USD-BTC-20260514-75000-C")

	contracts, err := adp.ListOptionContracts(ctx, "BTC")
	require.NoError(t, err)
	require.Len(t, contracts, 1)

	contract := contracts[0]
	require.Equal(t, "BTC-USD-BTC-20260514-75000-C", contract.Symbol)
	require.Equal(t, "BTC-USD-260514-75000-C", contract.ExchangeSymbol)
	require.Equal(t, "BTC", contract.BaseAsset)
	require.Equal(t, "USD", contract.QuoteAsset)
	require.Equal(t, "BTC", contract.SettleAsset)
	require.Equal(t, exchanges.OptionTypeCall, contract.Type)
	require.Equal(t, "0.01", contract.ContractSize.String())
	require.Equal(t, "0.0001", contract.TickSize.String())
	require.Equal(t, "0.1", contract.LotSize.String())

	require.Equal(t, "BTC-USD-260514-75000-C", adp.FormatSymbol("BTC-USD-BTC-20260514-75000-C"))
	require.Equal(t, "BTC-USD-BTC-20260514-75000-C", adp.ExtractSymbol("BTC-USD-260514-75000-C"))

	fetched, err := adp.FetchOptionContract(ctx, "BTC-USD-260514-75000-C")
	require.NoError(t, err)
	require.Equal(t, contract.Symbol, fetched.Symbol)

	testsuite.RunOptionPublicDataSuite(t, adp, testsuite.OptionPublicDataConfig{
		Underlying: "BTC",
		Symbol:     contract.Symbol,
		Native:     contract.ExchangeSymbol,
		Interval:   exchanges.Interval1h,
	})
}

func TestOKXOptionAdapterFetchSymbolDetailsLoadsFamily(t *testing.T) {
	ctx := context.Background()
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v5/public/instruments", r.URL.Path)
		require.Equal(t, "OPTION", r.URL.Query().Get("instType"))
		return okxOptionInstrumentHTTPResponse(), nil
	})

	adp, err := newOptionAdapterWithClient(ctx, Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	details, err := adp.FetchSymbolDetails(ctx, "BTC-USD-BTC-20260514-75000-C")
	require.NoError(t, err)
	require.Equal(t, "BTC-USD-BTC-20260514-75000-C", details.Symbol)
	require.Equal(t, int32(4), details.PricePrecision)
	require.Equal(t, int32(1), details.QuantityPrecision)
	require.Equal(t, "0.1", details.MinQuantity.String())
}

func TestOKXOptionAdapterFetchKlinesPassesTimeBounds(t *testing.T) {
	ctx := context.Background()
	start := time.UnixMilli(1778745600000)
	end := time.UnixMilli(1778832000000)
	sawCandles := false
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			require.Equal(t, "OPTION", r.URL.Query().Get("instType"))
			return okxOptionInstrumentHTTPResponse(), nil
		case "/api/v5/market/candles":
			sawCandles = true
			require.Equal(t, "BTC-USD-260514-75000-C", r.URL.Query().Get("instId"))
			require.Equal(t, "1H", r.URL.Query().Get("bar"))
			require.Equal(t, "1778745600000", r.URL.Query().Get("before"))
			require.Equal(t, "1778832000000", r.URL.Query().Get("after"))
			require.Equal(t, "7", r.URL.Query().Get("limit"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[["1778832000000","2","3","1.5","2.5","20","0.2","50","1"],["1778745600000","1","2","0.5","1.5","10","0.1","15","1"]]}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newOptionAdapterWithClient(ctx, Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	klines, err := adp.FetchKlines(ctx, "BTC-USD-BTC-20260514-75000-C", exchanges.Interval1h, &exchanges.KlineOpts{
		Start: &start,
		End:   &end,
		Limit: 7,
	})
	require.NoError(t, err)
	require.True(t, sawCandles)
	require.Len(t, klines, 2)
	require.Equal(t, "BTC-USD-BTC-20260514-75000-C", klines[0].Symbol)
	require.Equal(t, int64(1778745600000), klines[0].Timestamp)
	require.Equal(t, int64(1778832000000), klines[1].Timestamp)
}

func TestOKXOptionAdapterRESTTradingUsesOptionTradeEndpoints(t *testing.T) {
	ctx := context.Background()
	contract := "BTC-USD-BTC-20260514-75000-C"
	native := "BTC-USD-260514-75000-C"

	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			require.Equal(t, "OPTION", r.URL.Query().Get("instType"))
			return okxOptionInstrumentHTTPResponse(), nil
		case "/api/v5/trade/order":
			if r.Method == http.MethodPost {
				require.Equal(t, "test-key", r.Header.Get("OK-ACCESS-KEY"))
				var body map[string]any
				data, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, &body))
				require.Equal(t, native, body["instId"])
				require.Equal(t, "cross", body["tdMode"])
				require.Equal(t, "buy", body["side"])
				require.Equal(t, "limit", body["ordType"])
				require.Equal(t, "0.1", body["sz"])
				require.Equal(t, "2.5000", body["px"])
				require.Equal(t, "option-client-1", body["clOrdId"])
				return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"ordId":"okx-order-1","clOrdId":"option-client-1","sCode":"0","sMsg":""}]}`), nil
			}
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, native, r.URL.Query().Get("instId"))
			require.Equal(t, "okx-order-1", r.URL.Query().Get("ordId"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instType":"OPTION","instId":"BTC-USD-260514-75000-C","ordId":"okx-order-1","clOrdId":"option-client-1","side":"buy","ordType":"limit","state":"live","sz":"0.1","accFillSz":"0","px":"2.5000","avgPx":"","uTime":"1778745600000"}]}`), nil
		case "/api/v5/trade/cancel-order":
			var body map[string]string
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &body))
			require.Equal(t, native, body["instId"])
			require.Equal(t, "okx-order-1", body["ordId"])
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"ordId":"okx-order-1","clOrdId":"","sCode":"0","sMsg":""}]}`), nil
		case "/api/v5/trade/orders-pending":
			require.Equal(t, "OPTION", r.URL.Query().Get("instType"))
			require.Equal(t, native, r.URL.Query().Get("instId"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instType":"OPTION","instId":"BTC-USD-260514-75000-C","ordId":"okx-order-1","clOrdId":"option-client-1","side":"buy","ordType":"limit","state":"live","sz":"0.1","accFillSz":"0","px":"2.5000","avgPx":"","uTime":"1778745600000"}]}`), nil
		case "/api/v5/trade/cancel-batch-orders":
			var body []map[string]string
			data, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &body))
			require.Len(t, body, 1)
			require.Equal(t, native, body[0]["instId"])
			require.Equal(t, "okx-order-1", body[0]["ordId"])
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"ordId":"okx-order-1","clOrdId":"","sCode":"0","sMsg":""}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newOptionAdapterWithClient(ctx, Options{APIKey: "test-key", SecretKey: "test-secret", Passphrase: "test-pass"}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:      contract,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("2.5"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "option-client-1",
	})
	require.NoError(t, err)
	require.Equal(t, "okx-order-1", order.OrderID)
	require.Equal(t, contract, order.Symbol)

	fetched, err := adp.FetchOrderByID(ctx, "okx-order-1", contract)
	require.NoError(t, err)
	require.Equal(t, "okx-order-1", fetched.OrderID)
	require.Equal(t, "0.1", fetched.Quantity.String())

	openOrders, err := adp.FetchOpenOrders(ctx, contract)
	require.NoError(t, err)
	require.Len(t, openOrders, 1)
	require.Equal(t, contract, openOrders[0].Symbol)

	require.NoError(t, adp.CancelOrder(ctx, "okx-order-1", contract))
	require.NoError(t, adp.CancelAllOrders(ctx, contract))

	require.ErrorIs(t, adp.PlaceOrderWS(ctx, &exchanges.OrderParams{Symbol: contract}), exchanges.ErrNotSupported)
	_, err = adp.FetchOrders(ctx, contract)
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}

func TestOKXOptionAdapterLoadsDocumentedDefaultFamilies(t *testing.T) {
	ctx := context.Background()
	var requested []string
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v5/public/instruments", r.URL.Path)
		require.Equal(t, "OPTION", r.URL.Query().Get("instType"))
		family := r.URL.Query().Get("instFamily")
		require.NotEmpty(t, family)
		requested = append(requested, family)
		return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[]}`), nil
	})

	_, err := newOptionAdapterWithClient(ctx, Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.Equal(t, []string{"BTC-USD", "ETH-USD"}, requested)
}

func okxOptionInstrumentHTTPResponse() *http.Response {
	return okxJSONHTTPResponse(`{
		"code": "0",
		"msg": "",
		"data": [
			{
				"instType": "OPTION",
				"instId": "BTC-USD-260514-75000-C",
				"uly": "BTC-USD",
				"instFamily": "BTC-USD",
				"settleCcy": "BTC",
				"ctVal": "0.01",
				"ctMult": "1",
				"ctValCcy": "BTC",
				"optType": "C",
				"stk": "75000",
				"expTime": "1778745600000",
				"tickSz": "0.0001",
				"lotSz": "0.1",
				"minSz": "0.1",
				"state": "live"
			}
		]
	}`)
}
