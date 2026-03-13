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

func FormatSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	if !strings.HasSuffix(s, "-USDT-SWAP") {
		s += "-USDT-SWAP"
	}
	return s
}

func ExtractSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	s = strings.TrimSuffix(s, "-USDT-SWAP")
	s = strings.TrimSuffix(s, "-USDT")
	return s
}
