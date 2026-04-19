package sdk

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetInstrumentsPaginatesLinearResults(t *testing.T) {
	var hits atomic.Int32

	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/market/instruments-info", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))

			switch hits.Add(1) {
			case 1:
				require.Empty(t, r.URL.Query().Get("cursor"))
				return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"symbol":"BTCUSDT","baseCoin":"BTC","quoteCoin":"USDT","status":"Trading","priceScale":"2","priceFilter":{"tickSize":"0.1"},"lotSizeFilter":{"qtyStep":"0.001","minOrderQty":"0.001","minNotionalValue":"5"}}],"nextPageCursor":"cursor-2"}}`), nil
			case 2:
				require.Equal(t, "cursor-2", r.URL.Query().Get("cursor"))
				return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"symbol":"ETHUSDT","baseCoin":"ETH","quoteCoin":"USDT","status":"Trading","priceScale":"2","priceFilter":{"tickSize":"0.01"},"lotSizeFilter":{"qtyStep":"0.01","minOrderQty":"0.01","minNotionalValue":"5"}}],"nextPageCursor":""}}`), nil
			default:
				t.Fatalf("unexpected extra request %d", hits.Load())
				return nil, nil
			}
		}),
	}

	got, err := client.GetInstruments(context.Background(), "linear")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "BTCUSDT", got[0].Symbol)
	require.Equal(t, "ETHUSDT", got[1].Symbol)
}

func TestGetTickerParsesTickerList(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/market/tickers", r.URL.Path)
			require.Equal(t, "spot", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[{"symbol":"BTCUSDT","lastPrice":"50000","bid1Price":"49999","ask1Price":"50001","volume24h":"100","turnover24h":"5000000","highPrice24h":"51000","lowPrice24h":"49000","time":"1710000000000"}]}}`), nil
		}),
	}

	got, err := client.GetTicker(context.Background(), "spot", "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", got.Symbol)
	require.Equal(t, "50000", got.LastPrice)
	require.Equal(t, "1710000000000", got.Time)
}

func TestGetOrderBookParsesNumericArrays(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/market/orderbook", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"s":"BTCUSDT","b":[["49999","0.8"]],"a":[["50001","1.2"]],"ts":1710000000001,"u":10}}`), nil
		}),
	}

	got, err := client.GetOrderBook(context.Background(), "linear", "BTCUSDT", 5)
	require.NoError(t, err)
	require.Len(t, got.Asks, 1)
	require.Equal(t, NumberString("50001"), got.Asks[0][0])
	require.Equal(t, NumberString("0.8"), got.Bids[0][1])
}

func TestGetRecentTradesParsesList(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/market/recent-trade", r.URL.Path)
			require.Equal(t, "spot", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[{"execId":"trade-1","symbol":"BTCUSDT","price":"50000","size":"0.25","side":"Buy","time":"1710000000002"}]}}`), nil
		}),
	}

	got, err := client.GetRecentTrades(context.Background(), "spot", "BTCUSDT", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "trade-1", got[0].ExecID)
}

func TestGetKlinesParsesStringArrays(t *testing.T) {
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/market/kline", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			require.Equal(t, "60", r.URL.Query().Get("interval"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"category":"linear","symbol":"BTCUSDT","list":[["1710000000000","50000","51000","49000","50500","12","600000"]]}}`), nil
		}),
	}

	got, err := client.GetKlines(context.Background(), "linear", "BTCUSDT", "60", 0, 0, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, NumberString("50500"), got[0][4])
}

func TestGetOpenInterestParses(t *testing.T) {
	t.Parallel()
	payload := `{"retCode":0,"retMsg":"OK","result":{"category":"linear","symbol":"BTCUSDT","list":[{"openInterest":"12345.67","timestamp":"1700000000000"}],"nextPageCursor":"abc"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v5/market/open-interest", r.URL.Path)
		require.Equal(t, "linear", r.URL.Query().Get("category"))
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		require.Equal(t, "5min", r.URL.Query().Get("intervalTime"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	client := NewClient()
	client.baseURL = srv.URL
	oi, err := client.GetOpenInterest(context.Background(), "linear", "BTCUSDT", "5min", 0, 0, 50, "")
	require.NoError(t, err)
	require.Len(t, oi.List, 1)
	require.Equal(t, "12345.67", oi.List[0].OpenInterest)
	require.Equal(t, "abc", oi.NextPageCursor)
}

func TestGetFundingHistoryParses(t *testing.T) {
	t.Parallel()
	payload := `{"retCode":0,"retMsg":"OK","result":{"category":"linear","list":[{"symbol":"BTCUSDT","fundingRate":"0.0001","fundingRateTimestamp":"1700000000000"},{"symbol":"BTCUSDT","fundingRate":"0.00012","fundingRateTimestamp":"1700028800000"}]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v5/market/funding/history", r.URL.Path)
		require.Equal(t, "linear", r.URL.Query().Get("category"))
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		require.Equal(t, "2", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	client := NewClient()
	client.baseURL = srv.URL
	res, err := client.GetFundingHistory(context.Background(), "linear", "BTCUSDT", 0, 0, 2)
	require.NoError(t, err)
	require.Len(t, res, 2)
	require.Equal(t, "0.0001", res[0].FundingRate)
	require.Equal(t, "1700000000000", res[0].FundingRateTimestamp)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
