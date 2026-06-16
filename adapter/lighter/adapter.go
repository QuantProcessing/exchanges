package lighter

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	lightersdk "github.com/QuantProcessing/exchanges/sdk/lighter"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetOrderBookDetails(context.Context, *int, *string) (*lightersdk.OrderBookDetailsResponse, error)
	GetOrderBookOrders(context.Context, int, int64) (*lightersdk.OrderBookOrdersResponse, error)
	GetFundingRate(context.Context, int) (*lightersdk.FundingRateData, error)
	GetAccount(context.Context) (*lightersdk.AccountResponse, error)
	GetAccountActiveOrders(context.Context, int) (*lightersdk.AccountActiveOrdersResponse, error)
	PlaceOrder(context.Context, lightersdk.CreateOrderRequest) (*lightersdk.CreateOrderResponse, error)
	CancelOrder(context.Context, lightersdk.CancelOrderRequest) (*lightersdk.CancelOrderResponse, error)
}

type perpProvider struct {
	sdk         sdkClient
	insts       map[model.InstrumentID]model.Instrument
	marketIDs   map[model.InstrumentID]int
	marketIndex map[int]model.InstrumentID
	priceDec    map[model.InstrumentID]uint8
	sizeDec     map[model.InstrumentID]uint8
	last        map[model.InstrumentID]decimal.Decimal
}

func newPerpProvider(sdk sdkClient) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), marketIDs: make(map[model.InstrumentID]int), marketIndex: make(map[int]model.InstrumentID), priceDec: make(map[model.InstrumentID]uint8), sizeDec: make(map[model.InstrumentID]uint8), last: make(map[model.InstrumentID]decimal.Decimal)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	details, err := p.sdk.GetOrderBookDetails(ctx, nil, nil)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.marketIDs = make(map[model.InstrumentID]int)
	p.marketIndex = make(map[int]model.InstrumentID)
	p.priceDec = make(map[model.InstrumentID]uint8)
	p.sizeDec = make(map[model.InstrumentID]uint8)
	p.last = make(map[model.InstrumentID]decimal.Decimal)
	for _, detail := range details.OrderBookDetails {
		if detail.MarketType != "" && detail.MarketType != string(lightersdk.MarketTypePerp) {
			continue
		}
		base, quote := splitSymbol(detail.Symbol)
		makerFee, err := decimalFromString(detail.MakerFee, "0")
		if err != nil {
			return err
		}
		takerFee, err := decimalFromString(detail.TakerFee, "0")
		if err != nil {
			return err
		}
		inst := model.Instrument{ID: model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", base, quote), Venue: Venue}, RawSymbol: detail.Symbol, Type: model.InstrumentTypePerp, Base: model.Currency(base), Quote: model.Currency(quote), Settle: model.Currency(quote), PriceTick: decimalTick(detail.PriceDecimals), SizeTick: decimalTick(detail.SizeDecimals), MakerFee: makerFee, TakerFee: takerFee, MarginInit: lighterFraction(detail.DefaultInitialMarginFraction), MarginMaint: lighterFraction(detail.MaintenanceMarginFraction), Status: mapInstrumentStatus(detail.Status)}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.marketIDs[inst.ID] = detail.MarketId
		p.marketIndex[detail.MarketId] = inst.ID
		p.priceDec[inst.ID] = detail.PriceDecimals
		p.sizeDec[inst.ID] = detail.SizeDecimals
		p.last[inst.ID] = decimal.NewFromFloat(detail.LastTradePrice)
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
func (p *perpProvider) marketID(id model.InstrumentID) (int, error) {
	marketID, ok := p.marketIDs[id]
	if !ok {
		return 0, fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return marketID, nil
}
func (p *perpProvider) instrumentIDByMarketID(marketID int) (model.InstrumentID, bool) {
	id, ok := p.marketIndex[marketID]
	return id, ok
}
func (p *perpProvider) ensureLoaded(ctx context.Context) error {
	if len(p.insts) > 0 {
		return nil
	}
	return p.LoadAll(ctx)
}

type dataClient struct {
	id       string
	provider *perpProvider
	sdk      sdkClient
	ws       marketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]lighterMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

type lighterMarketTopic struct {
	kind     model.MarketDataType
	marketID int
}

func newDataClient(id string, provider *perpProvider, sdk sdkClient) *dataClient {
	return &dataClient{id: id, provider: provider, sdk: sdk, ws: lightersdk.NewWebsocketClient(context.Background()), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]lighterMarketTopic)}
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
	mid := c.provider.last[id]
	return model.Ticker{InstrumentID: id, Bid: mid, Ask: mid, Last: mid, Timestamp: time.Now()}, nil
}
func (c *dataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderBook{}, err
	}
	marketID, err := c.provider.marketID(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	depth, err := c.sdk.GetOrderBookOrders(ctx, marketID, int64(limit))
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range depth.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimalOrFallback(bid.Price, "0"), Size: decimalOrFallback(bid.RemainingBaseAmount, "0")})
	}
	for _, ask := range depth.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimalOrFallback(ask.Price, "0"), Size: decimalOrFallback(ask.RemainingBaseAmount, "0")})
	}
	return book, book.Validate()
}
func (c *dataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.FundingRate{}, err
	}
	marketID, err := c.provider.marketID(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	resp, err := c.sdk.GetFundingRate(ctx, marketID)
	if err != nil {
		return model.FundingRate{}, err
	}
	timestamp := time.Now()
	if resp.FundingTime > 0 {
		timestamp = parseLighterUnix(resp.FundingTime)
	}
	funding := model.FundingRate{
		InstrumentID:    id,
		Rate:            decimalOrFallback(resp.FundingRate, "0"),
		NextFundingTime: parseLighterUnix(resp.NextFundingTime),
		FundingInterval: time.Duration(resp.FundingIntervalHours) * time.Hour,
		Timestamp:       timestamp,
		InitTime:        time.Now(),
	}
	return funding, funding.Validate()
}
func (c *dataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	marketID, err := c.provider.marketID(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := lighterTopicFor(marketID, sub)
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
		err = c.ws.SubscribeTicker(marketID, func(raw []byte) {
			c.handleTicker(sub.InstrumentID, raw)
		})
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeOrderBook(marketID, func(raw []byte) {
			c.handleOrderBook(sub.InstrumentID, raw)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeTrades(marketID, func(raw []byte) {
			c.handleTrade(sub.InstrumentID, raw)
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
	marketID, err := c.provider.marketID(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := lighterTopicFor(marketID, sub)
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
		err = c.ws.UnsubscribeTicker(topic.marketID)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeOrderBook(topic.marketID)
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeTrades(topic.marketID)
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}
func (c *dataClient) handleTicker(id model.InstrumentID, raw []byte) {
	var event lightersdk.WsTickerEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		c.health.LastError = err
		return
	}
	bid := decimalOrFallback(event.Ticker.B.Price, "0")
	ask := decimalOrFallback(event.Ticker.A.Price, "0")
	last := decimal.Zero
	if bid.IsPositive() && ask.IsPositive() {
		last = bid.Add(ask).Div(decimal.NewFromInt(2))
	}
	ts := parseLighterUnix(event.Timestamp)
	emitTicker, emitQuote := c.tickerFanout(id)
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: ts}}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     bid,
			AskPrice:     ask,
			BidSize:      decimalOrFallback(event.Ticker.B.Size, "0"),
			AskSize:      decimalOrFallback(event.Ticker.A.Size, "0"),
			Timestamp:    ts,
			InitTime:     ts,
		}})
	}
}
func (c *dataClient) handleOrderBook(id model.InstrumentID, raw []byte) {
	var event lightersdk.WsOrderBookEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		c.health.LastError = err
		return
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseLighterUnix(firstNonZero(event.OrderBook.LastUpdatedAt, event.LastUpdatedAt, event.Timestamp))}
	for _, bid := range event.OrderBook.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimalOrFallback(bid.Price, "0"), Size: decimalOrFallback(bid.Size, "0")})
	}
	for _, ask := range event.OrderBook.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimalOrFallback(ask.Price, "0"), Size: decimalOrFallback(ask.Size, "0")})
	}
	_ = c.emitMarket(model.MarketEvent{OrderBook: &book})
}
func (c *dataClient) handleTrade(id model.InstrumentID, raw []byte) {
	var event lightersdk.WsTradeEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		c.health.LastError = err
		return
	}
	for _, trade := range append(event.Trades, event.LiquidationTrades...) {
		tradeID := trade.TradeIdStr
		if tradeID == "" && trade.TradeId != 0 {
			tradeID = strconv.FormatInt(trade.TradeId, 10)
		}
		if tradeID == "" {
			tradeID = fmt.Sprintf("%d:%s:%s", firstNonZero(trade.TransactionTime, trade.Timestamp), trade.Price, trade.Size)
		}
		ts := parseLighterUnix(firstNonZero(trade.TransactionTime, trade.Timestamp))
		if err := c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  id,
			Price:         decimalOrFallback(trade.Price, "0"),
			Size:          decimalOrFallback(trade.Size, "0"),
			AggressorSide: lighterAggressorSide(trade.IsMakerAsk),
			TradeID:       model.TradeID(tradeID),
			Timestamp:     ts,
			InitTime:      ts,
		}}); err != nil {
			return
		}
	}
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
		err := fmt.Errorf("%w: lighter market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *dataClient) topicActiveLocked(topic lighterMarketTopic) bool {
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
	accountIndex int64
	authToken    string
	authProvider func() (string, error)
	provider     *perpProvider
	sdk          sdkClient
	privateWS    accountWS
	events       chan model.ExecutionEvent
	mu           sync.Mutex
	registered   bool
	health       venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *perpProvider, sdk sdkClient) *executionClient {
	if accountID == "" {
		accountID = "lighter-perp"
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
	authToken, err := c.executionAuthToken()
	if err != nil {
		return err
	}
	if err := c.privateWS.SubscribeAccountAllOrders(c.accountIndex, authToken, c.handleOrderEvent); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeAccountAllTrades(c.accountIndex, authToken, c.handleTradeEvent); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeAccountAllPositions(c.accountIndex, authToken, c.handlePositionEvent); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}
func (c *executionClient) executionAuthToken() (string, error) {
	if c.authToken != "" {
		return c.authToken, nil
	}
	if c.authProvider == nil {
		return "", fmt.Errorf("%w: lighter private stream auth token", model.ErrNotSupported)
	}
	authToken, err := c.authProvider()
	if err != nil {
		return "", err
	}
	c.authToken = authToken
	return authToken, nil
}
func (c *executionClient) handleOrderEvent(raw []byte) {
	var event lightersdk.WsAccountAllOrdersEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		c.health.LastError = err
		return
	}
	for marketKey, orders := range event.Orders {
		for _, order := range orders {
			if order == nil {
				continue
			}
			id, ok := c.instrumentIDForOrder(marketKey, order.MarketIndex)
			if !ok {
				continue
			}
			report := c.orderReport(id, *order)
			_ = c.emitExecution(model.ExecutionEvent{Order: &report})
		}
	}
}
func (c *executionClient) handleTradeEvent(raw []byte) {
	var event lightersdk.WsAccountAllTradesEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		c.health.LastError = err
		return
	}
	for marketKey, trades := range event.Trades {
		marketID := parseInt(marketKey)
		for _, trade := range trades {
			if trade.MarketId != 0 {
				marketID = trade.MarketId
			}
			id, ok := c.provider.instrumentIDByMarketID(marketID)
			if !ok {
				c.health.LastError = fmt.Errorf("%w: lighter market %d", model.ErrInstrumentNotFound, marketID)
				continue
			}
			report := c.fillReport(id, trade)
			_ = c.emitExecution(model.ExecutionEvent{Fill: &report})
		}
	}
}
func (c *executionClient) handlePositionEvent(raw []byte) {
	var event lightersdk.WsAccountAllPositionsEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		c.health.LastError = err
		return
	}
	for marketKey, position := range event.Positions {
		if position == nil {
			continue
		}
		marketID := position.MarketId
		if marketID == 0 {
			marketID = parseInt(marketKey)
		}
		report, err := c.positionReport(marketID, position)
		if err != nil {
			c.health.LastError = err
			continue
		}
		_ = c.emitExecution(model.ExecutionEvent{Position: &report})
	}
}
func (c *executionClient) instrumentIDForOrder(marketKey string, orderMarketID int) (model.InstrumentID, bool) {
	marketID := orderMarketID
	if marketID == 0 {
		marketID = parseInt(marketKey)
	}
	id, ok := c.provider.instrumentIDByMarketID(marketID)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: lighter market %d", model.ErrInstrumentNotFound, marketID)
	}
	return id, ok
}
func (c *executionClient) orderReport(id model.InstrumentID, order lightersdk.Order) model.OrderStatusReport {
	quantity := decimalOrFallback(order.InitialBaseAmount, "0")
	filled := decimalOrFallback(order.FilledBaseAmount, "0")
	leaves := decimalOrFallback(order.RemainingBaseAmount, "0")
	if leaves.IsZero() && quantity.IsPositive() {
		leaves = leavesQuantity(quantity, filled)
	}
	fillValue := decimalOrFallback(order.FilledQuoteAmount, "0")
	avg := decimal.Zero
	if filled.IsPositive() && fillValue.IsPositive() {
		avg = fillValue.Div(filled)
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(order.OrderId),
		ClientOrderID:   model.ClientOrderID(firstNonEmpty(order.ClientOrderId, strconv.FormatInt(order.ClientOrderIndex, 10))),
		Status:          mapOrderStatus(order.Status),
		Side:            fromSide(order.Side, order.IsAsk),
		Type:            fromOrderType(order.OrderType),
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leaves,
		Price:           decimalOrFallback(order.Price, "0"),
		AveragePrice:    avg,
		LastUpdatedTime: parseLighterUnix(firstNonZero(order.UpdatedAt, order.TransactionTime, order.CreatedAt, order.Timestamp)),
	}
}
func (c *executionClient) fillReport(id model.InstrumentID, trade lightersdk.Trade) model.FillReport {
	side := model.OrderSideSell
	orderID := firstNonEmpty(trade.AskIdStr, strconv.FormatInt(trade.AskId, 10))
	if trade.BidAccountId == c.accountIndex {
		side = model.OrderSideBuy
		orderID = firstNonEmpty(trade.BidIdStr, strconv.FormatInt(trade.BidId, 10))
	}
	fee := decimal.NewFromInt(int64(trade.TakerFee)).Abs()
	return model.FillReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		OrderID:      model.OrderID(orderID),
		TradeID:      model.TradeID(firstNonEmpty(trade.TradeIdStr, strconv.FormatInt(trade.TradeId, 10))),
		Side:         side,
		Price:        decimalOrFallback(trade.Price, "0"),
		Quantity:     decimalOrFallback(trade.Size, "0"),
		Fee:          fee,
		FeeCurrency:  idQuote(id),
		Timestamp:    parseLighterUnix(firstNonZero(trade.TransactionTime, trade.Timestamp)),
	}
}
func (c *executionClient) positionReport(marketID int, position *lightersdk.Position) (model.PositionStatusReport, error) {
	id, ok := c.provider.instrumentIDByMarketID(marketID)
	if !ok {
		return model.PositionStatusReport{}, fmt.Errorf("%w: lighter market %d", model.ErrInstrumentNotFound, marketID)
	}
	qty := decimalOrFallback(position.Position, "0")
	if position.Sign < 0 && qty.IsPositive() {
		qty = qty.Neg()
	}
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(id.String()),
		Side:         positionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   decimalOrFallback(position.AvgEntryPrice, "0"),
		Timestamp:    time.Now(),
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
		err := fmt.Errorf("%w: lighter execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}
func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	if len(account.Accounts) > 0 {
		acc := account.Accounts[0]
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: "USDC", Free: acc.AvailableBalance, Total: acc.Collateral})
	}
	return snapshot, nil
}
func (c *executionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	marketID, err := c.provider.marketID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	req := lightersdk.CreateOrderRequest{MarketId: marketID, Price: uint32(scaleDecimal(cmd.Price, c.provider.priceDec[cmd.InstrumentID])), BaseAmount: scaleDecimal(cmd.Quantity, c.provider.sizeDec[cmd.InstrumentID]), IsAsk: toSide(cmd.Side), OrderType: toOrderType(cmd.Type), TimeInForce: toTIF(cmd.TimeInForce), ClientOrderId: parseInt64(string(cmd.ClientOrderID)), OrderExpiry: lightersdk.Default28DayOrderExpiry}
	resp, err := c.sdk.PlaceOrder(ctx, req)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: model.OrderID(resp.TxHash), ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusAccepted, Side: cmd.Side, Type: cmd.Type, Quantity: cmd.Quantity, Price: cmd.Price, LastUpdatedTime: time.Now()}, nil
}
func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	marketID, err := c.provider.marketID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	resp, err := c.sdk.CancelOrder(ctx, lightersdk.CancelOrderRequest{MarketId: marketID, OrderId: parseInt64(string(cmd.OrderID))})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: model.OrderID(resp.TxHash), ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}
func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	marketID, err := c.provider.marketID(id)
	if err != nil {
		return nil, err
	}
	active, err := c.sdk.GetAccountActiveOrders(ctx, marketID)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(active.Orders))
	for _, order := range active.Orders {
		reports = append(reports, c.orderReport(id, *order))
	}
	return reports, nil
}
func (c *executionClient) GeneratePositionStatusReports(ctx context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	marketID, err := c.provider.marketID(id)
	if err != nil {
		return nil, err
	}
	account, err := c.sdk.GetAccount(ctx)
	if err != nil {
		return nil, err
	}
	reports := []model.PositionStatusReport{}
	for _, acc := range account.Accounts {
		for _, position := range acc.Positions {
			if position == nil || position.MarketId != marketID {
				continue
			}
			report, err := c.positionReport(position.MarketId, position)
			if err != nil {
				return nil, err
			}
			reports = append(reports, report)
		}
	}
	return reports, nil
}

type Adapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := lightersdk.NewClient().WithCredentials(opts.PrivateKey, opts.AccountIndex, opts.KeyIndex)
	provider := newPerpProvider(client)
	exec := newExecutionClient(opts.AccountID, provider, client)
	exec.accountIndex = opts.AccountIndex
	if opts.PrivateKey != "" && opts.AccountIndex > 0 {
		exec.privateWS = lightersdk.NewWebsocketClient(context.Background())
		exec.authProvider = func() (string, error) {
			return client.CreateAuthToken(time.Now().Add(10 * time.Minute))
		}
	}
	return &Adapter{provider: provider, data: newDataClient("lighter-perp-data", provider, client), exec: exec}, nil
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

func accountIndexFromConfig(raw string) int64 {
	value, _ := strconv.ParseInt(raw, 10, 64)
	return value
}

func keyIndexFromConfig(raw string) uint8 {
	value, _ := strconv.ParseUint(raw, 10, 8)
	return uint8(value)
}

type marketWS interface {
	Connect() error
	SubscribeTicker(int, func([]byte)) error
	SubscribeOrderBook(int, func([]byte)) error
	SubscribeTrades(int, func([]byte)) error
	UnsubscribeTicker(int) error
	UnsubscribeOrderBook(int) error
	UnsubscribeTrades(int) error
	Close()
}

type accountWS interface {
	Connect() error
	SubscribeAccountAllOrders(int64, string, func([]byte)) error
	SubscribeAccountAllTrades(int64, string, func([]byte)) error
	SubscribeAccountAllPositions(int64, string, func([]byte)) error
	Close()
}
