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

type AccountMode string

const (
	AccountModeClassic AccountMode = ""
	AccountModeUTA     AccountMode = "UTA"
)

type Options struct {
	APIKey        string
	SecretKey     string
	Passphrase    string
	AccountMode   AccountMode
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

func (o Options) accountMode() (AccountMode, error) {
	switch mode := AccountMode(strings.ToUpper(strings.TrimSpace(string(o.AccountMode)))); mode {
	case AccountModeClassic, AccountModeUTA:
		return mode, nil
	default:
		return "", fmt.Errorf("bitget: unsupported account mode %q", o.AccountMode)
	}
}
