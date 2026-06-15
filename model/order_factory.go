package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

type OrderFactory struct {
	mu        sync.Mutex
	accountID AccountID
	prefix    string
	nextID    int
	nextList  int
	metadata  CommandMetadata
}

type OrderFactoryOption func(*OrderFactory)

func WithClientOrderIDPrefix(prefix string) OrderFactoryOption {
	return func(f *OrderFactory) {
		f.prefix = prefix
	}
}

func WithOrderMetadata(metadata CommandMetadata) OrderFactoryOption {
	return func(f *OrderFactory) {
		f.metadata = metadata.Clone()
	}
}

func NewOrderFactory(accountID AccountID, opts ...OrderFactoryOption) *OrderFactory {
	factory := &OrderFactory{accountID: accountID, prefix: string(accountID)}
	for _, opt := range opts {
		opt(factory)
	}
	return factory
}

func (f *OrderFactory) Market(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	return f.newOrder(instrumentID, side, OrderTypeMarket, quantity, decimal.Zero, opts...)
}

func (f *OrderFactory) Limit(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, price decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	return f.newOrder(instrumentID, side, OrderTypeLimit, quantity, price, opts...)
}

func (f *OrderFactory) MarketToLimit(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	return f.newOrder(instrumentID, side, OrderTypeMarketToLimit, quantity, decimal.Zero, opts...)
}

func (f *OrderFactory) StopMarket(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, triggerPrice decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	opts = append([]SubmitOrderOption{WithTriggerPrice(triggerPrice)}, opts...)
	return f.newOrder(instrumentID, side, OrderTypeStopMarket, quantity, decimal.Zero, opts...)
}

func (f *OrderFactory) StopLimit(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, price decimal.Decimal, triggerPrice decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	opts = append([]SubmitOrderOption{WithTriggerPrice(triggerPrice)}, opts...)
	return f.newOrder(instrumentID, side, OrderTypeStopLimit, quantity, price, opts...)
}

func (f *OrderFactory) MarketIfTouched(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, triggerPrice decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	opts = append([]SubmitOrderOption{WithTriggerPrice(triggerPrice)}, opts...)
	return f.newOrder(instrumentID, side, OrderTypeMarketIfTouched, quantity, decimal.Zero, opts...)
}

func (f *OrderFactory) LimitIfTouched(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, price decimal.Decimal, triggerPrice decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	opts = append([]SubmitOrderOption{WithTriggerPrice(triggerPrice)}, opts...)
	return f.newOrder(instrumentID, side, OrderTypeLimitIfTouched, quantity, price, opts...)
}

func (f *OrderFactory) TrailingStopMarket(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, trailingOffset decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	opts = append([]SubmitOrderOption{WithTrailingOffset(trailingOffset)}, opts...)
	return f.newOrder(instrumentID, side, OrderTypeTrailingStopMarket, quantity, decimal.Zero, opts...)
}

func (f *OrderFactory) TrailingStopLimit(instrumentID InstrumentID, side OrderSide, quantity decimal.Decimal, price decimal.Decimal, trailingOffset decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	opts = append([]SubmitOrderOption{WithTrailingOffset(trailingOffset)}, opts...)
	return f.newOrder(instrumentID, side, OrderTypeTrailingStopLimit, quantity, price, opts...)
}

type BracketOrderRequest struct {
	InstrumentID InstrumentID
	Side         OrderSide
	Quantity     decimal.Decimal
	EntryPrice   decimal.Decimal
	TakeProfit   decimal.Decimal
	StopLoss     decimal.Decimal
}

func (f *OrderFactory) Bracket(req BracketOrderRequest) OrderList {
	listID := f.nextOrderListID()
	entry := f.Limit(
		req.InstrumentID,
		req.Side,
		req.Quantity,
		req.EntryPrice,
		WithOrderListID(listID),
		WithContingency(ContingencyTypeOTO),
	)
	exitSide := req.Side.Opposite()
	stopLoss := f.StopMarket(
		req.InstrumentID,
		exitSide,
		req.Quantity,
		req.StopLoss,
		WithOrderListID(listID),
		WithParentClientOrderID(entry.ClientOrderID),
		WithContingency(ContingencyTypeOCO),
		WithReduceOnly(),
	)
	takeProfit := f.Limit(
		req.InstrumentID,
		exitSide,
		req.Quantity,
		req.TakeProfit,
		WithOrderListID(listID),
		WithParentClientOrderID(entry.ClientOrderID),
		WithContingency(ContingencyTypeOCO),
		WithReduceOnly(),
	)
	return OrderList{ID: listID, Orders: []SubmitOrder{entry, stopLoss, takeProfit}}
}

func (f *OrderFactory) newOrder(instrumentID InstrumentID, side OrderSide, typ OrderType, quantity decimal.Decimal, price decimal.Decimal, opts ...SubmitOrderOption) SubmitOrder {
	order := SubmitOrder{
		Metadata:      f.metadata.Clone(),
		AccountID:     f.accountID,
		InstrumentID:  instrumentID,
		ClientOrderID: f.nextClientOrderID(),
		Side:          side,
		Type:          typ,
		TimeInForce:   TimeInForceGTC,
		Quantity:      quantity,
		Price:         price,
	}
	for _, opt := range opts {
		opt(&order)
	}
	return order
}

func (f *OrderFactory) nextClientOrderID() ClientOrderID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return ClientOrderID(fmt.Sprintf("%s-%d", f.prefix, f.nextID))
}

func (f *OrderFactory) nextOrderListID() OrderListID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextList++
	return OrderListID(fmt.Sprintf("%s-list-%d", f.prefix, f.nextList))
}

type SubmitOrderOption func(*SubmitOrder)

func WithCommandMetadata(metadata CommandMetadata) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.Metadata = metadata.Clone()
	}
}

func WithClientOrderID(id ClientOrderID) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.ClientOrderID = id
	}
}

func WithOrderListID(id OrderListID) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.OrderListID = id
	}
}

func WithTriggerInstrumentID(id InstrumentID) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.TriggerInstrumentID = id
	}
}

func WithParentClientOrderID(id ClientOrderID) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.ParentClientOrderID = id
	}
}

func WithContingency(contingency ContingencyType) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.Contingency = contingency
	}
}

func WithTimeInForce(tif TimeInForce) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.TimeInForce = tif
	}
}

func WithPostOnly() SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.PostOnly = true
	}
}

func WithReduceOnly() SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.ReduceOnly = true
	}
}

func WithExpireTime(expire time.Time) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.ExpireTime = expire
	}
}

func WithTriggerPrice(price decimal.Decimal) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.TriggerPrice = price
	}
}

func WithActivationPrice(price decimal.Decimal) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.ActivationPrice = price
	}
}

func WithTrailingOffset(offset decimal.Decimal) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.TrailingOffset = offset
	}
}

func WithTrailingOffsetType(offsetType TrailingOffsetType) SubmitOrderOption {
	return func(order *SubmitOrder) {
		order.TrailingOffsetType = offsetType
	}
}
