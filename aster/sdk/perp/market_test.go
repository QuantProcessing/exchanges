package perp

import (
	"context"
	"fmt"
	"testing"
)

func TestGetKlines(t *testing.T) {
	client := NewClient()
	res, err := client.ContinousKlines(context.Background(), "BTCUSDT", "PERPETUAL", "1m", 10, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(res)
}

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
