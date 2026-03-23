package bitget

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/stretchr/testify/require"
)

func TestNewSpotAdapterAllowsPublicOnlyInit(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v3/market/instruments", r.URL.Path)
		return jsonHTTPResponse(`{"code":"00000","msg":"success","requestTime":1,"data":[{"symbol":"BTCUSDT","category":"SPOT","baseCoin":"BTC","quoteCoin":"USDT","minOrderQty":"0.0001","minOrderAmount":"5","pricePrecision":"2","quantityPrecision":"4","status":"online"}]}`), nil
	})

	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewPerpAdapterRejectsPartialCredentials(t *testing.T) {
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(`{"code":"00000","msg":"success","requestTime":1,"data":[{"symbol":"BTCUSDT","category":"USDT-FUTURES","baseCoin":"BTC","quoteCoin":"USDT","minOrderQty":"0.001","minOrderAmount":"5","pricePrecision":"1","quantityPrecision":"3","status":"online"}]}`), nil
	})

	_, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{
		APIKey: "key",
	}, exchanges.QuoteCurrencyUSDT, client)
	require.Error(t, err)
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestNewPerpAdapterWithCredentialsDoesNotProbeAccountSettings(t *testing.T) {
	calledSettings := false
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/market/instruments":
			return jsonHTTPResponse(`{"code":"00000","msg":"success","requestTime":1,"data":[{"symbol":"BTCUSDT","category":"USDT-FUTURES","baseCoin":"BTC","quoteCoin":"USDT","minOrderQty":"0.001","minOrderAmount":"5","pricePrecision":"1","quantityPrecision":"3","status":"online"}]}`), nil
		case "/api/v3/account/settings":
			calledSettings = true
			return nil, errors.New("unexpected account settings probe")
		default:
			return nil, errors.New("unexpected path")
		}
	})
	client.WithCredentials("key", "secret", "pass")

	adp, err := newPerpAdapterWithClient(context.Background(), func() {}, Options{
		APIKey:     "key",
		SecretKey:  "secret",
		Passphrase: "pass",
	}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
	require.False(t, calledSettings)
}

func TestNewSpotAdapterWithCredentialsDoesNotProbeAccountSettings(t *testing.T) {
	calledSettings := false
	client := newTestClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/v3/market/instruments":
			return jsonHTTPResponse(`{"code":"00000","msg":"success","requestTime":1,"data":[{"symbol":"BTCUSDT","category":"SPOT","baseCoin":"BTC","quoteCoin":"USDT","minOrderQty":"0.0001","minOrderAmount":"5","pricePrecision":"2","quantityPrecision":"4","status":"online"}]}`), nil
		case "/api/v3/account/settings":
			calledSettings = true
			return nil, errors.New("unexpected account settings probe")
		default:
			return nil, errors.New("unexpected path")
		}
	})
	client.WithCredentials("key", "secret", "pass")

	adp, err := newSpotAdapterWithClient(context.Background(), func() {}, Options{
		APIKey:     "key",
		SecretKey:  "secret",
		Passphrase: "pass",
	}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
	require.False(t, calledSettings)
}

func newTestClient(fn func(*http.Request) (*http.Response, error)) *sdk.Client {
	return sdk.NewClient().
		WithBaseURL("https://example.test").
		WithHTTPClient(&http.Client{Transport: roundTripFunc(fn)})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
