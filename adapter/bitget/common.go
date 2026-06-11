package bitget

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/sdk/bitget"
	"github.com/shopspring/decimal"
)

const exchangeName = "BITGET"

const (
	categorySpot        = "SPOT"
	categoryUSDTFutures = "USDT-FUTURES"
	categoryUSDCFutures = "USDC-FUTURES"
)

type MarketCatalog struct {
	spotByBase map[string]sdk.Instrument
	perpByBase map[string]sdk.Instrument
	bySymbol   map[string]sdk.Instrument
}

type marketCache = MarketCatalog

func newMarketCache() *MarketCatalog {
	return &MarketCatalog{
		spotByBase: make(map[string]sdk.Instrument),
		perpByBase: make(map[string]sdk.Instrument),
		bySymbol:   make(map[string]sdk.Instrument),
	}
}

func buildMarketCache(instruments []sdk.Instrument, quote exchanges.QuoteCurrency) *MarketCatalog {
	cache := newMarketCache()
	for _, inst := range instruments {
		if strings.ToLower(inst.Status) != "online" {
			continue
		}

		marketType := exchanges.MarketTypeSpot
		if isPerpCategory(inst.Category) {
			marketType = exchanges.MarketTypePerp
		}
		market := marketRefFromInstrument(inst, quote, marketType)
		if market.Base == "" || !isSupportedBitgetQuote(market.Quote) {
			continue
		}
		symbol := strings.ToUpper(inst.Symbol)
		cache.bySymbol[symbol] = inst

		switch strings.ToUpper(inst.Category) {
		case categorySpot:
			cache.spotByBase[market.Symbol()] = inst
			if string(market.Quote) == string(quote) {
				cache.spotByBase[market.Base] = inst
			}
		case categoryUSDTFutures, categoryUSDCFutures:
			cache.perpByBase[market.Symbol()] = inst
			if string(market.Quote) == string(quote) {
				cache.perpByBase[market.Base] = inst
			}
		}
	}
	return cache
}

func (c *MarketCatalog) FormatSymbol(symbol string, quote exchanges.QuoteCurrency, marketType exchanges.MarketType) string {
	market := exchanges.ParseMarketRef(symbol, quote, marketType)
	if c == nil {
		return market.Base + string(market.Quote)
	}

	var inst sdk.Instrument
	var ok bool
	switch marketType {
	case exchanges.MarketTypeSpot:
		inst, ok = c.spotByBase[market.Base]
		if ok && string(market.Quote) == string(quote) {
			return inst.Symbol
		}
		inst, ok = c.spotByBase[market.Symbol()]
	case exchanges.MarketTypePerp:
		inst, ok = c.perpByBase[market.Base]
		if ok && string(market.Quote) == string(quote) {
			return inst.Symbol
		}
		inst, ok = c.perpByBase[market.Symbol()]
	}
	if ok {
		return inst.Symbol
	}
	return market.Base + string(market.Quote)
}

func (c *MarketCatalog) ExtractSymbol(symbol string, quote exchanges.QuoteCurrency, marketType exchanges.MarketType) string {
	upper := strings.ToUpper(symbol)
	if c != nil {
		if inst, ok := c.bySymbol[upper]; ok && inst.BaseCoin != "" {
			return marketRefFromInstrument(inst, quote, marketType).Symbol()
		}
	}
	return exchanges.ParseMarketRef(upper, quote, marketType).Symbol()
}

func authError(message string) error {
	return exchanges.NewExchangeError(exchangeName, "", message, exchanges.ErrAuthFailed)
}

func hasAnyCredentials(opts Options) bool {
	return opts.APIKey != "" || opts.SecretKey != "" || opts.Passphrase != ""
}

func hasFullCredentials(opts Options) bool {
	return opts.APIKey != "" && opts.SecretKey != "" && opts.Passphrase != ""
}

func ensureSupportedAccountMode(opts Options) error {
	_, err := opts.accountMode()
	return err
}

func isPerpCategory(category string) bool {
	switch strings.ToUpper(category) {
	case categoryUSDTFutures, categoryUSDCFutures:
		return true
	default:
		return false
	}
}

func quoteToPerpCategory(quote exchanges.QuoteCurrency) string {
	switch quote {
	case exchanges.QuoteCurrencyUSDC:
		return categoryUSDCFutures
	default:
		return categoryUSDTFutures
	}
}

func requirePrivateAccess(client *sdk.Client) error {
	if client == nil || !client.HasCredentials() {
		return authError("bitget: private access requires api_key, secret_key, and passphrase")
	}
	return nil
}

func buildSymbolDetails(instruments []sdk.Instrument, quote exchanges.QuoteCurrency, marketType exchanges.MarketType) map[string]*exchanges.SymbolDetails {
	details := make(map[string]*exchanges.SymbolDetails)
	for _, inst := range instruments {
		if strings.ToLower(inst.Status) != "online" {
			continue
		}
		switch marketType {
		case exchanges.MarketTypeSpot:
			if strings.ToUpper(inst.Category) != categorySpot {
				continue
			}
		case exchanges.MarketTypePerp:
			if !isPerpCategory(inst.Category) {
				continue
			}
		}

		detail, err := symbolDetailsFromInstrument(inst)
		if err != nil {
			continue
		}
		market := marketRefFromInstrument(inst, quote, marketType)
		if market.Base == "" || !isSupportedBitgetQuote(market.Quote) {
			continue
		}
		detail.Symbol = market.Symbol()
		details[market.Symbol()] = detail
		if string(market.Quote) == string(quote) {
			details[market.Base] = detail
		}
	}
	return details
}

func symbolDetailsFromInstrument(inst sdk.Instrument) (*exchanges.SymbolDetails, error) {
	pricePrecision, err := strconv.ParseInt(inst.PricePrecision, 10, 32)
	if err != nil && inst.PricePrecision != "" {
		return nil, err
	}
	qtyPrecision, err := strconv.ParseInt(inst.QuantityPrecision, 10, 32)
	if err != nil && inst.QuantityPrecision != "" {
		return nil, err
	}

	minQty := parseDecimal(inst.MinOrderQty)
	minNotional := parseDecimal(inst.MinOrderAmount)
	return &exchanges.SymbolDetails{
		Symbol:            strings.ToUpper(inst.BaseCoin),
		PricePrecision:    int32(pricePrecision),
		QuantityPrecision: int32(qtyPrecision),
		MinQuantity:       minQty,
		MinNotional:       minNotional,
	}, nil
}

func marketRefFromInstrument(inst sdk.Instrument, defaultQuote exchanges.QuoteCurrency, marketType exchanges.MarketType) exchanges.MarketRef {
	base := strings.ToUpper(inst.BaseCoin)
	quote := strings.ToUpper(inst.QuoteCoin)
	if base != "" && quote != "" {
		return exchanges.MarketRef{
			Base:   base,
			Quote:  exchanges.QuoteCurrency(quote),
			Settle: exchanges.QuoteCurrency(quote),
			Type:   marketType,
		}
	}
	return exchanges.ParseMarketRef(inst.Symbol, defaultQuote, marketType)
}

func isSupportedBitgetQuote(quote exchanges.QuoteCurrency) bool {
	switch quote {
	case exchanges.QuoteCurrencyUSDT, exchanges.QuoteCurrencyUSDC:
		return true
	default:
		return false
	}
}

func parseDecimal(raw string) decimal.Decimal {
	if raw == "" {
		return decimal.Zero
	}
	parsed, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero
	}
	return parsed
}

func parseMillis(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func klineIntervalString(interval exchanges.Interval) (string, error) {
	switch interval {
	case exchanges.Interval1m,
		exchanges.Interval3m,
		exchanges.Interval5m,
		exchanges.Interval15m,
		exchanges.Interval30m:
		return string(interval), nil
	case exchanges.Interval1h:
		return "1H", nil
	case exchanges.Interval4h:
		return "4H", nil
	case exchanges.Interval6h:
		return "6H", nil
	case exchanges.Interval12h:
		return "12H", nil
	case exchanges.Interval1d:
		return "1D", nil
	default:
		return "", fmt.Errorf("bitget: unsupported interval %s", interval)
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
	case exchanges.Interval4h:
		return 4 * time.Hour, nil
	case exchanges.Interval6h:
		return 6 * time.Hour, nil
	case exchanges.Interval12h:
		return 12 * time.Hour, nil
	case exchanges.Interval1d:
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("bitget: unsupported interval %s", interval)
	}
}

func klineTimeRange(interval exchanges.Interval, opts *exchanges.KlineOpts) (int64, int64, int, error) {
	dur, err := intervalDuration(interval)
	if err != nil {
		return 0, 0, 0, err
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
	return start.UnixMilli(), end.UnixMilli(), limit, nil
}

func toTicker(symbol string, raw *sdk.Ticker) *exchanges.Ticker {
	ticker := &exchanges.Ticker{
		Symbol:     strings.ToUpper(symbol),
		LastPrice:  parseDecimal(raw.LastPrice),
		IndexPrice: parseDecimal(raw.IndexPrice),
		MarkPrice:  parseDecimal(raw.MarkPrice),
		Bid:        parseDecimal(raw.Bid1Price),
		Ask:        parseDecimal(raw.Ask1Price),
		Volume24h:  parseDecimal(raw.Volume24h),
		QuoteVol:   parseDecimal(raw.Turnover24h),
		High24h:    parseDecimal(raw.HighPrice24h),
		Low24h:     parseDecimal(raw.LowPrice24h),
		Timestamp:  parseMillis(raw.Timestamp),
	}
	if ticker.Bid.IsPositive() && ticker.Ask.IsPositive() {
		ticker.MidPrice = ticker.Bid.Add(ticker.Ask).Div(decimal.NewFromInt(2))
	}
	return ticker
}

func toOrderBook(symbol string, raw *sdk.OrderBook) *exchanges.OrderBook {
	return &exchanges.OrderBook{
		Symbol:    strings.ToUpper(symbol),
		Timestamp: parseMillis(raw.TS),
		Bids:      toLevels(raw.Bids),
		Asks:      toLevels(raw.Asks),
	}
}

func toLevels(levels [][]sdk.NumberString) []exchanges.Level {
	out := make([]exchanges.Level, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		out = append(out, exchanges.Level{
			Price:    parseDecimal(string(level[0])),
			Quantity: parseDecimal(string(level[1])),
		})
	}
	return out
}

func mapTrades(symbol string, raw []sdk.PublicFill) []exchanges.Trade {
	out := make([]exchanges.Trade, 0, len(raw))
	for _, fill := range raw {
		side := exchanges.TradeSideBuy
		if strings.EqualFold(fill.Side, "sell") {
			side = exchanges.TradeSideSell
		}
		out = append(out, exchanges.Trade{
			ID:        fill.ExecID,
			Symbol:    strings.ToUpper(symbol),
			Price:     parseDecimal(fill.Price),
			Quantity:  parseDecimal(fill.Size),
			Side:      side,
			Timestamp: parseMillis(fill.Timestamp),
		})
	}
	return out
}

func mapKlines(symbol string, interval exchanges.Interval, raw []sdk.Candle) []exchanges.Kline {
	out := make([]exchanges.Kline, 0, len(raw))
	for _, candle := range raw {
		out = append(out, exchanges.Kline{
			Symbol:    strings.ToUpper(symbol),
			Interval:  interval,
			Timestamp: parseMillis(string(candle[0])),
			Open:      parseDecimal(string(candle[1])),
			High:      parseDecimal(string(candle[2])),
			Low:       parseDecimal(string(candle[3])),
			Close:     parseDecimal(string(candle[4])),
			Volume:    parseDecimal(string(candle[5])),
			QuoteVol:  parseDecimal(string(candle[6])),
		})
	}
	return out
}
