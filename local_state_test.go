package exchanges_test

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type localStateStubExchange struct {
	stubExchange
	placeResp *exchanges.Order
	updates   []*exchanges.Order
	orderCB   exchanges.OrderUpdateCallback
}

func (s *localStateStubExchange) FetchAccount(context.Context) (*exchanges.Account, error) {
	return &exchanges.Account{}, nil
}

func (s *localStateStubExchange) WatchOrders(_ context.Context, cb exchanges.OrderUpdateCallback) error {
	s.orderCB = cb
	return nil
}

func (s *localStateStubExchange) EmitOrder(order *exchanges.Order) {
	if s.orderCB == nil || order == nil {
		return
	}
	copy := *order
	s.orderCB(&copy)
}

func (s *localStateStubExchange) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	order := *s.placeResp
	if order.Symbol == "" {
		order.Symbol = params.Symbol
	}

	updates := append([]*exchanges.Order(nil), s.updates...)
	if s.orderCB != nil && len(updates) > 0 {
		go func() {
			for _, update := range updates {
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
				copy := *update
				s.orderCB(&copy)
			}
		}()
	}

	return &order, nil
}

func (s *localStateStubExchange) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	updates := append([]*exchanges.Order(nil), s.updates...)
	if s.orderCB != nil && len(updates) > 0 {
		go func() {
			for _, update := range updates {
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
				copy := *update
				s.orderCB(&copy)
			}
		}()
	}

	return nil
}

func TestLocalStatePlaceOrderBackfillsOrderIDFromUpdates(t *testing.T) {
	t.Parallel()

	adp := &localStateStubExchange{
		placeResp: &exchanges.Order{
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusPending,
		},
		updates: []*exchanges.Order{
			{
				OrderID:       "exch-1",
				ClientOrderID: "cli-1",
				Symbol:        "ETH",
				Side:          exchanges.OrderSideBuy,
				Type:          exchanges.OrderTypeLimit,
				Quantity:      decimal.RequireFromString("0.1"),
				Price:         decimal.RequireFromString("100"),
				Status:        exchanges.OrderStatusNew,
			},
		},
	}

	state := exchanges.NewLocalState(adp, nil)
	require.NoError(t, state.Start(context.Background()))
	defer state.Close()

	result, err := state.PlaceOrder(context.Background(), &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
	})
	require.NoError(t, err)
	defer result.Done()

	require.Empty(t, result.Order.OrderID)
	require.Equal(t, "cli-1", result.Order.ClientOrderID)

	require.Eventually(t, func() bool {
		return result.Order.OrderID == "exch-1"
	}, time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := state.GetOrder("exch-1")
		return ok
	}, time.Second, 10*time.Millisecond)
}

func TestLocalStatePlaceOrderWSBackfillsOrderIDFromUpdates(t *testing.T) {
	t.Parallel()

	adp := &localStateStubExchange{
		updates: []*exchanges.Order{
			{
				OrderID:       "ws-exch-1",
				ClientOrderID: "ws-cli-1",
				Symbol:        "ETH",
				Side:          exchanges.OrderSideBuy,
				Type:          exchanges.OrderTypeLimit,
				Quantity:      decimal.RequireFromString("0.1"),
				Price:         decimal.RequireFromString("100"),
				Status:        exchanges.OrderStatusNew,
			},
		},
	}

	state := exchanges.NewLocalState(adp, nil)
	require.NoError(t, state.Start(context.Background()))
	defer state.Close()

	result, err := state.PlaceOrderWS(context.Background(), &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
		ClientID: "ws-cli-1",
	})
	require.NoError(t, err)
	defer result.Done()

	require.Empty(t, result.Order.OrderID)
	require.Equal(t, "ws-cli-1", result.Order.ClientOrderID)

	require.Eventually(t, func() bool {
		return result.Order.OrderID == "ws-exch-1"
	}, time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := state.GetOrder("ws-exch-1")
		return ok
	}, time.Second, 10*time.Millisecond)
}
