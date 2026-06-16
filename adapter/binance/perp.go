package binance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type perpSDK interface {
	ExchangeInfo(context.Context) (*perp.ExchangeInfoResponse, error)
	Ticker(context.Context, string) (*perp.TickerResponse, error)
	Depth(context.Context, string, int) (*perp.DepthResponse, error)
	GetFundingRate(context.Context, string) (*perp.FundingRateData, error)
	GetAccount(context.Context) (*perp.AccountResponse, error)
	PlaceOrder(context.Context, perp.PlaceOrderParams) (*perp.OrderResponse, error)
	CancelOrder(context.Context, perp.CancelOrderParams) (*perp.OrderResponse, error)
	GetOpenOrders(context.Context, string) ([]perp.OrderResponse, error)
}

type perpProvider struct {
	sdk   perpSDK
	insts map[model.InstrumentID]model.Instrument
}

func newPerpProvider(sdk perpSDK) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	info, err := p.sdk.ExchangeInfo(ctx)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	for _, symbol := range info.Symbols {
		inst := model.Instrument{
			ID:        model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", symbol.BaseAsset, symbol.QuoteAsset), Venue: Venue},
			RawSymbol: symbol.Symbol,
			Type:      model.InstrumentTypePerp,
			Base:      model.Currency(symbol.BaseAsset),
			Quote:     model.Currency(symbol.QuoteAsset),
			Settle:    model.Currency(defaultString(symbol.MarginAsset, symbol.QuoteAsset)),
			PriceTick: decimalFromFilter(symbol.Filters, "PRICE_FILTER", "tickSize", "0.00000001"),
			SizeTick:  decimalFromFilter(symbol.Filters, "LOT_SIZE", "stepSize", "0.00000001"),
			Status:    mapTradingStatus(symbol.Status),
		}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
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

func (p *perpProvider) rawSymbol(id model.InstrumentID) (string, error) {
	inst, ok := p.Get(id)
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return inst.RawSymbol, nil
}

func (p *perpProvider) instrumentIDByRaw(raw string) (model.InstrumentID, bool) {
	for id, inst := range p.insts {
		if strings.EqualFold(inst.RawSymbol, raw) {
			return id, true
		}
	}
	return model.InstrumentID{}, false
}

type perpDataClient struct {
	id       string
	provider *perpProvider
	sdk      perpSDK
	ws       perpMarketStream
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	mu       sync.Mutex
	health   venue.DataHealth
}

func newPerpDataClient(id string, provider *perpProvider, sdk perpSDK) *perpDataClient {
	return &perpDataClient{
		id:       id,
		provider: provider,
		sdk:      sdk,
		ws:       perp.NewWsMarketClient(context.Background()),
		events:   make(chan model.MarketEvent, 256),
		subs:     make(map[string]model.SubscribeMarketData),
	}
}

func (c *perpDataClient) Venue() model.Venue                    { return Venue }
func (c *perpDataClient) ClientID() string                      { return c.id }
func (c *perpDataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *perpDataClient) Connect(ctx context.Context) error {
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
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.Ticker{}, err
	}
	t, err := c.sdk.Ticker(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	return model.Ticker{InstrumentID: id, Last: decimal.RequireFromString(defaultString(t.LastPrice, "0")), Timestamp: binanceEventTime(t.CloseTime)}, nil
}
func (c *perpDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	depth, err := c.sdk.Depth(ctx, raw, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: binanceEventTime(defaultInt64(depth.E, depth.T))}
	for _, level := range depth.Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range depth.Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
	}
	return book, book.Validate()
}

func (c *perpDataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	resp, err := c.sdk.GetFundingRate(ctx, raw)
	if err != nil {
		return model.FundingRate{}, err
	}

	now := time.Now()
	funding := model.FundingRate{
		InstrumentID:    id,
		Rate:            decimal.RequireFromString(defaultString(resp.LastFundingRate, "0")),
		MarkPrice:       decimal.RequireFromString(defaultString(resp.MarkPrice, "0")),
		IndexPrice:      decimal.RequireFromString(defaultString(resp.IndexPrice, "0")),
		NextFundingTime: binanceEventTime(resp.NextFundingTime),
		FundingInterval: time.Duration(resp.FundingIntervalHours) * time.Hour,
		Timestamp:       now,
		InitTime:        now,
	}
	if resp.Time > 0 {
		funding.Timestamp = binanceEventTime(resp.Time)
	}
	return funding, funding.Validate()
}

func (c *perpDataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
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
			err = c.ws.SubscribeBookTicker(raw, func(event *perp.WsBookTickerEvent) error {
				return c.handleBookTicker(sub.InstrumentID, event)
			})
		}
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeLimitOrderBook(raw, sub.Depth, "250ms", func(event *perp.WsDepthEvent) error {
			book := model.OrderBook{InstrumentID: sub.InstrumentID, Timestamp: binanceEventTime(event.EventTime)}
			for _, level := range event.Bids {
				book.Bids = append(book.Bids, parseInterfaceBookLevel(level))
			}
			for _, level := range event.Asks {
				book.Asks = append(book.Asks, parseInterfaceBookLevel(level))
			}
			return c.emitMarket(model.MarketEvent{OrderBook: &book})
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeAggTrade(raw, func(event *perp.WsAggTradeEvent) error {
			return c.handleAggTrade(sub.InstrumentID, event)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		interval := binanceBarInterval(barType.Step)
		err = c.ws.SubscribeKline(raw, interval, func(event *perp.WsKlineEvent) error {
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

func (c *perpDataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
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
		err = c.ws.UnsubscribeLimitOrderBook(raw, sub.Depth, "250ms")
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

func (c *perpDataClient) handleBookTicker(id model.InstrumentID, event *perp.WsBookTickerEvent) error {
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
			Timestamp:    binanceEventTime(event.EventTime),
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
			Timestamp:    binanceEventTime(event.EventTime),
			InitTime:     binanceEventTime(event.EventTime),
		}})
	}
	return nil
}

func (c *perpDataClient) handleAggTrade(id model.InstrumentID, event *perp.WsAggTradeEvent) error {
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

func (c *perpDataClient) handleKline(barType model.BarType, event *perp.WsKlineEvent) error {
	return c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:   barType.Canonical(),
		Open:      decimal.RequireFromString(defaultString(event.Kline.OpenPrice, "0")),
		High:      decimal.RequireFromString(defaultString(event.Kline.HighPrice, "0")),
		Low:       decimal.RequireFromString(defaultString(event.Kline.LowPrice, "0")),
		Close:     decimal.RequireFromString(defaultString(event.Kline.ClosePrice, "0")),
		Volume:    decimal.RequireFromString(defaultString(event.Kline.Volume, "0")),
		Timestamp: binanceEventTime(defaultInt64(event.Kline.EndTime, event.EventTime)),
		InitTime:  binanceEventTime(event.EventTime),
	}})
}

func (c *perpDataClient) ensureStreamConnected() error {
	if c.ws == nil {
		c.ws = perp.NewWsMarketClient(context.Background())
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
		err := fmt.Errorf("%w: binance perp market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

type perpMarketStream interface {
	Connect() error
	Close()
	IsConnected() bool
	SetPostReconnect(func())
	SubscribeBookTicker(string, func(*perp.WsBookTickerEvent) error) error
	SubscribeLimitOrderBook(string, int, string, func(*perp.WsDepthEvent) error) error
	SubscribeAggTrade(string, func(*perp.WsAggTradeEvent) error) error
	SubscribeKline(string, string, func(*perp.WsKlineEvent) error) error
	UnsubscribeBookTicker(string) error
	UnsubscribeLimitOrderBook(string, int, string) error
	UnsubscribeAggTrade(string) error
	UnsubscribeKline(string, string) error
}

type perpExecutionClient struct {
	accountID         model.AccountID
	provider          *perpProvider
	sdk               perpSDK
	accountStream     perpAccountStream
	events            chan model.ExecutionEvent
	mu                sync.Mutex
	privateRegistered bool
	health            venue.ExecutionHealth
}

func newPerpExecutionClient(accountID model.AccountID, provider *perpProvider, sdk perpSDK, creds ...string) *perpExecutionClient {
	if accountID == "" {
		accountID = "binance-perp"
	}
	client := &perpExecutionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 2 && creds[0] != "" && creds[1] != "" {
		client.accountStream = perp.NewWsAccountClient(context.Background(), creds[0], creds[1])
	}
	return client
}

func (c *perpExecutionClient) Venue() model.Venue         { return Venue }
func (c *perpExecutionClient) AccountID() model.AccountID { return c.accountID }
func (c *perpExecutionClient) Connect(context.Context) error {
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
func (c *perpExecutionClient) Disconnect(context.Context) error {
	c.health.Connected = false
	if c.accountStream != nil {
		c.accountStream.Close()
	}
	return nil
}
func (c *perpExecutionClient) Health() venue.ExecutionHealth       { return c.health }
func (c *perpExecutionClient) Events() <-chan model.ExecutionEvent { return c.events }
func (c *perpExecutionClient) ResubscribeExecution(ctx context.Context) error {
	if c.accountStream == nil {
		return model.ErrNotSupported
	}
	return c.Connect(ctx)
}

func (c *perpExecutionClient) registerPrivateHandlers() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.privateRegistered {
		return
	}
	c.accountStream.SubscribeOrderUpdate(c.handleOrderUpdate)
	c.accountStream.SubscribeAccountUpdate(c.handleAccountUpdate)
	c.accountStream.SubscribeAccountConfigUpdate(func(*perp.AccountConfigUpdateEvent) {})
	c.accountStream.SetOnResubscribe(func() {
		c.health.LastEventTime = time.Now()
	})
	c.privateRegistered = true
}

func (c *perpExecutionClient) handleOrderUpdate(event *perp.OrderUpdateEvent) {
	id, ok := c.provider.instrumentIDByRaw(event.Order.Symbol)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, event.Order.Symbol)
		return
	}
	order := model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(strconv.FormatInt(event.Order.OrderID, 10)),
		ClientOrderID:   model.ClientOrderID(event.Order.ClientOrderID),
		Status:          mapOrderStatus(event.Order.OrderStatus),
		Side:            fromBinanceSide(event.Order.Side),
		Type:            fromBinanceType(event.Order.OrderType),
		Quantity:        decimal.RequireFromString(defaultString(event.Order.OriginalQty, "0")),
		FilledQuantity:  decimal.RequireFromString(defaultString(event.Order.AccumulatedFilledQty, "0")),
		Price:           decimal.RequireFromString(defaultString(event.Order.OriginalPrice, "0")),
		AveragePrice:    decimal.RequireFromString(defaultString(event.Order.AveragePrice, "0")),
		LastUpdatedTime: binanceEventTime(event.Order.TradeTime),
	}
	if err := c.emitExecution(model.ExecutionEvent{Order: &order}); err != nil {
		return
	}
	lastQty := decimal.RequireFromString(defaultString(event.Order.LastFilledQty, "0"))
	if event.Order.TradeID <= 0 || !lastQty.IsPositive() {
		return
	}
	fill := model.FillReport{
		AccountID:     c.accountID,
		InstrumentID:  id,
		OrderID:       order.OrderID,
		ClientOrderID: order.ClientOrderID,
		TradeID:       model.TradeID(strconv.FormatInt(event.Order.TradeID, 10)),
		Side:          order.Side,
		Price:         decimal.RequireFromString(defaultString(event.Order.LastFilledPrice, "0")),
		Quantity:      lastQty,
		Fee:           decimal.RequireFromString(defaultString(event.Order.Commission, "0")),
		FeeCurrency:   model.Currency(event.Order.CommissionAsset),
		Timestamp:     binanceEventTime(event.Order.TradeTime),
	}
	_ = c.emitExecution(model.ExecutionEvent{Fill: &fill})
}

func (c *perpExecutionClient) handleAccountUpdate(event *perp.AccountUpdateEvent) {
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: binanceEventTime(event.TransactionTime)}
	for _, bal := range event.UpdateData.Balances {
		total := decimal.RequireFromString(defaultString(bal.WalletBalance, "0"))
		free := decimal.RequireFromString(defaultString(bal.CrossWalletBalance, total.String()))
		snapshot.Balances = append(snapshot.Balances, model.Balance{
			Currency: model.Currency(bal.Asset),
			Free:     free.String(),
			Locked:   total.Sub(free).String(),
			Total:    total.String(),
		})
	}
	_ = c.emitExecution(model.ExecutionEvent{Account: &snapshot})
	for _, pos := range event.UpdateData.Positions {
		id, ok := c.provider.instrumentIDByRaw(pos.Symbol)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, pos.Symbol)
			continue
		}
		qty := decimal.RequireFromString(defaultString(pos.PositionAmount, "0"))
		position := model.PositionStatusReport{
			AccountID:    c.accountID,
			InstrumentID: id,
			PositionID:   model.PositionID(id.String()),
			Side:         binancePositionSide(qty, pos.PositionSide),
			Quantity:     qty.Abs(),
			EntryPrice:   decimal.RequireFromString(defaultString(pos.EntryPrice, "0")),
			Timestamp:    binanceEventTime(event.TransactionTime),
		}
		_ = c.emitExecution(model.ExecutionEvent{Position: &position})
	}
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
		err := fmt.Errorf("%w: binance perp execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}
func (c *perpExecutionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, asset := range account.Assets {
		total := decimal.RequireFromString(defaultString(asset.WalletBalance, "0"))
		free := decimal.RequireFromString(defaultString(asset.AvailableBalance, total.String()))
		snapshot.Balances = append(snapshot.Balances, model.Balance{
			Currency: model.Currency(asset.Asset),
			Free:     free.String(),
			Locked:   total.Sub(free).String(),
			Total:    total.String(),
		})
	}
	return snapshot, nil
}
func (c *perpExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.PlaceOrder(ctx, perp.PlaceOrderParams{
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
func (c *perpExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	raw, err := c.provider.rawSymbol(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.CancelOrder(ctx, perp.CancelOrderParams{Symbol: raw, OrderID: string(cmd.OrderID), OrigClientOrderID: string(cmd.ClientOrderID)})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapOrderResponse(cmd.InstrumentID, *resp), nil
}
func (c *perpExecutionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
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
func (c *perpExecutionClient) mapOrderResponse(id model.InstrumentID, resp perp.OrderResponse) model.OrderStatusReport {
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

func binancePositionSide(qty decimal.Decimal, raw string) model.PositionSide {
	switch {
	case strings.EqualFold(raw, "SHORT"):
		return model.PositionSideShort
	case strings.EqualFold(raw, "LONG"):
		return model.PositionSideLong
	case qty.IsNegative():
		return model.PositionSideShort
	case qty.IsPositive():
		return model.PositionSideLong
	default:
		return model.PositionSideFlat
	}
}

type perpAccountStream interface {
	Connect() error
	Close()
	SetOnResubscribe(func())
	SubscribeOrderUpdate(func(*perp.OrderUpdateEvent))
	SubscribeAccountUpdate(func(*perp.AccountUpdateEvent))
	SubscribeAccountConfigUpdate(func(*perp.AccountConfigUpdateEvent))
}
