package bitget

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	bitgetsdk "github.com/QuantProcessing/exchanges/sdk/bitget"
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
	topics   map[string]bitgetsdk.WSArg
	mu       sync.Mutex
	health   venue.DataHealth
}

func newDataClient(id string, provider *productProvider, sdk sdkClient) *dataClient {
	return &dataClient{
		id:       id,
		provider: provider,
		sdk:      sdk,
		ws:       bitgetsdk.NewPublicWSClient(),
		events:   make(chan model.MarketEvent, 256),
		subs:     make(map[string]model.SubscribeMarketData),
		topics:   make(map[string]bitgetsdk.WSArg),
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
		Timestamp:    time.Now(),
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

func parseBookLevel(raw []bitgetsdk.NumberString) model.OrderBookLevel {
	return model.OrderBookLevel{
		Price: decimal.RequireFromString(string(raw[0])),
		Size:  decimal.RequireFromString(string(raw[1])),
	}
}

func (c *dataClient) SubscribeMarketData(ctx context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	arg := bitgetMarketArg(c.provider.category, raw, sub)
	if bitgetArgEmpty(arg) {
		return model.ErrNotSupported
	}
	handler := c.marketHandler(sub)
	if handler == nil {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	argActive := c.argActiveLocked(arg)
	c.mu.Unlock()
	if argActive {
		c.mu.Lock()
		c.subs[sub.Key()] = sub
		c.topics[sub.Key()] = arg
		c.mu.Unlock()
		return nil
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeOrderBook, model.MarketDataTypeTradeTick, model.MarketDataTypeQuoteTick, model.MarketDataTypeBar:
		err = c.ws.Subscribe(ctx, arg, handler)
	}
	if err != nil {
		c.health.LastError = err
		return err
	}
	c.mu.Lock()
	c.subs[sub.Key()] = sub
	c.topics[sub.Key()] = arg
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
	arg := bitgetMarketArg(c.provider.category, raw, sub)
	if bitgetArgEmpty(arg) {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	if stored := c.topics[sub.Key()]; !bitgetArgEmpty(stored) {
		arg = stored
	}
	delete(c.subs, sub.Key())
	delete(c.topics, sub.Key())
	stillActive := c.argActiveLocked(arg)
	c.mu.Unlock()
	if stillActive {
		return nil
	}
	if err := c.ws.Unsubscribe(ctx, arg); err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *dataClient) handleTickerPayload(id model.InstrumentID, payload json.RawMessage) {
	var msg struct {
		Data []bitgetsdk.Ticker `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	if len(msg.Data) == 0 {
		return
	}
	ticker := msg.Data[0]
	emitTicker, emitQuote := c.tickerFanout(id)
	ts := parseUnixMillis(defaultString(ticker.Timestamp, "0"))
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{
			InstrumentID: id,
			Bid:          decimalOrFallback(ticker.Bid1Price, "0"),
			Ask:          decimalOrFallback(ticker.Ask1Price, "0"),
			Last:         decimalOrFallback(ticker.LastPrice, "0"),
			Timestamp:    ts,
		}}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     decimalOrFallback(ticker.Bid1Price, "0"),
			AskPrice:     decimalOrFallback(ticker.Ask1Price, "0"),
			BidSize:      decimalOrFallback(ticker.Bid1Size, "0"),
			AskSize:      decimalOrFallback(ticker.Ask1Size, "0"),
			Timestamp:    ts,
			InitTime:     ts,
		}})
	}
}

func (c *dataClient) handleBookPayload(id model.InstrumentID, payload json.RawMessage) {
	msg, err := bitgetsdk.DecodeOrderBookMessage(payload)
	if err != nil {
		c.health.LastError = err
		return
	}
	if len(msg.Data) == 0 {
		return
	}
	data := msg.Data[0]
	book := model.OrderBook{InstrumentID: id, Timestamp: parseUnixMillis(data.TS)}
	for _, level := range data.Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range data.Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
	}
	_ = c.emitMarket(model.MarketEvent{OrderBook: &book})
}

func (c *dataClient) handleTradePayload(id model.InstrumentID, payload json.RawMessage) {
	var msg struct {
		Data []bitgetsdk.PublicFill `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	for _, fill := range msg.Data {
		tradeID := fill.ExecID
		if tradeID == "" {
			tradeID = fmt.Sprintf("%s:%s:%s", fill.Timestamp, fill.Price, fill.Size)
		}
		ts := parseUnixMillis(defaultString(fill.Timestamp, "0"))
		if err := c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
			InstrumentID:  id,
			Price:         decimalOrFallback(fill.Price, "0"),
			Size:          decimalOrFallback(fill.Size, "0"),
			AggressorSide: bitgetAggressorSide(fill.Side),
			TradeID:       model.TradeID(tradeID),
			Timestamp:     ts,
			InitTime:      ts,
		}}); err != nil {
			return
		}
	}
}

func (c *dataClient) handleCandlePayload(barType model.BarType, payload json.RawMessage) {
	var msg struct {
		Data []bitgetsdk.Candle `json:"data"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		c.health.LastError = err
		return
	}
	for _, candle := range msg.Data {
		ts := parseUnixMillis(string(candle[0]))
		if err := c.emitMarket(model.MarketEvent{Bar: &model.Bar{
			BarType:   barType.Canonical(),
			Open:      decimalOrFallback(string(candle[1]), "0"),
			High:      decimalOrFallback(string(candle[2]), "0"),
			Low:       decimalOrFallback(string(candle[3]), "0"),
			Close:     decimalOrFallback(string(candle[4]), "0"),
			Volume:    decimalOrFallback(string(candle[5]), "0"),
			Timestamp: ts,
			InitTime:  ts,
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
		err := fmt.Errorf("%w: bitget market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func bitgetMarketArg(instType string, raw string, sub model.SubscribeMarketData) bitgetsdk.WSArg {
	arg := bitgetsdk.WSArg{InstType: instType, InstID: raw}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		arg.Channel = "ticker"
	case model.MarketDataTypeOrderBook:
		arg.Channel = fmt.Sprintf("books%d", sub.Depth)
	case model.MarketDataTypeTradeTick:
		arg.Channel = "trade"
	case model.MarketDataTypeBar:
		arg.Channel = "candle" + bitgetBarInterval(sub.BarType.Canonical().Step)
	}
	return arg
}

func (c *dataClient) marketHandler(sub model.SubscribeMarketData) func(json.RawMessage) {
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		return func(payload json.RawMessage) { c.handleTickerPayload(sub.InstrumentID, payload) }
	case model.MarketDataTypeOrderBook:
		return func(payload json.RawMessage) { c.handleBookPayload(sub.InstrumentID, payload) }
	case model.MarketDataTypeTradeTick:
		return func(payload json.RawMessage) { c.handleTradePayload(sub.InstrumentID, payload) }
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		return func(payload json.RawMessage) { c.handleCandlePayload(barType, payload) }
	default:
		return nil
	}
}

func (c *dataClient) argActiveLocked(arg bitgetsdk.WSArg) bool {
	for _, activeArg := range c.topics {
		if activeArg == arg {
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

func bitgetArgEmpty(arg bitgetsdk.WSArg) bool {
	return arg.InstType == "" && arg.Topic == "" && arg.Symbol == "" && arg.Channel == "" && arg.InstID == ""
}

type publicWS interface {
	Subscribe(context.Context, bitgetsdk.WSArg, func(json.RawMessage)) error
	Unsubscribe(context.Context, bitgetsdk.WSArg) error
	Close() error
}
