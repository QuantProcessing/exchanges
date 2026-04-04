package exchanges

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

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
