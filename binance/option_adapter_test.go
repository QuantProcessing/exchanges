package binance

import (
	"context"
	"net/http"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	optionsdk "github.com/QuantProcessing/exchanges/binance/sdk/option"
	"github.com/QuantProcessing/exchanges/testsuite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var _ exchanges.OptionExchange = (*OptionAdapter)(nil)

func TestBinanceOptionAdapterListsContracts(t *testing.T) {
	ctx := context.Background()
	client := newBinanceOptionTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/eapi/v1/exchangeInfo":
			return binanceJSONHTTPResponse(`{
				"optionSymbols": [
					{
						"symbol": "BTC-260626-140000-C",
						"status": "TRADING",
						"baseAsset": "BTC",
						"quoteAsset": "USDT",
						"underlying": "BTCUSDT",
						"settleAsset": "USDT",
						"side": "CALL",
						"strikePrice": "140000",
						"expiryDate": 1782460800000,
						"unit": 1,
						"priceScale": 1,
						"quantityScale": 3,
						"minQty": "0.001"
					},
					{
						"symbol": "ETH-260626-8000-P",
						"status": "TRADING",
						"baseAsset": "ETH",
						"quoteAsset": "USDT",
						"underlying": "ETHUSDT",
						"settleAsset": "USDT",
						"side": "PUT",
						"strikePrice": "8000",
						"expiryDate": 1782460800000,
						"unit": "1",
						"priceScale": 2,
						"quantityScale": 2,
						"minQty": "0.01"
					}
				]
			}`), nil
		case "/eapi/v1/ticker":
			require.Equal(t, "BTC-260626-140000-C", r.URL.Query().Get("symbol"))
			return binanceJSONHTTPResponse(`{"symbol":"BTC-260626-140000-C","lastPrice":"12.5","bidPrice":"12.4","askPrice":"12.6","highPrice":"13","lowPrice":"11","volume":"2","amount":"25","closeTime":1782460800000}`), nil
		case "/eapi/v1/depth":
			require.Equal(t, "BTC-260626-140000-C", r.URL.Query().Get("symbol"))
			require.Equal(t, "2", r.URL.Query().Get("limit"))
			return binanceJSONHTTPResponse(`{"T":1782460800000,"bids":[["12.4","3"]],"asks":[["12.6","5"]]}`), nil
		case "/eapi/v1/trades":
			require.Equal(t, "BTC-260626-140000-C", r.URL.Query().Get("symbol"))
			require.Equal(t, "1", r.URL.Query().Get("limit"))
			return binanceJSONHTTPResponse(`[{"id":1,"tradeId":1,"price":"12.5","qty":"3","side":"SELL","time":1782460800000}]`), nil
		case "/eapi/v1/klines":
			require.Equal(t, "BTC-260626-140000-C", r.URL.Query().Get("symbol"))
			require.Equal(t, "1h", r.URL.Query().Get("interval"))
			return binanceJSONHTTPResponse(`[[1782460800000,"12","13","11","12.5","3",1782464400000,"37.5"]]`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newOptionAdapterWithClient(ctx, Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	contracts, err := adp.ListOptionContracts(ctx, "BTC")
	require.NoError(t, err)
	require.Len(t, contracts, 1)

	contract := contracts[0]
	require.Equal(t, "BTC-USDT-USDT-20260626-140000-C", contract.Symbol)
	require.Equal(t, "BTC-260626-140000-C", contract.ExchangeSymbol)
	require.Equal(t, exchanges.OptionTypeCall, contract.Type)
	require.Equal(t, "140000", contract.StrikePrice.String())
	require.Equal(t, int64(1782460800000), contract.ExpiryTime)
	require.Equal(t, "1", contract.ContractSize.String())
	require.Equal(t, "0.1", contract.TickSize.String())
	require.Equal(t, "0.001", contract.LotSize.String())
	require.Equal(t, "0.001", contract.MinQuantity.String())

	require.Equal(t, "BTC-260626-140000-C", adp.FormatSymbol("BTC-USDT-USDT-20260626-140000-C"))
	require.Equal(t, "BTC-USDT-USDT-20260626-140000-C", adp.ExtractSymbol("BTC-260626-140000-C"))

	fetched, err := adp.FetchOptionContract(ctx, "BTC-260626-140000-C")
	require.NoError(t, err)
	require.Equal(t, contract.Symbol, fetched.Symbol)

	testsuite.RunOptionPublicDataSuite(t, adp, testsuite.OptionPublicDataConfig{
		Underlying: "BTC",
		Symbol:     contract.Symbol,
		Native:     contract.ExchangeSymbol,
		Interval:   exchanges.Interval1h,
	})
}

func TestBinanceOptionAdapterRESTTradingUsesSignedOptionEndpoints(t *testing.T) {
	ctx := context.Background()
	contract := "BTC-USDT-USDT-20260626-140000-C"
	native := "BTC-260626-140000-C"

	client := newBinanceOptionTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/eapi/v1/exchangeInfo":
			return binanceJSONHTTPResponse(`{"optionSymbols":[{"symbol":"BTC-260626-140000-C","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","underlying":"BTCUSDT","settleAsset":"USDT","side":"CALL","strikePrice":"140000","expiryDate":1782460800000,"unit":1,"priceScale":1,"quantityScale":3,"minQty":"0.001"}]}`), nil
		case "/eapi/v1/order":
			require.Equal(t, "test-key", r.Header.Get("X-MBX-APIKEY"))
			require.NotEmpty(t, r.URL.Query().Get("timestamp"))
			require.NotEmpty(t, r.URL.Query().Get("signature"))
			require.Equal(t, native, r.URL.Query().Get("symbol"))
			switch r.Method {
			case http.MethodPost:
				require.Equal(t, "BUY", r.URL.Query().Get("side"))
				require.Equal(t, "LIMIT", r.URL.Query().Get("type"))
				require.Equal(t, "0.123", r.URL.Query().Get("quantity"))
				require.Equal(t, "12.5", r.URL.Query().Get("price"))
				require.Equal(t, "GTC", r.URL.Query().Get("timeInForce"))
				require.Equal(t, "option-client-1", r.URL.Query().Get("clientOrderId"))
				return binanceJSONHTTPResponse(`{"id":"4611875134427365377","symbol":"BTC-260626-140000-C","price":"12.5","quantity":"0.123","executedQty":"0","fee":"0","side":"BUY","type":"LIMIT","timeInForce":"GTC","createDate":1782460800000,"status":"ACCEPTED","avgPrice":"0","reduceOnly":false,"clientOrderId":"option-client-1"}`), nil
			case http.MethodGet:
				require.Equal(t, "4611875134427365377", r.URL.Query().Get("orderId"))
				return binanceJSONHTTPResponse(`{"id":"4611875134427365377","symbol":"BTC-260626-140000-C","price":"12.5","quantity":"0.123","executedQty":"0.023","fee":"0.001","side":"BUY","type":"LIMIT","timeInForce":"GTC","createDate":1782460800000,"status":"PARTIALLY_FILLED","avgPrice":"12.4","reduceOnly":false,"clientOrderId":"option-client-1"}`), nil
			case http.MethodDelete:
				require.Equal(t, "4611875134427365377", r.URL.Query().Get("orderId"))
				return binanceJSONHTTPResponse(`{"id":"4611875134427365377","symbol":"BTC-260626-140000-C","price":"12.5","quantity":"0.123","executedQty":"0.023","fee":"0.001","side":"BUY","type":"LIMIT","timeInForce":"GTC","createDate":1782460800000,"status":"CANCELLED","avgPrice":"12.4","reduceOnly":false,"clientOrderId":"option-client-1"}`), nil
			default:
				t.Fatalf("unexpected order method: %s", r.Method)
				return nil, nil
			}
		case "/eapi/v1/openOrders":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, native, r.URL.Query().Get("symbol"))
			return binanceJSONHTTPResponse(`[{"id":"4611875134427365377","symbol":"BTC-260626-140000-C","price":"12.5","quantity":"0.123","executedQty":"0.023","fee":"0.001","side":"BUY","type":"LIMIT","timeInForce":"GTC","createDate":1782460800000,"status":"PARTIALLY_FILLED","avgPrice":"12.4","reduceOnly":false,"clientOrderId":"option-client-1"}]`), nil
		case "/eapi/v1/historyOrders":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, native, r.URL.Query().Get("symbol"))
			return binanceJSONHTTPResponse(`[{"id":"4611875134427365378","symbol":"BTC-260626-140000-C","price":"11.5","quantity":"0.1","executedQty":"0.1","fee":"0.001","side":"SELL","type":"LIMIT","timeInForce":"GTC","createDate":1782460800001,"status":"FILLED","avgPrice":"11.5","reduceOnly":false,"clientOrderId":"option-client-2"}]`), nil
		case "/eapi/v1/allOpenOrders":
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, native, r.URL.Query().Get("symbol"))
			return binanceJSONHTTPResponse(`{"code":0,"msg":"success"}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	}).WithCredentials("test-key", "test-secret")

	adp, err := newOptionAdapterWithClient(ctx, Options{APIKey: "test-key", SecretKey: "test-secret"}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)

	order, err := adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:      contract,
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.123"),
		Price:       decimal.RequireFromString("12.5"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "option-client-1",
	})
	require.NoError(t, err)
	require.Equal(t, "4611875134427365377", order.OrderID)
	require.Equal(t, contract, order.Symbol)

	fetched, err := adp.FetchOrderByID(ctx, "4611875134427365377", contract)
	require.NoError(t, err)
	require.Equal(t, exchanges.OrderStatusPartiallyFilled, fetched.Status)

	openOrders, err := adp.FetchOpenOrders(ctx, contract)
	require.NoError(t, err)
	require.Len(t, openOrders, 1)
	require.Equal(t, contract, openOrders[0].Symbol)

	allOrders, err := adp.FetchOrders(ctx, contract)
	require.NoError(t, err)
	require.Len(t, allOrders, 1)

	require.NoError(t, adp.CancelOrder(ctx, "4611875134427365377", contract))
	require.NoError(t, adp.CancelAllOrders(ctx, contract))
	require.ErrorIs(t, adp.PlaceOrderWS(ctx, &exchanges.OrderParams{Symbol: contract}), exchanges.ErrNotSupported)
}

func TestBinanceOptionAdapterRejectsUndocumentedMarketOptionOrders(t *testing.T) {
	ctx := context.Background()
	client := newBinanceOptionTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/eapi/v1/exchangeInfo", r.URL.Path)
		return binanceJSONHTTPResponse(`{"optionSymbols":[{"symbol":"BTC-260626-140000-C","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","underlying":"BTCUSDT","settleAsset":"USDT","side":"CALL","strikePrice":"140000","expiryDate":1782460800000,"unit":1,"priceScale":1,"quantityScale":3,"minQty":"0.001"}]}`), nil
	}).WithCredentials("test-key", "test-secret")

	adp, err := newOptionAdapterWithClient(ctx, Options{APIKey: "test-key", SecretKey: "test-secret"}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	_, err = adp.PlaceOrder(ctx, &exchanges.OrderParams{
		Symbol:   "BTC-USDT-USDT-20260626-140000-C",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("0.123"),
	})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}

func newBinanceOptionTestClient(fn func(*http.Request) (*http.Response, error)) *optionsdk.Client {
	client := optionsdk.NewClient().WithBaseURL("https://example.test")
	client.HTTPClient = &http.Client{Transport: binanceConstructorRoundTripFunc(fn)}
	return client
}
