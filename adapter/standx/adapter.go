package standx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	standxsdk "github.com/QuantProcessing/exchanges/sdk/standx"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	QuerySymbolInfo(context.Context, string) ([]standxsdk.SymbolInfo, error)
	QuerySymbolMarket(context.Context, string) (standxsdk.SymbolMarket, error)
	QueryDepthBook(context.Context, string, int) (standxsdk.DepthBook, error)
	QueryFundingRates(context.Context, string, int64, int64) ([]standxsdk.FundingRate, error)
	QueryBalances(context.Context) (*standxsdk.Balance, error)
	QueryUserAllOpenOrders(context.Context, string) ([]standxsdk.Order, error)
	QueryPositions(context.Context, string) ([]standxsdk.Position, error)
	CreateOrder(context.Context, standxsdk.CreateOrderRequest, map[string]string) (*standxsdk.APIResponse, error)
	CancelOrder(context.Context, standxsdk.CancelOrderRequest) (*standxsdk.APIResponse, error)
}

type perpProvider struct {
	sdk        sdkClient
	insts      map[model.InstrumentID]model.Instrument
	rawSymbols map[model.InstrumentID]string
	rawIndex   map[string]model.InstrumentID
	quotes     map[model.InstrumentID]string
}

func newPerpProvider(sdk sdkClient) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), rawSymbols: make(map[model.InstrumentID]string), rawIndex: make(map[string]model.InstrumentID), quotes: make(map[model.InstrumentID]string)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	symbols, err := p.sdk.QuerySymbolInfo(ctx, "")
	if err != nil {
		return err
	}
	insts := make(map[model.InstrumentID]model.Instrument)
	rawSymbols := make(map[model.InstrumentID]string)
	rawIndex := make(map[string]model.InstrumentID)
	quotes := make(map[model.InstrumentID]string)
	for _, sym := range symbols {
		base := strings.ToUpper(sym.BaseAsset)
		quote := strings.ToUpper(sym.QuoteAsset)
		id := model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", base, quote), Venue: Venue}
		makerFee, err := decimalFromString(sym.MakerFee, "0")
		if err != nil {
			return err
		}
		takerFee, err := decimalFromString(sym.TakerFee, "0")
		if err != nil {
			return err
		}
		marginInit, err := marginFromLeverage(firstNonEmpty(sym.MaxLeverage, sym.DefLeverage))
		if err != nil {
			return err
		}
		inst := model.Instrument{ID: id, RawSymbol: sym.Symbol, Type: model.InstrumentTypePerp, Base: model.Currency(base), Quote: model.Currency(quote), Settle: model.Currency(quote), PriceTick: decimalTick(sym.PriceTickDecimals), SizeTick: decimalTick(sym.QtyTickDecimals), MakerFee: makerFee, TakerFee: takerFee, MarginInit: marginInit, Status: mapInstrumentStatus(sym.Enabled)}
		if err := inst.Validate(); err != nil {
			return err
		}
		insts[id] = inst
		rawSymbols[id] = sym.Symbol
		rawIndex[sym.Symbol] = id
		quotes[id] = quote
	}
	p.insts = insts
	p.rawSymbols = rawSymbols
	p.rawIndex = rawIndex
	p.quotes = quotes
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

func (p *perpProvider) defaultQuote() string {
	for _, quote := range p.quotes {
		if quote != "" {
			return quote
		}
	}
	return "USDT"
}

type dataClient struct {
	id       string
	provider *perpProvider
	sdk      sdkClient
	ws       marketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]standxMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

type standxMarketTopic struct {
	kind   model.MarketDataType
	symbol string
}

func newDataClient(id string, provider *perpProvider, sdk sdkClient) *dataClient {
	return &dataClient{id: id, provider: provider, sdk: sdk, events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]standxMarketTopic)}
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
	market, err := c.sdk.QuerySymbolMarket(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	last := decimal.RequireFromString(firstNonEmpty(market.LastPrice, market.MidPrice, "0"))
	bid := decimal.RequireFromString(firstNonEmpty(market.Bid1, market.MidPrice, market.LastPrice, "0"))
	ask := decimal.RequireFromString(firstNonEmpty(market.Ask1, market.MidPrice, market.LastPrice, "0"))
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
	depth, err := c.sdk.QueryDepthBook(ctx, raw, limit)
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

func (c *dataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.FundingRate{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	end := time.Now().UnixMilli()
	start := end - int64(24*time.Hour/time.Millisecond)
	rates, err := c.sdk.QueryFundingRates(ctx, raw, start, end)
	if err != nil {
		return model.FundingRate{}, err
	}
	if len(rates) == 0 {
		return model.FundingRate{}, fmt.Errorf("%w: empty StandX funding history for %s", model.ErrInstrumentNotFound, id.String())
	}
	row := latestStandXFundingRate(rates)
	rate, err := decimalFromString(row.FundingRate, "0")
	if err != nil {
		return model.FundingRate{}, err
	}
	mark, err := decimalFromString(row.MarkPrice, "0")
	if err != nil {
		return model.FundingRate{}, err
	}
	index, err := decimalFromString(row.IndexPrice, "0")
	if err != nil {
		return model.FundingRate{}, err
	}
	timestamp := parseStandXTime(firstNonEmpty(row.Time, row.UpdatedAt, row.CreatedAt))
	funding := model.FundingRate{
		InstrumentID: id,
		Rate:         rate,
		MarkPrice:    mark,
		IndexPrice:   index,
		Timestamp:    timestamp,
		InitTime:     time.Now(),
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
	topic := standxTopicFor(raw, sub)
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
	case model.MarketDataTypeTicker:
		err = c.ws.SubscribePrice(raw, func(payload []byte) error {
			return c.handlePrice(sub.InstrumentID, payload)
		})
	case model.MarketDataTypeOrderBook, model.MarketDataTypeQuoteTick:
		err = c.ws.SubscribeDepthBook(raw, func(payload []byte) error {
			return c.handleDepth(sub.InstrumentID, payload)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribePublicTrade(raw, func(payload []byte) error {
			return c.handlePublicTrade(sub.InstrumentID, payload)
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
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := standxTopicFor(raw, sub)
	if topic.kind == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	delete(c.subs, sub.Key())
	delete(c.topics, sub.Key())
	c.mu.Unlock()
	return nil
}

func (c *dataClient) handlePrice(id model.InstrumentID, payload []byte) error {
	var data standxsdk.WSPriceData
	if err := json.Unmarshal(payload, &data); err != nil {
		c.health.LastError = err
		return err
	}
	bid := decimal.RequireFromString(firstNonEmpty(data.Spread[0], data.MidPrice, data.LastPrice, "0"))
	ask := decimal.RequireFromString(firstNonEmpty(data.Spread[1], data.MidPrice, data.LastPrice, "0"))
	last := decimal.RequireFromString(firstNonEmpty(data.LastPrice, data.MidPrice, data.Spread[0], "0"))
	return c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: parseStandXTime(data.Time)}})
}

func (c *dataClient) handleDepth(id model.InstrumentID, payload []byte) error {
	var data standxsdk.WSDepthData
	if err := json.Unmarshal(payload, &data); err != nil {
		c.health.LastError = err
		return err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range data.Bids {
		if len(bid) >= 2 {
			book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid[0]), Size: decimal.RequireFromString(bid[1])})
		}
	}
	for _, ask := range data.Asks {
		if len(ask) >= 2 {
			book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask[0]), Size: decimal.RequireFromString(ask[1])})
		}
	}
	emitBook, emitQuote := c.depthFanout(id)
	if emitBook {
		if err := c.emitMarket(model.MarketEvent{OrderBook: &book}); err != nil {
			return err
		}
	}
	if emitQuote {
		return c.emitQuoteFromBook(book)
	}
	return nil
}

func (c *dataClient) handlePublicTrade(id model.InstrumentID, payload []byte) error {
	var trades []standxsdk.RecentTrade
	if err := json.Unmarshal(payload, &trades); err != nil {
		var trade standxsdk.RecentTrade
		if err := json.Unmarshal(payload, &trade); err != nil {
			c.health.LastError = err
			return err
		}
		trades = []standxsdk.RecentTrade{trade}
	}
	for _, trade := range trades {
		ts := parseStandXTime(trade.Time)
		tradeID := fmt.Sprintf("%s:%s:%s:%s", firstNonEmpty(trade.Symbol, id.Symbol), trade.Time, trade.Price, trade.Qty)
		if err := c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  id,
			Price:         decimal.RequireFromString(firstNonEmpty(trade.Price, "0")),
			Size:          decimal.RequireFromString(firstNonEmpty(trade.Qty, "0")),
			AggressorSide: standxAggressorSide(trade.IsBuyerTaker),
			TradeID:       model.TradeID(tradeID),
			Timestamp:     ts,
			InitTime:      ts,
		}}); err != nil {
			return err
		}
	}
	return nil
}

func (c *dataClient) emitQuoteFromBook(book model.OrderBook) error {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		err := fmt.Errorf("%w: standx quote tick requires top of book", model.ErrInvalidMarketData)
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
		err := fmt.Errorf("%w: standx market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *dataClient) topicActiveLocked(topic standxMarketTopic) bool {
	for _, activeTopic := range c.topics {
		if activeTopic == topic {
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

type executionClient struct {
	accountID  model.AccountID
	provider   *perpProvider
	sdk        sdkClient
	privateWS  accountWS
	events     chan model.ExecutionEvent
	mu         sync.Mutex
	registered bool
	health     venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *perpProvider, sdk sdkClient) *executionClient {
	if accountID == "" {
		accountID = "standx-perp"
	}
	return &executionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
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
	if err := c.privateWS.Auth(); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeOrderUpdate(c.handleOrderUpdate); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeTradeUpdate(c.handleTradeUpdate); err != nil {
		return err
	}
	if err := c.privateWS.SubscribePositionUpdate(c.handlePositionUpdate); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}

func (c *executionClient) handleOrderUpdate(order *standxsdk.Order) {
	if order == nil {
		return
	}
	id, ok := c.provider.instrumentIDByRaw(order.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: standx symbol %s", model.ErrInstrumentNotFound, order.Symbol)
		return
	}
	report := c.mapOrder(id, *order)
	_ = c.emitExecution(model.ExecutionEvent{Order: &report})
}

func (c *executionClient) handleTradeUpdate(trade *standxsdk.Trade) {
	if trade == nil {
		return
	}
	id, ok := c.provider.instrumentIDByRaw(trade.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: standx symbol %s", model.ErrInstrumentNotFound, trade.Symbol)
		return
	}
	inst, _ := c.provider.Get(id)
	feeCurrency := model.Currency(trade.FeeAsset)
	if feeCurrency == "" {
		feeCurrency = inst.Quote
	}
	report := model.FillReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(strconv.Itoa(trade.OrderID)),
		TradeID:      model.TradeID(strconv.Itoa(trade.ID)),
		Side:         fromVenueSide(trade.Side),
		Price:        decimal.RequireFromString(firstNonEmpty(trade.Price, "0")),
		Quantity:     decimal.RequireFromString(firstNonEmpty(trade.Qty, "0")),
		Fee:          decimal.RequireFromString(firstNonEmpty(trade.FeeQty, "0")).Abs(),
		FeeCurrency:  feeCurrency,
		Timestamp:    parseStandXTime(firstNonEmpty(trade.UpdatedAt, trade.CreatedAt)),
	}
	_ = c.emitExecution(model.ExecutionEvent{Fill: &report})
}

func (c *executionClient) handlePositionUpdate(position *standxsdk.Position) {
	if position == nil {
		return
	}
	report, err := c.positionReport(*position)
	if err != nil {
		c.health.LastError = err
		return
	}
	_ = c.emitExecution(model.ExecutionEvent{Position: &report})
}

func (c *executionClient) positionReport(position standxsdk.Position) (model.PositionStatusReport, error) {
	id, ok := c.provider.instrumentIDByRaw(position.Symbol)
	if !ok {
		return model.PositionStatusReport{}, fmt.Errorf("%w: standx symbol %s", model.ErrInstrumentNotFound, position.Symbol)
	}
	qty := decimal.RequireFromString(firstNonEmpty(position.Qty, "0"))
	return model.PositionStatusReport{AccountID: c.accountID, InstrumentID: id, PositionID: model.PositionID(id.String()), Side: positionSide(qty), Quantity: qty.Abs(), EntryPrice: decimal.RequireFromString(firstNonEmpty(position.EntryPrice, "0")), Timestamp: parseStandXTime(firstNonEmpty(position.UpdatedAt, position.Time, position.CreatedAt))}, nil
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
		err := fmt.Errorf("%w: standx execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.AccountSnapshot{}, err
	}
	bal, err := c.sdk.QueryBalances(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	total := firstNonEmpty(bal.Equity, bal.Balance, "0")
	free := firstNonEmpty(bal.CrossAvailable, total)
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(c.provider.defaultQuote()), Free: free, Locked: firstNonEmpty(bal.Locked, lockedAmount(total, free)), Total: total})
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
	req := standxsdk.CreateOrderRequest{Symbol: raw, Side: toVenueSide(cmd.Side), OrderType: toVenueOrderType(cmd.Type), Qty: cmd.Quantity.String(), Price: zeroBlank(cmd.Price), TimeInForce: toVenueTIF(cmd.TimeInForce), ClientOrdID: string(cmd.ClientOrderID)}
	resp, err := c.sdk.CreateOrder(ctx, req, nil)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	orderID := firstNonEmpty(string(cmd.ClientOrderID), resp.RequestID, "accepted")
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: model.OrderID(orderID), ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusAccepted, Side: cmd.Side, Type: cmd.Type, Quantity: cmd.Quantity, Price: cmd.Price, LastUpdatedTime: time.Now()}, nil
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	req := standxsdk.CancelOrderRequest{OrderID: string(cmd.OrderID), ClOrdID: string(cmd.ClientOrderID), Symbol: raw}
	if _, err := c.sdk.CancelOrder(ctx, req); err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: cmd.OrderID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.QueryUserAllOpenOrders(ctx, raw)
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
	positions, err := c.sdk.QueryPositions(ctx, raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(positions))
	for _, position := range positions {
		if position.Symbol != raw {
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

func (c *executionClient) mapOrder(id model.InstrumentID, order standxsdk.Order) model.OrderStatusReport {
	orderID := strconv.Itoa(order.ID)
	if order.ID == 0 {
		orderID = order.ClOrdID
	}
	quantity := decimal.RequireFromString(firstNonEmpty(order.Qty, "0"))
	filled := decimal.RequireFromString(firstNonEmpty(order.FillQty, "0"))
	status := mapOrderStatus(order.Status)
	if status == model.OrderStatusAccepted && filled.IsPositive() {
		status = model.OrderStatusPartiallyFilled
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(orderID), ClientOrderID: model.ClientOrderID(order.ClOrdID), Status: status, Side: fromVenueSide(order.Side), Type: fromVenueOrderType(order.OrderType), Quantity: quantity, FilledQuantity: filled, LeavesQuantity: leavesQuantity(quantity, filled), Price: decimal.RequireFromString(firstNonEmpty(order.Price, "0")), AveragePrice: decimal.RequireFromString(firstNonEmpty(order.FillAvgPrice, "0")), LastUpdatedTime: parseStandXTime(firstNonEmpty(order.UpdatedAt, order.CreatedAt))}
}

type Adapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := standxsdk.NewClient()
	if opts.PrivateKey != "" {
		var err error
		client, err = client.WithCredentials(opts.PrivateKey)
		if err != nil {
			return nil, err
		}
	}
	provider := newPerpProvider(client)
	data := newDataClient("standx-perp-data", provider, client)
	data.ws = standxsdk.NewWsMarketClient(context.Background())
	exec := newExecutionClient(opts.AccountID, provider, client)
	if opts.PrivateKey != "" {
		exec.privateWS = standxsdk.NewWsAccountClient(context.Background(), client)
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, FundingRates: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}

func latestStandXFundingRate(rows []standxsdk.FundingRate) standxsdk.FundingRate {
	latest := rows[0]
	latestTime := parseStandXTime(firstNonEmpty(latest.Time, latest.UpdatedAt, latest.CreatedAt))
	for _, row := range rows[1:] {
		rowTime := parseStandXTime(firstNonEmpty(row.Time, row.UpdatedAt, row.CreatedAt))
		if rowTime.After(latestTime) {
			latest = row
			latestTime = rowTime
		}
	}
	return latest
}

type marketWS interface {
	Connect() error
	SubscribePrice(string, func([]byte) error) error
	SubscribeDepthBook(string, func([]byte) error) error
	SubscribePublicTrade(string, func([]byte) error) error
	Close()
}

type accountWS interface {
	Connect() error
	Auth() error
	SubscribeOrderUpdate(func(*standxsdk.Order)) error
	SubscribeTradeUpdate(func(*standxsdk.Trade)) error
	SubscribePositionUpdate(func(*standxsdk.Position)) error
	Close()
}
