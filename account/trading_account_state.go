package account

import (
	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

func (a *TradingAccount) Balance() decimal.Decimal {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.balance
}

func (a *TradingAccount) Position(symbol string) (*exchanges.Position, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	position, ok := a.positions[symbol]
	if !ok {
		return nil, false
	}
	copy := *position
	return &copy, true
}

func (a *TradingAccount) Positions() []exchanges.Position {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]exchanges.Position, 0, len(a.positions))
	for _, position := range a.positions {
		result = append(result, *position)
	}
	return result
}

func (a *TradingAccount) OpenOrder(orderID string) (*exchanges.Order, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	order, ok := a.orders[orderID]
	if !ok {
		return nil, false
	}
	copy := *order
	return &copy, true
}

func (a *TradingAccount) OpenOrders() []exchanges.Order {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]exchanges.Order, 0, len(a.orders))
	for _, order := range a.orders {
		result = append(result, *order)
	}
	return result
}

func (a *TradingAccount) SubscribeOrders() *Subscription[exchanges.Order] {
	return a.orderBus.Subscribe()
}

func (a *TradingAccount) SubscribePositions() *Subscription[exchanges.Position] {
	return a.positionBus.Subscribe()
}

func (a *TradingAccount) applyAccountSnapshot(runGen uint64, acc *exchanges.Account) {
	if !a.isActiveRun(runGen) {
		return
	}
	if acc == nil {
		acc = &exchanges.Account{}
	}

	orders := make(map[string]*exchanges.Order, len(acc.Orders))
	for _, order := range acc.Orders {
		copy := order
		orders[order.OrderID] = &copy
	}

	positions := make(map[string]*exchanges.Position, len(acc.Positions))
	for _, position := range acc.Positions {
		copy := position
		positions[position.Symbol] = &copy
	}

	a.mu.Lock()
	if !a.isActiveRun(runGen) {
		a.mu.Unlock()
		return
	}
	a.balance = acc.TotalBalance
	a.orders = orders
	a.positions = positions
	a.mu.Unlock()
}

func (a *TradingAccount) resetSnapshotState() {
	a.mu.Lock()
	a.balance = decimal.Zero
	a.orders = make(map[string]*exchanges.Order)
	a.positions = make(map[string]*exchanges.Position)
	a.mu.Unlock()
}

func (a *TradingAccount) applyOrderUpdate(runGen uint64, order *exchanges.Order) {
	if order == nil || !a.isActiveRun(runGen) {
		return
	}

	a.mu.Lock()
	if !a.isActiveRun(runGen) {
		a.mu.Unlock()
		return
	}
	isTerminal := order.Status == exchanges.OrderStatusFilled ||
		order.Status == exchanges.OrderStatusCancelled ||
		order.Status == exchanges.OrderStatusRejected

	if isTerminal {
		delete(a.orders, order.OrderID)
	} else {
		copy := *order
		a.orders[order.OrderID] = &copy
	}
	a.mu.Unlock()

	if !a.isActiveRun(runGen) {
		return
	}
	a.orderBus.Publish(order)
	a.flows.Route(order)
}

func (a *TradingAccount) applyPositionUpdate(runGen uint64, position *exchanges.Position) {
	if position == nil || !a.isActiveRun(runGen) {
		return
	}

	a.mu.Lock()
	if !a.isActiveRun(runGen) {
		a.mu.Unlock()
		return
	}
	copy := *position
	a.positions[position.Symbol] = &copy
	a.mu.Unlock()

	if !a.isActiveRun(runGen) {
		return
	}
	a.positionBus.Publish(position)
}
