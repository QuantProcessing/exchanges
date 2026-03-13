
package spot

import (
	"context"
	"testing"
)

func TestGetExchangeInfo(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	info, err := client.GetExchangeInfo(ctx)
	if err != nil {
		t.Fatalf("Failed to get exchange info: %v", err)
	}

	t.Logf("Exchange info: %v", info)
}

func TestGetTicker(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	ticker, err := client.GetTicker(ctx, "90000002")
	if err != nil {
		t.Fatalf("Failed to get ticker: %v", err)
	}

	t.Logf("Ticker: %v", ticker)
}

func TestGetKLine(t *testing.T) {
	client := NewClient()
	ctx := context.Background()

	kline, err := client.GetKline(ctx, "90000002", PriceTypeLastPrice, KlineTypeMinute30, 10, "", "", "")
	if err != nil {
		t.Fatalf("Failed to get kline: %v", err)
	}

	t.Logf("Kline: %v", kline)
}
