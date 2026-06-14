package model

import "fmt"

type OrderEventKind string

const (
	OrderEventInitialized     OrderEventKind = "order_initialized"
	OrderEventDenied          OrderEventKind = "order_denied"
	OrderEventEmulated        OrderEventKind = "order_emulated"
	OrderEventReleased        OrderEventKind = "order_released"
	OrderEventSubmitted       OrderEventKind = "order_submitted"
	OrderEventAccepted        OrderEventKind = "order_accepted"
	OrderEventRejected        OrderEventKind = "order_rejected"
	OrderEventTriggered       OrderEventKind = "order_triggered"
	OrderEventPendingUpdate   OrderEventKind = "order_pending_update"
	OrderEventPendingCancel   OrderEventKind = "order_pending_cancel"
	OrderEventUpdated         OrderEventKind = "order_updated"
	OrderEventModifyRejected  OrderEventKind = "order_modify_rejected"
	OrderEventCancelRejected  OrderEventKind = "order_cancel_rejected"
	OrderEventCanceled        OrderEventKind = "order_canceled"
	OrderEventExpired         OrderEventKind = "order_expired"
	OrderEventPartiallyFilled OrderEventKind = "order_partially_filled"
	OrderEventFilled          OrderEventKind = "order_filled"
)

func (k OrderEventKind) TargetStatus() (OrderStatus, bool) {
	switch k {
	case OrderEventInitialized:
		return OrderStatusInitialized, true
	case OrderEventDenied:
		return OrderStatusDenied, true
	case OrderEventEmulated:
		return OrderStatusEmulated, true
	case OrderEventReleased:
		return OrderStatusReleased, true
	case OrderEventSubmitted:
		return OrderStatusSubmitted, true
	case OrderEventAccepted:
		return OrderStatusAccepted, true
	case OrderEventRejected:
		return OrderStatusRejected, true
	case OrderEventTriggered:
		return OrderStatusTriggered, true
	case OrderEventPendingUpdate:
		return OrderStatusPendingUpdate, true
	case OrderEventPendingCancel:
		return OrderStatusPendingCancel, true
	case OrderEventUpdated, OrderEventModifyRejected, OrderEventCancelRejected:
		return OrderStatusAccepted, true
	case OrderEventCanceled:
		return OrderStatusCanceled, true
	case OrderEventExpired:
		return OrderStatusExpired, true
	case OrderEventPartiallyFilled:
		return OrderStatusPartiallyFilled, true
	case OrderEventFilled:
		return OrderStatusFilled, true
	default:
		return "", false
	}
}

type OrderLifecycleEvent struct {
	Metadata       CommandMetadata
	AccountID      AccountID
	InstrumentID   InstrumentID
	OrderID        OrderID
	ClientOrderID  ClientOrderID
	VenueOrderID   VenueOrderID
	Kind           OrderEventKind
	PreviousStatus OrderStatus
	Status         OrderStatus
	Reason         string
	Report         *OrderStatusReport
}

func (e OrderLifecycleEvent) Validate() error {
	if e.AccountID == "" || (e.OrderID == "" && e.ClientOrderID == "") || e.Kind == "" || e.Status == "" {
		return fmt.Errorf("%w: invalid order lifecycle event", ErrInvalidOrder)
	}
	if err := e.InstrumentID.Validate(); err != nil {
		return err
	}
	target, ok := e.Kind.TargetStatus()
	if !ok {
		return fmt.Errorf("%w: unknown order event kind %s", ErrInvalidOrder, e.Kind)
	}
	if e.Status != target {
		return fmt.Errorf("%w: event %s targets %s, got %s", ErrInvalidOrder, e.Kind, target, e.Status)
	}
	if e.Report != nil {
		if err := e.Report.Validate(); err != nil {
			return err
		}
	}
	return nil
}
