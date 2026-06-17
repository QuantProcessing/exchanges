package grvt

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	grvtsdk "github.com/QuantProcessing/exchanges/sdk/grvt"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetInstruments(context.Context) ([]grvtsdk.Instrument, error)
	GetTicker(context.Context, string) (*grvtsdk.GetTickerResponse, error)
	GetOrderBook(context.Context, string, int) (*grvtsdk.GetOrderBookResponse, error)
	GetAccountSummary(context.Context) (*grvtsdk.GetAccountSummaryResponse, error)
	GetOpenOrders(context.Context, string) ([]grvtsdk.Order, error)
	CreateOrder(context.Context, *grvtsdk.CreateOrderRequest, map[string]grvtsdk.Instrument) (*grvtsdk.CreateOrderResponse, error)
	CancelOrder(context.Context, string) error
}

type perpProvider struct {
	sdk         sdkClient
	insts       map[model.InstrumentID]model.Instrument
	rawSymbols  map[model.InstrumentID]string
	rawIndex    map[string]model.InstrumentID
	baseSymbols map[model.InstrumentID]string
	instruments map[string]grvtsdk.Instrument
}

func newPerpProvider(sdk sdkClient) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), rawSymbols: make(map[model.InstrumentID]string), rawIndex: make(map[string]model.InstrumentID), baseSymbols: make(map[model.InstrumentID]string), instruments: make(map[string]grvtsdk.Instrument)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	instruments, err := p.sdk.GetInstruments(ctx)
	if err != nil {
		return err
	}
	insts := make(map[model.InstrumentID]model.Instrument)
	rawSymbols := make(map[model.InstrumentID]string)
	rawIndex := make(map[string]model.InstrumentID)
	baseSymbols := make(map[model.InstrumentID]string)
	instrumentMap := make(map[string]grvtsdk.Instrument)
	for _, raw := range instruments {
		if !isPerp(raw) {
			continue
		}
		priceTick, err := decimalFromString(raw.TickSize, "0.000001")
		if err != nil {
			return err
		}
		sizeTick, err := decimalFromString(raw.MinSize, "0.000001")
		if err != nil {
			return err
		}
		base := strings.ToUpper(raw.Base)
		quote := strings.ToUpper(raw.Quote)
		id := model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", base, quote), Venue: Venue}
		inst := model.Instrument{ID: id, RawSymbol: raw.Instrument, Type: model.InstrumentTypePerp, Base: model.Currency(base), Quote: model.Currency(quote), Settle: model.Currency(quote), PriceTick: priceTick, SizeTick: sizeTick, Status: model.InstrumentStatusTrading}
		if err := inst.Validate(); err != nil {
			return err
		}
		insts[id] = inst
		rawSymbols[id] = raw.Instrument
		rawIndex[raw.Instrument] = id
		baseSymbols[id] = base
		instrumentMap[raw.Instrument] = raw
	}
	p.insts = insts
	p.rawSymbols = rawSymbols
	p.rawIndex = rawIndex
	p.baseSymbols = baseSymbols
	p.instruments = instrumentMap
	return nil
}

func (p *perpProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	inst, ok := p.insts[id]
	return inst, ok
}

func (p *perpProvider) List() []model.Instrument {
	out := make([]model.Instrument, 0, len(p.insts))
	for _, inst := range p.insts {
		out = append(out, inst)
	}
	return out
}

func (p *perpProvider) ensureLoaded(ctx context.Context) error {
	if len(p.insts) > 0 {
		return nil
	}
	return p.LoadAll(ctx)
}

func (p *perpProvider) rawSymbol(id model.InstrumentID) (string, error) {
	raw, ok := p.rawSymbols[id]
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return raw, nil
}

func (p *perpProvider) instrumentIDByRaw(raw string) (model.InstrumentID, bool) {
	id, ok := p.rawIndex[raw]
	return id, ok
}

func (p *perpProvider) baseSymbol(id model.InstrumentID) (string, error) {
	base, ok := p.baseSymbols[id]
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return base, nil
}

func (p *perpProvider) instrumentMap() map[string]grvtsdk.Instrument {
	out := make(map[string]grvtsdk.Instrument, len(p.instruments))
	for raw, inst := range p.instruments {
		out[raw] = inst
	}
	return out
}

type dataClient struct {
	id       string
	provider *perpProvider
	sdk      sdkClient
	ws       marketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]grvtMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

type grvtMarketTopic struct {
	kind           model.MarketDataType
	instrument     string
	tickerInterval grvtsdk.TickerSnapRate
	bookInterval   grvtsdk.OrderBookSnapRate
	bookDepth      grvtsdk.OrderBookSnapDepth
	tradeLimit     grvtsdk.TradeLimit
	klineInterval  grvtsdk.KlineInterval
	klineType      grvtsdk.KlineType
}

func newDataClient(id string, provider *perpProvider, sdk sdkClient) *dataClient {
	return &dataClient{id: id, provider: provider, sdk: sdk, events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]grvtMarketTopic)}
}

func (c *dataClient) Venue() model.Venue                    { return Venue }
func (c *dataClient) ClientID() string                      { return c.id }
func (c *dataClient) Instruments() venue.InstrumentProvider { return c.provider }

func (c *dataClient) Connect(ctx context.Context) error {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.Connected = true
	c.health.InstrumentReady = true
	c.health.LastEventTime = time.Now()
	c.health.LastError = nil
	return nil
}

func (c *dataClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.ws != nil {
		c.ws.Close()
	}
	return nil
}

func (c *dataClient) Health() venue.DataHealth { return c.health }
func (c *dataClient) Events() <-chan model.MarketEvent {
	return c.events
}

func (c *dataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.Ticker{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.Ticker{}, err
	}
	resp, err := c.sdk.GetTicker(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	t := resp.Result
	last := decimal.RequireFromString(firstNonEmpty(t.LastPrice, t.MidPrice, "0"))
	bid := decimal.RequireFromString(firstNonEmpty(t.BestBidPrice, t.MidPrice, t.LastPrice, "0"))
	ask := decimal.RequireFromString(firstNonEmpty(t.BestAskPrice, t.MidPrice, t.LastPrice, "0"))
	ticker := model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: time.Now()}
	return ticker, ticker.Validate()
}

func (c *dataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderBook{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	resp, err := c.sdk.GetOrderBook(ctx, raw, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range resp.Result.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid.Price), Size: decimal.RequireFromString(bid.Size)})
	}
	for _, ask := range resp.Result.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask.Price), Size: decimal.RequireFromString(ask.Size)})
	}
	return book, book.Validate()
}

func (c *dataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.FundingRate{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	resp, err := c.sdk.GetTicker(ctx, raw)
	if err != nil {
		return model.FundingRate{}, err
	}
	ticker := resp.Result
	rate, err := decimalFromString(ticker.FundingRate, "0")
	if err != nil {
		return model.FundingRate{}, err
	}
	timestamp := time.Now()
	if ticker.EventTime != "" {
		timestamp = parseGRVTTime(ticker.EventTime)
	}
	var fundingInterval time.Duration
	if inst, ok := c.provider.instruments[raw]; ok && inst.FundingIntervalHours > 0 {
		fundingInterval = time.Duration(inst.FundingIntervalHours) * time.Hour
	}
	var nextFundingTime time.Time
	if ticker.NextFundingTime != "" {
		nextFundingTime = parseGRVTTime(ticker.NextFundingTime)
	}
	funding := model.FundingRate{
		InstrumentID:    id,
		Rate:            rate,
		NextFundingTime: nextFundingTime,
		FundingInterval: fundingInterval,
		Timestamp:       timestamp,
		InitTime:        time.Now(),
	}
	return funding, funding.Validate()
}

func (c *dataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if c.ws == nil {
		return model.ErrNotSupported
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := grvtTopicFor(raw, sub)
	if topic.kind == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	topicActive := c.topicActiveLocked(topic)
	c.mu.Unlock()
	if topicActive {
		c.mu.Lock()
		c.subs[sub.Key()] = sub
		c.topics[sub.Key()] = topic
		c.mu.Unlock()
		c.health.Connected = true
		c.health.LastError = nil
		return nil
	}
	if err := c.ws.Connect(); err != nil {
		c.health.LastError = err
		return err
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		err = c.ws.SubscribeTickerSnap(raw, topic.tickerInterval, func(event grvtsdk.WsFeeData[grvtsdk.Ticker]) error {
			return c.handleTicker(sub.InstrumentID, event.Feed)
		})
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeOrderbookSnap(raw, topic.bookInterval, topic.bookDepth, func(event grvtsdk.WsFeeData[grvtsdk.OrderBook]) error {
			return c.handleOrderBook(sub.InstrumentID, event.Feed)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeTrade(raw, topic.tradeLimit, func(event grvtsdk.WsFeeData[grvtsdk.Trade]) error {
			return c.handleTrade(sub.InstrumentID, event.Feed)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeKline(raw, topic.klineInterval, topic.klineType, func(event grvtsdk.WsFeeData[grvtsdk.KLine]) error {
			return c.handleKline(barType, event.Feed)
		})
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	c.mu.Lock()
	c.subs[sub.Key()] = sub
	c.topics[sub.Key()] = topic
	c.mu.Unlock()
	c.health.Connected = true
	c.health.LastError = nil
	return nil
}

func (c *dataClient) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if c.ws == nil {
		return model.ErrNotSupported
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := grvtTopicFor(raw, sub)
	if topic.kind == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	if stored := c.topics[sub.Key()]; stored.kind != "" {
		topic = stored
	}
	delete(c.subs, sub.Key())
	delete(c.topics, sub.Key())
	stillActive := c.topicActiveLocked(topic)
	c.mu.Unlock()
	if stillActive {
		return nil
	}
	switch topic.kind {
	case model.MarketDataTypeTicker:
		err = c.ws.UnsubscribeTickerSnap(raw, topic.tickerInterval)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeOrderbookSnap(raw, topic.bookInterval, topic.bookDepth)
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeTrade(raw, topic.tradeLimit)
	case model.MarketDataTypeBar:
		err = c.ws.UnsubscribeKline(raw, topic.klineInterval, topic.klineType)
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *dataClient) handleTicker(id model.InstrumentID, raw grvtsdk.Ticker) error {
	last := decimal.RequireFromString(firstNonEmpty(raw.LastPrice, raw.MidPrice, "0"))
	bid := decimal.RequireFromString(firstNonEmpty(raw.BestBidPrice, raw.MidPrice, raw.LastPrice, "0"))
	ask := decimal.RequireFromString(firstNonEmpty(raw.BestAskPrice, raw.MidPrice, raw.LastPrice, "0"))
	ts := parseGRVTTime(raw.EventTime)
	emitTicker, emitQuote := c.tickerFanout(id)
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: ts}}); err != nil {
			return err
		}
	}
	if emitQuote {
		return c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     bid,
			AskPrice:     ask,
			BidSize:      decimal.RequireFromString(firstNonEmpty(raw.BestBidSize, "0")),
			AskSize:      decimal.RequireFromString(firstNonEmpty(raw.BestAskSize, "0")),
			Timestamp:    ts,
			InitTime:     ts,
		}})
	}
	return nil
}

func (c *dataClient) handleOrderBook(id model.InstrumentID, raw grvtsdk.OrderBook) error {
	book := model.OrderBook{InstrumentID: id, Timestamp: parseGRVTTime(raw.EventTime)}
	for _, bid := range raw.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid.Price), Size: decimal.RequireFromString(bid.Size)})
	}
	for _, ask := range raw.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask.Price), Size: decimal.RequireFromString(ask.Size)})
	}
	return c.emitMarket(model.MarketEvent{OrderBook: &book})
}

func (c *dataClient) handleTrade(id model.InstrumentID, raw grvtsdk.Trade) error {
	ts := parseGRVTTime(raw.EventTime)
	return c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
		InstrumentID:  id,
		Price:         decimal.RequireFromString(firstNonEmpty(raw.Price, "0")),
		Size:          decimal.RequireFromString(firstNonEmpty(raw.Size, "0")),
		AggressorSide: grvtAggressorSide(raw.IsTakerBuyer),
		TradeID:       model.TradeID(firstNonEmpty(raw.TradeId, fmt.Sprintf("%s:%s:%s", raw.EventTime, raw.Price, raw.Size))),
		Timestamp:     ts,
		InitTime:      ts,
	}})
}

func (c *dataClient) handleKline(barType model.BarType, raw grvtsdk.KLine) error {
	ts := parseGRVTTime(firstNonEmpty(raw.CloseTime, raw.OpenTime))
	return c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:   barType.Canonical(),
		Open:      decimal.RequireFromString(firstNonEmpty(raw.Open, "0")),
		High:      decimal.RequireFromString(firstNonEmpty(raw.High, "0")),
		Low:       decimal.RequireFromString(firstNonEmpty(raw.Low, "0")),
		Close:     decimal.RequireFromString(firstNonEmpty(raw.Close, "0")),
		Volume:    decimal.RequireFromString(firstNonEmpty(raw.VolumeB, "0")),
		Timestamp: ts,
		InitTime:  ts,
	}})
}

func (c *dataClient) emitMarket(event model.MarketEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: grvt market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *dataClient) topicActiveLocked(topic grvtMarketTopic) bool {
	for _, activeTopic := range c.topics {
		if activeTopic == topic {
			return true
		}
	}
	return false
}

func (c *dataClient) tickerFanout(id model.InstrumentID) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, emitTicker := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()]
	_, emitQuote := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()]
	return emitTicker, emitQuote
}

type executionClient struct {
	accountID    model.AccountID
	subAccountID uint64
	provider     *perpProvider
	sdk          sdkClient
	privateWS    accountWS
	events       chan model.ExecutionEvent
	mu           sync.Mutex
	registered   bool
	health       venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *perpProvider, sdk sdkClient, subAccountID uint64) *executionClient {
	if accountID == "" {
		accountID = "grvt-perp"
	}
	return &executionClient{accountID: accountID, subAccountID: subAccountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
}

func (c *executionClient) Venue() model.Venue         { return Venue }
func (c *executionClient) AccountID() model.AccountID { return c.accountID }

func (c *executionClient) Connect(ctx context.Context) error {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	if c.privateWS != nil {
		if err := c.subscribePrivate(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	c.health.Connected = true
	c.health.AccountReady = true
	c.health.LastEventTime = time.Now()
	c.health.LastError = nil
	return nil
}

func (c *executionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.privateWS != nil {
		c.privateWS.Close()
	}
	return nil
}

func (c *executionClient) Health() venue.ExecutionHealth       { return c.health }
func (c *executionClient) Events() <-chan model.ExecutionEvent { return c.events }

func (c *executionClient) ResubscribeExecution(ctx context.Context) error {
	if c.privateWS == nil {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()
	return c.subscribePrivate(ctx)
}

func (c *executionClient) subscribePrivate(context.Context) error {
	c.mu.Lock()
	if c.registered {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := c.privateWS.Connect(); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeOrderUpdate("all", c.handleOrderUpdate); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeFill("all", c.handleFill); err != nil {
		return err
	}
	if err := c.privateWS.SubscribePositions("all", c.handlePosition); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}

func (c *executionClient) handleOrderUpdate(event grvtsdk.WsFeeData[grvtsdk.Order]) error {
	if len(event.Feed.Legs) == 0 {
		return nil
	}
	id, ok := c.provider.instrumentIDByRaw(event.Feed.Legs[0].Instrument)
	if !ok {
		err := fmt.Errorf("%w: grvt instrument %s", model.ErrInstrumentNotFound, event.Feed.Legs[0].Instrument)
		c.health.LastError = err
		return err
	}
	report := c.mapOrder(id, event.Feed)
	return c.emitExecution(model.ExecutionEvent{Order: &report})
}

func (c *executionClient) handleFill(event grvtsdk.WsFeeData[grvtsdk.WsFill]) error {
	id, ok := c.provider.instrumentIDByRaw(event.Feed.Instrument)
	if !ok {
		err := fmt.Errorf("%w: grvt instrument %s", model.ErrInstrumentNotFound, event.Feed.Instrument)
		c.health.LastError = err
		return err
	}
	inst, _ := c.provider.Get(id)
	report := model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       model.OrderID(event.Feed.OrderID),
		ClientOrderID: model.ClientOrderID(event.Feed.ClientOrderID),
		TradeID:       model.TradeID(event.Feed.TradeID),
		Side:          fromVenueSide(event.Feed.IsBuyer),
		Price:         decimal.RequireFromString(firstNonEmpty(event.Feed.Price, "0")),
		Quantity:      decimal.RequireFromString(firstNonEmpty(event.Feed.Size, "0")),
		Fee:           decimal.RequireFromString(firstNonEmpty(event.Feed.Fee, "0")).Abs(),
		FeeCurrency:   inst.Quote,
		Timestamp:     parseGRVTTime(event.Feed.EventTime),
	}
	return c.emitExecution(model.ExecutionEvent{Fill: &report})
}

func (c *executionClient) handlePosition(event grvtsdk.WsFeeData[grvtsdk.Position]) error {
	report, err := c.positionReport(event.Feed)
	if err != nil {
		c.health.LastError = err
		return err
	}
	return c.emitExecution(model.ExecutionEvent{Position: &report})
}

func (c *executionClient) positionReport(position grvtsdk.Position) (model.PositionStatusReport, error) {
	id, ok := c.provider.instrumentIDByRaw(position.Instrument)
	if !ok {
		return model.PositionStatusReport{}, fmt.Errorf("%w: grvt instrument %s", model.ErrInstrumentNotFound, position.Instrument)
	}
	qty := decimal.RequireFromString(firstNonEmpty(position.Size, "0"))
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(id.String()),
		Side:         positionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   decimal.RequireFromString(firstNonEmpty(position.EntryPrice, "0")),
		Timestamp:    parseGRVTTime(position.EventTime),
	}, nil
}

func (c *executionClient) emitExecution(event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: grvt execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	account, err := c.sdk.GetAccountSummary(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	summary := account.Result
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	seen := make(map[string]bool)
	for _, bal := range summary.SpotBalance {
		currency := strings.ToUpper(bal.Currency)
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(currency), Free: bal.Balance, Total: bal.Balance})
		seen[currency] = true
	}
	settle := strings.ToUpper(summary.SettleCurrency)
	if settle != "" && !seen[settle] {
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(settle), Free: summary.AvailableBalance, Locked: lockedAmount(summary.TotalEquity, summary.AvailableBalance), Total: summary.TotalEquity})
	}
	return snapshot, nil
}

func (c *executionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	req := &grvtsdk.CreateOrderRequest{Order: grvtsdk.OrderRequest{
		SubAccountID: c.subAccountID,
		IsMarket:     cmd.Type == model.OrderTypeMarket,
		TimeInForce:  toVenueTIF(cmd.TimeInForce),
		Legs: []grvtsdk.OrderLeg{{
			Instrument:    raw,
			Size:          cmd.Quantity.String(),
			LimitPrice:    zeroBlank(cmd.Price),
			IsBuyintAsset: cmd.Side == model.OrderSideBuy,
		}},
		Metadata: grvtsdk.OrderMetadata{ClientOrderID: string(cmd.ClientOrderID), CreatedTime: time.Now().UTC().Format(time.RFC3339Nano)},
	}}
	resp, err := c.sdk.CreateOrder(ctx, req, c.provider.instrumentMap())
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	report := c.mapOrder(cmd.InstrumentID, resp.Result)
	if report.OrderID == "" {
		report.OrderID = model.OrderID(firstNonEmpty(string(cmd.ClientOrderID), "accepted"))
	}
	report.AccountID = c.accountID
	report.ClientOrderID = cmd.ClientOrderID
	report.Side = cmd.Side
	report.Type = cmd.Type
	report.Quantity = cmd.Quantity
	report.Price = cmd.Price
	return report, nil
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := c.sdk.CancelOrder(ctx, string(cmd.OrderID)); err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: cmd.OrderID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	base, err := c.provider.baseSymbol(id)
	if err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetOpenOrders(ctx, base)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		if len(order.Legs) > 0 && order.Legs[0].Instrument != raw {
			continue
		}
		reports = append(reports, c.mapOrder(id, order))
	}
	return reports, nil
}

func (c *executionClient) GeneratePositionStatusReports(ctx context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	account, err := c.sdk.GetAccountSummary(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(account.Result.Position))
	for _, position := range account.Result.Position {
		if position.Instrument != raw {
			continue
		}
		report, err := c.positionReport(position)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func (c *executionClient) mapOrder(id model.InstrumentID, order grvtsdk.Order) model.OrderStatusReport {
	report := model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(order.OrderID), ClientOrderID: model.ClientOrderID(order.Metadata.ClientOrderID), Status: mapOrderStatus(order.State.Status), Type: fromVenueOrderType(order.IsMarket), LastUpdatedTime: parseGRVTTime(order.State.UpdateTime)}
	if len(order.Legs) > 0 {
		leg := order.Legs[0]
		report.Side = fromVenueSide(leg.IsBuyintAsset)
		report.Quantity = decimal.RequireFromString(firstNonEmpty(leg.Size, "0"))
		report.Price = decimal.RequireFromString(firstNonEmpty(leg.LimitPrice, "0"))
	}
	if len(order.State.TradedSize) > 0 {
		report.FilledQuantity = decimal.RequireFromString(firstNonEmpty(order.State.TradedSize[0], "0"))
	}
	if len(order.State.BookSize) > 0 {
		report.LeavesQuantity = decimal.RequireFromString(firstNonEmpty(order.State.BookSize[0], "0"))
	} else {
		report.LeavesQuantity = leavesQuantity(report.Quantity, report.FilledQuantity)
	}
	if len(order.State.AvgFillPrice) > 0 {
		report.AveragePrice = decimal.RequireFromString(firstNonEmpty(order.State.AvgFillPrice[0], "0"))
	}
	if report.Status == model.OrderStatusAccepted && report.FilledQuantity.IsPositive() {
		report.Status = model.OrderStatusPartiallyFilled
	}
	return report
}

type Adapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := grvtsdk.NewClient()
	if opts.APIKey != "" || opts.SubAccountID != "" || opts.PrivateKey != "" {
		client = client.WithCredentials(opts.APIKey, opts.SubAccountID, opts.PrivateKey)
	}
	provider := newPerpProvider(client)
	subAccountID := parseUint64(opts.SubAccountID)
	data := newDataClient("grvt-perp-data", provider, client)
	data.ws = grvtsdk.NewMarketWebsocketClient(context.Background(), client)
	exec := newExecutionClient(opts.AccountID, provider, client, subAccountID)
	if opts.APIKey != "" || opts.SubAccountID != "" || opts.PrivateKey != "" {
		exec.privateWS = grvtsdk.NewAccountWebsocketClient(context.Background(), client)
	}
	return &Adapter{provider: provider, data: data, exec: exec}, nil
}

func (a *Adapter) Venue() model.Venue                    { return Venue }
func (a *Adapter) Instruments() venue.InstrumentProvider { return a.provider }
func (a *Adapter) Data() venue.DataClient                { return a.data }
func (a *Adapter) Execution() venue.ExecutionClient      { return a.exec }
func (a *Adapter) Close(ctx context.Context) error {
	_ = a.data.Disconnect(ctx)
	return a.exec.Disconnect(ctx)
}
func (a *Adapter) Capabilities() venue.DeclaredCapabilities {
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, FundingRates: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}

type marketWS interface {
	Connect() error
	SubscribeTickerSnap(string, grvtsdk.TickerSnapRate, func(grvtsdk.WsFeeData[grvtsdk.Ticker]) error) error
	SubscribeOrderbookSnap(string, grvtsdk.OrderBookSnapRate, grvtsdk.OrderBookSnapDepth, func(grvtsdk.WsFeeData[grvtsdk.OrderBook]) error) error
	SubscribeTrade(string, grvtsdk.TradeLimit, func(grvtsdk.WsFeeData[grvtsdk.Trade]) error) error
	SubscribeKline(string, grvtsdk.KlineInterval, grvtsdk.KlineType, func(grvtsdk.WsFeeData[grvtsdk.KLine]) error) error
	UnsubscribeTickerSnap(string, grvtsdk.TickerSnapRate) error
	UnsubscribeOrderbookSnap(string, grvtsdk.OrderBookSnapRate, grvtsdk.OrderBookSnapDepth) error
	UnsubscribeTrade(string, grvtsdk.TradeLimit) error
	UnsubscribeKline(string, grvtsdk.KlineInterval, grvtsdk.KlineType) error
	Close()
}

type accountWS interface {
	Connect() error
	SubscribeOrderUpdate(string, func(grvtsdk.WsFeeData[grvtsdk.Order]) error) error
	SubscribeFill(string, func(grvtsdk.WsFeeData[grvtsdk.WsFill]) error) error
	SubscribePositions(string, func(grvtsdk.WsFeeData[grvtsdk.Position]) error) error
	Close()
}
