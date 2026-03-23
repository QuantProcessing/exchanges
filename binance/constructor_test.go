package binance

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	perpsdk "github.com/QuantProcessing/exchanges/binance/sdk/perp"
	spotsdk "github.com/QuantProcessing/exchanges/binance/sdk/spot"
	"github.com/stretchr/testify/require"
)

func TestNewAdapterWithClientAllowsPublicOnlyConstruction(t *testing.T) {
	client := newBinancePerpTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/fapi/v1/exchangeInfo", r.URL.Path)
		return binanceJSONHTTPResponse(`{"timezone":"UTC","serverTime":1,"symbols":[{"symbol":"BTCUSDT","pair":"BTCUSDT","contractType":"PERPETUAL","status":"TRADING","baseAsset":"BTC","quoteAsset":"USDT","marginAsset":"USDT","pricePrecision":1,"quantityPrecision":3,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.1"},{"filterType":"LOT_SIZE","minQty":"0.001","stepSize":"0.001"},{"filterType":"MIN_NOTIONAL","notional":"5"}]}]}`), nil
	})

	adp, err := newPerpAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newPerpAdapterWithClient(context.Background(), Options{APIKey: "key"}, exchanges.QuoteCurrencyUSDT, newBinancePerpTestClient(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
		return nil, nil
	}))
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func TestNewSpotAdapterWithClientAllowsPublicOnlyConstruction(t *testing.T) {
	client := newBinanceSpotTestClient(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v3/exchangeInfo", r.URL.Path)
		return binanceJSONHTTPResponse(`{"timezone":"UTC","serverTime":1,"symbols":[{"symbol":"BTCUSDT","status":"TRADING","baseAsset":"BTC","baseAssetPrecision":6,"quoteAsset":"USDT","quotePrecision":2,"filters":[{"filterType":"PRICE_FILTER","tickSize":"0.01"},{"filterType":"LOT_SIZE","minQty":"0.0001","stepSize":"0.0001"}]}]}`), nil
	})

	adp, err := newSpotAdapterWithClient(context.Background(), Options{}, exchanges.QuoteCurrencyUSDT, client)
	require.NoError(t, err)
	require.NotNil(t, adp)
}

func TestNewSpotAdapterWithClientRejectsPartialCredentials(t *testing.T) {
	_, err := newSpotAdapterWithClient(context.Background(), Options{SecretKey: "secret"}, exchanges.QuoteCurrencyUSDT, newBinanceSpotTestClient(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
		return nil, nil
	}))
	require.ErrorIs(t, err, exchanges.ErrAuthFailed)
}

func newBinancePerpTestClient(fn func(*http.Request) (*http.Response, error)) *perpsdk.Client {
	client := perpsdk.NewClient()
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{Transport: binanceConstructorRoundTripFunc(fn)}
	return client
}

func newBinanceSpotTestClient(fn func(*http.Request) (*http.Response, error)) *spotsdk.Client {
	client := spotsdk.NewClient().WithBaseURL("https://example.test")
	client.HTTPClient = &http.Client{Transport: binanceConstructorRoundTripFunc(fn)}
	return client
}

type binanceConstructorRoundTripFunc func(*http.Request) (*http.Response, error)

func (f binanceConstructorRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func binanceJSONHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
