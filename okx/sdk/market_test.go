package okx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetFundingRate tests the GetFundingRate method
func TestGetFundingRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := NewClient()
	ctx := context.Background()

	// Test with BTC-USDT-SWAP
	rate, err := client.GetFundingRate(ctx, "BTC-USDT-SWAP")
	if err != nil {
		t.Fatalf("Failed to get funding rate: %v", err)
	}

	if rate == nil {
		t.Fatal("Expected funding rate, got nil")
	}

	if rate.Symbol == "" {
		t.Error("Expected non-empty symbol")
	}

	t.Logf("Symbol: %s",  rate.Symbol)
	t.Logf("Funding rate (hourly): %s", rate.FundingRate)
	t.Logf("Funding interval hours: %d", rate.FundingIntervalHours)
	t.Logf("Next funding time: %s", rate.NextFundingTime)
}

func TestGetOpenInterestParses(t *testing.T) {
	t.Parallel()

	payload := `{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","instType":"SWAP","oi":"12345.6","oiCcy":"123.456","ts":"1700000000000"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v5/public/open-interest", r.URL.Path)
		require.Equal(t, "BTC-USDT-SWAP", r.URL.Query().Get("instId"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	oi, err := c.GetOpenInterest(context.Background(), "BTC-USDT-SWAP")
	require.NoError(t, err)
	require.Equal(t, "BTC-USDT-SWAP", oi.InstId)
	require.Equal(t, "12345.6", oi.OI)
	require.Equal(t, "123.456", oi.OICcy)
}

func TestGetFundingRateHistoryParses(t *testing.T) {
	t.Parallel()

	payload := `{"code":"0","msg":"","data":[
		{"instId":"BTC-USDT-SWAP","fundingRate":"0.0001","realizedRate":"0.0001","fundingTime":"1700000000000","method":"current_period"},
		{"instId":"BTC-USDT-SWAP","fundingRate":"0.00012","realizedRate":"0.00012","fundingTime":"1700028800000","method":"current_period"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v5/public/funding-rate-history", r.URL.Path)
		require.Equal(t, "BTC-USDT-SWAP", r.URL.Query().Get("instId"))
		require.Equal(t, "5", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	hist, err := c.GetFundingRateHistory(context.Background(), "BTC-USDT-SWAP", 0, 0, 5)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "0.0001", hist[0].FundingRate)
	require.Equal(t, "1700028800000", hist[1].FundingTime)
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

	t.Logf("Total instruments with funding rates: %d", len(rates))

	// Show first 3 rates
	for i, rate := range rates {
		if i >= 3 {
			break
		}
		t.Logf("%s: rate=%s (hourly), interval=%dh", rate.Symbol, rate.FundingRate, rate.FundingIntervalHours)
	}
}
