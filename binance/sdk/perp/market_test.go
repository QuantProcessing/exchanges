package perp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetFundingRate tests retrieving funding rate for a specific symbol
func TestGetFundingRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := NewClient()
	ctx := context.Background()

	// Test with BTCUSDT
	rate, err := client.GetFundingRate(ctx, "BTCUSDT")
	if err != nil {
		t.Fatalf("Failed to get funding rate: %v", err)
	}

	if rate == nil {
		t.Fatal("Expected funding rate, got nil")
	}

	if rate.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", rate.Symbol)
	}

	if rate.LastFundingRate == "" {
		t.Error("Expected non-empty funding rate")
	}

	t.Logf("BTCUSDT funding rate: %s", rate.LastFundingRate)
	t.Logf("Next funding time: %d", rate.NextFundingTime)
}

// TestGetAllFundingRates tests retrieving all funding rates
func TestGetAllFundingRates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := NewClient()
	ctx := context.Background()

	rates, err := client.GetAllFundingRates(ctx)
	if err != nil {
		t.Fatalf("Failed to get all funding rates: %v", err)
	}

	if len(rates) == 0 {
		t.Fatal("Expected at least one funding rate, got empty array")
	}

	t.Logf("Total symbols with funding rates: %d", len(rates))

	// Show first 3 rates
	for i, rate := range rates {
		if i >= 3 {
			break
		}
		t.Logf("%s: rate=%s", rate.Symbol, rate.LastFundingRate)
	}
}

func TestGetOpenInterestParses(t *testing.T) {
	t.Parallel()

	payload := `{"symbol":"BTCUSDT","openInterest":"12345.678","time":1700000000000}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/fapi/v1/openInterest", r.URL.Path)
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	oi, err := c.GetOpenInterest(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", oi.Symbol)
	require.Equal(t, "12345.678", oi.OpenInterest)
	require.Equal(t, int64(1700000000000), oi.Time)
}

func TestGetFundingRateHistoryParses(t *testing.T) {
	t.Parallel()

	payload := `[
		{"symbol":"BTCUSDT","fundingRate":"0.0001","fundingTime":1700000000000,"markPrice":"50000"},
		{"symbol":"BTCUSDT","fundingRate":"0.00012","fundingTime":1700028800000,"markPrice":"50100"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/fapi/v1/fundingRate", r.URL.Path)
		require.Equal(t, "BTCUSDT", r.URL.Query().Get("symbol"))
		require.Equal(t, "5", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	hist, err := c.GetFundingRateHistory(context.Background(), "BTCUSDT", 0, 0, 5)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "0.0001", hist[0].FundingRate)
	require.Equal(t, int64(1700028800000), hist[1].FundingTime)
}
