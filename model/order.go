package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderSide string
type OrderType string
type OrderStatus string
type TimeInForce string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"

	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"

	OrderStatusSubmitted       OrderStatus = "submitted"
	OrderStatusAccepted        OrderStatus = "accepted"
	OrderStatusRejected        OrderStatus = "rejected"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled          OrderStatus = "filled"
	OrderStatusCanceled        OrderStatus = "canceled"
	OrderStatusExpired         OrderStatus = "expired"
)

type SubmitOrder struct {
	InstrumentID InstrumentID
	Side         OrderSide
	Type         OrderType
	Quantity     decimal.Decimal
	Price        decimal.Decimal
	ClientID     ClientOrderID
	ReduceOnly   bool
	TimeInForce  TimeInForce
}

type ModifyOrder struct {
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	Quantity     decimal.Decimal
	Price        decimal.Decimal
}

type CancelOrder struct {
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
}

type CancelAllOrders struct {
	InstrumentID InstrumentID
}

type OrderStatusReport struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	Status       OrderStatus
	Side         OrderSide
	Type         OrderType
	Quantity     decimal.Decimal
	FilledQty    decimal.Decimal
	AvgPrice     decimal.Decimal
	EventTime    time.Time
}

type FillReport struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	OrderID      OrderID
	ClientID     ClientOrderID
	TradeID      TradeID
	Side         OrderSide
	Quantity     decimal.Decimal
	Price        decimal.Decimal
	Fee          Money
	EventTime    time.Time
}

type PositionSide string

const (
	PositionSideLong  PositionSide = "long"
	PositionSideShort PositionSide = "short"
	PositionSideFlat  PositionSide = "flat"
)

type PositionStatusReport struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	PositionID   PositionID
	Side         PositionSide
	Quantity     decimal.Decimal
	AvgPrice     decimal.Decimal
	Unrealized   Money
	EventTime    time.Time
}

type ExecutionEvent struct {
	AccountState *AccountState
	OrderEvent   *OrderEvent
	Order        *OrderStatusReport
	Fill         *FillReport
	Position     *PositionStatusReport
}
