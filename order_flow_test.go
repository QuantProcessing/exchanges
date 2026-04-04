package exchanges

import (
	"context"
	"sync/atomic"
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

	publicSeen := make(chan OrderStatus, 2)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case order, ok := <-flow.C():
				if !ok {
					return
				}
				publicSeen <- order.Status
			case <-done:
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	waitDone := make(chan struct{})
	var got *Order
	var err error
	go func() {
		defer close(waitDone)
		got, err = flow.Wait(ctx, func(o *Order) bool {
			return o.Status == OrderStatusFilled
		})
	}()

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
		flow.publish(&Order{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Status:        OrderStatusCancelled,
		})
	}()

	select {
	case <-waitDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected wait to finish")
	}
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, OrderStatusFilled, got.Status)
	require.Equal(t, "exch-1", got.OrderID)
	require.Equal(t, decimal.RequireFromString("0.25"), got.FilledQuantity)

	seen := map[OrderStatus]bool{}
	deadline := time.After(100 * time.Millisecond)
	for !seen[OrderStatusNew] || !seen[OrderStatusFilled] {
		select {
		case status := <-publicSeen:
			seen[status] = true
		case <-deadline:
			t.Fatal("expected public consumer to receive new and filled updates")
		}
	}
}

func TestOrderFlowWaitReturnsCurrentLatestSnapshotImmediately(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&Order{
		OrderID:        "exch-immediate",
		ClientOrderID:  "cli-immediate",
		Status:         OrderStatusFilled,
		FilledQuantity: decimal.RequireFromString("1"),
	})
	defer flow.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	got, err := flow.Wait(ctx, func(o *Order) bool {
		return o.Status == OrderStatusFilled
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, OrderStatusFilled, got.Status)
	require.Equal(t, "exch-immediate", got.OrderID)

	got.Status = OrderStatusCancelled
	require.Equal(t, OrderStatusFilled, flow.Latest().Status)
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

func TestOrderFlowWaitDoesNotReplayHistoricalSnapshot(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&Order{
		ClientOrderID: "cli-history",
		Status:        OrderStatusPending,
	})
	defer flow.Close()

	flow.publish(&Order{
		OrderID:       "exch-history",
		ClientOrderID: "cli-history",
		Status:        OrderStatusFilled,
	})
	flow.publish(&Order{
		OrderID:       "exch-history",
		ClientOrderID: "cli-history",
		Status:        OrderStatusCancelled,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	got, err := flow.Wait(ctx, func(o *Order) bool {
		return o.Status == OrderStatusFilled
	})
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, got)
	require.Equal(t, OrderStatusCancelled, flow.Latest().Status)
}

func TestOrderFlowCloseWakesMultipleWaiters(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&Order{ClientOrderID: "multi"})

	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var predicateCalls atomic.Int32
	ready := make(chan struct{}, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := flow.Wait(waitCtx, func(o *Order) bool {
				if predicateCalls.Add(1)%2 == 0 {
					ready <- struct{}{}
				}
				return o.Status == OrderStatusFilled
			})
			errs <- err
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case <-time.After(time.Second):
			t.Fatal("expected both waiters to reach the waiting path before close")
		}
	}

	flow.Close()

	for i := 0; i < 2; i++ {
		err := <-errs
		require.EqualError(t, err, "order flow closed")
	}
}
