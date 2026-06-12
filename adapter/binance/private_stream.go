package binance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type ResubscribeHook func(context.Context) error

type spotUserStream interface {
	Connect() error
	Close()
	SubscribeExecutionReport(func(*spot.ExecutionReportEvent))
	SubscribeAccountPosition(func(*spot.AccountPositionEvent))
}

type spotAPIStream interface {
	Close()
	SetPostReconnect(func())
}

type perpUserStream interface {
	Connect() error
	Close()
	SubscribeAccountUpdate(func(*perp.AccountUpdateEvent))
	SubscribeOrderUpdate(func(*perp.OrderUpdateEvent))
	SetOnResubscribe(func())
}

type spotPrivateStream struct {
	mu             sync.Mutex
	started        bool
	accountID      model.AccountID
	user           spotUserStream
	api            spotAPIStream
	onResub        ResubscribeHook
	emitAccount    func(model.AccountState)
	emitOrder      func(model.OrderStatusReport)
	emitOrderEvent func(model.OrderEvent)
	emitFill       func(model.FillReport)
	normalizer     symbolNormalizer
	lastReconnect  time.Time
}

func newSpotPrivateStream(accountID model.AccountID, user spotUserStream, api spotAPIStream, onResub ResubscribeHook, emitAccount func(model.AccountState), emitOrder func(model.OrderStatusReport), emitFill func(model.FillReport)) *spotPrivateStream {
	return &spotPrivateStream{
		accountID:   accountID,
		user:        user,
		api:         api,
		onResub:     onResub,
		emitAccount: emitAccount,
		emitOrder:   emitOrder,
		emitFill:    emitFill,
	}
}

func (s *spotPrivateStream) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if s.api != nil {
		s.api.SetPostReconnect(func() {
			s.handleResubscribe(context.Background())
		})
	}
	s.user.SubscribeExecutionReport(s.handleExecutionReport)
	s.user.SubscribeAccountPosition(s.handleAccountPosition)
	if err := s.user.Connect(); err != nil {
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *spotPrivateStream) Disconnect(context.Context) error {
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
	if s.user != nil {
		s.user.Close()
	}
	if s.api != nil {
		s.api.Close()
	}
	return nil
}

func (s *spotPrivateStream) handleResubscribe(ctx context.Context) {
	s.mu.Lock()
	s.lastReconnect = time.Now()
	s.mu.Unlock()
	if s.user != nil {
		_ = s.user.Connect()
	}
	if s.onResub != nil {
		_ = s.onResub(ctx)
	}
}

func (s *spotPrivateStream) handleExecutionReport(e *spot.ExecutionReportEvent) {
	id, err := s.normalizer.ToInstrumentID(e.Symbol, venue.ProductHintSpot)
	if err != nil {
		return
	}
	if s.emitOrder != nil {
		s.emitOrder(spotOrderReportFromStream(s.accountID, id, e))
	}
	if s.emitOrderEvent != nil {
		s.emitOrderEvent(spotOrderEventFromStream(s.accountID, id, e))
	}
	if s.emitFill != nil {
		if fill, ok := spotFillReportFromStream(s.accountID, id, e); ok {
			s.emitFill(fill)
		}
	}
}

func (s *spotPrivateStream) handleAccountPosition(e *spot.AccountPositionEvent) {
	if e == nil || s.emitAccount == nil {
		return
	}
	state := model.AccountState{
		AccountID: s.accountID,
		Venue:     model.VenueBinance,
		Type:      model.AccountTypeCash,
		Reported:  false,
		EventTime: timeFromUnixMilli(e.EventTime),
		InitTime:  time.Now(),
	}
	for _, b := range e.Balances {
		free := model.Money{Amount: parseDecimal(b.Free), Currency: model.Currency(b.Asset)}
		locked := model.Money{Amount: parseDecimal(b.Locked), Currency: model.Currency(b.Asset)}
		total := model.Money{Amount: free.Amount.Add(locked.Amount), Currency: model.Currency(b.Asset)}
		if total.Amount.IsZero() {
			continue
		}
		bal, err := model.NewBalance(total, locked, free)
		if err != nil {
			continue
		}
		state.Balances = append(state.Balances, bal)
	}
	s.emitAccount(state)
}

type perpPrivateStream struct {
	mu             sync.Mutex
	started        bool
	accountID      model.AccountID
	user           perpUserStream
	onResub        ResubscribeHook
	emitOrder      func(model.OrderStatusReport)
	emitOrderEvent func(model.OrderEvent)
	emitFill       func(model.FillReport)
	emitPosition   func(model.PositionStatusReport)
	normalizer     symbolNormalizer
}

func newPerpPrivateStream(accountID model.AccountID, user perpUserStream, onResub ResubscribeHook, emitOrder func(model.OrderStatusReport), emitFill func(model.FillReport), emitPosition func(model.PositionStatusReport)) *perpPrivateStream {
	return &perpPrivateStream{
		accountID:    accountID,
		user:         user,
		onResub:      onResub,
		emitOrder:    emitOrder,
		emitFill:     emitFill,
		emitPosition: emitPosition,
	}
}

func (s *perpPrivateStream) Connect(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if s.onResub != nil {
		s.user.SetOnResubscribe(func() {
			_ = s.onResub(context.Background())
		})
	}
	s.user.SubscribeOrderUpdate(s.handleOrderUpdate)
	s.user.SubscribeAccountUpdate(s.handleAccountUpdate)
	if err := s.user.Connect(); err != nil {
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *perpPrivateStream) Disconnect(context.Context) error {
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
	if s.user != nil {
		s.user.Close()
	}
	return nil
}

func (s *perpPrivateStream) handleOrderUpdate(e *perp.OrderUpdateEvent) {
	id, err := s.normalizer.ToInstrumentID(e.Order.Symbol, venue.ProductHintPerp)
	if err != nil {
		return
	}
	if s.emitOrder != nil {
		s.emitOrder(perpOrderReportFromStream(s.accountID, id, e))
	}
	if s.emitOrderEvent != nil {
		s.emitOrderEvent(perpOrderEventFromStream(s.accountID, id, e))
	}
	if s.emitFill != nil {
		if fill, ok := perpFillReportFromStream(s.accountID, id, e); ok {
			s.emitFill(fill)
		}
	}
}

func (s *perpPrivateStream) handleAccountUpdate(e *perp.AccountUpdateEvent) {
	if e == nil || s.emitPosition == nil {
		return
	}
	positions, err := perpPositionsFromAccountUpdate(s.accountID, e)
	if err != nil {
		return
	}
	for _, pos := range positions {
		s.emitPosition(pos)
	}
}

type spotSDKUserStream struct {
	account *spot.WsAccountClient
}

func (s spotSDKUserStream) Connect() error { return s.account.Connect() }
func (s spotSDKUserStream) Close()         { s.account.Close() }
func (s spotSDKUserStream) SubscribeExecutionReport(h func(*spot.ExecutionReportEvent)) {
	s.account.SubscribeExecutionReport(h)
}
func (s spotSDKUserStream) SubscribeAccountPosition(h func(*spot.AccountPositionEvent)) {
	s.account.SubscribeAccountPosition(h)
}

type spotSDKAPIStream struct {
	api *spot.WsAPIClient
}

func (s spotSDKAPIStream) Close() { s.api.Close() }
func (s spotSDKAPIStream) SetPostReconnect(h func()) {
	s.api.SetPostReconnect(h)
}

type perpSDKUserStream struct {
	account *perp.WsAccountClient
}

func (s perpSDKUserStream) Connect() error { return s.account.Connect() }
func (s perpSDKUserStream) Close()         { s.account.Close() }
func (s perpSDKUserStream) SubscribeAccountUpdate(h func(*perp.AccountUpdateEvent)) {
	s.account.SubscribeAccountUpdate(h)
}
func (s perpSDKUserStream) SubscribeOrderUpdate(h func(*perp.OrderUpdateEvent)) {
	s.account.SubscribeOrderUpdate(h)
}
func (s perpSDKUserStream) SetOnResubscribe(h func()) {
	s.account.SetOnResubscribe(h)
}

func requirePrivateCredentials(apiKey, secretKey string) error {
	if apiKey == "" || secretKey == "" {
		return fmt.Errorf("%w: binance private stream requires api_key and secret_key", model.ErrInvalidAccountState)
	}
	return nil
}
