package account

import (
	"testing"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/stretchr/testify/require"
)

func TestCanOrderTransitionFollowsNautilusLifecycle(t *testing.T) {
	allowed := [][2]model.OrderStatus{
		{"", model.OrderStatusInitialized},
		{"", model.OrderStatusSubmitted},
		{model.OrderStatusInitialized, model.OrderStatusDenied},
		{model.OrderStatusInitialized, model.OrderStatusEmulated},
		{model.OrderStatusInitialized, model.OrderStatusAccepted},
		{model.OrderStatusInitialized, model.OrderStatusRejected},
		{model.OrderStatusInitialized, model.OrderStatusCanceled},
		{model.OrderStatusInitialized, model.OrderStatusExpired},
		{model.OrderStatusInitialized, model.OrderStatusTriggered},
		{model.OrderStatusEmulated, model.OrderStatusCanceled},
		{model.OrderStatusEmulated, model.OrderStatusExpired},
		{model.OrderStatusEmulated, model.OrderStatusReleased},
		{model.OrderStatusReleased, model.OrderStatusDenied},
		{model.OrderStatusReleased, model.OrderStatusCanceled},
		{model.OrderStatusReleased, model.OrderStatusSubmitted},
		{model.OrderStatusInitialized, model.OrderStatusSubmitted},
		{model.OrderStatusSubmitted, model.OrderStatusPendingUpdate},
		{model.OrderStatusSubmitted, model.OrderStatusPendingCancel},
		{model.OrderStatusSubmitted, model.OrderStatusAccepted},
		{model.OrderStatusSubmitted, model.OrderStatusRejected},
		{model.OrderStatusSubmitted, model.OrderStatusCanceled},
		{model.OrderStatusSubmitted, model.OrderStatusPartiallyFilled},
		{model.OrderStatusSubmitted, model.OrderStatusFilled},
		{model.OrderStatusAccepted, model.OrderStatusRejected},
		{model.OrderStatusAccepted, model.OrderStatusTriggered},
		{model.OrderStatusAccepted, model.OrderStatusPendingUpdate},
		{model.OrderStatusPendingUpdate, model.OrderStatusAccepted},
		{model.OrderStatusPendingUpdate, model.OrderStatusRejected},
		{model.OrderStatusPendingUpdate, model.OrderStatusCanceled},
		{model.OrderStatusPendingUpdate, model.OrderStatusExpired},
		{model.OrderStatusPendingUpdate, model.OrderStatusTriggered},
		{model.OrderStatusPendingUpdate, model.OrderStatusPendingCancel},
		{model.OrderStatusPendingUpdate, model.OrderStatusPartiallyFilled},
		{model.OrderStatusPendingUpdate, model.OrderStatusFilled},
		{model.OrderStatusAccepted, model.OrderStatusPendingCancel},
		{model.OrderStatusPendingCancel, model.OrderStatusAccepted},
		{model.OrderStatusPendingCancel, model.OrderStatusRejected},
		{model.OrderStatusPendingCancel, model.OrderStatusCanceled},
		{model.OrderStatusPendingCancel, model.OrderStatusExpired},
		{model.OrderStatusPendingCancel, model.OrderStatusPartiallyFilled},
		{model.OrderStatusPendingCancel, model.OrderStatusFilled},
		{model.OrderStatusAccepted, model.OrderStatusCanceled},
		{model.OrderStatusAccepted, model.OrderStatusExpired},
		{model.OrderStatusAccepted, model.OrderStatusPartiallyFilled},
		{model.OrderStatusPartiallyFilled, model.OrderStatusFilled},
		{model.OrderStatusCanceled, model.OrderStatusPartiallyFilled},
		{model.OrderStatusCanceled, model.OrderStatusFilled},
		{model.OrderStatusTriggered, model.OrderStatusRejected},
		{model.OrderStatusTriggered, model.OrderStatusPendingUpdate},
		{model.OrderStatusTriggered, model.OrderStatusPendingCancel},
		{model.OrderStatusTriggered, model.OrderStatusCanceled},
		{model.OrderStatusTriggered, model.OrderStatusExpired},
		{model.OrderStatusTriggered, model.OrderStatusPartiallyFilled},
		{model.OrderStatusTriggered, model.OrderStatusFilled},
	}

	for _, pair := range allowed {
		require.Truef(t, CanOrderTransition(pair[0], pair[1]), "expected %s -> %s", pair[0], pair[1])
	}
}

func TestCanOrderTransitionRejectsTerminalAndBackwardTransitions(t *testing.T) {
	rejected := [][2]model.OrderStatus{
		{model.OrderStatusFilled, model.OrderStatusAccepted},
		{model.OrderStatusCanceled, model.OrderStatusAccepted},
		{model.OrderStatusRejected, model.OrderStatusAccepted},
		{model.OrderStatusExpired, model.OrderStatusAccepted},
		{model.OrderStatusAccepted, model.OrderStatusSubmitted},
		{model.OrderStatusPartiallyFilled, model.OrderStatusAccepted},
		{model.OrderStatusDenied, model.OrderStatusSubmitted},
	}

	for _, pair := range rejected {
		require.Falsef(t, CanOrderTransition(pair[0], pair[1]), "expected %s -> %s to be rejected", pair[0], pair[1])
	}
}

func TestNextOrderStatusKeepsPendingUpdateOnSubmittedEcho(t *testing.T) {
	next, ok := NextOrderStatus(model.OrderStatusPendingUpdate, model.OrderStatusSubmitted)
	require.True(t, ok)
	require.Equal(t, model.OrderStatusPendingUpdate, next)
}
