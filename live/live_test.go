package live

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/platform"
	"github.com/QuantProcessing/exchanges/strategy"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestRunnerStartsStrategiesBeforePlatformStartupEvents(t *testing.T) {
	b := bus.New()
	node := platform.NewNode(platform.Config{Bus: b, Cache: cache.New()})
	data := newLiveDataClient()
	exec := newLiveExecutionClient()
	require.NoError(t, node.AddDataClient(data))
	require.NoError(t, node.AddExecutionClient(exec))
	rec := &recordingStrategy{id: "live"}
	runner := NewRunner(Config{Node: node, Bus: b, Strategies: []strategy.Strategy{rec}})

	require.NoError(t, runner.Start(context.Background()))
	require.Eventually(t, func() bool {
		return rec.hasTopic(strategy.TopicExecution)
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, runner.Stop(context.Background()))
	require.True(t, rec.isStopped())
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
	gotNode, ok := rec.runtime.(*platform.Node)
	require.True(t, ok)
	require.Same(t, node, gotNode)

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

type recordingStrategy struct {
	mu      sync.Mutex
	id      string
	events  []bus.Envelope
	stopped bool
}

func (s *recordingStrategy) ID() string                                      { return s.id }
func (s *recordingStrategy) OnStart(context.Context, strategy.Runtime) error { return nil }
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
