package perp

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
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
