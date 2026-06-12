package account

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrderStateMachineAppliesLegalTransitions(t *testing.T) {
	sm := OrderStateMachine{}
	var current *model.OrderStatusReport

	submitted, changed, err := sm.ApplyEvent(current, testOrderEvent(model.OrderEventSubmitted, model.OrderStatusSubmitted))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.OrderStatusSubmitted, submitted.Status)

	accepted, changed, err := sm.ApplyEvent(&submitted, testOrderEvent(model.OrderEventAccepted, model.OrderStatusAccepted))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.OrderStatusAccepted, accepted.Status)

	partialEvent := testOrderEvent(model.OrderEventPartiallyFilled, model.OrderStatusPartiallyFilled)
	partialEvent.FilledQty = decimal.RequireFromString("0.4")
	partiallyFilled, changed, err := sm.ApplyEvent(&accepted, partialEvent)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.OrderStatusPartiallyFilled, partiallyFilled.Status)
	require.True(t, partiallyFilled.FilledQty.Equal(decimal.RequireFromString("0.4")))

	filledEvent := testOrderEvent(model.OrderEventFilled, model.OrderStatusFilled)
	filledEvent.FilledQty = decimal.RequireFromString("1")
	filled, changed, err := sm.ApplyEvent(&partiallyFilled, filledEvent)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.OrderStatusFilled, filled.Status)
}

func TestOrderStateMachineAllowsSubmittedToRejected(t *testing.T) {
	sm := OrderStateMachine{}
	submitted, _, err := sm.ApplyEvent(nil, testOrderEvent(model.OrderEventSubmitted, model.OrderStatusSubmitted))
	require.NoError(t, err)

	rejected, changed, err := sm.ApplyEvent(&submitted, testOrderEvent(model.OrderEventRejected, model.OrderStatusRejected))
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, model.OrderStatusRejected, rejected.Status)
}

func TestOrderStateMachineRejectsBackwardTransitions(t *testing.T) {
	sm := OrderStateMachine{}
	filled, _, err := sm.ApplyEvent(nil, testOrderEvent(model.OrderEventFilled, model.OrderStatusFilled))
	require.NoError(t, err)

	_, _, err = sm.ApplyEvent(&filled, testOrderEvent(model.OrderEventAccepted, model.OrderStatusAccepted))
	require.ErrorIs(t, err, model.ErrInvalidAccountState)
}

func TestOrderStateMachineIgnoresDuplicateTerminalEvent(t *testing.T) {
	sm := OrderStateMachine{}
	event := testOrderEvent(model.OrderEventCanceled, model.OrderStatusCanceled)
	canceled, changed, err := sm.ApplyEvent(nil, event)
	require.NoError(t, err)
	require.True(t, changed)

	again, changed, err := sm.ApplyEvent(&canceled, event)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, canceled, again)
}

func TestOrderStateMachineRejectsTerminalStatusChange(t *testing.T) {
	sm := OrderStateMachine{}
	canceled, _, err := sm.ApplyEvent(nil, testOrderEvent(model.OrderEventCanceled, model.OrderStatusCanceled))
	require.NoError(t, err)

	_, _, err = sm.ApplyEvent(&canceled, testOrderEvent(model.OrderEventFilled, model.OrderStatusFilled))
	require.True(t, errors.Is(err, model.ErrInvalidAccountState))
}

func testOrderEvent(eventType model.OrderEventType, status model.OrderStatus) model.OrderEvent {
	return model.OrderEvent{
		EventID:      "event-1",
		AccountID:    "acct-1",
		InstrumentID: model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		OrderID:      "venue-1",
		ClientID:     "client-1",
		Type:         eventType,
		Status:       status,
		Side:         model.OrderSideBuy,
		OrderType:    model.OrderTypeLimit,
		Quantity:     decimal.NewFromInt(1),
		EventTime:    time.Unix(100, 0),
	}
}
