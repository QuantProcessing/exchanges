package exchanges

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// ID Generation
// ============================================================================

// idCounter is atomically incremented on each GenerateID call.
// Initialized to UnixMilli so IDs fit within Lighter's max client_order_index of (1<<48)-1.
var idCounter = atomic.Int64{}

func init() {
	idCounter.Store(time.Now().UnixMilli())
}

// GenerateID returns a unique numeric string safe for concurrent use.
// Values stay within [1, 2^48-1] range required by Lighter; other exchanges accept any numeric string.
func GenerateID() string {
	return strconv.FormatInt(idCounter.Add(1), 10)
}

// ============================================================================
// Precision Utilities (decimal)
// ============================================================================

// RoundToPrecision rounds a decimal value to the given number of decimal places.
func RoundToPrecision(value decimal.Decimal, precision int32) decimal.Decimal {
	return value.Round(precision)
}

// FloorToPrecision truncates (floors) a decimal value to the given number of decimal places.
func FloorToPrecision(value decimal.Decimal, precision int32) decimal.Decimal {
	return value.Truncate(precision)
}

// CountDecimalPlaces returns how many significant decimal places a string representation has.
// Trailing zeros are not counted: "0.00010" → 4, "0.10" → 1.
func CountDecimalPlaces(s string) int32 {
	// Strip trailing zeros to get the real precision
	// e.g. "0.00010" → "0.0001" → 4 decimal places
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".") // handle "10." edge case
	idx := strings.Index(s, ".")
	if idx == -1 {
		return 0
	}
	return int32(len(s) - idx - 1)
}

// RoundToSignificantFigures rounds a decimal to n significant figures.
func RoundToSignificantFigures(value decimal.Decimal, sigFigs int32) decimal.Decimal {
	if value.IsZero() {
		return decimal.Zero
	}
	// Calculate the number of digits before decimal point
	abs := value.Abs()
	// Use string representation to figure out significant digits
	str := abs.String()
	// Find first non-zero digit position
	dotPos := -1
	firstNonZero := -1
	for i, ch := range str {
		if ch == '.' {
			dotPos = i
			continue
		}
		if ch != '0' && firstNonZero == -1 {
			firstNonZero = i
		}
	}
	if firstNonZero == -1 {
		return decimal.Zero
	}
	// Calculate precision needed
	var precision int32
	if dotPos == -1 || firstNonZero < dotPos {
		// Integer part has significant digits
		intDigits := int32(dotPos - firstNonZero)
		if dotPos == -1 {
			intDigits = int32(len(str) - firstNonZero)
		}
		if intDigits >= sigFigs {
			precision = 0
		} else {
			precision = sigFigs - intDigits
		}
	} else {
		// All significant digits are after decimal point
		zerosAfterDot := int32(firstNonZero - dotPos - 1)
		precision = zerosAfterDot + sigFigs
	}
	return value.Round(precision)
}

// FormatDecimal formats a decimal to string, stripping trailing zeros.
func FormatDecimal(d decimal.Decimal) string {
	return d.String()
}

// ============================================================================
// Order Validation
// ============================================================================

// ValidateAndFormatParams validates and formats order parameters using cached symbol details.
// It rounds price and truncates quantity to the correct precision.
func ValidateAndFormatParams(params *OrderParams, details *SymbolDetails) error {
	if details == nil {
		return nil
	}
	// Format precision
	params.Price = RoundToPrecision(params.Price, details.PricePrecision)
	params.Quantity = FloorToPrecision(params.Quantity, details.QuantityPrecision)

	// Validate minimum quantity
	if !details.MinQuantity.IsZero() && params.Quantity.LessThan(details.MinQuantity) {
		return fmt.Errorf("%w: quantity %s < min %s", ErrMinQuantity, params.Quantity, details.MinQuantity)
	}

	// Validate minimum notional
	if !details.MinNotional.IsZero() && !params.Price.IsZero() {
		notional := params.Price.Mul(params.Quantity)
		if notional.LessThan(details.MinNotional) {
			return fmt.Errorf("%w: notional %s < min %s", ErrMinNotional, notional, details.MinNotional)
		}
	}

	return nil
}

// ============================================================================
// Order Status Utilities
// ============================================================================

// DerivePartialFillStatus adjusts order status to PARTIALLY_FILLED when the
// exchange reports status as NEW but FilledQuantity > 0.
// Exchanges like Binance, OKX, and Lighter natively report PARTIALLY_FILLED;
// this helper is for exchanges that don't (Hyperliquid, Nado, StandX, GRVT, EdgeX).
func DerivePartialFillStatus(order *Order) {
	if order.Status == OrderStatusNew && order.FilledQuantity.IsPositive() {
		order.Status = OrderStatusPartiallyFilled
	}
}
