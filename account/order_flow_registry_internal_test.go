package account

import (
	"fmt"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderFlowRegistryReplaysPendingFillsAfterOrderIDBinding(t *testing.T) {
	t.Parallel()

	registry := newOrderFlowRegistry()
	flow := registry.Register(&exchanges.Order{ClientOrderID: "cli-1"})
	defer flow.Close()

	registry.RouteFill(&exchanges.Fill{
		TradeID:   "trade-1",
		OrderID:   "exch-1",
		Price:     decimal.RequireFromString("101"),
		Quantity:  decimal.RequireFromString("0.25"),
		Timestamp: 1,
	})

	select {
	case <-flow.Fills():
		t.Fatal("fill should stay pending until the order id is bound")
	case <-time.After(50 * time.Millisecond):
	}

	registry.RouteOrder(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Quantity:      decimal.RequireFromString("1"),
		Status:        exchanges.OrderStatusNew,
	})

	select {
	case fill := <-flow.Fills():
		require.Equal(t, "trade-1", fill.TradeID)
	case <-time.After(time.Second):
		t.Fatal("expected pending fill replay")
	}
}

func TestOrderFlowRegistryCapsPendingFillsPerKey(t *testing.T) {
	t.Parallel()

	registry := newOrderFlowRegistry()
	flow := registry.Register(&exchanges.Order{ClientOrderID: "cli-1"})
	defer flow.Close()

	for i := 0; i < 40; i++ {
		registry.RouteFill(&exchanges.Fill{
			TradeID:   fmt.Sprintf("trade-%02d", i),
			OrderID:   "exch-1",
			Price:     decimal.RequireFromString("100"),
			Quantity:  decimal.RequireFromString("0.01"),
			Timestamp: int64(i),
		})
	}

	registry.RouteOrder(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Quantity:      decimal.RequireFromString("1"),
		Status:        exchanges.OrderStatusNew,
	})

	seen := make([]string, 0, 32)
	for i := 0; i < 32; i++ {
		select {
		case fill := <-flow.Fills():
			seen = append(seen, fill.TradeID)
		case <-time.After(time.Second):
			t.Fatalf("expected replayed fill %d", i)
		}
	}

	require.Equal(t, "trade-08", seen[0])
	require.Equal(t, "trade-39", seen[len(seen)-1])
}
