package model

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type PositionEventKind string

const (
	PositionEventOpened  PositionEventKind = "position_opened"
	PositionEventChanged PositionEventKind = "position_changed"
	PositionEventClosed  PositionEventKind = "position_closed"
)

type PositionLifecycleEvent struct {
	AccountID        AccountID
	InstrumentID     InstrumentID
	PositionID       PositionID
	Kind             PositionEventKind
	PreviousSide     PositionSide
	PreviousQuantity decimal.Decimal
	Side             PositionSide
	Quantity         decimal.Decimal
	Report           *PositionStatusReport
}

func (e PositionLifecycleEvent) Validate() error {
	if e.AccountID == "" || e.PositionID == "" || e.Kind == "" {
		return fmt.Errorf("%w: invalid position lifecycle event", ErrInvalidOrder)
	}
	if err := e.InstrumentID.Validate(); err != nil {
		return err
	}
	if e.Quantity.IsNegative() || e.PreviousQuantity.IsNegative() {
		return fmt.Errorf("%w: invalid position quantities", ErrInvalidOrder)
	}
	switch e.Kind {
	case PositionEventOpened:
		if !isFlatPosition(e.PreviousSide, e.PreviousQuantity) || isFlatPosition(e.Side, e.Quantity) {
			return fmt.Errorf("%w: invalid position opened transition", ErrInvalidOrder)
		}
	case PositionEventChanged:
		if isFlatPosition(e.PreviousSide, e.PreviousQuantity) || isFlatPosition(e.Side, e.Quantity) {
			return fmt.Errorf("%w: invalid position changed transition", ErrInvalidOrder)
		}
	case PositionEventClosed:
		if isFlatPosition(e.PreviousSide, e.PreviousQuantity) || !isFlatPosition(e.Side, e.Quantity) {
			return fmt.Errorf("%w: invalid position closed transition", ErrInvalidOrder)
		}
	default:
		return fmt.Errorf("%w: unknown position event kind %s", ErrInvalidOrder, e.Kind)
	}
	if e.Report != nil {
		if err := e.Report.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func NewPositionLifecycleEvent(previous *PositionStatusReport, next PositionStatusReport) (PositionLifecycleEvent, bool) {
	previousSide := PositionSideFlat
	previousQuantity := decimal.Zero
	if previous != nil {
		previousSide = previous.Side
		previousQuantity = previous.Quantity
	}
	previousFlat := isFlatPosition(previousSide, previousQuantity)
	nextFlat := isFlatPosition(next.Side, next.Quantity)
	if previousFlat && nextFlat {
		return PositionLifecycleEvent{}, false
	}
	kind := PositionEventChanged
	if previousFlat && !nextFlat {
		kind = PositionEventOpened
	} else if !previousFlat && nextFlat {
		kind = PositionEventClosed
	} else if previousSide == next.Side && previousQuantity.Equal(next.Quantity) {
		return PositionLifecycleEvent{}, false
	}
	event := PositionLifecycleEvent{
		AccountID:        next.AccountID,
		InstrumentID:     next.InstrumentID,
		PositionID:       next.PositionID,
		Kind:             kind,
		PreviousSide:     previousSide,
		PreviousQuantity: previousQuantity,
		Side:             next.Side,
		Quantity:         next.Quantity,
		Report:           &next,
	}
	return event, true
}

func isFlatPosition(side PositionSide, quantity decimal.Decimal) bool {
	return side == "" || side == PositionSideFlat || quantity.IsZero()
}
