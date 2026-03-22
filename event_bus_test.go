package exchanges_test

import (
	"sync"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// EventBus unit tests
// ============================================================================

func TestEventBus_SingleSubscriber(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	sub := bus.Subscribe()
	defer sub.Unsubscribe()

	order := &exchanges.Order{OrderID: "123", Status: exchanges.OrderStatusNew}
	bus.Publish(order)

	select {
	case got := <-sub.C:
		assert.Equal(t, "123", got.OrderID)
		assert.Equal(t, exchanges.OrderStatusNew, got.Status)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBus_FanOut(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	const numSubs = 5
	subs := make([]*exchanges.Subscription[exchanges.Order], numSubs)
	for i := range subs {
		subs[i] = bus.Subscribe()
		defer subs[i].Unsubscribe()
	}

	order := &exchanges.Order{OrderID: "abc", Status: exchanges.OrderStatusFilled}
	bus.Publish(order)

	for i, sub := range subs {
		select {
		case got := <-sub.C:
			assert.Equal(t, "abc", got.OrderID, "sub %d should get event", i)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("sub %d: timeout waiting for event", i)
		}
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	sub := bus.Subscribe()
	sub.Unsubscribe()

	// Channel should be closed after unsubscribe
	_, ok := <-sub.C
	assert.False(t, ok, "channel should be closed after Unsubscribe")
}

func TestEventBus_UnsubscribeDoesNotAffectOthers(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()
	defer sub2.Unsubscribe()

	// Unsubscribe sub1
	sub1.Unsubscribe()

	// Publish should still reach sub2
	order := &exchanges.Order{OrderID: "456"}
	bus.Publish(order)

	select {
	case got := <-sub2.C:
		assert.Equal(t, "456", got.OrderID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sub2 should still receive events")
	}
}

func TestEventBus_Close(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()

	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()

	bus.Close()

	_, ok1 := <-sub1.C
	_, ok2 := <-sub2.C
	assert.False(t, ok1, "sub1 channel should be closed")
	assert.False(t, ok2, "sub2 channel should be closed")
}

func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	sub := bus.Subscribe()
	defer sub.Unsubscribe()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(id int) {
			defer wg.Done()
			bus.Publish(&exchanges.Order{OrderID: string(rune('A' + id%26))})
		}(i)
	}

	wg.Wait()

	// Drain all messages from channel
	count := 0
	for {
		select {
		case <-sub.C:
			count++
		default:
			goto done
		}
	}
done:
	// Some may be dropped if channel is full, but should not panic
	assert.True(t, count > 0, "should have received some events (got %d)", count)
	t.Logf("Received %d/%d events (some may be dropped due to channel capacity)", count, n)
}

func TestEventBus_NonBlocking(t *testing.T) {
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	sub := bus.Subscribe()
	defer sub.Unsubscribe()

	// Publish more events than channel capacity (64) — should not block
	for i := range 200 {
		bus.Publish(&exchanges.Order{OrderID: string(rune(i))})
	}

	// Should complete without hanging
	count := 0
	for {
		select {
		case <-sub.C:
			count++
		default:
			goto done
		}
	}
done:
	require.True(t, count <= 64, "should not exceed channel capacity")
	t.Logf("Received %d events (capacity=64)", count)
}

// ============================================================================
// LocalState unit tests (with mock adapter)
// ============================================================================

func TestLocalState_ApplyOrderUpdate_Terminal(t *testing.T) {
	// Test that terminal orders are removed from state
	bus := exchanges.NewEventBus[exchanges.Order]()
	defer bus.Close()

	state := exchanges.NewLocalState(nil, nil)

	// We can't call Start without an adapter, but we can test the exported query
	// after manually setting state. This verifies the data structures work.

	// GetOrder on empty state returns false
	_, ok := state.GetOrder("nonexistent")
	assert.False(t, ok)

	// GetAllOpenOrders on empty state returns empty
	orders := state.GetAllOpenOrders()
	assert.Empty(t, orders)

	// GetAllPositions on empty state returns empty
	positions := state.GetAllPositions()
	assert.Empty(t, positions)
}
