package account_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/account"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubExchange struct {
	placed *exchanges.OrderParams
}

func (s *stubExchange) GetExchange() string { return "stub" }

func (s *stubExchange) GetMarketType() exchanges.MarketType { return exchanges.MarketTypeSpot }

func (s *stubExchange) Close() error { return nil }

func (s *stubExchange) FormatSymbol(symbol string) string { return symbol }

func (s *stubExchange) ExtractSymbol(symbol string) string { return symbol }

func (s *stubExchange) ListSymbols() []string { return nil }

func (s *stubExchange) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, nil
}

func (s *stubExchange) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, nil
}

func (s *stubExchange) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, nil
}

func (s *stubExchange) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, nil
}

func (s *stubExchange) PlaceOrder(_ context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	copy := *params
	s.placed = &copy
	return &exchanges.Order{}, nil
}

func (s *stubExchange) PlaceOrderWS(context.Context, *exchanges.OrderParams) error { return nil }

func (s *stubExchange) CancelOrder(context.Context, string, string) error { return nil }

func (s *stubExchange) CancelOrderWS(context.Context, string, string) error { return nil }

func (s *stubExchange) CancelAllOrders(context.Context, string) error { return nil }

func (s *stubExchange) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, nil
}

func (s *stubExchange) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (s *stubExchange) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, nil
}

func (s *stubExchange) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, nil
}

func (s *stubExchange) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return nil
}

func (s *stubExchange) GetLocalOrderBook(string, int) *exchanges.OrderBook { return nil }

func (s *stubExchange) StopWatchOrderBook(context.Context, string) error { return nil }

func (s *stubExchange) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error { return nil }

func (s *stubExchange) WatchFills(context.Context, exchanges.FillCallback) error { return nil }

func (s *stubExchange) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return nil
}

func (s *stubExchange) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return nil
}

func (s *stubExchange) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return nil
}

func (s *stubExchange) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return nil
}

func (s *stubExchange) StopWatchOrders(context.Context) error { return nil }

func (s *stubExchange) StopWatchFills(context.Context) error { return nil }

func (s *stubExchange) StopWatchPositions(context.Context) error { return nil }

func (s *stubExchange) StopWatchTicker(context.Context, string) error { return nil }

func (s *stubExchange) StopWatchTrades(context.Context, string) error { return nil }

func (s *stubExchange) StopWatchKlines(context.Context, string, exchanges.Interval) error { return nil }

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
	emitOrderOnCancel      *exchanges.Order
	emitPositionOnCancel   *exchanges.Position
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
	orderCancelEmits       atomic.Int32
	positionCancelEmits    atomic.Int32
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
			if s.emitOrderOnCancel != nil && s.orderCB != nil {
				copy := *s.emitOrderOnCancel
				s.orderCB(&copy)
				s.orderCancelEmits.Add(1)
			}
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
			if s.emitPositionOnCancel != nil && s.positionCB != nil {
				copy := *s.emitPositionOnCancel
				s.positionCB(&copy)
				s.positionCancelEmits.Add(1)
			}
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
	s.emitOrderCallbacks(syncUpdates)
	if s.placeReturnDelay > 0 {
		time.Sleep(s.placeReturnDelay)
	}
	if len(updates) > 0 {
		go func() {
			for _, update := range updates {
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
				s.emitOrderCallbacks([]*exchanges.Order{update})
			}
		}()
	}

	return order, nil
}

func (s *accountRuntimeStubExchange) PlaceOrderWS(ctx context.Context, _ *exchanges.OrderParams) error {
	updates := append([]*exchanges.Order(nil), s.updates...)
	syncUpdates := append([]*exchanges.Order(nil), s.syncPlaceWSUpdates...)
	s.emitOrderCallbacks(syncUpdates)
	if s.placeWSReturnDelay > 0 {
		time.Sleep(s.placeWSReturnDelay)
	}
	if len(updates) > 0 {
		go func() {
			for _, update := range updates {
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Millisecond):
				}
				s.emitOrderCallbacks([]*exchanges.Order{update})
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

func (s *accountRuntimeStubExchange) EmitStaleOrder(order *exchanges.Order) {
	s.watchMu.Lock()
	callbacks := append([]exchanges.OrderUpdateCallback(nil), s.staleOrderCBs...)
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

func (s *accountRuntimeStubExchange) emitOrderCallbacks(updates []*exchanges.Order) {
	if len(updates) == 0 {
		return
	}

	s.watchMu.Lock()
	callback := s.orderCB
	s.watchMu.Unlock()
	if callback == nil {
		return
	}

	for _, update := range updates {
		if update == nil {
			continue
		}
		copy := *update
		callback(&copy)
	}
}

func TestTradingAccountPlaceReturnsFlowAndBackfillsOrderID(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		placeResp: &exchanges.Order{
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Side:          exchanges.OrderSideBuy,
			Type:          exchanges.OrderTypeLimit,
			Quantity:      decimal.RequireFromString("0.1"),
			Price:         decimal.RequireFromString("100"),
			Status:        exchanges.OrderStatusPending,
		},
		updates: []*exchanges.Order{{
			OrderID:       "exch-1",
			ClientOrderID: "cli-1",
			Symbol:        "ETH",
			Status:        exchanges.OrderStatusNew,
		}},
	}

	acct := account.NewTradingAccount(adp, nil)
	require.NoError(t, acct.Start(context.Background()))
	defer acct.Close()

	flow, err := acct.Place(context.Background(), &exchanges.OrderParams{
		Symbol:   "ETH",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeLimit,
		Quantity: decimal.RequireFromString("0.1"),
		Price:    decimal.RequireFromString("100"),
	})
	require.NoError(t, err)
	defer flow.Close()

	require.Eventually(t, func() bool {
		latest := flow.Latest()
		return latest != nil && latest.OrderID == "exch-1"
	}, time.Second, 10*time.Millisecond)
}

func TestTradingAccountEmptyQueries(t *testing.T) {
	acct := account.NewTradingAccount(nil, nil)

	_, ok := acct.OpenOrder("nonexistent")
	assert.False(t, ok)

	orders := acct.OpenOrders()
	assert.Empty(t, orders)

	positions := acct.Positions()
	assert.Empty(t, positions)
}

func TestTradingAccountStartFailsWhenFetchAccountFails(t *testing.T) {
	t.Parallel()

	adp := &accountRuntimeStubExchange{
		fetchAccountErr: errors.New("boom"),
	}
	acct := account.NewTradingAccount(adp, nil)

	err := acct.Start(context.Background())
	require.ErrorContains(t, err, "boom")
}
