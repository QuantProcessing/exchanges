package sdk

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetInstrumentsParsesEnvelope(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/v3/market/instruments", r.URL.Path)
			require.Equal(t, "SPOT", r.URL.Query().Get("category"))
			return jsonResponse(`{"code":"00000","msg":"success","requestTime":1,"data":[{"symbol":"BTCUSDT","category":"SPOT","baseCoin":"BTC","quoteCoin":"USDT","minOrderQty":"0.0001","minOrderAmount":"5","pricePrecision":"2","quantityPrecision":"4","status":"online"}]}`), nil
		}),
	}

	got, err := client.GetInstruments(context.Background(), "SPOT", "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "BTCUSDT", got[0].Symbol)
}

func TestGetTickerParsesTickerList(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/v3/market/tickers", r.URL.Path)
			require.Equal(t, "USDT-FUTURES", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"code":"00000","msg":"success","requestTime":1,"data":[{"category":"USDT-FUTURES","symbol":"BTCUSDT","ts":"1710000000000","lastPrice":"50000","bid1Price":"49999","ask1Price":"50001","volume24h":"100","turnover24h":"5000000"}]}`), nil
		}),
	}

	got, err := client.GetTicker(context.Background(), "USDT-FUTURES", "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", got.Symbol)
	require.Equal(t, "50000", got.LastPrice)
}

func TestGetOrderBookParsesNumericArrays(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/v3/market/orderbook", r.URL.Path)
			return jsonResponse(`{"code":"00000","msg":"success","requestTime":1,"data":{"a":[[50001,1.2]],"b":[[49999,0.8]],"ts":"1710000000001"}}`), nil
		}),
	}

	got, err := client.GetOrderBook(context.Background(), "SPOT", "BTCUSDT", 5)
	require.NoError(t, err)
	require.Len(t, got.Asks, 1)
	require.Equal(t, NumberString("50001"), got.Asks[0][0])
	require.Equal(t, NumberString("0.8"), got.Bids[0][1])
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetOpenInterestParses(t *testing.T) {
	t.Parallel()
	payload := `{"code":"00000","msg":"success","data":{"symbol":"BTCUSDT","amount":"1234.56","timestamp":"1700000000000"}}`
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/v2/mix/market/open-interest", r.URL.Path)
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			require.Equal(t, "USDT-FUTURES", r.URL.Query().Get("productType"))
			return jsonResponse(payload), nil
		}),
	}
	oi, err := client.GetOpenInterest(context.Background(), "BTCUSDT", "USDT-FUTURES")
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", oi.Symbol)
	require.Equal(t, "1234.56", oi.Amount)
	require.Equal(t, "1700000000000", oi.Timestamp)
}

func TestGetHistoryFundRateParses(t *testing.T) {
	t.Parallel()
	payload := `{"code":"00000","msg":"success","data":[{"symbol":"BTCUSDT","fundingRate":"0.0001","fundingTime":"1700000000000"},{"symbol":"BTCUSDT","fundingRate":"0.00012","fundingTime":"1700028800000"}]}`
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/api/v2/mix/market/history-fund-rate", r.URL.Path)
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			require.Equal(t, "USDT-FUTURES", r.URL.Query().Get("productType"))
			require.Equal(t, "2", r.URL.Query().Get("pageSize"))
			return jsonResponse(payload), nil
		}),
	}
	hist, err := client.GetHistoryFundRate(context.Background(), "BTCUSDT", "USDT-FUTURES", 2, 1)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "0.0001", hist[0].FundingRate)
	require.Equal(t, "1700000000000", hist[0].FundingTime)
}
