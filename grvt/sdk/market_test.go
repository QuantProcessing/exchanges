//go:build grvt

package grvt

import (
	"context"
	"testing"
)

// TestGetFundingRate tests retrieving real-time funding rate for a specific instrument
func TestGetFundingRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := NewClient()
	ctx := context.Background()

	// Test with BTC_USDT_Perp (GRVT uses this format)
	rate, err := client.GetFundingRate(ctx, "BTC_USDT_Perp")
	if err != nil {
		t.Fatalf("Failed to get funding rate: %v", err)
	}

	if rate == nil {
		t.Fatal("Expected funding rate, got nil")
	}

	if rate.Instrument == "" {
		t.Error("Expected non-empty instrument")
	}

	if rate.FundingRate == "" {
		t.Error("Expected non-empty funding rate")
	}

	t.Logf("Instrument: %s", rate.Instrument)
	t.Logf("Funding rate: %s", rate.FundingRate)
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

	t.Logf("Total perpetual instruments with funding rates: %d", len(rates))
	
	// Show first 3 rates
	for i, rate := range rates {
		if i >= 3 {
			break
		}
		t.Logf("%s: rate=%s", rate.Instrument, rate.FundingRate)
	}
}
