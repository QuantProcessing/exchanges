package kernel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComponentLifecycleTransitionsAndHealth(t *testing.T) {
	started := false
	stopped := false
	component := NewComponent("execution-engine", ComponentHooks{
		Start: func(context.Context) error {
			started = true
			return nil
		},
		Stop: func(context.Context) error {
			stopped = true
			return nil
		},
	})

	require.Equal(t, ComponentStateInitialized, component.State())
	require.NoError(t, component.Start(context.Background()))
	require.True(t, started)
	require.Equal(t, ComponentStateRunning, component.State())

	component.Degrade(errors.New("queue pressure"))
	require.Equal(t, ComponentStateDegraded, component.State())
	require.Equal(t, "queue pressure", component.Health().LastError)

	require.NoError(t, component.Stop(context.Background()))
	require.True(t, stopped)
	require.Equal(t, ComponentStateStopped, component.State())
	require.NoError(t, component.Start(context.Background()))
	require.Equal(t, ComponentStateRunning, component.State())

	component.Fault(errors.New("fatal"))
	require.Equal(t, ComponentStateFaulted, component.State())
	require.Equal(t, "fatal", component.Health().LastError)
	require.ErrorIs(t, component.Start(context.Background()), ErrComponentFaulted)
}

func TestTestClockAdvancesNanosecondTimersDeterministically(t *testing.T) {
	clock := NewTestClock(time.Unix(100, 0))
	timer := clock.After(10 * time.Nanosecond)

	clock.Advance(9 * time.Nanosecond)
	require.Empty(t, timer)

	clock.Advance(time.Nanosecond)
	require.Equal(t, time.Unix(100, 10), <-timer)
	require.Equal(t, time.Unix(100, 10), clock.Now())
}

func TestMsgBusPublishesWithBackpressureMetricsAndRequests(t *testing.T) {
	clock := NewTestClock(time.Unix(100, 0))
	msgbus := NewMsgBus(MsgBusConfig{Clock: clock, DefaultBuffer: 1})
	sub := msgbus.Subscribe("orders", 1)
	defer sub.Close()

	require.NoError(t, msgbus.Publish(context.Background(), "orders", "accepted"))
	require.ErrorIs(t, msgbus.Publish(context.Background(), "orders", "filled"), ErrBackpressure)
	stats := msgbus.Stats()
	require.Equal(t, int64(1), stats.Published)
	require.Equal(t, int64(1), stats.Dropped)

	env := <-sub.C()
	require.Equal(t, "orders", env.Topic)
	require.Equal(t, "accepted", env.Message)
	require.Equal(t, clock.Now(), env.Timestamp)

	msgbus.RegisterEndpoint("risk.check", func(_ context.Context, req Request) (Response, error) {
		return Response{CorrelationID: req.CorrelationID, Payload: "ok:" + req.Payload.(string)}, nil
	})
	resp, err := msgbus.Request(context.Background(), "risk.check", "order-1")
	require.NoError(t, err)
	require.Equal(t, "ok:order-1", resp.Payload)
	require.NotEmpty(t, resp.CorrelationID)
}
