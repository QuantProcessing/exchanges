package exchanges

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

type TradingAccount struct {
	mu       sync.Mutex
	state    *LocalState
	adp      Exchange
	logger   Logger
	flows    *orderFlowRegistry
	orderSub *Subscription[Order]
	started  bool
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
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return nil
	}
	if err := a.state.Start(ctx); err != nil {
		a.mu.Unlock()
		return err
	}
	a.orderSub = a.state.SubscribeOrders()
	a.started = true
	sub := a.orderSub
	a.mu.Unlock()
	go a.consumeOrderUpdates(ctx, sub)
	return nil
}

func (a *TradingAccount) Close() {
	a.mu.Lock()
	sub := a.orderSub
	a.orderSub = nil
	a.started = false
	a.mu.Unlock()

	if sub != nil {
		sub.Unsubscribe()
	}
	a.flows.CloseAll()
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
	result, err := a.state.PlaceOrder(ctx, params)
	if err != nil {
		return nil, err
	}
	return a.newFlowFromResult(result), nil
}

func (a *TradingAccount) PlaceWS(ctx context.Context, params *OrderParams) (*OrderFlow, error) {
	if strings.TrimSpace(params.ClientID) == "" {
		return nil, fmt.Errorf("client id required for PlaceWS")
	}
	result, err := a.state.PlaceOrderWS(ctx, params)
	if err != nil {
		return nil, err
	}
	return a.newFlowFromResult(result), nil
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

func (a *TradingAccount) newFlowFromResult(result *OrderResult) *OrderFlow {
	flow := newOrderFlow(result.Order)
	a.flows.Add(flow)

	go func() {
		defer result.Done()
		for {
			select {
			case <-flow.done:
				return
			case order, ok := <-result.Sub.C:
				if !ok {
					return
				}
				flow.publish(order)
			}
		}
	}()

	return flow
}
