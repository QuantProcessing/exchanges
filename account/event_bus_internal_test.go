package account

import (
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/stretchr/testify/assert"
)

func TestEventBusSingleSubscriber(t *testing.T) {
	bus := newEventBus[exchanges.Order]()
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
