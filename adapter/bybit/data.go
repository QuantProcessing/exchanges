package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	bybitsdk "github.com/QuantProcessing/exchanges/sdk/bybit"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

type dataClient struct {
	id       string
	provider *productProvider
	sdk      sdkClient
	ws       publicWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]string
	mu       sync.Mutex
	health   venue.DataHealth
}

func newDataClient(id string, provider *productProvider, sdk sdkClient) *dataClient {
	return &dataClient{
		id:       id,
		provider: provider,
		sdk:      sdk,
		ws:       bybitsdk.NewPublicWSClient(provider.category),
		events:   make(chan model.MarketEvent, 256),
		subs:     make(map[string]model.SubscribeMarketData),
		topics:   make(map[string]string),
	}
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
		return c.ws.Close()
	}
	return nil
}

func (c *dataClient) Health() venue.DataHealth { return c.health }
func (c *dataClient) Events() <-chan model.MarketEvent {
	return c.events
}

func (c *dataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.Ticker{}, err
	}
	ticker, err := c.sdk.GetTicker(ctx, c.provider.category, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	return model.Ticker{
		InstrumentID: id,
		Bid:          decimalOrFallback(ticker.Bid1Price, "0"),
		Ask:          decimalOrFallback(ticker.Ask1Price, "0"),
		Last:         decimalOrFallback(ticker.LastPrice, "0"),
		Timestamp:    parseUnixMillis(defaultString(ticker.Time, ticker.TS)),
	}, nil
}

func (c *dataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	depth, err := c.sdk.GetOrderBook(ctx, c.provider.category, raw, limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseUnixMillisInt(depth.TS)}
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

func (c *dataClient) FetchFundingRate(ctx context.Context, id model.InstrumentID) (model.FundingRate, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.FundingRate{}, err
	}
	tickers, err := c.sdk.GetTickers(ctx, c.provider.category)
	if err != nil {
		return model.FundingRate{}, err
	}
	var ticker bybitsdk.Ticker
	found := false
	for _, row := range tickers {
		if row.Symbol == raw {
			ticker = row
			found = true
			break
		}
	}
	if !found || ticker.FundingRate == "" {
		return model.FundingRate{}, fmt.Errorf("%w: empty Bybit current funding rate for %s", model.ErrInstrumentNotFound, id.String())
	}
	intervalHours, err := strconv.ParseInt(ticker.FundingIntervalHour, 10, 64)
	if err != nil {
		return model.FundingRate{}, fmt.Errorf("invalid Bybit funding interval %q for %s: %w", ticker.FundingIntervalHour, id.String(), err)
	}
	funding := model.FundingRate{
		InstrumentID:    id,
		Rate:            decimalOrFallback(ticker.FundingRate, "0"),
		NextFundingTime: parseUnixMillis(ticker.NextFundingTime),
		FundingInterval: time.Duration(intervalHours) * time.Hour,
		Timestamp:       parseUnixMillis(defaultString(ticker.Time, ticker.TS)),
		InitTime:        time.Now(),
	}
	return funding, funding.Validate()
}

func parseBookLevel(raw []bybitsdk.NumberString) model.OrderBookLevel {
	return model.OrderBookLevel{
		Price: decimal.RequireFromString(string(raw[0])),
		Size:  decimal.RequireFromString(string(raw[1])),
	}
}

func requireTicker(ticker *bybitsdk.Ticker) (*bybitsdk.Ticker, error) {
	if ticker == nil {
		return nil, fmt.Errorf("%w: empty Bybit ticker", model.ErrInstrumentNotFound)
	}
	return ticker, nil
}

func (c *dataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	topic := c.marketTopic(raw, sub)
	if topic == "" {
		return model.ErrNotSupported
	}
	handler := c.marketHandler(sub)
	if handler == nil {
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
		return nil
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeOrderBook, model.MarketDataTypeTradeTick, model.MarketDataTypeQuoteTick, model.MarketDataTypeBar:
		err = c.ws.Subscribe(ctx, topic, handler)
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	c.mu.Lock()
	c.subs[sub.Key()] = sub
	c.topics[sub.Key()] = topic
	c.mu.Unlock()
	return nil
}

func (c *dataClient) UnsubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	topic := c.marketTopic(raw, sub)
	if topic == "" {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	if stored := c.topics[sub.Key()]; stored != "" {
		topic = stored
	}
	delete(c.subs, sub.Key())
	delete(c.topics, sub.Key())
	stillActive := c.topicActiveLocked(topic)
	c.mu.Unlock()
	if stillActive {
		return nil
	}
	if err := c.ws.Unsubscribe(ctx, topic); err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *dataClient) handleTickerPayload(id model.InstrumentID, payload json.RawMessage) {
	var msg struct {
		TS   int64         `json:"ts"`
		Data bybitWSTicker `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	emitTicker, emitQuote := c.tickerFanout(id)
	ts := parseUnixMillisInt(msg.TS)
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{
			InstrumentID: id,
			Bid:          decimalOrFallback(msg.Data.Bid1Price, "0"),
			Ask:          decimalOrFallback(msg.Data.Ask1Price, "0"),
			Last:         decimalOrFallback(msg.Data.LastPrice, "0"),
			Timestamp:    ts,
		}}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     decimalOrFallback(msg.Data.Bid1Price, "0"),
			AskPrice:     decimalOrFallback(msg.Data.Ask1Price, "0"),
			BidSize:      decimalOrFallback(msg.Data.Bid1Size, "0"),
			AskSize:      decimalOrFallback(msg.Data.Ask1Size, "0"),
			Timestamp:    ts,
			InitTime:     ts,
		}})
	}
}

func (c *dataClient) handleBookPayload(id model.InstrumentID, depth int, payload json.RawMessage) {
	msg, err := bybitsdk.DecodeOrderBookMessage(payload)
	if err != nil {
		c.health.LastError = err
		return
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseUnixMillisInt(msg.TS)}
	for _, level := range msg.Data.Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range msg.Data.Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
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

func (c *dataClient) handleTradePayload(id model.InstrumentID, payload json.RawMessage) {
	var msg struct {
		TS   int64             `json:"ts"`
		Data []bybitWSTradeRow `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	for _, trade := range msg.Data {
		tradeID := trade.TradeID
		if tradeID == "" {
			tradeID = fmt.Sprintf("%s:%d:%s:%s", trade.Symbol, trade.TradeTime, trade.Price, trade.Size)
		}
		ts := parseUnixMillisInt(defaultInt64(trade.TradeTime, msg.TS))
		if err := c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  id,
			Price:         decimalOrFallback(trade.Price, "0"),
			Size:          decimalOrFallback(trade.Size, "0"),
			AggressorSide: bybitAggressorSide(trade.Side),
			TradeID:       model.TradeID(tradeID),
			Timestamp:     ts,
			InitTime:      parseUnixMillisInt(msg.TS),
		}}); err != nil {
			return
		}
	}
}

func (c *dataClient) handleKlinePayload(barType model.BarType, payload json.RawMessage) {
	var msg struct {
		TS   int64             `json:"ts"`
		Data []bybitWSKlineRow `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	for _, row := range msg.Data {
		ts := parseUnixMillisInt(defaultInt64(row.Timestamp, defaultInt64(row.End, msg.TS)))
		if err := c.emitMarket(model.MarketEvent{Bar: &model.Bar{
			BarType:    barType.Canonical(),
			Open:       decimalOrFallback(row.Open, "0"),
			High:       decimalOrFallback(row.High, "0"),
			Low:        decimalOrFallback(row.Low, "0"),
			Close:      decimalOrFallback(row.Close, "0"),
			Volume:     decimalOrFallback(row.Volume, "0"),
			Timestamp:  ts,
			InitTime:   parseUnixMillisInt(msg.TS),
			IsRevision: !row.Confirm,
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
		err := fmt.Errorf("%w: bybit market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func (c *dataClient) marketTopic(raw string, sub model.SubscribeMarketData) string {
	switch sub.Type {
	case model.MarketDataTypeTicker:
		return fmt.Sprintf("tickers.%s", raw)
	case model.MarketDataTypeOrderBook:
		return fmt.Sprintf("orderbook.%d.%s", sub.Depth, raw)
	case model.MarketDataTypeQuoteTick:
		if c.provider.category == "spot" {
			return fmt.Sprintf("orderbook.1.%s", raw)
		}
		return fmt.Sprintf("tickers.%s", raw)
	case model.MarketDataTypeTradeTick:
		return fmt.Sprintf("publicTrade.%s", raw)
	case model.MarketDataTypeBar:
		return fmt.Sprintf("kline.%s.%s", bybitBarInterval(sub.BarType.Canonical().Step), raw)
	default:
		return ""
	}
}

func (c *dataClient) marketHandler(sub model.SubscribeMarketData) func(json.RawMessage) {
	switch sub.Type {
	case model.MarketDataTypeTicker:
		return func(payload json.RawMessage) { c.handleTickerPayload(sub.InstrumentID, payload) }
	case model.MarketDataTypeOrderBook:
		return func(payload json.RawMessage) { c.handleBookPayload(sub.InstrumentID, sub.Depth, payload) }
	case model.MarketDataTypeQuoteTick:
		if c.provider.category == "spot" {
			return func(payload json.RawMessage) { c.handleBookPayload(sub.InstrumentID, 1, payload) }
		}
		return func(payload json.RawMessage) { c.handleTickerPayload(sub.InstrumentID, payload) }
	case model.MarketDataTypeTradeTick:
		return func(payload json.RawMessage) { c.handleTradePayload(sub.InstrumentID, payload) }
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		return func(payload json.RawMessage) { c.handleKlinePayload(barType, payload) }
	default:
		return nil
	}
}

func (c *dataClient) topicActiveLocked(topic string) bool {
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

func (c *dataClient) bookFanout(id model.InstrumentID, depth int) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, emitBook := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeOrderBook}.Key()]
	_, emitQuote := c.subs[model.SubscribeMarketData{InstrumentID: id, Type: model.MarketDataTypeQuoteTick}.Key()]
	return emitBook, emitQuote && depth == 1
}

func (c *dataClient) emitQuoteFromBook(book model.OrderBook) error {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		err := fmt.Errorf("%w: bybit quote tick requires top of book", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
	return c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
		InstrumentID: book.InstrumentID,
		BidPrice:     book.Bids[0].Price,
		AskPrice:     book.Asks[0].Price,
		BidSize:      book.Bids[0].Size,
		AskSize:      book.Asks[0].Size,
		Timestamp:    book.Timestamp,
		InitTime:     book.Timestamp,
	}})
}

type bybitWSTicker struct {
	Symbol    string `json:"symbol"`
	LastPrice string `json:"lastPrice"`
	Bid1Price string `json:"bid1Price"`
	Bid1Size  string `json:"bid1Size"`
	Ask1Price string `json:"ask1Price"`
	Ask1Size  string `json:"ask1Size"`
}

type bybitWSTradeRow struct {
	TradeTime int64  `json:"T"`
	Symbol    string `json:"s"`
	Side      string `json:"S"`
	Size      string `json:"v"`
	Price     string `json:"p"`
	TradeID   string `json:"i"`
}

type bybitWSKlineRow struct {
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	Interval  string `json:"interval"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	Turnover  string `json:"turnover"`
	Confirm   bool   `json:"confirm"`
	Timestamp int64  `json:"timestamp"`
}

type publicWS interface {
	Subscribe(context.Context, string, func(json.RawMessage)) error
	Unsubscribe(context.Context, string) error
	Close() error
}
