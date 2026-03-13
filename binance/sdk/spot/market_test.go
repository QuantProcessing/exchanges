package spot

import (
	"context"
	"fmt"
	"testing"
)

func Test_GetDepth(t *testing.T) {
	ctx := context.Background()
	client := NewClient()

	orderbooks, err := client.Depth(ctx, "BTCUSDT", 100)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(orderbooks)
}

func Test_GetTicker(t *testing.T) {
	ctx := context.Background()
	client := NewClient()

	ticker, err := client.Ticker(ctx, "BTCUSDT")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(ticker)
}
