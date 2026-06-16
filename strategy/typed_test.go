package strategy

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/portfolio"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTypedStrategyProvidesNautilusStyleCallbacks(t *testing.T) {
	b := bus.New()
	rt := newTypedRuntime()
	impl := &nautilusStyleStrategy{
		accountID:    "acct",
		instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
	}
	wrapped := NewTyped("imbalance", impl)
	engine := NewEngine(b, WithRuntime(rt))
	require.NoError(t, engine.Add(wrapped))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	require.Equal(t, "imbalance", wrapped.ID())
	require.Eventually(t, func() bool {
		return rt.hasSubscription(model.SubscribeMarketData{
			InstrumentID: impl.instrumentID,
			Type:         model.MarketDataTypeOrderBook,
			Depth:        2,
		})
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, b.Publish(context.Background(), TopicMarketData, model.MarketEvent{OrderBook: &model.OrderBook{
		InstrumentID: impl.instrumentID,
		Bids: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("100"),
			Size:  decimal.RequireFromString("3"),
		}},
		Asks: []model.OrderBookLevel{{
			Price: decimal.RequireFromString("101"),
			Size:  decimal.RequireFromString("1"),
		}},
		Timestamp: time.Unix(1, 0),
	}}))
	require.Eventually(t, func() bool {
		return rt.submittedClientOrderID() == "typed-client-1"
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, b.Publish(context.Background(), TopicExecution, model.ExecutionEvent{Order: &model.OrderStatusReport{
		AccountID:     "acct",
		InstrumentID:  impl.instrumentID,
		OrderID:       "order-1",
		ClientOrderID: "typed-client-1",
		Status:        model.OrderStatusAccepted,
	}}))
	require.NoError(t, b.Publish(context.Background(), TopicExecution, model.ExecutionEvent{Fill: &model.FillReport{
		AccountID:     "acct",
		InstrumentID:  impl.instrumentID,
		OrderID:       "order-1",
		ClientOrderID: "typed-client-1",
		TradeID:       "trade-1",
		Side:          model.OrderSideBuy,
		Price:         decimal.RequireFromString("101"),
		Quantity:      decimal.RequireFromString("0.01"),
		Timestamp:     time.Unix(2, 0),
	}}))
	require.Eventually(t, func() bool {
		return impl.counts() == [3]int{1, 1, 1}
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesTradeTickCallbacks(t *testing.T) {
	b := bus.New()
	impl := &tradeTickCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("trade-ticks", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	tick := model.TradeTick{
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Price:         decimal.RequireFromString("100.25"),
		Size:          decimal.RequireFromString("0.2"),
		AggressorSide: model.AggressorSideSeller,
		TradeID:       "venue-trade-1",
		Timestamp:     time.Unix(3, 0),
	}
	require.NoError(t, b.Publish(context.Background(), TopicMarketData, model.MarketEvent{Trade: &tick}))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got == tick
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesQuoteTickCallbacks(t *testing.T) {
	b := bus.New()
	impl := &quoteTickCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("quote-ticks", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	quote := model.QuoteTick{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1.5"),
		AskSize:      decimal.RequireFromString("2.5"),
		Timestamp:    time.Unix(4, 0),
	}
	require.NoError(t, b.Publish(context.Background(), TopicMarketData, model.MarketEvent{Quote: &quote}))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got == quote
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesBarCallbacks(t *testing.T) {
	b := bus.New()
	impl := &barCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("bars", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	bar := model.Bar{
		BarType:   model.NewTimeBarType(model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"), time.Minute),
		Open:      decimal.RequireFromString("100"),
		High:      decimal.RequireFromString("102"),
		Low:       decimal.RequireFromString("99"),
		Close:     decimal.RequireFromString("101"),
		Volume:    decimal.RequireFromString("12.5"),
		Timestamp: time.Unix(4, 0),
	}
	require.NoError(t, b.Publish(context.Background(), TopicMarketData, model.MarketEvent{Bar: &bar}))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got == bar
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesFundingRateCallbacks(t *testing.T) {
	b := bus.New()
	impl := &fundingRateCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("funding-rates", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	funding := model.FundingRate{
		InstrumentID:    model.MustInstrumentID("BTC-USDT-PERP.BINANCE"),
		Rate:            decimal.RequireFromString("0.0002"),
		MarkPrice:       decimal.RequireFromString("43125.50"),
		IndexPrice:      decimal.RequireFromString("43120.00"),
		NextFundingTime: time.Unix(800, 0),
		FundingInterval: 8 * time.Hour,
		Timestamp:       time.Unix(700, 0),
	}
	require.NoError(t, b.Publish(context.Background(), TopicMarketData, model.MarketEvent{FundingRate: &funding}))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got == funding
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesCustomDataCallbacks(t *testing.T) {
	b := bus.New()
	impl := &customDataCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("custom-data", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	custom := model.CustomData{
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		Type:         "funding_rate",
		Fields:       map[string]string{"rate": "0.0001"},
		Timestamp:    time.Unix(5, 0),
	}
	require.NoError(t, b.Publish(context.Background(), TopicMarketData, model.MarketEvent{Custom: &custom}))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got.InstrumentID == custom.InstrumentID && got.Type == custom.Type && got.Fields["rate"] == "0.0001"
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesTimerCallbacks(t *testing.T) {
	b := bus.New()
	impl := &timerCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("timers", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	event := TimerEvent{Name: "heartbeat", Timestamp: time.Unix(42, 0)}
	require.NoError(t, b.Publish(context.Background(), TopicTimer, event))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got == event
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesErrorCallbacks(t *testing.T) {
	b := bus.New()
	impl := &errorCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("errors", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	cause := errors.New("risk denied order")
	event := ErrorEvent{Source: "risk", Err: cause}
	require.NoError(t, b.Publish(context.Background(), TopicError, event))

	require.Eventually(t, func() bool {
		got, ok := impl.last()
		return ok && got.Source == "risk" && errors.Is(got.Err, cause)
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyConfigValidatesIdentity(t *testing.T) {
	impl := &timerCallbackStrategy{}
	wrapped, err := NewTypedWithConfig(StrategyConfig{ID: "configured-strategy"}, impl)
	require.NoError(t, err)
	require.Equal(t, "configured-strategy", wrapped.ID())

	_, err = NewTypedWithConfig(StrategyConfig{}, impl)
	require.ErrorContains(t, err, "strategy id is required")
}

func TestTypedStrategyDispatchesNautilusLifecycleEvents(t *testing.T) {
	b := bus.New()
	impl := &lifecycleCallbackStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("lifecycle", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	events := []model.OrderLifecycleEvent{
		lifecycleEvent(instID, model.OrderEventInitialized, model.OrderStatusInitialized),
		lifecycleEvent(instID, model.OrderEventDenied, model.OrderStatusDenied),
		lifecycleEvent(instID, model.OrderEventEmulated, model.OrderStatusEmulated),
		lifecycleEvent(instID, model.OrderEventReleased, model.OrderStatusReleased),
		lifecycleEvent(instID, model.OrderEventSubmitted, model.OrderStatusSubmitted),
		lifecycleEvent(instID, model.OrderEventAccepted, model.OrderStatusAccepted),
		lifecycleEvent(instID, model.OrderEventRejected, model.OrderStatusRejected),
		lifecycleEvent(instID, model.OrderEventTriggered, model.OrderStatusTriggered),
		lifecycleEvent(instID, model.OrderEventPendingUpdate, model.OrderStatusPendingUpdate),
		lifecycleEvent(instID, model.OrderEventUpdated, model.OrderStatusAccepted),
		lifecycleEvent(instID, model.OrderEventPendingCancel, model.OrderStatusPendingCancel),
		lifecycleEvent(instID, model.OrderEventCancelRejected, model.OrderStatusAccepted),
		lifecycleEvent(instID, model.OrderEventModifyRejected, model.OrderStatusAccepted),
		lifecycleEvent(instID, model.OrderEventCanceled, model.OrderStatusCanceled),
		lifecycleEvent(instID, model.OrderEventExpired, model.OrderStatusExpired),
		lifecycleEvent(instID, model.OrderEventPartiallyFilled, model.OrderStatusPartiallyFilled),
		lifecycleEvent(instID, model.OrderEventFilled, model.OrderStatusFilled),
	}
	for i := range events {
		require.NoError(t, b.Publish(context.Background(), TopicExecution, model.ExecutionEvent{Lifecycle: &events[i]}))
	}

	require.Eventually(t, func() bool {
		return impl.count(model.OrderEventDenied) == 1 &&
			impl.count(model.OrderEventEmulated) == 1 &&
			impl.count(model.OrderEventReleased) == 1 &&
			impl.count(model.OrderEventTriggered) == 1 &&
			impl.count(model.OrderEventPendingUpdate) == 1 &&
			impl.count(model.OrderEventUpdated) == 1 &&
			impl.count(model.OrderEventPendingCancel) == 1 &&
			impl.count(model.OrderEventCancelRejected) == 1 &&
			impl.count(model.OrderEventModifyRejected) == 1 &&
			impl.count(model.OrderEventExpired) == 1 &&
			impl.genericCount() == len(events)
	}, time.Second, 10*time.Millisecond)
}

func TestTypedStrategyDispatchesPositionLifecycleEvents(t *testing.T) {
	b := bus.New()
	impl := &positionLifecycleStrategy{}
	engine := NewEngine(b, WithRuntime(newTypedRuntime()))
	require.NoError(t, engine.Add(NewTyped("positions", impl)))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	instID := model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
	events := []model.PositionLifecycleEvent{
		positionLifecycleEvent(instID, model.PositionEventOpened, model.PositionSideFlat, model.PositionSideLong),
		positionLifecycleEvent(instID, model.PositionEventChanged, model.PositionSideLong, model.PositionSideLong),
		positionLifecycleEvent(instID, model.PositionEventClosed, model.PositionSideLong, model.PositionSideFlat),
	}
	for i := range events {
		require.NoError(t, b.Publish(context.Background(), TopicExecution, model.ExecutionEvent{PositionLifecycle: &events[i]}))
	}

	require.Eventually(t, func() bool {
		return impl.count(model.PositionEventOpened) == 1 &&
			impl.count(model.PositionEventChanged) == 1 &&
			impl.count(model.PositionEventClosed) == 1 &&
			impl.genericCount() == len(events)
	}, time.Second, 10*time.Millisecond)
}

func lifecycleEvent(instID model.InstrumentID, kind model.OrderEventKind, status model.OrderStatus) model.OrderLifecycleEvent {
	return model.OrderLifecycleEvent{
		AccountID:    "acct",
		InstrumentID: instID,
		OrderID:      model.OrderID("order-" + string(kind)),
		Kind:         kind,
		Status:       status,
	}
}

func positionLifecycleEvent(instID model.InstrumentID, kind model.PositionEventKind, previous model.PositionSide, side model.PositionSide) model.PositionLifecycleEvent {
	quantity := decimal.RequireFromString("1")
	if kind == model.PositionEventClosed {
		quantity = decimal.Zero
	}
	previousQuantity := decimal.Zero
	if previous != model.PositionSideFlat {
		previousQuantity = decimal.RequireFromString("1")
	}
	return model.PositionLifecycleEvent{
		AccountID:        "acct",
		InstrumentID:     instID,
		PositionID:       model.PositionID("pos-" + string(kind)),
		Kind:             kind,
		PreviousSide:     previous,
		PreviousQuantity: previousQuantity,
		Side:             side,
		Quantity:         quantity,
	}
}

type lifecycleCallbackStrategy struct {
	mu      sync.Mutex
	events  map[model.OrderEventKind]int
	generic int
}

func (s *lifecycleCallbackStrategy) OnOrderLifecycle(_ context.Context, event model.OrderLifecycleEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generic++
	return nil
}

func (s *lifecycleCallbackStrategy) OnOrderDenied(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderEmulated(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderReleased(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderTriggered(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderPendingUpdate(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderUpdated(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderPendingCancel(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderCancelRejected(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderModifyRejected(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *lifecycleCallbackStrategy) OnOrderExpired(ctx context.Context, event model.OrderLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}

func (s *lifecycleCallbackStrategy) recordSpecific(_ context.Context, event model.OrderLifecycleEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.events == nil {
		s.events = make(map[model.OrderEventKind]int)
	}
	s.events[event.Kind]++
	return nil
}

func (s *lifecycleCallbackStrategy) count(kind model.OrderEventKind) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events[kind]
}

func (s *lifecycleCallbackStrategy) genericCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generic
}

type positionLifecycleStrategy struct {
	mu      sync.Mutex
	events  map[model.PositionEventKind]int
	generic int
}

func (s *positionLifecycleStrategy) OnPositionLifecycle(_ context.Context, event model.PositionLifecycleEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generic++
	return nil
}

func (s *positionLifecycleStrategy) OnPositionOpened(ctx context.Context, event model.PositionLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *positionLifecycleStrategy) OnPositionChanged(ctx context.Context, event model.PositionLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}
func (s *positionLifecycleStrategy) OnPositionClosed(ctx context.Context, event model.PositionLifecycleEvent) error {
	return s.recordSpecific(ctx, event)
}

func (s *positionLifecycleStrategy) recordSpecific(_ context.Context, event model.PositionLifecycleEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.events == nil {
		s.events = make(map[model.PositionEventKind]int)
	}
	s.events[event.Kind]++
	return nil
}

func (s *positionLifecycleStrategy) count(kind model.PositionEventKind) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events[kind]
}

func (s *positionLifecycleStrategy) genericCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generic
}

type nautilusStyleStrategy struct {
	mu           sync.Mutex
	runtime      Runtime
	accountID    model.AccountID
	instrumentID model.InstrumentID
	books        int
	accepted     int
	filled       int
}

func (s *nautilusStyleStrategy) OnStart(ctx context.Context, rt Runtime) error {
	s.runtime = rt
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *nautilusStyleStrategy) OnOrderBook(ctx context.Context, book model.OrderBook) error {
	s.mu.Lock()
	s.books++
	s.mu.Unlock()
	order := s.runtime.OrderFactory(s.accountID).Limit(
		book.InstrumentID,
		model.OrderSideBuy,
		decimal.RequireFromString("0.01"),
		book.Asks[0].Price,
		model.WithClientOrderID("typed-client-1"),
	)
	_, err := s.runtime.SubmitOrder(ctx, order)
	return err
}

func (s *nautilusStyleStrategy) OnOrderAccepted(context.Context, model.OrderStatusReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accepted++
	return nil
}

func (s *nautilusStyleStrategy) OnOrderFilled(context.Context, model.FillReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filled++
	return nil
}

func (s *nautilusStyleStrategy) counts() [3]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return [3]int{s.books, s.accepted, s.filled}
}

type tradeTickCallbackStrategy struct {
	mu   sync.Mutex
	tick model.TradeTick
	seen bool
}

func (s *tradeTickCallbackStrategy) OnTradeTick(_ context.Context, tick model.TradeTick) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tick = tick
	s.seen = true
	return nil
}

func (s *tradeTickCallbackStrategy) last() (model.TradeTick, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tick, s.seen
}

type quoteTickCallbackStrategy struct {
	mu    sync.Mutex
	quote model.QuoteTick
	seen  bool
}

func (s *quoteTickCallbackStrategy) OnQuoteTick(_ context.Context, quote model.QuoteTick) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quote = quote
	s.seen = true
	return nil
}

func (s *quoteTickCallbackStrategy) last() (model.QuoteTick, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.quote, s.seen
}

type barCallbackStrategy struct {
	mu   sync.Mutex
	bar  model.Bar
	seen bool
}

func (s *barCallbackStrategy) OnBar(_ context.Context, bar model.Bar) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bar = bar
	s.seen = true
	return nil
}

func (s *barCallbackStrategy) last() (model.Bar, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bar, s.seen
}

type fundingRateCallbackStrategy struct {
	mu      sync.Mutex
	funding model.FundingRate
	seen    bool
}

func (s *fundingRateCallbackStrategy) OnFundingRate(_ context.Context, funding model.FundingRate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.funding = funding
	s.seen = true
	return nil
}

func (s *fundingRateCallbackStrategy) last() (model.FundingRate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.funding, s.seen
}

type customDataCallbackStrategy struct {
	mu     sync.Mutex
	custom model.CustomData
	seen   bool
}

func (s *customDataCallbackStrategy) OnCustomData(_ context.Context, custom model.CustomData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.custom = custom
	s.seen = true
	return nil
}

func (s *customDataCallbackStrategy) last() (model.CustomData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.custom, s.seen
}

type timerCallbackStrategy struct {
	mu    sync.Mutex
	event TimerEvent
	seen  bool
}

func (s *timerCallbackStrategy) OnTimer(_ context.Context, event TimerEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.event = event
	s.seen = true
	return nil
}

func (s *timerCallbackStrategy) last() (TimerEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.event, s.seen
}

type errorCallbackStrategy struct {
	mu    sync.Mutex
	event ErrorEvent
	seen  bool
}

func (s *errorCallbackStrategy) OnError(_ context.Context, event ErrorEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.event = event
	s.seen = true
	return nil
}

func (s *errorCallbackStrategy) last() (ErrorEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.event, s.seen
}

type typedRuntime struct {
	mu          sync.Mutex
	cache       *cache.Cache
	subs        []model.SubscribeMarketData
	submissions []model.SubmitOrder
	requests    []model.DataRequest
	factories   map[model.AccountID]*model.OrderFactory
	logger      *slog.Logger
}

func newTypedRuntime() *typedRuntime {
	return &typedRuntime{cache: cache.New(), factories: make(map[model.AccountID]*model.OrderFactory)}
}

func (r *typedRuntime) Cache() *cache.Cache { return r.cache }

func (r *typedRuntime) Portfolio() *portfolio.Portfolio { return nil }

func (r *typedRuntime) Clock() Clock { return WallClock{} }

func (r *typedRuntime) Logger() *slog.Logger {
	return r.logger
}

func (r *typedRuntime) SetTimer(context.Context, string, time.Duration) error {
	return nil
}

func (r *typedRuntime) CancelTimer(context.Context, string) error {
	return nil
}

func (r *typedRuntime) OrderFactory(accountID model.AccountID) *model.OrderFactory {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories[accountID] == nil {
		r.factories[accountID] = model.NewOrderFactory(accountID)
	}
	return r.factories[accountID]
}

func (r *typedRuntime) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs = append(r.subs, sub)
	return nil
}

func (r *typedRuntime) UnsubscribeMarketData(context.Context, model.SubscribeMarketData) error {
	return nil
}
func (r *typedRuntime) SubscribeTicker(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeTicker})
}
func (r *typedRuntime) UnsubscribeTicker(context.Context, model.InstrumentID) error { return nil }
func (r *typedRuntime) SubscribeTradeTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeTradeTick})
}
func (r *typedRuntime) UnsubscribeTradeTicks(context.Context, model.InstrumentID) error { return nil }
func (r *typedRuntime) SubscribeQuoteTicks(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeQuoteTick})
}
func (r *typedRuntime) UnsubscribeQuoteTicks(context.Context, model.InstrumentID) error { return nil }
func (r *typedRuntime) SubscribeFundingRates(ctx context.Context, instrumentID model.InstrumentID) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeFundingRate})
}
func (r *typedRuntime) UnsubscribeFundingRates(context.Context, model.InstrumentID) error {
	return nil
}
func (r *typedRuntime) SubscribeBars(ctx context.Context, barType model.BarType) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: barType.InstrumentID, Type: model.MarketDataTypeBar, BarType: barType})
}
func (r *typedRuntime) UnsubscribeBars(context.Context, model.BarType) error { return nil }
func (r *typedRuntime) SubscribeOrderBookDepth(ctx context.Context, instrumentID model.InstrumentID, depth int) error {
	return r.SubscribeMarketData(ctx, model.SubscribeMarketData{InstrumentID: instrumentID, Type: model.MarketDataTypeOrderBook, Depth: depth})
}
func (r *typedRuntime) UnsubscribeOrderBookDepth(context.Context, model.InstrumentID, int) error {
	return nil
}

func (r *typedRuntime) RequestData(_ context.Context, request model.DataRequest) (model.DataResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, request)
	return model.DataResponse{
		Metadata:     request.Metadata,
		RequestID:    request.RequestID,
		InstrumentID: request.InstrumentID,
		Type:         request.Type,
		BarType:      request.BarType,
		IsFinal:      true,
	}, nil
}

func (r *typedRuntime) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.submissions = append(r.submissions, order)
	return model.OrderStatusReport{
		AccountID:     order.AccountID,
		InstrumentID:  order.InstrumentID,
		OrderID:       "order-1",
		ClientOrderID: order.ClientOrderID,
		Status:        model.OrderStatusAccepted,
	}, nil
}

func (r *typedRuntime) SubmitOrderList(ctx context.Context, list model.OrderList) ([]model.OrderStatusReport, error) {
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

func (r *typedRuntime) ModifyOrder(context.Context, model.ModifyOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:    "acct",
		InstrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:      "order-1",
		Status:       model.OrderStatusAccepted,
	}, nil
}

func (r *typedRuntime) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}

func (r *typedRuntime) BatchCancelOrders(context.Context, model.BatchCancelOrders) ([]model.OrderStatusReport, error) {
	return nil, nil
}

func (r *typedRuntime) CancelAllOrders(context.Context, model.CancelAllOrders) ([]model.OrderStatusReport, error) {
	return nil, nil
}

func (r *typedRuntime) QueryOrder(context.Context, model.QueryOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}

func (r *typedRuntime) QueryAccount(_ context.Context, query model.QueryAccount) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: query.AccountID}, nil
}

func (r *typedRuntime) hasSubscription(want model.SubscribeMarketData) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sub := range r.subs {
		if sub == want {
			return true
		}
	}
	return false
}

func (r *typedRuntime) submittedClientOrderID() model.ClientOrderID {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.submissions) == 0 {
		return ""
	}
	return r.submissions[0].ClientOrderID
}
