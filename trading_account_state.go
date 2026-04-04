package exchanges

import (
	"context"
	"fmt"
	"strings"
	"time"

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

func (a *TradingAccount) Place(ctx context.Context, params *OrderParams) (*OrderFlow, error) {
	allSub := a.SubscribeOrders()
	order, err := a.adp.PlaceOrder(ctx, params)
	if err != nil {
		allSub.Unsubscribe()
		return nil, err
	}

	return a.newFlowFromResult(a.trackOrderResult(allSub, order)), nil
}

func (a *TradingAccount) PlaceWS(ctx context.Context, params *OrderParams) (*OrderFlow, error) {
	if strings.TrimSpace(params.ClientID) == "" {
		return nil, fmt.Errorf("client id required for PlaceWS")
	}

	allSub := a.SubscribeOrders()
	if err := a.adp.PlaceOrderWS(ctx, params); err != nil {
		allSub.Unsubscribe()
		return nil, err
	}

	return a.newFlowFromResult(a.trackOrderResult(allSub, &Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	})), nil
}

func (a *TradingAccount) trackOrderResult(allSub *Subscription[Order], order *Order) *OrderResult {
	orderID := order.OrderID
	clientOrderID := order.ClientOrderID
	filteredCh := make(chan *Order, 16)
	result := &OrderResult{
		Order:  order,
		cancel: allSub.Unsubscribe,
	}

	go func() {
		defer close(filteredCh)
		for update := range allSub.C {
			match := (orderID != "" && update.OrderID == orderID) ||
				(clientOrderID != "" && update.ClientOrderID == clientOrderID)
			if !match {
				continue
			}

			updated := *update
			if updated.OrderID == "" {
				updated.OrderID = result.Order.OrderID
			}
			if updated.ClientOrderID == "" {
				updated.ClientOrderID = result.Order.ClientOrderID
			}
			*result.Order = updated
			orderID = result.Order.OrderID
			clientOrderID = result.Order.ClientOrderID

			select {
			case filteredCh <- update:
			default:
			}

			if update.Status == OrderStatusFilled ||
				update.Status == OrderStatusCancelled ||
				update.Status == OrderStatusRejected {
				allSub.Unsubscribe()
				return
			}
		}
	}()

	result.Sub = &Subscription[Order]{C: filteredCh}
	return result
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
