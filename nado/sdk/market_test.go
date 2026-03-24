package nado

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/internal/testenv"
)

func requireFullEnv(t *testing.T) {
	t.Helper()
	testenv.RequireFull(t, "NADO_PRIVATE_KEY", "NADO_SUBACCOUNT_NAME")
}

func GetEnv() (string, string) {
	pk := os.Getenv("NADO_PRIVATE_KEY")
	subaccount := os.Getenv("NADO_SUBACCOUNT_NAME")
	return pk, subaccount
}

func retryNadoPublic[T any](t *testing.T, op string, fn func() (T, error)) T {
	t.Helper()

	var zero T
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		value, err := fn()
		if err == nil {
			return value
		}
		lastErr = err
		lower := strings.ToLower(err.Error())
		if !strings.Contains(lower, "eof") && !strings.Contains(lower, "timeout") {
			t.Fatalf("%s failed: %v", op, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("%s failed after retries: %v", op, lastErr)
	return zero
}

func TestGetNonces(t *testing.T) {
	requireFullEnv(t)
	privateKey, subaccount := GetEnv()
	client, err := NewClient().WithCredentials(privateKey, subaccount)
	if err != nil {
		t.Fatal(err)
	}
	nonces, err := client.GetNonces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(nonces)
}

func TestGetCandlesticks(t *testing.T) {
	requireFullEnv(t)
	privateKey, subaccount := GetEnv()
	client, err := NewClient().WithCredentials(privateKey, subaccount)
	if err != nil {
		t.Fatal(err)
	}
	candlesticks := retryNadoPublic(t, "GetCandlesticks", func() ([]ArchiveCandlestick, error) {
		return client.GetCandlesticks(context.Background(), CandlestickRequest{
			Candlesticks: Candlesticks{
				ProductID:   1,
				Granularity: 60,
				Limit:       10,
			},
		})
	})
	fmt.Println(candlesticks)
}

func TestGetContracts(t *testing.T) {
	client := NewClient()
	contracts, err := client.GetContracts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(contracts)
}

func TestGetTickers(t *testing.T) {
	client := NewClient()
	tickers, err := client.GetTickers(context.Background(), MarketTypePerp, nil)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(tickers)
}

// TestGetFundingRate tests the GetFundingRate method
func TestGetFundingRate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	client := NewClient()
	ctx := context.Background()

	// Test with product ID 36 (commonly LIT-PERP in Nado)
	rate, err := client.GetFundingRate(ctx, 36)
	if err != nil {
		t.Fatalf("Failed to get funding rate: %v", err)
	}

	if rate == nil {
		t.Fatal("Expected funding rate, got nil")
	}

	if rate.ProductID != 36 {
		t.Errorf("Expected ProductID 36, got %d", rate.ProductID)
	}

	if rate.FundingIntervalHours != 1 {
		t.Errorf("Expected FundingIntervalHours to be 1, got %d", rate.FundingIntervalHours)
	}

	t.Logf("Product ID: %d", rate.ProductID)
	t.Logf("Symbol: %s", rate.Symbol)
	t.Logf("Funding rate (hourly): %s", rate.FundingRate)
	t.Logf("Funding interval hours: %d", rate.FundingIntervalHours)
	t.Logf("Funding time: %d", rate.FundingTime)
	t.Logf("Next funding time: %d", rate.NextFundingTime)
	t.Logf("Update time: %d", rate.UpdateTime)
}

// TestGetAllFundingRates tests the GetAllFundingRates method
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

	t.Logf("Total products with funding rates: %d", len(rates))

	// Show first 3 rates
	for i, rate := range rates {
		if i >= 3 {
			break
		}
		t.Logf("Product %d (%s): rate=%s (hourly), interval=%dh",
			rate.ProductID, rate.Symbol, rate.FundingRate, rate.FundingIntervalHours)
	}

	// Verify all have 1-hour interval
	for _, rate := range rates {
		if rate.FundingIntervalHours != 1 {
			t.Errorf("Expected all rates to have 1-hour interval, got %d for product %d",
				rate.FundingIntervalHours, rate.ProductID)
		}
	}
}
