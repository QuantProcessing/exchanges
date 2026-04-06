package account

import (
	"context"
	"fmt"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
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
	return a.flows.Register(&exchanges.Order{
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
	}), nil
}

func (a *TradingAccount) Place(ctx context.Context, params *exchanges.OrderParams) (*OrderFlow, error) {
	order, err := a.adp.PlaceOrder(ctx, params)
	if err != nil {
		return nil, err
	}
	return a.flows.Register(order), nil
}

func (a *TradingAccount) PlaceWS(ctx context.Context, params *exchanges.OrderParams) (*OrderFlow, error) {
	if strings.TrimSpace(params.ClientID) == "" {
		return nil, fmt.Errorf("client id required for PlaceWS")
	}
	if err := a.adp.PlaceOrderWS(ctx, params); err != nil {
		return nil, err
	}
	return a.flows.Register(&exchanges.Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}), nil
}

func matchesTrackedOrder(order *exchanges.Order, orderID, clientOrderID string) bool {
	if order == nil {
		return false
	}
	return (orderID != "" && order.OrderID == orderID) ||
		(clientOrderID != "" && order.ClientOrderID == clientOrderID)
}

func isTerminalOrderStatus(status exchanges.OrderStatus) bool {
	return status == exchanges.OrderStatusFilled ||
		status == exchanges.OrderStatusCancelled ||
		status == exchanges.OrderStatusRejected
}
