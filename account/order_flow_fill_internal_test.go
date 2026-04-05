package account

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderFlowPublishFillEmitsRawFillAndMergedSnapshot(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Symbol:        "ETH",
		Side:          exchanges.OrderSideBuy,
		Type:          exchanges.OrderTypeLimit,
		Quantity:      decimal.RequireFromString("1"),
		OrderPrice:    decimal.RequireFromString("100"),
		Status:        exchanges.OrderStatusNew,
	})
	defer flow.Close()

	flow.publishFill(&exchanges.Fill{
		TradeID:       "trade-1",
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Symbol:        "ETH",
		Side:          exchanges.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.25"),
		Fee:           decimal.RequireFromString("0.01"),
		FeeAsset:      "USDT",
		Timestamp:     123,
	})

	select {
	case fill := <-flow.Fills():
		require.Equal(t, "trade-1", fill.TradeID)
		require.Equal(t, decimal.RequireFromString("101"), fill.Price)
	case <-time.After(time.Second):
		t.Fatal("expected raw fill")
	}

	select {
	case order := <-flow.C():
		require.Equal(t, exchanges.OrderStatusPartiallyFilled, order.Status)
		require.Equal(t, decimal.RequireFromString("0.25"), order.FilledQuantity)
		require.Equal(t, decimal.RequireFromString("0.25"), order.LastFillQuantity)
		require.Equal(t, decimal.RequireFromString("101"), order.LastFillPrice)
		require.Equal(t, decimal.RequireFromString("101"), order.AverageFillPrice)
	case <-time.After(time.Second):
		t.Fatal("expected merged order snapshot")
	}
}

func TestOrderFlowRawFilledConfirmationWaitsForFillDrivenTerminal(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Symbol:        "ETH",
		Side:          exchanges.OrderSideBuy,
		Type:          exchanges.OrderTypeLimit,
		Quantity:      decimal.RequireFromString("1"),
		Status:        exchanges.OrderStatusNew,
	})
	defer flow.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	waitDone := make(chan *exchanges.Order, 1)
	go func() {
		got, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
			return o.Status == exchanges.OrderStatusFilled
		})
		require.NoError(t, err)
		waitDone <- got
	}()

	flow.publishOrder(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Quantity:      decimal.RequireFromString("1"),
		Status:        exchanges.OrderStatusFilled,
	})

	select {
	case <-waitDone:
		t.Fatal("filled wait should still block until fill data arrives")
	case <-time.After(50 * time.Millisecond):
	}

	flow.publishFill(&exchanges.Fill{
		TradeID:       "trade-1",
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Symbol:        "ETH",
		Side:          exchanges.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("1"),
		Timestamp:     124,
	})

	select {
	case got := <-waitDone:
		require.Equal(t, exchanges.OrderStatusFilled, got.Status)
		require.Equal(t, decimal.RequireFromString("1"), got.FilledQuantity)
		require.Equal(t, decimal.RequireFromString("101"), got.LastFillPrice)
	case <-time.After(time.Second):
		t.Fatal("expected filled snapshot after fill data arrived")
	}
}

func TestOrderFlowWaitReturnsFillDrivenSnapshot(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&exchanges.Order{
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Quantity:      decimal.RequireFromString("2"),
		Status:        exchanges.OrderStatusNew,
	})
	defer flow.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	waitDone := make(chan *exchanges.Order, 1)
	go func() {
		got, err := flow.Wait(ctx, func(o *exchanges.Order) bool {
			return o.LastFillQuantity.Equal(decimal.RequireFromString("0.4"))
		})
		require.NoError(t, err)
		waitDone <- got
	}()

	flow.publishFill(&exchanges.Fill{
		TradeID:       "trade-1",
		OrderID:       "exch-1",
		ClientOrderID: "cli-1",
		Price:         decimal.RequireFromString("99"),
		Quantity:      decimal.RequireFromString("0.4"),
		Timestamp:     125,
	})

	select {
	case got := <-waitDone:
		require.Equal(t, exchanges.OrderStatusPartiallyFilled, got.Status)
		require.Equal(t, decimal.RequireFromString("0.4"), got.LastFillQuantity)
	case <-time.After(time.Second):
		t.Fatal("expected fill-driven snapshot")
	}
}
