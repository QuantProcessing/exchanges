package exchanges

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrderFlowRegistryUnregistersClosedFlow(t *testing.T) {
	t.Parallel()

	registry := newOrderFlowRegistry()
	flow := registry.Register(&Order{ClientOrderID: "close-cli"})

	require.False(t, registryEmpty(registry))

	flow.Close()

	require.Eventually(t, func() bool {
		return registryEmpty(registry)
	}, time.Second, 10*time.Millisecond)
}

func TestOrderFlowRegistryUnregistersTrackedFlowAtTerminalStatus(t *testing.T) {
	t.Parallel()

	registry := newOrderFlowRegistry()
	flow := registry.Register(&Order{ClientOrderID: "track-cli"})
	defer flow.Close()

	registry.Route(&Order{
		ClientOrderID: "track-cli",
		OrderID:       "track-order",
		Status:        OrderStatusFilled,
	})

	require.Eventually(t, func() bool {
		return registryEmpty(registry)
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, OrderStatusFilled, flow.Latest().Status)
}

func TestTradingAccountPlacedFlowUnregistersAtTerminalStatus(t *testing.T) {
	t.Parallel()

	acct := &TradingAccount{flows: newOrderFlowRegistry()}
	bus := NewEventBus[Order]()
	sub := bus.Subscribe()
	flow := acct.newPlacedFlow(sub, &Order{ClientOrderID: "place-cli"})
	defer flow.Close()

	bus.Publish(&Order{
		ClientOrderID: "place-cli",
		OrderID:       "place-order",
		Status:        OrderStatusFilled,
	})

	require.Eventually(t, func() bool {
		return registryEmpty(acct.flows)
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, OrderStatusFilled, flow.Latest().Status)
}

func registryEmpty(registry *orderFlowRegistry) bool {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	return len(registry.all) == 0 &&
		len(registry.byOrderID) == 0 &&
		len(registry.byClientID) == 0
}
