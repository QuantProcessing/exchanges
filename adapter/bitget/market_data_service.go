package bitget

import (
	"context"

	exchanges "github.com/QuantProcessing/exchanges"
)

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, a.perpCategory, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, a.perpCategory, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(symbol, raw), nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetRecentFills(ctx, a.perpCategory, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(symbol, raw), nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	startTime, endTime, limit, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetCandles(ctx, a.perpCategory, a.FormatSymbol(symbol), rawInterval, "market", startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	return mapKlines(symbol, interval, raw), nil
}

// FetchHistoricalTrades returns recent public fills. Bitget's /api/v3/market/fills
// endpoint does not accept cursor, fromId, or time-range parameters — so opts.FromID,
// opts.Start, and opts.End are ignored. Only opts.Limit is honored.
func (a *Adapter) FetchHistoricalTrades(ctx context.Context, symbol string, opts *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	sym := a.FormatSymbol(symbol)
	limit := 100
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	raw, err := a.client.GetRecentFills(ctx, a.perpCategory, sym, limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(symbol, raw), nil
}

func (a *SpotAdapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, categorySpot, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return toTicker(symbol, raw), nil
}

func (a *SpotAdapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, categorySpot, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return toOrderBook(symbol, raw), nil
}

func (a *SpotAdapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetRecentFills(ctx, categorySpot, a.FormatSymbol(symbol), limit)
	if err != nil {
		return nil, err
	}
	return mapTrades(symbol, raw), nil
}

func (a *SpotAdapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	rawInterval, err := klineIntervalString(interval)
	if err != nil {
		return nil, err
	}
	startTime, endTime, limit, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetCandles(ctx, categorySpot, a.FormatSymbol(symbol), rawInterval, "market", startTime, endTime, limit)
	if err != nil {
		return nil, err
	}
	return mapKlines(symbol, interval, raw), nil
}
