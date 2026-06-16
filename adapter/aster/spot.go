package aster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	asterspot "github.com/QuantProcessing/exchanges/sdk/aster/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type spotSDK interface {
	ExchangeInfo(context.Context) (*asterspot.ExchangeInfoResponse, error)
	Ticker(context.Context, string) (*asterspot.TickerResponse, error)
	Depth(context.Context, string, int) (*asterspot.DepthResponse, error)
	GetAccount(context.Context) (*asterspot.AccountResponse, error)
	PlaceOrder(context.Context, asterspot.PlaceOrderParams) (*asterspot.OrderResponse, error)
	CancelOrder(context.Context, string, int64, string) (*asterspot.CancelOrderResponse, error)
	GetOpenOrders(context.Context, string) ([]asterspot.OrderResponse, error)
}

type spotProvider struct {
	sdk      spotSDK
	insts    map[model.InstrumentID]model.Instrument
	rawIndex map[string]model.InstrumentID
}

func newSpotProvider(sdk spotSDK) *spotProvider {
	return &spotProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), rawIndex: make(map[string]model.InstrumentID)}
}

func (p *spotProvider) LoadAll(ctx context.Context) error {
	info, err := p.sdk.ExchangeInfo(ctx)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.rawIndex = make(map[string]model.InstrumentID)
	for _, symbol := range info.Symbols {
		inst := model.Instrument{
			ID:        model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-SPOT", symbol.BaseAsset, symbol.QuoteAsset), Venue: Venue},
			RawSymbol: symbol.Symbol,
			Type:      model.InstrumentTypeSpot,
			Base:      model.Currency(symbol.BaseAsset),
			Quote:     model.Currency(symbol.QuoteAsset),
			PriceTick: decimalFromFilter(symbol.Filters, "PRICE_FILTER", "tickSize", "0.00000001"),
			SizeTick:  decimalFromFilter(symbol.Filters, "LOT_SIZE", "stepSize", "0.00000001"),
			Status:    mapTradingStatus(symbol.Status),
		}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.rawIndex[inst.RawSymbol] = inst.ID
	}
	return nil
}

func (p *spotProvider) Get(id model.InstrumentID) (model.Instrument, bool) {
	inst, ok := p.insts[id]
	return inst, ok
}

func (p *spotProvider) List() []model.Instrument {
	out := make([]model.Instrument, 0, len(p.insts))
	for _, inst := range p.insts {
		out = append(out, inst)
	}
	return out
}
func (p *spotProvider) ensureLoaded(ctx context.Context) error {
	if len(p.insts) > 0 {
		return nil
	}
	return p.LoadAll(ctx)
}

func (p *spotProvider) rawSymbol(id model.InstrumentID) (string, error) {
	inst, ok := p.Get(id)
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return inst.RawSymbol, nil
}
func (p *spotProvider) instrumentIDByRaw(raw string) (model.InstrumentID, bool) {
	id, ok := p.rawIndex[raw]
	return id, ok
}

type spotDataClient struct {
	id       string
	provider *spotProvider
	sdk      spotSDK
	ws       spotMarketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	mu       sync.Mutex
	health   venue.DataHealth
}

func newSpotDataClient(id string, provider *spotProvider, sdk spotSDK) *spotDataClient {
	return &spotDataClient{id: id, provider: provider, sdk: sdk, ws: asterspot.NewWsMarketClient(context.Background()), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData)}
}

func (c *spotDataClient) Venue() model.Venue                    { return Venue }
func (c *spotDataClient) ClientID() string                      { return c.id }
func (c *spotDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *spotDataClient) Connect(ctx context.Context) error {
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
func (c *spotDataClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.ws != nil {
		c.ws.Close()
	}
	return nil
}
func (c *spotDataClient) Health() venue.DataHealth { return c.health }
func (c *spotDataClient) Events() <-chan model.MarketEvent {
	return c.events
}
func (c *spotDataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.Ticker{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.Ticker{}, err
	}
	t, err := c.sdk.Ticker(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	return model.Ticker{
		InstrumentID: id,
		Bid:          decimal.RequireFromString(defaultString(t.BidPrice, "0")),
		Ask:          decimal.RequireFromString(defaultString(t.AskPrice, "0")),
		Last:         decimal.RequireFromString(defaultString(t.LastPrice, "0")),
		Timestamp:    parseAsterTime(t.CloseTime),
	}, nil
}
func (c *spotDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderBook{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	depth, err := c.sdk.Depth(ctx, raw, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, level := range depth.Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range depth.Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
	}
	return book, book.Validate()
}
func (c *spotDataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
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
	if err := c.ws.Connect(); err != nil {
		c.health.LastError = err
		return err
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		if !c.hasBookTickerSubscription(sub.InstrumentID) {
			err = c.ws.SubscribeBookTicker(raw, func(event *asterspot.BookTickerEvent) error {
				return c.handleBookTicker(sub.InstrumentID, event)
			})
		}
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeLimitOrderBook(raw, asterBookDepth(sub.Depth), "100ms", func(event *asterspot.DepthEvent) error {
			return c.handleDepth(sub.InstrumentID, event)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeAggTrade(raw, func(event *asterspot.AggTradeEvent) error {
			return c.handleAggTrade(sub.InstrumentID, event)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeKline(raw, asterBarInterval(barType.Step), func(event *asterspot.KlineEvent) error {
			return c.handleKline(barType, event)
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
	c.mu.Unlock()
	c.health.Connected = true
	c.health.LastError = nil
	return nil
}
func (c *spotDataClient) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
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
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		c.mu.Lock()
		delete(c.subs, sub.Key())
		stillActive := c.hasBookTickerSubscriptionLocked(sub.InstrumentID)
		c.mu.Unlock()
		if stillActive {
			return nil
		}
		err = c.ws.Unsubscribe(fmt.Sprintf("%s@bookTicker", strings.ToLower(raw)))
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeLimitOrderBook(raw, asterBookDepth(sub.Depth), "100ms")
	case model.MarketDataTypeTradeTick:
		err = c.ws.Unsubscribe(fmt.Sprintf("%s@aggTrade", strings.ToLower(raw)))
	case model.MarketDataTypeBar:
		err = c.ws.Unsubscribe(fmt.Sprintf("%s@kline_%s", strings.ToLower(raw), asterBarInterval(sub.BarType.Canonical().Step)))
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	c.mu.Lock()
	delete(c.subs, sub.Key())
	c.mu.Unlock()
	return nil
}

func (c *spotDataClient) hasBookTickerSubscription(id model.InstrumentID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hasBookTickerSubscriptionLocked(id)
}

func (c *spotDataClient) hasBookTickerSubscriptionLocked(id model.InstrumentID) bool {
	tickerKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()
	quoteKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()
	_, hasTicker := c.subs[tickerKey]
	_, hasQuote := c.subs[quoteKey]
	return hasTicker || hasQuote
}

func (c *spotDataClient) handleBookTicker(id model.InstrumentID, event *asterspot.BookTickerEvent) error {
	if event == nil {
		return nil
	}
	bid := decimal.RequireFromString(defaultString(event.BestBidPrice, "0"))
	ask := decimal.RequireFromString(defaultString(event.BestAskPrice, "0"))
	last := bid
	if bid.IsPositive() && ask.IsPositive() {
		last = bid.Add(ask).Div(decimal.NewFromInt(2))
	} else if ask.IsPositive() {
		last = ask
	}
	c.mu.Lock()
	tickerKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()
	quoteKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()
	_, emitTicker := c.subs[tickerKey]
	_, emitQuote := c.subs[quoteKey]
	c.mu.Unlock()
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: time.Now()}}); err != nil {
			return err
		}
	}
	if emitQuote {
		return c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     bid,
			AskPrice:     ask,
			BidSize:      decimal.RequireFromString(defaultString(event.BestBidQty, "0")),
			AskSize:      decimal.RequireFromString(defaultString(event.BestAskQty, "0")),
			Timestamp:    time.Now(),
			InitTime:     time.Now(),
		}})
	}
	return nil
}

func (c *spotDataClient) handleAggTrade(id model.InstrumentID, event *asterspot.AggTradeEvent) error {
	if event == nil {
		return nil
	}
	return c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
		InstrumentID:  id,
		Price:         decimal.RequireFromString(defaultString(event.Price, "0")),
		Size:          decimal.RequireFromString(defaultString(event.Quantity, "0")),
		AggressorSide: asterAggressorSide(event.IsBuyerMaker),
		TradeID:       model.TradeID(int64String(event.AggTradeID)),
		Timestamp:     parseAsterTime(event.TradeTime),
		InitTime:      parseAsterTime(event.EventTime),
	}})
}

func (c *spotDataClient) handleKline(barType model.BarType, event *asterspot.KlineEvent) error {
	if event == nil {
		return nil
	}
	return c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:   barType.Canonical(),
		Open:      decimal.RequireFromString(defaultString(event.Kline.OpenPrice, "0")),
		High:      decimal.RequireFromString(defaultString(event.Kline.HighPrice, "0")),
		Low:       decimal.RequireFromString(defaultString(event.Kline.LowPrice, "0")),
		Close:     decimal.RequireFromString(defaultString(event.Kline.ClosePrice, "0")),
		Volume:    decimal.RequireFromString(defaultString(event.Kline.Volume, "0")),
		Timestamp: parseAsterTime(firstPositiveInt64(event.Kline.CloseTime, event.EventTime)),
		InitTime:  parseAsterTime(event.EventTime),
	}})
}

func (c *spotDataClient) handleDepth(id model.InstrumentID, event *asterspot.DepthEvent) error {
	if event == nil {
		return nil
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseAsterTime(event.EventTime)}
	for _, bid := range event.Bids {
		book.Bids = append(book.Bids, parseBookLevel(bid))
	}
	for _, ask := range event.Asks {
		book.Asks = append(book.Asks, parseBookLevel(ask))
	}
	return c.emitMarket(model.MarketEvent{OrderBook: &book})
}
func (c *spotDataClient) emitMarket(event model.MarketEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: aster spot market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

type spotExecutionClient struct {
	accountID  model.AccountID
	provider   *spotProvider
	sdk        spotSDK
	privateWS  spotAccountWS
	events     chan model.ExecutionEvent
	mu         sync.Mutex
	registered bool
	health     venue.ExecutionHealth
}

func newSpotExecutionClient(accountID model.AccountID, provider *spotProvider, sdk spotSDK) *spotExecutionClient {
	if accountID == "" {
		accountID = "aster-spot"
	}
	return &spotExecutionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 64)}
}

func (c *spotExecutionClient) Venue() model.Venue         { return Venue }
func (c *spotExecutionClient) AccountID() model.AccountID { return c.accountID }
func (c *spotExecutionClient) Connect(ctx context.Context) error {
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
func (c *spotExecutionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.privateWS != nil {
		c.privateWS.Close()
	}
	return nil
}
func (c *spotExecutionClient) Health() venue.ExecutionHealth       { return c.health }
func (c *spotExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
func (c *spotExecutionClient) ResubscribeExecution(ctx context.Context) error {
	if c.privateWS == nil {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()
	return c.subscribePrivate(ctx)
}
func (c *spotExecutionClient) subscribePrivate(context.Context) error {
	c.mu.Lock()
	if c.registered {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := c.privateWS.Connect(); err != nil {
		return err
	}
	c.privateWS.SubscribeExecutionReport(c.handleExecutionReport)
	c.privateWS.SubscribeAccountPosition(c.handleAccountPosition)
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}
func (c *spotExecutionClient) handleExecutionReport(event *asterspot.ExecutionReportEvent) {
	if event == nil {
		return
	}
	id, ok := c.provider.instrumentIDByRaw(event.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: aster spot symbol %s", model.ErrInstrumentNotFound, event.Symbol)
		return
	}
	order := c.executionOrderReport(id, event)
	_ = c.emitExecution(model.ExecutionEvent{Order: &order})
	if decimal.RequireFromString(defaultString(event.LastExecutedQuantity, "0")).IsPositive() && event.TradeID > 0 {
		fill := c.executionFillReport(id, event)
		_ = c.emitExecution(model.ExecutionEvent{Fill: &fill})
	}
}
func (c *spotExecutionClient) handleAccountPosition(event *asterspot.AccountPositionEvent) {
	if event == nil {
		return
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: parseAsterTime(event.EventTime)}
	for _, bal := range event.Balances {
		free := decimal.RequireFromString(defaultString(bal.Free, "0"))
		locked := decimal.RequireFromString(defaultString(bal.Locked, "0"))
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(bal.Asset), Free: free.String(), Locked: locked.String(), Total: free.Add(locked).String()})
	}
	_ = c.emitExecution(model.ExecutionEvent{Account: &snapshot})
}
func (c *spotExecutionClient) executionOrderReport(id model.InstrumentID, event *asterspot.ExecutionReportEvent) model.OrderStatusReport {
	quantity := decimal.RequireFromString(defaultString(event.Quantity, "0"))
	filled := decimal.RequireFromString(defaultString(event.CumulativeFilledQuantity, "0"))
	status := mapOrderStatus(event.OrderStatus)
	if status == model.OrderStatusAccepted && filled.IsPositive() {
		status = model.OrderStatusPartiallyFilled
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(int64String(event.OrderID)),
		ClientOrderID:   model.ClientOrderID(event.ClientOrderID),
		Status:          status,
		Side:            fromVenueSide(event.Side),
		Type:            fromVenueType(event.OrderType),
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leavesQuantity(quantity, filled),
		Price:           decimal.RequireFromString(defaultString(event.Price, "0")),
		AveragePrice:    decimal.RequireFromString(defaultString(event.LastExecutedPrice, "0")),
		LastUpdatedTime: parseAsterTime(firstPositiveInt64(event.TransactionTime, event.EventTime)),
	}
}
func (c *spotExecutionClient) executionFillReport(id model.InstrumentID, event *asterspot.ExecutionReportEvent) model.FillReport {
	inst, _ := c.provider.Get(id)
	feeCurrency := model.Currency(event.CommissionAsset)
	if feeCurrency == "" {
		feeCurrency = inst.Quote
	}
	return model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       model.OrderID(int64String(event.OrderID)),
		ClientOrderID: model.ClientOrderID(event.ClientOrderID),
		TradeID:       model.TradeID(int64String(event.TradeID)),
		Side:          fromVenueSide(event.Side),
		Price:         decimal.RequireFromString(defaultString(event.LastExecutedPrice, "0")),
		Quantity:      decimal.RequireFromString(defaultString(event.LastExecutedQuantity, "0")),
		Fee:           decimal.RequireFromString(defaultString(event.CommissionAmount, "0")).Abs(),
		FeeCurrency:   feeCurrency,
		Timestamp:     parseAsterTime(firstPositiveInt64(event.TransactionTime, event.EventTime)),
	}
}
func (c *spotExecutionClient) emitExecution(event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: aster spot execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}
func (c *spotExecutionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.AccountSnapshot{}, err
	}
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, bal := range account.Balances {
		free := decimal.RequireFromString(defaultString(bal.Free, "0"))
		locked := decimal.RequireFromString(defaultString(bal.Locked, "0"))
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(bal.Asset), Free: free.String(), Locked: locked.String(), Total: free.Add(locked).String()})
	}
	return snapshot, nil
}
func (c *spotExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
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
	resp, err := c.sdk.PlaceOrder(ctx, asterspot.PlaceOrderParams{Symbol: raw, Side: toVenueSide(cmd.Side), Type: toVenueType(cmd.Type), TimeInForce: toVenueTIF(cmd.TimeInForce), Quantity: cmd.Quantity.String(), Price: zeroBlank(cmd.Price), NewClientOrderID: string(cmd.ClientOrderID)})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapOrderResponse(cmd.InstrumentID, *resp), nil
}
func (c *spotExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	orderID, _ := strconv.ParseInt(string(cmd.OrderID), 10, 64)
	resp, err := c.sdk.CancelOrder(ctx, raw, orderID, string(cmd.ClientOrderID))
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: model.OrderID(strconv.FormatInt(resp.OrderID, 10)), Status: mapOrderStatus(resp.Status)}, nil
}
func (c *spotExecutionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetOpenOrders(ctx, raw)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		reports = append(reports, c.mapOrderResponse(id, order))
	}
	return reports, nil
}
func (c *spotExecutionClient) mapOrderResponse(id model.InstrumentID, resp asterspot.OrderResponse) model.OrderStatusReport {
	quantity := decimal.RequireFromString(defaultString(resp.OrigQty, "0"))
	filled := decimal.RequireFromString(defaultString(resp.ExecutedQty, "0"))
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(strconv.FormatInt(resp.OrderID, 10)), ClientOrderID: model.ClientOrderID(resp.ClientOrderID), Status: mapOrderStatus(resp.Status), Side: fromVenueSide(resp.Side), Type: fromVenueType(resp.Type), Quantity: quantity, FilledQuantity: filled, LeavesQuantity: leavesQuantity(quantity, filled), Price: decimal.RequireFromString(defaultString(resp.Price, "0")), LastUpdatedTime: time.Now()}
}

type spotMarketWS interface {
	Connect() error
	SubscribeBookTicker(string, func(*asterspot.BookTickerEvent) error) error
	SubscribeLimitOrderBook(string, int, string, func(*asterspot.DepthEvent) error) error
	SubscribeAggTrade(string, func(*asterspot.AggTradeEvent) error) error
	SubscribeKline(string, string, func(*asterspot.KlineEvent) error) error
	Unsubscribe(string) error
	UnsubscribeLimitOrderBook(string, int, string) error
	Close()
}

type spotAccountWS interface {
	Connect() error
	SubscribeExecutionReport(func(*asterspot.ExecutionReportEvent))
	SubscribeAccountPosition(func(*asterspot.AccountPositionEvent))
	Close()
}
