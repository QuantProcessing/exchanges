package okx

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	okxsdk "github.com/QuantProcessing/exchanges/okx/sdk"
	"github.com/stretchr/testify/require"
)

func TestNewAdapterWithClientAllowsPublicOnlyConstruction(t *testing.T) {
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v5/public/instruments", r.URL.Path)
		require.Equal(t, "SWAP", r.URL.Query().Get("instType"))
		return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","baseCcy":"BTC","quoteCcy":"USDT","ctVal":"0.01","ctValCcy":"BTC","tickSz":"0.1","lotSz":"1","minSz":"1","instType":"SWAP","state":"live"}]}`), nil
	})

	adp, err := newPerpAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newPerpAdapterWithClient(context.Background(), Options{APIKey: "key"}, exchanges.QuoteCurrencyUSDT, newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
		return nil, nil
	}))
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestNewSpotAdapterWithClientAllowsPublicOnlyConstruction(t *testing.T) {
	client := newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v5/public/instruments", r.URL.Path)
		require.Equal(t, "SPOT", r.URL.Query().Get("instType"))
		return okxJSONHTTPResponse(`{"code":"0","msg":"","data":[{"instId":"BTC-USDT","baseCcy":"BTC","quoteCcy":"USDT","tickSz":"0.01","lotSz":"0.0001","minSz":"0.0001","instType":"SPOT","state":"live"}]}`), nil
	})

	adp, err := newSpotAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewSpotAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newSpotAdapterWithClient(context.Background(), Options{SecretKey: "secret", Passphrase: "pass"}, exchanges.QuoteCurrencyUSDT, newOKXTestClient(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
		return nil, nil
	}))
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func newOKXTestClient(fn func(*http.Request) (*http.Response, error)) *okxsdk.Client {
	client := okxsdk.NewClient()
	client.HTTPClient = &http.Client{Transport: okxConstructorRoundTripFunc(fn)}
	return client
}

type okxConstructorRoundTripFunc func(*http.Request) (*http.Response, error)

func (f okxConstructorRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func okxJSONHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
