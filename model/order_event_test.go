package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrderEventKindMapsNautilusLifecycleToStatus(t *testing.T) {
	cases := []struct {
		kind   OrderEventKind
		status OrderStatus
	}{
		{OrderEventInitialized, OrderStatusInitialized},
		{OrderEventDenied, OrderStatusDenied},
		{OrderEventEmulated, OrderStatusEmulated},
		{OrderEventReleased, OrderStatusReleased},
		{OrderEventSubmitted, OrderStatusSubmitted},
		{OrderEventAccepted, OrderStatusAccepted},
		{OrderEventRejected, OrderStatusRejected},
		{OrderEventTriggered, OrderStatusTriggered},
		{OrderEventPendingUpdate, OrderStatusPendingUpdate},
		{OrderEventPendingCancel, OrderStatusPendingCancel},
		{OrderEventUpdated, OrderStatusAccepted},
		{OrderEventModifyRejected, OrderStatusAccepted},
		{OrderEventCancelRejected, OrderStatusAccepted},
		{OrderEventCanceled, OrderStatusCanceled},
		{OrderEventExpired, OrderStatusExpired},
		{OrderEventPartiallyFilled, OrderStatusPartiallyFilled},
		{OrderEventFilled, OrderStatusFilled},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			status, ok := tc.kind.TargetStatus()
			require.True(t, ok)
			require.Equal(t, tc.status, status)
		})
	}
}

func TestOrderLifecycleEventValidateRequiresNautilusEventShape(t *testing.T) {
	event := OrderLifecycleEvent{
		AccountID:    "acct",
		InstrumentID: MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		Kind:         OrderEventAccepted,
		Status:       OrderStatusAccepted,
	}
	require.NoError(t, event.Validate())

	event.Kind = OrderEventKind("unknown")
	require.ErrorIs(t, event.Validate(), ErrInvalidOrder)

	event.Kind = OrderEventAccepted
	event.Status = OrderStatusFilled
	require.ErrorIs(t, event.Validate(), ErrInvalidOrder)
}

func TestOrderLifecycleEventAllowsRiskDeniedWithoutVenueOrderID(t *testing.T) {
	event := OrderLifecycleEvent{
		AccountID:     "acct",
		InstrumentID:  MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-denied",
		Kind:          OrderEventDenied,
		Status:        OrderStatusDenied,
		Reason:        "max order notional exceeded",
	}

	require.NoError(t, event.Validate())
}
