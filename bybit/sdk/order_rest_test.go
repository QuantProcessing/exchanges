package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlaceOrderPostsBody(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v5/order/create", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "spot", payload["category"])
			require.Equal(t, "BTCUSDT", payload["symbol"])
			require.Equal(t, "baseCoin", payload["marketUnit"])
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"orderId":"1","orderLinkId":"cid-1"}}`), nil
		}),
	}

	got, err := client.PlaceOrder(context.Background(), PlaceOrderRequest{
		Category:    "spot",
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		OrderType:   "Market",
		Qty:         "2",
		MarketUnit:  "baseCoin",
		OrderLinkID: "cid-1",
	})
	require.NoError(t, err)
	require.Equal(t, "1", got.OrderID)
}

func TestCancelOrderPostsBody(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v5/order/cancel", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "linear", payload["category"])
			require.Equal(t, "BTCUSDT", payload["symbol"])
			require.Equal(t, "1", payload["orderId"])
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"orderId":"1","orderLinkId":"cid-1"}}`), nil
		}),
	}

	_, err := client.CancelOrder(context.Background(), CancelOrderRequest{
		Category: "linear",
		Symbol:   "BTCUSDT",
		OrderID:  "1",
	})
	require.NoError(t, err)
}

func TestCancelAllOrdersPostsBody(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v5/order/cancel-all", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "spot", payload["category"])
			require.Equal(t, "BTCUSDT", payload["symbol"])
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"success":"1"}}`), nil
		}),
	}

	err := client.CancelAllOrders(context.Background(), CancelAllOrdersRequest{
		Category: "spot",
		Symbol:   "BTCUSDT",
	})
	require.NoError(t, err)
}

func TestAmendOrderPostsBody(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v5/order/amend", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload map[string]string
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "linear", payload["category"])
			require.Equal(t, "BTCUSDT", payload["symbol"])
			require.Equal(t, "1", payload["orderId"])
			require.Equal(t, "0.2", payload["qty"])
			require.Equal(t, "101", payload["price"])
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"orderId":"1","orderLinkId":"cid-1"}}`), nil
		}),
	}

	_, err := client.AmendOrder(context.Background(), AmendOrderRequest{
		Category: "linear",
		Symbol:   "BTCUSDT",
		OrderID:  "1",
		Qty:      "0.2",
		Price:    "101",
	})
	require.NoError(t, err)
}

func TestGetOpenOrdersParsesList(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/order/realtime", r.URL.Path)
			require.Equal(t, "spot", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"1","orderLinkId":"cid-1","symbol":"BTCUSDT","side":"Buy","orderType":"Limit","timeInForce":"GTC","price":"50000","qty":"0.1","cumExecQty":"0","avgPrice":"0","orderStatus":"New","reduceOnly":false,"createdTime":"1710000000000","updatedTime":"1710000000001"}]}}`), nil
		}),
	}

	got, err := client.GetOpenOrders(context.Background(), "spot", "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "1", got[0].OrderID)
}

func TestGetOrderHistoryParsesList(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/order/history", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"1","orderLinkId":"cid-1","symbol":"BTCUSDT","side":"Sell","orderType":"Market","timeInForce":"IOC","price":"0","qty":"0.1","cumExecQty":"0.1","avgPrice":"50010","orderStatus":"Filled","reduceOnly":true,"createdTime":"1710000000000","updatedTime":"1710000000002"}]}}`), nil
		}),
	}

	got, err := client.GetOrderHistory(context.Background(), "linear", "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Filled", got[0].OrderStatus)
}

func TestGetOrderHistoryFilteredPassesOrderID(t *testing.T) {
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v5/order/history", r.URL.Path)
			require.Equal(t, "linear", r.URL.Query().Get("category"))
			require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
			require.Equal(t, "1", r.URL.Query().Get("orderId"))
			return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[]}}`), nil
		}),
	}

	_, err := client.GetOrderHistoryFiltered(context.Background(), "linear", "BTCUSDT", "1", "")
	require.NoError(t, err)
}

func TestGetOrderHistoryPaginates(t *testing.T) {
	hits := 0
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			hits++
			require.Equal(t, "/v5/order/history", r.URL.Path)
			switch hits {
			case 1:
				require.Empty(t, r.URL.Query().Get("cursor"))
				return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"1"}],"nextPageCursor":"cursor-2"}}`), nil
			case 2:
				require.Equal(t, "cursor-2", r.URL.Query().Get("cursor"))
				return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"2"}],"nextPageCursor":""}}`), nil
			default:
				t.Fatalf("unexpected request %d", hits)
				return nil, nil
			}
		}),
	}

	got, err := client.GetOrderHistory(context.Background(), "linear", "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestGetRealtimeOrdersPaginates(t *testing.T) {
	hits := 0
	client := NewClient().WithCredentials("api-key", "secret-key")
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			hits++
			require.Equal(t, "/v5/order/realtime", r.URL.Path)
			switch hits {
			case 1:
				require.Empty(t, r.URL.Query().Get("cursor"))
				return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"1"}],"nextPageCursor":"cursor-2"}}`), nil
			case 2:
				require.Equal(t, "cursor-2", r.URL.Query().Get("cursor"))
				return jsonResponse(`{"retCode":0,"retMsg":"OK","result":{"list":[{"orderId":"2"}],"nextPageCursor":""}}`), nil
			default:
				t.Fatalf("unexpected request %d", hits)
				return nil, nil
			}
		}),
	}

	got, err := client.GetRealtimeOrders(context.Background(), "linear", "BTCUSDT", "", "", "", 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
}
