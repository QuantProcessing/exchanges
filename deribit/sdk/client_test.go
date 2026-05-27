package sdk

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeribitClientGetInstrumentsEncodesQueryAndDecodesResult(t *testing.T) {
	t.Parallel()

	client := NewClient().WithBaseURL("https://example.test")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v2/public/get_instruments", r.URL.Path)
		require.Equal(t, "BTC", r.URL.Query().Get("currency"))
		require.Equal(t, "option", r.URL.Query().Get("kind"))
		require.Empty(t, r.URL.Query().Get("expired"))
		return jsonHTTPResponse(`{
			"jsonrpc": "2.0",
			"result": [
				{
					"instrument_name": "BTC-14MAY26-72000-C",
					"kind": "option",
					"base_currency": "BTC",
					"quote_currency": "BTC",
					"settlement_currency": "BTC",
					"counter_currency": "USD",
					"option_type": "call",
					"strike": 72000,
					"expiration_timestamp": 1778745600000,
					"tick_size": 0.0001,
					"min_trade_amount": 0.1,
					"contract_size": 1,
					"state": "open",
					"is_active": true
				}
			]
		}`), nil
	})}

	instruments, err := client.GetInstruments(context.Background(), "BTC", "option", false)
	require.NoError(t, err)
	require.Len(t, instruments, 1)
	require.Equal(t, "BTC-14MAY26-72000-C", instruments[0].InstrumentName)
	require.Equal(t, "call", instruments[0].OptionType)
	require.Equal(t, 72000.0, instruments[0].Strike)
}

func TestDeribitClientSurfacesJSONRPCError(t *testing.T) {
	t.Parallel()

	client := NewClient().WithBaseURL("https://example.test")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(`{
			"jsonrpc": "2.0",
			"error": {"code": 10001, "message": "instrument not found"}
		}`), nil
	})}

	_, err := client.GetTicker(context.Background(), "BTC-UNKNOWN")
	require.Error(t, err)
	require.Contains(t, err.Error(), "10001")
	require.Contains(t, err.Error(), "instrument not found")
}

func TestDeribitClientMarketDataEndpointsEncodeQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		call   func(context.Context, *Client) error
		path   string
		query  map[string]string
		result string
	}{
		{
			name: "ticker",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetTicker(ctx, "BTC-PERPETUAL")
				return err
			},
			path: "/api/v2/public/ticker",
			query: map[string]string{
				"instrument_name": "BTC-PERPETUAL",
			},
			result: `{"instrument_name":"BTC-PERPETUAL","last_price":79632}`,
		},
		{
			name: "orderbook",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetOrderBook(ctx, "BTC-PERPETUAL", 5)
				return err
			},
			path: "/api/v2/public/get_order_book",
			query: map[string]string{
				"instrument_name": "BTC-PERPETUAL",
				"depth":           "5",
			},
			result: `{"instrument_name":"BTC-PERPETUAL","bids":[[1,2]],"asks":[[3,4]]}`,
		},
		{
			name: "trades",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetLastTradesByInstrument(ctx, "BTC-PERPETUAL", 7)
				return err
			},
			path: "/api/v2/public/get_last_trades_by_instrument",
			query: map[string]string{
				"instrument_name": "BTC-PERPETUAL",
				"count":           "7",
			},
			result: `{"trades":[{"trade_id":"1","instrument_name":"BTC-PERPETUAL"}],"has_more":false}`,
		},
		{
			name: "chart",
			call: func(ctx context.Context, client *Client) error {
				_, err := client.GetTradingViewChartData(ctx, "BTC-PERPETUAL", 1778722800000, 1778726400000, "60")
				return err
			},
			path: "/api/v2/public/get_tradingview_chart_data",
			query: map[string]string{
				"instrument_name": "BTC-PERPETUAL",
				"start_timestamp": "1778722800000",
				"end_timestamp":   "1778726400000",
				"resolution":      "60",
			},
			result: `{"status":"ok","ticks":[1778722800000],"open":[1],"high":[2],"low":[0.5],"close":[1.5],"volume":[3],"cost":[4]}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient().WithBaseURL("https://example.test")
			client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, tc.path, r.URL.Path)
				for key, value := range tc.query {
					require.Equal(t, value, r.URL.Query().Get(key), key)
				}
				return jsonHTTPResponse(`{"jsonrpc":"2.0","result":` + tc.result + `}`), nil
			})}

			require.NoError(t, tc.call(context.Background(), client))
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
