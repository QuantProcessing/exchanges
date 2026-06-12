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
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

type binanceSpotMarketStream interface {
	Connect() error
	Close()
	SubscribeBookTicker(string, func(*spot.BookTickerEvent) error) error
	UnsubscribeBookTicker(string) error
	SubscribeIncrementOrderBook(string, string, func(*spot.WsDepthEvent) error) error
	UnsubscribeIncrementOrderBook(string, string) error
	SubscribeLimitOrderBook(string, int, string, func(*spot.DepthEvent) error) error
	UnsubscribeLimitOrderBook(string, int, string) error
	SubscribeTrade(string, func(*spot.TradeEvent) error) error
	UnsubscribeTrade(string) error
	SubscribeKline(string, string, func(*spot.KlineEvent) error) error
	UnsubscribeKline(string, string) error
}

type binancePerpMarketStream interface {
	Connect() error
	Close()
	SubscribeBookTicker(string, func(*perp.WsBookTickerEvent) error) error
	UnsubscribeBookTicker(string) error
	SubscribeIncrementOrderBook(string, string, func(*perp.WsDepthEvent) error) error
	UnsubscribeIncrementOrderBook(string, string) error
	SubscribeLimitOrderBook(string, int, string, func(*perp.WsDepthEvent) error) error
	UnsubscribeLimitOrderBook(string, int, string) error
	SubscribeAggTrade(string, func(*perp.WsAggTradeEvent) error) error
	UnsubscribeAggTrade(string) error
	SubscribeKline(string, string, func(*perp.WsKlineEvent) error) error
	UnsubscribeKline(string, string) error
}

type binanceMarketReconnectStream interface {
	SetPostReconnect(func())
}

type marketSubscription struct {
	id      string
	closeFn func() error
	done    chan struct{}
	once    sync.Once
	err     error
}

func newMarketSubscription(id string, closeFn func() error) venue.Subscription {
	return &marketSubscription{
		id:      id,
		closeFn: closeFn,
		done:    make(chan struct{}),
	}
}

func (s *marketSubscription) ID() string { return s.id }

func (s *marketSubscription) Close() error {
	s.once.Do(func() {
		if s.closeFn != nil {
			s.err = s.closeFn()
		}
		close(s.done)
	})
	return s.err
}

func (s *marketSubscription) Done() <-chan struct{} { return s.done }

func (s *marketSubscription) Err() error { return s.err }

func (c *marketDataClient) SubscribeTicker(ctx context.Context, id model.InstrumentID, h venue.TickerHandler) (venue.Subscription, error) {
	inst, raw, err := c.marketInstrumentAndRaw(id)
	if err != nil {
		return nil, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		streamSymbol := strings.ToLower(raw)
		if c.spotStream == nil {
			return nil, fmt.Errorf("%w: binance spot ticker stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil ticker handler")
		}
		if err := connectMarketStream(ctx, c.spotStream.Connect); err != nil {
			return nil, err
		}
		if err := c.spotStream.SubscribeBookTicker(streamSymbol, func(e *spot.BookTickerEvent) error {
			if e == nil {
				return nil
			}
			h(model.Ticker{
				InstrumentID: id,
				Bid:          parseDecimal(e.BestBidPrice),
				Ask:          parseDecimal(e.BestAskPrice),
				EventTime:    time.Now(),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription("binance:"+streamSymbol+":bookTicker", func() error {
			return c.spotStream.UnsubscribeBookTicker(streamSymbol)
		}), nil
	case model.InstrumentTypeCryptoPerp:
		streamSymbol := strings.ToLower(raw)
		if c.perpStream == nil {
			return nil, fmt.Errorf("%w: binance perp ticker stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil ticker handler")
		}
		if err := connectMarketStream(ctx, c.perpStream.Connect); err != nil {
			return nil, err
		}
		if err := c.perpStream.SubscribeBookTicker(streamSymbol, func(e *perp.WsBookTickerEvent) error {
			if e == nil {
				return nil
			}
			h(model.Ticker{
				InstrumentID: id,
				Bid:          parseDecimal(e.BestBidPrice),
				Ask:          parseDecimal(e.BestAskPrice),
				EventTime:    eventTime(e.EventTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription("binance:"+streamSymbol+":bookTicker", func() error {
			return c.perpStream.UnsubscribeBookTicker(streamSymbol)
		}), nil
	default:
		return nil, fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *marketDataClient) SubscribeOrderBook(ctx context.Context, id model.InstrumentID, depth int, h venue.OrderBookHandler) (venue.Subscription, error) {
	mode, levels, err := classifyOrderBookDepth(depth)
	if err != nil {
		return nil, err
	}
	inst, raw, err := c.marketInstrumentAndRaw(id)
	if err != nil {
		return nil, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		const interval = "100ms"
		streamSymbol := strings.ToLower(raw)
		if c.spotStream == nil {
			return nil, fmt.Errorf("%w: binance spot order book stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil order book handler")
		}
		if err := connectMarketStream(ctx, c.spotStream.Connect); err != nil {
			return nil, err
		}
		if mode == orderBookStreamDiff {
			return c.subscribeSpotDiffOrderBook(ctx, id, raw, streamSymbol, levels, interval, h)
		}
		if err := c.spotStream.SubscribeLimitOrderBook(streamSymbol, levels, interval, func(e *spot.DepthEvent) error {
			if e == nil {
				return nil
			}
			h(model.OrderBook{
				InstrumentID: id,
				Bids:         stringBookLevels(e.Bids),
				Asks:         stringBookLevels(e.Asks),
				EventTime:    eventTime(e.EventTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription(fmt.Sprintf("binance:%s:depth%d", streamSymbol, levels), func() error {
			return c.spotStream.UnsubscribeLimitOrderBook(streamSymbol, levels, interval)
		}), nil
	case model.InstrumentTypeCryptoPerp:
		const interval = "250ms"
		streamSymbol := strings.ToLower(raw)
		if c.perpStream == nil {
			return nil, fmt.Errorf("%w: binance perp order book stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil order book handler")
		}
		if err := connectMarketStream(ctx, c.perpStream.Connect); err != nil {
			return nil, err
		}
		if mode == orderBookStreamDiff {
			return c.subscribePerpDiffOrderBook(ctx, id, raw, streamSymbol, levels, interval, h)
		}
		if err := c.perpStream.SubscribeLimitOrderBook(streamSymbol, levels, interval, func(e *perp.WsDepthEvent) error {
			if e == nil {
				return nil
			}
			h(model.OrderBook{
				InstrumentID: id,
				Bids:         interfaceBookLevels(e.Bids),
				Asks:         interfaceBookLevels(e.Asks),
				EventTime:    eventTime(e.TransactionTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription(fmt.Sprintf("binance:%s:depth%d", streamSymbol, levels), func() error {
			return c.perpStream.UnsubscribeLimitOrderBook(streamSymbol, levels, interval)
		}), nil
	default:
		return nil, fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *marketDataClient) SubscribeTrades(ctx context.Context, id model.InstrumentID, h venue.TradeHandler) (venue.Subscription, error) {
	inst, raw, err := c.marketInstrumentAndRaw(id)
	if err != nil {
		return nil, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		streamSymbol := strings.ToLower(raw)
		if c.spotStream == nil {
			return nil, fmt.Errorf("%w: binance spot trade stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil trade handler")
		}
		if err := connectMarketStream(ctx, c.spotStream.Connect); err != nil {
			return nil, err
		}
		if err := c.spotStream.SubscribeTrade(streamSymbol, func(e *spot.TradeEvent) error {
			if e == nil {
				return nil
			}
			h(model.Trade{
				InstrumentID: id,
				TradeID:      model.TradeID(strconv.FormatInt(e.TradeID, 10)),
				Price:        parseDecimal(e.Price),
				Size:         parseDecimal(e.Quantity),
				Side:         tradeSide(e.IsBuyerMaker),
				EventTime:    eventTime(e.TradeTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription("binance:"+streamSymbol+":trade", func() error {
			return c.spotStream.UnsubscribeTrade(streamSymbol)
		}), nil
	case model.InstrumentTypeCryptoPerp:
		streamSymbol := strings.ToLower(raw)
		if c.perpStream == nil {
			return nil, fmt.Errorf("%w: binance perp agg trade stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil trade handler")
		}
		if err := connectMarketStream(ctx, c.perpStream.Connect); err != nil {
			return nil, err
		}
		if err := c.perpStream.SubscribeAggTrade(streamSymbol, func(e *perp.WsAggTradeEvent) error {
			if e == nil {
				return nil
			}
			h(model.Trade{
				InstrumentID: id,
				TradeID:      model.TradeID(strconv.FormatInt(e.AggTradeID, 10)),
				Price:        parseDecimal(e.Price),
				Size:         parseDecimal(e.Quantity),
				Side:         tradeSide(e.IsBuyerMaker),
				EventTime:    eventTime(e.TradeTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription("binance:"+streamSymbol+":aggTrade", func() error {
			return c.perpStream.UnsubscribeAggTrade(streamSymbol)
		}), nil
	default:
		return nil, fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *marketDataClient) SubscribeBars(ctx context.Context, id model.InstrumentID, spec model.BarSpec, h venue.BarHandler) (venue.Subscription, error) {
	if spec.Interval == "" {
		return nil, fmt.Errorf("%w: binance bar stream requires interval", model.ErrNotSupported)
	}
	inst, raw, err := c.marketInstrumentAndRaw(id)
	if err != nil {
		return nil, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		streamSymbol := strings.ToLower(raw)
		if c.spotStream == nil {
			return nil, fmt.Errorf("%w: binance spot bar stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil bar handler")
		}
		if err := connectMarketStream(ctx, c.spotStream.Connect); err != nil {
			return nil, err
		}
		if err := c.spotStream.SubscribeKline(streamSymbol, spec.Interval, func(e *spot.KlineEvent) error {
			if e == nil || !e.Kline.IsClosed {
				return nil
			}
			h(model.Bar{
				InstrumentID: id,
				Spec:         spec,
				Open:         parseDecimal(e.Kline.OpenPrice),
				High:         parseDecimal(e.Kline.HighPrice),
				Low:          parseDecimal(e.Kline.LowPrice),
				Close:        parseDecimal(e.Kline.ClosePrice),
				Volume:       parseDecimal(e.Kline.Volume),
				EventTime:    eventTime(e.Kline.CloseTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription("binance:"+streamSymbol+":kline_"+spec.Interval, func() error {
			return c.spotStream.UnsubscribeKline(streamSymbol, spec.Interval)
		}), nil
	case model.InstrumentTypeCryptoPerp:
		streamSymbol := strings.ToLower(raw)
		if c.perpStream == nil {
			return nil, fmt.Errorf("%w: binance perp bar stream", model.ErrNotSupported)
		}
		if h == nil {
			return nil, fmt.Errorf("binance: nil bar handler")
		}
		if err := connectMarketStream(ctx, c.perpStream.Connect); err != nil {
			return nil, err
		}
		if err := c.perpStream.SubscribeKline(streamSymbol, spec.Interval, func(e *perp.WsKlineEvent) error {
			if e == nil || !e.Kline.IsClosed {
				return nil
			}
			h(model.Bar{
				InstrumentID: id,
				Spec:         spec,
				Open:         parseDecimal(e.Kline.OpenPrice),
				High:         parseDecimal(e.Kline.HighPrice),
				Low:          parseDecimal(e.Kline.LowPrice),
				Close:        parseDecimal(e.Kline.ClosePrice),
				Volume:       parseDecimal(e.Kline.Volume),
				EventTime:    eventTime(e.Kline.EndTime),
			})
			return nil
		}); err != nil {
			return nil, err
		}
		return newMarketSubscription("binance:"+streamSymbol+":kline_"+spec.Interval, func() error {
			return c.perpStream.UnsubscribeKline(streamSymbol, spec.Interval)
		}), nil
	default:
		return nil, fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *marketDataClient) subscribeSpotDiffOrderBook(ctx context.Context, id model.InstrumentID, raw string, streamSymbol string, limit int, interval string, h venue.OrderBookHandler) (venue.Subscription, error) {
	if c.spot == nil {
		return nil, fmt.Errorf("%w: binance spot order book snapshot", model.ErrNotSupported)
	}
	runtime := newOrderBookRuntime(id, limit, func(ctx context.Context) (orderBookSnapshot, error) {
		depth, err := c.spot.Depth(ctx, raw, limit)
		if err != nil {
			return orderBookSnapshot{}, err
		}
		return orderBookSnapshot{
			sequence:  depth.LastUpdateID,
			bids:      stringBookLevels(depth.Bids),
			asks:      stringBookLevels(depth.Asks),
			eventTime: time.Now(),
		}, nil
	}, h)
	runtime.BeginBuffer()
	if err := c.spotStream.SubscribeIncrementOrderBook(streamSymbol, interval, func(e *spot.WsDepthEvent) error {
		if e == nil {
			return nil
		}
		return runtime.HandleDelta(context.Background(), bookDelta{
			first:     e.FirstUpdateID,
			final:     e.FinalUpdateID,
			previous:  e.FinalUpdateIDLast,
			bids:      stringBookLevels(e.Bids),
			asks:      stringBookLevels(e.Asks),
			eventTime: eventTime(e.EventTime),
		})
	}); err != nil {
		return nil, err
	}
	if err := runtime.Rebuild(ctx); err != nil {
		_ = c.spotStream.UnsubscribeIncrementOrderBook(streamSymbol, interval)
		return nil, err
	}
	unregister := c.registerSpotBookRebuilder(fmt.Sprintf("%s:%d:%p", streamSymbol, limit, runtime), runtime.Rebuild)
	return newMarketSubscription(fmt.Sprintf("binance:%s:depth", streamSymbol), func() error {
		unregister()
		return c.spotStream.UnsubscribeIncrementOrderBook(streamSymbol, interval)
	}), nil
}

func (c *marketDataClient) subscribePerpDiffOrderBook(ctx context.Context, id model.InstrumentID, raw string, streamSymbol string, limit int, interval string, h venue.OrderBookHandler) (venue.Subscription, error) {
	if c.perp == nil {
		return nil, fmt.Errorf("%w: binance perp order book snapshot", model.ErrNotSupported)
	}
	runtime := newOrderBookRuntime(id, limit, func(ctx context.Context) (orderBookSnapshot, error) {
		depth, err := c.perp.Depth(ctx, raw, limit)
		if err != nil {
			return orderBookSnapshot{}, err
		}
		eventTime := time.Now()
		if depth.T > 0 {
			eventTime = eventTimeFromMillis(depth.T)
		} else if depth.E > 0 {
			eventTime = eventTimeFromMillis(depth.E)
		}
		bids, asks := perpBookLevels(depth)
		return orderBookSnapshot{
			sequence:  depth.LastUpdateID,
			bids:      bids,
			asks:      asks,
			eventTime: eventTime,
		}, nil
	}, h)
	runtime.BeginBuffer()
	if err := c.perpStream.SubscribeIncrementOrderBook(streamSymbol, interval, func(e *perp.WsDepthEvent) error {
		if e == nil {
			return nil
		}
		return runtime.HandleDelta(context.Background(), bookDelta{
			first:     e.FirstUpdateID,
			final:     e.FinalUpdateID,
			previous:  e.FinalUpdateIDLast,
			bids:      interfaceBookLevels(e.Bids),
			asks:      interfaceBookLevels(e.Asks),
			eventTime: eventTime(e.TransactionTime),
		})
	}); err != nil {
		return nil, err
	}
	if err := runtime.Rebuild(ctx); err != nil {
		_ = c.perpStream.UnsubscribeIncrementOrderBook(streamSymbol, interval)
		return nil, err
	}
	unregister := c.registerPerpBookRebuilder(fmt.Sprintf("%s:%d:%p", streamSymbol, limit, runtime), runtime.Rebuild)
	return newMarketSubscription(fmt.Sprintf("binance:%s:depth", streamSymbol), func() error {
		unregister()
		return c.perpStream.UnsubscribeIncrementOrderBook(streamSymbol, interval)
	}), nil
}

func (c *marketDataClient) Close() error {
	if c.spotStream != nil {
		c.spotStream.Close()
	}
	if c.perpStream != nil {
		c.perpStream.Close()
	}
	return nil
}

func (c *marketDataClient) marketInstrumentAndRaw(id model.InstrumentID) (model.Instrument, string, error) {
	inst, err := c.loadedInstrument(id)
	if err != nil {
		return model.Instrument{}, "", err
	}
	raw, err := c.normalizer.ToVenueSymbol(id)
	if err != nil {
		return model.Instrument{}, "", err
	}
	return inst, raw, nil
}

func connectMarketStream(ctx context.Context, connect func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return connect()
}

type orderBookStreamMode int

const (
	orderBookStreamPartial orderBookStreamMode = iota
	orderBookStreamDiff
)

func classifyOrderBookDepth(depth int) (orderBookStreamMode, int, error) {
	switch depth {
	case 0:
		return orderBookStreamDiff, 1000, nil
	case 5, 10, 20:
		return orderBookStreamPartial, depth, nil
	case 50, 100, 500, 1000:
		return orderBookStreamDiff, depth, nil
	default:
		return 0, 0, fmt.Errorf("%w: binance order book depth must be 0, 5, 10, 20, 50, 100, 500, or 1000", model.ErrNotSupported)
	}
}

func eventTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return eventTimeFromMillis(ms)
}

func eventTimeFromMillis(ms int64) time.Time {
	return timeFromUnixMilli(ms)
}

func interfaceBookLevels(levels [][]interface{}) []model.OrderBookLevel {
	out := make([]model.OrderBookLevel, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		out = append(out, model.OrderBookLevel{
			Price: parseDecimalInterface(level[0]),
			Size:  parseDecimalInterface(level[1]),
		})
	}
	return out
}

func tradeSide(isBuyerMaker bool) model.OrderSide {
	if isBuyerMaker {
		return model.OrderSideSell
	}
	return model.OrderSideBuy
}
