package nado

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	nadosdk "github.com/QuantProcessing/exchanges/sdk/nado"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetContracts(context.Context, *bool) (nadosdk.ContractV2Map, error)
	GetSymbols(context.Context, *string) (*nadosdk.SymbolsInfo, error)
	GetOrderBook(context.Context, string, int) (*nadosdk.OrderBookV2, error)
	GetAccount(context.Context) (*nadosdk.AccountInfo, error)
	GetAccountProductOrders(context.Context, int64, string) (*nadosdk.AccountProductOrders, error)
	PlaceOrder(context.Context, nadosdk.ClientOrderInput) (*nadosdk.PlaceOrderResponse, error)
	CancelOrders(context.Context, nadosdk.CancelOrdersInput) (*nadosdk.CancelOrdersResponse, error)
}

type perpProvider struct {
	sdk          sdkClient
	insts        map[model.InstrumentID]model.Instrument
	productIDs   map[model.InstrumentID]int64
	tickers      map[model.InstrumentID]string
	productIndex map[int64]model.InstrumentID
	last         map[model.InstrumentID]decimal.Decimal
}

func newPerpProvider(sdk sdkClient) *perpProvider {
	return &perpProvider{sdk: sdk, insts: make(map[model.InstrumentID]model.Instrument), productIDs: make(map[model.InstrumentID]int64), tickers: make(map[model.InstrumentID]string), productIndex: make(map[int64]model.InstrumentID), last: make(map[model.InstrumentID]decimal.Decimal)}
}

func nadoX18Decimal(raw string) (decimal.Decimal, error) {
	if raw == "" {
		return decimal.Zero, nil
	}
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, err
	}
	return value.Div(decimal.NewFromInt(1_000_000_000_000_000_000)), nil
}

func nadoMarginWeight(raw string) (decimal.Decimal, error) {
	weight, err := nadoX18Decimal(raw)
	if err != nil || weight.IsZero() {
		return weight, err
	}
	one := decimal.NewFromInt(1)
	if weight.GreaterThan(one) {
		return weight.Sub(one), nil
	}
	return one.Sub(weight), nil
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	contracts, err := p.sdk.GetContracts(ctx, nil)
	if err != nil {
		return err
	}
	productType := "perp"
	symbols, err := p.sdk.GetSymbols(ctx, &productType)
	if err != nil {
		return err
	}
	symbolsByProductID := make(map[int]nadosdk.Symbol, len(symbols.Symbols))
	for _, symbol := range symbols.Symbols {
		symbolsByProductID[symbol.ProductID] = symbol
	}
	p.insts = make(map[model.InstrumentID]model.Instrument)
	p.productIDs = make(map[model.InstrumentID]int64)
	p.tickers = make(map[model.InstrumentID]string)
	p.productIndex = make(map[int64]model.InstrumentID)
	p.last = make(map[model.InstrumentID]decimal.Decimal)
	for _, contract := range contracts {
		if contract.ProductType != "" && !strings.EqualFold(contract.ProductType, "perp") {
			continue
		}
		symbol := symbolsByProductID[contract.ProductID]
		makerFee, err := nadoX18Decimal(symbol.MakerFeeRateX18)
		if err != nil {
			return err
		}
		takerFee, err := nadoX18Decimal(symbol.TakerFeeRateX18)
		if err != nil {
			return err
		}
		marginInit, err := nadoMarginWeight(symbol.LongWeightInitialX18)
		if err != nil {
			return err
		}
		marginMaint, err := nadoMarginWeight(symbol.LongWeightMaintenanceX18)
		if err != nil {
			return err
		}
		inst := model.Instrument{ID: model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", contract.BaseCurrency, contract.QuoteCurrency), Venue: Venue}, RawSymbol: contract.TickerID, Type: model.InstrumentTypePerp, Base: model.Currency(contract.BaseCurrency), Quote: model.Currency(contract.QuoteCurrency), Settle: model.Currency(contract.QuoteCurrency), PriceTick: decimal.RequireFromString("0.000001"), SizeTick: decimal.RequireFromString("0.000001"), MakerFee: makerFee, TakerFee: takerFee, MarginInit: marginInit, MarginMaint: marginMaint, Status: model.InstrumentStatusTrading}
		if err := inst.Validate(); err != nil {
			return err
		}
		p.insts[inst.ID] = inst
		p.productIDs[inst.ID] = int64(contract.ProductID)
		p.tickers[inst.ID] = contract.TickerID
		p.productIndex[int64(contract.ProductID)] = inst.ID
		p.last[inst.ID] = decimal.NewFromFloat(contract.LastPrice)
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
func (p *perpProvider) productID(id model.InstrumentID) (int64, error) {
	productID, ok := p.productIDs[id]
	if !ok {
		return 0, fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return productID, nil
}
func (p *perpProvider) instrumentIDByProductID(productID int64) (model.InstrumentID, bool) {
	id, ok := p.productIndex[productID]
	return id, ok
}

type dataClient struct {
	id       string
	provider *perpProvider
	sdk      sdkClient
	ws       marketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]nadoMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

type nadoMarketTopic struct {
	kind        model.MarketDataType
	productID   int64
	granularity int32
}

func newDataClient(id string, provider *perpProvider, sdk sdkClient) *dataClient {
	return &dataClient{id: id, provider: provider, sdk: sdk, ws: nadosdk.NewWsMarketClient(context.Background()), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]nadoMarketTopic)}
}
func (c *dataClient) Venue() model.Venue                    { return Venue }
func (c *dataClient) ClientID() string                      { return c.id }
func (c *dataClient) Instruments() venue.InstrumentProvider { return c.provider }
func (c *dataClient) Connect(ctx context.Context) error {
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
	if len(c.provider.List()) == 0 {
		if err := c.provider.LoadAll(ctx); err != nil {
			return model.Ticker{}, err
		}
	}
	mid := c.provider.last[id]
	return model.Ticker{InstrumentID: id, Bid: mid, Ask: mid, Last: mid, Timestamp: time.Now()}, nil
}
func (c *dataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	ticker := c.provider.tickers[id]
	bookRaw, err := c.sdk.GetOrderBook(ctx, ticker, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range bookRaw.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.NewFromFloat(bid[0]), Size: decimal.NewFromFloat(bid[1])})
	}
	for _, ask := range bookRaw.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.NewFromFloat(ask[0]), Size: decimal.NewFromFloat(ask[1])})
	}
	return book, book.Validate()
}
func (c *dataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	productID, err := c.provider.productID(sub.InstrumentID)
	if err != nil {
		return err
	}
	topic := nadoTopicFor(productID, sub)
	if topic.kind == "" {
		return model.ErrNotSupported
	}
	if err := c.ws.Connect(); err != nil {
		c.health.LastError = err
		return err
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
		return nil
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		err = c.ws.SubscribeTicker(productID, func(ticker *nadosdk.Ticker) {
			c.handleTicker(sub.InstrumentID, ticker)
		})
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeOrderBook(productID, func(book *nadosdk.OrderBook) {
			c.handleOrderBook(sub.InstrumentID, book)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeTrades(productID, func(trade *nadosdk.Trade) {
			c.handleTrade(sub.InstrumentID, trade)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeLatestCandlestick(productID, nadoBarGranularity(barType.Step), func(candle *nadosdk.Candlestick) {
			c.handleCandlestick(barType, candle)
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
	return nil
}
func (c *dataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	productID, err := c.provider.productID(sub.InstrumentID)
	if err != nil {
		return err
	}
	topic := nadoTopicFor(productID, sub)
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
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		err = c.ws.UnsubscribeTicker(productID)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeOrderBook(productID)
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeTrades(productID)
	case model.MarketDataTypeBar:
		err = c.ws.UnsubscribeLatestCandlestick(productID, topic.granularity)
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}
func (c *dataClient) handleTicker(id model.InstrumentID, ticker *nadosdk.Ticker) {
	if ticker == nil {
		return
	}
	emitTicker, emitQuote := c.tickerFanout(id)
	ts := parseNadoTime(ticker.Timestamp)
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: decimal.RequireFromString(firstNonEmpty(ticker.BidPrice, "0")), Ask: decimal.RequireFromString(firstNonEmpty(ticker.AskPrice, "0")), Last: decimal.Zero, Timestamp: ts}}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{InstrumentID: id, BidPrice: decimal.RequireFromString(firstNonEmpty(ticker.BidPrice, "0")), AskPrice: decimal.RequireFromString(firstNonEmpty(ticker.AskPrice, "0")), BidSize: decimal.RequireFromString(firstNonEmpty(ticker.BidQty, "0")), AskSize: decimal.RequireFromString(firstNonEmpty(ticker.AskQty, "0")), Timestamp: ts, InitTime: ts}})
	}
}
func (c *dataClient) handleOrderBook(id model.InstrumentID, raw *nadosdk.OrderBook) {
	if raw == nil {
		return
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseNadoTime(firstNonEmpty(raw.MaxTimestamp, raw.MinTimestamp))}
	for _, bid := range raw.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid[0]), Size: decimal.RequireFromString(bid[1])})
	}
	for _, ask := range raw.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask[0]), Size: decimal.RequireFromString(ask[1])})
	}
	_ = c.emitMarket(model.MarketEvent{OrderBook: &book})
}
func (c *dataClient) handleTrade(id model.InstrumentID, trade *nadosdk.Trade) {
	if trade == nil {
		return
	}
	size := firstNonEmpty(trade.TakerQty, trade.MakerQty)
	ts := parseNadoTime(trade.Timestamp)
	_ = c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{InstrumentID: id, Price: decimal.RequireFromString(firstNonEmpty(trade.Price, "0")), Size: decimal.RequireFromString(firstNonEmpty(size, "0")), AggressorSide: nadoAggressorSide(trade.IsTakerBuyer), TradeID: model.TradeID(fmt.Sprintf("%d:%s:%s:%s", trade.ProductId, trade.Timestamp, trade.Price, size)), Timestamp: ts, InitTime: ts}})
}
func (c *dataClient) handleCandlestick(barType model.BarType, candle *nadosdk.Candlestick) {
	if candle == nil {
		return
	}
	open, err := nadoX18Decimal(candle.OpenX18)
	if err != nil {
		c.health.LastError = err
		return
	}
	high, err := nadoX18Decimal(candle.HighX18)
	if err != nil {
		c.health.LastError = err
		return
	}
	low, err := nadoX18Decimal(candle.LowX18)
	if err != nil {
		c.health.LastError = err
		return
	}
	closePrice, err := nadoX18Decimal(candle.CloseX18)
	if err != nil {
		c.health.LastError = err
		return
	}
	ts := parseNadoTime(candle.Timestamp)
	_ = c.emitMarket(model.MarketEvent{Bar: &model.Bar{BarType: barType.Canonical(), Open: open, High: high, Low: low, Close: closePrice, Volume: decimal.RequireFromString(firstNonEmpty(candle.Volume, "0")), Timestamp: ts, InitTime: ts}})
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
		err := fmt.Errorf("%w: nado market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}
func (c *dataClient) topicActiveLocked(topic nadoMarketTopic) bool {
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
	accountID  model.AccountID
	sender     string
	provider   *perpProvider
	sdk        sdkClient
	privateWS  accountWS
	events     chan model.ExecutionEvent
	mu         sync.Mutex
	registered bool
	health     venue.ExecutionHealth
}

func newExecutionClient(accountID model.AccountID, provider *perpProvider, sdk sdkClient, sender string, creds ...string) *executionClient {
	if accountID == "" {
		accountID = "nado-perp"
	}
	client := &executionClient{accountID: accountID, sender: sender, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 1 && creds[0] != "" {
		ws := nadosdk.NewWsAccountClient(context.Background()).WithCredentials(creds[0])
		if len(creds) >= 2 {
			ws.SetSubaccount(creds[1])
		}
		client.privateWS = ws
	}
	return client
}
func (c *executionClient) Venue() model.Venue         { return Venue }
func (c *executionClient) AccountID() model.AccountID { return c.accountID }
func (c *executionClient) Connect(ctx context.Context) error {
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
	if err := c.privateWS.SubscribeOrders(nil, c.handleOrderUpdate); err != nil {
		return err
	}
	if err := c.privateWS.SubscribeFills(nil, c.handleFill); err != nil {
		return err
	}
	if err := c.privateWS.SubscribePositions(nil, c.handlePosition); err != nil {
		return err
	}
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}
func (c *executionClient) handleOrderUpdate(order *nadosdk.OrderUpdate) {
	if order == nil {
		return
	}
	id, ok := c.provider.instrumentIDByProductID(order.ProductId)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: product %d", model.ErrInstrumentNotFound, order.ProductId)
		return
	}
	report := model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(order.Digest), Status: mapNadoOrderStatus(order.Reason), Type: model.OrderTypeLimit, Quantity: decimal.RequireFromString(firstNonEmpty(order.Amount, "0")), LastUpdatedTime: parseNadoTime(order.Timestamp)}
	_ = c.emitExecution(model.ExecutionEvent{Order: &report})
}
func (c *executionClient) handleFill(fill *nadosdk.Fill) {
	if fill == nil {
		return
	}
	id, ok := c.provider.instrumentIDByProductID(fill.ProductId)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: product %d", model.ErrInstrumentNotFound, fill.ProductId)
		return
	}
	orderID := firstNonEmpty(fill.Digest, fill.OrderID)
	if orderID == "" {
		c.health.LastError = fmt.Errorf("%w: nado fill %s missing order identity", model.ErrInvalidOrder, fill.TradeId)
		return
	}
	report := model.FillReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(orderID), TradeID: model.TradeID(fill.TradeId), Side: fromNadoSide(fill.Side), Price: decimal.RequireFromString(firstNonEmpty(fill.Price, "0")), Quantity: decimal.RequireFromString(firstNonEmpty(fill.Size, "0")), Fee: decimal.RequireFromString(firstNonEmpty(fill.Fee, "0")).Abs(), FeeCurrency: model.Currency("USDC"), Timestamp: parseNadoUnix(fill.Time)}
	_ = c.emitExecution(model.ExecutionEvent{Fill: &report})
}
func (c *executionClient) handlePosition(position *nadosdk.PositionChange) {
	if position == nil {
		return
	}
	id, ok := c.provider.instrumentIDByProductID(position.ProductId)
	if !ok {
		c.health.LastError = fmt.Errorf("%w: product %d", model.ErrInstrumentNotFound, position.ProductId)
		return
	}
	qty := decimal.RequireFromString(firstNonEmpty(position.Amount, "0"))
	report := model.PositionStatusReport{AccountID: c.accountID, InstrumentID: id, PositionID: model.PositionID(id.String()), Side: nadoPositionSide(qty, position.Side), Quantity: qty.Abs(), EntryPrice: decimal.RequireFromString(firstNonEmpty(position.EntryPrice, "0")), Timestamp: time.Now()}
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
		err := fmt.Errorf("%w: nado execution event channel full", model.ErrInvalidExecutionEvent)
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
	for _, bal := range account.PerpBalances {
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: "USDC", Free: bal.Balance.Amount, Total: bal.Balance.Amount})
	}
	return snapshot, nil
}
func (c *executionClient) SubmitOrder(ctx context.Context, cmd model.SubmitOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	productID, err := c.provider.productID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	orderType := nadosdk.OrderTypeLimit
	if cmd.Type == model.OrderTypeMarket {
		orderType = nadosdk.OrderTypeMarket
	}
	resp, err := c.sdk.PlaceOrder(ctx, nadosdk.ClientOrderInput{ProductId: productID, Price: cmd.Price.String(), Amount: cmd.Quantity.String(), Side: nadoSide(cmd.Side), OrderType: orderType})
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: model.OrderID(resp.Digest), ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusAccepted, Side: cmd.Side, Type: cmd.Type, Quantity: cmd.Quantity, Price: cmd.Price, LastUpdatedTime: time.Now()}, nil
}
func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	productID, err := c.provider.productID(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	if _, err := c.sdk.CancelOrders(ctx, nadosdk.CancelOrdersInput{ProductIds: []int64{productID}, Digests: []string{string(cmd.OrderID)}}); err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: cmd.OrderID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}
func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	productID, err := c.provider.productID(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetAccountProductOrders(ctx, productID, c.sender)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders.Orders))
	for _, order := range orders.Orders {
		reports = append(reports, model.OrderStatusReport{AccountID: c.accountID, InstrumentID: id, OrderID: model.OrderID(order.Digest), Status: model.OrderStatusAccepted, Type: model.OrderTypeLimit, Quantity: decimal.RequireFromString(order.Amount), FilledQuantity: decimal.RequireFromString("0"), Price: decimal.RequireFromString(order.PriceX18), LastUpdatedTime: time.Now()})
	}
	return reports, nil
}

type Adapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := nadosdk.NewClient()
	if opts.PrivateKey != "" {
		var err error
		client, err = client.WithCredentials(opts.PrivateKey, opts.Subaccount)
		if err != nil {
			return nil, err
		}
	}
	provider := newPerpProvider(client)
	return &Adapter{provider: provider, data: newDataClient("nado-perp-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, opts.Sender, opts.PrivateKey, opts.Subaccount)}, nil
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
	return venue.DeclaredCapabilities{Venue: Venue, Instruments: true, MarketData: venue.MarketDataCapabilities{Snapshots: true, Ticker: true, OrderBook: true, TickerStream: true, OrderBookStream: true, TradeTicks: true, QuoteTicks: true, Bars: true, Streams: true}, Execution: venue.ExecutionCapabilities{Submit: true, Cancel: true, OrderReports: true, PrivateStream: true, Resubscribe: true}, Account: venue.AccountCapabilities{Snapshot: true}}
}

func nadoTopicFor(productID int64, sub model.SubscribeMarketData) nadoMarketTopic {
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		return nadoMarketTopic{kind: model.MarketDataTypeTicker, productID: productID}
	case model.MarketDataTypeOrderBook:
		return nadoMarketTopic{kind: model.MarketDataTypeOrderBook, productID: productID}
	case model.MarketDataTypeTradeTick:
		return nadoMarketTopic{kind: model.MarketDataTypeTradeTick, productID: productID}
	case model.MarketDataTypeBar:
		return nadoMarketTopic{kind: model.MarketDataTypeBar, productID: productID, granularity: nadoBarGranularity(sub.BarType.Canonical().Step)}
	default:
		return nadoMarketTopic{}
	}
}

func nadoBarGranularity(step time.Duration) int32 {
	switch step {
	case time.Minute:
		return 60
	case 5 * time.Minute:
		return 300
	case 15 * time.Minute:
		return 900
	case 30 * time.Minute:
		return 1800
	case time.Hour:
		return 3600
	case 4 * time.Hour:
		return 14400
	case 24 * time.Hour:
		return 86400
	default:
		return 60
	}
}

func nadoAggressorSide(isTakerBuyer bool) model.AggressorSide {
	if isTakerBuyer {
		return model.AggressorSideBuyer
	}
	return model.AggressorSideSeller
}

func nadoSide(side model.OrderSide) nadosdk.OrderSide {
	if side == model.OrderSideSell {
		return nadosdk.OrderSideSell
	}
	return nadosdk.OrderSideBuy
}

func fromNadoSide(side string) model.OrderSide {
	if strings.EqualFold(side, string(nadosdk.OrderSideSell)) || strings.EqualFold(side, "short") {
		return model.OrderSideSell
	}
	return model.OrderSideBuy
}

func mapNadoOrderStatus(reason string) model.OrderStatus {
	switch strings.ToLower(reason) {
	case "filled":
		return model.OrderStatusFilled
	case "canceled", "cancelled":
		return model.OrderStatusCanceled
	case "rejected":
		return model.OrderStatusRejected
	default:
		return model.OrderStatusAccepted
	}
}

func nadoPositionSide(qty decimal.Decimal, raw string) model.PositionSide {
	switch strings.ToLower(raw) {
	case "short", "sell":
		return model.PositionSideShort
	case "long", "buy":
		return model.PositionSideLong
	}
	if qty.IsNegative() {
		return model.PositionSideShort
	}
	if qty.IsPositive() {
		return model.PositionSideLong
	}
	return model.PositionSideFlat
}

func parseNadoTime(raw string) time.Time {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return time.Now()
	}
	return parseNadoUnix(value)
}

func parseNadoUnix(value int64) time.Time {
	switch {
	case value > 1_000_000_000_000_000:
		return time.UnixMicro(value)
	case value > 1_000_000_000_000:
		return time.UnixMilli(value)
	case value > 1_000_000_000:
		return time.Unix(value, 0)
	default:
		return time.UnixMilli(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type marketWS interface {
	Connect() error
	SubscribeTicker(int64, func(*nadosdk.Ticker)) error
	SubscribeOrderBook(int64, func(*nadosdk.OrderBook)) error
	SubscribeTrades(int64, func(*nadosdk.Trade)) error
	SubscribeLatestCandlestick(int64, int32, func(*nadosdk.Candlestick)) error
	UnsubscribeTicker(int64) error
	UnsubscribeOrderBook(int64) error
	UnsubscribeTrades(int64) error
	UnsubscribeLatestCandlestick(int64, int32) error
	Close()
}

type accountWS interface {
	Connect() error
	SubscribeOrders(*int64, func(*nadosdk.OrderUpdate)) error
	SubscribeFills(*int64, func(*nadosdk.Fill)) error
	SubscribePositions(*int64, func(*nadosdk.PositionChange)) error
	Close()
}
