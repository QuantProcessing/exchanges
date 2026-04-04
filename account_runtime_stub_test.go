package exchanges_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
)

type accountRuntimeStubExchange struct {
	stubExchange
	fetchAccountResp       *exchanges.Account
	fetchAccountErr        error
	fetchAccountBlock      chan struct{}
	fetchAccountStarted    chan struct{}
	placeResp              *exchanges.Order
	updates                []*exchanges.Order
	syncPlaceUpdates       []*exchanges.Order
	syncPlaceWSUpdates     []*exchanges.Order
	positionUpdates        []*exchanges.Position
	watchOrdersErr         error
	watchPositionsErr      error
	keepCanceledCallbacks  bool
	placeReturnDelay       time.Duration
	placeWSReturnDelay     time.Duration
	orderCB                exchanges.OrderUpdateCallback
	positionCB             exchanges.PositionUpdateCallback
	staleOrderCBs          []exchanges.OrderUpdateCallback
	stalePositionCBs       []exchanges.PositionUpdateCallback
	fetchAccountCalls      atomic.Int32
	watchOrdersCalls       atomic.Int32
	watchPositionsCalls    atomic.Int32
	watchOrdersCanceled    atomic.Int32
	watchPositionsCanceled atomic.Int32
	orderWatchID           atomic.Int64
	positionWatchID        atomic.Int64
	watchMu                sync.Mutex
}

func (s *accountRuntimeStubExchange) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	s.fetchAccountCalls.Add(1)
	if s.fetchAccountStarted != nil {
		select {
		case <-s.fetchAccountStarted:
		default:
			close(s.fetchAccountStarted)
		}
	}
	if s.fetchAccountBlock != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.fetchAccountBlock:
		}
	}
	if s.fetchAccountErr != nil {
		return nil, s.fetchAccountErr
	}
	if s.fetchAccountResp != nil {
		return s.fetchAccountResp, nil
	}
	return &exchanges.Account{}, nil
}

func (s *accountRuntimeStubExchange) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	s.watchOrdersCalls.Add(1)
	if s.watchOrdersErr != nil {
		return s.watchOrdersErr
	}
	watchID := s.orderWatchID.Add(1)
	s.watchMu.Lock()
	s.orderCB = cb
	s.watchMu.Unlock()
	go func() {
		<-ctx.Done()
		s.watchMu.Lock()
		if s.orderWatchID.Load() == watchID {
			if s.keepCanceledCallbacks && s.orderCB != nil {
				s.staleOrderCBs = append(s.staleOrderCBs, s.orderCB)
			}
			s.orderCB = nil
		}
		s.watchMu.Unlock()
		s.watchOrdersCanceled.Add(1)
	}()
	return nil
}

func (s *accountRuntimeStubExchange) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	s.watchPositionsCalls.Add(1)
	if s.watchPositionsErr != nil {
		return s.watchPositionsErr
	}
	watchID := s.positionWatchID.Add(1)
	s.watchMu.Lock()
	s.positionCB = cb
	s.watchMu.Unlock()
	go func() {
		<-ctx.Done()
		s.watchMu.Lock()
		if s.positionWatchID.Load() == watchID {
			if s.keepCanceledCallbacks && s.positionCB != nil {
				s.stalePositionCBs = append(s.stalePositionCBs, s.positionCB)
			}
			s.positionCB = nil
		}
		s.watchMu.Unlock()
		s.watchPositionsCanceled.Add(1)
	}()
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
	s.watchMu.Lock()
	callbacks := make([]exchanges.OrderUpdateCallback, 0, 1+len(s.staleOrderCBs))
	if s.orderCB != nil {
		callbacks = append(callbacks, s.orderCB)
	}
	callbacks = append(callbacks, s.staleOrderCBs...)
	s.watchMu.Unlock()
	if len(callbacks) == 0 || order == nil {
		return
	}
	for _, cb := range callbacks {
		if cb == nil {
			continue
		}
		copy := *order
		cb(&copy)
	}
}

func (s *accountRuntimeStubExchange) EmitPosition(pos *exchanges.Position) {
	s.watchMu.Lock()
	callbacks := make([]exchanges.PositionUpdateCallback, 0, 1+len(s.stalePositionCBs))
	if s.positionCB != nil {
		callbacks = append(callbacks, s.positionCB)
	}
	callbacks = append(callbacks, s.stalePositionCBs...)
	s.watchMu.Unlock()
	if len(callbacks) == 0 || pos == nil {
		return
	}
	for _, cb := range callbacks {
		if cb == nil {
			continue
		}
		copy := *pos
		cb(&copy)
	}
}
