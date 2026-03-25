package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/require"
)

func TestDecibelRESTAddsBearerAuthorization(t *testing.T) {
	var (
		seenAuth   string
		seenOrigin string
		seenPath   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenOrigin = r.Header.Get("Origin")
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient("test-api-key")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	markets, err := client.GetMarkets(context.Background())
	require.NoError(t, err)
	require.Empty(t, markets)
	require.Equal(t, "Bearer test-api-key", seenAuth)
	require.NotEmpty(t, seenOrigin)
	require.Equal(t, "/api/v1/markets", seenPath)
}

func TestDecibelRESTNormalizesExchangeErrors(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
		want       error
	}{
		{
			name:       "auth",
			statusCode: http.StatusUnauthorized,
			body:       `{"code":"AUTH","message":"invalid token"}`,
			want:       exchanges.ErrAuthFailed,
		},
		{
			name:       "rate-limit",
			statusCode: http.StatusTooManyRequests,
			body:       `{"code":"RATE_LIMIT","message":"slow down"}`,
			want:       exchanges.ErrRateLimited,
		},
		{
			name:       "symbol-miss",
			statusCode: http.StatusNotFound,
			body:       `{"code":"MARKET_NOT_FOUND","message":"market not found"}`,
			want:       exchanges.ErrSymbolNotFound,
		},
		{
			name:       "precision",
			statusCode: http.StatusBadRequest,
			body:       `{"code":"BAD_PRECISION","message":"price precision exceeded"}`,
			want:       exchanges.ErrInvalidPrecision,
		},
		{
			name:       "order-miss",
			statusCode: http.StatusNotFound,
			body:       `{"code":"ORDER_NOT_FOUND","message":"order not found"}`,
			want:       exchanges.ErrOrderNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			client := NewClient("test-api-key")
			client.BaseURL = server.URL
			client.HTTPClient = server.Client()

			err := client.get(context.Background(), "/api/v1/markets", nil, &[]Market{})
			require.Error(t, err)
			require.ErrorIs(t, err, tc.want)

			var exchangeErr *exchanges.ExchangeError
			require.ErrorAs(t, err, &exchangeErr)
			require.Equal(t, "DECIBEL", exchangeErr.Exchange)
		})
	}
}
