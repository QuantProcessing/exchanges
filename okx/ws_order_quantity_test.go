package okx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpPlaceOrderWSConvertsCoinQuantityToContractsForMarketOrders(t *testing.T) {
	type capturedRequest struct {
		Op   string `json:"op"`
		ID   string `json:"id"`
		Args []struct {
			InstIdCode int64  `json:"instIdCode"`
			Sz         string `json:"sz"`
			OrdType    string `json:"ordType"`
		} `json:"args"`
	}

	var captured capturedRequest
	wsURL := newOKXOrderActionWSServer(t, func(payload []byte) []byte {
		require.NoError(t, json.Unmarshal(payload, &captured))
		require.Len(t, captured.Args, 1)
		return []byte(`{"id":"` + captured.ID + `","code":"0","msg":"","data":[{"ordId":"order-1","clOrdId":"cid-1","sCode":"0","sMsg":"","subCode":"","ts":"1"}]}`)
	})

	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","instIdCode":123456,"baseCcy":"BTC","quoteCcy":"USDT","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.1","lotSz":"1","minSz":"1","instType":"SWAP","state":"live"}]}`), nil
		case "/api/v5/account/config":
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"posMode":"net_mode"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
			return nil, nil
		}
	})

	adp, err := newPerpAdapterWithClient(
		context.Background(),
		Options{APIKey: "key", SecretKey: "secret", Passphrase: "pass"},
		exchanges.QuoteCurrencyUSDT,
		client,
	)
	require.NoError(t, err)

	adp.wsPrivate.URL = wsURL
	err = adp.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("1"),
		ClientID: "cid-1",
	})
	require.NoError(t, err)
	require.Equal(t, "order", captured.Op)
	require.Equal(t, "market", captured.Args[0].OrdType)
	require.Equal(t, int64(123456), captured.Args[0].InstIdCode)
	require.Equal(t, "100", captured.Args[0].Sz)
}

func newOKXOrderActionWSServer(t *testing.T, onAction func(payload []byte) []byte) string {
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
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"login","code":"0","msg":""}`)))
				continue
			}

			require.NoError(t, conn.WriteMessage(websocket.TextMessage, onAction(payload)))
			return
		}
	}))
	t.Cleanup(server.Close)

	return "ws" + strings.TrimPrefix(server.URL, "http")
}
