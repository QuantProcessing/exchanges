package exchanges_test

import (
	"context"
	"sync/atomic"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

type accountRuntimeStubExchange struct {
	stubExchange
	fetchAccountResp    *exchanges.Account
	fetchAccountErr     error
	placeResp           *exchanges.Order
	updates             []*exchanges.Order
	syncPlaceUpdates    []*exchanges.Order
	syncPlaceWSUpdates  []*exchanges.Order
	positionUpdates     []*exchanges.Position
	watchOrdersErr      error
	watchPositionsErr   error
	placeReturnDelay    time.Duration
	placeWSReturnDelay  time.Duration
	orderCB             exchanges.OrderUpdateCallback
	positionCB          exchanges.PositionUpdateCallback
	fetchAccountCalls   atomic.Int32
	watchOrdersCalls    atomic.Int32
	watchPositionsCalls atomic.Int32
}

func (s *accountRuntimeStubExchange) FetchAccount(context.Context) (*exchanges.Account, error) {
	s.fetchAccountCalls.Add(1)
	if s.fetchAccountErr != nil {
		return nil, s.fetchAccountErr
	}
	if s.fetchAccountResp != nil {
		return s.fetchAccountResp, nil
	}
	return &exchanges.Account{}, nil
}

func (s *accountRuntimeStubExchange) WatchOrders(_ context.Context, cb exchanges.OrderUpdateCallback) error {
	s.watchOrdersCalls.Add(1)
	if s.watchOrdersErr != nil {
		return s.watchOrdersErr
	}
	s.orderCB = cb
	return nil
}

func (s *accountRuntimeStubExchange) WatchPositions(_ context.Context, cb exchanges.PositionUpdateCallback) error {
	s.watchPositionsCalls.Add(1)
	if s.watchPositionsErr != nil {
		return s.watchPositionsErr
	}
	s.positionCB = cb
	return nil
}

func (s *accountRuntimeStubExchange) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	order := &exchanges.Order{}
	if s.placeResp != nil {
		copy := *s.placeResp
		order = &copy
	}
	if order.Symbol == "" {
		order.Symbol = params.Symbol
	}

	updates := append([]*exchanges.Order(nil), s.updates...)
	syncUpdates := append([]*exchanges.Order(nil), s.syncPlaceUpdates...)
	if s.orderCB != nil {
		for _, update := range syncUpdates {
			if update == nil {
				continue
			}
			copy := *update
			s.orderCB(&copy)
		}
	}
	if s.placeReturnDelay > 0 {
		time.Sleep(s.placeReturnDelay)
	}
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

	return order, nil
}

func (s *accountRuntimeStubExchange) PlaceOrderWS(ctx context.Context, _ *exchanges.OrderParams) error {
	updates := append([]*exchanges.Order(nil), s.updates...)
	syncUpdates := append([]*exchanges.Order(nil), s.syncPlaceWSUpdates...)
	if s.orderCB != nil {
		for _, update := range syncUpdates {
			if update == nil {
				continue
			}
			copy := *update
			s.orderCB(&copy)
		}
	}
	if s.placeWSReturnDelay > 0 {
		time.Sleep(s.placeWSReturnDelay)
	}
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

func (s *accountRuntimeStubExchange) EmitOrder(order *exchanges.Order) {
	if s.orderCB == nil || order == nil {
		return
	}
	copy := *order
	s.orderCB(&copy)
}

func (s *accountRuntimeStubExchange) EmitPosition(pos *exchanges.Position) {
	if s.positionCB == nil || pos == nil {
		return
	}
	copy := *pos
	s.positionCB(&copy)
}
