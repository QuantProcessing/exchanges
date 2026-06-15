package edgex

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	edgexperp "github.com/QuantProcessing/exchanges/sdk/edgex/perp"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type sdkClient interface {
	GetExchangeInfo(context.Context) (*edgexperp.ExchangeInfo, error)
	GetTicker(context.Context, string) (*edgexperp.Ticker, error)
	GetOrderBook(context.Context, string, int) (*edgexperp.OrderBook, error)
	GetAccountAsset(context.Context) (*edgexperp.AccountAsset, error)
	GetOpenOrders(context.Context, *string) ([]edgexperp.Order, error)
	PlaceOrder(context.Context, edgexperp.PlaceOrderParams, *edgexperp.Contract, *edgexperp.Coin) (*edgexperp.CreateOrderData, error)
	CancelOrder(context.Context, string) (*edgexperp.CancelOrderData, error)
}

type perpProvider struct {
	sdk           sdkClient
	insts         map[model.InstrumentID]model.Instrument
	contractIDs   map[model.InstrumentID]string
	contractIndex map[string]model.InstrumentID
	contracts     map[model.InstrumentID]edgexperp.Contract
	quoteCoins    map[model.InstrumentID]edgexperp.Coin
	coins         map[string]edgexperp.Coin
}

func newPerpProvider(sdk sdkClient) *perpProvider {
	return &perpProvider{
		sdk:           sdk,
		insts:         make(map[model.InstrumentID]model.Instrument),
		contractIDs:   make(map[model.InstrumentID]string),
		contractIndex: make(map[string]model.InstrumentID),
		contracts:     make(map[model.InstrumentID]edgexperp.Contract),
		quoteCoins:    make(map[model.InstrumentID]edgexperp.Coin),
		coins:         make(map[string]edgexperp.Coin),
	}
}

func (p *perpProvider) LoadAll(ctx context.Context) error {
	info, err := p.sdk.GetExchangeInfo(ctx)
	if err != nil {
		return err
	}
	coins := make(map[string]edgexperp.Coin, len(info.CoinList))
	for _, coin := range info.CoinList {
		coins[coin.CoinId] = coin
	}
	insts := make(map[model.InstrumentID]model.Instrument)
	contractIDs := make(map[model.InstrumentID]string)
	contractIndex := make(map[string]model.InstrumentID)
	contracts := make(map[model.InstrumentID]edgexperp.Contract)
	quoteCoins := make(map[model.InstrumentID]edgexperp.Coin)
	for _, contract := range info.ContractList {
		base, ok := coins[contract.BaseCoinId]
		if !ok {
			return fmt.Errorf("%w: missing EdgeX base coin %s", model.ErrInvalidInstrument, contract.BaseCoinId)
		}
		quote, ok := coins[contract.QuoteCoinId]
		if !ok {
			return fmt.Errorf("%w: missing EdgeX quote coin %s", model.ErrInvalidInstrument, contract.QuoteCoinId)
		}
		priceTick, err := decimalFromString(contract.TickSize, "0.000001")
		if err != nil {
			return err
		}
		sizeTick, err := decimalFromString(contract.StepSize, "0.000001")
		if err != nil {
			return err
		}
		makerFee, err := decimalFromString(contract.DefaultMakerFeeRate, "0")
		if err != nil {
			return err
		}
		takerFee, err := decimalFromString(contract.DefaultTakerFeeRate, "0")
		if err != nil {
			return err
		}
		marginInit, err := edgeXInitialMargin(contract)
		if err != nil {
			return err
		}
		marginMaint, err := edgeXMaintenanceMargin(contract)
		if err != nil {
			return err
		}
		baseName := strings.ToUpper(base.CoinName)
		quoteName := strings.ToUpper(quote.CoinName)
		id := model.InstrumentID{Symbol: fmt.Sprintf("%s-%s-PERP", baseName, quoteName), Venue: Venue}
		rawSymbol := contract.ContractName
		if rawSymbol == "" {
			rawSymbol = contract.ContractId
		}
		inst := model.Instrument{
			ID:          id,
			RawSymbol:   rawSymbol,
			Type:        model.InstrumentTypePerp,
			Base:        model.Currency(baseName),
			Quote:       model.Currency(quoteName),
			Settle:      model.Currency(quoteName),
			PriceTick:   priceTick,
			SizeTick:    sizeTick,
			MakerFee:    makerFee,
			TakerFee:    takerFee,
			MarginInit:  marginInit,
			MarginMaint: marginMaint,
			Status:      mapInstrumentStatus(contract.EnableTrade),
		}
		if err := inst.Validate(); err != nil {
			return err
		}
		insts[id] = inst
		contractIDs[id] = contract.ContractId
		contractIndex[contract.ContractId] = id
		contracts[id] = contract
		quoteCoins[id] = quote
	}
	p.insts = insts
	p.contractIDs = contractIDs
	p.contractIndex = contractIndex
	p.contracts = contracts
	p.quoteCoins = quoteCoins
	p.coins = coins
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

func (p *perpProvider) contractID(id model.InstrumentID) (string, error) {
	contractID, ok := p.contractIDs[id]
	if !ok {
		return "", fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	return contractID, nil
}

func (p *perpProvider) instrumentIDByContractID(contractID string) (model.InstrumentID, bool) {
	id, ok := p.contractIndex[contractID]
	return id, ok
}

func (p *perpProvider) contractAndQuote(id model.InstrumentID) (*edgexperp.Contract, *edgexperp.Coin, error) {
	contract, ok := p.contracts[id]
	if !ok {
		return nil, nil, fmt.Errorf("%w: %s", model.ErrInstrumentNotFound, id.String())
	}
	quote, ok := p.quoteCoins[id]
	if !ok {
		return nil, nil, fmt.Errorf("%w: missing EdgeX quote coin for %s", model.ErrInvalidInstrument, id.String())
	}
	return &contract, &quote, nil
}

func (p *perpProvider) coinName(coinID string) string {
	if coin, ok := p.coins[coinID]; ok && coin.CoinName != "" {
		return strings.ToUpper(coin.CoinName)
	}
	return strings.ToUpper(coinID)
}

type dataClient struct {
	id       string
	provider *perpProvider
	sdk      sdkClient
	ws       marketWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]edgeXMarketTopic
	mu       sync.Mutex
	health   venue.DataHealth
}

type edgeXMarketTopic struct {
	kind      model.MarketDataType
	contract  string
	depth     edgexperp.OrderBookDepth
	priceType edgexperp.PriceType
	interval  edgexperp.KlineInterval
}

func newDataClient(id string, provider *perpProvider, sdk sdkClient) *dataClient {
	return &dataClient{id: id, provider: provider, sdk: sdk, ws: edgexperp.NewWsMarketClient(context.Background()), events: make(chan model.MarketEvent, 256), subs: make(map[string]model.SubscribeMarketData), topics: make(map[string]edgeXMarketTopic)}
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

func (c *dataClient) Events() <-chan model.MarketEvent { return c.events }

func (c *dataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.Ticker{}, err
	}
	contractID, err := c.provider.contractID(id)
	if err != nil {
		return model.Ticker{}, err
	}
	raw, err := c.sdk.GetTicker(ctx, contractID)
	if err != nil {
		return model.Ticker{}, err
	}
	last, err := decimalFromString(firstNonEmpty(raw.LastPrice, raw.Close), "0")
	if err != nil {
		return model.Ticker{}, err
	}
	ticker := model.Ticker{InstrumentID: id, Bid: last, Ask: last, Last: last, Timestamp: time.Now()}
	return ticker, ticker.Validate()
}

func (c *dataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.OrderBook{}, err
	}
	contractID, err := c.provider.contractID(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	raw, err := c.sdk.GetOrderBook(ctx, contractID, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range raw.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid.Price), Size: decimal.RequireFromString(bid.Size)})
	}
	for _, ask := range raw.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask.Price), Size: decimal.RequireFromString(ask.Size)})
	}
	return book, book.Validate()
}

func (c *dataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	if err := c.provider.ensureLoaded(ctx); err != nil {
		c.health.LastError = err
		return err
	}
	contractID, err := c.provider.contractID(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := edgeXTopicFor(contractID, sub)
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
		c.health.LastError = nil
		return nil
	}
	switch sub.Type {
	case model.MarketDataTypeTicker:
		err = c.ws.SubscribeTicker(contractID, func(event *edgexperp.WsTickerEvent) {
			c.handleTicker(sub.InstrumentID, event)
		})
	case model.MarketDataTypeOrderBook:
		depth := edgeXBookDepth(sub.Depth)
		err = c.ws.SubscribeOrderBook(contractID, depth, func(event *edgexperp.WsDepthEvent) {
			c.handleOrderBook(sub.InstrumentID, depth, event)
		})
	case model.MarketDataTypeQuoteTick:
		depth := edgexperp.OrderBookDepth15
		err = c.ws.SubscribeOrderBook(contractID, depth, func(event *edgexperp.WsDepthEvent) {
			c.handleOrderBook(sub.InstrumentID, depth, event)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeTrades(contractID, func(event *edgexperp.WsTradeEvent) {
			c.handleTrade(sub.InstrumentID, event)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeKline(contractID, edgexperp.PriceTypeLastPrice, edgeXKlineInterval(barType.Step), func(event *edgexperp.WsKlineEvent) {
			c.handleKline(barType, event)
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
	contractID, err := c.provider.contractID(sub.InstrumentID)
	if err != nil {
		c.health.LastError = err
		return err
	}
	topic := edgeXTopicFor(contractID, sub)
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
	case model.MarketDataTypeTicker:
		err = c.ws.UnsubscribeTicker(contractID)
	case model.MarketDataTypeOrderBook:
		err = c.ws.UnsubscribeOrderBook(contractID, edgeXBookDepth(sub.Depth))
	case model.MarketDataTypeQuoteTick:
		err = c.ws.UnsubscribeOrderBook(contractID, edgexperp.OrderBookDepth15)
	case model.MarketDataTypeTradeTick:
		err = c.ws.UnsubscribeTrades(contractID)
	case model.MarketDataTypeBar:
		err = c.ws.UnsubscribeKline(contractID, edgexperp.PriceTypeLastPrice, topic.interval)
	default:
		err = model.ErrNotSupported
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *dataClient) handleTicker(id model.InstrumentID, event *edgexperp.WsTickerEvent) {
	if event == nil || len(event.Content.Data) == 0 {
		return
	}
	raw := event.Content.Data[0]
	last := decimal.RequireFromString(firstNonEmpty(raw.LastPrice, raw.Close, "0"))
	_ = c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{InstrumentID: id, Bid: last, Ask: last, Last: last, Timestamp: time.Now()}})
}

func (c *dataClient) handleOrderBook(id model.InstrumentID, depth edgexperp.OrderBookDepth, event *edgexperp.WsDepthEvent) {
	if event == nil || len(event.Content.Data) == 0 {
		return
	}
	raw := event.Content.Data[0]
	book := model.OrderBook{InstrumentID: id, Timestamp: time.Now()}
	for _, bid := range raw.Bids {
		book.Bids = append(book.Bids, model.OrderBookLevel{Price: decimal.RequireFromString(bid.Price), Size: decimal.RequireFromString(bid.Size)})
	}
	for _, ask := range raw.Asks {
		book.Asks = append(book.Asks, model.OrderBookLevel{Price: decimal.RequireFromString(ask.Price), Size: decimal.RequireFromString(ask.Size)})
	}
	emitBook, emitQuote := c.bookFanout(id, depth)
	if emitBook {
		if err := c.emitMarket(model.MarketEvent{OrderBook: &book}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitQuoteFromBook(book)
	}
}

func (c *dataClient) handleTrade(id model.InstrumentID, event *edgexperp.WsTradeEvent) {
	if event == nil || len(event.Content.Data) == 0 {
		return
	}
	for _, raw := range event.Content.Data {
		ts := parseEdgeXTime(raw.Time)
		if err := c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  id,
			Price:         decimal.RequireFromString(firstNonEmpty(raw.Price, "0")),
			Size:          decimal.RequireFromString(firstNonEmpty(raw.Size, "0")),
			AggressorSide: edgeXAggressorSide(raw.IsBuyerMaker),
			TradeID:       model.TradeID(firstNonEmpty(raw.TicketId, fmt.Sprintf("%s:%s:%s", raw.Time, raw.Price, raw.Size))),
			Timestamp:     ts,
			InitTime:      ts,
		}}); err != nil {
			return
		}
	}
}

func (c *dataClient) handleKline(barType model.BarType, event *edgexperp.WsKlineEvent) {
	if event == nil || len(event.Content.Data) == 0 {
		return
	}
	for _, raw := range event.Content.Data {
		ts := parseEdgeXTime(raw.KlineTime)
		if err := c.emitMarket(model.MarketEvent{Bar: &model.Bar{
			BarType:   barType.Canonical(),
			Open:      decimal.RequireFromString(firstNonEmpty(raw.Open, "0")),
			High:      decimal.RequireFromString(firstNonEmpty(raw.High, "0")),
			Low:       decimal.RequireFromString(firstNonEmpty(raw.Low, "0")),
			Close:     decimal.RequireFromString(firstNonEmpty(raw.Close, "0")),
			Volume:    decimal.RequireFromString(firstNonEmpty(raw.Size, "0")),
			Timestamp: ts,
			InitTime:  ts,
		}}); err != nil {
			return
		}
	}
}

func (c *dataClient) emitQuoteFromBook(book model.OrderBook) error {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		err := fmt.Errorf("%w: edgex quote tick requires top of book", model.ErrInvalidMarketData)
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
		err := fmt.Errorf("%w: edgex market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *dataClient) topicActiveLocked(topic edgeXMarketTopic) bool {
	for _, activeTopic := range c.topics {
		if activeTopic == topic {
			return true
		}
	}
	return false
}

func (c *dataClient) bookFanout(id model.InstrumentID, depth edgexperp.OrderBookDepth) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, emitBook := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeOrderBook}.Key()]
	_, emitQuote := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()]
	return emitBook, emitQuote && depth == edgexperp.OrderBookDepth15
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

func newExecutionClient(accountID model.AccountID, provider *perpProvider, sdk sdkClient, creds ...string) *executionClient {
	if accountID == "" {
		accountID = "edgex-perp"
	}
	client := &executionClient{accountID: accountID, provider: provider, sdk: sdk, events: make(chan model.ExecutionEvent, 256)}
	if len(creds) >= 2 && creds[0] != "" && creds[1] != "" {
		client.privateWS = edgexperp.NewWsAccountClient(context.Background(), creds[0], creds[1])
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
	c.privateWS.SubscribeOrderUpdate(c.handleOrderUpdates)
	c.privateWS.SubscribeOrderFillUpdate(c.handleFillUpdates)
	c.privateWS.SubscribePositionUpdate(c.handlePositionUpdates)
	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()
	return nil
}

func (c *executionClient) handleOrderUpdates(orders []edgexperp.Order) {
	for _, order := range orders {
		id, ok := c.provider.instrumentIDByContractID(order.ContractId)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: edgex contract %s", model.ErrInstrumentNotFound, order.ContractId)
			continue
		}
		report := c.orderReport(id, order)
		_ = c.emitExecution(model.ExecutionEvent{Order: &report})
	}
}

func (c *executionClient) handleFillUpdates(fills []edgexperp.OrderFillTransaction) {
	for _, fill := range fills {
		id, ok := c.provider.instrumentIDByContractID(fill.ContractId)
		if !ok {
			c.health.LastError = fmt.Errorf("%w: edgex contract %s", model.ErrInstrumentNotFound, fill.ContractId)
			continue
		}
		inst, _ := c.provider.Get(id)
		report := model.FillReport{
			AccountID:    c.accountID,
			InstrumentID: id,
			OrderID:      model.OrderID(fill.OrderId),
			TradeID:      model.TradeID(fill.Id),
			Side:         fromVenueSide(edgexperp.Side(fill.OrderSide)),
			Price:        decimal.RequireFromString(firstNonEmpty(fill.FillPrice, "0")),
			Quantity:     decimal.RequireFromString(firstNonEmpty(fill.FillSize, "0")),
			Fee:          decimal.RequireFromString(firstNonEmpty(fill.FillFee, "0")).Abs(),
			FeeCurrency:  inst.Quote,
			Timestamp:    parseEdgeXTime(firstNonEmpty(fill.MatchTime, fill.UpdatedTime, fill.CreatedTime)),
		}
		_ = c.emitExecution(model.ExecutionEvent{Fill: &report})
	}
}

func (c *executionClient) handlePositionUpdates(positions []edgexperp.PositionInfo) {
	for _, position := range positions {
		report, err := c.positionReport(position)
		if err != nil {
			c.health.LastError = err
			continue
		}
		_ = c.emitExecution(model.ExecutionEvent{Position: &report})
	}
}

func (c *executionClient) orderReport(id model.InstrumentID, order edgexperp.Order) model.OrderStatusReport {
	quantity := decimal.RequireFromString(firstNonEmpty(order.Size, "0"))
	filled := decimal.RequireFromString(firstNonEmpty(order.CumFillSize, "0"))
	avg := decimal.Zero
	fillValue := decimal.RequireFromString(firstNonEmpty(order.CumFillValue, "0"))
	if filled.IsPositive() && fillValue.IsPositive() {
		avg = fillValue.Div(filled)
	}
	return model.OrderStatusReport{
		AccountID:       c.accountID,
		InstrumentID:    id,
		OrderID:         model.OrderID(order.Id),
		ClientOrderID:   model.ClientOrderID(order.ClientOrderId),
		Status:          mapOrderStatus(order.Status),
		Side:            fromVenueSide(order.Side),
		Type:            fromVenueOrderType(order.Type),
		Quantity:        quantity,
		FilledQuantity:  filled,
		LeavesQuantity:  leavesQuantity(quantity, filled),
		Price:           decimal.RequireFromString(firstNonEmpty(order.Price, "0")),
		AveragePrice:    avg,
		LastUpdatedTime: parseEdgeXTime(firstNonEmpty(order.UpdatedTime, order.CreatedTime)),
	}
}

func (c *executionClient) positionReport(position edgexperp.PositionInfo) (model.PositionStatusReport, error) {
	id, ok := c.provider.instrumentIDByContractID(position.ContractId)
	if !ok {
		return model.PositionStatusReport{}, fmt.Errorf("%w: edgex contract %s", model.ErrInstrumentNotFound, position.ContractId)
	}
	qty := decimal.RequireFromString(firstNonEmpty(position.OpenSize, "0"))
	entry := decimal.Zero
	value := decimal.RequireFromString(firstNonEmpty(position.OpenValue, "0"))
	if !qty.IsZero() && value.IsPositive() {
		entry = value.Div(qty.Abs())
	}
	report := model.PositionStatusReport{
		AccountID:    c.accountID,
		InstrumentID: id,
		PositionID:   model.PositionID(id.String()),
		Side:         positionSide(qty),
		Quantity:     qty.Abs(),
		EntryPrice:   entry,
		Timestamp:    parseEdgeXTime(firstNonEmpty(position.UpdatedTime, position.CreatedTime)),
	}
	return report, nil
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
		err := fmt.Errorf("%w: edgex execution event channel full", model.ErrInvalidExecutionEvent)
		c.health.LastError = err
		return err
	}
}

func (c *executionClient) QueryAccount(ctx context.Context) (model.AccountSnapshot, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return model.AccountSnapshot{}, err
	}
	account, err := c.sdk.GetAccountAsset(ctx)
	if err != nil {
		return model.AccountSnapshot{}, err
	}
	freeByCoin := make(map[string]string, len(account.CollateralAssetModelList))
	totalByCoin := make(map[string]string, len(account.CollateralAssetModelList))
	for _, asset := range account.CollateralAssetModelList {
		freeByCoin[asset.CoinId] = asset.AvailableAmount
		totalByCoin[asset.CoinId] = asset.TotalEquity
	}
	snapshot := model.AccountSnapshot{AccountID: c.accountID, Venue: Venue, Timestamp: time.Now()}
	seen := make(map[string]bool)
	for _, collateral := range account.CollateralList {
		total := firstNonEmpty(totalByCoin[collateral.CoinId], collateral.Amount)
		free := firstNonEmpty(freeByCoin[collateral.CoinId], collateral.Amount)
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(c.provider.coinName(collateral.CoinId)), Free: free, Locked: lockedAmount(total, free), Total: total})
		seen[collateral.CoinId] = true
	}
	for _, asset := range account.CollateralAssetModelList {
		if seen[asset.CoinId] {
			continue
		}
		total := asset.TotalEquity
		free := asset.AvailableAmount
		snapshot.Balances = append(snapshot.Balances, model.Balance{Currency: model.Currency(c.provider.coinName(asset.CoinId)), Free: free, Locked: lockedAmount(total, free), Total: total})
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
	contract, quote, err := c.provider.contractAndQuote(cmd.InstrumentID)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	params := edgexperp.PlaceOrderParams{
		ContractId:    contract.ContractId,
		Side:          toVenueSide(cmd.Side),
		Type:          toVenueOrderType(cmd.Type),
		Quantity:      cmd.Quantity.String(),
		Price:         zeroBlank(cmd.Price),
		ClientOrderId: string(cmd.ClientOrderID),
		TimeInForce:   toVenueTIF(cmd.TimeInForce),
	}
	resp, err := c.sdk.PlaceOrder(ctx, params, contract, quote)
	if err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: model.OrderID(resp.OrderId), ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusAccepted, Side: cmd.Side, Type: cmd.Type, Quantity: cmd.Quantity, Price: cmd.Price, LastUpdatedTime: time.Now()}, nil
}

func (c *executionClient) CancelOrder(ctx context.Context, cmd model.CancelOrder) (model.OrderStatusReport, error) {
	if err := cmd.Validate(); err != nil {
		return model.OrderStatusReport{}, err
	}
	if _, err := c.sdk.CancelOrder(ctx, string(cmd.OrderID)); err != nil {
		return model.OrderStatusReport{}, err
	}
	return model.OrderStatusReport{AccountID: c.accountID, InstrumentID: cmd.InstrumentID, OrderID: cmd.OrderID, ClientOrderID: cmd.ClientOrderID, Status: model.OrderStatusCanceled, LastUpdatedTime: time.Now()}, nil
}

func (c *executionClient) GenerateOrderStatusReports(ctx context.Context, id model.InstrumentID) ([]model.OrderStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	contractID, err := c.provider.contractID(id)
	if err != nil {
		return nil, err
	}
	orders, err := c.sdk.GetOpenOrders(ctx, &contractID)
	if err != nil {
		return nil, err
	}
	reports := make([]model.OrderStatusReport, 0, len(orders))
	for _, order := range orders {
		reports = append(reports, c.orderReport(id, order))
	}
	return reports, nil
}

func (c *executionClient) GeneratePositionStatusReports(ctx context.Context, id model.InstrumentID) ([]model.PositionStatusReport, error) {
	if err := c.provider.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	contractID, err := c.provider.contractID(id)
	if err != nil {
		return nil, err
	}
	account, err := c.sdk.GetAccountAsset(ctx)
	if err != nil {
		return nil, err
	}
	reports := make([]model.PositionStatusReport, 0, len(account.PositionList))
	for _, position := range account.PositionList {
		if position.ContractId != contractID {
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

type Adapter struct {
	provider *perpProvider
	data     venue.DataClient
	exec     venue.ExecutionClient
}

func NewPerpAdapter(_ context.Context, opts Options) (*Adapter, error) {
	client := edgexperp.NewClient()
	if opts.StarkPrivateKey != "" || opts.ExchangeAccountID != "" {
		client = client.WithCredentials(opts.StarkPrivateKey, opts.ExchangeAccountID)
	}
	provider := newPerpProvider(client)
	return &Adapter{provider: provider, data: newDataClient("edgex-perp-data", provider, client), exec: newExecutionClient(opts.AccountID, provider, client, opts.StarkPrivateKey, opts.ExchangeAccountID)}, nil
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

type marketWS interface {
	Connect() error
	SubscribeTicker(string, func(*edgexperp.WsTickerEvent)) error
	SubscribeOrderBook(string, edgexperp.OrderBookDepth, func(*edgexperp.WsDepthEvent)) error
	SubscribeTrades(string, func(*edgexperp.WsTradeEvent)) error
	SubscribeKline(string, edgexperp.PriceType, edgexperp.KlineInterval, func(*edgexperp.WsKlineEvent)) error
	UnsubscribeTicker(string) error
	UnsubscribeOrderBook(string, edgexperp.OrderBookDepth) error
	UnsubscribeTrades(string) error
	UnsubscribeKline(string, edgexperp.PriceType, edgexperp.KlineInterval) error
	Close()
}

type accountWS interface {
	Connect() error
	SubscribeOrderUpdate(func([]edgexperp.Order))
	SubscribeOrderFillUpdate(func([]edgexperp.OrderFillTransaction))
	SubscribePositionUpdate(func([]edgexperp.PositionInfo))
	Close()
}
