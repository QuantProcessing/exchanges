package okx

import (
	"context"
	"fmt"

	"github.com/QuantProcessing/exchanges/model"
	sdkokx "github.com/QuantProcessing/exchanges/sdk/okx"
	"github.com/QuantProcessing/exchanges/venue"
	"github.com/shopspring/decimal"
)

var _ venue.MarketDataClient = (*marketDataClient)(nil)

type okxMarketDataClient interface {
	GetTicker(ctx context.Context, instId string) ([]sdkokx.Ticker, error)
	GetOrderBook(ctx context.Context, instId string, sz *int) ([]sdkokx.OrderBook, error)
}

type marketDataClient struct {
	instruments venue.InstrumentProvider
	normalizer  symbolNormalizer
	client      okxMarketDataClient
}

func newMarketDataClient(instruments venue.InstrumentProvider, client okxMarketDataClient) *marketDataClient {
	return &marketDataClient{instruments: instruments, client: client}
}

func (c *marketDataClient) FetchTicker(ctx context.Context, id model.InstrumentID) (model.Ticker, error) {
	if _, err := c.loadedInstrument(id); err != nil {
		return model.Ticker{}, err
	}
	raw, err := c.normalizer.ToVenueSymbol(id)
	if err != nil {
		return model.Ticker{}, err
	}
	if c.client == nil {
		return model.Ticker{}, fmt.Errorf("%w: okx market data", model.ErrNotSupported)
	}
	tickers, err := c.client.GetTicker(ctx, raw)
	if err != nil {
		return model.Ticker{}, err
	}
	if len(tickers) == 0 {
		return model.Ticker{}, fmt.Errorf("%w: empty okx ticker for %s", model.ErrInstrumentNotLoaded, id.String())
	}
	t := tickers[0]
	return model.Ticker{
		InstrumentID: id,
		Bid:          parseString(t.BidPx),
		Ask:          parseString(t.AskPx),
		Last:         parseString(t.Last),
		EventTime:    timeFromOKXMillis(t.Ts),
	}, nil
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
	if c.client == nil {
		return model.OrderBook{}, fmt.Errorf("%w: okx market data", model.ErrNotSupported)
	}
	depth := limit
	books, err := c.client.GetOrderBook(ctx, raw, &depth)
	if err != nil {
		return model.OrderBook{}, err
	}
	if len(books) == 0 {
		return model.OrderBook{}, fmt.Errorf("%w: empty okx orderbook for %s", model.ErrInstrumentNotLoaded, id.String())
	}
	book := books[0]
	multiplier := decimal.NewFromInt(1)
	if inst.Type == model.InstrumentTypeCryptoPerp && inst.Multiplier.IsPositive() {
		multiplier = inst.Multiplier
	}
	return model.OrderBook{
		InstrumentID: id,
		Bids:         okxBookLevels(book.Bids, multiplier),
		Asks:         okxBookLevels(book.Asks, multiplier),
		EventTime:    timeFromOKXMillis(book.Ts),
	}, nil
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

func okxBookLevels(levels [][]string, multiplier decimal.Decimal) []model.OrderBookLevel {
	out := make([]model.OrderBookLevel, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		out = append(out, model.OrderBookLevel{
			Price: parseString(level[0]),
			Size:  parseString(level[1]).Mul(multiplier),
		})
	}
	return out
}
