package okx

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	okxsdk "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
)

type dataClient struct {
	id       string
	provider *productProvider
	sdk      sdkClient
	ws       publicWS
	events   chan model.MarketEvent
	subs     map[string]model.SubscribeMarketData
	topics   map[string]okxsdk.WsSubscribeArgs
	mu       sync.Mutex
	health   venue.DataHealth
}

func newDataClient(id string, provider *productProvider, sdk sdkClient) *dataClient {
	return &dataClient{
		id:       id,
		provider: provider,
		sdk:      sdk,
		ws:       newPublicWS(),
		events:   make(chan model.MarketEvent, 256),
		subs:     make(map[string]model.SubscribeMarketData),
		topics:   make(map[string]okxsdk.WsSubscribeArgs),
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
	tickers, err := c.sdk.GetTicker(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	if len(tickers) == 0 {
		return model.Ticker{}, fmt.Errorf("%w: empty OKX ticker for %s", model.ErrInstrumentNotFound, id.String())
	}
	ticker := tickers[0]
	return model.Ticker{
		InstrumentID: id,
		Bid:          decimalOrFallback(ticker.BidPx, "0"),
		Ask:          decimalOrFallback(ticker.AskPx, "0"),
		Last:         decimalOrFallback(ticker.Last, "0"),
		Timestamp:    parseUnixMillis(ticker.Ts),
	}, nil
}

func (c *dataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	raw, err := c.provider.rawSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	books, err := c.sdk.GetOrderBook(ctx, raw, &limit)
	if err != nil {
		return model.OrderBook{}, err
	}
	if len(books) == 0 {
		return model.OrderBook{}, fmt.Errorf("%w: empty OKX order book for %s", model.ErrInstrumentNotFound, id.String())
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseUnixMillis(books[0].Ts)}
	for _, level := range books[0].Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range books[0].Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
	}
	if err := book.Validate(); err != nil {
		return model.OrderBook{}, err
	}
	return book, nil
}

func (c *dataClient) SubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	if c.ws == nil {
		return model.ErrNotSupported
	}
	arg := okxMarketArg(raw, sub)
	if okxArgEmpty(arg) {
		return model.ErrNotSupported
	}
	if err := c.ws.Connect(); err != nil {
		c.health.LastError = err
		return err
	}
	c.mu.Lock()
	argActive := c.argActiveLocked(arg)
	c.mu.Unlock()
	if argActive {
		c.mu.Lock()
		c.subs[sub.Key()] = sub
		c.topics[sub.Key()] = arg
		c.mu.Unlock()
		c.health.Connected = true
		return nil
	}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		err = c.ws.SubscribeTicker(raw, func(ticker *okxsdk.Ticker) {
			c.handleTicker(sub.InstrumentID, ticker)
		})
	case model.MarketDataTypeOrderBook:
		err = c.ws.SubscribeOrderBook(raw, func(book *okxsdk.OrderBook, _ string) {
			c.handleOrderBook(sub.InstrumentID, book)
		})
	case model.MarketDataTypeTradeTick:
		err = c.ws.SubscribeTrades(raw, func(trade *okxsdk.PublicTrade) {
			c.handleTrade(sub.InstrumentID, trade)
		})
	case model.MarketDataTypeBar:
		barType := sub.BarType.Canonical()
		err = c.ws.SubscribeCandles(raw, okxBarChannel(barType.Step), func(candle okxsdk.Candle) {
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
	c.topics[sub.Key()] = arg
	c.mu.Unlock()
	c.health.Connected = true
	return nil
}

func (c *dataClient) UnsubscribeMarketData(_ context.Context, sub model.SubscribeMarketData) error {
	if err := sub.Validate(); err != nil {
		return err
	}
	raw, err := c.provider.rawSymbol(sub.InstrumentID)
	if err != nil {
		return err
	}
	if c.ws == nil {
		return model.ErrNotSupported
	}
	arg := okxMarketArg(raw, sub)
	if okxArgEmpty(arg) {
		return model.ErrNotSupported
	}
	c.mu.Lock()
	if stored := c.topics[sub.Key()]; !okxArgEmpty(stored) {
		arg = stored
	}
	delete(c.subs, sub.Key())
	delete(c.topics, sub.Key())
	stillActive := c.argActiveLocked(arg)
	c.mu.Unlock()
	if stillActive {
		return nil
	}
	if err := c.ws.Unsubscribe(arg); err != nil {
		c.health.LastError = err
		return err
	}
	return nil
}

func (c *dataClient) handleTicker(id model.InstrumentID, ticker *okxsdk.Ticker) {
	if ticker == nil {
		return
	}
	emitTicker, emitQuote := c.tickerFanout(id)
	ts := parseUnixMillis(ticker.Ts)
	if emitTicker {
		if err := c.emitMarket(model.MarketEvent{Ticker: &model.Ticker{
			InstrumentID: id,
			Bid:          decimalOrFallback(ticker.BidPx, "0"),
			Ask:          decimalOrFallback(ticker.AskPx, "0"),
			Last:         decimalOrFallback(ticker.Last, "0"),
			Timestamp:    ts,
		}}); err != nil {
			return
		}
	}
	if emitQuote {
		_ = c.emitMarket(model.MarketEvent{Quote: &model.QuoteTick{
			InstrumentID: id,
			BidPrice:     decimalOrFallback(ticker.BidPx, "0"),
			AskPrice:     decimalOrFallback(ticker.AskPx, "0"),
			BidSize:      decimalOrFallback(ticker.BidSz, "0"),
			AskSize:      decimalOrFallback(ticker.AskSz, "0"),
			Timestamp:    ts,
			InitTime:     ts,
		}})
	}
}

func (c *dataClient) handleOrderBook(id model.InstrumentID, venueBook *okxsdk.OrderBook) {
	if venueBook == nil {
		return
	}
	book := model.OrderBook{InstrumentID: id, Timestamp: parseUnixMillis(venueBook.Ts)}
	for _, level := range venueBook.Bids {
		book.Bids = append(book.Bids, parseBookLevel(level))
	}
	for _, level := range venueBook.Asks {
		book.Asks = append(book.Asks, parseBookLevel(level))
	}
	_ = c.emitMarket(model.MarketEvent{OrderBook: &book})
}

func (c *dataClient) handleTrade(id model.InstrumentID, trade *okxsdk.PublicTrade) {
	if trade == nil {
		return
	}
	ts := parseUnixMillis(trade.Ts)
	_ = c.emitMarket(model.MarketEvent{Trade: &model.TradeTick{
		InstrumentID:  id,
		Price:         decimalOrFallback(trade.Px, "0"),
		Size:          decimalOrFallback(trade.Sz, "0"),
		AggressorSide: okxAggressorSide(trade.Side),
		TradeID:       model.TradeID(defaultString(trade.TradeId, fmt.Sprintf("%s:%s:%s", trade.Ts, trade.Px, trade.Sz))),
		Timestamp:     ts,
		InitTime:      ts,
	}})
}

func (c *dataClient) handleCandle(barType model.BarType, candle okxsdk.Candle) {
	ts := parseUnixMillis(candle[0])
	_ = c.emitMarket(model.MarketEvent{Bar: &model.Bar{
		BarType:    barType.Canonical(),
		Open:       decimalOrFallback(candle[1], "0"),
		High:       decimalOrFallback(candle[2], "0"),
		Low:        decimalOrFallback(candle[3], "0"),
		Close:      decimalOrFallback(candle[4], "0"),
		Volume:     decimalOrFallback(candle[5], "0"),
		Timestamp:  ts,
		InitTime:   ts,
		IsRevision: candle[8] != "1",
	}})
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
		err := fmt.Errorf("%w: okx market event channel full", model.ErrInvalidMarketData)
		c.health.LastError = err
		return err
	}
}

func okxMarketArg(raw string, sub model.SubscribeMarketData) okxsdk.WsSubscribeArgs {
	arg := okxsdk.WsSubscribeArgs{InstId: raw}
	switch sub.Type {
	case model.MarketDataTypeTicker, model.MarketDataTypeQuoteTick:
		arg.Channel = "tickers"
	case model.MarketDataTypeOrderBook:
		arg.Channel = "books"
	case model.MarketDataTypeTradeTick:
		arg.Channel = "trades"
	case model.MarketDataTypeBar:
		arg.Channel = okxBarChannel(sub.BarType.Canonical().Step)
	}
	return arg
}

func (c *dataClient) argActiveLocked(arg okxsdk.WsSubscribeArgs) bool {
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

func okxArgEmpty(arg okxsdk.WsSubscribeArgs) bool {
	return arg.Channel == "" && arg.InstType == "" && arg.InstId == ""
}

type publicWS interface {
	Connect() error
	SubscribeTicker(string, func(*okxsdk.Ticker)) error
	SubscribeOrderBook(string, func(*okxsdk.OrderBook, string)) error
	SubscribeTrades(string, func(*okxsdk.PublicTrade)) error
	SubscribeCandles(string, string, func(okxsdk.Candle)) error
	Unsubscribe(okxsdk.WsSubscribeArgs) error
	Close() error
}

type okxPublicWS struct {
	client *okxsdk.WSClient
}

func newPublicWS() *okxPublicWS {
	return &okxPublicWS{client: okxsdk.NewWSClient(context.Background())}
}

func (w *okxPublicWS) Connect() error {
	return w.client.Connect()
}

func (w *okxPublicWS) SubscribeTicker(instID string, handler func(*okxsdk.Ticker)) error {
	return w.client.SubscribeTicker(instID, handler)
}

func (w *okxPublicWS) SubscribeOrderBook(instID string, handler func(*okxsdk.OrderBook, string)) error {
	return w.client.SubscribeOrderBook(instID, handler)
}

func (w *okxPublicWS) SubscribeTrades(instID string, handler func(*okxsdk.PublicTrade)) error {
	return w.client.SubscribeTrades(instID, handler)
}

func (w *okxPublicWS) SubscribeCandles(instID string, channel string, handler func(okxsdk.Candle)) error {
	return w.client.SubscribeCandles(instID, channel, handler)
}

func (w *okxPublicWS) Unsubscribe(arg okxsdk.WsSubscribeArgs) error {
	return w.client.Unsubscribe(arg)
}

func (w *okxPublicWS) Close() error {
	if w.client.Conn == nil {
		return nil
	}
	return w.client.Conn.Close()
}
