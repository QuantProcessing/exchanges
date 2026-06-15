package live

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestRunnerHealthTracksKernelLifecycleState(t *testing.T) {
	node := platform.NewNode(platform.Config{Bus: bus.New(), Cache: cache.New()})
	runner := NewRunner(Config{Node: node})
	require.Equal(t, kernel.ComponentStateInitialized, runner.Health().State)
	require.Equal(t, kernel.ComponentStateInitialized, runner.Health().Platform.State)

	require.NoError(t, runner.Start(context.Background()))
	health := runner.Health()
	require.Equal(t, kernel.ComponentStateRunning, health.State)
	require.Equal(t, kernel.ComponentStateRunning, health.Platform.State)

	require.NoError(t, runner.Stop(context.Background()))
	health = runner.Health()
	require.Equal(t, kernel.ComponentStateStopped, health.State)
	require.Equal(t, kernel.ComponentStateStopped, health.Platform.State)
}

func TestTradingNodeHealthIncludesPlatformRiskLifecycle(t *testing.T) {
	engine := risk.NewEngine(cache.New(), risk.Config{})
	node, err := NewTradingNode(NodeConfig{Risk: engine})
	require.NoError(t, err)
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().State)
	require.Equal(t, kernel.ComponentStateInitialized, node.Health().Platform.Risk.State)

	require.NoError(t, node.Start(context.Background()))
	health := node.Health()
	require.Equal(t, kernel.ComponentStateRunning, health.State)
	require.Equal(t, kernel.ComponentStateRunning, health.Platform.State)
	require.Equal(t, kernel.ComponentStateRunning, health.Platform.Risk.State)

	require.NoError(t, node.Stop(context.Background()))
	health = node.Health()
	require.Equal(t, kernel.ComponentStateStopped, health.State)
	require.Equal(t, kernel.ComponentStateStopped, health.Platform.Risk.State)
}

func TestLiveHealthIncludesClientAndStrategySnapshots(t *testing.T) {
	data := newLiveDataClient()
	exec := newLiveExecutionClient()
	rec := &recordingStrategy{id: "health-strategy"}
	node, err := NewTradingNode(NodeConfig{
		DataClients:      []venue.DataClient{data},
		ExecutionClients: []venue.ExecutionClient{exec},
		Strategies:       []strategy.Strategy{rec},
	})
	require.NoError(t, err)

	require.NoError(t, node.Start(context.Background()))
	health := node.Health()
	require.Len(t, health.Platform.Data, 1)
	require.Equal(t, model.Venue("BINANCE"), health.Platform.Data[0].Venue)
	require.Equal(t, "live-data", health.Platform.Data[0].ClientID)
	require.True(t, health.Platform.Data[0].Health.Connected)
	require.Len(t, health.Platform.Execution, 1)
	require.Equal(t, model.Venue("BINANCE"), health.Platform.Execution[0].Venue)
	require.Equal(t, model.AccountID("acct"), health.Platform.Execution[0].AccountID)
	require.True(t, health.Platform.Execution[0].Health.Connected)
	require.Len(t, health.Strategies, 1)
	require.Equal(t, "health-strategy", health.Strategies[0].ID)
	require.Equal(t, kernel.ComponentStateRunning, health.Strategies[0].State)

	require.NoError(t, node.Stop(context.Background()))
	health = node.Health()
	require.Equal(t, kernel.ComponentStateStopped, health.Strategies[0].State)
}

func TestRunnerStartsPlatformBeforeStrategies(t *testing.T) {
	rec := &shutdownRecorder{}
	b := bus.New()
	node := platform.NewNode(platform.Config{Bus: b, Cache: cache.New()})
	data := &startupDataClient{liveDataClient: newLiveDataClient(), rec: rec}
	exec := &startupExecutionClient{liveExecutionClient: newLiveExecutionClient(), rec: rec}
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	strat := &startupPhaseStrategy{id: "live", node: node, rec: rec}
	runner := NewRunner(Config{Node: node, Bus: b, Strategies: []strategy.Strategy{strat}})

	require.NoError(t, runner.Start(context.Background()))
	events := rec.Events()
	require.Less(t, indexOfEvent(events, "data-load"), indexOfEvent(events, "data-connect"), events)
	require.Less(t, indexOfEvent(events, "data-connect"), indexOfEvent(events, "exec-connect"), events)
	require.Less(t, indexOfEvent(events, "exec-connect"), indexOfEvent(events, "exec-query-account"), events)
	require.Less(t, indexOfEvent(events, "exec-query-account"), indexOfEvent(events, "strategy-start"), events)
	require.NoError(t, runner.Stop(context.Background()))
	require.True(t, strat.isStopped())
}

func TestRunnerPassesPlatformNodeRuntimeToStrategies(t *testing.T) {
	b := bus.New()
	node := platform.NewNode(platform.Config{Bus: b, Cache: cache.New()})
	data := newLiveDataClient()
	exec := newLiveExecutionClient()
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	rec := &runtimeCaptureStrategy{id: "runtime"}
	runner := NewRunner(Config{Node: node, Bus: b, Strategies: []strategy.Strategy{rec}})

	require.NoError(t, runner.Start(context.Background()))
	require.Same(t, node.Cache(), rec.runtime.Cache())

	report, err := rec.runtime.SubmitOrder(context.Background(), model.SubmitOrder{
		AccountID:     "acct",
		InstrumentID:  model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		ClientOrderID: "live-client-1",
		Side:          model.OrderSideBuy,
		Type:          model.OrderTypeMarket,
		Quantity:      decimal.RequireFromString("1"),
	})
	require.NoError(t, err)
	require.Equal(t, model.OrderID("live-order-1"), report.OrderID)
	require.Equal(t, model.StrategyID("runtime"), report.Metadata.StrategyID)
	_, orderCached := node.Cache().OrderByClientID("acct", "live-client-1")
	require.True(t, orderCached)
	require.NoError(t, runner.Stop(context.Background()))
}

func TestTradingNodeAppliesStrategyOnStartSubscriptions(t *testing.T) {
	data := newLiveDataClient()
	exec := newLiveExecutionClient()
	impl := &liveTypedSubscriptionStrategy{instrumentID: model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")}
	node, err := NewTradingNode(NodeConfig{
		DataClients:      []venue.DataClient{data},
		ExecutionClients: []venue.ExecutionClient{exec},
		Strategies:       []strategy.Strategy{strategy.NewTyped("live-typed", impl)},
	})
	require.NoError(t, err)

	require.NoError(t, node.Start(context.Background()))
	defer node.Stop(context.Background())

	require.Eventually(t, func() bool {
		return data.hasSubscription(model.SubscribeMarketData{
			InstrumentID: impl.instrumentID,
			Type:         model.MarketDataTypeOrderBook,
			Depth:        2,
		})
	}, time.Second, 10*time.Millisecond)

	data.EmitOrderBook(model.OrderBook{
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
	})
	require.Eventually(t, func() bool {
		return impl.bookCount() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestRunnerStopsStrategiesBeforePlatformShutdown(t *testing.T) {
	rec := &shutdownRecorder{}
	b := bus.New()
	node := platform.NewNode(platform.Config{Bus: b, Cache: cache.New()})
	data := &shutdownDataClient{liveDataClient: newLiveDataClient(), rec: rec}
	exec := &shutdownExecutionClient{liveExecutionClient: newLiveExecutionClient(), rec: rec}
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	runner := NewRunner(Config{
		Node:       node,
		Bus:        b,
		Strategies: []strategy.Strategy{&shutdownStrategy{id: "shutdown", rec: rec}},
	})

	require.NoError(t, runner.Start(context.Background()))
	require.NoError(t, runner.Stop(context.Background()))

	events := rec.Events()
	require.Less(t, indexOfEvent(events, "strategy-stop"), indexOfEvent(events, "data-disconnect"), events)
	require.Less(t, indexOfEvent(events, "strategy-stop"), indexOfEvent(events, "exec-disconnect"), events)
}

func TestRunnerGracefullyStopsOnFatalPlatformStreamException(t *testing.T) {
	rec := &shutdownRecorder{}
	b := bus.New()
	data := &fatalShutdownDataClient{
		shutdownDataClient: &shutdownDataClient{liveDataClient: newLiveDataClient(), rec: rec},
	}
	exec := &shutdownExecutionClient{liveExecutionClient: newLiveExecutionClient(), rec: rec}
	node := platform.NewNode(platform.Config{
		Bus:             b,
		Cache:           cache.New(),
		ReconnectPolicy: platform.RetryPolicy{MaxAttempts: 1},
	})
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	strat := &shutdownStrategy{id: "fatal-shutdown", rec: rec}
	runner := NewRunner(Config{Node: node, Bus: b, Strategies: []strategy.Strategy{strat}})

	require.NoError(t, runner.Start(context.Background()))
	data.BreakStream()

	require.Eventually(t, func() bool {
		health := runner.Health()
		return health.State == kernel.ComponentStateStopped &&
			health.Platform.State == kernel.ComponentStateStopped &&
			health.Platform.LastError != nil &&
			strat.isStopped()
	}, time.Second, 10*time.Millisecond)
	health := runner.Health()
	require.ErrorContains(t, health.Platform.LastError, "data client reused closed event channel")
	require.Contains(t, health.LastError, "data client reused closed event channel")
	events := rec.Events()
	require.Less(t, indexOfEvent(events, "strategy-stop"), indexOfEvent(events, "data-disconnect"), events)
	require.Less(t, indexOfEvent(events, "strategy-stop"), indexOfEvent(events, "exec-disconnect"), events)
}

func TestRunnerGracefullyStopsOnStrategyEngineException(t *testing.T) {
	rec := &shutdownRecorder{}
	b := bus.New()
	node := platform.NewNode(platform.Config{Bus: b, Cache: cache.New()})
	strat := &fatalEventStrategy{
		shutdownStrategy: shutdownStrategy{id: "fatal-event", rec: rec},
		err:              fmt.Errorf("strategy event handler failed"),
	}
	runner := NewRunner(Config{Node: node, Bus: b, Strategies: []strategy.Strategy{strat}})

	require.NoError(t, runner.Start(context.Background()))
	require.NoError(t, b.Publish(context.Background(), strategy.TopicTimer, strategy.TimerEvent{
		Name:      "fatal",
		Timestamp: time.Unix(1, 0),
	}))

	require.Eventually(t, func() bool {
		health := runner.Health()
		return health.State == kernel.ComponentStateStopped &&
			strat.isStopped() &&
			health.LastError == "strategy fatal-event event handler failed: strategy event handler failed"
	}, time.Second, 10*time.Millisecond)
}

type recordingStrategy struct {
	mu      sync.Mutex
	id      string
	events  []bus.Envelope
	started bool
	stopped bool
}

func (s *recordingStrategy) ID() string { return s.id }
func (s *recordingStrategy) OnStart(context.Context, strategy.Runtime) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = true
	return nil
}
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

func (s *recordingStrategy) hasTopic(topic string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, env := range s.events {
		if env.Topic == topic {
			return true
		}
	}
	return false
}

func (s *recordingStrategy) isStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *recordingStrategy) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

type runtimeCaptureStrategy struct {
	id      string
	runtime strategy.Runtime
}

func (s *runtimeCaptureStrategy) ID() string { return s.id }
func (s *runtimeCaptureStrategy) OnStart(_ context.Context, rt strategy.Runtime) error {
	s.runtime = rt
	return nil
}
func (s *runtimeCaptureStrategy) OnEvent(context.Context, bus.Envelope) error { return nil }
func (s *runtimeCaptureStrategy) OnStop(context.Context) error                { return nil }

type liveDataClient struct {
	provider *liveProvider
	events   chan model.MarketEvent
	mu       sync.Mutex
	subs     []model.SubscribeMarketData
}

func newLiveDataClient() *liveDataClient {
	return &liveDataClient{provider: newLiveProvider(), events: make(chan model.MarketEvent, 8)}
}
func (c *liveDataClient) Venue() model.Venue                    { return "BINANCE" }
func (c *liveDataClient) ClientID() string                      { return "live-data" }
func (c *liveDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *liveDataClient) Connect(context.Context) error         { return nil }
func (c *liveDataClient) Disconnect(context.Context) error      { return nil }
func (c *liveDataClient) Health() venue.DataHealth              { return venue.DataHealth{Connected: true} }
func (c *liveDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, nil
}
func (c *liveDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, nil
}
func (c *liveDataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = append(c.subs, sub)
	return nil
}
func (c *liveDataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.subs {
		if existing == sub {
			c.subs = append(c.subs[:i], c.subs[i+1:]...)
			return nil
		}
	}
	return nil
}
func (c *liveDataClient) Events() <-chan model.MarketEvent { return c.events }
func (c *liveDataClient) EmitOrderBook(book model.OrderBook) {
	c.events <- model.MarketEvent{OrderBook: &book}
}
func (c *liveDataClient) hasSubscription(want model.SubscribeMarketData) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sub := range c.subs {
		if sub == want {
			return true
		}
	}
	return false
}

type liveTypedSubscriptionStrategy struct {
	mu           sync.Mutex
	instrumentID model.InstrumentID
	books        int
}

func (s *liveTypedSubscriptionStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *liveTypedSubscriptionStrategy) OnOrderBook(context.Context, model.OrderBook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books++
	return nil
}

func (s *liveTypedSubscriptionStrategy) bookCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.books
}

type startupPhaseStrategy struct {
	mu      sync.Mutex
	id      string
	node    *platform.Node
	rec     *shutdownRecorder
	stopped bool
}

func (s *startupPhaseStrategy) ID() string { return s.id }

func (s *startupPhaseStrategy) OnStart(context.Context, strategy.Runtime) error {
	s.rec.Add("strategy-start")
	health := s.node.Health()
	if !health.Ready || health.State != kernel.ComponentStateRunning {
		return fmt.Errorf("platform not ready before strategy start: ready=%v state=%s", health.Ready, health.State)
	}
	return nil
}

func (s *startupPhaseStrategy) OnEvent(context.Context, bus.Envelope) error { return nil }

func (s *startupPhaseStrategy) OnStop(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	return nil
}

func (s *startupPhaseStrategy) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

type startupDataClient struct {
	*liveDataClient
	rec *shutdownRecorder
}

func (c *startupDataClient) Instruments() venue.InstrumentProvider {
	return startupInstrumentProvider{InstrumentProvider: c.liveDataClient.Instruments(), rec: c.rec}
}

func (c *startupDataClient) Connect(ctx context.Context) error {
	c.rec.Add("data-connect")
	return c.liveDataClient.Connect(ctx)
}

type startupInstrumentProvider struct {
	venue.InstrumentProvider
	rec *shutdownRecorder
}

func (p startupInstrumentProvider) LoadAll(ctx context.Context) error {
	p.rec.Add("data-load")
	return p.InstrumentProvider.LoadAll(ctx)
}

type startupExecutionClient struct {
	*liveExecutionClient
	rec *shutdownRecorder
}

func (c *startupExecutionClient) Connect(ctx context.Context) error {
	c.rec.Add("exec-connect")
	return c.liveExecutionClient.Connect(ctx)
}

func (c *startupExecutionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	c.rec.Add("exec-query-account")
	return c.liveExecutionClient.QueryAccount(ctx)
}

type shutdownRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *shutdownRecorder) Add(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *shutdownRecorder) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type shutdownStrategy struct {
	mu      sync.Mutex
	id      string
	rec     *shutdownRecorder
	stopped bool
}

func (s *shutdownStrategy) ID() string                                      { return s.id }
func (s *shutdownStrategy) OnStart(context.Context, strategy.Runtime) error { return nil }
func (s *shutdownStrategy) OnEvent(context.Context, bus.Envelope) error     { return nil }
func (s *shutdownStrategy) OnStop(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	s.rec.Add("strategy-stop")
	return nil
}

func (s *shutdownStrategy) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

type fatalEventStrategy struct {
	shutdownStrategy
	err error
}

func (s *fatalEventStrategy) OnEvent(context.Context, bus.Envelope) error {
	return s.err
}

type shutdownDataClient struct {
	*liveDataClient
	rec *shutdownRecorder
}

func (c *shutdownDataClient) Connect(context.Context) error {
	c.rec.Add("data-connect")
	return nil
}

func (c *shutdownDataClient) Disconnect(context.Context) error {
	c.rec.Add("data-disconnect")
	return nil
}

type fatalShutdownDataClient struct {
	*shutdownDataClient
}

func (c *fatalShutdownDataClient) BreakStream() {
	close(c.liveDataClient.events)
}

type shutdownExecutionClient struct {
	*liveExecutionClient
	rec *shutdownRecorder
}

func (c *shutdownExecutionClient) Connect(context.Context) error {
	c.rec.Add("exec-connect")
	return nil
}

func (c *shutdownExecutionClient) Disconnect(context.Context) error {
	c.rec.Add("exec-disconnect")
	return nil
}

func indexOfEvent(events []string, want string) int {
	for i, event := range events {
		if event == want {
			return i
		}
	}
	return len(events)
}

type liveProvider struct {
	inst model.Instrument
}

func newLiveProvider() *liveProvider {
	return &liveProvider{inst: model.Instrument{
		ID:        model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}}
}
func (p *liveProvider) LoadAll(context.Context) error { return nil }
func (p *liveProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.inst, p.inst.ID == id
}
func (p *liveProvider) List() []model.Instrument { return []model.Instrument{p.inst} }

type liveExecutionClient struct {
	events chan model.ExecutionEvent
	calls  []string
}

func newLiveExecutionClient() *liveExecutionClient {
	return &liveExecutionClient{events: make(chan model.ExecutionEvent, 4)}
}
func (c *liveExecutionClient) Venue() model.Venue               { return "BINANCE" }
func (c *liveExecutionClient) AccountID() model.AccountID       { return "acct" }
func (c *liveExecutionClient) Connect(context.Context) error    { return nil }
func (c *liveExecutionClient) Disconnect(context.Context) error { return nil }
func (c *liveExecutionClient) Health() venue.ExecutionHealth {
	return venue.ExecutionHealth{Connected: true}
}
func (c *liveExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	return model.AccountSnapshot{AccountID: "acct", Venue: "BINANCE"}, nil
}
func (c *liveExecutionClient) SubmitOrder(context.Context, model.SubmitOrder) (model.OrderStatusReport, error) {
	c.calls = append(c.calls, "submit")
	return model.OrderStatusReport{
		AccountID:      "acct",
		InstrumentID:   model.MustInstrumentID("BTC-USDT-SPOT.BINANCE"),
		OrderID:        "live-order-1",
		ClientOrderID:  "live-client-1",
		Status:         model.OrderStatusAccepted,
		Quantity:       decimal.RequireFromString("1"),
		LeavesQuantity: decimal.RequireFromString("1"),
	}, nil
}
func (c *liveExecutionClient) CancelOrder(context.Context, model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{}, nil
}
func (c *liveExecutionClient) GenerateOrderStatusReports(_ context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	return []model.OrderStatusReport{{AccountID: "acct", InstrumentID: id, OrderID: "startup", Status: model.OrderStatusAccepted}}, nil
}
func (c *liveExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
