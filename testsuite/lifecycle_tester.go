package testsuite

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantProcessing/exchanges/account"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/shopspring/decimal"
)

type LifecycleTesterConfig struct{}

type LifecycleTester struct {
	cfg LifecycleTesterConfig
}

func NewLifecycleTester(cfg LifecycleTesterConfig) *LifecycleTester {
	return &LifecycleTester{cfg: cfg}
}

func (l *LifecycleTester) Run(_ context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "lifecycle", []contractCase{
		{id: "TC-L01", name: "Order event vocabulary", run: func() error {
			for _, event := range lifecycleEvents() {
				if _, ok := event.kind.TargetStatus(); !ok {
					return fmt.Errorf("event %s has no target status", event.kind)
				}
				if event.status == "" {
					return fmt.Errorf("event %s targets empty status", event.kind)
				}
			}
			return nil
		}},
		{id: "TC-L02", name: "Allowed order transitions", run: func() error {
			for _, pair := range allowedLifecycleTransitions() {
				if !account.CanOrderTransition(pair.from, pair.to) {
					return fmt.Errorf("expected transition %s -> %s", pair.from, pair.to)
				}
			}
			return nil
		}},
		{id: "TC-L03", name: "Rejected terminal and backward transitions", run: func() error {
			for _, pair := range rejectedLifecycleTransitions() {
				if account.CanOrderTransition(pair.from, pair.to) {
					return fmt.Errorf("unexpected transition %s -> %s", pair.from, pair.to)
				}
			}
			return nil
		}},
		{id: "TC-L04", name: "Position lifecycle classification", run: func() error {
			instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
			open := model.PositionStatusReport{
				AccountID:    "acct",
				InstrumentID: instID,
				PositionID:   "pos-1",
				Side:         model.PositionSideLong,
				Quantity:     decimal.RequireFromString("1"),
			}
			opened, ok := model.NewPositionLifecycleEvent(nil, open)
			if !ok || opened.Kind != model.PositionEventOpened {
				return fmt.Errorf("expected opened event, got %s", opened.Kind)
			}
			changedReport := open
			changedReport.Quantity = decimal.RequireFromString("1.5")
			changed, ok := model.NewPositionLifecycleEvent(&open, changedReport)
			if !ok || changed.Kind != model.PositionEventChanged {
				return fmt.Errorf("expected changed event, got %s", changed.Kind)
			}
			closedReport := changedReport
			closedReport.Side = model.PositionSideFlat
			closedReport.Quantity = decimal.Zero
			closed, ok := model.NewPositionLifecycleEvent(&changedReport, closedReport)
			if !ok || closed.Kind != model.PositionEventClosed {
				return fmt.Errorf("expected closed event, got %s", closed.Kind)
			}
			for _, event := range []model.PositionLifecycleEvent{opened, changed, closed} {
				if err := event.Validate(); err != nil {
					return err
				}
			}
			return nil
		}},
	})
}

type lifecycleEventCase struct {
	kind   model.OrderEventKind
	status model.OrderStatus
}

func lifecycleEvents() []lifecycleEventCase {
	return []lifecycleEventCase{
		{model.OrderEventInitialized, model.OrderStatusInitialized},
		{model.OrderEventDenied, model.OrderStatusDenied},
		{model.OrderEventEmulated, model.OrderStatusEmulated},
		{model.OrderEventReleased, model.OrderStatusReleased},
		{model.OrderEventSubmitted, model.OrderStatusSubmitted},
		{model.OrderEventAccepted, model.OrderStatusAccepted},
		{model.OrderEventRejected, model.OrderStatusRejected},
		{model.OrderEventTriggered, model.OrderStatusTriggered},
		{model.OrderEventPendingUpdate, model.OrderStatusPendingUpdate},
		{model.OrderEventPendingCancel, model.OrderStatusPendingCancel},
		{model.OrderEventUpdated, model.OrderStatusAccepted},
		{model.OrderEventModifyRejected, model.OrderStatusAccepted},
		{model.OrderEventCancelRejected, model.OrderStatusAccepted},
		{model.OrderEventCanceled, model.OrderStatusCanceled},
		{model.OrderEventExpired, model.OrderStatusExpired},
		{model.OrderEventPartiallyFilled, model.OrderStatusPartiallyFilled},
		{model.OrderEventFilled, model.OrderStatusFilled},
	}
}

type lifecycleTransitionCase struct {
	from model.OrderStatus
	to   model.OrderStatus
}

func allowedLifecycleTransitions() []lifecycleTransitionCase {
	return []lifecycleTransitionCase{
		{"", model.OrderStatusInitialized},
		{"", model.OrderStatusSubmitted},
		{"", model.OrderStatusAccepted},
		{model.OrderStatusInitialized, model.OrderStatusDenied},
		{model.OrderStatusInitialized, model.OrderStatusEmulated},
		{model.OrderStatusEmulated, model.OrderStatusReleased},
		{model.OrderStatusReleased, model.OrderStatusSubmitted},
		{model.OrderStatusInitialized, model.OrderStatusSubmitted},
		{model.OrderStatusSubmitted, model.OrderStatusAccepted},
		{model.OrderStatusSubmitted, model.OrderStatusRejected},
		{model.OrderStatusAccepted, model.OrderStatusTriggered},
		{model.OrderStatusAccepted, model.OrderStatusPendingUpdate},
		{model.OrderStatusPendingUpdate, model.OrderStatusAccepted},
		{model.OrderStatusAccepted, model.OrderStatusPendingCancel},
		{model.OrderStatusPendingCancel, model.OrderStatusAccepted},
		{model.OrderStatusPendingCancel, model.OrderStatusCanceled},
		{model.OrderStatusAccepted, model.OrderStatusCanceled},
		{model.OrderStatusAccepted, model.OrderStatusExpired},
		{model.OrderStatusAccepted, model.OrderStatusPartiallyFilled},
		{model.OrderStatusPartiallyFilled, model.OrderStatusFilled},
		{model.OrderStatusTriggered, model.OrderStatusFilled},
	}
}

func rejectedLifecycleTransitions() []lifecycleTransitionCase {
	return []lifecycleTransitionCase{
		{model.OrderStatusFilled, model.OrderStatusAccepted},
		{model.OrderStatusCanceled, model.OrderStatusAccepted},
		{model.OrderStatusRejected, model.OrderStatusAccepted},
		{model.OrderStatusExpired, model.OrderStatusAccepted},
		{model.OrderStatusAccepted, model.OrderStatusSubmitted},
		{model.OrderStatusPartiallyFilled, model.OrderStatusAccepted},
		{model.OrderStatusDenied, model.OrderStatusSubmitted},
	}
}
