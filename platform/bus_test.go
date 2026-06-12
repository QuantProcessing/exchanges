package platform

import (
	"context"
	"testing"

	"github.com/QuantProcessing/exchanges/venue"
	"github.com/stretchr/testify/require"
)

func TestBusSendsToRegisteredEndpoint(t *testing.T) {
	bus := NewBus()
	defer func() { require.NoError(t, bus.Close()) }()

	var received Event
	sub := bus.Register("ExecEngine.process", func(_ context.Context, ev Event) error {
		received = ev
		return nil
	})
	defer func() { require.NoError(t, sub.Close()) }()

	require.NoError(t, bus.Send(context.Background(), "ExecEngine.process", "accepted"))
	require.Equal(t, "ExecEngine.process", received.Endpoint)
	require.Equal(t, "accepted", received.Message)
}

func TestBusSendUnknownEndpointReturnsError(t *testing.T) {
	bus := NewBus()
	defer func() { require.NoError(t, bus.Close()) }()

	err := bus.Send(context.Background(), "ExecEngine.missing", "message")
	require.ErrorIs(t, err, ErrEndpointNotFound)
}

func TestBusPublishesToMultipleTopicSubscribers(t *testing.T) {
	bus := NewBus()
	defer func() { require.NoError(t, bus.Close()) }()

	sub1, ch1 := bus.Subscribe("events.execution", 1)
	sub2, ch2 := bus.Subscribe("events.execution", 1)
	defer func() { require.NoError(t, sub1.Close()) }()
	defer func() { require.NoError(t, sub2.Close()) }()

	require.NoError(t, bus.Publish("events.execution", "fill"))
	require.Equal(t, "fill", (<-ch1).Message)
	require.Equal(t, "fill", (<-ch2).Message)
}

func TestBusClosingOneSubscriptionDoesNotCloseOthers(t *testing.T) {
	bus := NewBus()
	defer func() { require.NoError(t, bus.Close()) }()

	sub1, ch1 := bus.Subscribe("events.execution", 1)
	sub2, ch2 := bus.Subscribe("events.execution", 1)
	require.NoError(t, sub1.Close())

	require.NoError(t, bus.Publish("events.execution", "order"))
	_, ok := <-ch1
	require.False(t, ok)
	require.Equal(t, "order", (<-ch2).Message)
	require.NoError(t, sub2.Close())
}

func TestBusCloseClosesAllSubscriptions(t *testing.T) {
	bus := NewBus()
	sub, ch := bus.Subscribe("events.execution", 1)
	var _ venue.Subscription = sub

	require.NoError(t, bus.Close())
	_, ok := <-ch
	require.False(t, ok)
	require.NoError(t, bus.Publish("events.execution", "ignored"))
	require.Equal(t, uint64(1), bus.Health().ClosedPublishes)
}
