package hyperliquid

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	hlsdk "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
	hlspot "github.com/QuantProcessing/exchanges/sdk/hyperliquid/spot"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type spotSDK interface {
	GetSpotMeta(context.Context) (*hlspot.SpotMeta, error)
	AllMids(context.Context) (map[string]string, error)
	L2Book(context.Context, string) (*hlspot.L2BookResponse, error)
	GetBalance() (*hlspot.Balance, error)
	PlaceOrder(context.Context, hlspot.PlaceOrderRequest) (*hlspot.OrderStatus, error)
	CancelOrder(context.Context, hlspot.CancelOrderRequest) (*string, error)
	UserOpenOrders(context.Context, string) ([]hlspot.Order, error)
}

type spotProvider struct {
	sdk      spotSDK
	insts    map[model.InstrumentID]model.Instrument
	assetIDs map[model.InstrumentID]int
	rawIndex map[string]model.InstrumentID
}

func newSpotProvider(sdk spotSDK) *spotProvider {
	return &spotProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), assetIDs: make(map[model.InstrumentID]int), rawIndex: make(map[string]model.InstrumentID)}
}

func (p *spotProvider) LoadAll(ctx context.Context) error {
	meta, err := p.sdk.GetSpotMeta(ctx)
	if err != nil {
		return err
	}
	tokenByIndex := make(map[int]struct {
		name       string
		szDecimals int
	})
	for _, token := range meta.Tokens {
		tokenByIndex[token.Index] = struct {
			name       string
			szDecimals int
		}{name: token.Name, szDecimals: token.SzDecimals}
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.assetIDs = make(map[model.InstrumentID]int)
	p.rawIndex = make(map[string]model.InstrumentID)
	for _, pair := range meta.Universe {
		if len(pair.Tokens) < 2 {
			continue
		}
		base := tokenByIndex[pair.Tokens[0]]
		quote := tokenByIndex[pair.Tokens[1]]
		inst := model.Instrument{
			ID:        model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-SPOT", base.name, quote.name), Venue: Venue},
			RawSymbol: pair.Name,
			Type:      model.InstrumentTypeSpot,
			Base:      model.Currency(base.name),
			Quote:     model.Currency(quote.name),
			PriceTick: decimal.RequireFromString("0.000001"),
			SizeTick:  decimalTick(base.szDecimals),
			Status:    model.InstrumentStatusTrading,
		}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.assetIDs[inst.ID] = pair.Index
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

func (p *spotProvider) assetID(id model.InstrumentID) (int, error) {
	assetID, ok := p.assetIDs[id]
	if !ok {
		return 0, fmt.Errorf("%w: missing asset id for %s", model.ErrInstrumentNotFound, id.String())
	}
	return assetID, nil
}

type spotDataClient struct {
	id       string
	provider *spotProvider
	sdk      spotSDK
	ws       spotMarketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]hlMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

func newSpotDataClient(id string, provider *spotProvider, sdk spotSDK) *spotDataClient {
	return &spotDataClient{id: id, provider: provider, sdk: sdk, ws: hlspot.NewWebsocketClient(hlsdk.NewWebsocketClient(context.Background())), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]hlMarketTopic)}
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
	mids, err := c.sdk.AllMids(ctx)
	if err != nil {
		return model.Ticker{}, err
	}
	mid := decimalOrFallback(mids[raw], "0")
	return model.Ticker{InstrumentID: id, Bid: mid, Ask: mid, Last: mid, Timestamp: time.Now()}, nil
}
func (c *spotDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	depth, err := c.sdk.L2Book(ctx, raw)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	if len(depth.Levels) > 0 {
		for _, level := range depth.Levels[0] {
			book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(level.Px), Size: decimal.RequireFromString(level.Sz)})
		}
	}
	if len(depth.Levels) > 1 {
		for _, level := range depth.Levels[1] {
			book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(level.Px), Size: decimal.RequireFromString(level.Sz)})
		}
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
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := hlTopicFor(raw, sub)
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
		err = c.ws.SubscribeBbo(raw, func(event hlsdk.WsBbo) {
			c.handleBbo(sub.InstrumentID, event)
		})
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeL2Book(raw, func(event hlsdk.WsL2Book) {
			c.handleL2Book(sub.InstrumentID, event)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeTrades(raw, func(trades []hlsdk.WsTrade) {
			c.handleTrades(sub.InstrumentID, trades)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeCandle(raw, topic.interval, func(candle hlsdk.WsCandle) {
			c.handleCandle(barType, candle)
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

func (c *spotDataClient) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if c.ws == nil {
		return model.ErrNotSupported
	}
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			c.health.LastError = err
			return err
		}
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := hlTopicFor(raw, sub)
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
		err = c.ws.UnsubscribeBbo(topic.coin)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeL2Book(topic.coin)
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeTrades(topic.coin)
	case model.MarketDataTypeBar:
		err = c.ws.UnsubscribeCandle(topic.coin, topic.interval)
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *spotDataClient) handleBbo(id model.InstrumentID, raw hlsdk.WsBbo) {
	bid := decimal.Zero
	ask := decimal.Zero
	if len(raw.Bbo) > 0 {
		bid = decimalOrFallback(raw.Bbo[0].Px, "0")
	}
	if len(raw.Bbo) > 1 {
		ask = decimalOrFallback(raw.Bbo[1].Px, "0")
	}
	last := bid
	if bid.IsPositive() && ask.IsPositive() {
		last = bid.Add(ask).Div(decimal.NewFromInt(2))
	} else if ask.IsPositive() {
		last = ask
	}
	ts := parseHLTime(raw.Time)
	emitTicker, emitQuote := c.bboFanout(id)
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: bid, Ask: ask, Last: last, Timestamp: ts}}); err != nil {
			return
		}
	}
	if emitQuote && len(raw.Bbo) > 1 {
		_ = c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     bid,
			AskPrice:     ask,
			BidSize:      decimalOrFallback(raw.Bbo[0].Sz, "0"),
			AskSize:      decimalOrFallback(raw.Bbo[1].Sz, "0"),
			Timestamp:    ts,
			InitTime:     ts,
		}})
	}
}

func (c *spotDataClient) handleL2Book(id model.InstrumentID, raw hlsdk.WsL2Book) {
	book := model.OrderBook{InstrumentID: id, Timestamp: parseHLTime(raw.Time)}
	if len(raw.Levels) > 0 {
		for _, level := range raw.Levels[0] {
			book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimalOrFallback(level.Px, "0"), Size: decimalOrFallback(level.Sz, "0")})
		}
	}
	if len(raw.Levels) > 1 {
		for _, level := range raw.Levels[1] {
			book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimalOrFallback(level.Px, "0"), Size: decimalOrFallback(level.Sz, "0")})
		}
	}
	_ = c.emitMarket(model.MarketEvent{OrderBook: &book})
}

func (c *spotDataClient) handleTrades(id model.InstrumentID, trades []hlsdk.WsTrade) {
	for _, trade := range trades {
		tradeID := ""
		if trade.Tid != 0 {
			tradeID = fmt.Sprintf("%d", trade.Tid)
		}
		if tradeID == "" {
			tradeID = firstNonEmpty(trade.Hash, fmt.Sprintf("%s:%s:%d", trade.Px, trade.Sz, trade.Time))
		}
		ts := parseHLTime(trade.Time)
		if err := c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  id,
			Price:         decimalOrFallback(trade.Px, "0"),
			Size:          decimalOrFallback(trade.Sz, "0"),
			AggressorSide: hlAggressorSide(trade.Side),
			TradeID:       model.TradeID(tradeID),
			Timestamp:     ts,
			InitTime:      ts,
		}}); err != nil {
			return
		}
	}
}

func (c *spotDataClient) handleCandle(barType model.BarType, candle hlsdk.WsCandle) {
	ts := parseHLTime(firstPositiveInt64(candle.TClose, candle.T))
	_ = c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:   barType.Canonical(),
		Open:      decimalOrFallback(candle.O, "0"),
		High:      decimalOrFallback(candle.H, "0"),
		Low:       decimalOrFallback(candle.L, "0"),
		Close:     decimalOrFallback(candle.C, "0"),
		Volume:    decimalOrFallback(candle.V, "0"),
		Timestamp: ts,
		InitTime:  ts,
	}})
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
		err := fmt.Errorf("%w: hyperliquid spot market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *spotDataClient) topicActiveLocked(topic hlMarketTopic) bool {
	for _, activeTopic := range c.topics {
		if activeTopic == topic {
			return true
		}
	}
	return false
}

func (c *spotDataClient) bboFanout(id model.InstrumentID) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, emitTicker := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()]
	_, emitQuote := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()]
	return emitTicker, emitQuote
}

type spotExecutionClient struct {
	accountID   model.AccountID
	accountAddr string
	provider    *spotProvider
	sdk         spotSDK
	privateWS   spotAccountWS
	events      chan model.ExecutionEvent
	mu          sync.Mutex
	registered  bool
	health      venue.ExecutionHealth
}

func newSpotExecutionClient(accountID model.AccountID, accountAddr string, provider *spotProvider, sdk spotSDK) *spotExecutionClient {
	if accountID == "" {
		accountID = "hyperliquid-spot"
	}
	return &spotExecutionClient{accountID: accountID, accountAddr: accountAddr, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 64)}
}

func (c *spotExecutionClient) Venue() model.Venue         { return Venue }
func (c *spotExecutionClient) AccountID() model.AccountID { return c.accountID }
func (c *spotExecutionClient) Connect(ctx context.Context) error {
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			c.health.LastError = err
			return err
		}
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
	if c.privateWS == nil || c.accountAddr == "" {
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
	if err := c.privateWS.SubscribeOrderUpdates(c.accountAddr, c.handleOrderUpdates); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeUserFills(c.accountAddr, c.handleUserFills); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}

func (c *spotExecutionClient) QueryAccount(context.Context) (model.AccountSnapshot, error) {
	balance, err := c.sdk.GetBalance()
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	for _, bal := range balance.Balances {
		locked := decimalOrFallback(bal.Hold, "0")
		total := decimalOrFallback(bal.Total, "0")
		free := total.Sub(locked)
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(bal.Coin), Free: free.String(), Locked: locked.String(), Total: total.String()})
	}
	return snapshot, nil
}
func (c *spotExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	assetID, err := c.provider.assetID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	clientID := string(cmd.ClientOrderID)
	status, err := c.sdk.PlaceOrder(ctx, hlspot.PlaceOrderRequest{AssetID: assetID, IsBuy: cmd.Side == model.OrderSideBuy, Price: float64OrZero(cmd.Price), Size: float64OrZero(cmd.Quantity), ClientOrderID: &clientID, OrderType: hlspot.OrderType{Limit: &hlspot.OrderTypeLimit{Tif: toTIF(cmd.TimeInForce)}}})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapOrderStatus(cmd, status), nil
}
func (c *spotExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	assetID, err := c.provider.assetID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if _, err := c.sdk.CancelOrder(ctx, hlspot.CancelOrderRequest{AssetID: assetID, OrderID: parseOrderID(cmd.OrderID)}); err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: cmd.OrderID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}
func (c *spotExecutionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			return nil, err
		}
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.UserOpenOrders(ctx, c.accountAddr)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		if order.Coin != raw {
			continue
		}
		reports = append(reports, c.openOrderReport(id, order))
	}
	return reports, nil
}

func (c *spotExecutionClient) handleOrderUpdates(updates []hlsdk.WsOrderUpdate) {
	for _, update := range updates {
		id, ok := c.provider.instrumentIDByRaw(update.Order.Coin)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: hyperliquid spot coin %s", model.ErrInstrumentNotFound, update.Order.Coin)
			continue
		}
		report := c.orderUpdateReport(id, update)
		_ = c.emitExecution(model.ExecutionEvent{Order: &report})
	}
}

func (c *spotExecutionClient) handleUserFills(fills hlsdk.WsUserFills) {
	for _, fill := range fills.Fills {
		id, ok := c.provider.instrumentIDByRaw(fill.Coin)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: hyperliquid spot coin %s", model.ErrInstrumentNotFound, fill.Coin)
			continue
		}
		inst, _ := c.provider.Get(id)
		feeCurrency := model.Currency(fill.FeeToken)
		if feeCurrency == "" {
			feeCurrency = inst.Quote
		}
		tradeID := fmt.Sprintf("%d", fill.Tid)
		if fill.Tid == 0 && fill.Hash != "" {
			tradeID = fill.Hash
		}
		report := model.FillReport{
			AccountID:    c.accountID,
			InstrumentID: id,
			OrderID:      model.OrderID(fmt.Sprintf("%d", fill.Oid)),
			TradeID:      model.TradeID(tradeID),
			Side:         sideFromWire(fill.Side),
			Price:        decimalOrFallback(fill.Px, "0"),
			Quantity:     decimalOrFallback(fill.Sz, "0"),
			Fee:          decimalOrFallback(fill.Fee, "0").Abs(),
			FeeCurrency:  feeCurrency,
			Timestamp:    parseHLTime(fill.Time),
		}
		_ = c.emitExecution(model.ExecutionEvent{Fill: &report})
	}
}

func (c *spotExecutionClient) orderUpdateReport(id model.InstrumentID, update hlsdk.WsOrderUpdate) model.OrderStatusReport {
	order := update.Order
	quantity := decimalOrFallback(firstNonEmpty(order.OrigSz, order.Sz), "0")
	leaves := decimalOrFallback(order.Sz, "0")
	filled := filledQuantity(quantity, leaves)
	status := mapHLOrderStatus(update.Status)
	if status == model.OrderStatusAccepted && filled.IsPositive() {
		status = model.OrderStatusPartiallyFilled
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(fmt.Sprintf("%d", order.Oid)),
		ClientOrderID:   model.ClientOrderID(order.Cliod),
		Status:          status,
		Side:            sideFromWire(order.Side),
		Type:            model.OrderTypeLimit,
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leavesQuantity(quantity, filled),
		Price:           decimalOrFallback(order.LimitPx, "0"),
		LastUpdatedTime: parseHLTime(firstPositiveInt64(update.StatusTimestamp, order.Timestamp)),
	}
}

func (c *spotExecutionClient) openOrderReport(id model.InstrumentID, order hlspot.Order) model.OrderStatusReport {
	quantity := decimalOrFallback(firstNonEmpty(order.OrigSz, order.Sz), "0")
	leaves := decimalOrFallback(order.Sz, "0")
	filled := filledQuantity(quantity, leaves)
	status := model.OrderStatusAccepted
	if filled.IsPositive() {
		status = model.OrderStatusPartiallyFilled
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(fmt.Sprintf("%d", order.Oid)),
		Status:          status,
		Side:            sideFromWire(order.Side),
		Type:            model.OrderTypeLimit,
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leavesQuantity(quantity, filled),
		Price:           decimalOrFallback(order.LimitPx, "0"),
		LastUpdatedTime: parseHLTime(order.Timestamp),
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
		err := fmt.Errorf("%w: hyperliquid spot execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *spotExecutionClient) mapOrderStatus(cmd model.SubmitOrder, status *hlspot.OrderStatus) model.OrderStatusReport {
	report := model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusAccepted, Side: cmd.Side, Type: cmd.Type, Quantity: cmd.Quantity, Price: cmd.Price, LastUpdatedTime: time.Now()}
	switch {
	case status.Resting != nil:
		report.OrderID = model.OrderID(fmt.Sprintf("%d", status.Resting.Oid))
	case status.Filled != nil:
		report.OrderID = model.OrderID(fmt.Sprintf("%d", status.Filled.Oid))
		report.Status = model.OrderStatusFilled
		report.FilledQuantity = decimalOrFallback(status.Filled.TotalSz, "0")
		report.AveragePrice = decimalOrFallback(status.Filled.AvgPx, "0")
	case status.Error != nil:
		report.OrderID = model.OrderID("rejected")
		report.Status = model.OrderStatusRejected
	}
	return report
}

type spotMarketWS interface {
	Connect() error
	SubscribeBbo(string, func(hlsdk.WsBbo)) error
	SubscribeL2Book(string, func(hlsdk.WsL2Book)) error
	SubscribeTrades(string, func([]hlsdk.WsTrade)) error
	SubscribeCandle(string, string, func(hlsdk.WsCandle)) error
	UnsubscribeBbo(string) error
	UnsubscribeL2Book(string) error
	UnsubscribeTrades(string) error
	UnsubscribeCandle(string, string) error
	Close()
}

type spotAccountWS interface {
	Connect() error
	SubscribeOrderUpdates(string, func([]hlsdk.WsOrderUpdate)) error
	SubscribeUserFills(string, func(hlsdk.WsUserFills)) error
	Close()
}
