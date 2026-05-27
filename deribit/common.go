package deribit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/deribit/sdk"
	"github.com/shopspring/decimal"
)

const (
	kindFuture = "future"
	kindOption = "option"
)

type marketClient interface {
	GetInstruments(ctx context.Context, currency, kind string, expired bool) ([]sdk.Instrument, error)
	GetTicker(ctx context.Context, instrumentName string) (*sdk.Ticker, error)
	GetOrderBook(ctx context.Context, instrumentName string, depth int) (*sdk.OrderBook, error)
	GetLastTradesByInstrument(ctx context.Context, instrumentName string, count int) (*sdk.TradesResult, error)
	GetTradingViewChartData(ctx context.Context, instrumentName string, start, end int64, resolution string) (*sdk.TradingViewChartData, error)
	Buy(ctx context.Context, req sdk.OrderRequest) (*sdk.OrderResult, error)
	Sell(ctx context.Context, req sdk.OrderRequest) (*sdk.OrderResult, error)
	CancelOrder(ctx context.Context, orderID string) (*sdk.OrderRecord, error)
	CancelAll(ctx context.Context) (int64, error)
	CancelAllByInstrument(ctx context.Context, instrumentName string) (int64, error)
	GetOrderState(ctx context.Context, orderID string) (*sdk.OrderRecord, error)
	GetOpenOrdersByInstrument(ctx context.Context, instrumentName string) ([]sdk.OrderRecord, error)
	GetOrderHistoryByInstrument(ctx context.Context, instrumentName string, count int) ([]sdk.OrderRecord, error)
	HasCredentials() bool
}

var _ marketClient = (*sdk.Client)(nil)

func buildPerpMarketCache(instruments []sdk.Instrument) (*perpMarketCache, map[string]*exchanges.SymbolDetails) {
	cache := &perpMarketCache{
		byBase:   make(map[string]sdk.Instrument),
		bySymbol: make(map[string]sdk.Instrument),
	}
	details := make(map[string]*exchanges.SymbolDetails)
	for _, inst := range instruments {
		if !isActivePerpetual(inst) {
			continue
		}
		base := strings.ToUpper(inst.BaseCurrency)
		symbol := strings.ToUpper(inst.InstrumentName)
		if base == "" || symbol == "" {
			continue
		}
		cache.byBase[base] = inst
		cache.bySymbol[symbol] = inst
		details[base] = symbolDetailsFromInstrument(base, inst)
	}
	return cache, details
}

func isActivePerpetual(inst sdk.Instrument) bool {
	return strings.EqualFold(inst.Kind, kindFuture) &&
		strings.EqualFold(inst.SettlementPeriod, "perpetual") &&
		(inst.IsActive || strings.EqualFold(inst.State, "open"))
}

func symbolDetailsFromInstrument(symbol string, inst sdk.Instrument) *exchanges.SymbolDetails {
	minQty := decimalFromFloat(inst.MinTradeAmount)
	return &exchanges.SymbolDetails{
		Symbol:            strings.ToUpper(symbol),
		PricePrecision:    exchanges.CountDecimalPlaces(decimalFromFloat(inst.TickSize).String()),
		QuantityPrecision: exchanges.CountDecimalPlaces(minQty.String()),
		MinQuantity:       minQty,
	}
}

func decimalFromFloat(value float64) decimal.Decimal {
	if value == 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(value)
}

func hasAnyCredentials(opts Options) bool {
	return strings.TrimSpace(opts.APIKey) != "" || strings.TrimSpace(opts.SecretKey) != ""
}

func hasFullCredentials(opts Options) bool {
	return strings.TrimSpace(opts.APIKey) != "" && strings.TrimSpace(opts.SecretKey) != ""
}

func deribitAuthError(message string) error {
	return exchanges.NewExchangeError(exchangeName, "", message, exchanges.ErrAuthFailed)
}

func requirePrivateClientAccess(client marketClient) error {
	if client == nil || !client.HasCredentials() {
		return deribitAuthError("deribit: private access requires api_key and secret_key")
	}
	return nil
}

func toTicker(symbol string, raw *sdk.Ticker) *exchanges.Ticker {
	bid := decimalFromFloat(raw.BestBidPrice)
	ask := decimalFromFloat(raw.BestAskPrice)
	ticker := &exchanges.Ticker{
		Symbol:     strings.ToUpper(symbol),
		LastPrice:  decimalFromFloat(raw.LastPrice),
		IndexPrice: decimalFromFloat(raw.IndexPrice),
		MarkPrice:  decimalFromFloat(raw.MarkPrice),
		Bid:        bid,
		Ask:        ask,
		Volume24h:  decimalFromFloat(raw.Stats.Volume),
		QuoteVol:   decimalFromFloat(firstPositiveFloat(raw.Stats.VolumeUSD, raw.Stats.VolumeNotional)),
		High24h:    decimalFromFloat(raw.Stats.High),
		Low24h:     decimalFromFloat(raw.Stats.Low),
		Timestamp:  raw.Timestamp,
	}
	if bid.IsPositive() && ask.IsPositive() {
		ticker.MidPrice = bid.Add(ask).Div(decimal.NewFromInt(2))
	}
	return ticker
}

func toOrderBook(symbol string, raw *sdk.OrderBook) *exchanges.OrderBook {
	book := &exchanges.OrderBook{
		Symbol:    strings.ToUpper(symbol),
		Timestamp: raw.Timestamp,
		Bids:      make([]exchanges.Level, 0, len(raw.Bids)),
		Asks:      make([]exchanges.Level, 0, len(raw.Asks)),
	}
	for _, level := range raw.Bids {
		if len(level) < 2 {
			continue
		}
		book.Bids = append(book.Bids, exchanges.Level{Price: decimalFromFloat(level[0]), Quantity: decimalFromFloat(level[1])})
	}
	for _, level := range raw.Asks {
		if len(level) < 2 {
			continue
		}
		book.Asks = append(book.Asks, exchanges.Level{Price: decimalFromFloat(level[0]), Quantity: decimalFromFloat(level[1])})
	}
	return book
}

func mapTrades(symbol string, raw []sdk.Trade) []exchanges.Trade {
	out := make([]exchanges.Trade, 0, len(raw))
	for _, trade := range raw {
		side := exchanges.TradeSideBuy
		if strings.EqualFold(trade.Direction, "sell") {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        trade.TradeID,
			Symbol:    strings.ToUpper(symbol),
			Price:     decimalFromFloat(trade.Price),
			Quantity:  decimalFromFloat(trade.Amount),
			Side:      side,
			Timestamp: trade.Timestamp,
		})
	}
	return out
}

func mapKlines(symbol string, interval exchanges.Interval, raw *sdk.TradingViewChartData) []exchanges.Kline {
	n := minInt(len(raw.Ticks), len(raw.Open), len(raw.High), len(raw.Low), len(raw.Close), len(raw.Volume))
	out := make([]exchanges.Kline, 0, n)
	for i := 0; i < n; i++ {
		kline := exchanges.Kline{
			Symbol:    strings.ToUpper(symbol),
			Interval:  interval,
			Timestamp: raw.Ticks[i],
			Open:      decimalFromFloat(raw.Open[i]),
			High:      decimalFromFloat(raw.High[i]),
			Low:       decimalFromFloat(raw.Low[i]),
			Close:     decimalFromFloat(raw.Close[i]),
			Volume:    decimalFromFloat(raw.Volume[i]),
		}
		if i < len(raw.Cost) {
			kline.QuoteVol = decimalFromFloat(raw.Cost[i])
		}
		out = append(out, kline)
	}
	return out
}

func deribitDepth(limit int) int {
	if limit <= 0 {
		return 20
	}
	allowed := []int{1, 5, 10, 20, 50, 100, 1000, 10000}
	for _, value := range allowed {
		if limit <= value {
			return value
		}
	}
	return 10000
}

func deribitCount(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func deribitCurrencyParam(currency string) string {
	currency = strings.TrimSpace(currency)
	if strings.EqualFold(currency, "any") {
		return "any"
	}
	return strings.ToUpper(currency)
}

func deribitResolution(interval exchanges.Interval) (string, error) {
	switch interval {
	case exchanges.Interval1m:
		return "1", nil
	case exchanges.Interval3m:
		return "3", nil
	case exchanges.Interval5m:
		return "5", nil
	case exchanges.Interval15m:
		return "15", nil
	case exchanges.Interval30m:
		return "30", nil
	case exchanges.Interval1h:
		return "60", nil
	case exchanges.Interval2h:
		return "120", nil
	case exchanges.Interval3d:
		return "", fmt.Errorf("deribit: unsupported interval %s", interval)
	case exchanges.Interval4h:
		return "", fmt.Errorf("deribit: unsupported interval %s", interval)
	case exchanges.Interval6h:
		return "360", nil
	case exchanges.Interval12h:
		return "720", nil
	case exchanges.Interval1d:
		return "1D", nil
	default:
		return "", fmt.Errorf("deribit: unsupported interval %s", interval)
	}
}

func klineTimeRange(interval exchanges.Interval, opts *exchanges.KlineOpts) (int64, int64, error) {
	end := time.Now().UTC()
	if opts != nil && opts.End != nil {
		end = opts.End.UTC()
	}
	limit := 100
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	dur, err := intervalDuration(interval)
	if err != nil {
		return 0, 0, err
	}
	start := end.Add(-time.Duration(limit) * dur)
	if opts != nil && opts.Start != nil {
		start = opts.Start.UTC()
	}
	return start.UnixMilli(), end.UnixMilli(), nil
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
	case exchanges.Interval12h:
		return 12 * time.Hour, nil
	case exchanges.Interval1d:
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("deribit: unsupported interval %s", interval)
	}
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func parseNativeDate(raw string) (int64, bool) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if len(raw) != 7 {
		return 0, false
	}
	day, err := strconv.Atoi(raw[:2])
	if err != nil {
		return 0, false
	}
	year, err := strconv.Atoi(raw[5:])
	if err != nil {
		return 0, false
	}
	month, ok := monthNumber(raw[2:5])
	if !ok {
		return 0, false
	}
	ts := time.Date(2000+year, month, day, 0, 0, 0, 0, time.UTC)
	return ts.UnixMilli(), true
}

func monthNumber(raw string) (time.Month, bool) {
	switch strings.ToUpper(raw) {
	case "JAN":
		return time.January, true
	case "FEB":
		return time.February, true
	case "MAR":
		return time.March, true
	case "APR":
		return time.April, true
	case "MAY":
		return time.May, true
	case "JUN":
		return time.June, true
	case "JUL":
		return time.July, true
	case "AUG":
		return time.August, true
	case "SEP":
		return time.September, true
	case "OCT":
		return time.October, true
	case "NOV":
		return time.November, true
	case "DEC":
		return time.December, true
	default:
		return 0, false
	}
}
