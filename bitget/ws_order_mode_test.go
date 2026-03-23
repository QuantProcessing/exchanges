package bitget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"unsafe"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUTAOrderModeWSRoutesPlaceOrderToWS(t *testing.T) {
	var restHits atomic.Int32
	restServer := newRejectingRESTServer(t, &restHits)
	wsServer := newPrivateTradeWSServer(t, false)

	adp := newUTASpotOrderModeTestAdapter(t, restServer.URL, wsServer)
	adp.SetOrderMode(exchanges.OrderModeWS)

	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("100"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cid-uta",
	})
	require.NoError(t, err)
	require.Equal(t, int32(0), restHits.Load(), "OrderModeWS should avoid the REST place-order path")
	require.Equal(t, "ws-order", order.OrderID)
}

func TestClassicOrderModeWSRoutesPlaceOrderToWS(t *testing.T) {
	var restHits atomic.Int32
	restServer := newRejectingRESTServer(t, &restHits)
	wsServer := newPrivateTradeWSServer(t, true)

	adp := newClassicSpotOrderModeTestAdapter(t, restServer.URL, wsServer)
	adp.SetOrderMode(exchanges.OrderModeWS)

	order, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:      "BTC",
		Side:        exchanges.OrderSideBuy,
		Type:        exchanges.OrderTypeLimit,
		Quantity:    decimal.RequireFromString("0.1"),
		Price:       decimal.RequireFromString("100"),
		TimeInForce: exchanges.TimeInForceGTC,
		ClientID:    "cid-classic",
	})
	require.NoError(t, err)
	require.Equal(t, int32(0), restHits.Load(), "OrderModeWS should avoid the classic REST place-order path")
	require.Equal(t, "ws-order", order.OrderID)
}

func TestClassicOrderModeWSRoutesCancelOrderToWS(t *testing.T) {
	var restHits atomic.Int32
	restServer := newRejectingRESTServer(t, &restHits)
	wsServer := newPrivateTradeWSServer(t, true)

	adp := newClassicPerpOrderModeTestAdapter(t, restServer.URL, wsServer)
	adp.SetOrderMode(exchanges.OrderModeWS)

	err := adp.CancelOrder(context.Background(), "cancel-me", "BTC")
	require.NoError(t, err)
	require.Equal(t, int32(0), restHits.Load(), "OrderModeWS should avoid the classic REST cancel-order path")
}

func newRejectingRESTServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"rest-hit","msg":"REST should not be used in this test"}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func newPrivateTradeWSServer(t *testing.T, classic bool) string {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}

			if strings.Contains(string(payload), `"op":"login"`) {
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"login","code":"0","msg":"success"}`)))
				continue
			}

			resp := buildTradeAck(t, payload, classic)
			require.NoError(t, conn.WriteMessage(websocket.TextMessage, resp))
			return
		}
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func buildTradeAck(t *testing.T, payload []byte, classic bool) []byte {
	t.Helper()

	if classic {
		var req struct {
			Args []struct {
				ID      string `json:"id"`
				InstType string `json:"instType"`
				Channel string `json:"channel"`
				InstID  string `json:"instId"`
			} `json:"args"`
		}
		require.NoError(t, json.Unmarshal(payload, &req))
		require.NotEmpty(t, req.Args)

		resp := map[string]any{
			"event": "trade",
			"arg": []map[string]any{{
				"id":       req.Args[0].ID,
				"instType": req.Args[0].InstType,
				"channel":  req.Args[0].Channel,
				"instId":   req.Args[0].InstID,
				"params": map[string]any{
					"orderId":   "ws-order",
					"clientOid": "ws-client",
				},
			}},
			"code": 0,
			"msg":  "Success",
		}
		out, err := json.Marshal(resp)
		require.NoError(t, err)
		return out
	}

	var req struct {
		ID       string `json:"id"`
		Topic    string `json:"topic"`
		Category string `json:"category"`
		Args     []struct {
			Symbol string `json:"symbol"`
		} `json:"args"`
	}
	require.NoError(t, json.Unmarshal(payload, &req))
	require.NotEmpty(t, req.Args)

	resp := map[string]any{
		"event":    "trade",
		"id":       req.ID,
		"category": req.Category,
		"topic":    req.Topic,
		"args": []map[string]any{{
			"symbol":    req.Args[0].Symbol,
			"orderId":   "ws-order",
			"clientOid": "ws-client",
			"cTime":     "1710000000000",
		}},
		"code": "0",
		"msg":  "success",
		"ts":   "1710000000001",
	}
	out, err := json.Marshal(resp)
	require.NoError(t, err)
	return out
}

func newUTASpotOrderModeTestAdapter(t *testing.T, restURL, wsURL string) *SpotAdapter {
	t.Helper()

	client := sdk.NewClient().
		WithBaseURL(restURL).
		WithCredentials("api-key", "secret-key", "passphrase")
	wsClient := sdk.NewPrivateWSClient().
		WithCredentials("api-key", "secret-key", "passphrase")
	setPrivateWSURL(t, wsClient, wsURL)

	adp := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeSpot, exchanges.NopLogger),
		client:      client,
		privateWS:   wsClient,
		quote:       exchanges.QuoteCurrencyUSDT,
		markets: &marketCache{
			spotByBase: map[string]sdk.Instrument{"BTC": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot, Status: "online"}},
			bySymbol:   map[string]sdk.Instrument{"BTCUSDT": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot, Status: "online"}},
		},
	}
	adp.private = &utaSpotProfile{adp: adp}
	return adp
}

func newClassicSpotOrderModeTestAdapter(t *testing.T, restURL, wsURL string) *SpotAdapter {
	t.Helper()

	client := sdk.NewClient().
		WithBaseURL(restURL).
		WithCredentials("api-key", "secret-key", "passphrase")
	wsClient := sdk.NewPrivateWSClient().
		WithCredentials("api-key", "secret-key", "passphrase").
		WithClassicMode()
	setPrivateWSURL(t, wsClient, wsURL)

	adp := &SpotAdapter{
		BaseAdapter: exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypeSpot, exchanges.NopLogger),
		client:      client,
		privateWS:   wsClient,
		quote:       exchanges.QuoteCurrencyUSDT,
		markets: &marketCache{
			spotByBase: map[string]sdk.Instrument{"BTC": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot, Status: "online"}},
			bySymbol:   map[string]sdk.Instrument{"BTCUSDT": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categorySpot, Status: "online"}},
		},
	}
	adp.private = &classicSpotProfile{adp: adp}
	return adp
}

func newClassicPerpOrderModeTestAdapter(t *testing.T, restURL, wsURL string) *Adapter {
	t.Helper()

	client := sdk.NewClient().
		WithBaseURL(restURL).
		WithCredentials("api-key", "secret-key", "passphrase")
	wsClient := sdk.NewPrivateWSClient().
		WithCredentials("api-key", "secret-key", "passphrase").
		WithClassicMode()
	setPrivateWSURL(t, wsClient, wsURL)

	adp := &Adapter{
		BaseAdapter:  exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypePerp, exchanges.NopLogger),
		client:       client,
		privateWS:    wsClient,
		quote:        exchanges.QuoteCurrencyUSDT,
		perpCategory: categoryUSDTFutures,
		markets: &marketCache{
			perpByBase: map[string]sdk.Instrument{"BTC": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categoryUSDTFutures, Status: "online"}},
			bySymbol:   map[string]sdk.Instrument{"BTCUSDT": {Symbol: "BTCUSDT", BaseCoin: "BTC", QuoteCoin: "USDT", Category: categoryUSDTFutures, Status: "online"}},
		},
	}
	adp.private = &classicPerpProfile{adp: adp}
	return adp
}

func setPrivateWSURL(t *testing.T, client *sdk.PrivateWSClient, url string) {
	t.Helper()

	value := reflect.ValueOf(client).Elem().FieldByName("url")
	require.True(t, value.IsValid(), "PrivateWSClient.url should exist")
	reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().SetString(url)
}
