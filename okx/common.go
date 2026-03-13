package okx

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseString(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseInt64(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

// FormatSymbol converts base currency to OKX perp format (e.g., BTC → BTC-USDT-SWAP)
// Kept for backward compatibility.
func FormatSymbol(symbol string) string {
	return FormatSymbolWithQuote(symbol, "USDT", "SWAP")
}

// ExtractSymbol extracts base currency from OKX format (e.g., BTC-USDT-SWAP → BTC)
// Kept for backward compatibility.
func ExtractSymbol(symbol string) string {
	return ExtractSymbolWithQuote(symbol, "USDT")
}

// FormatSymbolWithQuote converts base currency with configurable quote and instType
// (e.g., BTC + USDC + SWAP → BTC-USDC-SWAP)
func FormatSymbolWithQuote(symbol, quote, instType string) string {
	s := strings.ToUpper(symbol)
	suffix := "-" + strings.ToUpper(quote) + "-" + strings.ToUpper(instType)
	if !strings.HasSuffix(s, suffix) {
		s += suffix
	}
	return s
}

// FormatSpotSymbolWithQuote converts base currency for OKX spot (e.g., BTC + USDC → BTC-USDC)
func FormatSpotSymbolWithQuote(symbol, quote string) string {
	s := strings.ToUpper(symbol)
	suffix := "-" + strings.ToUpper(quote)
	if strings.Contains(s, "-") {
		return s
	}
	return s + suffix
}

// ExtractSymbolWithQuote extracts base currency by trimming quote-related suffixes
func ExtractSymbolWithQuote(symbol, quote string) string {
	s := strings.ToUpper(symbol)
	q := strings.ToUpper(quote)
	s = strings.TrimSuffix(s, "-"+q+"-SWAP")
	s = strings.TrimSuffix(s, "-"+q)
	return s
}
