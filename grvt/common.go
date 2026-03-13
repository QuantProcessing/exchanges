//go:build grvt

package grvt

import (
	"strconv"

	"github.com/shopspring/decimal"
)

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseGrvtFloat(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func parseGrvtTimestamp(s string) int64 {
	ts, _ := strconv.ParseInt(s, 10, 64)
	return ts
}
