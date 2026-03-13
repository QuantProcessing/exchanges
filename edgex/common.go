//go:build edgex

package edgex

import (
	"strconv"

	"github.com/shopspring/decimal"
)

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func countStringDecimalPlaces(s string) int32 {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return 0
	}
	exp := d.Exponent()
	if exp >= 0 {
		return 0
	}
	return -exp
}

func parseEdgexFloat(s string) decimal.Decimal {
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
