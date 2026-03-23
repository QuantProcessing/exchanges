package backpack

import (
	"context"
	"fmt"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
)

type adapterRESTClient interface {
	GetMarkets(ctx context.Context) ([]sdk.Market, error)
	GetTicker(ctx context.Context, symbol string) (*sdk.Ticker, error)
	GetDepth(ctx context.Context, symbol string, limit int) (*sdk.Depth, error)
	GetTrades(ctx context.Context, symbol string, limit int) ([]sdk.Trade, error)
	GetFundingRates(ctx context.Context) ([]sdk.FundingRate, error)
	GetKlines(ctx context.Context, symbol, interval string, startTime, endTime int64, priceType string) ([]sdk.Kline, error)
	GetAccount(ctx context.Context) (*sdk.AccountSettings, error)
	GetBalances(ctx context.Context) (map[string]sdk.CapitalBalance, error)
	GetOpenOrders(ctx context.Context, marketType, symbol string) ([]sdk.Order, error)
	GetOpenPositions(ctx context.Context, symbol string) ([]sdk.Position, error)
	ExecuteOrder(ctx context.Context, req sdk.CreateOrderRequest) (*sdk.Order, error)
	CancelOrder(ctx context.Context, req sdk.CancelOrderRequest) (*sdk.Order, error)
	CancelOpenOrders(ctx context.Context, symbol, marketType string) error
}

var _ adapterRESTClient = (*sdk.Client)(nil)

type marketCache struct {
	spotByBase map[string]sdk.Market
	perpByBase map[string]sdk.Market
	bySymbol   map[string]sdk.Market
}

func buildMarketCache(markets []sdk.Market, quote exchanges.QuoteCurrency) (*marketCache, error) {
	cache := &marketCache{
		spotByBase: make(map[string]sdk.Market),
		perpByBase: make(map[string]sdk.Market),
		bySymbol:   make(map[string]sdk.Market),
	}

	for _, market := range markets {
		if strings.ToUpper(market.QuoteSymbol) != string(quote) {
			continue
		}
		if !market.Visible {
			continue
		}

		base := strings.ToUpper(market.BaseSymbol)
		cache.bySymbol[market.Symbol] = market

		switch strings.ToUpper(market.MarketType) {
		case "SPOT":
			cache.spotByBase[base] = market
		case "PERP":
			cache.perpByBase[base] = market
		}
	}

	return cache, nil
}

func symbolDetailsFromMarket(market sdk.Market) (*exchanges.SymbolDetails, error) {
	minQty, _ := decimal.NewFromString(market.Filters.Quantity.MinQuantity)
	minPrice, _ := decimal.NewFromString(market.Filters.Price.MinPrice)

	return &exchanges.SymbolDetails{
		Symbol:            strings.ToUpper(market.BaseSymbol),
		PricePrecision:    precisionFromIncrement(market.Filters.Price.TickSize),
		QuantityPrecision: precisionFromIncrement(market.Filters.Quantity.StepSize),
		MinQuantity:       minQty,
		MinNotional:       minQty.Mul(minPrice),
	}, nil
}

func precisionFromIncrement(raw string) int32 {
	if raw == "" {
		return 0
	}
	return exchanges.CountDecimalPlaces(raw)
}

func buildSymbolDetails(markets []sdk.Market, quote exchanges.QuoteCurrency, marketType exchanges.MarketType) map[string]*exchanges.SymbolDetails {
	details := make(map[string]*exchanges.SymbolDetails)
	for _, market := range markets {
		if strings.ToUpper(market.QuoteSymbol) != string(quote) {
			continue
		}
		if !market.Visible {
			continue
		}
		switch marketType {
		case exchanges.MarketTypeSpot:
			if strings.ToUpper(market.MarketType) != "SPOT" {
				continue
			}
		case exchanges.MarketTypePerp:
			if strings.ToUpper(market.MarketType) != "PERP" {
				continue
			}
		}
		detail, err := symbolDetailsFromMarket(market)
		if err != nil {
			continue
		}
		details[detail.Symbol] = detail
	}
	return details
}

func parseDecimal(raw string) decimal.Decimal {
	d, _ := decimal.NewFromString(raw)
	return d
}

func microsToMillis(v int64) int64 {
	if v > 100_000_000_000_000 {
		return v / 1000
	}
	return v
}

func klineIntervalString(interval exchanges.Interval) (string, error) {
	switch interval {
	case exchanges.Interval1M:
		return "1month", nil
	case exchanges.Interval1m,
		exchanges.Interval3m,
		exchanges.Interval5m,
		exchanges.Interval15m,
		exchanges.Interval30m,
		exchanges.Interval1h,
		exchanges.Interval2h,
		exchanges.Interval4h,
		exchanges.Interval6h,
		exchanges.Interval8h,
		exchanges.Interval12h,
		exchanges.Interval1d,
		exchanges.Interval3d,
		exchanges.Interval1w:
		return string(interval), nil
	default:
		return "", fmt.Errorf("backpack: unsupported interval %s", interval)
	}
}

func intervalDuration(interval exchanges.Interval) (time.Duration, error) {
	switch interval {
	case exchanges.Interval1m:
		return time.Minute, nil
	case exchanges.Interval3m:
		return 3 * time.Minute, nil
	case exchanges.Interval5m:
		return 5 * time.Minute, nil
	case exchanges.Interval15m:
		return 15 * time.Minute, nil
	case exchanges.Interval30m:
		return 30 * time.Minute, nil
	case exchanges.Interval1h:
		return time.Hour, nil
	case exchanges.Interval2h:
		return 2 * time.Hour, nil
	case exchanges.Interval4h:
		return 4 * time.Hour, nil
	case exchanges.Interval6h:
		return 6 * time.Hour, nil
	case exchanges.Interval8h:
		return 8 * time.Hour, nil
	case exchanges.Interval12h:
		return 12 * time.Hour, nil
	case exchanges.Interval1d:
		return 24 * time.Hour, nil
	case exchanges.Interval3d:
		return 72 * time.Hour, nil
	case exchanges.Interval1w:
		return 7 * 24 * time.Hour, nil
	case exchanges.Interval1M:
		return 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("backpack: unsupported interval %s", interval)
	}
}

func klineTimeRange(interval exchanges.Interval, opts *exchanges.KlineOpts) (int64, int64, error) {
	dur, err := intervalDuration(interval)
	if err != nil {
		return 0, 0, err
	}

	end := time.Now().UTC()
	if opts != nil && opts.End != nil {
		end = opts.End.UTC()
	}

	limit := 100
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	start := end.Add(-time.Duration(limit) * dur)
	if opts != nil && opts.Start != nil {
		start = opts.Start.UTC()
	}

	return start.Unix(), end.Unix(), nil
}

func parseBackpackTime(raw string) int64 {
	if raw == "" {
		return 0
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC().UnixMilli()
		}
	}
	return 0
}

func mapKlines(symbol string, interval exchanges.Interval, raw []sdk.Kline) []exchanges.Kline {
	out := make([]exchanges.Kline, 0, len(raw))
	for _, item := range raw {
		out = append(out, exchanges.Kline{
			Symbol:    strings.ToUpper(symbol),
			Interval:  interval,
			Open:      parseDecimal(item.Open),
			High:      parseDecimal(item.High),
			Low:       parseDecimal(item.Low),
			Close:     parseDecimal(item.Close),
			Volume:    parseDecimal(item.Volume),
			QuoteVol:  parseDecimal(item.QuoteVolume),
			Timestamp: parseBackpackTime(item.Start),
		})
	}
	return out
}

func toTicker(symbol string, raw *sdk.Ticker) *exchanges.Ticker {
	return &exchanges.Ticker{
		Symbol:    strings.ToUpper(symbol),
		LastPrice: parseDecimal(raw.LastPrice),
		High24h:   parseDecimal(raw.High),
		Low24h:    parseDecimal(raw.Low),
		Volume24h: parseDecimal(raw.Volume),
		QuoteVol:  parseDecimal(raw.QuoteVolume),
		Timestamp: time.Now().UnixMilli(),
	}
}
