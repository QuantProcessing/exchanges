package exchanges

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

type TradingAccount struct {
	mu sync.RWMutex

	adp    Exchange
	logger Logger

	orders    map[string]*Order
	positions map[string]*Position
	balance   decimal.Decimal

	orderBus    *EventBus[Order]
	positionBus *EventBus[Position]
	flows       *orderFlowRegistry

	started bool
	done    chan struct{}
}

type TradingAccountOption func(*TradingAccount)

func NewTradingAccount(adp Exchange, logger Logger, _ ...TradingAccountOption) *TradingAccount {
	if logger == nil {
		logger = NopLogger
	}
	return &TradingAccount{
		adp:         adp,
		logger:      logger,
		orders:      make(map[string]*Order),
		positions:   make(map[string]*Position),
		orderBus:    NewEventBus[Order](),
		positionBus: NewEventBus[Position](),
		flows:       newOrderFlowRegistry(),
		done:        make(chan struct{}),
	}
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
