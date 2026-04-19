package perp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
	"github.com/stretchr/testify/require"
)

// TestGetFundingRate tests the GetFundingRate method
// Note: This is an integration test that requires network access
func TestGetFundingRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	baseClient := hyperliquid.NewClient()
	client := NewClient(baseClient)
	ctx := context.Background()

	// Test with BTC
	fundingRate, err := client.GetFundingRate(ctx, "BTC")
	if err != nil {
		t.Fatalf("Failed to get funding rate for BTC: %v", err)
	}

	if fundingRate == nil {
		t.Fatal("Expected funding rate, got nil")
	}

	if fundingRate.Coin != "BTC" {
		t.Errorf("Expected coin BTC, got %s", fundingRate.Coin)
	}

	if fundingRate.FundingRate == "" {
		t.Error("Expected non-empty funding rate")
	}

	t.Logf("BTC funding rate: %s", fundingRate.FundingRate)
}

// TestGetFundingRate_InvalidCoin tests error handling for invalid coin
func TestGetFundingRate_InvalidCoin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	baseClient := hyperliquid.NewClient()
	client := NewClient(baseClient)
	ctx := context.Background()

	// Test with an invalid coin
	_, err := client.GetFundingRate(ctx, "INVALID_COIN_XYZ")
	if err == nil {
		t.Fatal("Expected error for invalid coin, got nil")
	}

	t.Logf("Got expected error: %v", err)
}

// TestGetAllFundingRates tests the GetAllFundingRates method
func TestGetAllFundingRates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	baseClient := hyperliquid.NewClient()
	client := NewClient(baseClient)
	ctx := context.Background()

	// Get all funding rates
	rates, err := client.GetAllFundingRates(ctx)
	if err != nil {
		t.Fatalf("Failed to get all funding rates: %v", err)
	}

	if len(rates) == 0 {
		t.Fatal("Expected at least one funding rate, got empty map")
	}

	// Check that BTC and ETH are present
	btcRate, hasBTC := rates["BTC"]
	if !hasBTC {
		t.Error("Expected BTC in funding rates map")
	} else {
		t.Logf("BTC funding rate: %s", btcRate)
	}

	ethRate, hasETH := rates["ETH"]
	if !hasETH {
		t.Error("Expected ETH in funding rates map")
	} else {
		t.Logf("ETH funding rate: %s", ethRate)
	}

	t.Logf("Total coins with funding rates: %d", len(rates))
}

func TestGetFundingRateHistoryParses(t *testing.T) {
	t.Parallel()

	payload := `[
		{"coin":"BTC","fundingRate":"0.0000125","premium":"0.0001","time":1700000000000},
		{"coin":"BTC","fundingRate":"0.0000130","premium":"0.0001","time":1700003600000}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/info", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		require.Contains(t, string(body), `"type":"fundingHistory"`)
		require.Contains(t, string(body), `"coin":"BTC"`)
		require.Contains(t, string(body), `"startTime":1700000000000`)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	baseClient := hyperliquid.NewClient()
	baseClient.BaseURL = srv.URL
	c := NewClient(baseClient)

	hist, err := c.GetFundingRateHistory(context.Background(), "BTC", 1700000000000, 0)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, "0.0000125", hist[0].FundingRate)
	require.Equal(t, int64(1700000000000), hist[0].Time)
}
