package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderEventType string

const (
	OrderEventSubmitted       OrderEventType = "submitted"
	OrderEventAccepted        OrderEventType = "accepted"
	OrderEventRejected        OrderEventType = "rejected"
	OrderEventPartiallyFilled OrderEventType = "partially_filled"
	OrderEventFilled          OrderEventType = "filled"
	OrderEventCanceled        OrderEventType = "canceled"
	OrderEventExpired         OrderEventType = "expired"
	OrderEventModified        OrderEventType = "modified"
)

type OrderEvent struct {
	EventID      string
	AccountID    AccountID
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	Type         OrderEventType
	Status       OrderStatus
	Side         OrderSide
	OrderType    OrderType
	Quantity     decimal.Decimal
	FilledQty    decimal.Decimal
	AvgPrice     decimal.Decimal
	Text         string
	EventTime    time.Time
}
