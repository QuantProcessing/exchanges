package bitget

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

const exchangeName = "BITGET"

const (
	categorySpot        = "SPOT"
	categoryUSDTFutures = "USDT-FUTURES"
	categoryUSDCFutures = "USDC-FUTURES"
	accountModeUnified  = "unified"
	accountModeHybrid   = "hybrid"
)

type marketCache struct {
	spotByBase map[string]sdk.Instrument
	perpByBase map[string]sdk.Instrument
	bySymbol   map[string]sdk.Instrument
}

func newMarketCache() *marketCache {
	return &marketCache{
		spotByBase: make(map[string]sdk.Instrument),
		perpByBase: make(map[string]sdk.Instrument),
		bySymbol:   make(map[string]sdk.Instrument),
	}
}

func buildMarketCache(instruments []sdk.Instrument, quote exchanges.QuoteCurrency) *marketCache {
	cache := newMarketCache()
	for _, inst := range instruments {
		if strings.ToUpper(inst.QuoteCoin) != string(quote) {
			continue
		}
		if strings.ToLower(inst.Status) != "online" {
			continue
		}

		base := strings.ToUpper(inst.BaseCoin)
		symbol := strings.ToUpper(inst.Symbol)
		cache.bySymbol[symbol] = inst

		switch strings.ToUpper(inst.Category) {
		case categorySpot:
			cache.spotByBase[base] = inst
		case categoryUSDTFutures, categoryUSDCFutures:
			cache.perpByBase[base] = inst
		}
	}
	return cache
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

func validatePrivateInit(ctx context.Context, client *sdk.Client, opts Options) error {
	_, err := detectPrivateAccountMode(ctx, client, opts)
	return err
}

func detectPrivateAccountMode(ctx context.Context, client *sdk.Client, opts Options) (string, error) {
	if !hasAnyCredentials(opts) {
		mode, err := opts.accountMode()
		if err != nil {
			return "", err
		}
		return mode, nil
	}
	if !hasFullCredentials(opts) {
		return "", authError("bitget: api_key, secret_key, and passphrase must all be set together")
	}

	mode, err := opts.accountMode()
	if err != nil {
		return "", err
	}

	switch mode {
	case accountModeClassic:
		return accountModeClassic, nil
	case accountModeUTA:
		if err := validateUTAAccount(ctx, client); err != nil {
			return "", err
		}
		return accountModeUTA, nil
	default:
		detected, err := autoDetectAccountMode(ctx, client)
		if err != nil {
			return "", err
		}
		return detected, nil
	}
}

func autoDetectAccountMode(ctx context.Context, client *sdk.Client) (string, error) {
	settings, err := client.GetAccountSettings(ctx)
	if err != nil {
		if isClassicAccountModeError(err) {
			return accountModeClassic, nil
		}
		return "", err
	}
	if isUTASettings(settings) {
		return accountModeUTA, nil
	}
	return accountModeClassic, nil
}

func validateUTAAccount(ctx context.Context, client *sdk.Client) error {
	settings, err := client.GetAccountSettings(ctx)
	if err != nil {
		if isClassicAccountModeError(err) {
			return authError("bitget: UTA account required for private access")
		}
		return err
	}
	if !isUTASettings(settings) {
		return authError("bitget: UTA account required for private access")
	}
	return nil
}

func isUTASettings(settings *sdk.AccountSettings) bool {
	if settings == nil {
		return false
	}
	mode := strings.ToLower(settings.AccountMode)
	return mode == accountModeUnified || mode == accountModeHybrid
}

func isClassicAccountModeError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "40084") || strings.Contains(lower, "classic account mode")
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
		if strings.ToUpper(inst.QuoteCoin) != string(quote) {
			continue
		}
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
		details[detail.Symbol] = detail
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
