package account

import (
	"context"
	"sync"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderFlowWaitReturnsMatchingLatestSnapshot(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&exchanges.Order{
		ClientOrderID: "cli-1",
		Status:        exchanges.OrderStatusPending,
	})
	defer flow.Close()

	publicSeen := make(chan exchanges.OrderStatus, 2)
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
	var got *exchanges.Order
	var err error
	go func() {
		defer close(waitDone)
		got, err = flow.Wait(ctx, func(o *exchanges.Order) bool {
			return o.Status == exchanges.OrderStatusFilled
		})
	}()

	go func() {
		time.Sleep(10 * time.Millisecond)
		flow.publish(&exchanges.Order{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Status:        exchanges.OrderStatusNew,
		})
		flow.publish(&exchanges.Order{
			OrderID:        "exch-1",
			ClientOrderID:  "cli-1",
			Status:         exchanges.OrderStatusFilled,
			FilledQuantity: decimal.RequireFromString("0.25"),
		})
		flow.publish(&exchanges.Order{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Status:        exchanges.OrderStatusCancelled,
		})
	}()

	select {
	case <-waitDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected wait to finish")
	}
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, exchanges.OrderStatusFilled, got.Status)
	require.Equal(t, "exch-1", got.OrderID)
	require.Equal(t, decimal.RequireFromString("0.25"), got.FilledQuantity)

	seen := map[exchanges.OrderStatus]bool{}
	deadline := time.After(100 * time.Millisecond)
	for !seen[exchanges.OrderStatusNew] || !seen[exchanges.OrderStatusFilled] {
		select {
		case status := <-publicSeen:
			seen[status] = true
		case <-deadline:
			t.Fatal("expected public consumer to receive new and filled updates")
		}
	}
}

func TestOrderFlowCloseWakesMultipleWaiters(t *testing.T) {
	t.Parallel()

	flow := newOrderFlow(&exchanges.Order{ClientOrderID: "multi"})

	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	errs := make(chan error, 2)
	startWaiter := func() <-chan struct{} {
		ready := make(chan struct{})
		go func() {
			calls := 0
			var readyOnce sync.Once
			_, err := flow.Wait(waitCtx, func(o *exchanges.Order) bool {
				calls++
				if calls == 2 {
					readyOnce.Do(func() {
						close(ready)
					})
				}
				return o.Status == exchanges.OrderStatusFilled
			})
			errs <- err
		}()
		return ready
	}

	ready1 := startWaiter()
	ready2 := startWaiter()

	for _, ready := range []<-chan struct{}{ready1, ready2} {
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
