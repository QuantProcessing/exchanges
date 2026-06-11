package bybit

import (
	"fmt"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
)

var defaultOptionUnderlyings = []string{"BTC", "ETH", "SOL"}

var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDT,
	exchanges.QuoteCurrencyUSDC,
}

type AccountMode string

const (
	AccountModeAuto    AccountMode = ""
	AccountModeUnified AccountMode = "UNIFIED"
	AccountModeClassic AccountMode = "CLASSIC"
)

type Options struct {
	APIKey            string
	SecretKey         string
	AccountMode       AccountMode
	QuoteCurrency     exchanges.QuoteCurrency
	OptionUnderlyings []string
	Logger            exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}

func (o Options) quoteCurrency() (exchanges.QuoteCurrency, error) {
	q := o.QuoteCurrency
	if q == "" {
		return exchanges.QuoteCurrencyUSDT, nil
	}
	for _, supported := range supportedQuoteCurrencies {
		if q == supported {
			return q, nil
		}
	}
	return "", fmt.Errorf("bybit: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}

func (o Options) accountMode() (AccountMode, error) {
	switch mode := AccountMode(strings.ToUpper(strings.TrimSpace(string(o.AccountMode)))); mode {
	case AccountModeAuto, AccountModeUnified, AccountModeClassic:
		return mode, nil
	default:
		return "", fmt.Errorf("bybit: unsupported account mode %q", o.AccountMode)
	}
}

func (o Options) optionUnderlyings() []string {
	values := normalizeOptionUnderlyings(o.OptionUnderlyings)
	if len(values) > 0 {
		return values
	}
	return append([]string(nil), defaultOptionUnderlyings...)
}

func parseOptionUnderlyings(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return normalizeOptionUnderlyings(strings.Split(raw, ","))
}

func normalizeOptionUnderlyings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		upper := strings.ToUpper(strings.TrimSpace(value))
		if upper == "" {
			continue
		}
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		out = append(out, upper)
	}
	return out
}
