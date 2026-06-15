package testsuite

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/data"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type DataEngineTesterConfig struct {
	InstrumentID model.InstrumentID
}

type DataEngineTester struct {
	cfg DataEngineTesterConfig
}

func NewDataEngineTester(cfg DataEngineTesterConfig) *DataEngineTester {
	return &DataEngineTester{cfg: cfg}
}

func (d *DataEngineTester) Run(ctx context.Context, t *testing.T) ContractReport {
	t.Helper()
	return runContractCases(t, "data-engine", []contractCase{
		{id: "TC-DE01", name: "Engine replays idempotent subscriptions on restart", run: func() error {
			return d.runSubscriptionReplay(ctx)
		}},
		{id: "TC-DE02", name: "Engine forwards and caches live stream events", run: func() error {
			return d.runStreamForwarding(ctx)
		}},
		{id: "TC-DE03", name: "Catalog requests preserve correlation metadata", run: func() error {
			return d.runCatalogCorrelation(ctx)
		}},
		{id: "TC-DE04", name: "Trade ticks aggregate into bars", run: func() error {
			return d.runTradeAggregation(ctx)
		}},
		{id: "TC-DE05", name: "Health reports clients subscriptions events and errors", run: func() error {
			return d.runHealthMetrics(ctx)
		}},
		{id: "TC-DE06", name: "Engine reconnects closed streams and replays subscriptions", run: func() error {
			return d.runReconnectReplay(ctx)
		}},
		{id: "TC-DE07", name: "Health marks stale clients", run: func() error {
			return d.runStaleHealth(ctx)
		}},
	})
}

func (d *DataEngineTester) runSubscriptionReplay(ctx context.Context) error {
	instrumentID := d.instrumentID()
	client := newDataEngineFakeClient(instrumentID)
	engine := data.NewEngine(data.Config{Cache: cache.New()})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	sub := model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeOrderBook,
		Depth:        5,
	}
	if err := engine.Subscribe(ctx, sub); err != nil {
		return err
	}
	if err := engine.Subscribe(ctx, sub); err != nil {
		return err
	}
	if got := client.SubscriptionCount(sub); got != 0 {
		return fmt.Errorf("expected deferred pre-start subscription, got %d calls", got)
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	if got := client.SubscriptionCount(sub); got != 1 {
		return fmt.Errorf("expected one subscription after start, got %d", got)
	}
	if got := client.ConnectCount(); got != 1 {
		return fmt.Errorf("expected one connect after start, got %d", got)
	}
	if err := engine.Stop(ctx); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	if got := client.SubscriptionCount(sub); got != 2 {
		return fmt.Errorf("expected subscription replay after restart, got %d", got)
	}
	if got := client.ConnectCount(); got != 2 {
		return fmt.Errorf("expected two connects after restart, got %d", got)
	}
	if err := engine.Unsubscribe(ctx, sub); err != nil {
		return err
	}
	if got := client.UnsubscriptionCount(sub); got != 1 {
		return fmt.Errorf("expected one live unsubscribe, got %d", got)
	}
	return nil
}

func (d *DataEngineTester) runStreamForwarding(ctx context.Context) error {
	instrumentID := d.instrumentID()
	client := newDataEngineFakeClient(instrumentID)
	c := cache.New()
	engine := data.NewEngine(data.Config{Cache: c})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	client.Emit(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: instrumentID,
		Last:         decimal.RequireFromString("100"),
		Timestamp:    time.Unix(5, 0),
	}})
	if _, err := dataEngineWaitEvent(ctx, engine.Events(), func(event model.MarketEvent) (model.MarketEvent, bool) {
		return event, event.Ticker != nil
	}); err != nil {
		return err
	}
	ticker, ok := c.Ticker(instrumentID)
	if !ok {
		return fmt.Errorf("ticker was not cached")
	}
	if !ticker.Last.Equal(decimal.RequireFromString("100")) {
		return fmt.Errorf("cached ticker last mismatch: %s", ticker.Last)
	}
	return nil
}

func (d *DataEngineTester) runCatalogCorrelation(ctx context.Context) error {
	instrumentID := d.instrumentID()
	catalog := data.NewMemoryCatalog(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: instrumentID,
		BidPrice:     decimal.RequireFromString("100"),
		AskPrice:     decimal.RequireFromString("101"),
		BidSize:      decimal.RequireFromString("1"),
		AskSize:      decimal.RequireFromString("1"),
		Timestamp:    time.Unix(10, 0),
	}})
	engine := data.NewEngine(data.Config{Catalog: catalog})
	response, err := engine.Request(ctx, model.DataRequest{
		Metadata:     model.CommandMetadata{CommandID: "tc-de03-request"},
		RequestID:    "tc-de03",
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeQuoteTick,
		Start:        time.Unix(1, 0),
		End:          time.Unix(20, 0),
		Limit:        10,
	})
	if err != nil {
		return err
	}
	if response.Metadata.CorrelationID != model.CorrelationID("tc-de03") {
		return fmt.Errorf("correlation id mismatch: %s", response.Metadata.CorrelationID)
	}
	if response.Metadata.CommandID != model.CommandID("tc-de03-request") {
		return fmt.Errorf("command id mismatch: %s", response.Metadata.CommandID)
	}
	if len(response.Events) != 1 || response.Events[0].Quote == nil {
		return fmt.Errorf("expected one quote response event, got %d", len(response.Events))
	}
	return nil
}

func (d *DataEngineTester) runTradeAggregation(ctx context.Context) error {
	instrumentID := d.instrumentID()
	client := newDataEngineFakeClient(instrumentID)
	barType := model.NewTimeBarType(instrumentID, time.Minute)
	engine := data.NewEngine(data.Config{Cache: cache.New()})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.AddBarAggregation(barType); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	client.Emit(model.MarketEvent{Trade: dataEngineTrade(instrumentID, "tc-de04-1", "100", "1", time.Unix(0, 0))})
	client.Emit(model.MarketEvent{Trade: dataEngineTrade(instrumentID, "tc-de04-2", "102", "2", time.Unix(30, 0))})
	client.Emit(model.MarketEvent{Trade: dataEngineTrade(instrumentID, "tc-de04-3", "101", "1", time.Unix(60, 0))})
	event, err := dataEngineWaitEvent(ctx, engine.Events(), func(event model.MarketEvent) (model.MarketEvent, bool) {
		return event, event.Bar != nil
	})
	if err != nil {
		return err
	}
	bar := event.Bar
	if bar.BarType.Canonical() != barType.Canonical() {
		return fmt.Errorf("bar type mismatch: %s", bar.BarType.Canonical())
	}
	if !bar.Open.Equal(decimal.RequireFromString("100")) ||
		!bar.High.Equal(decimal.RequireFromString("102")) ||
		!bar.Low.Equal(decimal.RequireFromString("100")) ||
		!bar.Close.Equal(decimal.RequireFromString("102")) ||
		!bar.Volume.Equal(decimal.RequireFromString("3")) {
		return fmt.Errorf("bar OHLCV mismatch: open=%s high=%s low=%s close=%s volume=%s", bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
	}
	return nil
}

func (d *DataEngineTester) runHealthMetrics(ctx context.Context) error {
	instrumentID := d.instrumentID()
	client := newDataEngineFakeClient(instrumentID)
	engine := data.NewEngine(data.Config{Cache: cache.New()})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	sub := model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTicker,
	}
	if err := engine.Subscribe(ctx, sub); err != nil {
		return err
	}
	client.Emit(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: instrumentID,
		Last:         decimal.RequireFromString("100"),
		Timestamp:    time.Unix(5, 0),
	}})
	if _, err := dataEngineWaitEvent(ctx, engine.Events(), func(event model.MarketEvent) (model.MarketEvent, bool) {
		return event, event.Ticker != nil
	}); err != nil {
		return err
	}
	_, _ = engine.Request(ctx, model.DataRequest{
		RequestID:    "tc-de05-unsupported",
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeBar,
		BarType:      model.NewTimeBarType(instrumentID, time.Minute),
	})
	health := engine.Health()
	if !health.Running {
		return fmt.Errorf("expected engine to be running")
	}
	if health.Clients != 1 || health.Subscriptions != 1 {
		return fmt.Errorf("health topology mismatch: clients=%d subscriptions=%d", health.Clients, health.Subscriptions)
	}
	if health.Events < 1 || health.LastEventTime.IsZero() {
		return fmt.Errorf("health event counters were not updated: events=%d last=%s", health.Events, health.LastEventTime)
	}
	if health.LastError == nil {
		return fmt.Errorf("expected unsupported request to be recorded as last error")
	}
	if len(health.ClientsHealth) != 1 || !health.ClientsHealth[0].Health.Connected {
		return fmt.Errorf("client health was not surfaced")
	}
	return nil
}

func (d *DataEngineTester) runReconnectReplay(ctx context.Context) error {
	instrumentID := d.instrumentID()
	client := newDataEngineFakeClient(instrumentID)
	engine := data.NewEngine(data.Config{
		Cache:           cache.New(),
		ReconnectPolicy: data.RetryPolicy{MaxAttempts: 2},
	})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	sub := model.SubscribeMarketData{
		InstrumentID: instrumentID,
		Type:         model.MarketDataTypeTicker,
	}
	if err := engine.Subscribe(ctx, sub); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return err
	}
	defer engine.Stop(ctx)
	client.BreakStream()
	if err := dataEngineWaitUntil(ctx, time.Second, func() bool {
		return client.ConnectCount() >= 2 && client.SubscriptionCount(sub) >= 2
	}); err != nil {
		return err
	}
	client.Emit(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: instrumentID,
		Last:         decimal.RequireFromString("103"),
		Timestamp:    time.Unix(9, 0),
	}})
	event, err := dataEngineWaitEvent(ctx, engine.Events(), func(event model.MarketEvent) (model.MarketEvent, bool) {
		return event, event.Ticker != nil
	})
	if err != nil {
		return err
	}
	if !event.Ticker.Last.Equal(decimal.RequireFromString("103")) {
		return fmt.Errorf("recovered ticker mismatch: %s", event.Ticker.Last)
	}
	return nil
}

func (d *DataEngineTester) runStaleHealth(ctx context.Context) error {
	client := newDataEngineFakeClient(d.instrumentID())
	client.SetHealth(venue.DataHealth{
		Connected:       true,
		InstrumentReady: true,
		LastEventTime:   time.Unix(1, 0),
	})
	engine := data.NewEngine(data.Config{Cache: cache.New(), StaleAfter: time.Nanosecond})
	if err := engine.AddClient(client); err != nil {
		return err
	}
	health := engine.Health()
	if health.StaleClients != 1 {
		return fmt.Errorf("expected one stale client, got %d", health.StaleClients)
	}
	if len(health.ClientsHealth) != 1 || !health.ClientsHealth[0].Stale {
		return fmt.Errorf("expected stale client health detail")
	}
	return nil
}

func (d *DataEngineTester) instrumentID() model.InstrumentID {
	if d.cfg.InstrumentID != (model.InstrumentID{}) {
		return d.cfg.InstrumentID
	}
	return model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
}

func dataEngineTrade(instrumentID model.InstrumentID, id model.TradeID, price string, size string, ts time.Time) *model.TradeTick {
	return &model.TradeTick{
		InstrumentID:  instrumentID,
		Price:         decimal.RequireFromString(price),
		Size:          decimal.RequireFromString(size),
		AggressorSide: model.AggressorSideBuyer,
		TradeID:       id,
		Timestamp:     ts,
	}
}

func dataEngineWaitEvent(ctx context.Context, events <-chan model.MarketEvent, match func(model.MarketEvent) (model.MarketEvent, bool)) (model.MarketEvent, error) {
	waitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for {
		select {
		case <-waitCtx.Done():
			return model.MarketEvent{}, fmt.Errorf("timed out waiting for matching data engine event")
		case event := <-events:
			if matched, ok := match(event); ok {
				return matched, nil
			}
		}
	}
}

func dataEngineWaitUntil(ctx context.Context, timeout time.Duration, ready func() bool) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ready() {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for data engine condition")
		case <-ticker.C:
		}
	}
}

type dataEngineFakeClient struct {
	mu           sync.Mutex
	instrumentID model.InstrumentID
	provider     *dataEngineFakeProvider
	events       chan model.MarketEvent
	nextEvents   chan model.MarketEvent
	health       venue.DataHealth
	connects     int
	connected    bool
	subs         map[string]int
	unsubs       map[string]int
}

func newDataEngineFakeClient(instrumentID model.InstrumentID) *dataEngineFakeClient {
	return &dataEngineFakeClient{
		instrumentID: instrumentID,
		provider:     newDataEngineFakeProvider(instrumentID),
		events:       make(chan model.MarketEvent, 16),
		health:       venue.DataHealth{Connected: true, InstrumentReady: true},
		subs:         make(map[string]int),
		unsubs:       make(map[string]int),
	}
}

func (c *dataEngineFakeClient) Venue() model.Venue                    { return c.instrumentID.Venue }
func (c *dataEngineFakeClient) ClientID() string                      { return "testsuite-data-engine-fake" }
func (c *dataEngineFakeClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *dataEngineFakeClient) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connects++
	c.connected = true
	if c.nextEvents != nil {
		c.events = c.nextEvents
		c.nextEvents = nil
	}
	return nil
}
func (c *dataEngineFakeClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}
func (c *dataEngineFakeClient) Health() venue.DataHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	health := c.health
	health.Connected = c.connected || health.Connected
	return health
}
func (c *dataEngineFakeClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, model.ErrNotSupported
}
func (c *dataEngineFakeClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, model.ErrNotSupported
}
func (c *dataEngineFakeClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs[sub.Key()]++
	return nil
}
func (c *dataEngineFakeClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unsubs[sub.Key()]++
	return nil
}
func (c *dataEngineFakeClient) Events() <-chan model.MarketEvent { return c.events }
func (c *dataEngineFakeClient) Emit(event model.MarketEvent)     { c.events <- event }
func (c *dataEngineFakeClient) BreakStream() {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.events
	c.nextEvents = make(chan model.MarketEvent, 16)
	close(old)
}
func (c *dataEngineFakeClient) SetHealth(health venue.DataHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health = health
}
func (c *dataEngineFakeClient) SubscriptionCount(sub model.SubscribeMarketData) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subs[sub.Key()]
}
func (c *dataEngineFakeClient) UnsubscriptionCount(sub model.SubscribeMarketData) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unsubs[sub.Key()]
}
func (c *dataEngineFakeClient) ConnectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connects
}

type dataEngineFakeProvider struct {
	inst model.Instrument
}

func newDataEngineFakeProvider(instrumentID model.InstrumentID) *dataEngineFakeProvider {
	return &dataEngineFakeProvider{inst: model.Instrument{
		ID:        instrumentID,
		RawSymbol: "BTCUSDT",
		Type:      model.InstrumentTypeSpot,
		Base:      "BTC",
		Quote:     "USDT",
		PriceTick: decimal.RequireFromString("0.01"),
		SizeTick:  decimal.RequireFromString("0.0001"),
		Status:    model.InstrumentStatusTrading,
	}}
}

func (p *dataEngineFakeProvider) LoadAll(context.Context) error { return nil }
func (p *dataEngineFakeProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.inst, p.inst.ID == id
}
func (p *dataEngineFakeProvider) List() []model.Instrument {
	return []model.Instrument{p.inst}
}

var _ venue.DataClient = (*dataEngineFakeClient)(nil)
var _ venue.StreamingDataClient = (*dataEngineFakeClient)(nil)
