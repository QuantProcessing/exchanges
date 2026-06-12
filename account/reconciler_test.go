package account

import (
	"errors"
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestReconcilerAppliesOrderEventThroughStateMachine(t *testing.T) {
	c := NewCache()
	r := NewReconciler(c)

	accepted := testOrderEvent(model.OrderEventAccepted, model.OrderStatusAccepted)
	require.NoError(t, r.ApplyEvent(model.ExecutionEvent{OrderEvent: &accepted}))

	flow, ok := r.FlowByClientID(accepted.ClientID)
	require.True(t, ok)
	latest, ok := flow.Latest()
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, latest.Status)

	cached, ok := c.OrderByClientID(accepted.AccountID, accepted.ClientID)
	require.True(t, ok)
	require.Equal(t, model.OrderStatusAccepted, cached.Status)

	require.Equal(t, accepted, <-flow.Events())

	filled := testOrderEvent(model.OrderEventFilled, model.OrderStatusFilled)
	filled.FilledQty = accepted.Quantity
	require.NoError(t, r.ApplyEvent(model.ExecutionEvent{OrderEvent: &filled}))

	latest, ok = flow.Latest()
	require.True(t, ok)
	require.Equal(t, model.OrderStatusFilled, latest.Status)
}

func TestReconcilerRejectsInvalidOrderEventTransition(t *testing.T) {
	r := NewReconciler(NewCache())

	filled := testOrderEvent(model.OrderEventFilled, model.OrderStatusFilled)
	require.NoError(t, r.ApplyEvent(model.ExecutionEvent{OrderEvent: &filled}))

	accepted := testOrderEvent(model.OrderEventAccepted, model.OrderStatusAccepted)
	err := r.ApplyEvent(model.ExecutionEvent{OrderEvent: &accepted})
	require.True(t, errors.Is(err, model.ErrInvalidAccountState))
}
