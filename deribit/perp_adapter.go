package deribit

import (
	"context"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/deribit/sdk"
	"github.com/shopspring/decimal"
)

type Adapter struct {
	*exchanges.BaseAdapter
	client  marketClient
	markets *perpMarketCache
}

type perpMarketCache struct {
	byBase   map[string]sdk.Instrument
	bySymbol map[string]sdk.Instrument
}

func NewAdapter(ctx context.Context, opts Options) (*Adapter, error) {
	return newPerpAdapterWithClient(ctx, opts, sdk.NewClient())
}

// newPerpAdapterWithClient eagerly loads public instrument metadata so
// ListSymbols and FetchSymbolDetails are available immediately after construction.
func newPerpAdapterWithClient(ctx context.Context, opts Options, client marketClient) (*Adapter, error) {
	instruments, err := client.GetInstruments(ctx, "any", kindFuture, false)
	if err != nil {
		return nil, err
	}
	markets, details := buildPerpMarketCache(instruments)
	base := exchanges.NewBaseAdapter(exchangeName, exchanges.MarketTypePerp, opts.logger())
	base.SetSymbolDetails(details)
	return &Adapter{
		BaseAdapter: base,
		client:      client,
		markets:     markets,
	}, nil
}

func (a *Adapter) Close() error { return nil }

func (a *Adapter) FormatSymbol(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	if upper == "" {
		return upper
	}
	if strings.Contains(upper, "-") {
		return upper
	}
	if a.markets != nil {
		if inst, ok := a.markets.byBase[upper]; ok {
			return strings.ToUpper(inst.InstrumentName)
		}
	}
	return upper + "-PERPETUAL"
}

func (a *Adapter) ExtractSymbol(symbol string) string {
	upper := strings.ToUpper(strings.TrimSpace(symbol))
	if a.markets != nil {
		if inst, ok := a.markets.bySymbol[upper]; ok && inst.BaseCurrency != "" {
			return strings.ToUpper(inst.BaseCurrency)
		}
	}
	return strings.TrimSuffix(upper, "-PERPETUAL")
}

func (a *Adapter) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	raw, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	canonical := a.ExtractSymbol(firstNonEmpty(raw.InstrumentName, symbol))
	return toTicker(canonical, raw), nil
}

func (a *Adapter) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	raw, err := a.client.GetOrderBook(ctx, a.FormatSymbol(symbol), deribitDepth(limit))
	if err != nil {
		return nil, err
	}
	return toOrderBook(a.ExtractSymbol(firstNonEmpty(raw.InstrumentName, symbol)), raw), nil
}

func (a *Adapter) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	raw, err := a.client.GetLastTradesByInstrument(ctx, a.FormatSymbol(symbol), deribitCount(limit))
	if err != nil {
		return nil, err
	}
	return mapTrades(a.ExtractSymbol(symbol), raw.Trades), nil
}

func (a *Adapter) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	resolution, err := deribitResolution(interval)
	if err != nil {
		return nil, err
	}
	start, end, err := klineTimeRange(interval, opts)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.GetTradingViewChartData(ctx, a.FormatSymbol(symbol), start, end, resolution)
	if err != nil {
		return nil, err
	}
	return mapKlines(a.ExtractSymbol(symbol), interval, raw), nil
}

func (a *Adapter) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	_ = ctx
	return a.GetSymbolDetail(a.ExtractSymbol(symbol))
}

func (a *Adapter) FetchFundingRate(ctx context.Context, symbol string) (*exchanges.FundingRate, error) {
	raw, err := a.client.GetTicker(ctx, a.FormatSymbol(symbol))
	if err != nil {
		return nil, err
	}
	return &exchanges.FundingRate{
		Symbol:               a.ExtractSymbol(firstNonEmpty(raw.InstrumentName, symbol)),
		FundingRate:          decimalFromFloat(raw.Funding8h),
		FundingIntervalHours: 8,
		UpdateTime:           raw.Timestamp,
	}, nil
}

func (a *Adapter) FetchAllFundingRates(ctx context.Context) ([]exchanges.FundingRate, error) {
	symbols := a.ListSymbols()
	out := make([]exchanges.FundingRate, 0, len(symbols))
	for _, symbol := range symbols {
		rate, err := a.FetchFundingRate(ctx, symbol)
		if err != nil {
			return nil, err
		}
		out = append(out, *rate)
	}
	return out, nil
}

func (a *Adapter) FetchFundingRateHistory(context.Context, string, *exchanges.FundingRateHistoryOpts) ([]exchanges.FundingRate, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenInterest(context.Context, string) (*exchanges.OpenInterest, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchPositions(context.Context) ([]exchanges.Position, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) SetLeverage(context.Context, string, int) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrder(context.Context, string, string, *exchanges.ModifyOrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) ModifyOrderWS(context.Context, string, string, *exchanges.ModifyOrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) PlaceOrder(context.Context, *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) CancelOrder(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) CancelAllOrders(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, exchanges.ErrNotSupported
}

func (a *Adapter) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, exchanges.ErrNotSupported
}

func (a *Adapter) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchOrderBook(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchFills(context.Context, exchanges.FillCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchOrders(context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchFills(context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchPositions(context.Context) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchTicker(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchTrades(context.Context, string) error {
	return exchanges.ErrNotSupported
}

func (a *Adapter) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return exchanges.ErrNotSupported
}
