package grvt

import (
	"context"
	"testing"
)

// TestGetTickerFundingFields tests retrieving raw current funding fields from ticker.
func TestGetTickerFundingFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := newLiveClient()
	ctx := context.Background()

	var ticker *GetTickerResponse
	retryGRVTLive(t, "GetTicker", func() error {
		var err error
		ticker, err = client.GetTicker(ctx, "BTC_USDT_Perp")
		return err
	})

	if ticker == nil {
		t.Fatal("Expected ticker, got nil")
	}

	if ticker.Result.Instrument == "" {
		t.Error("Expected non-empty instrument")
	}

	if ticker.Result.FundingRate == "" {
		t.Error("Expected non-empty funding rate")
	}

	t.Logf("Instrument: %s", ticker.Result.Instrument)
	t.Logf("Funding rate: %s", ticker.Result.FundingRate)
	t.Logf("Next funding time: %s", ticker.Result.NextFundingTime)
}
