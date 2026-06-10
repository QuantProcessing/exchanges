package okx

import (
	"fmt"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
)

var defaultOptionFamilies = []string{"BTC-USD", "ETH-USD"}

// supportedQuoteCurrencies lists the quote currencies supported by OKX.
var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDT,
	exchanges.QuoteCurrencyUSDC,
}

// Options configures an OKX adapter.
type Options struct {
	APIKey         string
	SecretKey      string
	Passphrase     string
	QuoteCurrency  exchanges.QuoteCurrency // "USDT" (default for CEX) or "USDC"
	OptionFamilies []string
	Logger         exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}

// quoteCurrency returns the validated quote currency, defaulting to USDT for CEX.
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
	return "", fmt.Errorf("okx: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}

func (o Options) hasFullCredentials() bool {
	return o.APIKey != "" && o.SecretKey != "" && o.Passphrase != ""
}

func (o Options) validateCredentials() error {
	if o.APIKey == "" && o.SecretKey == "" && o.Passphrase == "" {
		return nil
	}
	if !o.hasFullCredentials() {
		return exchanges.NewExchangeError("OKX", "", "api_key, secret_key, and passphrase must be set together", exchanges.ErrAuthFailed)
	}
	return nil
}

func (o Options) optionFamilies() []string {
	values := normalizeOptionFamilies(o.OptionFamilies)
	if len(values) > 0 {
		return values
	}
	return append([]string(nil), defaultOptionFamilies...)
}

func parseOptionFamilies(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return normalizeOptionFamilies(strings.Split(raw, ","))
}

func normalizeOptionFamilies(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		upper := strings.ToUpper(strings.TrimSpace(value))
		if upper == "" {
			continue
		}
		if !strings.Contains(upper, "-") {
			upper += "-USD"
		}
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		out = append(out, upper)
	}
	return out
}
