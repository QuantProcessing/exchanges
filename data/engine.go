package data

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/bus"
	"github.com/QuantProcessing/exchanges/cache"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

type Config struct {
	Bus             *bus.Bus
	Cache           *cache.Cache
	Catalog         Catalog
	ReconnectPolicy RetryPolicy
	StaleAfter      time.Duration
}

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

type Health struct {
	Running        bool
	Clients        int
	Subscriptions  int
	Events         int64
	Requests       int64
	AggregatedBars int64
	StaleClients   int
	LastEventTime  time.Time
	LastError      error
	ClientsHealth  []ClientHealth
}

type ClientHealth struct {
	Venue    model.Venue
	ClientID string
	Health   venue.DataHealth
	Stale    bool
}

type Engine struct {
	mu            sync.RWMutex
	bus           *bus.Bus
	cache         *cache.Cache
	catalog       Catalog
	clients       map[model.Venue]venue.DataClient
	streaming     map[model.Venue]venue.StreamingDataClient
	subscriptions map[string]model.SubscribeMarketData
	aggregators   map[string]*BarAggregator
	events        chan model.MarketEvent
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	reconnect     RetryPolicy
	staleAfter    time.Duration
	running       bool
	eventCount    int64
	requestCount  int64
	barCount      int64
	lastEventTime time.Time
	lastError     error
}

func NewEngine(cfg Config) *Engine {
	b := cfg.Bus
	if b == nil {
		b = bus.New()
	}
	c := cfg.Cache
	if c == nil {
		c = cache.New()
	}
	return &Engine{
		bus:           b,
		cache:         c,
		catalog:       cfg.Catalog,
		clients:       make(map[model.Venue]venue.DataClient),
		streaming:     make(map[model.Venue]venue.StreamingDataClient),
		subscriptions: make(map[string]model.SubscribeMarketData),
		aggregators:   make(map[string]*BarAggregator),
		events:        make(chan model.MarketEvent, 256),
		reconnect:     cfg.ReconnectPolicy,
		staleAfter:    cfg.StaleAfter,
	}
}

func (e *Engine) AddClient(client venue.DataClient) error {
	if client == nil {
		return fmt.Errorf("%w: data client is required", model.ErrInvalidMarketData)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clients[client.Venue()] = client
	if streaming, ok := client.(venue.StreamingDataClient); ok {
		e.streaming[client.Venue()] = streaming
	}
	return nil
}

func (e *Engine) AddBarAggregation(barType model.BarType) error {
	aggregator, err := NewBarAggregator(barType)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aggregators[barType.Canonical().String()] = aggregator
	return nil
}

func (e *Engine) Events() <-chan model.MarketEvent {
	if e == nil {
		return nil
	}
	return e.events
}

func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.running = true
	clients := make([]venue.DataClient, 0, len(e.clients))
	for _, client := range e.clients {
		clients = append(clients, client)
	}
	e.mu.Unlock()

	connected := make([]venue.DataClient, 0, len(clients))
	for _, client := range clients {
		if provider := client.Instruments(); provider != nil {
			if err := provider.LoadAll(ctx); err != nil {
				return e.abortStart(ctx, cancel, connected, err)
			}
			for _, instrument := range provider.List() {
				if err := e.cache.PutInstrument(instrument); err != nil {
					return e.abortStart(ctx, cancel, connected, err)
				}
			}
		}
		if err := client.Connect(ctx); err != nil {
			return e.abortStart(ctx, cancel, connected, err)
		}
		connected = append(connected, client)
		if streaming, ok := client.(venue.StreamingDataClient); ok {
			if err := e.applySubscriptions(ctx, client.Venue(), streaming); err != nil {
				return e.abortStart(ctx, cancel, connected, err)
			}
			e.wg.Add(1)
			go e.forward(runCtx, client.Venue(), streaming)
		}
	}
	return nil
}

func (e *Engine) abortStart(ctx context.Context, cancel context.CancelFunc, connected []venue.DataClient, cause error) error {
	if cancel != nil {
		cancel()
	}
	e.wg.Wait()
	var err error
	err = errors.Join(err, cause)
	for i := len(connected) - 1; i >= 0; i-- {
		err = errors.Join(err, connected[i].Disconnect(ctx))
	}
	e.mu.Lock()
	e.cancel = nil
	e.running = false
	e.lastError = err
	e.mu.Unlock()
	return err
}

func (e *Engine) Stop(ctx context.Context) error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return nil
	}
	cancel := e.cancel
	e.cancel = nil
	e.running = false
	clients := make([]venue.DataClient, 0, len(e.clients))
	for _, client := range e.clients {
		clients = append(clients, client)
	}
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	e.wg.Wait()
	var stopErr error
	for _, client := range clients {
		stopErr = errors.Join(stopErr, client.Disconnect(ctx))
	}
	if stopErr != nil {
		e.recordError(stopErr)
	}
	return stopErr
}

func (e *Engine) Subscribe(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		e.recordError(err)
		return err
	}
	e.mu.Lock()
	key := sub.Key()
	if _, exists := e.subscriptions[key]; exists {
		e.mu.Unlock()
		return nil
	}
	streaming := e.streaming[sub.InstrumentID.Venue]
	if streaming == nil {
		e.mu.Unlock()
		err := fmt.Errorf("%w: no streaming data client for venue %s", model.ErrNotSupported, sub.InstrumentID.Venue)
		e.recordError(err)
		return err
	}
	e.subscriptions[key] = sub
	running := e.running
	e.mu.Unlock()
	if running {
		if err := streaming.SubscribeMarketData(ctx, sub); err != nil {
			e.mu.Lock()
			delete(e.subscriptions, key)
			e.mu.Unlock()
			e.recordError(err)
			return err
		}
	}
	return nil
}

func (e *Engine) Unsubscribe(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		e.recordError(err)
		return err
	}
	e.mu.Lock()
	key := sub.Key()
	if _, exists := e.subscriptions[key]; !exists {
		e.mu.Unlock()
		return nil
	}
	delete(e.subscriptions, key)
	running := e.running
	streaming := e.streaming[sub.InstrumentID.Venue]
	e.mu.Unlock()
	if running && streaming != nil {
		if err := streaming.UnsubscribeMarketData(ctx, sub); err != nil {
			e.recordError(err)
			return err
		}
	}
	return nil
}

func (e *Engine) Request(ctx context.Context, request model.DataRequest) (model.DataResponse, error) {
	if err := request.Validate(); err != nil {
		e.recordError(err)
		return model.DataResponse{}, err
	}
	e.mu.Lock()
	e.requestCount++
	catalog := e.catalog
	client := e.clients[request.InstrumentID.Venue]
	e.mu.Unlock()
	if catalog != nil {
		response, err := catalog.Query(ctx, request)
		if err == nil {
			e.cacheResponse(response)
			return response, nil
		}
		if client == nil {
			e.recordError(err)
			return model.DataResponse{}, err
		}
	}
	if client == nil {
		err := fmt.Errorf("%w: no data client for venue %s", model.ErrNotSupported, request.InstrumentID.Venue)
		e.recordError(err)
		return model.DataResponse{}, err
	}
	response, err := e.requestLive(ctx, client, request)
	if err != nil {
		e.recordError(err)
		return model.DataResponse{}, err
	}
	e.cacheResponse(response)
	return response, nil
}

func (e *Engine) Health() Health {
	e.mu.RLock()
	defer e.mu.RUnlock()
	clients := make([]ClientHealth, 0, len(e.clients))
	now := time.Now()
	for _, client := range e.clients {
		clientHealth := client.Health()
		stale := e.isStale(now, clientHealth)
		if stale {
			clients = append(clients, ClientHealth{
				Venue:    client.Venue(),
				ClientID: client.ClientID(),
				Health:   clientHealth,
				Stale:    true,
			})
			continue
		}
		clients = append(clients, ClientHealth{
			Venue:    client.Venue(),
			ClientID: client.ClientID(),
			Health:   clientHealth,
		})
	}
	staleClients := 0
	for _, client := range clients {
		if client.Stale {
			staleClients++
		}
	}
	return Health{
		Running:        e.running,
		Clients:        len(e.clients),
		Subscriptions:  len(e.subscriptions),
		Events:         e.eventCount,
		Requests:       e.requestCount,
		AggregatedBars: e.barCount,
		StaleClients:   staleClients,
		LastEventTime:  e.lastEventTime,
		LastError:      e.lastError,
		ClientsHealth:  clients,
	}
}

func (e *Engine) applySubscriptions(ctx context.Context, venueID model.Venue, streaming venue.StreamingDataClient) error {
	e.mu.RLock()
	subs := make([]model.SubscribeMarketData, 0, len(e.subscriptions))
	for _, sub := range e.subscriptions {
		if sub.InstrumentID.Venue == venueID {
			subs = append(subs, sub)
		}
	}
	e.mu.RUnlock()
	for _, sub := range subs {
		if err := streaming.SubscribeMarketData(ctx, sub); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) forward(ctx context.Context, venueID model.Venue, streaming venue.StreamingDataClient) {
	defer e.wg.Done()
	events := streaming.Events()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err := e.recoverStream(ctx, venueID, streaming); err != nil {
					e.recordError(err)
					return
				}
				next := streaming.Events()
				if next == events {
					e.recordError(fmt.Errorf("%w: data client reused closed event channel", model.ErrNotSupported))
					return
				}
				events = next
				continue
			}
			if err := e.processEvent(ctx, event); err != nil {
				e.recordError(err)
			}
		}
	}
}

func (e *Engine) recoverStream(ctx context.Context, venueID model.Venue, streaming venue.StreamingDataClient) error {
	attempts := e.reconnect.attempts()
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := e.reconnectStreamOnce(ctx, venueID, streaming); err != nil {
			lastErr = err
			if attempt == attempts-1 {
				return lastErr
			}
			if err := e.waitReconnectBackoff(ctx); err != nil {
				return err
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (e *Engine) reconnectStreamOnce(ctx context.Context, venueID model.Venue, streaming venue.StreamingDataClient) error {
	if connectable, ok := streaming.(interface{ Connect(context.Context) error }); ok {
		if err := connectable.Connect(ctx); err != nil {
			return err
		}
	}
	return e.applySubscriptions(ctx, venueID, streaming)
}

func (e *Engine) waitReconnectBackoff(ctx context.Context) error {
	backoff := e.reconnect.Backoff
	if backoff <= 0 {
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (p RetryPolicy) attempts() int {
	if p.MaxAttempts <= 0 {
		return 1
	}
	return p.MaxAttempts
}

func (e *Engine) isStale(now time.Time, health venue.DataHealth) bool {
	if e.staleAfter <= 0 || health.LastEventTime.IsZero() {
		return false
	}
	return now.Sub(health.LastEventTime) > e.staleAfter
}

func (e *Engine) processEvent(ctx context.Context, event model.MarketEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if err := e.cache.PutMarketEvent(event); err != nil {
		return err
	}
	e.recordEvent(event)
	if err := e.publish(ctx, event); err != nil {
		return err
	}
	for _, bar := range e.aggregate(event) {
		barEvent := model.MarketEvent{Bar: &bar}
		if err := e.cache.PutMarketEvent(barEvent); err != nil {
			return err
		}
		e.recordBar(bar)
		if err := e.publish(ctx, barEvent); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) publish(ctx context.Context, event model.MarketEvent) error {
	if err := e.bus.Publish(ctx, "market.data", event); err != nil {
		return err
	}
	select {
	case e.events <- event:
	default:
	}
	return nil
}

func (e *Engine) aggregate(event model.MarketEvent) []model.Bar {
	e.mu.Lock()
	defer e.mu.Unlock()
	bars := make([]model.Bar, 0)
	for _, aggregator := range e.aggregators {
		bar, ok, err := aggregator.Update(event)
		if err != nil {
			e.lastError = err
			continue
		}
		if ok {
			bars = append(bars, bar)
		}
	}
	return bars
}

func (e *Engine) requestLive(ctx context.Context, client venue.DataClient, request model.DataRequest) (model.DataResponse, error) {
	response := model.DataResponse{
		Metadata:     responseMetadata(request),
		RequestID:    request.RequestID,
		InstrumentID: request.InstrumentID,
		Type:         request.Type,
		BarType:      request.BarType.Canonical(),
		IsFinal:      true,
	}
	var event model.MarketEvent
	switch request.Type {
	case model.MarketDataTypeTicker:
		ticker, err := client.FetchTicker(ctx, request.InstrumentID)
		if err != nil {
			return model.DataResponse{}, err
		}
		event.Ticker = &ticker
	case model.MarketDataTypeOrderBook:
		book, err := client.FetchOrderBook(ctx, request.InstrumentID, request.Depth)
		if err != nil {
			return model.DataResponse{}, err
		}
		event.OrderBook = &book
	case model.MarketDataTypeFundingRate:
		provider, ok := client.(venue.FundingRateProvider)
		if !ok {
			return model.DataResponse{}, fmt.Errorf("%w: funding rate requests are not supported by venue.DataClient", model.ErrNotSupported)
		}
		funding, err := provider.FetchFundingRate(ctx, request.InstrumentID)
		if err != nil {
			return model.DataResponse{}, err
		}
		event.FundingRate = &funding
	default:
		return model.DataResponse{}, fmt.Errorf("%w: request type %s is not supported by venue.DataClient", model.ErrNotSupported, request.Type)
	}
	response.Events = []model.MarketEvent{event}
	if err := response.Validate(); err != nil {
		return model.DataResponse{}, err
	}
	return response, nil
}

func (e *Engine) cacheResponse(response model.DataResponse) {
	for _, event := range response.Events {
		_ = e.cache.PutMarketEvent(event)
	}
}

func (e *Engine) recordEvent(event model.MarketEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventCount++
	e.lastEventTime = marketEventTime(event)
}

func (e *Engine) recordBar(bar model.Bar) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.barCount++
	e.lastEventTime = bar.Timestamp
}

func (e *Engine) recordError(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastError = err
}
