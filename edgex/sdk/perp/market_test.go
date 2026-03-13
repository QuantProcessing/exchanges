
package perp

import (
	"context"
	"testing"
)

// TestGetFundingRate tests retrieving funding rate for a specific contract
func TestGetFundingRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := NewClient()
	ctx := context.Background()

	// Test with BTC contract (contractId: 10000001 from API example)
	// Note: You may need to adjust the contractId based on actual EdgeX contracts
	rate, err := client.GetFundingRate(ctx, "10000001")
	if err != nil {
		t.Fatalf("Failed to get funding rate: %v", err)
	}

	if rate == nil {
		t.Fatal("Expected funding rate, got nil")
	}

	if rate.ContractId == "" {
		t.Error("Expected non-empty contract ID")
	}

	if rate.FundingRate == "" {
		t.Error("Expected non-empty funding rate")
	}

	t.Logf("Contract %s funding rate: %s", rate.ContractId, rate.FundingRate)
	t.Logf("Index price: %s", rate.IndexPrice)
	t.Logf("Oracle price: %s", rate.OraclePrice)
}

// TestGetAllFundingRates tests retrieving all funding rates
// Note: EdgeX API may not return all rates when contractId is not specified
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

	// EdgeX may return empty array when no contractId is specified
	// This is expected behavior and not an error
	t.Logf("Total contracts with funding rates: %d", len(rates))
	
	if len(rates) > 0 {
		// Show first 3 rates if available
		for i, rate := range rates {
			if i >= 3 {
				break
			}
			t.Logf("Contract %s: rate=%s, indexPrice=%s", 
				rate.ContractId, rate.FundingRate, rate.IndexPrice)
		}
	} else {
		t.Log("No funding rates returned (API behavior when contractId not specified)")
	}
}
