package okx

import (
	"context"
	"testing"
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
