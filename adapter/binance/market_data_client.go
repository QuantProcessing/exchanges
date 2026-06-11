package binance

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

var _ venue.MarketDataClient = (*marketDataClient)(nil)

type binanceSpotMarketDataClient interface {
	BookTicker(ctx context.Context, symbol string) (*spot.BookTickerResponse, error)
	Ticker(ctx context.Context, symbol string) (*spot.TickerResponse, error)
	Depth(ctx context.Context, symbol string, limit int) (*spot.DepthResponse, error)
}

type binancePerpMarketDataClient interface {
	Ticker(ctx context.Context, symbol string) (*perp.TickerResponse, error)
	Depth(ctx context.Context, symbol string, limit int) (*perp.DepthResponse, error)
}

type marketDataClient struct {
	instruments venue.InstrumentProvider
	normalizer  symbolNormalizer
	spot        binanceSpotMarketDataClient
	perp        binancePerpMarketDataClient
}

func newMarketDataClient(instruments venue.InstrumentProvider, spotClient binanceSpotMarketDataClient, perpClient binancePerpMarketDataClient) *marketDataClient {
	return &marketDataClient{
		instruments: instruments,
		spot:        spotClient,
		perp:        perpClient,
	}
}

func (c *marketDataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
	inst, err := c.loadedInstrument(id)
	if err != nil {
		return model.Ticker{}, err
	}
	raw, err := c.normalizer.ToVenueSymbol(id)
	if err != nil {
		return model.Ticker{}, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		if c.spot == nil {
			return model.Ticker{}, fmt.Errorf("%w: binance spot market data", model.ErrNotSupported)
		}
		book, err := c.spot.BookTicker(ctx, raw)
		if err != nil {
			return model.Ticker{}, err
		}
		ticker, err := c.spot.Ticker(ctx, raw)
		if err != nil {
			return model.Ticker{}, err
		}
		return model.Ticker{
			InstrumentID: id,
			Bid:          parseDecimal(book.BidPrice),
			Ask:          parseDecimal(book.AskPrice),
			Last:         parseDecimal(ticker.LastPrice),
			EventTime:    time.Now(),
		}, nil
	case model.InstrumentTypeCryptoPerp:
		if c.perp == nil {
			return model.Ticker{}, fmt.Errorf("%w: binance perp market data", model.ErrNotSupported)
		}
		ticker, err := c.perp.Ticker(ctx, raw)
		if err != nil {
			return model.Ticker{}, err
		}
		depth, err := c.perp.Depth(ctx, raw, 5)
		if err != nil {
			return model.Ticker{}, err
		}
		bids, asks := perpBookLevels(depth)
		out := model.Ticker{
			InstrumentID: id,
			Last:         parseDecimal(ticker.LastPrice),
			EventTime:    time.Now(),
		}
		if len(bids) > 0 {
			out.Bid = bids[0].Price
		}
		if len(asks) > 0 {
			out.Ask = asks[0].Price
		}
		return out, nil
	default:
		return model.Ticker{}, fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *marketDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
	inst, err := c.loadedInstrument(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	raw, err := c.normalizer.ToVenueSymbol(id)
	if err != nil {
		return model.OrderBook{}, err
	}
	switch inst.Type {
	case model.InstrumentTypeCurrencyPair:
		if c.spot == nil {
			return model.OrderBook{}, fmt.Errorf("%w: binance spot market data", model.ErrNotSupported)
		}
		depth, err := c.spot.Depth(ctx, raw, limit)
		if err != nil {
			return model.OrderBook{}, err
		}
		return model.OrderBook{
			InstrumentID: id,
			Bids:         stringBookLevels(depth.Bids),
			Asks:         stringBookLevels(depth.Asks),
			EventTime:    time.Now(),
		}, nil
	case model.InstrumentTypeCryptoPerp:
		if c.perp == nil {
			return model.OrderBook{}, fmt.Errorf("%w: binance perp market data", model.ErrNotSupported)
		}
		depth, err := c.perp.Depth(ctx, raw, limit)
		if err != nil {
			return model.OrderBook{}, err
		}
		bids, asks := perpBookLevels(depth)
		return model.OrderBook{
			InstrumentID: id,
			Bids:         bids,
			Asks:         asks,
			EventTime:    time.Now(),
		}, nil
	default:
		return model.OrderBook{}, fmt.Errorf("%w: unsupported instrument type %s", model.ErrNotSupported, inst.Type)
	}
}

func (c *marketDataClient) FetchTrades(context.Context, model.InstrumentID, venue.TradeQuery) ([]model.Trade, error) {
	return nil, model.ErrNotSupported
}

func (c *marketDataClient) FetchBars(context.Context, model.InstrumentID, model.BarSpec, venue.BarQuery) ([]model.Bar, error) {
	return nil, model.ErrNotSupported
}

func (c *marketDataClient) SubscribeTicker(context.Context, model.InstrumentID, venue.TickerHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *marketDataClient) SubscribeOrderBook(context.Context, model.InstrumentID, int, venue.OrderBookHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *marketDataClient) SubscribeTrades(context.Context, model.InstrumentID, venue.TradeHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *marketDataClient) SubscribeBars(context.Context, model.InstrumentID, model.BarSpec, venue.BarHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *marketDataClient) loadedInstrument(id model.InstrumentID) (model.Instrument, error) {
	if c.instruments == nil {
		return model.Instrument{}, fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
	}
	inst, ok := c.instruments.Get(id)
	if !ok {
		return model.Instrument{}, fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
	}
	return inst, nil
}

func perpBookLevels(depth *perp.DepthResponse) ([]model.OrderBookLevel, []model.OrderBookLevel) {
	if depth == nil {
		return nil, nil
	}
	return stringBookLevels(depth.Bids), stringBookLevels(depth.Asks)
}

func stringBookLevels(levels [][]string) []model.OrderBookLevel {
	out := make([]model.OrderBookLevel, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		out = append(out, model.OrderBookLevel{
			Price: parseDecimal(level[0]),
			Size:  parseDecimal(level[1]),
		})
	}
	return out
}
