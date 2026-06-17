package hyperliquid

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	hlsdk "github.com/QuantProcessing/exchanges/sdk/hyperliquid"
	hlperp "github.com/QuantProcessing/exchanges/sdk/hyperliquid/perp"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type perpSDK interface {
	GetPrepMeta(context.Context) (*hlperp.PrepMeta, error)
	GetMetaAndAssetCtxs(context.Context) (*hlperp.MetaAndAssetCtxsFull, error)
	AllMids(context.Context) (map[string]string, error)
	L2Book(context.Context, string) (*hlperp.L2BookResponse, error)
	GetBalance(context.Context) (*hlperp.PerpPosition, error)
	UserOpenOrders(context.Context, string) ([]hlperp.Order, error)
	PlaceOrder(context.Context, hlperp.PlaceOrderRequest) (*hlperp.OrderStatus, error)
	CancelOrder(context.Context, hlperp.CancelOrderRequest) (*string, error)
}

type perpProvider struct {
	sdk      perpSDK
	insts    map[model.InstrumentID]model.Instrument
	assetIDs map[model.InstrumentID]int
	rawIndex map[string]model.InstrumentID
}

func newPerpProvider(sdk perpSDK) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), assetIDs: make(map[model.InstrumentID]int), rawIndex: make(map[string]model.InstrumentID)}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	meta, err := p.sdk.GetPrepMeta(ctx)
	if err != nil {
		return err
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.assetIDs = make(map[model.InstrumentID]int)
	p.rawIndex = make(map[string]model.InstrumentID)
	for idx, asset := range meta.Universe {
		marginInit := decimal.Zero
		if asset.MaxLeverage > 0 {
			marginInit = decimal.NewFromInt(1).Div(decimal.NewFromInt(int64(asset.MaxLeverage)))
		}
		inst := model.Instrument{ID: model.InstrumentID{Symbol: fmt.Sprintf("%s-USD-PERP", asset.Name), Venue: Venue}, RawSymbol: asset.Name, Type: model.InstrumentTypePerp, Base: model.Currency(asset.Name), Quote: "USD", Settle: "USDC", PriceTick: decimal.RequireFromString("0.000001"), SizeTick: decimalTick(asset.SzDecimals), MarginInit: marginInit, Status: model.InstrumentStatusTrading}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.assetIDs[inst.ID] = idx
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
func (p *perpProvider) assetID(id model.InstrumentID) (int, error) {
	assetID, ok := p.assetIDs[id]
	if !ok {
		return 0, fmt.Errorf("%w: missing asset id for %s", model.ErrInstrumentNotFound, id.String())
	}
	return assetID, nil
}

type perpDataClient struct {
	id       string
	provider *perpProvider
	sdk      perpSDK
	ws       perpMarketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]hlMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

func newPerpDataClient(id string, provider *perpProvider, sdk perpSDK) *perpDataClient {
	return &perpDataClient{id: id, provider: provider, sdk: sdk, ws: hlperp.NewWebsocketClient(hlsdk.NewWebsocketClient(context.Background())), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]hlMarketTopic)}
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
	mids, err := c.sdk.AllMids(ctx)
	if err != nil {
		return model.Ticker{}, err
	}
	mid := decimalOrFallback(mids[raw], "0")
	return model.Ticker{InstrumentID: id, Bid: mid, Ask: mid, Last: mid, Timestamp: time.Now()}, nil
}
func (c *perpDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderBook{}, err
	}
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
func (c *perpDataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.FundingRate{}, err
	}
	assetID, err := c.provider.assetID(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	resp, err := c.sdk.GetMetaAndAssetCtxs(ctx)
	if err != nil {
		return model.FundingRate{}, err
	}
	if assetID >= len(resp.AssetCtxs) {
		return model.FundingRate{}, fmt.Errorf("%w: missing Hyperliquid asset context for %s", model.ErrInstrumentNotFound, id.String())
	}
	asset := resp.AssetCtxs[assetID]
	timestamp := time.Now().UTC().Truncate(time.Hour)
	funding := model.FundingRate{
		InstrumentID:    id,
		Rate:            decimalOrFallback(asset.Funding, "0"),
		NextFundingTime: timestamp.Add(time.Hour),
		FundingInterval: time.Hour,
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
func (c *perpDataClient) handleBbo(id model.InstrumentID, raw hlsdk.WsBbo) {
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
func (c *perpDataClient) handleL2Book(id model.InstrumentID, raw hlsdk.WsL2Book) {
	book := model.OrderBook{InstrumentID: id, Timestamp: parseHLTime(raw.Time)}
	if len(raw.Levels) > 0 {
		for _, bid := range raw.Levels[0] {
			book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimalOrFallback(bid.Px, "0"), Size: decimalOrFallback(bid.Sz, "0")})
		}
	}
	if len(raw.Levels) > 1 {
		for _, ask := range raw.Levels[1] {
			book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimalOrFallback(ask.Px, "0"), Size: decimalOrFallback(ask.Sz, "0")})
		}
	}
	_ = c.emitMarket(model.MarketEvent{OrderBook: &book})
}
func (c *perpDataClient) handleTrades(id model.InstrumentID, trades []hlsdk.WsTrade) {
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
func (c *perpDataClient) handleCandle(barType model.BarType, candle hlsdk.WsCandle) {
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
		err := fmt.Errorf("%w: hyperliquid market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *perpDataClient) topicActiveLocked(topic hlMarketTopic) bool {
	for _, activeTopic := range c.topics {
		if activeTopic == topic {
			return true
		}
	}
	return false
}

func (c *perpDataClient) bboFanout(id model.InstrumentID) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, emitTicker := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeTicker}.Key()]
	_, emitQuote := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()]
	return emitTicker, emitQuote
}

type perpExecutionClient struct {
	accountID   model.AccountID
	accountAddr string
	provider    *perpProvider
	sdk         perpSDK
	privateWS   perpAccountWS
	events      chan model.ExecutionEvent
	mu          sync.Mutex
	registered  bool
	health      venue.ExecutionHealth
}

func newPerpExecutionClient(accountID model.AccountID, accountAddr string, provider *perpProvider, sdk perpSDK) *perpExecutionClient {
	if accountID == "" {
		accountID = "hyperliquid-perp"
	}
	return &perpExecutionClient{accountID: accountID, accountAddr: accountAddr, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 64)}
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
	if c.privateWS == nil || c.accountAddr == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()
	return c.subscribePrivate(ctx)
}
func (c *perpExecutionClient) subscribePrivate(context.Context) error {
	if c.accountAddr == "" {
		return model.ErrNotSupported
	}
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
	if err := c.privateWS.SubscribeWebData2(c.accountAddr, c.handlePerpPosition); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}
func (c *perpExecutionClient) handleOrderUpdates(updates []hlsdk.WsOrderUpdate) {
	for _, update := range updates {
		id, ok := c.provider.instrumentIDByRaw(update.Order.Coin)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: hyperliquid coin %s", model.ErrInstrumentNotFound, update.Order.Coin)
			continue
		}
		report := c.orderUpdateReport(id, update)
		_ = c.emitExecution(model.ExecutionEvent{Order: &report})
	}
}
func (c *perpExecutionClient) handleUserFills(fills hlsdk.WsUserFills) {
	for _, fill := range fills.Fills {
		id, ok := c.provider.instrumentIDByRaw(fill.Coin)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: hyperliquid coin %s", model.ErrInstrumentNotFound, fill.Coin)
			continue
		}
		inst, _ := c.provider.Get(id)
		feeCurrency := model.Currency(fill.FeeToken)
		if feeCurrency == "" {
			feeCurrency = inst.Settle
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
func (c *perpExecutionClient) handlePerpPosition(position hlperp.PerpPosition) {
	for _, asset := range position.AssetPositions {
		report, err := c.positionReport(asset.Position.Coin, asset.Position.Szi, asset.Position.EntryPx, position.Time)
		if err != nil {
			c.health.LastError = err
			continue
		}
		_ = c.emitExecution(model.ExecutionEvent{Position: &report})
	}
}
func (c *perpExecutionClient) orderUpdateReport(id model.InstrumentID, update hlsdk.WsOrderUpdate) model.OrderStatusReport {
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
func (c *perpExecutionClient) positionReport(rawCoin, rawQty, rawEntry string, rawTime int64) (model.PositionStatusReport, error) {
	id, ok := c.provider.instrumentIDByRaw(rawCoin)
	if !ok {
		return model.PositionStatusReport{}, fmt.Errorf("%w: hyperliquid coin %s", model.ErrInstrumentNotFound, rawCoin)
	}
	qty := decimalOrFallback(rawQty, "0")
	return model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(id.String()),
		Side:         positionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   decimalOrFallback(rawEntry, "0"),
		Timestamp:    parseHLTime(rawTime),
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
		err := fmt.Errorf("%w: hyperliquid execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}
func (c *perpExecutionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.AccountSnapshot{}, err
	}
	balance, err := c.sdk.GetBalance(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	total := decimalOrFallback(balance.MarginSummary.AccountValue, balance.Withdrawable)
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now(), Balances: []model.Balance{{Currency: "USDC", Free: balance.Withdrawable, Total: total.String()}}}
	return snapshot, nil
}
func (c *perpExecutionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderStatusReport{}, err
	}
	assetID, err := c.provider.assetID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	clientID := string(cmd.ClientOrderID)
	status, err := c.sdk.PlaceOrder(ctx, hlperp.PlaceOrderRequest{AssetID: assetID, IsBuy: cmd.Side == model.OrderSideBuy, Price: float64OrZero(cmd.Price), Size: float64OrZero(cmd.Quantity), ClientOrderID: &clientID, OrderType: hlperp.OrderType{Limit: &hlperp.OrderTypeLimit{Tif: toTIF(cmd.TimeInForce)}}})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return c.mapOrderStatus(cmd, status), nil
}
func (c *perpExecutionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	assetID, err := c.provider.assetID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if _, err := c.sdk.CancelOrder(ctx, hlperp.CancelOrderRequest{AssetID: assetID, OrderID: parseOrderID(cmd.OrderID)}); err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: cmd.OrderID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}
func (c *perpExecutionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	orders, err := c.sdk.UserOpenOrders(ctx, c.accountAddr)
	if err != nil {
		return nil, err
	}
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		if order.Coin == raw {
			quantity := decimalOrFallback(firstNonEmpty(order.OrigSz, order.Sz), "0")
			leaves := decimalOrFallback(order.Sz, "0")
			filled := filledQuantity(quantity, leaves)
			status := model.OrderStatusAccepted
			if filled.IsPositive() {
				status = model.OrderStatusPartiallyFilled
			}
			reports = append(reports, model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(fmt.Sprintf("%d", order.Oid)), Status: status, Side: sideFromWire(order.Side), Type: model.OrderTypeLimit, Quantity: quantity, FilledQuantity: filled, LeavesQuantity: leavesQuantity(quantity, filled), Price: decimalOrFallback(order.LimitPx, "0"), LastUpdatedTime: parseHLTime(order.Timestamp)})
		}
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
	position, err := c.sdk.GetBalance(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(position.AssetPositions))
	for _, asset := range position.AssetPositions {
		if asset.Position.Coin != raw {
			continue
		}
		report, err := c.positionReport(asset.Position.Coin, asset.Position.Szi, asset.Position.EntryPx, position.Time)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}
func (c *perpExecutionClient) mapOrderStatus(cmd model.SubmitOrder, status *hlperp.OrderStatus) model.OrderStatusReport {
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

type perpMarketWS interface {
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

type perpAccountWS interface {
	Connect() error
	SubscribeOrderUpdates(string, func([]hlsdk.WsOrderUpdate)) error
	SubscribeUserFills(string, func(hlsdk.WsUserFills)) error
	SubscribeWebData2(string, func(hlperp.PerpPosition)) error
	Close()
}
