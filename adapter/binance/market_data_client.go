package binance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/sdk/binance/perp"
	"github.com/QuantProcessing/exchanges/sdk/binance/spot"
	"github.com/QuantProcessing/exchanges/venue"
)

var _ venue.DataClient = (*marketDataClient)(nil)

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
	mu          sync.Mutex
	clientID    string
	instruments venue.InstrumentProvider
	normalizer  symbolNormalizer
	spot        binanceSpotMarketDataClient
	perp        binancePerpMarketDataClient
	spotStream  binanceSpotMarketStream
	perpStream  binancePerpMarketStream
	health      venue.DataHealth

	spotBookRebuilders map[string]func(context.Context) error
	perpBookRebuilders map[string]func(context.Context) error
}

func newMarketDataClient(instruments venue.InstrumentProvider, spotClient binanceSpotMarketDataClient, perpClient binancePerpMarketDataClient) *marketDataClient {
	return newMarketDataClientWithID("binance-market-data", instruments, spotClient, perpClient)
}

func newMarketDataClientWithID(clientID string, instruments venue.InstrumentProvider, spotClient binanceSpotMarketDataClient, perpClient binancePerpMarketDataClient) *marketDataClient {
	return &marketDataClient{
		clientID:    clientID,
		instruments: instruments,
		spot:        spotClient,
		perp:        perpClient,
	}
}

func (c *marketDataClient) Venue() model.Venue { return model.VenueBinance }

func (c *marketDataClient) ClientID() string { return c.clientID }

func (c *marketDataClient) Instruments() venue.InstrumentProvider { return c.instruments }

func (c *marketDataClient) Connect(ctx context.Context) error {
	if c.instruments != nil {
		if err := c.instruments.LoadAll(ctx); err != nil {
			c.setDataError(err)
			return err
		}
	}
	if c.spotStream != nil {
		if err := connectMarketStream(ctx, c.spotStream.Connect); err != nil {
			c.setDataError(err)
			return err
		}
	}
	if c.perpStream != nil {
		if err := connectMarketStream(ctx, c.perpStream.Connect); err != nil {
			c.setDataError(err)
			return err
		}
	}
	c.mu.Lock()
	c.health.Connected = true
	c.health.InstrumentReady = c.instruments == nil || len(c.instruments.List()) > 0
	c.health.LastEventTime = time.Now()
	c.health.LastError = nil
	c.mu.Unlock()
	return nil
}

func (c *marketDataClient) Disconnect(context.Context) error {
	if c.spotStream != nil {
		c.spotStream.Close()
	}
	if c.perpStream != nil {
		c.perpStream.Close()
	}
	c.mu.Lock()
	c.health.Connected = false
	c.mu.Unlock()
	return nil
}

func (c *marketDataClient) Health() venue.DataHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.health
}

func (c *marketDataClient) setDataError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.health.LastError = err
}

func (c *marketDataClient) registerSpotBookRebuilder(key string, rebuild func(context.Context) error) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.spotBookRebuilders == nil {
		c.spotBookRebuilders = make(map[string]func(context.Context) error)
	}
	c.spotBookRebuilders[key] = rebuild
	if stream, ok := c.spotStream.(binanceMarketReconnectStream); ok {
		stream.SetPostReconnect(func() {
			c.rebuildSpotOrderBooks(context.Background())
		})
	}
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.spotBookRebuilders, key)
	}
}

func (c *marketDataClient) registerPerpBookRebuilder(key string, rebuild func(context.Context) error) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.perpBookRebuilders == nil {
		c.perpBookRebuilders = make(map[string]func(context.Context) error)
	}
	c.perpBookRebuilders[key] = rebuild
	if stream, ok := c.perpStream.(binanceMarketReconnectStream); ok {
		stream.SetPostReconnect(func() {
			c.rebuildPerpOrderBooks(context.Background())
		})
	}
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.perpBookRebuilders, key)
	}
}

func (c *marketDataClient) rebuildSpotOrderBooks(ctx context.Context) {
	rebuilders := c.spotRebuilders()
	for _, rebuild := range rebuilders {
		_ = rebuild(ctx)
	}
}

func (c *marketDataClient) rebuildPerpOrderBooks(ctx context.Context) {
	rebuilders := c.perpRebuilders()
	for _, rebuild := range rebuilders {
		_ = rebuild(ctx)
	}
}

func (c *marketDataClient) spotRebuilders() []func(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	rebuilders := make([]func(context.Context) error, 0, len(c.spotBookRebuilders))
	for _, rebuild := range c.spotBookRebuilders {
		rebuilders = append(rebuilders, rebuild)
	}
	return rebuilders
}

func (c *marketDataClient) perpRebuilders() []func(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	rebuilders := make([]func(context.Context) error, 0, len(c.perpBookRebuilders))
	for _, rebuild := range c.perpBookRebuilders {
		rebuilders = append(rebuilders, rebuild)
	}
	return rebuilders
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
