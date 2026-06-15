package strategy

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEngineDeliversSubscribedEvents(t *testing.T) {
	b := bus.New()
	s := &recordingStrategy{id: "s1"}
	engine := NewEngine(b)
	require.NoError(t, engine.Add(s))
	require.NoError(t, engine.Start(context.Background()))
	require.NoError(t, b.Publish(context.Background(), "execution", "filled"))
	require.NoError(t, b.Publish(context.Background(), "market.data", "ticker"))
	require.Eventually(t, func() bool {
		return s.hasMessages("filled", "ticker")
	}, eventuallyWait, eventuallyTick)
	require.NoError(t, engine.Stop(context.Background()))
	require.True(t, s.isStopped())
}

func TestEnginePassesCommandRuntimeToStrategies(t *testing.T) {
	b := bus.New()
	rt := &fakeRuntime{cache: cache.New()}
	s := &runtimeStrategy{id: "runtime"}
	engine := NewEngine(b, WithRuntime(rt))
	require.NoError(t, engine.Add(s))
	require.NoError(t, engine.Start(context.Background()))

	report, err := s.runtime.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		Quantity:      decimal.RequireFromString("1"),
		Metadata: model.CommandMetadata{
			CommandID:     "cmd-1",
			CorrelationID: "corr-1",
			Params:        map[string]string{"intent": "entry"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("order-1"), report.OrderID)
	require.Same(t, rt.cache, s.runtime.Cache())
	require.Equal(t, model.StrategyID("runtime"), rt.lastSubmit.Metadata.StrategyID)
	require.Equal(t, model.CommandID("cmd-1"), rt.lastSubmit.Metadata.CommandID)
	require.Equal(t, model.CorrelationID("corr-1"), rt.lastSubmit.Metadata.CorrelationID)
	require.Equal(t, "entry", rt.lastSubmit.Metadata.Params["intent"])
	require.False(t, rt.lastSubmit.Metadata.TsInit.IsZero())
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEngineFreezesRuntimeIdentityAtAdd(t *testing.T) {
	b := bus.New()
	rt := &fakeRuntime{cache: cache.New()}
	s := &mutableIDStrategy{id: "alpha"}
	engine := NewEngine(b, WithRuntime(rt), WithTraderID("TRADER-001"))
	require.NoError(t, engine.Add(s))
	s.id = "beta"
	require.NoError(t, engine.Start(context.Background()))

	_, err := s.runtime.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "client-immutable",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		Quantity:      decimal.RequireFromString("1"),
	})
	require.NoError(t, err)
	require.Equal(t, model.TraderID("TRADER-001"), rt.lastSubmit.Metadata.TraderID)
	require.Equal(t, model.StrategyID("alpha"), rt.lastSubmit.Metadata.StrategyID)
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEnginePassesStrategyScopedLogger(t *testing.T) {
	var buf bytes.Buffer
	rt := &fakeRuntime{
		cache:  cache.New(),
		logger: slog.New(slog.NewJSONHandler(&buf, nil)),
	}
	s := &runtimeStrategy{id: "logger-strategy"}
	engine := NewEngine(bus.New(), WithRuntime(rt), WithTraderID("TRADER-001"))
	require.NoError(t, engine.Add(s))
	require.NoError(t, engine.Start(context.Background()))

	s.runtime.Logger().Info("strategy ready", "phase", "start")

	line := strings.TrimSpace(buf.String())
	require.Contains(t, line, `"msg":"strategy ready"`)
	require.Contains(t, line, `"trader_id":"TRADER-001"`)
	require.Contains(t, line, `"strategy_id":"logger-strategy"`)
	require.Contains(t, line, `"phase":"start"`)
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEnginePassesCommandMetadataToDataRequests(t *testing.T) {
	b := bus.New()
	rt := &fakeRuntime{cache: cache.New()}
	s := &runtimeStrategy{id: "requester"}
	engine := NewEngine(b, WithRuntime(rt), WithTraderID("TRADER-REQ"))
	require.NoError(t, engine.Add(s))
	require.NoError(t, engine.Start(context.Background()))

	_, err := s.runtime.RequestData(context.Background(), model.DataRequest{
		Metadata:     model.CommandMetadata{CommandID: "request-command"},
		RequestID:    "request-1",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Type:         model.MarketDataTypeTicker,
	})
	require.NoError(t, err)
	require.Equal(t, model.TraderID("TRADER-REQ"), rt.lastDataRequest.Metadata.TraderID)
	require.Equal(t, model.StrategyID("requester"), rt.lastDataRequest.Metadata.StrategyID)
	require.Equal(t, model.CommandID("request-command"), rt.lastDataRequest.Metadata.CommandID)
	require.False(t, rt.lastDataRequest.Metadata.TsInit.IsZero())
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEngineRejectsInvalidStrategyIdentity(t *testing.T) {
	engine := NewEngine(bus.New())
	require.ErrorContains(t, engine.Add(nil), "strategy is required")
	require.ErrorContains(t, engine.Add(&recordingStrategy{}), "strategy id is required")
	require.NoError(t, engine.Add(&recordingStrategy{id: "duplicate"}))
	require.ErrorContains(t, engine.Add(&recordingStrategy{id: "duplicate"}), "duplicate strategy id")
}

func TestEngineSerializesAsyncCallbacksPerStrategy(t *testing.T) {
	b := bus.New()
	s := newNonReentrantStrategy()
	engine := NewEngine(b)
	require.NoError(t, engine.Add(s))
	require.NoError(t, engine.Start(context.Background()))

	require.NoError(t, b.Publish(context.Background(), TopicExecution, "first"))
	require.Eventually(t, func() bool {
		return s.enteredFirst.Load()
	}, eventuallyWait, eventuallyTick)

	require.NoError(t, b.Publish(context.Background(), TopicMarketData, "second"))
	time.Sleep(25 * time.Millisecond)
	close(s.release)

	require.Eventually(t, func() bool {
		return s.count.Load() == 2
	}, eventuallyWait, eventuallyTick)
	require.False(t, s.overlapped.Load())
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEngineIsolatesAsyncStrategyActors(t *testing.T) {
	b := bus.New()
	blocked := newBlockingStrategy("blocked")
	fast := &recordingStrategy{id: "fast"}
	engine := NewEngine(b)
	require.NoError(t, engine.Add(blocked))
	require.NoError(t, engine.Add(fast))
	require.NoError(t, engine.Start(context.Background()))

	require.NoError(t, b.Publish(context.Background(), TopicTimer, "isolation"))
	require.Eventually(t, func() bool {
		return blocked.entered.Load()
	}, eventuallyWait, eventuallyTick)
	require.Eventually(t, func() bool {
		return fast.hasMessages("isolation")
	}, eventuallyWait, eventuallyTick)
	close(blocked.release)
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEngineReportsStrategyActorErrorsWithoutSkippingPeers(t *testing.T) {
	b := bus.New()
	requireErr := errors.New("strategy actor failed")
	failing := &failingStrategy{id: "failing", err: requireErr}
	healthy := &recordingStrategy{id: "healthy"}
	engine := NewEngine(b)
	require.NoError(t, engine.Add(failing))
	require.NoError(t, engine.Add(healthy))
	require.NoError(t, engine.Start(context.Background()))

	require.NoError(t, b.Publish(context.Background(), TopicTimer, "peer-event"))
	select {
	case err := <-engine.Errors():
		require.ErrorIs(t, err, requireErr)
	case <-time.After(eventuallyWait):
		t.Fatal("engine did not report strategy actor error")
	}
	require.Eventually(t, func() bool {
		return healthy.hasMessages("peer-event")
	}, eventuallyWait, eventuallyTick)
	require.NoError(t, engine.Stop(context.Background()))
}

type recordingStrategy struct {
	mu      sync.Mutex
	id      string
	events  []bus.Envelope
	stopped bool
}

func (s *recordingStrategy) ID() string                             { return s.id }
func (s *recordingStrategy) OnStart(context.Context, Runtime) error { return nil }
func (s *recordingStrategy) OnEvent(_ context.Context, env bus.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, env)
	return nil
}
func (s *recordingStrategy) OnStop(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	return nil
}

func (s *recordingStrategy) hasMessages(messages ...any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[any]bool, len(s.events))
	for _, event := range s.events {
		seen[event.Message] = true
	}
	for _, message := range messages {
		if !seen[message] {
			return false
		}
	}
	return true
}

func (s *recordingStrategy) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

type runtimeStrategy struct {
	id      string
	runtime Runtime
}

func (s *runtimeStrategy) ID() string { return s.id }
func (s *runtimeStrategy) OnStart(_ context.Context, rt Runtime) error {
	s.runtime = rt
	return nil
}
func (s *runtimeStrategy) OnEvent(context.Context, bus.Envelope) error { return nil }
func (s *runtimeStrategy) OnStop(context.Context) error                { return nil }

type mutableIDStrategy struct {
	id      string
	runtime Runtime
}

func (s *mutableIDStrategy) ID() string { return s.id }
func (s *mutableIDStrategy) OnStart(_ context.Context, rt Runtime) error {
	s.runtime = rt
	return nil
}
func (s *mutableIDStrategy) OnEvent(context.Context, bus.Envelope) error { return nil }
func (s *mutableIDStrategy) OnStop(context.Context) error                { return nil }

type nonReentrantStrategy struct {
	active       atomic.Int32
	count        atomic.Int64
	enteredFirst atomic.Bool
	overlapped   atomic.Bool
	release      chan struct{}
}

func newNonReentrantStrategy() *nonReentrantStrategy {
	return &nonReentrantStrategy{release: make(chan struct{})}
}

func (s *nonReentrantStrategy) ID() string                             { return "non-reentrant" }
func (s *nonReentrantStrategy) OnStart(context.Context, Runtime) error { return nil }
func (s *nonReentrantStrategy) OnEvent(context.Context, bus.Envelope) error {
	if !s.active.CompareAndSwap(0, 1) {
		s.overlapped.Store(true)
		s.count.Add(1)
		return nil
	}
	defer s.active.Store(0)
	if s.count.Load() == 0 {
		s.enteredFirst.Store(true)
		<-s.release
	}
	s.count.Add(1)
	return nil
}
func (s *nonReentrantStrategy) OnStop(context.Context) error { return nil }

type blockingStrategy struct {
	id      string
	entered atomic.Bool
	release chan struct{}
}

func newBlockingStrategy(id string) *blockingStrategy {
	return &blockingStrategy{id: id, release: make(chan struct{})}
}

func (s *blockingStrategy) ID() string                             { return s.id }
func (s *blockingStrategy) OnStart(context.Context, Runtime) error { return nil }
func (s *blockingStrategy) OnEvent(context.Context, bus.Envelope) error {
	s.entered.Store(true)
	<-s.release
	return nil
}
func (s *blockingStrategy) OnStop(context.Context) error { return nil }

type failingStrategy struct {
	id  string
	err error
}

func (s *failingStrategy) ID() string                             { return s.id }
func (s *failingStrategy) OnStart(context.Context, Runtime) error { return nil }
func (s *failingStrategy) OnEvent(context.Context, bus.Envelope) error {
	return s.err
}
func (s *failingStrategy) OnStop(context.Context) error { return nil }

type fakeRuntime struct {
	cache           *cache.Cache
	factories       map[model.AccountID]*model.OrderFactory
	logger          *slog.Logger
	lastDataRequest model.DataRequest
	lastSubmit      model.SubmitOrder
}

func (r *fakeRuntime) Cache() *cache.Cache { return r.cache }
func (r *fakeRuntime) Portfolio() *portfolio.Portfolio {
	return nil
}
func (r *fakeRuntime) Clock() Clock { return WallClock{} }
func (r *fakeRuntime) Logger() *slog.Logger {
	return r.logger
}
func (r *fakeRuntime) SetTimer(context.Context, string, time.Duration) error {
	return nil
}
func (r *fakeRuntime) CancelTimer(context.Context, string) error {
	return nil
}
func (r *fakeRuntime) OrderFactory(accountID model.AccountID) *model.OrderFactory {
	if r.factories == nil {
		r.factories = make(map[model.AccountID]*model.OrderFactory)
	}
	if r.factories[accountID] == nil {
		r.factories[accountID] = model.NewOrderFactory(accountID)
	}
	return r.factories[accountID]
}
func (r *fakeRuntime) SubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (r *fakeRuntime) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (r *fakeRuntime) SubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeTicker})
}
func (r *fakeRuntime) UnsubscribeTicker(context.Context, model.InstrumentID) error { return nil }
func (r *fakeRuntime) SubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeTradeTick})
}
func (r *fakeRuntime) UnsubscribeTradeTicks(context.Context, model.InstrumentID) error { return nil }
func (r *fakeRuntime) SubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeQuoteTick})
}
func (r *fakeRuntime) UnsubscribeQuoteTicks(context.Context, model.InstrumentID) error { return nil }
func (r *fakeRuntime) SubscribeBars(ctx context.Context, barType model.BarType) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: barType.InstrumentID, Type: model.MarketDataTypeBar, BarType: barType})
}
func (r *fakeRuntime) UnsubscribeBars(context.Context, model.BarType) error { return nil }
func (r *fakeRuntime) SubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeOrderBook, Depth: depth})
}
func (r *fakeRuntime) UnsubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error {
	return nil
}
func (r *fakeRuntime) RequestData(_ context.Context, request model.DataRequest) (model.DataResponse, error) {
	r.lastDataRequest = request
	return model.DataResponse{
		Metadata:     request.Metadata,
		RequestID:    request.RequestID,
		InstrumentID: request.InstrumentID,
		Type:         request.Type,
		BarType:      request.BarType,
		IsFinal:      true,
	}, nil
}
func (r *fakeRuntime) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	r.lastSubmit = order
	return model.OrderStatusReport{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}, nil
}
func (r *fakeRuntime) SubmitOrderList(ctx context.Context, list model.OrderList) ([]model.OrderStatusReport, error) {
	reports := make([]model.OrderStatusReport, 0, len(list.Orders))
	for _, order := range list.Orders {
		if order.ParentClientOrderID != "" {
			continue
		}
		report, err := r.SubmitOrder(ctx, order)
		if err != nil {
			return reports, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
func (r *fakeRuntime) ModifyOrder(context.Context, model.ModifyOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}, nil
}
func (r *fakeRuntime) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (r *fakeRuntime) BatchCancelOrders(context.Context, model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (r *fakeRuntime) CancelAllOrders(context.Context, model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	return nil, nil
}
func (r *fakeRuntime) QueryOrder(context.Context, model.QueryOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (r *fakeRuntime) QueryAccount(_ context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: query.AccountID}, nil
}
