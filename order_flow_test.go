package exchanges

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderFlowWaitReturnsMatchingLatestSnapshot(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&Order{
		ClientOrderID: "cli-1",
		Status:        OrderStatusPending,
	})
	defer flow.Close()

	go func() {
		time.Sleep(10 * time.Millisecond)
		flow.publish(&Order{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Status:        OrderStatusNew,
		})
		flow.publish(&Order{
			OrderID:        "exch-1",
			ClientOrderID:  "cli-1",
			Status:         OrderStatusFilled,
			FilledQuantity: decimal.RequireFromString("0.25"),
		})
	}()

	got, err := flow.Wait(context.Background(), func(o *Order) bool {
		return o.Status == OrderStatusFilled
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, OrderStatusFilled, got.Status)
	require.Equal(t, "exch-1", flow.Latest().OrderID)
	require.Equal(t, decimal.RequireFromString("0.25"), flow.Latest().FilledQuantity)
}

func TestOrderFlowCloseClosesThePublicChannel(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&Order{ClientOrderID: "cli-close"})
	ch := flow.C()
	flow.Close()

	select {
	case _, ok := <-ch:
		require.False(t, ok)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected closed flow channel")
	}
}
