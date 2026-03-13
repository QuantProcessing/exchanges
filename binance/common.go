package binance

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseInt64(v interface{}) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case int64:
		return val
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	default:
		return 0
	}
}

func parseDecimalInterface(v interface{}) decimal.Decimal {
	switch val := v.(type) {
	case float64:
		return decimal.NewFromFloat(val)
	case string:
		d, _ := decimal.NewFromString(val)
		return d
	default:
		return decimal.Zero
	}
}

func boolToMarginType(isolated bool) string {
	if isolated {
		return "ISOLATED"
	}
	return "CROSSED"
}

// FormatSymbol converts base currency to exchange-specific format (e.g., BTC → btcusdt)
// Kept for backward compatibility and SDK-level usage.
func FormatSymbol(symbol string) string {
	return FormatSymbolWithQuote(symbol, "USDT")
}

// ExtractSymbol extracts base currency from exchange-specific format (e.g., BTCUSDT → BTC)
// Kept for backward compatibility and SDK-level usage.
func ExtractSymbol(symbol string) string {
	return ExtractSymbolWithQuote(symbol, "USDT")
}

// FormatSymbolWithQuote converts base currency with configurable quote (e.g., BTC + USDC → btcusdc)
func FormatSymbolWithQuote(symbol, quote string) string {
	s := strings.ToLower(symbol)
	q := strings.ToLower(quote)
	if !strings.HasSuffix(s, q) {
		s += q
	}
	return s
}

// ExtractSymbolWithQuote extracts base currency by trimming configurable quote (e.g., BTCUSDC → BTC)
func ExtractSymbolWithQuote(symbol, quote string) string {
	s := strings.ToUpper(symbol)
	return strings.TrimSuffix(s, strings.ToUpper(quote))
}

