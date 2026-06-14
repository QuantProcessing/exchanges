package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

func (s OrderSide) Opposite() OrderSide {
	if s == OrderSideBuy {
		return OrderSideSell
	}
	if s == OrderSideSell {
		return OrderSideBuy
	}
	return ""
}

type ContingencyType string

const (
	ContingencyTypeOTO ContingencyType = "oto"
	ContingencyTypeOCO ContingencyType = "oco"
	ContingencyTypeOUO ContingencyType = "ouo"
)

func (c ContingencyType) Validate() error {
	if c == "" {
		return nil
	}
	switch c {
	case ContingencyTypeOTO, ContingencyTypeOCO, ContingencyTypeOUO:
		return nil
	default:
		return fmt.Errorf("%w: invalid contingency type %q", ErrInvalidOrder, c)
	}
}

type OrderType string

const (
	OrderTypeMarket             OrderType = "market"
	OrderTypeLimit              OrderType = "limit"
	OrderTypeMarketToLimit      OrderType = "market_to_limit"
	OrderTypeStopMarket         OrderType = "stop_market"
	OrderTypeStopLimit          OrderType = "stop_limit"
	OrderTypeMarketIfTouched    OrderType = "market_if_touched"
	OrderTypeLimitIfTouched     OrderType = "limit_if_touched"
	OrderTypeTrailingStopMarket OrderType = "trailing_stop_market"
	OrderTypeTrailingStopLimit  OrderType = "trailing_stop_limit"
)

type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "gtc"
	TimeInForceIOC TimeInForce = "ioc"
	TimeInForceFOK TimeInForce = "fok"
	TimeInForceGTD TimeInForce = "gtd"
	TimeInForceDAY TimeInForce = "day"
)

func (t TimeInForce) Validate() error {
	if t == "" {
		return nil
	}
	switch t {
	case TimeInForceGTC, TimeInForceIOC, TimeInForceFOK, TimeInForceGTD, TimeInForceDAY:
		return nil
	default:
		return fmt.Errorf("%w: invalid time in force %q", ErrInvalidOrder, t)
	}
}

type OrderStatus string

const (
	OrderStatusInitialized     OrderStatus = "initialized"
	OrderStatusDenied          OrderStatus = "denied"
	OrderStatusEmulated        OrderStatus = "emulated"
	OrderStatusReleased        OrderStatus = "released"
	OrderStatusSubmitted       OrderStatus = "submitted"
	OrderStatusAccepted        OrderStatus = "accepted"
	OrderStatusTriggered       OrderStatus = "triggered"
	OrderStatusPendingCancel   OrderStatus = "pending_cancel"
	OrderStatusPendingUpdate   OrderStatus = "pending_update"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled          OrderStatus = "filled"
	OrderStatusCanceled        OrderStatus = "canceled"
	OrderStatusRejected        OrderStatus = "rejected"
	OrderStatusExpired         OrderStatus = "expired"
)

func (s OrderStatus) IsTerminal() bool {
	switch s {
	case OrderStatusDenied, OrderStatusFilled, OrderStatusCanceled, OrderStatusRejected, OrderStatusExpired:
		return true
	default:
		return false
	}
}

func (s OrderStatus) IsOpen() bool {
	return s != "" && !s.IsTerminal()
}

type SubmitOrder struct {
	AccountID           AccountID
	InstrumentID        InstrumentID
	OrderListID         OrderListID
	ParentClientOrderID ClientOrderID
	ClientOrderID       ClientOrderID
	Side                OrderSide
	Type                OrderType
	Contingency         ContingencyType
	TimeInForce         TimeInForce
	Quantity            decimal.Decimal
	Price               decimal.Decimal
	TriggerPrice        decimal.Decimal
	ActivationPrice     decimal.Decimal
	TrailingOffset      decimal.Decimal
	PostOnly            bool
	ReduceOnly          bool
	ExpireTime          time.Time
}

func (o SubmitOrder) Validate() error {
	if o.AccountID == "" || o.Side == "" || o.Type == "" || !o.Quantity.IsPositive() {
		return fmt.Errorf("%w: invalid submit order", ErrInvalidOrder)
	}
	if err := o.TimeInForce.Validate(); err != nil {
		return err
	}
	if err := o.Contingency.Validate(); err != nil {
		return err
	}
	if err := o.InstrumentID.Validate(); err != nil {
		return err
	}
	if o.TimeInForce == TimeInForceGTD {
		if !o.ExpireTime.After(time.Unix(0, 0)) {
			return fmt.Errorf("%w: GTD order requires expire time after UNIX epoch", ErrInvalidOrder)
		}
	} else if !o.ExpireTime.IsZero() {
		return fmt.Errorf("%w: expire time requires GTD time in force", ErrInvalidOrder)
	}
	if o.Type == OrderTypeMarket && o.TimeInForce == TimeInForceGTD {
		return fmt.Errorf("%w: market order does not support GTD time in force", ErrInvalidOrder)
	}
	if (o.Type == OrderTypeLimit || o.Type == OrderTypeStopLimit || o.Type == OrderTypeLimitIfTouched || o.Type == OrderTypeTrailingStopLimit) && !o.Price.IsPositive() {
		return fmt.Errorf("%w: limit order requires price", ErrInvalidOrder)
	}
	if (o.Type == OrderTypeStopMarket || o.Type == OrderTypeStopLimit || o.Type == OrderTypeMarketIfTouched || o.Type == OrderTypeLimitIfTouched) && !o.TriggerPrice.IsPositive() {
		return fmt.Errorf("%w: trigger order requires trigger price", ErrInvalidOrder)
	}
	if (o.Type == OrderTypeTrailingStopMarket || o.Type == OrderTypeTrailingStopLimit) && !o.TrailingOffset.IsPositive() {
		return fmt.Errorf("%w: trailing order requires trailing offset", ErrInvalidOrder)
	}
	if o.PostOnly && (o.Type == OrderTypeMarket || o.Type == OrderTypeMarketToLimit || o.Type == OrderTypeStopMarket || o.Type == OrderTypeMarketIfTouched || o.Type == OrderTypeTrailingStopMarket) {
		return fmt.Errorf("%w: post-only requires limit-style order", ErrInvalidOrder)
	}
	return nil
}

type CancelOrder struct {
	AccountID     AccountID
	InstrumentID  InstrumentID
	OrderID       OrderID
	ClientOrderID ClientOrderID
}

func (o CancelOrder) Validate() error {
	if o.AccountID == "" || (o.OrderID == "" && o.ClientOrderID == "") {
		return fmt.Errorf("%w: invalid cancel order", ErrInvalidOrder)
	}
	return o.InstrumentID.Validate()
}

type BatchCancelOrders struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	Cancels      []CancelOrder
}

func (b BatchCancelOrders) Validate() error {
	if b.AccountID == "" || len(b.Cancels) == 0 {
		return fmt.Errorf("%w: invalid batch cancel orders", ErrInvalidOrder)
	}
	if err := b.InstrumentID.Validate(); err != nil {
		return err
	}
	for _, cancel := range b.Cancels {
		if cancel.AccountID != "" && cancel.AccountID != b.AccountID {
			return fmt.Errorf("%w: batch cancel account mismatch", ErrInvalidOrder)
		}
		if cancel.InstrumentID != (InstrumentID{}) && cancel.InstrumentID != b.InstrumentID {
			return fmt.Errorf("%w: batch cancel instrument mismatch", ErrInvalidOrder)
		}
		if cancel.OrderID == "" && cancel.ClientOrderID == "" {
			return fmt.Errorf("%w: batch cancel requires order identity", ErrInvalidOrder)
		}
	}
	return nil
}

type CancelAllOrders struct {
	AccountID    AccountID
	InstrumentID InstrumentID
	OrderSide    OrderSide
}

func (c CancelAllOrders) Validate() error {
	if c.AccountID == "" {
		return fmt.Errorf("%w: invalid cancel all orders", ErrInvalidOrder)
	}
	if c.OrderSide != "" && c.OrderSide != OrderSideBuy && c.OrderSide != OrderSideSell {
		return fmt.Errorf("%w: invalid cancel all order side", ErrInvalidOrder)
	}
	return c.InstrumentID.Validate()
}

func (c CancelAllOrders) MatchesOrder(order OrderStatusReport) bool {
	if order.AccountID != c.AccountID || order.InstrumentID != c.InstrumentID {
		return false
	}
	return c.OrderSide == "" || order.Side == c.OrderSide
}

type QueryOrder struct {
	AccountID     AccountID
	InstrumentID  InstrumentID
	OrderID       OrderID
	VenueOrderID  VenueOrderID
	ClientOrderID ClientOrderID
}

func (q QueryOrder) Validate() error {
	if q.AccountID == "" || (q.OrderID == "" && q.ClientOrderID == "" && q.VenueOrderID == "") {
		return fmt.Errorf("%w: invalid query order", ErrInvalidOrder)
	}
	return q.InstrumentID.Validate()
}

type ModifyOrder struct {
	AccountID       AccountID
	InstrumentID    InstrumentID
	OrderID         OrderID
	VenueOrderID    VenueOrderID
	ClientOrderID   ClientOrderID
	Quantity        decimal.Decimal
	Price           decimal.Decimal
	TriggerPrice    decimal.Decimal
	ActivationPrice decimal.Decimal
	TrailingOffset  decimal.Decimal
	TimeInForce     TimeInForce
}

func (o ModifyOrder) Validate() error {
	if o.AccountID == "" || (o.OrderID == "" && o.ClientOrderID == "") {
		return fmt.Errorf("%w: invalid modify order", ErrInvalidOrder)
	}
	if err := o.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := o.TimeInForce.Validate(); err != nil {
		return err
	}
	hasChange := o.TimeInForce != ""
	for _, value := range []decimal.Decimal{o.Quantity, o.Price, o.TriggerPrice, o.ActivationPrice, o.TrailingOffset} {
		if value.IsNegative() {
			return fmt.Errorf("%w: invalid modify value", ErrInvalidOrder)
		}
		if value.IsPositive() {
			hasChange = true
		}
	}
	if !hasChange {
		return fmt.Errorf("%w: modify order requires a changed field", ErrInvalidOrder)
	}
	return nil
}

func ApplyOrderModification(order OrderStatusReport, modify ModifyOrder) (OrderStatusReport, bool, error) {
	if err := modify.Validate(); err != nil {
		return OrderStatusReport{}, false, err
	}
	if modify.AccountID != order.AccountID {
		return OrderStatusReport{}, false, fmt.Errorf("%w: account mismatch", ErrInvalidOrder)
	}
	if modify.InstrumentID != order.InstrumentID {
		return OrderStatusReport{}, false, fmt.Errorf("%w: instrument mismatch", ErrInvalidOrder)
	}
	if modify.OrderID != "" && order.OrderID != "" && modify.OrderID != order.OrderID {
		return OrderStatusReport{}, false, fmt.Errorf("%w: order id mismatch", ErrInvalidOrder)
	}
	if modify.ClientOrderID != "" && order.ClientOrderID != "" && modify.ClientOrderID != order.ClientOrderID {
		return OrderStatusReport{}, false, fmt.Errorf("%w: client order id mismatch", ErrInvalidOrder)
	}
	if modify.VenueOrderID != "" && order.VenueOrderID != "" && modify.VenueOrderID != order.VenueOrderID {
		return OrderStatusReport{}, false, fmt.Errorf("%w: venue order id mismatch", ErrInvalidOrder)
	}
	if !order.Status.IsOpen() || order.Status == OrderStatusPendingCancel {
		return OrderStatusReport{}, false, fmt.Errorf("%w: order is not modifiable", ErrInvalidOrder)
	}

	updated := order
	changed := false
	if modify.Quantity.IsPositive() {
		if modify.Quantity.LessThan(order.FilledQuantity) {
			return OrderStatusReport{}, false, fmt.Errorf("%w: quantity below filled quantity", ErrInvalidOrder)
		}
		if !modify.Quantity.Equal(order.Quantity) {
			updated.Quantity = modify.Quantity
			updated.LeavesQuantity = modify.Quantity.Sub(order.FilledQuantity)
			if updated.LeavesQuantity.IsNegative() {
				updated.LeavesQuantity = decimal.Zero
			}
			changed = true
		}
	}
	if modify.Price.IsPositive() {
		if !orderTypeAllowsLimitPrice(order.Type) {
			return OrderStatusReport{}, false, fmt.Errorf("%w: order type does not support price modification", ErrInvalidOrder)
		}
		if !modify.Price.Equal(order.Price) {
			updated.Price = modify.Price
			changed = true
		}
	}
	if modify.TriggerPrice.IsPositive() {
		if !orderTypeAllowsTriggerPrice(order.Type) {
			return OrderStatusReport{}, false, fmt.Errorf("%w: order type does not support trigger modification", ErrInvalidOrder)
		}
		if !modify.TriggerPrice.Equal(order.TriggerPrice) {
			updated.TriggerPrice = modify.TriggerPrice
			changed = true
		}
	}
	if modify.ActivationPrice.IsPositive() {
		if !orderTypeAllowsTrailing(order.Type) {
			return OrderStatusReport{}, false, fmt.Errorf("%w: order type does not support activation modification", ErrInvalidOrder)
		}
		if !modify.ActivationPrice.Equal(order.ActivationPrice) {
			updated.ActivationPrice = modify.ActivationPrice
			changed = true
		}
	}
	if modify.TrailingOffset.IsPositive() {
		if !orderTypeAllowsTrailing(order.Type) {
			return OrderStatusReport{}, false, fmt.Errorf("%w: order type does not support trailing modification", ErrInvalidOrder)
		}
		if !modify.TrailingOffset.Equal(order.TrailingOffset) {
			updated.TrailingOffset = modify.TrailingOffset
			changed = true
		}
	}
	if modify.TimeInForce != "" && modify.TimeInForce != order.TimeInForce {
		updated.TimeInForce = modify.TimeInForce
		changed = true
	}
	if !changed {
		return OrderStatusReport{}, false, fmt.Errorf("%w: modify order did not change order", ErrInvalidOrder)
	}
	updated.Status = OrderStatusAccepted
	return updated, true, nil
}

func orderTypeAllowsLimitPrice(t OrderType) bool {
	return t == OrderTypeLimit ||
		t == OrderTypeStopLimit ||
		t == OrderTypeLimitIfTouched ||
		t == OrderTypeTrailingStopLimit ||
		t == OrderTypeMarketToLimit
}

func orderTypeAllowsTriggerPrice(t OrderType) bool {
	return t == OrderTypeStopMarket ||
		t == OrderTypeStopLimit ||
		t == OrderTypeMarketIfTouched ||
		t == OrderTypeLimitIfTouched
}

func orderTypeAllowsTrailing(t OrderType) bool {
	return t == OrderTypeTrailingStopMarket || t == OrderTypeTrailingStopLimit
}

type OrderStatusReport struct {
	AccountID           AccountID
	InstrumentID        InstrumentID
	OrderListID         OrderListID
	OrderID             OrderID
	VenueOrderID        VenueOrderID
	ParentClientOrderID ClientOrderID
	ClientOrderID       ClientOrderID
	Status              OrderStatus
	Side                OrderSide
	Type                OrderType
	Contingency         ContingencyType
	Quantity            decimal.Decimal
	FilledQuantity      decimal.Decimal
	LeavesQuantity      decimal.Decimal
	Price               decimal.Decimal
	TriggerPrice        decimal.Decimal
	ActivationPrice     decimal.Decimal
	TrailingOffset      decimal.Decimal
	PostOnly            bool
	ReduceOnly          bool
	TimeInForce         TimeInForce
	ExpireTime          time.Time
	AveragePrice        decimal.Decimal
	LastUpdatedTime     time.Time
}

func (r OrderStatusReport) Validate() error {
	if r.AccountID == "" || r.OrderID == "" || r.Status == "" {
		return fmt.Errorf("%w: invalid order report", ErrInvalidOrder)
	}
	if err := r.InstrumentID.Validate(); err != nil {
		return err
	}
	if err := r.Contingency.Validate(); err != nil {
		return err
	}
	if r.FilledQuantity.IsNegative() || r.Quantity.IsNegative() {
		return fmt.Errorf("%w: invalid order quantities", ErrInvalidOrder)
	}
	if r.LeavesQuantity.IsNegative() {
		return fmt.Errorf("%w: invalid leaves quantity", ErrInvalidOrder)
	}
	if r.Quantity.IsPositive() && r.FilledQuantity.GreaterThan(r.Quantity) {
		return fmt.Errorf("%w: filled quantity exceeds order quantity", ErrInvalidOrder)
	}
	return nil
}

type FillReport struct {
	AccountID     AccountID
	InstrumentID  InstrumentID
	OrderID       OrderID
	VenueOrderID  VenueOrderID
	ClientOrderID ClientOrderID
	TradeID       TradeID
	Side          OrderSide
	Price         decimal.Decimal
	Quantity      decimal.Decimal
	Fee           decimal.Decimal
	FeeCurrency   Currency
	Timestamp     time.Time
}

func (r FillReport) Validate() error {
	if r.AccountID == "" || r.OrderID == "" || r.TradeID == "" {
		return fmt.Errorf("%w: invalid fill report", ErrInvalidOrder)
	}
	if err := r.InstrumentID.Validate(); err != nil {
		return err
	}
	if !r.Price.IsPositive() || !r.Quantity.IsPositive() {
		return fmt.Errorf("%w: invalid fill price or quantity", ErrInvalidOrder)
	}
	if r.Fee.IsNegative() {
		return fmt.Errorf("%w: invalid fill fee", ErrInvalidOrder)
	}
	return nil
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
	EntryPrice   decimal.Decimal
	Timestamp    time.Time
}

func (r PositionStatusReport) Validate() error {
	if r.AccountID == "" || r.PositionID == "" {
		return fmt.Errorf("%w: invalid position report", ErrInvalidOrder)
	}
	if err := r.InstrumentID.Validate(); err != nil {
		return err
	}
	if r.EntryPrice.IsNegative() {
		return fmt.Errorf("%w: invalid position entry price", ErrInvalidOrder)
	}
	if r.Side != "" && r.Side != PositionSideLong && r.Side != PositionSideShort && r.Side != PositionSideFlat {
		return fmt.Errorf("%w: invalid position side", ErrInvalidOrder)
	}
	return nil
}
