package exchanges

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

type TradingAccount struct {
	state    *LocalState
	adp      Exchange
	logger   Logger
	flows    *orderFlowRegistry
	orderSub *Subscription[Order]
}

type TradingAccountOption func(*TradingAccount)

func NewTradingAccount(adp Exchange, logger Logger, _ ...TradingAccountOption) *TradingAccount {
	state := NewLocalState(adp, logger)
	return &TradingAccount{
		state:  state,
		adp:    adp,
		logger: state.logger,
		flows:  newOrderFlowRegistry(),
	}
}

func (a *TradingAccount) Start(ctx context.Context) error {
	if err := a.state.Start(ctx); err != nil {
		return err
	}
	a.orderSub = a.state.SubscribeOrders()
	go a.consumeOrderUpdates(ctx, a.orderSub)
	return nil
}

func (a *TradingAccount) Close() {
	if a.orderSub != nil {
		a.orderSub.Unsubscribe()
	}
	a.state.Close()
}

func (a *TradingAccount) Balance() decimal.Decimal { return a.state.GetBalance() }
func (a *TradingAccount) Position(symbol string) (*Position, bool) {
	return a.state.GetPosition(symbol)
}
func (a *TradingAccount) Positions() []Position                   { return a.state.GetAllPositions() }
func (a *TradingAccount) OpenOrder(orderID string) (*Order, bool) { return a.state.GetOrder(orderID) }
func (a *TradingAccount) OpenOrders() []Order                     { return a.state.GetAllOpenOrders() }
func (a *TradingAccount) SubscribeOrders() *Subscription[Order]   { return a.state.SubscribeOrders() }
func (a *TradingAccount) SubscribePositions() *Subscription[Position] {
	return a.state.SubscribePositions()
}

func (a *TradingAccount) Place(ctx context.Context, params *OrderParams) (*OrderFlow, error) {
	order, err := a.adp.PlaceOrder(ctx, params)
	if err != nil {
		return nil, err
	}
	return a.flows.Register(order), nil
}

func (a *TradingAccount) PlaceWS(ctx context.Context, params *OrderParams) (*OrderFlow, error) {
	if strings.TrimSpace(params.ClientID) == "" {
		return nil, fmt.Errorf("client id required for PlaceWS")
	}
	if err := a.adp.PlaceOrderWS(ctx, params); err != nil {
		return nil, err
	}
	return a.flows.Register(&Order{
		ClientOrderID: params.ClientID,
		Symbol:        params.Symbol,
		Side:          params.Side,
		Type:          params.Type,
		Quantity:      params.Quantity,
		Price:         params.Price,
		Status:        OrderStatusPending,
	}), nil
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
	return a.flows.Register(&Order{
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
	}), nil
}

func (a *TradingAccount) consumeOrderUpdates(ctx context.Context, sub *Subscription[Order]) {
	for {
		select {
		case <-ctx.Done():
			return
		case order, ok := <-sub.C:
			if !ok {
				return
			}
			a.flows.Route(order)
		}
	}
}
