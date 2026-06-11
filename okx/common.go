package okx

import (
	"strconv"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
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
	market := exchanges.ParseMarketRef(symbol, exchanges.QuoteCurrency(quote), exchanges.MarketTypePerp)
	suffix := "-" + strings.ToUpper(instType)
	if strings.HasSuffix(strings.ToUpper(symbol), suffix) {
		return strings.ToUpper(symbol)
	}
	return market.Base + "-" + string(market.Quote) + suffix
}

// FormatSpotSymbolWithQuote converts base currency for OKX spot (e.g., BTC + USDC → BTC-USDC)
func FormatSpotSymbolWithQuote(symbol, quote string) string {
	market := exchanges.ParseMarketRef(symbol, exchanges.QuoteCurrency(quote), exchanges.MarketTypeSpot)
	return market.Base + "-" + string(market.Quote)
}

// ExtractSymbolWithQuote extracts base currency by trimming quote-related suffixes
func ExtractSymbolWithQuote(symbol, quote string) string {
	market := exchanges.ParseMarketRef(symbol, exchanges.QuoteCurrency(quote), "")
	return market.Symbol()
}

func isSupportedOKXQuote(quote exchanges.QuoteCurrency) bool {
	switch quote {
	case exchanges.QuoteCurrencyUSDT, exchanges.QuoteCurrencyUSDC:
		return true
	default:
		return false
	}
}
