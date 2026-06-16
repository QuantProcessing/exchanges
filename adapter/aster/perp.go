package aster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	asterperp "github.com/QuantProcessing/exchanges/sdk/aster/perp"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type perpSDK interface {
	ExchangeInfo(context.Context) (*asterperp.ExchangeInfoResponse, error)
	Ticker(context.Context, string) (*asterperp.TickerResponse, error)
	Depth(context.Context, string, int) (*asterperp.DepthResponse, error)
	GetFundingRate(context.Context, string) (*asterperp.FundingRateData, error)
	GetAccount(context.Context) (*asterperp.AccountResponse, error)
	PlaceOrder(context.Context, asterperp.PlaceOrderParams) (*asterperp.OrderResponse, error)
	CancelOrder(context.Context, asterperp.CancelOrderParams) (*asterperp.OrderResponse, error)
	GetOpenOrders(context.Context, string) ([]asterperp.OrderResponse, error)
}

type perpProvider struct {
	sdk      perpSDK
	insts    map[model.InstrumentID]model.Instrument
	rawIndex map[string]model.InstrumentID
}

func newPerpProvider(sdk perpSDK) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), rawIndex: make(map[string]model.InstrumentID)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	info, err := p.sdk.ExchangeInfo(ctx)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.rawIndex = make(map[string]model.InstrumentID)
	for _, symbol := range info.Symbols {
		inst := model.Instrument{ID: model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", symbol.BaseAsset, symbol.QuoteAsset), Venue: Venue}, RawSymbol: symbol.Symbol, Type: model.InstrumentTypePerp, Base: model.Currency(symbol.BaseAsset), Quote: model.Currency(symbol.QuoteAsset), Settle: model.Currency(defaultString(symbol.MarginAsset, symbol.QuoteAsset)), PriceTick: decimalFromFilter(symbol.Filters, "PRICE_FILTER", "tickSize", "0.00000001"), SizeTick: decimalFromFilter(symbol.Filters, "LOT_SIZE", "stepSize", "0.00000001"), Status: mapTradingStatus(symbol.Status)}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.rawIndex[inst.RawSymbol] = inst.ID
	}
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
	inst, ok := p.Get(id)
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return inst.RawSymbol, nil
}
func (p *perpProvider) instrumentIDByRaw(raw string) (model.InstrumentID, bool) {
	id, ok := p.rawIndex[raw]
	return id, ok
}

type perpDataClient struct {
	id       string
	provider *perpProvider
	sdk      perpSDK
	ws       perpMarketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	mu       sync.Mutex
	health   venue.DataHealth
}

func newPerpDataClient(id string, provider *perpProvider, sdk perpSDK) *perpDataClient {
	return &perpDataClient{id: id, provider: provider, sdk: sdk, ws: asterperp.NewWsMarketClient(context.Background()), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData)}
}
func (c *perpDataClient) Venue() model.Venue                    { return Venue }
func (c *perpDataClient) ClientID() string                      { return c.id }
func (c *perpDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *perpDataClient) Connect(ctx context.Context) error {
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
func (c *perpDataClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.ws != nil {
		c.ws.Close()
	}
	return nil
}
func (c *perpDataClient) Health() venue.DataHealth { return c.health }
func (c *perpDataClient) Events() <-chan model.MarketEvent {
	return c.events
}
func (c *perpDataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
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
	return model.Ticker{InstrumentID: id, Last: decimal.RequireFromString(defaultString(t.LastPrice, "0")), Timestamp: parseAsterTime(t.CloseTime)}, nil
}
func (c *perpDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
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
	book := model.OrderBook{InstrumentID: id, Timestamp: parseAsterTime(firstPositiveInt64(depth.E, depth.T))}
	for _, level := range depth.Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range depth.Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
	}
	return book, book.Validate()
}
func (c *perpDataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.FundingRate{}, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	resp, err := c.sdk.GetFundingRate(ctx, raw)
	if err != nil {
		return model.FundingRate{}, err
	}
	timestamp := time.Now()
	if resp.Time > 0 {
		timestamp = parseAsterTime(resp.Time)
	} else if resp.FundingTime > 0 {
		timestamp = parseAsterTime(resp.FundingTime)
	}
	funding := model.FundingRate{
		InstrumentID:    id,
		Rate:            decimal.RequireFromString(defaultString(resp.LastFundingRate, "0")),
		MarkPrice:       decimal.RequireFromString(defaultString(resp.MarkPrice, "0")),
		IndexPrice:      decimal.RequireFromString(defaultString(resp.IndexPrice, "0")),
		NextFundingTime: parseAsterTime(resp.NextFundingTime),
		FundingInterval: time.Duration(resp.FundingIntervalHours) * time.Hour,
		Timestamp:       timestamp,
		InitTime:        time.Now(),
	}
	return funding, funding.Validate()
}
func (c *perpDataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
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
	streamSymbol := strings.ToLower(raw)
	if err := c.ws.Connect(); err != nil {
		c.health.LastError = err
		return err
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		if !c.hasBookTickerSubscription(sub.InstrumentID) {
			err = c.ws.SubscribeBookTicker(streamSymbol, func(event *asterperp.WsBookTickerEvent) error {
				return c.handleBookTicker(sub.InstrumentID, event)
			})
		}
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeLimitOrderBook(streamSymbol, asterBookDepth(sub.Depth), "100ms", func(event *asterperp.WsDepthEvent) error {
			return c.handleDepth(sub.InstrumentID, event)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeAggTrade(streamSymbol, func(event *asterperp.WsAggTradeEvent) error {
			return c.handleAggTrade(sub.InstrumentID, event)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeKline(streamSymbol, asterBarInterval(barType.Step), func(event *asterperp.WsKlineEvent) error {
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
func (c *perpDataClient) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
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
	streamSymbol := strings.ToLower(raw)
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		c.mu.Lock()
		delete(c.subs, sub.Key())
		stillActive := c.hasBookTickerSubscriptionLocked(sub.InstrumentID)
		c.mu.Unlock()
		if stillActive {
			return nil
		}
		err = c.ws.UnsubscribeBookTicker(streamSymbol)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeLimitOrderBook(streamSymbol, asterBookDepth(sub.Depth), "100ms")
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeAggTrade(streamSymbol)
	case model.MarketDataTypeBar:
		err = c.ws.UnsubscribeKline(streamSymbol, asterBarInterval(sub.BarType.Canonical().Step))
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

func (c *perpDataClient) hasBookTickerSubscription(id model.InstrumentID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hasBookTickerSubscriptionLocked(id)
}

func (c *perpDataClient) hasBookTickerSubscriptionLocked(id model.InstrumentID) bool {
	tickerKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()
	quoteKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()
	_, hasTicker := c.subs[tickerKey]
	_, hasQuote := c.subs[quoteKey]
	return hasTicker || hasQuote
}

func (c *perpDataClient) handleBookTicker(id model.InstrumentID, event *asterperp.WsBookTickerEvent) error {
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
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: parseAsterTime(event.EventTime)}}); err != nil {
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
			Timestamp:    parseAsterTime(event.EventTime),
			InitTime:     parseAsterTime(event.EventTime),
		}})
	}
	return nil
}

func (c *perpDataClient) handleAggTrade(id model.InstrumentID, event *asterperp.WsAggTradeEvent) error {
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

func (c *perpDataClient) handleKline(barType model.BarType, event *asterperp.WsKlineEvent) error {
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
		Timestamp: parseAsterTime(firstPositiveInt64(event.Kline.EndTime, event.EventTime)),
		InitTime:  parseAsterTime(event.EventTime),
	}})
}

func (c *perpDataClient) handleDepth(id model.InstrumentID, event *asterperp.WsDepthEvent) error {
	if event == nil {
		return nil
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseAsterTime(event.EventTime)}
	for _, bid := range event.Bids {
		book.Bids = append(book.Bids, parseBookLevelAny(bid))
	}
	for _, ask := range event.Asks {
		book.Asks = append(book.Asks, parseBookLevelAny(ask))
	}
	return c.emitMarket(model.MarketEvent{OrderBook: &book})
}
func (c *perpDataClient) emitMarket(event model.MarketEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: aster perp market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

type perpExecutionClient struct {
	accountID  model.AccountID
	provider   *perpProvider
	sdk        perpSDK
	privateWS  perpAccountWS
	events     chan model.ExecutionEvent
	mu         sync.Mutex
	registered bool
	health     venue.ExecutionHealth
}

func newPerpExecutionClient(accountID model.AccountID, provider *perpProvider, sdk perpSDK) *perpExecutionClient {
	if accountID == "" {
		accountID = "aster-perp"
	}
	return &perpExecutionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 64)}
}
func (c *perpExecutionClient) Venue() model.Venue         { return Venue }
func (c *perpExecutionClient) AccountID() model.AccountID { return c.accountID }
func (c *perpExecutionClient) Connect(ctx context.Context) error {
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
func (c *perpExecutionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.privateWS != nil {
		c.privateWS.Close()
	}
	return nil
}
func (c *perpExecutionClient) Health() venue.ExecutionHealth       { return c.health }
func (c *perpExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
func (c *perpExecutionClient) ResubscribeExecution(ctx context.Context) error {
	if c.privateWS == nil {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()
	return c.subscribePrivate(ctx)
}
func (c *perpExecutionClient) subscribePrivate(context.Context) error {
	c.mu.Lock()
	if c.registered {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := c.privateWS.Connect(); err != nil {
		return err
	}
	c.privateWS.SubscribeOrderUpdate(c.handleOrderUpdate)
	c.privateWS.SubscribeAccountUpdate(c.handleAccountUpdate)
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}
func (c *perpExecutionClient) handleOrderUpdate(event *asterperp.OrderUpdateEvent) {
	if event == nil {
		return
	}
	id, ok := c.provider.instrumentIDByRaw(event.Order.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: aster perp symbol %s", model.ErrInstrumentNotFound, event.Order.Symbol)
		return
	}
	order := c.orderUpdateReport(id, event)
	_ = c.emitExecution(model.ExecutionEvent{Order: &order})
	if decimal.RequireFromString(defaultString(event.Order.LastFilledQty, "0")).IsPositive() && event.Order.TradeID > 0 {
		fill := c.fillReport(id, event)
		_ = c.emitExecution(model.ExecutionEvent{Fill: &fill})
	}
}
func (c *perpExecutionClient) handleAccountUpdate(event *asterperp.AccountUpdateEvent) {
	if event == nil {
		return
	}
	for _, position := range event.UpdateData.Positions {
		report, err := c.positionReport(position.Symbol, position.PositionAmount, position.EntryPrice, firstPositiveInt64(event.TransactionTime, event.EventTime))
		if err != nil {
			c.health.LastError = err
			continue
		}
		_ = c.emitExecution(model.ExecutionEvent{Position: &report})
	}
	if len(event.UpdateData.Balances) > 0 {
		snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: parseAsterTime(firstPositiveInt64(event.TransactionTime, event.EventTime))}
		for _, bal := range event.UpdateData.Balances {
			total := decimal.RequireFromString(defaultString(bal.WalletBalance, "0"))
			snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(bal.Asset), Free: total.String(), Total: total.String()})
		}
		_ = c.emitExecution(model.ExecutionEvent{Account: &snapshot})
	}
}
func (c *perpExecutionClient) orderUpdateReport(id model.InstrumentID, event *asterperp.OrderUpdateEvent) model.OrderStatusReport {
	order := event.Order
	quantity := decimal.RequireFromString(defaultString(order.OriginalQty, "0"))
	filled := decimal.RequireFromString(defaultString(order.AccumulatedFilledQty, "0"))
	status := mapOrderStatus(order.OrderStatus)
	if status == model.OrderStatusAccepted && filled.IsPositive() {
		status = model.OrderStatusPartiallyFilled
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(int64String(order.OrderID)),
		ClientOrderID:   model.ClientOrderID(order.ClientOrderID),
		Status:          status,
		Side:            fromVenueSide(order.Side),
		Type:            fromVenueType(order.OrderType),
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leavesQuantity(quantity, filled),
		Price:           decimal.RequireFromString(defaultString(order.OriginalPrice, "0")),
		AveragePrice:    decimal.RequireFromString(defaultString(firstNonEmpty(order.AveragePrice, order.LastFilledPrice), "0")),
		LastUpdatedTime: parseAsterTime(firstPositiveInt64(event.TransactionTime, event.EventTime)),
	}
}
func (c *perpExecutionClient) fillReport(id model.InstrumentID, event *asterperp.OrderUpdateEvent) model.FillReport {
	inst, _ := c.provider.Get(id)
	order := event.Order
	feeCurrency := model.Currency(order.CommissionAsset)
	if feeCurrency == "" {
		feeCurrency = inst.Settle
	}
	return model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       model.OrderID(int64String(order.OrderID)),
		ClientOrderID: model.ClientOrderID(order.ClientOrderID),
		TradeID:       model.TradeID(int64String(order.TradeID)),
		Side:          fromVenueSide(order.Side),
		Price:         decimal.RequireFromString(defaultString(order.LastFilledPrice, "0")),
		Quantity:      decimal.RequireFromString(defaultString(order.LastFilledQty, "0")),
		Fee:           decimal.RequireFromString(defaultString(order.Commission, "0")).Abs(),
		FeeCurrency:   feeCurrency,
		Timestamp:     parseAsterTime(firstPositiveInt64(order.TradeTime, event.TransactionTime, event.EventTime)),
	}
}
func (c *perpExecutionClient) positionReport(rawSymbol, rawQty, rawEntry string, rawTime int64) (model.PositionStatusReport, error) {
	id, ok := c.provider.instrumentIDByRaw(rawSymbol)
	if !ok {
		return model.PositionStatusReport{}, fmt.Errorf("%w: aster perp symbol %s", model.ErrInstrumentNotFound, rawSymbol)
	}
	qty := decimal.RequireFromString(defaultString(rawQty, "0"))
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(id.String()),
		Side:         positionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   decimal.RequireFromString(defaultString(rawEntry, "0")),
		Timestamp:    parseAsterTime(rawTime),
	}, nil
}
func (c *perpExecutionClient) emitExecution(event model.ExecutionEvent) error {
	if err := event.Validate(); err != nil {
		c.health.LastError = err
		return err
	}
	c.health.LastEventTime = time.Now()
	select {
	case c.events <- event:
		return nil
	default:
		err := fmt.Errorf("%w: aster perp execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}
func (c *perpExecutionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.AccountSnapshot{}, err
	}
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, asset := range account.Assets {
		total := decimal.RequireFromString(defaultString(asset.WalletBalance, "0"))
		free := decimal.RequireFromString(defaultString(asset.AvailableBalance, total.String()))
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(asset.Asset), Free: free.String(), Locked: total.Sub(free).String(), Total: total.String()})
	}
	return snapshot, nil
}
func (c *perpExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
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
	resp, err := c.sdk.PlaceOrder(ctx, asterperp.PlaceOrderParams{Symbol: raw, Side: toVenueSide(cmd.Side), Type: asterperp.OrderType(toVenueType(cmd.Type)), TimeInForce: asterperp.TimeInForce(toVenueTIF(cmd.TimeInForce)), Quantity: cmd.Quantity.String(), Price: zeroBlank(cmd.Price), NewClientOrderID: string(cmd.ClientOrderID)})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapOrderResponse(cmd.InstrumentID, *resp), nil
}
func (c *perpExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.CancelOrder(ctx, asterperp.CancelOrderParams{Symbol: raw, OrderID: string(cmd.OrderID), OrigClientOrderID: string(cmd.ClientOrderID)})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapOrderResponse(cmd.InstrumentID, *resp), nil
}
func (c *perpExecutionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
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
func (c *perpExecutionClient) GeneratePositionStatusReports(ctx context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(account.Positions))
	for _, position := range account.Positions {
		if position.Symbol != raw {
			continue
		}
		report, err := c.positionReport(position.Symbol, position.PositionAmt, position.EntryPrice, position.UpdateTime)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
func (c *perpExecutionClient) mapOrderResponse(id model.InstrumentID, resp asterperp.OrderResponse) model.OrderStatusReport {
	quantity := decimal.RequireFromString(defaultString(resp.OrigQty, "0"))
	filled := decimal.RequireFromString(defaultString(resp.ExecutedQty, "0"))
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(strconv.FormatInt(resp.OrderID, 10)), ClientOrderID: model.ClientOrderID(resp.ClientOrderID), Status: mapOrderStatus(resp.Status), Side: fromVenueSide(resp.Side), Type: fromVenueType(resp.Type), Quantity: quantity, FilledQuantity: filled, LeavesQuantity: leavesQuantity(quantity, filled), Price: decimal.RequireFromString(defaultString(resp.Price, "0")), LastUpdatedTime: parseAsterTime(resp.UpdateTime)}
}

type perpMarketWS interface {
	Connect() error
	SubscribeBookTicker(string, func(*asterperp.WsBookTickerEvent) error) error
	SubscribeLimitOrderBook(string, int, string, func(*asterperp.WsDepthEvent) error) error
	SubscribeAggTrade(string, func(*asterperp.WsAggTradeEvent) error) error
	SubscribeKline(string, string, func(*asterperp.WsKlineEvent) error) error
	UnsubscribeBookTicker(string) error
	UnsubscribeLimitOrderBook(string, int, string) error
	UnsubscribeAggTrade(string) error
	UnsubscribeKline(string, string) error
	Close()
}

type perpAccountWS interface {
	Connect() error
	SubscribeOrderUpdate(func(*asterperp.OrderUpdateEvent))
	SubscribeAccountUpdate(func(*asterperp.AccountUpdateEvent))
	Close()
}
