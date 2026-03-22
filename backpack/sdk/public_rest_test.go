package sdk

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetKlinesBuildsExpectedQuery(t *testing.T) {
	t.Parallel()

	var gotQuery map[string]string
	client := NewClient()
	client.baseURL = "https://example.test"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotQuery = map[string]string{
				"symbol":    r.URL.Query().Get("symbol"),
				"interval":  r.URL.Query().Get("interval"),
				"startTime": r.URL.Query().Get("startTime"),
				"endTime":   r.URL.Query().Get("endTime"),
				"priceType": r.URL.Query().Get("priceType"),
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`[]`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, err := client.GetKlines(context.Background(), "BTC_USDC", "1month", 1710000000, 1710003600, "Last")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"symbol":    "BTC_USDC",
		"interval":  "1month",
		"startTime": "1710000000",
		"endTime":   "1710003600",
		"priceType": "Last",
	}, gotQuery)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
