package exchanges

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// =============================================================================
// Instrument-type discriminator
// =============================================================================

// InstrumentType identifies the kind of contract a Position represents.
// Every Position returned by an adapter MUST set this field; the zero value
// is reserved as a programming error and rejected by compliance tests.
type InstrumentType string

const (
	InstrumentTypeSpot   InstrumentType = "spot"
	InstrumentTypePerp   InstrumentType = "perp"
	InstrumentTypeFuture InstrumentType = "future"
	InstrumentTypeOption InstrumentType = "option"
)

// =============================================================================
// Option primitives
// =============================================================================

// OptionKind distinguishes call vs put. The single-letter form matches the
// instrument-id convention used by most venues (Binance/Deribit/OKX/Bybit
// all encode call/put as a trailing "-C"/"-P").
type OptionKind string

const (
	OptionCall OptionKind = "C"
	OptionPut  OptionKind = "P"
)

// OptionSymbolParts is the parsed form of a repository-canonical option symbol.
type OptionSymbolParts struct {
	BaseAsset   string
	QuoteAsset  string
	SettleAsset string
	ExpiryTime  int64
	StrikePrice decimal.Decimal
	Kind        OptionKind
	Legacy      bool
}

// NewOptionSymbol builds the repository-canonical option contract symbol:
// BASE-QUOTE-SETTLE-YYYYMMDD-STRIKE-C or BASE-QUOTE-SETTLE-YYYYMMDD-STRIKE-P.
func NewOptionSymbol(base, quote, settle string, expiryTime int64, strike decimal.Decimal, kind OptionKind) string {
	base = strings.ToUpper(strings.TrimSpace(base))
	quote = strings.ToUpper(strings.TrimSpace(quote))
	settle = strings.ToUpper(strings.TrimSpace(settle))
	kind = NormalizeOptionKind(string(kind))
	suffix := kind.Suffix()
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

func parseOptionSymbolParts(base, quote, settle, date, strike, kind string, legacy bool) (OptionSymbolParts, bool) {
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
	optionKind := NormalizeOptionKind(kind)
	if optionKind.Suffix() == "" {
		return OptionSymbolParts{}, false
	}
	return OptionSymbolParts{
		BaseAsset:   strings.ToUpper(strings.TrimSpace(base)),
		QuoteAsset:  strings.ToUpper(strings.TrimSpace(quote)),
		SettleAsset: strings.ToUpper(strings.TrimSpace(settle)),
		ExpiryTime:  expiry.UTC().UnixMilli(),
		StrikePrice: strikePrice,
		Kind:        optionKind,
		Legacy:      legacy,
	}, true
}

// Suffix returns the canonical single-letter option side.
func (k OptionKind) Suffix() string {
	switch NormalizeOptionKind(string(k)) {
	case OptionCall:
		return "C"
	case OptionPut:
		return "P"
	default:
		return ""
	}
}

// NormalizeOptionKind converts exchange spellings such as C/P or Call/Put.
func NormalizeOptionKind(raw string) OptionKind {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "C", "CALL":
		return OptionCall
	case "P", "PUT":
		return OptionPut
	default:
		return ""
	}
}

// OptionInstrument identifies a single option contract.
//
// Use this struct to construct order parameters and chain queries instead of
// passing the venue-specific string ID; adapters provide FormatInstrument /
// ParseInstrument to translate to and from the wire format.
type OptionInstrument struct {
	Underlying string          `json:"underlying"` // Base symbol, e.g. "BTC"
	Expiry     time.Time       `json:"expiry"`     // UTC expiry timestamp
	Strike     decimal.Decimal `json:"strike"`     // Strike price in quote currency
	Kind       OptionKind      `json:"kind"`       // Call or Put
	Settlement string          `json:"settlement"` // Quote/settle asset, e.g. "USDT", "USDC", "USD"
}

// Greeks bundles the standard option risk sensitivities.
//
// Sign convention: Delta is +ve for long calls and short puts, -ve for long
// puts and short calls. Vega/Theta/Rho follow the per-1%/per-day/per-1%
// conventions used by Binance and OKX; venues that publish per-unit values
// are normalised by the adapter.
type Greeks struct {
	Delta decimal.Decimal `json:"delta"`
	Gamma decimal.Decimal `json:"gamma"`
	Vega  decimal.Decimal `json:"vega"`
	Theta decimal.Decimal `json:"theta"`
	Rho   decimal.Decimal `json:"rho"`
	IV    decimal.Decimal `json:"iv"` // implied volatility (decimal, e.g. 0.65 for 65%)
}

// OptionPositionData is the option-specific payload attached to a Position
// when InstrumentType == InstrumentTypeOption. It is nil for all other
// instrument types.
//
// Note that Position.Quantity is measured in contracts; the underlying
// quantity is Quantity * ContractSize.
type OptionPositionData struct {
	Instrument   OptionInstrument `json:"instrument"`
	Greeks       Greeks           `json:"greeks"`
	MarkIV       decimal.Decimal  `json:"mark_iv"`
	Premium      decimal.Decimal  `json:"premium"`       // current mark premium per contract
	ContractSize decimal.Decimal  `json:"contract_size"` // multiplier from contracts → underlying units
}

// OptionMark is the venue's authoritative mark quote for a single option
// instrument. It bundles the mark price, mark IV, the underlying reference
// price (used by the venue's mark IV calc), and the Greeks computed at mark.
type OptionMark struct {
	InstrumentID    string          `json:"instrument_id"`
	MarkPrice       decimal.Decimal `json:"mark_price"`
	MarkIV          decimal.Decimal `json:"mark_iv"`
	IndexPrice      decimal.Decimal `json:"index_price"`
	UnderlyingPrice decimal.Decimal `json:"underlying_price"`
	Greeks          Greeks          `json:"greeks"`
	Timestamp       int64           `json:"timestamp"` // milliseconds since epoch
}

// OptionChainOpts narrows a FetchOptionChain query. All fields are optional;
// the zero value means "no filter" for that dimension.
type OptionChainOpts struct {
	Expiry    *time.Time      // exact expiry filter
	MinStrike decimal.Decimal // inclusive lower bound; zero value = no lower bound
	MaxStrike decimal.Decimal // inclusive upper bound; zero value = no upper bound
	Kind      *OptionKind     // Call-only or Put-only; nil = both
}
