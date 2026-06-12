package account

import (
	"fmt"

	"github.com/QuantProcessing/exchanges/model"
)

type OrderStateMachine struct{}

func (OrderStateMachine) ApplyEvent(current *model.OrderStatusReport, event model.OrderEvent) (model.OrderStatusReport, bool, error) {
	if event.AccountID == "" {
		return model.OrderStatusReport{}, false, fmt.Errorf("%w: missing order event account id", model.ErrInvalidAccountState)
	}
	if err := event.InstrumentID.Validate(); err != nil {
		return model.OrderStatusReport{}, false, err
	}

	next := reportFromOrderEvent(event)
	if current == nil {
		return next, true, nil
	}
	if isDuplicateTerminalEvent(*current, next) {
		return *current, false, nil
	}
	if !canTransition(current.Status, next.Status) {
		return model.OrderStatusReport{}, false, fmt.Errorf(
			"%w: invalid order transition %s -> %s",
			model.ErrInvalidAccountState,
			current.Status,
			next.Status,
		)
	}
	merged := mergeOrderReport(*current, next)
	return merged, !sameOrderReport(*current, merged), nil
}

func reportFromOrderEvent(event model.OrderEvent) model.OrderStatusReport {
	status := event.Status
	if status == "" {
		status = statusFromOrderEventType(event.Type)
	}
	return model.OrderStatusReport{
		AccountID:    event.AccountID,
		InstrumentID: event.InstrumentID,
		OrderID:      event.OrderID,
		ClientID:     event.ClientID,
		Status:       status,
		Side:         event.Side,
		Type:         event.OrderType,
		Quantity:     event.Quantity,
		FilledQty:    event.FilledQty,
		AvgPrice:     event.AvgPrice,
		EventTime:    event.EventTime,
	}
}

func statusFromOrderEventType(eventType model.OrderEventType) model.OrderStatus {
	switch eventType {
	case model.OrderEventSubmitted:
		return model.OrderStatusSubmitted
	case model.OrderEventAccepted, model.OrderEventModified:
		return model.OrderStatusAccepted
	case model.OrderEventRejected:
		return model.OrderStatusRejected
	case model.OrderEventPartiallyFilled:
		return model.OrderStatusPartiallyFilled
	case model.OrderEventFilled:
		return model.OrderStatusFilled
	case model.OrderEventCanceled:
		return model.OrderStatusCanceled
	case model.OrderEventExpired:
		return model.OrderStatusExpired
	default:
		return ""
	}
}

func canTransition(from, to model.OrderStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case "":
		return true
	case model.OrderStatusSubmitted:
		return to == model.OrderStatusAccepted ||
			to == model.OrderStatusRejected ||
			to == model.OrderStatusCanceled ||
			to == model.OrderStatusExpired ||
			to == model.OrderStatusPartiallyFilled ||
			to == model.OrderStatusFilled
	case model.OrderStatusAccepted:
		return to == model.OrderStatusPartiallyFilled ||
			to == model.OrderStatusFilled ||
			to == model.OrderStatusCanceled ||
			to == model.OrderStatusExpired ||
			to == model.OrderStatusRejected
	case model.OrderStatusPartiallyFilled:
		return to == model.OrderStatusFilled ||
			to == model.OrderStatusCanceled ||
			to == model.OrderStatusExpired
	default:
		return false
	}
}

func isTerminalOrderStatus(status model.OrderStatus) bool {
	switch status {
	case model.OrderStatusFilled, model.OrderStatusCanceled, model.OrderStatusRejected, model.OrderStatusExpired:
		return true
	default:
		return false
	}
}

func isDuplicateTerminalEvent(current, next model.OrderStatusReport) bool {
	return isTerminalOrderStatus(current.Status) &&
		current.Status == next.Status &&
		current.AccountID == next.AccountID &&
		current.OrderID == next.OrderID &&
		current.ClientID == next.ClientID &&
		current.FilledQty.Equal(next.FilledQty) &&
		current.EventTime.Equal(next.EventTime)
}

func mergeOrderReport(current, next model.OrderStatusReport) model.OrderStatusReport {
	if next.OrderID == "" {
		next.OrderID = current.OrderID
	}
	if next.ClientID == "" {
		next.ClientID = current.ClientID
	}
	if next.Side == "" {
		next.Side = current.Side
	}
	if next.Type == "" {
		next.Type = current.Type
	}
	if next.Quantity.IsZero() {
		next.Quantity = current.Quantity
	}
	if next.AvgPrice.IsZero() {
		next.AvgPrice = current.AvgPrice
	}
	if next.EventTime.IsZero() {
		next.EventTime = current.EventTime
	}
	return next
}

func sameOrderReport(a, b model.OrderStatusReport) bool {
	return a.AccountID == b.AccountID &&
		a.InstrumentID == b.InstrumentID &&
		a.OrderID == b.OrderID &&
		a.ClientID == b.ClientID &&
		a.Status == b.Status &&
		a.FilledQty.Equal(b.FilledQty) &&
		a.AvgPrice.Equal(b.AvgPrice) &&
		a.EventTime.Equal(b.EventTime)
}
