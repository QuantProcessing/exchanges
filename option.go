package exchanges

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// OptionSymbolParts is the parsed form of a repository-canonical option symbol.
type OptionSymbolParts struct {
	BaseAsset   string
	QuoteAsset  string
	SettleAsset string
	ExpiryTime  int64
	StrikePrice decimal.Decimal
	Type        OptionType
	Legacy      bool
}

// NewOptionSymbol builds the repository-canonical option contract symbol:
// BASE-QUOTE-SETTLE-YYYYMMDD-STRIKE-C or BASE-QUOTE-SETTLE-YYYYMMDD-STRIKE-P.
func NewOptionSymbol(base, quote, settle string, expiryTime int64, strike decimal.Decimal, typ OptionType) string {
	base = strings.ToUpper(strings.TrimSpace(base))
	quote = strings.ToUpper(strings.TrimSpace(quote))
	settle = strings.ToUpper(strings.TrimSpace(settle))
	typ = NormalizeOptionType(string(typ))
	suffix := typ.Suffix()
	if base == "" || quote == "" || settle == "" || expiryTime <= 0 || suffix == "" {
		return ""
	}
	date := time.UnixMilli(expiryTime).UTC().Format("20060102")
	return strings.ToUpper(strings.Join([]string{
		base,
		quote,
		settle,
		date,
		strike.String(),
		suffix,
	}, "-"))
}

// ParseOptionSymbol parses both the current canonical option symbol and the
// legacy BASE-YYYYMMDD-STRIKE-C/P form accepted by early option adapters.
func ParseOptionSymbol(symbol string) (OptionSymbolParts, bool) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(symbol)), "-")
	switch len(parts) {
	case 6:
		return parseOptionSymbolParts(parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], false)
	case 4:
		return parseOptionSymbolParts(parts[0], "", "", parts[1], parts[2], parts[3], true)
	default:
		return OptionSymbolParts{}, false
	}
}

func parseOptionSymbolParts(base, quote, settle, date, strike, typ string, legacy bool) (OptionSymbolParts, bool) {
	if strings.TrimSpace(base) == "" || len(date) != 8 {
		return OptionSymbolParts{}, false
	}
	expiry, err := time.Parse("20060102", date)
	if err != nil {
		return OptionSymbolParts{}, false
	}
	strikePrice, err := decimal.NewFromString(strike)
	if err != nil {
		return OptionSymbolParts{}, false
	}
	optionType := NormalizeOptionType(typ)
	if optionType.Suffix() == "" {
		return OptionSymbolParts{}, false
	}
	return OptionSymbolParts{
		BaseAsset:   strings.ToUpper(strings.TrimSpace(base)),
		QuoteAsset:  strings.ToUpper(strings.TrimSpace(quote)),
		SettleAsset: strings.ToUpper(strings.TrimSpace(settle)),
		ExpiryTime:  expiry.UTC().UnixMilli(),
		StrikePrice: strikePrice,
		Type:        optionType,
		Legacy:      legacy,
	}, true
}

// Suffix returns the canonical single-letter option side.
func (t OptionType) Suffix() string {
	switch NormalizeOptionType(string(t)) {
	case OptionTypeCall:
		return "C"
	case OptionTypePut:
		return "P"
	default:
		return ""
	}
}

// NormalizeOptionType converts exchange spellings such as C/P or Call/Put.
func NormalizeOptionType(raw string) OptionType {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "C", "CALL":
		return OptionTypeCall
	case "P", "PUT":
		return OptionTypePut
	default:
		return ""
	}
}
