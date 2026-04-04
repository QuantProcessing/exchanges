package exchanges

import (
	"github.com/shopspring/decimal"
)

func (a *TradingAccount) Balance() decimal.Decimal {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.balance
}

func (a *TradingAccount) Position(symbol string) (*Position, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	position, ok := a.positions[symbol]
	if !ok {
		return nil, false
	}
	copy := *position
	return &copy, true
}

func (a *TradingAccount) Positions() []Position {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]Position, 0, len(a.positions))
	for _, position := range a.positions {
		result = append(result, *position)
	}
	return result
}

func (a *TradingAccount) OpenOrder(orderID string) (*Order, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	order, ok := a.orders[orderID]
	if !ok {
		return nil, false
	}
	copy := *order
	return &copy, true
}

func (a *TradingAccount) OpenOrders() []Order {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]Order, 0, len(a.orders))
	for _, order := range a.orders {
		result = append(result, *order)
	}
	return result
}

func (a *TradingAccount) SubscribeOrders() *Subscription[Order] {
	return a.orderBus.Subscribe()
}

func (a *TradingAccount) SubscribePositions() *Subscription[Position] {
	return a.positionBus.Subscribe()
}

func (a *TradingAccount) applyAccountSnapshot(acc *Account) {
	if acc == nil {
		acc = &Account{}
	}

	orders := make(map[string]*Order, len(acc.Orders))
	for _, order := range acc.Orders {
		copy := order
		orders[order.OrderID] = &copy
	}

	positions := make(map[string]*Position, len(acc.Positions))
	for _, position := range acc.Positions {
		copy := position
		positions[position.Symbol] = &copy
	}

	a.mu.Lock()
	a.balance = acc.TotalBalance
	a.orders = orders
	a.positions = positions
	a.mu.Unlock()
}

func (a *TradingAccount) resetSnapshotState() {
	a.mu.Lock()
	a.balance = decimal.Zero
	a.orders = make(map[string]*Order)
	a.positions = make(map[string]*Position)
	a.mu.Unlock()
}

func (a *TradingAccount) applyOrderUpdate(order *Order) {
	if order == nil {
		return
	}

	a.mu.Lock()
	isTerminal := order.Status == OrderStatusFilled ||
		order.Status == OrderStatusCancelled ||
		order.Status == OrderStatusRejected

	if isTerminal {
		delete(a.orders, order.OrderID)
	} else {
		copy := *order
		a.orders[order.OrderID] = &copy
	}
	a.mu.Unlock()

	a.orderBus.Publish(order)
	a.flows.Route(order)
}

func (a *TradingAccount) applyPositionUpdate(position *Position) {
	if position == nil {
		return
	}

	a.mu.Lock()
	copy := *position
	a.positions[position.Symbol] = &copy
	a.mu.Unlock()

	a.positionBus.Publish(position)
}
