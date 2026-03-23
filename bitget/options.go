package bitget

import (
	"fmt"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
)

var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDT,
	exchanges.QuoteCurrencyUSDC,
}

type Options struct {
	APIKey        string
	SecretKey     string
	Passphrase    string
	AccountMode   string
	QuoteCurrency exchanges.QuoteCurrency
	Logger        exchanges.Logger
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
	return "", fmt.Errorf("bitget: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}

func (o Options) accountMode() (string, error) {
	switch mode := strings.ToLower(strings.TrimSpace(o.AccountMode)); mode {
	case "", accountModeAuto:
		return accountModeAuto, nil
	case accountModeUTA, accountModeClassic:
		return mode, nil
	default:
		return "", fmt.Errorf("bitget: unsupported account mode %q, supported: auto, uta, classic", o.AccountMode)
	}
}
