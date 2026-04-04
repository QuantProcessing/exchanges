package exchanges

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (a *TradingAccount) Cancel(ctx context.Context, orderID, symbol string) error {
	return a.adp.CancelOrder(ctx, orderID, symbol)
}

func (a *TradingAccount) CancelWS(ctx context.Context, orderID, symbol string) error {
	return a.adp.CancelOrderWS(ctx, orderID, symbol)
}

func (a *TradingAccount) Track(orderID, clientOrderID string) (*OrderFlow, error) {
	if strings.TrimSpace(orderID) == "" && strings.TrimSpace(clientOrderID) == "" {
		return nil, fmt.Errorf("order id or client order id required")
	}
	return a.flows.Register(&Order{
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
	}), nil
}

func (a *TradingAccount) Place(ctx context.Context, params *OrderParams) (*OrderFlow, error) {
	allSub := a.SubscribeOrders()
	order, err := a.adp.PlaceOrder(ctx, params)
	if err != nil {
		allSub.Unsubscribe()
		return nil, err
	}

	return a.newPlacedFlow(allSub, order), nil
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

	return a.newPlacedFlow(allSub, &Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}), nil
}

func (a *TradingAccount) newPlacedFlow(allSub *Subscription[Order], initial *Order) *OrderFlow {
	flow := newOrderFlow(initial)
	a.flows.Add(flow)

	go a.bridgePlacedFlow(flow, allSub, initial)

	return flow
}

func (a *TradingAccount) bridgePlacedFlow(flow *OrderFlow, allSub *Subscription[Order], initial *Order) {
	defer allSub.Unsubscribe()

	current := cloneOrder(initial)
	orderID := ""
	clientOrderID := ""
	if current != nil {
		orderID = current.OrderID
		clientOrderID = current.ClientOrderID
	}

	for {
		select {
		case <-flow.done:
			return
		case update, ok := <-allSub.C:
			if !ok {
				return
			}
			if !matchesTrackedOrder(update, orderID, clientOrderID) {
				continue
			}

			next := cloneOrder(update)
			if next == nil {
				continue
			}
			if next.OrderID == "" && current != nil {
				next.OrderID = current.OrderID
			}
			if next.ClientOrderID == "" && current != nil {
				next.ClientOrderID = current.ClientOrderID
			}

			current = next
			orderID = current.OrderID
			clientOrderID = current.ClientOrderID
			flow.publish(current)

			if isTerminalOrderStatus(current.Status) {
				return
			}
		}
	}
}

func matchesTrackedOrder(order *Order, orderID, clientOrderID string) bool {
	if order == nil {
		return false
	}
	return (orderID != "" && order.OrderID == orderID) ||
		(clientOrderID != "" && order.ClientOrderID == clientOrderID)
}

func isTerminalOrderStatus(status OrderStatus) bool {
	return status == OrderStatusFilled ||
		status == OrderStatusCancelled ||
		status == OrderStatusRejected
}
