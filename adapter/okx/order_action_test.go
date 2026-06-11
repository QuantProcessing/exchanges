package okx

import (
	"context"
	"net/http"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPerpPlaceOrderReturnsInnerActionError(t *testing.T) {
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v5/public/instruments":
			require.Equal(t, "SWAP", r.URL.Query().Get("instType"))
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","instIdCode":123456,"baseCcy":"BTC","quoteCcy":"USDT","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.1","lotSz":"1","minSz":"1","instType":"SWAP","state":"live"}]}`), nil
		case "/api/v5/account/config":
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"posMode":"net_mode"}]}`), nil
		case "/api/v5/trade/order":
			return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"ordId":"","clOrdId":"cid-1","sCode":"51000","sMsg":"order rejected","subCode":"51149","ts":"1"}]}`), nil
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

	_, err = adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("1"),
		ClientID: "cid-1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sCode=51000")
	require.Contains(t, err.Error(), "subCode=51149")
}
