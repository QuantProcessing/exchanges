package nado

import (
	"strconv"

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

// parseX18 parses an x18 (18 decimal places) number string to decimal.Decimal
func parseX18(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	// x18 numbers are scaled by 10^18
	scale := decimal.New(1, 18) // 10^18
	return d.Div(scale)
}
