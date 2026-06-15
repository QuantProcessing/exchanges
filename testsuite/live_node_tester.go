package testsuite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/kernel"
	"github.com/QuantProcessing/exchanges/live"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/risk"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type LiveNodeTesterConfig struct {
	InstrumentID model.InstrumentID
}

type LiveNodeTester struct {
	cfg LiveNodeTesterConfig
}

func NewLiveNodeTester(cfg LiveNodeTesterConfig) *LiveNodeTester {
	return &LiveNodeTester{cfg: cfg}
}

func (l *LiveNodeTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "live", []contractCase{
		{id: "TC-LIVE01", name: "Trading node assembles live runtime dependencies", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			node, err := fixture.NewNode()
			if err != nil {
				return err
			}
			if node.Bus() == nil || node.Cache() == nil || node.Portfolio() == nil || node.Platform() == nil {
				return fmt.Errorf("node dependencies were not assembled")
			}
			if node.Risk() == nil {
				return fmt.Errorf("node risk engine was not assembled")
			}
			if node.Cache() != fixture.cache {
				return fmt.Errorf("trading node did not preserve configured cache")
			}
			defaultNode, err := live.NewNodeBuilder().Build()
			if err != nil {
				return err
			}
			if defaultNode.Cache() == nil || defaultNode.Portfolio() == nil || defaultNode.Risk() == nil {
				return fmt.Errorf("node builder did not assemble default runtime dependencies")
			}
			if node.Health().State != kernel.ComponentStateInitialized {
				return fmt.Errorf("runner health state mismatch: %s", node.Health().State)
			}
			if node.Health().Platform.State != kernel.ComponentStateInitialized {
				return fmt.Errorf("platform health state mismatch: %s", node.Health().Platform.State)
			}
			return nil
		}},
		{id: "TC-LIVE02", name: "Live node start and stop update lifecycle health", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			node, err := fixture.NewNode()
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			health := node.Health()
			if health.State != kernel.ComponentStateRunning || health.Platform.State != kernel.ComponentStateRunning {
				_ = node.Stop(context.Background())
				return fmt.Errorf("running health mismatch: runner=%s platform=%s", health.State, health.Platform.State)
			}
			if !health.Platform.Ready || health.Platform.Risk.State != kernel.ComponentStateRunning {
				_ = node.Stop(context.Background())
				return fmt.Errorf("platform readiness mismatch: ready=%v risk=%s", health.Platform.Ready, health.Platform.Risk.State)
			}
			if !fixture.data.Connected() || !fixture.execution.Connected() {
				_ = node.Stop(context.Background())
				return fmt.Errorf("venue clients were not connected")
			}
			if err := node.Stop(context.Background()); err != nil {
				return err
			}
			health = node.Health()
			if health.State != kernel.ComponentStateStopped || health.Platform.State != kernel.ComponentStateStopped {
				return fmt.Errorf("stopped health mismatch: runner=%s platform=%s", health.State, health.Platform.State)
			}
			if fixture.data.Connected() || fixture.execution.Connected() {
				return fmt.Errorf("venue clients were not disconnected")
			}
			return nil
		}},
		{id: "TC-LIVE03", name: "Startup applies strategy market-data subscriptions", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			impl := &liveNodeTypedSubscriptionStrategy{instrumentID: fixture.instrumentID}
			node, err := fixture.NewNode(strategy.NewTyped("live-subscription", impl))
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			defer node.Stop(context.Background())
			want := model.SubscribeMarketData{
				InstrumentID: fixture.instrumentID,
				Type:         model.MarketDataTypeOrderBook,
				Depth:        2,
			}
			return liveNodeEventually(time.Second, func() bool {
				return fixture.data.HasSubscription(want)
			}, "market-data subscription was not applied")
		}},
		{id: "TC-LIVE04", name: "Strategy runtime submits orders through execution client", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			rec := &liveNodeRuntimeCaptureStrategy{id: "live-runtime"}
			node, err := fixture.NewNode(rec)
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			defer node.Stop(context.Background())
			if rec.Runtime() == nil {
				return fmt.Errorf("strategy did not receive runtime")
			}
			report, err := rec.Runtime().SubmitOrder(ctx, model.SubmitOrder{
				AccountID:     fixture.execution.AccountID(),
				InstrumentID:  fixture.instrumentID,
				ClientOrderID: "tc-live04-client",
				Side:          model.OrderSideBuy,
				Type:          model.OrderTypeMarket,
				Quantity:      decimal.RequireFromString("1"),
			})
			if err != nil {
				return err
			}
			if report.Metadata.StrategyID != model.StrategyID(rec.id) || report.Metadata.TsInit.IsZero() {
				return fmt.Errorf("strategy metadata was not propagated: %#v", report.Metadata)
			}
			submitted, ok := fixture.execution.LastSubmit()
			if !ok || submitted.ClientOrderID != "tc-live04-client" || submitted.Metadata.StrategyID != model.StrategyID(rec.id) {
				return fmt.Errorf("execution client did not receive strategy command metadata: %#v", submitted)
			}
			cached, ok := node.Cache().OrderByClientID(fixture.execution.AccountID(), "tc-live04-client")
			if !ok || cached.OrderID != report.OrderID || cached.Metadata.StrategyID != model.StrategyID(rec.id) {
				return fmt.Errorf("submitted order was not cached with metadata: %#v", cached)
			}
			return nil
		}},
		{id: "TC-LIVE05", name: "Live market-data events reach typed strategy callbacks", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			impl := &liveNodeTypedSubscriptionStrategy{instrumentID: fixture.instrumentID}
			node, err := fixture.NewNode(strategy.NewTyped("live-market-events", impl))
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			defer node.Stop(context.Background())
			fixture.data.EmitOrderBook(model.OrderBook{
				InstrumentID: fixture.instrumentID,
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
			return liveNodeEventually(time.Second, func() bool {
				return impl.BookCount() == 1
			}, "typed strategy did not receive order book event")
		}},
		{id: "TC-LIVE06", name: "Startup phases complete before strategy start", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			rec := fixture.RecordStartup()
			var node *live.Node
			probe := &liveNodeStartupPhaseStrategy{
				node: func() *live.Node { return node },
				rec:  rec,
			}
			var err error
			node, err = fixture.NewNode(probe)
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			defer node.Stop(context.Background())
			events := rec.Events()
			for _, pair := range [][2]string{
				{"data-load", "data-connect"},
				{"data-connect", "exec-connect"},
				{"exec-connect", "exec-query-account"},
				{"exec-query-account", "strategy-start"},
			} {
				if !liveNodeEventBefore(events, pair[0], pair[1]) {
					return fmt.Errorf("startup phase order mismatch: expected %s before %s in %v", pair[0], pair[1], events)
				}
			}
			return nil
		}},
		{id: "TC-LIVE07", name: "Health snapshots include clients and strategies", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			strat := &liveNodeRuntimeCaptureStrategy{id: "live-health"}
			node, err := fixture.NewNode(strat)
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			health := node.Health()
			if len(health.Platform.Data) != 1 {
				_ = node.Stop(context.Background())
				return fmt.Errorf("expected one data health snapshot, got %d", len(health.Platform.Data))
			}
			if got := health.Platform.Data[0]; got.Venue != fixture.instrumentID.Venue || got.ClientID == "" || !got.Health.Connected {
				_ = node.Stop(context.Background())
				return fmt.Errorf("data health snapshot mismatch: %#v", got)
			}
			if len(health.Platform.Execution) != 1 {
				_ = node.Stop(context.Background())
				return fmt.Errorf("expected one execution health snapshot, got %d", len(health.Platform.Execution))
			}
			if got := health.Platform.Execution[0]; got.Venue != fixture.instrumentID.Venue || got.AccountID != fixture.execution.AccountID() || !got.Health.Connected {
				_ = node.Stop(context.Background())
				return fmt.Errorf("execution health snapshot mismatch: %#v", got)
			}
			if len(health.Strategies) != 1 || health.Strategies[0].ID != "live-health" || health.Strategies[0].State != kernel.ComponentStateRunning {
				_ = node.Stop(context.Background())
				return fmt.Errorf("strategy health snapshot mismatch: %#v", health.Strategies)
			}
			if err := node.Stop(context.Background()); err != nil {
				return err
			}
			health = node.Health()
			if len(health.Strategies) != 1 || health.Strategies[0].State != kernel.ComponentStateStopped {
				return fmt.Errorf("stopped strategy health mismatch: %#v", health.Strategies)
			}
			if len(health.Platform.Data) != 1 || len(health.Platform.Execution) != 1 {
				return fmt.Errorf("stopped client health snapshots missing: %#v", health.Platform)
			}
			return nil
		}},
		{id: "TC-LIVE08", name: "Reconnect policy retries data and execution recovery", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			node, err := fixture.NewNodeWithReconnectPolicy(live.RetryPolicy{MaxAttempts: 3})
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			defer node.Stop(context.Background())
			if err := node.Platform().SubscribeTicker(ctx, fixture.instrumentID); err != nil {
				return err
			}
			fixture.data.SetConnectErrors(errors.New("temporary data reconnect 1"), errors.New("temporary data reconnect 2"))
			fixture.execution.SetConnectErrors(errors.New("temporary execution reconnect 1"), errors.New("temporary execution reconnect 2"))
			fixture.data.BreakStream()
			fixture.execution.BreakStream()
			want := model.SubscribeMarketData{
				InstrumentID: fixture.instrumentID,
				Type:         model.MarketDataTypeTicker,
			}
			return liveNodeEventually(time.Second, func() bool {
				health := node.Health().Platform
				return fixture.data.ConnectCount() >= 4 &&
					fixture.data.SubscriptionCount(want) >= 2 &&
					fixture.execution.ConnectCount() >= 4 &&
					fixture.execution.ResubscribeCount() >= 1 &&
					fixture.execution.QueryAccountCount() >= 2 &&
					health.LastError == nil
			}, "reconnect policy did not recover data and execution streams")
		}},
		{id: "TC-LIVE09", name: "Fatal runtime exception triggers graceful shutdown", run: func() error {
			fixture := newLiveNodeTesterFixture(l.instrumentID())
			strat := &liveNodeRuntimeCaptureStrategy{id: "live-fatal"}
			node, err := fixture.NewNodeWithReconnectPolicy(live.RetryPolicy{MaxAttempts: 1}, strat)
			if err != nil {
				return err
			}
			if err := node.Start(ctx); err != nil {
				return err
			}
			fixture.data.SetConnectErrors(errors.New("fatal data reconnect"))
			fixture.data.BreakStream()
			return liveNodeEventually(time.Second, func() bool {
				health := node.Health()
				return health.State == kernel.ComponentStateStopped &&
					health.Platform.State == kernel.ComponentStateStopped &&
					strat.Stopped() &&
					!fixture.data.Connected() &&
					!fixture.execution.Connected() &&
					health.Platform.LastError != nil &&
					strings.Contains(health.Platform.LastError.Error(), "fatal data reconnect") &&
					strings.Contains(health.LastError, "fatal data reconnect")
			}, "fatal runtime exception did not gracefully stop live node")
		}},
	})
}

func (l *LiveNodeTester) instrumentID() model.InstrumentID {
	if l != nil && l.cfg.InstrumentID != (model.InstrumentID{}) {
		return l.cfg.InstrumentID
	}
	return model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
}

type liveNodeTesterFixture struct {
	instrumentID model.InstrumentID
	cache        *cache.Cache
	risk         *risk.Engine
	data         *liveNodeDataClient
	execution    *liveNodeExecutionClient
	rec          *liveNodeStartupRecorder
}

func newLiveNodeTesterFixture(instrumentID model.InstrumentID) *liveNodeTesterFixture {
	c := cache.New()
	return &liveNodeTesterFixture{
		instrumentID: instrumentID,
		cache:        c,
		risk:         risk.NewEngine(c, risk.Config{}),
		data:         newLiveNodeDataClient(instrumentID),
		execution:    newLiveNodeExecutionClient(instrumentID),
	}
}

func (f *liveNodeTesterFixture) RecordStartup() *liveNodeStartupRecorder {
	f.rec = &liveNodeStartupRecorder{}
	return f.rec
}

func (f *liveNodeTesterFixture) NewNode(strategies ...strategy.Strategy) (*live.Node, error) {
	return f.NewNodeWithReconnectPolicy(live.RetryPolicy{}, strategies...)
}

func (f *liveNodeTesterFixture) NewNodeWithReconnectPolicy(policy live.RetryPolicy, strategies ...strategy.Strategy) (*live.Node, error) {
	f.data.rec = f.rec
	f.data.provider.rec = f.rec
	f.execution.rec = f.rec
	builder := live.NewNodeBuilder().
		WithBus(bus.New()).
		WithCache(f.cache).
		WithRisk(f.risk).
		WithReconnectPolicy(policy).
		AddDataClient(f.data).
		AddExecutionClient(f.execution)
	for _, strategy := range strategies {
		builder.AddStrategy(strategy)
	}
	return builder.Build()
}

type liveNodeRuntimeCaptureStrategy struct {
	mu      sync.Mutex
	id      string
	runtime strategy.Runtime
	stopped bool
}

func (s *liveNodeRuntimeCaptureStrategy) ID() string { return s.id }

func (s *liveNodeRuntimeCaptureStrategy) OnStart(_ context.Context, rt strategy.Runtime) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime = rt
	return nil
}

func (s *liveNodeRuntimeCaptureStrategy) OnEvent(context.Context, bus.Envelope) error {
	return nil
}

func (s *liveNodeRuntimeCaptureStrategy) OnStop(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	return nil
}

func (s *liveNodeRuntimeCaptureStrategy) Runtime() strategy.Runtime {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runtime
}

func (s *liveNodeRuntimeCaptureStrategy) Stopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

type liveNodeTypedSubscriptionStrategy struct {
	mu           sync.Mutex
	instrumentID model.InstrumentID
	books        int
}

func (s *liveNodeTypedSubscriptionStrategy) OnStart(ctx context.Context, rt strategy.Runtime) error {
	return rt.SubscribeOrderBookDepth(ctx, s.instrumentID, 2)
}

func (s *liveNodeTypedSubscriptionStrategy) OnOrderBook(context.Context, model.OrderBook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books++
	return nil
}

func (s *liveNodeTypedSubscriptionStrategy) BookCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.books
}

type liveNodeStartupPhaseStrategy struct {
	node func() *live.Node
	rec  *liveNodeStartupRecorder
}

func (s *liveNodeStartupPhaseStrategy) ID() string { return "live-startup-phase" }

func (s *liveNodeStartupPhaseStrategy) OnStart(context.Context, strategy.Runtime) error {
	s.rec.Add("strategy-start")
	node := s.node()
	if node == nil {
		return fmt.Errorf("live node was not assigned before strategy start")
	}
	health := node.Health()
	if !health.Platform.Ready || health.Platform.State != kernel.ComponentStateRunning {
		return fmt.Errorf("platform not ready before strategy start: ready=%v state=%s", health.Platform.Ready, health.Platform.State)
	}
	return nil
}

func (s *liveNodeStartupPhaseStrategy) OnEvent(context.Context, bus.Envelope) error { return nil }
func (s *liveNodeStartupPhaseStrategy) OnStop(context.Context) error                { return nil }

type liveNodeStartupRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *liveNodeStartupRecorder) Add(event string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *liveNodeStartupRecorder) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type liveNodeDataClient struct {
	provider          *liveNodeProvider
	events            chan model.MarketEvent
	replacementEvents chan model.MarketEvent

	mu          sync.Mutex
	connected   bool
	connects    int
	connectErrs []error
	subs        []model.SubscribeMarketData
	rec         *liveNodeStartupRecorder
}

func newLiveNodeDataClient(instrumentID model.InstrumentID) *liveNodeDataClient {
	return &liveNodeDataClient{
		provider: newLiveNodeProvider(instrumentID),
		events:   make(chan model.MarketEvent, 16),
	}
}

func (c *liveNodeDataClient) Venue() model.Venue { return c.provider.inst.ID.Venue }
func (c *liveNodeDataClient) ClientID() string   { return "live-node-data" }
func (c *liveNodeDataClient) Instruments() venue.InstrumentProvider {
	return c.provider
}

func (c *liveNodeDataClient) Connect(context.Context) error {
	c.rec.Add("data-connect")
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connects++
	if len(c.connectErrs) > 0 {
		err := c.connectErrs[0]
		c.connectErrs = c.connectErrs[1:]
		c.connected = false
		return err
	}
	if c.replacementEvents != nil {
		c.events = c.replacementEvents
		c.replacementEvents = nil
	}
	c.connected = true
	return nil
}

func (c *liveNodeDataClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

func (c *liveNodeDataClient) Health() venue.DataHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return venue.DataHealth{Connected: c.connected, InstrumentReady: true}
}

func (c *liveNodeDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, nil
}

func (c *liveNodeDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, nil
}

func (c *liveNodeDataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = append(c.subs, sub)
	return nil
}

func (c *liveNodeDataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
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

func (c *liveNodeDataClient) Events() <-chan model.MarketEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events
}

func (c *liveNodeDataClient) EmitOrderBook(book model.OrderBook) {
	c.mu.Lock()
	events := c.events
	c.mu.Unlock()
	events <- model.MarketEvent{OrderBook: &book}
}

func (c *liveNodeDataClient) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *liveNodeDataClient) HasSubscription(want model.SubscribeMarketData) bool {
	return c.SubscriptionCount(want) > 0
}

func (c *liveNodeDataClient) SubscriptionCount(want model.SubscribeMarketData) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	var count int
	for _, sub := range c.subs {
		if sub == want {
			count++
		}
	}
	return count
}

func (c *liveNodeDataClient) ConnectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connects
}

func (c *liveNodeDataClient) SetConnectErrors(errs ...error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectErrs = append([]error(nil), errs...)
}

func (c *liveNodeDataClient) BreakStream() {
	c.mu.Lock()
	old := c.events
	c.replacementEvents = make(chan model.MarketEvent, 16)
	c.connected = false
	c.mu.Unlock()
	close(old)
}

type liveNodeProvider struct {
	inst model.Instrument
	rec  *liveNodeStartupRecorder
}

func newLiveNodeProvider(instrumentID model.InstrumentID) *liveNodeProvider {
	base, quote, settle, typ := liveNodeInstrumentParts(instrumentID)
	return &liveNodeProvider{inst: model.Instrument{
		ID:        instrumentID,
		RawSymbol: instrumentID.Symbol,
		Type:      typ,
		Base:      base,
		Quote:     quote,
		Settle:    settle,
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}}
}

func liveNodeInstrumentParts(instrumentID model.InstrumentID) (model.Currency, model.Currency, model.Currency, model.InstrumentType) {
	parts := strings.Split(instrumentID.Symbol, "-")
	base := model.Currency("BTC")
	quote := model.Currency("USDT")
	typ := model.InstrumentTypeSpot
	if len(parts) >= 2 {
		base = model.Currency(parts[0])
		quote = model.Currency(parts[1])
	}
	if len(parts) >= 3 {
		typ = model.InstrumentType(strings.ToLower(parts[2]))
	}
	var settle model.Currency
	switch typ {
	case model.InstrumentTypePerp, model.InstrumentTypeFuture, model.InstrumentTypeOption:
		settle = quote
	}
	return base, quote, settle, typ
}

func (p *liveNodeProvider) LoadAll(context.Context) error {
	p.rec.Add("data-load")
	return nil
}

func (p *liveNodeProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.inst, p.inst.ID == id
}

func (p *liveNodeProvider) List() []model.Instrument {
	return []model.Instrument{p.inst}
}

type liveNodeExecutionClient struct {
	instrumentID      model.InstrumentID
	events            chan model.ExecutionEvent
	replacementEvents chan model.ExecutionEvent

	mu           sync.Mutex
	connected    bool
	connects     int
	connectErrs  []error
	queries      int
	resubscribes int
	submits      []model.SubmitOrder
	nextID       int
	rec          *liveNodeStartupRecorder
}

func newLiveNodeExecutionClient(instrumentID model.InstrumentID) *liveNodeExecutionClient {
	return &liveNodeExecutionClient{
		instrumentID: instrumentID,
		events:       make(chan model.ExecutionEvent, 16),
	}
}

func (c *liveNodeExecutionClient) Venue() model.Venue         { return c.instrumentID.Venue }
func (c *liveNodeExecutionClient) AccountID() model.AccountID { return "acct-live" }

func (c *liveNodeExecutionClient) Connect(context.Context) error {
	c.rec.Add("exec-connect")
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connects++
	if len(c.connectErrs) > 0 {
		err := c.connectErrs[0]
		c.connectErrs = c.connectErrs[1:]
		c.connected = false
		return err
	}
	if c.replacementEvents != nil {
		c.events = c.replacementEvents
		c.replacementEvents = nil
	}
	c.connected = true
	return nil
}

func (c *liveNodeExecutionClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

func (c *liveNodeExecutionClient) Health() venue.ExecutionHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return venue.ExecutionHealth{Connected: c.connected, AccountReady: true}
}

func (c *liveNodeExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	c.rec.Add("exec-query-account")
	c.mu.Lock()
	c.queries++
	c.mu.Unlock()
	return model.AccountSnapshot{
		AccountID:    c.AccountID(),
		Venue:        c.Venue(),
		Type:         model.AccountTypeCash,
		BaseCurrency: "USDT",
		Timestamp:    time.Unix(1, 0),
	}, nil
}

func (c *liveNodeExecutionClient) SubmitOrder(_ context.Context, order model.SubmitOrder) (model.OrderStatusReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.submits = append(c.submits, order)
	c.nextID++
	return model.OrderStatusReport{
		AccountID:       order.AccountID,
		InstrumentID:    order.InstrumentID,
		OrderID:         model.OrderID(fmt.Sprintf("live-order-%d", c.nextID)),
		ClientOrderID:   order.ClientOrderID,
		Status:          model.OrderStatusAccepted,
		Side:            order.Side,
		Type:            order.Type,
		Quantity:        order.Quantity,
		LeavesQuantity:  order.Quantity,
		TimeInForce:     order.TimeInForce,
		LastUpdatedTime: time.Unix(int64(c.nextID), 0),
	}, nil
}

func (c *liveNodeExecutionClient) CancelOrder(_ context.Context, cancel model.CancelOrder) (model.OrderStatusReport, error) {
	return model.OrderStatusReport{
		AccountID:     cancel.AccountID,
		InstrumentID:  cancel.InstrumentID,
		OrderID:       cancel.OrderID,
		ClientOrderID: cancel.ClientOrderID,
		Status:        model.OrderStatusCanceled,
	}, nil
}

func (c *liveNodeExecutionClient) GenerateOrderStatusReports(context.Context, model.InstrumentID) ([]model.OrderStatusReport, error) {
	return nil, nil
}

func (c *liveNodeExecutionClient) ResubscribeExecution(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resubscribes++
	return nil
}

func (c *liveNodeExecutionClient) Events() <-chan model.ExecutionEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events
}

func (c *liveNodeExecutionClient) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *liveNodeExecutionClient) LastSubmit() (model.SubmitOrder, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.submits) == 0 {
		return model.SubmitOrder{}, false
	}
	return c.submits[len(c.submits)-1], true
}

func (c *liveNodeExecutionClient) ConnectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connects
}

func (c *liveNodeExecutionClient) QueryAccountCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.queries
}

func (c *liveNodeExecutionClient) ResubscribeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resubscribes
}

func (c *liveNodeExecutionClient) SetConnectErrors(errs ...error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectErrs = append([]error(nil), errs...)
}

func (c *liveNodeExecutionClient) BreakStream() {
	c.mu.Lock()
	old := c.events
	c.replacementEvents = make(chan model.ExecutionEvent, 16)
	c.connected = false
	c.mu.Unlock()
	close(old)
}

func liveNodeEventually(timeout time.Duration, predicate func() bool, message string) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	if predicate() {
		return nil
	}
	return fmt.Errorf("%s", message)
}

func liveNodeEventBefore(events []string, before string, after string) bool {
	beforeIndex := len(events)
	afterIndex := len(events)
	for i, event := range events {
		if event == before && beforeIndex == len(events) {
			beforeIndex = i
		}
		if event == after && afterIndex == len(events) {
			afterIndex = i
		}
	}
	return beforeIndex < afterIndex
}
