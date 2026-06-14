package backpack

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	backpacksdk "github.com/QuantProcessing/exchanges/sdk/backpack"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetMarkets(context.Context) ([]backpacksdk.Market, error)
	GetTicker(context.Context, string) (*backpacksdk.Ticker, error)
	GetOrderBook(context.Context, string, int) (*backpacksdk.Depth, error)
	GetBalances(context.Context) (map[string]backpacksdk.CapitalBalance, error)
	GetOpenOrders(context.Context, string, string) ([]backpacksdk.Order, error)
	GetOpenPositions(context.Context, string) ([]backpacksdk.Position, error)
	PlaceOrder(context.Context, backpacksdk.CreateOrderRequest) (*backpacksdk.Order, error)
	CancelOrder(context.Context, backpacksdk.CancelOrderRequest) (*backpacksdk.Order, error)
}

type perpProvider struct {
	sdk        sdkClient
	insts      map[model.InstrumentID]model.Instrument
	rawSymbols map[model.InstrumentID]string
	rawIndex   map[string]model.InstrumentID
}

func newPerpProvider(sdk sdkClient) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), rawSymbols: make(map[model.InstrumentID]string), rawIndex: make(map[string]model.InstrumentID)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	markets, err := p.sdk.GetMarkets(ctx)
	if err != nil {
		return err
	}
	insts := make(map[model.InstrumentID]model.Instrument)
	rawSymbols := make(map[model.InstrumentID]string)
	rawIndex := make(map[string]model.InstrumentID)
	for _, market := range markets {
		if !isPerp(market.MarketType) {
			continue
		}
		priceTick, err := decimalFromString(market.Filters.Price.TickSize, "0.000001")
		if err != nil {
			return err
		}
		sizeTick, err := decimalFromString(market.Filters.Quantity.StepSize, "0.000001")
		if err != nil {
			return err
		}
		base := strings.ToUpper(market.BaseSymbol)
		quote := strings.ToUpper(market.QuoteSymbol)
		id := model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", base, quote), Venue: Venue}
		inst := model.Instrument{ID: id, RawSymbol: market.Symbol, Type: model.InstrumentTypePerp, Base: model.Currency(base), Quote: model.Currency(quote), Settle: model.Currency(quote), PriceTick: priceTick, SizeTick: sizeTick, Status: mapInstrumentStatus(market.Visible)}
		if err := inst.Validate(); err != nil {
			return err
		}
		insts[id] = inst
		rawSymbols[id] = market.Symbol
		rawIndex[market.Symbol] = id
	}
	p.insts = insts
	p.rawSymbols = rawSymbols
	p.rawIndex = rawIndex
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

type dataClient struct {
	id       string
	provider *perpProvider
	sdk      sdkClient
	ws       streamWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]string
	mu       sync.Mutex
	health   venue.DataHealth
}

func newDataClient(id string, provider *perpProvider, sdk sdkClient) *dataClient {
	return &dataClient{
		id:       id,
		provider: provider,
		sdk:      sdk,
		ws:       backpacksdk.NewWSClient(),
		events:   make(chan model.MarketEvent, 256),
		subs:     make(map[string]model.SubscribeMarketData),
		topics:   make(map[string]string),
	}
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
		return c.ws.Close()
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
	t, err := c.sdk.GetTicker(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	last := decimal.RequireFromString(firstNonEmpty(t.LastPrice, "0"))
	ticker := model.Ticker{InstrumentID: id, Bid: last, Ask: last, Last: last, Timestamp: time.Now()}
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
	depth, err := c.sdk.GetOrderBook(ctx, raw, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range depth.Bids {
		if len(bid) < 2 {
			continue
		}
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid[0]), Size: decimal.RequireFromString(bid[1])})
	}
	for _, ask := range depth.Asks {
		if len(ask) < 2 {
			continue
		}
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask[0]), Size: decimal.RequireFromString(ask[1])})
	}
	return book, book.Validate()
}

func (c *dataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	stream := backpackMarketStream(raw, sub)
	if stream == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	topicActive := c.topicActiveLocked(stream)
	c.mu.Unlock()
	if topicActive {
		c.mu.Lock()
		c.subs[sub.Key()] = sub
		c.topics[sub.Key()] = stream
		c.mu.Unlock()
		c.health.Connected = true
		return nil
	}
	switch sub.Type {
	case model.MarketDataTypeTicker:
		err = c.ws.Subscribe(ctx, stream, false, func(payload json.RawMessage) {
			c.handleBookTicker(sub.InstrumentID, payload)
		})
	case model.MarketDataTypeOrderBook, model.MarketDataTypeQuoteTick:
		err = c.ws.Subscribe(ctx, stream, false, func(payload json.RawMessage) {
			c.handleDepth(sub.InstrumentID, payload)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.Subscribe(ctx, stream, false, func(payload json.RawMessage) {
			c.handleTrade(sub.InstrumentID, payload)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.Subscribe(ctx, stream, false, func(payload json.RawMessage) {
			c.handleKline(barType, payload)
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
	c.topics[sub.Key()] = stream
	c.mu.Unlock()
	c.health.Connected = true
	return nil
}

func (c *dataClient) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	stream := backpackMarketStream(raw, sub)
	if stream == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	if stored := c.topics[sub.Key()]; stored != "" {
		stream = stored
	}
	delete(c.subs, sub.Key())
	delete(c.topics, sub.Key())
	stillActive := c.topicActiveLocked(stream)
	c.mu.Unlock()
	if stillActive {
		return nil
	}
	if err := c.ws.Unsubscribe(ctx, stream); err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *dataClient) handleBookTicker(id model.InstrumentID, payload json.RawMessage) {
	var msg struct {
		EventType       string         `json:"e"`
		EventTime       microTimestamp `json:"E"`
		Symbol          string         `json:"s"`
		AskPrice        string         `json:"a"`
		BidPrice        string         `json:"b"`
		EngineTimestamp microTimestamp `json:"T"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	timestamp := msg.EngineTimestamp.Int64()
	if timestamp == 0 {
		timestamp = msg.EventTime.Int64()
	}
	_ = c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{
		InstrumentID: id,
		Bid:          decimal.RequireFromString(firstNonEmpty(msg.BidPrice, "0")),
		Ask:          decimal.RequireFromString(firstNonEmpty(msg.AskPrice, "0")),
		Last:         decimal.Zero,
		Timestamp:    parseUnixMicros(timestamp),
	}})
}

func (c *dataClient) handleDepth(id model.InstrumentID, payload json.RawMessage) {
	var depth backpacksdk.DepthEvent
	if err := json.Unmarshal(payload, &depth); err != nil {
		c.health.LastError = err
		return
	}
	timestamp := depth.EngineTimestamp
	if timestamp == 0 {
		timestamp = depth.EventTime
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseUnixMicros(timestamp)}
	for _, bid := range depth.Bids {
		if len(bid) >= 2 {
			book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid[0]), Size: decimal.RequireFromString(bid[1])})
		}
	}
	for _, ask := range depth.Asks {
		if len(ask) >= 2 {
			book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask[0]), Size: decimal.RequireFromString(ask[1])})
		}
	}
	emitBook, emitQuote := c.depthFanout(id)
	if emitBook {
		if err := c.emitMarket(model.MarketEvent{OrderBook: &book}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitQuoteFromBook(book)
	}
}

func (c *dataClient) handleTrade(id model.InstrumentID, payload json.RawMessage) {
	var trade backpacksdk.Trade
	if err := json.Unmarshal(payload, &trade); err != nil {
		c.health.LastError = err
		return
	}
	tradeID := ""
	if trade.ID != 0 {
		tradeID = fmt.Sprintf("%d", trade.ID)
	}
	if tradeID == "" {
		tradeID = fmt.Sprintf("%s:%d:%s:%s", id.String(), trade.Timestamp, trade.Price, trade.Quantity)
	}
	ts := parseUnixMicros(trade.Timestamp)
	_ = c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
		InstrumentID:  id,
		Price:         decimal.RequireFromString(firstNonEmpty(trade.Price, "0")),
		Size:          decimal.RequireFromString(firstNonEmpty(trade.Quantity, "0")),
		AggressorSide: backpackAggressorSide(trade.IsBuyerMaker),
		TradeID:       model.TradeID(tradeID),
		Timestamp:     ts,
		InitTime:      ts,
	}})
}

func (c *dataClient) handleKline(barType model.BarType, payload json.RawMessage) {
	var kline backpacksdk.Kline
	if err := json.Unmarshal(payload, &kline); err != nil {
		c.health.LastError = err
		return
	}
	ts := parseBackpackTime(firstNonEmpty(kline.End, kline.Start))
	_ = c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:   barType.Canonical(),
		Open:      decimal.RequireFromString(firstNonEmpty(kline.Open, "0")),
		High:      decimal.RequireFromString(firstNonEmpty(kline.High, "0")),
		Low:       decimal.RequireFromString(firstNonEmpty(kline.Low, "0")),
		Close:     decimal.RequireFromString(firstNonEmpty(kline.Close, "0")),
		Volume:    decimal.RequireFromString(firstNonEmpty(kline.Volume, "0")),
		Timestamp: ts,
		InitTime:  ts,
	}})
}

func (c *dataClient) emitQuoteFromBook(book model.OrderBook) error {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		err := fmt.Errorf("%w: backpack quote tick requires top of book", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
	return c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{InstrumentID: book.InstrumentID, BidPrice: book.Bids[0].Price, AskPrice: book.Asks[0].Price, BidSize: book.Bids[0].Size, AskSize: book.Asks[0].Size, Timestamp: book.Timestamp, InitTime: book.Timestamp}})
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
		err := fmt.Errorf("%w: backpack market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *dataClient) topicActiveLocked(stream string) bool {
	for _, activeStream := range c.topics {
		if activeStream == stream {
			return true
		}
	}
	return false
}

func (c *dataClient) depthFanout(id model.InstrumentID) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, emitBook := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeOrderBook}.Key()]
	_, emitQuote := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()]
	return emitBook, emitQuote
}

func backpackMarketStream(raw string, sub model.SubscribeMarketData) string {
	switch sub.Type {
	case model.MarketDataTypeTicker:
		return "bookTicker." + raw
	case model.MarketDataTypeOrderBook, model.MarketDataTypeQuoteTick:
		return "depth." + raw
	case model.MarketDataTypeTradeTick:
		return "trade." + raw
	case model.MarketDataTypeBar:
		return "kline." + backpackBarInterval(sub.BarType.Canonical().Step) + "." + raw
	default:
		return ""
	}
}

type executionClient struct {
	accountID  model.AccountID
	provider   *perpProvider
	sdk        sdkClient
	privateWS  streamWS
	events     chan model.ExecutionEvent
	mu         sync.Mutex
	registered bool
	health     venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *perpProvider, sdk sdkClient, creds ...string) *executionClient {
	if accountID == "" {
		accountID = "backpack-perp"
	}
	client := &executionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 2 && creds[0] != "" && creds[1] != "" {
		client.privateWS = backpacksdk.NewWSClient().WithCredentials(creds[0], creds[1])
	}
	return client
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
		return c.privateWS.Close()
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

func (c *executionClient) subscribePrivate(ctx context.Context) error {
	c.mu.Lock()
	if c.registered {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := c.privateWS.Subscribe(ctx, "account.orderUpdate", true, c.handleOrderUpdate); err != nil {
		return err
	}
	if err := c.privateWS.Subscribe(ctx, "account.positionUpdate", true, c.handlePositionUpdate); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}

func (c *executionClient) handleOrderUpdate(payload json.RawMessage) {
	var update backpacksdk.OrderUpdateEvent
	if err := json.Unmarshal(payload, &update); err != nil {
		c.health.LastError = err
		return
	}
	id, ok := c.provider.instrumentIDByRaw(update.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, update.Symbol)
		return
	}
	report := c.mapOrderUpdate(id, update)
	_ = c.emitExecution(model.ExecutionEvent{Order: &report})
	fillQuantity := decimal.RequireFromString(firstNonEmpty(update.FillQuantity, "0"))
	if !fillQuantity.IsPositive() || update.TradeID.String() == "" {
		return
	}
	fill := model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       model.OrderID(update.OrderID),
		ClientOrderID: model.ClientOrderID(update.ClientID.String()),
		TradeID:       model.TradeID(update.TradeID.String()),
		Side:          fromVenueSide(update.Side),
		Price:         decimal.RequireFromString(firstNonEmpty(update.FillPrice, "0")),
		Quantity:      fillQuantity,
		Fee:           decimal.RequireFromString(firstNonEmpty(update.Fee, "0")).Abs(),
		FeeCurrency:   model.Currency(update.FeeSymbol),
		Timestamp:     parseUnixMicros(firstNonZero(update.EngineTimestamp, update.EventTime)),
	}
	_ = c.emitExecution(model.ExecutionEvent{Fill: &fill})
}

func (c *executionClient) handlePositionUpdate(payload json.RawMessage) {
	var update backpacksdk.PositionUpdateEvent
	if err := json.Unmarshal(payload, &update); err != nil {
		c.health.LastError = err
		return
	}
	id, ok := c.provider.instrumentIDByRaw(update.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, update.Symbol)
		return
	}
	report := c.mapPositionUpdate(id, update)
	_ = c.emitExecution(model.ExecutionEvent{Position: &report})
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
		err := fmt.Errorf("%w: backpack execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	balances, err := c.sdk.GetBalances(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for currency, balance := range balances {
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(strings.ToUpper(currency)), Free: balance.Available, Locked: balance.Locked, Total: totalBalance(balance)})
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
	req := backpacksdk.CreateOrderRequest{Symbol: raw, Side: toVenueSide(cmd.Side), OrderType: toVenueOrderType(cmd.Type), Quantity: cmd.Quantity.String(), Price: zeroBlank(cmd.Price), TimeInForce: toVenueTIF(cmd.TimeInForce), ClientID: parseUint32(string(cmd.ClientOrderID))}
	order, err := c.sdk.PlaceOrder(ctx, req)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	report := c.mapOrder(cmd.InstrumentID, *order)
	if report.ClientOrderID == "" {
		report.ClientOrderID = cmd.ClientOrderID
	}
	if report.Side == "" {
		report.Side = cmd.Side
	}
	if report.Type == "" {
		report.Type = cmd.Type
	}
	if report.Quantity.IsZero() {
		report.Quantity = cmd.Quantity
	}
	if report.Price.IsZero() {
		report.Price = cmd.Price
	}
	return report, nil
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	order, err := c.sdk.CancelOrder(ctx, backpacksdk.CancelOrderRequest{OrderID: string(cmd.OrderID), Symbol: raw})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	report := c.mapOrder(cmd.InstrumentID, *order)
	if report.OrderID == "" {
		report.OrderID = cmd.OrderID
	}
	report.ClientOrderID = cmd.ClientOrderID
	report.Status = model.OrderStatusCanceled
	return report, nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetOpenOrders(ctx, "PERP", raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
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
	positions, err := c.sdk.GetOpenPositions(ctx, raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(positions))
	for _, position := range positions {
		reports = append(reports, c.mapPosition(id, position))
	}
	return reports, nil
}

func (c *executionClient) mapOrder(id model.InstrumentID, order backpacksdk.Order) model.OrderStatusReport {
	quantity := decimal.RequireFromString(firstNonEmpty(order.Quantity, "0"))
	filled := decimal.RequireFromString(firstNonEmpty(order.ExecutedQuantity, "0"))
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(order.ID), ClientOrderID: clientOrderID(order.ClientID), Status: mapOrderStatus(order.Status), Side: fromVenueSide(order.Side), Type: fromVenueOrderType(order.OrderType), Quantity: quantity, FilledQuantity: filled, LeavesQuantity: leavesQuantity(quantity, filled), Price: decimal.RequireFromString(firstNonEmpty(order.Price, "0")), LastUpdatedTime: parseUnixMicros(order.CreatedAt)}
}

func (c *executionClient) mapOrderUpdate(id model.InstrumentID, update backpacksdk.OrderUpdateEvent) model.OrderStatusReport {
	quantity := decimal.RequireFromString(firstNonEmpty(update.Quantity, "0"))
	filled := decimal.RequireFromString(firstNonEmpty(update.ExecutedQuantity, "0"))
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(update.OrderID),
		ClientOrderID:   model.ClientOrderID(update.ClientID.String()),
		Status:          mapOrderStatus(update.OrderState),
		Side:            fromVenueSide(update.Side),
		Type:            fromVenueOrderType(update.OrderType),
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leavesQuantity(quantity, filled),
		Price:           decimal.RequireFromString(firstNonEmpty(update.Price, "0")),
		LastUpdatedTime: parseUnixMicros(firstNonZero(update.EngineTimestamp, update.EventTime)),
	}
}

func (c *executionClient) mapPosition(id model.InstrumentID, position backpacksdk.Position) model.PositionStatusReport {
	qty := decimal.RequireFromString(firstNonEmpty(position.NetQuantity, "0"))
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(firstNonEmpty(position.PositionID, id.String())),
		Side:         backpackPositionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   decimal.RequireFromString(firstNonEmpty(position.EntryPrice, "0")),
		Timestamp:    time.Now(),
	}
}

func (c *executionClient) mapPositionUpdate(id model.InstrumentID, update backpacksdk.PositionUpdateEvent) model.PositionStatusReport {
	qty := decimal.RequireFromString(firstNonEmpty(update.NetQuantity.String(), "0"))
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(firstNonEmpty(update.PositionID, id.String())),
		Side:         backpackPositionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   decimal.RequireFromString(firstNonEmpty(update.EntryPrice.String(), "0")),
		Timestamp:    parseUnixMicros(firstNonZero(update.EngineTimestamp, update.EventTime)),
	}
}

type Adapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := backpacksdk.NewClient()
	if opts.APIKey != "" || opts.PrivateKey != "" {
		client = client.WithCredentials(opts.APIKey, opts.PrivateKey)
	}
	provider := newPerpProvider(client)
	return &Adapter{provider: provider, data: newDataClient("backpack-perp-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, opts.APIKey, opts.PrivateKey)}, nil
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}

type streamWS interface {
	Subscribe(context.Context, string, bool, func(json.RawMessage)) error
	Unsubscribe(context.Context, string) error
	Close() error
}

func parseUnixMicros(value int64) time.Time {
	if value <= 0 {
		return time.Now()
	}
	return time.UnixMicro(value)
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func leavesQuantity(quantity, filled decimal.Decimal) decimal.Decimal {
	leaves := quantity.Sub(filled)
	if leaves.IsNegative() {
		return decimal.Zero
	}
	return leaves
}

func backpackPositionSide(qty decimal.Decimal) model.PositionSide {
	if qty.IsNegative() {
		return model.PositionSideShort
	}
	if qty.IsPositive() {
		return model.PositionSideLong
	}
	return model.PositionSideFlat
}

type microTimestamp int64

func (t *microTimestamp) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), `"`)
	if raw == "" || raw == "null" {
		*t = 0
		return nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return err
	}
	*t = microTimestamp(value)
	return nil
}

func (t microTimestamp) Int64() int64 {
	return int64(t)
}
