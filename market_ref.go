package exchanges

import "strings"

// MarketRef is the adapter-level market identity.
//
// Base-only strings are legacy convenience. New adapter code should carry at
// least base + quote, and perp/future code should carry settle when it differs
// from quote.
type MarketRef struct {
	Base        string
	Quote       QuoteCurrency
	Settle      QuoteCurrency
	Type        MarketType
	VenueSymbol string
}

// ParseMarketRef parses common market identity forms into a normalized
// MarketRef. Supported forms include "BTC", "BTC/USDT", "BTC/USDT:USDC",
// "BTC-USDT", and "BTC-USDT-SWAP".
func ParseMarketRef(symbol string, defaultQuote QuoteCurrency, marketType MarketType) MarketRef {
	s := strings.TrimSpace(strings.ToUpper(symbol))
	defaultQuote = normalizeQuote(defaultQuote)
	ref := MarketRef{
		Base:   s,
		Quote:  defaultQuote,
		Settle: defaultQuote,
		Type:   marketType,
	}

	if s == "" {
		return ref
	}

	if strings.Contains(s, "/") {
		base, rest, _ := strings.Cut(s, "/")
		quote, settle, _ := strings.Cut(rest, ":")
		ref.Base = strings.TrimSpace(base)
		ref.Quote = normalizeQuote(QuoteCurrency(strings.TrimSpace(quote)))
		if settle == "" {
			ref.Settle = ref.Quote
		} else {
			ref.Settle = normalizeQuote(QuoteCurrency(strings.TrimSpace(settle)))
		}
		return ref
	}

	if strings.Contains(s, "-") {
		parts := strings.Split(s, "-")
		if len(parts) >= 2 && isKnownQuote(parts[1]) {
			ref.Base = strings.TrimSpace(parts[0])
			ref.Quote = normalizeQuote(QuoteCurrency(strings.TrimSpace(parts[1])))
			ref.Settle = ref.Quote
			if len(parts) >= 4 && isKnownQuote(parts[3]) {
				ref.Settle = normalizeQuote(QuoteCurrency(strings.TrimSpace(parts[3])))
			}
			return ref
		}
	}

	if base, quote, ok := splitKnownQuoteSuffix(s); ok {
		ref.Base = base
		ref.Quote = normalizeQuote(QuoteCurrency(quote))
		ref.Settle = ref.Quote
		return ref
	}

	ref.Base = s
	return ref
}

func (m MarketRef) Symbol() string {
	if m.Quote == "" {
		return strings.ToUpper(m.Base)
	}
	return strings.ToUpper(m.Base) + "/" + string(normalizeQuote(m.Quote))
}

func (m MarketRef) String() string {
	symbol := m.Symbol()
	if m.Settle != "" && normalizeQuote(m.Settle) != normalizeQuote(m.Quote) {
		return symbol + ":" + string(normalizeQuote(m.Settle))
	}
	return symbol
}

func (m MarketRef) Key() string {
	typ := string(m.Type)
	if typ == "" {
		typ = "unknown"
	}
	settle := normalizeQuote(m.Settle)
	if settle == "" {
		settle = normalizeQuote(m.Quote)
	}
	return typ + ":" + m.Symbol() + ":" + string(settle)
}

func normalizeQuote(q QuoteCurrency) QuoteCurrency {
	q = QuoteCurrency(strings.TrimSpace(strings.ToUpper(string(q))))
	if q == "" {
		return QuoteCurrencyUSDT
	}
	return q
}

func isKnownQuote(s string) bool {
	switch QuoteCurrency(strings.TrimSpace(strings.ToUpper(s))) {
	case QuoteCurrencyUSDT, QuoteCurrencyUSDC, QuoteCurrencyDUSD:
		return true
	default:
		return false
	}
}

func splitKnownQuoteSuffix(symbol string) (base string, quote string, ok bool) {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	for _, q := range []QuoteCurrency{QuoteCurrencyUSDT, QuoteCurrencyUSDC, QuoteCurrencyDUSD} {
		quote := string(q)
		if strings.HasSuffix(s, quote) && len(s) > len(quote) {
			return strings.TrimSuffix(s, quote), quote, true
		}
	}
	return "", "", false
}
