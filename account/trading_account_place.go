package account

import (
	"context"
	"fmt"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

type placementClientIDInitializer interface {
	EnsureClientID(params *exchanges.OrderParams) error
}

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
	if err := a.ensurePlacementClientID(params); err != nil {
		return nil, err
	}

	flow := a.flows.Register(pendingPlacementOrder(params))
	order, err := a.adp.PlaceOrder(ctx, params)
	if err != nil {
		flow.Close()
		return nil, err
	}

	if order != nil && strings.TrimSpace(order.ClientOrderID) == "" {
		order = cloneOrder(order)
		order.ClientOrderID = params.ClientID
	}
	flow.seedPlacement(order)
	a.flows.Bind(flow, order)
	return flow, nil
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

func (a *TradingAccount) ensurePlacementClientID(params *exchanges.OrderParams) error {
	if params == nil {
		return fmt.Errorf("order params required")
	}

	if initializer, ok := a.adp.(placementClientIDInitializer); ok {
		if err := initializer.EnsureClientID(params); err != nil {
			return err
		}
	}

	params.ClientID = strings.TrimSpace(params.ClientID)
	if params.ClientID == "" {
		params.ClientID = exchanges.GenerateID()
	}
	return nil
}

func pendingPlacementOrder(params *exchanges.OrderParams) *exchanges.Order {
	if params == nil {
		return &exchanges.Order{Status: exchanges.OrderStatusPending, Timestamp: time.Now().UnixMilli()}
	}

	return &exchanges.Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		OrderPrice:    params.Price,
		ReduceOnly:    params.ReduceOnly,
		TimeInForce:   params.TimeInForce,
		Status:        exchanges.OrderStatusPending,
		Timestamp:     time.Now().UnixMilli(),
	}
}
