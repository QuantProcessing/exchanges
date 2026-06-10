package binance

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/binance/sdk/option"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type optionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f optionRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newOptionAdapterWithRoundTrip(fn optionRoundTripFunc) *OptionAdapter {
	return &OptionAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("BINANCE_OPTION", exchanges.MarketTypeOption, exchanges.NopLogger),
		client: &option.Client{
			BaseURL:    "https://example.test",
			APIKey:     "key",
			SecretKey:  "secret",
			HTTPClient: &http.Client{Transport: fn},
			Logger:     zap.NewNop().Sugar(),
		},
		apiKey:    "key",
		secretKey: "secret",
	}
}

func TestOptionAdapterRejectsMarketOrdersLocally(t *testing.T) {
	t.Parallel()

	adp := newOptionAdapterWithRoundTrip(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("market option order should be rejected before HTTP request")
		return nil, nil
	})

	_, err := adp.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:   "BTC-251226-100000-C",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("1"),
	})
	require.ErrorIs(t, err, exchanges.ErrNotSupported)
}

func TestOptionAdapterMapsSignedShortQuantityToAbsoluteContracts(t *testing.T) {
	t.Parallel()

	adp := newOptionAdapterWithRoundTrip(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Path {
		case "/eapi/v1/position":
			body = `[{
				"entryPrice":"1000",
				"symbol":"BTC-251226-100000-C",
				"side":"SHORT",
				"quantity":"-0.1",
				"unrealizedPNL":"-5",
				"markPrice":"1050",
				"strikePrice":"100000",
				"expiryDate":1766736000000,
				"priceScale":2,
				"quantityScale":2,
				"optionSide":"CALL",
				"quoteAsset":"USDT",
				"time":1762872654561
			}]`
		case "/eapi/v1/mark":
			require.Equal(t, "BTC-251226-100000-C", r.URL.Query().Get("symbol"))
			body = `[{
				"symbol":"BTC-251226-100000-C",
				"markPrice":"1050",
				"markIV":"0.7",
				"delta":"0.4",
				"theta":"-12",
				"gamma":"0.01",
				"vega":"3"
			}]`
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	positions, err := adp.FetchOptionPositions(context.Background())
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, exchanges.PositionSideShort, positions[0].Side)
	require.True(t, positions[0].Quantity.Equal(decimal.RequireFromString("0.1")))
	require.True(t, positions[0].Option.Greeks.Delta.Equal(decimal.RequireFromString("0.4")))
}
