package data

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestEngineSubscribesIdempotentlyAndReplaysAfterRestart(t *testing.T) {
	client := newEngineFakeDataClient(engineTestInstrumentID())
	engine := NewEngine(Config{Cache: cache.New()})
	require.NoError(t, engine.AddClient(client))
	sub := model.SubscribeMarketData{
		InstrumentID: engineTestInstrumentID(),
		Type:         model.MarketDataTypeOrderBook,
		Depth:        5,
	}

	require.NoError(t, engine.Subscribe(context.Background(), sub))
	require.NoError(t, engine.Subscribe(context.Background(), sub))
	require.Equal(t, 0, client.SubscriptionCount(sub))

	require.NoError(t, engine.Start(context.Background()))
	require.Equal(t, 1, client.SubscriptionCount(sub))
	require.Equal(t, 1, client.ConnectCount())
	require.NoError(t, engine.Stop(context.Background()))

	require.NoError(t, engine.Start(context.Background()))
	require.Equal(t, 2, client.SubscriptionCount(sub))
	require.Equal(t, 2, client.ConnectCount())
	require.NoError(t, engine.Unsubscribe(context.Background(), sub))
	require.Equal(t, 1, client.UnsubscriptionCount(sub))
	require.NoError(t, engine.Stop(context.Background()))
}

func TestEngineForwardsCachesAndAggregatesTradeBars(t *testing.T) {
	client := newEngineFakeDataClient(engineTestInstrumentID())
	c := cache.New()
	b := bus.New()
	barType := model.NewTimeBarType(engineTestInstrumentID(), time.Minute)
	engine := NewEngine(Config{Bus: b, Cache: c})
	require.NoError(t, engine.AddClient(client))
	require.NoError(t, engine.AddBarAggregation(barType))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())

	client.Emit(model.MarketEvent{Trade: engineTestTrade("trade-1", "100", "1", time.Unix(0, 0))})
	client.Emit(model.MarketEvent{Trade: engineTestTrade("trade-2", "102", "2", time.Unix(30, 0))})
	client.Emit(model.MarketEvent{Trade: engineTestTrade("trade-3", "101", "1", time.Unix(60, 0))})

	bar := requireEngineEvent(t, engine.Events(), func(event model.MarketEvent) (*model.Bar, bool) {
		if event.Bar == nil {
			return nil, false
		}
		return event.Bar, true
	})
	require.Equal(t, barType.Canonical(), bar.BarType.Canonical())
	require.Equal(t, decimal.RequireFromString("100"), bar.Open)
	require.Equal(t, decimal.RequireFromString("102"), bar.High)
	require.Equal(t, decimal.RequireFromString("100"), bar.Low)
	require.Equal(t, decimal.RequireFromString("102"), bar.Close)
	require.Equal(t, decimal.RequireFromString("3"), bar.Volume)

	cachedTrade, ok := c.TradeTick(engineTestInstrumentID())
	require.True(t, ok)
	require.Equal(t, model.TradeID("trade-3"), cachedTrade.TradeID)
	cachedBar, ok := c.Bar(barType)
	require.True(t, ok)
	require.Equal(t, bar.Close, cachedBar.Close)
	require.Equal(t, int64(3), engine.Health().Events)
	require.Equal(t, int64(1), engine.Health().AggregatedBars)
}

func TestEngineRequestUsesCatalogAndCorrelationID(t *testing.T) {
	instID := engineTestInstrumentID()
	catalog := NewMemoryCatalog(
		model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: instID,
			BidPrice:     decimal.RequireFromString("100"),
			AskPrice:     decimal.RequireFromString("101"),
			BidSize:      decimal.RequireFromString("1"),
			AskSize:      decimal.RequireFromString("1"),
			Timestamp:    time.Unix(10, 0),
		}},
	)
	engine := NewEngine(Config{Catalog: catalog})

	response, err := engine.Request(context.Background(), model.DataRequest{
		Metadata:     model.CommandMetadata{CommandID: "request-quotes"},
		RequestID:    "quotes-1",
		InstrumentID: instID,
		Type:         model.MarketDataTypeQuoteTick,
		Start:        time.Unix(1, 0),
		End:          time.Unix(20, 0),
		Limit:        10,
	})
	require.NoError(t, err)
	require.Equal(t, model.DataRequestID("quotes-1"), response.RequestID)
	require.Equal(t, model.CorrelationID("quotes-1"), response.Metadata.CorrelationID)
	require.Equal(t, model.CommandID("request-quotes"), response.Metadata.CommandID)
	require.True(t, response.IsFinal)
	require.Len(t, response.Events, 1)
	require.NotNil(t, response.Events[0].Quote)
	require.Equal(t, int64(1), engine.Health().Requests)
}

func TestEngineRequestUsesCatalogForFundingRates(t *testing.T) {
	instID := model.MustInstrumentID("BTC-USDT-PERP.BINANCE")
	funding := model.FundingRate{
		InstrumentID:    instID,
		Rate:            decimal.RequireFromString("0.0003"),
		NextFundingTime: time.Unix(800, 0),
		FundingInterval: 8 * time.Hour,
		Timestamp:       time.Unix(700, 0),
	}
	engine := NewEngine(Config{Catalog: NewMemoryCatalog(model.MarketEvent{FundingRate: &funding})})

	response, err := engine.Request(context.Background(), model.DataRequest{
		Metadata:     model.CommandMetadata{CommandID: "request-funding"},
		RequestID:    "funding-1",
		InstrumentID: instID,
		Type:         model.MarketDataTypeFundingRate,
		Start:        time.Unix(600, 0),
		End:          time.Unix(900, 0),
		Limit:        1,
	})
	require.NoError(t, err)
	require.Equal(t, model.MarketDataTypeFundingRate, response.Type)
	require.Len(t, response.Events, 1)
	require.Equal(t, funding, *response.Events[0].FundingRate)
}

func TestEngineHealthTracksClientsSubscriptionsAndLastError(t *testing.T) {
	client := newEngineFakeDataClient(engineTestInstrumentID())
	engine := NewEngine(Config{Cache: cache.New()})
	require.NoError(t, engine.AddClient(client))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())
	sub := model.SubscribeMarketData{
		InstrumentID: engineTestInstrumentID(),
		Type:         model.MarketDataTypeTicker,
	}
	require.NoError(t, engine.Subscribe(context.Background(), sub))
	client.Emit(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: engineTestInstrumentID(),
		Last:         decimal.RequireFromString("100"),
		Timestamp:    time.Unix(5, 0),
	}})
	requireEngineEvent(t, engine.Events(), func(event model.MarketEvent) (*model.Ticker, bool) {
		if event.Ticker == nil {
			return nil, false
		}
		return event.Ticker, true
	})

	_, err := engine.Request(context.Background(), model.DataRequest{
		RequestID:    "unsupported-bars",
		InstrumentID: engineTestInstrumentID(),
		Type:         model.MarketDataTypeBar,
		BarType:      model.NewTimeBarType(engineTestInstrumentID(), time.Minute),
	})
	require.Error(t, err)

	health := engine.Health()
	require.True(t, health.Running)
	require.Equal(t, 1, health.Clients)
	require.Equal(t, 1, health.Subscriptions)
	require.Equal(t, int64(1), health.Events)
	require.NotNil(t, health.LastError)
	require.False(t, health.LastEventTime.IsZero())
}

func TestEngineReconnectsClosedStreamAndReplaysSubscriptions(t *testing.T) {
	client := newEngineFakeDataClient(engineTestInstrumentID())
	engine := NewEngine(Config{
		Cache:           cache.New(),
		ReconnectPolicy: RetryPolicy{MaxAttempts: 2},
	})
	require.NoError(t, engine.AddClient(client))
	sub := model.SubscribeMarketData{
		InstrumentID: engineTestInstrumentID(),
		Type:         model.MarketDataTypeTicker,
	}
	require.NoError(t, engine.Subscribe(context.Background(), sub))
	require.NoError(t, engine.Start(context.Background()))
	defer engine.Stop(context.Background())
	require.Equal(t, 1, client.SubscriptionCount(sub))

	client.BreakStream()
	require.Eventually(t, func() bool {
		return client.ConnectCount() >= 2 && client.SubscriptionCount(sub) >= 2
	}, time.Second, 10*time.Millisecond)

	client.Emit(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: engineTestInstrumentID(),
		Last:         decimal.RequireFromString("103"),
		Timestamp:    time.Unix(9, 0),
	}})
	ticker := requireEngineEvent(t, engine.Events(), func(event model.MarketEvent) (*model.Ticker, bool) {
		if event.Ticker == nil {
			return nil, false
		}
		return event.Ticker, true
	})
	require.Equal(t, decimal.RequireFromString("103"), ticker.Last)
}

func TestEngineHealthMarksStaleClients(t *testing.T) {
	client := newEngineFakeDataClient(engineTestInstrumentID())
	client.SetHealth(venue.DataHealth{
		Connected:       true,
		InstrumentReady: true,
		LastEventTime:   time.Unix(1, 0),
	})
	engine := NewEngine(Config{Cache: cache.New(), StaleAfter: time.Nanosecond})
	require.NoError(t, engine.AddClient(client))

	health := engine.Health()
	require.Equal(t, 1, health.StaleClients)
	require.Len(t, health.ClientsHealth, 1)
	require.True(t, health.ClientsHealth[0].Stale)
}

func requireEngineEvent[T any](t *testing.T, events <-chan model.MarketEvent, match func(model.MarketEvent) (*T, bool)) *T {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-events:
			if value, ok := match(event); ok {
				return value
			}
		case <-deadline:
			t.Fatal("timed out waiting for matching engine event")
		}
	}
}

func engineTestInstrumentID() model.InstrumentID {
	return model.MustInstrumentID("BTC-USDT-SPOT.BINANCE")
}

func engineTestTrade(id model.TradeID, price string, size string, ts time.Time) *model.TradeTick {
	return &model.TradeTick{
		InstrumentID:  engineTestInstrumentID(),
		Price:         decimal.RequireFromString(price),
		Size:          decimal.RequireFromString(size),
		AggressorSide: model.AggressorSideBuyer,
		TradeID:       id,
		Timestamp:     ts,
	}
}

type engineFakeDataClient struct {
	mu           sync.Mutex
	instrumentID model.InstrumentID
	provider     *engineFakeProvider
	events       chan model.MarketEvent
	nextEvents   chan model.MarketEvent
	health       venue.DataHealth
	connects     int
	subs         map[string]int
	unsubs       map[string]int
}

func newEngineFakeDataClient(instrumentID model.InstrumentID) *engineFakeDataClient {
	return &engineFakeDataClient{
		instrumentID: instrumentID,
		provider:     newEngineFakeProvider(instrumentID),
		events:       make(chan model.MarketEvent, 16),
		health:       venue.DataHealth{Connected: true, InstrumentReady: true},
		subs:         make(map[string]int),
		unsubs:       make(map[string]int),
	}
}

func (c *engineFakeDataClient) Venue() model.Venue                    { return c.instrumentID.Venue }
func (c *engineFakeDataClient) ClientID() string                      { return "engine-fake-data" }
func (c *engineFakeDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *engineFakeDataClient) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connects++
	if c.nextEvents != nil {
		c.events = c.nextEvents
		c.nextEvents = nil
	}
	return nil
}
func (c *engineFakeDataClient) Disconnect(context.Context) error { return nil }
func (c *engineFakeDataClient) Health() venue.DataHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.health
}
func (c *engineFakeDataClient) FetchTicker(context.Context, model.InstrumentID) (model.Ticker, error) {
	return model.Ticker{}, model.ErrNotSupported
}
func (c *engineFakeDataClient) FetchOrderBook(context.Context, model.InstrumentID, int) (model.OrderBook, error) {
	return model.OrderBook{}, model.ErrNotSupported
}
func (c *engineFakeDataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs[sub.Key()]++
	return nil
}
func (c *engineFakeDataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unsubs[sub.Key()]++
	return nil
}
func (c *engineFakeDataClient) Events() <-chan model.MarketEvent { return c.events }
func (c *engineFakeDataClient) Emit(event model.MarketEvent)     { c.events <- event }
func (c *engineFakeDataClient) BreakStream() {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.events
	c.nextEvents = make(chan model.MarketEvent, 16)
	close(old)
}
func (c *engineFakeDataClient) SetHealth(health venue.DataHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health = health
}
func (c *engineFakeDataClient) SubscriptionCount(sub model.SubscribeMarketData) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subs[sub.Key()]
}
func (c *engineFakeDataClient) UnsubscriptionCount(sub model.SubscribeMarketData) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unsubs[sub.Key()]
}
func (c *engineFakeDataClient) ConnectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connects
}

type engineFakeProvider struct {
	inst model.Instrument
}

func newEngineFakeProvider(instrumentID model.InstrumentID) *engineFakeProvider {
	return &engineFakeProvider{inst: model.Instrument{
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

func (p *engineFakeProvider) LoadAll(context.Context) error { return nil }
func (p *engineFakeProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	return p.inst, p.inst.ID == id
}
func (p *engineFakeProvider) List() []model.Instrument {
	return []model.Instrument{p.inst}
}

func TestEngineFakeDataClientImplementsVenueInterfaces(t *testing.T) {
	var _ venue.DataClient = (*engineFakeDataClient)(nil)
	var _ venue.StreamingDataClient = (*engineFakeDataClient)(nil)
	require.NotNil(t, fmt.Sprintf("%T", newEngineFakeDataClient(engineTestInstrumentID())))
}
