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

var _ venue.MarketDataClient = (*v2MarketDataClient)(nil)

type binanceSpotMarketDataClient interface {
	BookTicker(ctx context.Context, symbol string) (*spot.BookTickerResponse, error)
	Ticker(ctx context.Context, symbol string) (*spot.TickerResponse, error)
	Depth(ctx context.Context, symbol string, limit int) (*spot.DepthResponse, error)
}

type binancePerpMarketDataClient interface {
	Ticker(ctx context.Context, symbol string) (*perp.TickerResponse, error)
	Depth(ctx context.Context, symbol string, limit int) (*perp.DepthResponse, error)
}

type v2MarketDataClient struct {
	instruments venue.InstrumentProvider
	normalizer  v2SymbolNormalizer
	spot        binanceSpotMarketDataClient
	perp        binancePerpMarketDataClient
}

func newV2MarketDataClient(instruments venue.InstrumentProvider, spotClient binanceSpotMarketDataClient, perpClient binancePerpMarketDataClient) *v2MarketDataClient {
	return &v2MarketDataClient{
		instruments: instruments,
		spot:        spotClient,
		perp:        perpClient,
	}
}

func (c *v2MarketDataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
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
		bids, asks := v2PerpBookLevels(depth)
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

func (c *v2MarketDataClient) FetchOrderBook(ctx context.Context, id model.InstrumentID, limit int) (model.OrderBook, error) {
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
			Bids:         v2StringBookLevels(depth.Bids),
			Asks:         v2StringBookLevels(depth.Asks),
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
		bids, asks := v2PerpBookLevels(depth)
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

func (c *v2MarketDataClient) FetchTrades(context.Context, model.InstrumentID, venue.TradeQuery) ([]model.Trade, error) {
	return nil, model.ErrNotSupported
}

func (c *v2MarketDataClient) FetchBars(context.Context, model.InstrumentID, model.BarSpec, venue.BarQuery) ([]model.Bar, error) {
	return nil, model.ErrNotSupported
}

func (c *v2MarketDataClient) SubscribeTicker(context.Context, model.InstrumentID, venue.TickerHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *v2MarketDataClient) SubscribeOrderBook(context.Context, model.InstrumentID, int, venue.OrderBookHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *v2MarketDataClient) SubscribeTrades(context.Context, model.InstrumentID, venue.TradeHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *v2MarketDataClient) SubscribeBars(context.Context, model.InstrumentID, model.BarSpec, venue.BarHandler) (venue.Subscription, error) {
	return nil, model.ErrNotSupported
}

func (c *v2MarketDataClient) loadedInstrument(id model.InstrumentID) (model.Instrument, error) {
	if c.instruments == nil {
		return model.Instrument{}, fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
	}
	inst, ok := c.instruments.Get(id)
	if !ok {
		return model.Instrument{}, fmt.Errorf("%w: %s", model.ErrInstrumentNotLoaded, id.String())
	}
	return inst, nil
}

func v2PerpBookLevels(depth *perp.DepthResponse) ([]model.OrderBookLevel, []model.OrderBookLevel) {
	if depth == nil {
		return nil, nil
	}
	return v2StringBookLevels(depth.Bids), v2StringBookLevels(depth.Asks)
}

func v2StringBookLevels(levels [][]string) []model.OrderBookLevel {
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
