package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type spotSDK interface {
	ExchangeInfo(context.Context) (*spot.ExchangeInfoResponse, error)
	Ticker(context.Context, string) (*spot.TickerResponse, error)
	Depth(context.Context, string, int) (*spot.DepthResponse, error)
	GetAccount(context.Context) (*spot.AccountResponse, error)
	PlaceOrder(context.Context, spot.PlaceOrderParams) (*spot.OrderResponse, error)
	CancelOrder(context.Context, string, int64, string) (*spot.CancelOrderResponse, error)
	GetOpenOrders(context.Context, string) ([]spot.OrderResponse, error)
}

type spotProvider struct {
	sdk   spotSDK
	insts map[model.InstrumentID]model.Instrument
	byRaw map[string]model.InstrumentID
}

func newSpotProvider(sdk spotSDK) *spotProvider {
	return &spotProvider{
		sdk:   sdk,
		insts: make(map[model.InstrumentID]model.Instrument),
		byRaw: make(map[string]model.InstrumentID),
	}
}

func (p *spotProvider) LoadAll(ctx context.Context) error {
	info, err := p.sdk.ExchangeInfo(ctx)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.byRaw = make(map[string]model.InstrumentID)
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
		p.byRaw[strings.ToUpper(inst.RawSymbol)] = inst.ID
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

func (p *spotProvider) rawSymbol(id model.InstrumentID) (string, error) {
	inst, ok := p.Get(id)
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return inst.RawSymbol, nil
}

func (p *spotProvider) instrumentIDByRaw(raw string) (model.InstrumentID, bool) {
	id, ok := p.byRaw[strings.ToUpper(raw)]
	return id, ok
}

func decimalFromFilter(filters []map[string]interface{}, filterType, key, fallback string) decimal.Decimal {
	for _, filter := range filters {
		if fmt.Sprint(filter["filterType"]) == filterType {
			return decimal.RequireFromString(fmt.Sprint(filter[key]))
		}
	}
	return decimal.RequireFromString(fallback)
}

func mapTradingStatus(raw string) model.InstrumentStatus {
	if strings.EqualFold(raw, "TRADING") {
		return model.InstrumentStatusTrading
	}
	return model.InstrumentStatusHalted
}

type spotDataClient struct {
	id       string
	provider *spotProvider
	sdk      spotSDK
	ws       spotMarketStream
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	mu       sync.Mutex
	health   venue.DataHealth
}

func newSpotDataClient(id string, provider *spotProvider, sdk spotSDK) *spotDataClient {
	return &spotDataClient{
		id:       id,
		provider: provider,
		sdk:      sdk,
		ws:       spot.NewWsMarketClient(context.Background()),
		events:   make(chan model.MarketEvent, 256),
		subs:     make(map[string]model.SubscribeMarketData),
	}
}

func (c *spotDataClient) Venue() model.Venue                    { return Venue }
func (c *spotDataClient) ClientID() string                      { return c.id }
func (c *spotDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *spotDataClient) Connect(ctx context.Context) error {
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			c.health.LastError = err
			return err
		}
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
		Timestamp:    binanceEventTime(t.CloseTime),
	}, nil
}

func (c *spotDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
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
	if err := book.Validate(); err != nil {
		return model.OrderBook{}, err
	}
	return book, nil
}

func (c *spotDataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	if err := c.ensureStreamConnected(); err != nil {
		c.health.LastError = err
		return err
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		if !c.hasBookTickerSubscription(sub.InstrumentID) {
			err = c.ws.SubscribeBookTicker(raw, func(event *spot.BookTickerEvent) error {
				return c.handleBookTicker(sub.InstrumentID, event)
			})
		}
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeLimitOrderBook(raw, sub.Depth, "100ms", func(event *spot.DepthEvent) error {
			book := model.OrderBook{InstrumentID: sub.InstrumentID, Timestamp: binanceEventTime(event.EventTime)}
			for _, level := range event.Bids {
				book.Bids = append(book.Bids, parseBookLevel(level))
			}
			for _, level := range event.Asks {
				book.Asks = append(book.Asks, parseBookLevel(level))
			}
			return c.emitMarket(model.MarketEvent{OrderBook: &book})
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeAggTrade(raw, func(event *spot.AggTradeEvent) error {
			return c.handleAggTrade(sub.InstrumentID, event)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		interval := binanceBarInterval(barType.Step)
		err = c.ws.SubscribeKline(raw, interval, func(event *spot.KlineEvent) error {
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
	return nil
}

func (c *spotDataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
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
		err = c.ws.UnsubscribeBookTicker(raw)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeLimitOrderBook(raw, sub.Depth, "100ms")
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeAggTrade(raw)
	case model.MarketDataTypeBar:
		err = c.ws.UnsubscribeKline(raw, binanceBarInterval(sub.BarType.Canonical().Step))
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

func (c *spotDataClient) handleBookTicker(id model.InstrumentID, event *spot.BookTickerEvent) error {
	c.mu.Lock()
	tickerKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()
	quoteKey := model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()
	_, emitTicker := c.subs[tickerKey]
	_, emitQuote := c.subs[quoteKey]
	c.mu.Unlock()
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{
			InstrumentID: id,
			Bid:          decimal.RequireFromString(defaultString(event.BestBidPrice, "0")),
			Ask:          decimal.RequireFromString(defaultString(event.BestAskPrice, "0")),
			Timestamp:    time.Now(),
		}}); err != nil {
			return err
		}
	}
	if emitQuote {
		return c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     decimal.RequireFromString(defaultString(event.BestBidPrice, "0")),
			AskPrice:     decimal.RequireFromString(defaultString(event.BestAskPrice, "0")),
			BidSize:      decimal.RequireFromString(defaultString(event.BestBidQty, "0")),
			AskSize:      decimal.RequireFromString(defaultString(event.BestAskQty, "0")),
			Timestamp:    time.Now(),
			InitTime:     time.Now(),
		}})
	}
	return nil
}

func (c *spotDataClient) handleAggTrade(id model.InstrumentID, event *spot.AggTradeEvent) error {
	return c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
		InstrumentID:  id,
		Price:         decimal.RequireFromString(defaultString(event.Price, "0")),
		Size:          decimal.RequireFromString(defaultString(event.Quantity, "0")),
		AggressorSide: binanceAggressorSide(event.IsBuyerMaker),
		TradeID:       model.TradeID(fmt.Sprintf("%d", event.AggTradeID)),
		Timestamp:     binanceEventTime(event.TradeTime),
		InitTime:      binanceEventTime(event.EventTime),
	}})
}

func (c *spotDataClient) handleKline(barType model.BarType, event *spot.KlineEvent) error {
	return c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:   barType.Canonical(),
		Open:      decimal.RequireFromString(defaultString(event.Kline.OpenPrice, "0")),
		High:      decimal.RequireFromString(defaultString(event.Kline.HighPrice, "0")),
		Low:       decimal.RequireFromString(defaultString(event.Kline.LowPrice, "0")),
		Close:     decimal.RequireFromString(defaultString(event.Kline.ClosePrice, "0")),
		Volume:    decimal.RequireFromString(defaultString(event.Kline.Volume, "0")),
		Timestamp: binanceEventTime(defaultInt64(event.Kline.CloseTime, event.EventTime)),
		InitTime:  binanceEventTime(event.EventTime),
	}})
}

func (c *spotDataClient) ensureStreamConnected() error {
	if c.ws == nil {
		c.ws = spot.NewWsMarketClient(context.Background())
	}
	if c.ws.IsConnected() {
		return nil
	}
	c.ws.SetPostReconnect(func() {
		c.mu.Lock()
		c.health.LastEventTime = time.Now()
		c.mu.Unlock()
	})
	return c.ws.Connect()
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
		err := fmt.Errorf("%w: binance spot market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func parseBookLevel(raw []string) model.OrderBookLevel {
	return model.OrderBookLevel{
		Price: decimal.RequireFromString(raw[0]),
		Size:  decimal.RequireFromString(raw[1]),
	}
}

func parseInterfaceBookLevel(raw []interface{}) model.OrderBookLevel {
	return model.OrderBookLevel{
		Price: decimal.RequireFromString(fmt.Sprint(raw[0])),
		Size:  decimal.RequireFromString(fmt.Sprint(raw[1])),
	}
}

func binanceEventTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}

type spotMarketStream interface {
	Connect() error
	Close()
	IsConnected() bool
	SetPostReconnect(func())
	SubscribeBookTicker(string, func(*spot.BookTickerEvent) error) error
	SubscribeLimitOrderBook(string, int, string, func(*spot.DepthEvent) error) error
	SubscribeAggTrade(string, func(*spot.AggTradeEvent) error) error
	SubscribeKline(string, string, func(*spot.KlineEvent) error) error
	UnsubscribeBookTicker(string) error
	UnsubscribeLimitOrderBook(string, int, string) error
	UnsubscribeAggTrade(string) error
	UnsubscribeKline(string, string) error
}

type spotExecutionClient struct {
	accountID         model.AccountID
	provider          *spotProvider
	sdk               spotSDK
	accountStream     spotAccountStream
	events            chan model.ExecutionEvent
	mu                sync.Mutex
	privateRegistered bool
	health            venue.ExecutionHealth
}

func newSpotExecutionClient(accountID model.AccountID, provider *spotProvider, sdk spotSDK, creds ...string) *spotExecutionClient {
	if accountID == "" {
		accountID = "binance-spot"
	}
	client := &spotExecutionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 2 && creds[0] != "" && creds[1] != "" {
		client.accountStream = spot.NewWsAccountClient(spot.NewWsAPIClient(context.Background()), creds[0], creds[1])
	}
	return client
}

func (c *spotExecutionClient) Venue() model.Venue         { return Venue }
func (c *spotExecutionClient) AccountID() model.AccountID { return c.accountID }
func (c *spotExecutionClient) Connect(context.Context) error {
	if c.accountStream != nil {
		c.registerPrivateHandlers()
		if err := c.accountStream.Connect(); err != nil {
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
	if c.accountStream != nil {
		c.accountStream.Close()
	}
	return nil
}
func (c *spotExecutionClient) Health() venue.ExecutionHealth       { return c.health }
func (c *spotExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }

func (c *spotExecutionClient) ResubscribeExecution(ctx context.Context) error {
	if c.accountStream == nil {
		return model.ErrNotSupported
	}
	return c.Connect(ctx)
}

func (c *spotExecutionClient) registerPrivateHandlers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.privateRegistered {
		return
	}
	c.accountStream.SubscribeExecutionReport(c.handleExecutionReport)
	c.accountStream.SubscribeAccountPosition(c.handleAccountPosition)
	c.privateRegistered = true
}

func (c *spotExecutionClient) handleExecutionReport(event *spot.ExecutionReportEvent) {
	id, ok := c.provider.instrumentIDByRaw(event.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, event.Symbol)
		return
	}
	order := model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(strconv.FormatInt(event.OrderID, 10)),
		ClientOrderID:   model.ClientOrderID(event.ClientOrderID),
		Status:          mapOrderStatus(event.OrderStatus),
		Side:            fromBinanceSide(event.Side),
		Type:            fromBinanceType(event.OrderType),
		Quantity:        decimal.RequireFromString(defaultString(event.Quantity, "0")),
		FilledQuantity:  decimal.RequireFromString(defaultString(event.CumulativeFilledQuantity, "0")),
		Price:           decimal.RequireFromString(defaultString(event.Price, "0")),
		LastUpdatedTime: binanceEventTime(event.TransactionTime),
	}
	if err := c.emitExecution(model.ExecutionEvent{Order: &order}); err != nil {
		return
	}
	lastQty := decimal.RequireFromString(defaultString(event.LastExecutedQuantity, "0"))
	if event.TradeID <= 0 || !lastQty.IsPositive() {
		return
	}
	fill := model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       model.TradeID(strconv.FormatInt(event.TradeID, 10)),
		Side:          order.Side,
		Price:         decimal.RequireFromString(defaultString(event.LastExecutedPrice, "0")),
		Quantity:      lastQty,
		Fee:           decimal.RequireFromString(defaultString(event.CommissionAmount, "0")),
		FeeCurrency:   model.Currency(event.CommissionAsset),
		Timestamp:     binanceEventTime(event.TransactionTime),
	}
	_ = c.emitExecution(model.ExecutionEvent{Fill: &fill})
}

func (c *spotExecutionClient) handleAccountPosition(event *spot.AccountPositionEvent) {
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: binanceEventTime(event.EventTime)}
	for _, bal := range event.Balances {
		free := decimal.RequireFromString(defaultString(bal.Free, "0"))
		locked := decimal.RequireFromString(defaultString(bal.Locked, "0"))
		snapshot.Balances = append(snapshot.Balances, model.Balance{
			Currency: model.Currency(bal.Asset),
			Free:     free.String(),
			Locked:   locked.String(),
			Total:    free.Add(locked).String(),
		})
	}
	_ = c.emitExecution(model.ExecutionEvent{Account: &snapshot})
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
		err := fmt.Errorf("%w: binance spot execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *spotExecutionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, bal := range account.Balances {
		free := decimal.RequireFromString(defaultString(bal.Free, "0"))
		locked := decimal.RequireFromString(defaultString(bal.Locked, "0"))
		snapshot.Balances = append(snapshot.Balances, model.Balance{
			Currency: model.Currency(bal.Asset),
			Free:     free.String(),
			Locked:   locked.String(),
			Total:    free.Add(locked).String(),
		})
	}
	return snapshot, nil
}

func (c *spotExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.PlaceOrder(ctx, spot.PlaceOrderParams{
		Symbol:           raw,
		Side:             toBinanceSide(cmd.Side),
		Type:             toBinanceType(cmd.Type),
		TimeInForce:      toBinanceTIF(cmd.TimeInForce),
		Quantity:         cmd.Quantity.String(),
		Price:            zeroBlank(cmd.Price),
		NewClientOrderID: string(cmd.ClientOrderID),
	})
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
	return model.OrderStatusReport{
		AccountID:    c.accountID,
		InstrumentID: cmd.InstrumentID,
		OrderID:      model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		Status:       mapOrderStatus(resp.Status),
	}, nil
}

func (c *spotExecutionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
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

func (c *spotExecutionClient) mapOrderResponse(id model.InstrumentID, resp spot.OrderResponse) model.OrderStatusReport {
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(strconv.FormatInt(resp.OrderID, 10)),
		ClientOrderID:   model.ClientOrderID(resp.ClientOrderID),
		Status:          mapOrderStatus(resp.Status),
		Side:            fromBinanceSide(resp.Side),
		Type:            fromBinanceType(resp.Type),
		Quantity:        decimal.RequireFromString(defaultString(resp.OrigQty, "0")),
		FilledQuantity:  decimal.RequireFromString(defaultString(resp.ExecutedQty, "0")),
		Price:           decimal.RequireFromString(defaultString(resp.Price, "0")),
		LastUpdatedTime: time.Now(),
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func zeroBlank(value decimal.Decimal) string {
	if value.IsZero() {
		return ""
	}
	return value.String()
}

func toBinanceSide(side model.OrderSide) string {
	if side == model.OrderSideSell {
		return "SELL"
	}
	return "BUY"
}

func fromBinanceSide(side string) model.OrderSide {
	if strings.EqualFold(side, "SELL") {
		return model.OrderSideSell
	}
	if strings.EqualFold(side, "BUY") {
		return model.OrderSideBuy
	}
	return ""
}

func toBinanceType(orderType model.OrderType) string {
	if orderType == model.OrderTypeMarket {
		return "MARKET"
	}
	return "LIMIT"
}

func fromBinanceType(orderType string) model.OrderType {
	if strings.EqualFold(orderType, "MARKET") {
		return model.OrderTypeMarket
	}
	if strings.EqualFold(orderType, "LIMIT") {
		return model.OrderTypeLimit
	}
	return ""
}

func toBinanceTIF(tif model.TimeInForce) string {
	switch tif {
	case model.TimeInForceIOC:
		return "IOC"
	case model.TimeInForceFOK:
		return "FOK"
	default:
		return "GTC"
	}
}

func mapOrderStatus(status string) model.OrderStatus {
	switch strings.ToUpper(status) {
	case "FILLED":
		return model.OrderStatusFilled
	case "PARTIALLY_FILLED":
		return model.OrderStatusPartiallyFilled
	case "CANCELED":
		return model.OrderStatusCanceled
	case "REJECTED", "EXPIRED":
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

type spotAccountStream interface {
	Connect() error
	Close()
	SubscribeExecutionReport(func(*spot.ExecutionReportEvent))
	SubscribeAccountPosition(func(*spot.AccountPositionEvent))
}
