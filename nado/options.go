package nado

import (
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

// supportedQuoteCurrencies lists the quote currencies supported by Nado.
var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDT,
}

// Options configures a Nado adapter.
type Options struct {
	PrivateKey     string
	SubAccountName string
	QuoteCurrency  exchanges.QuoteCurrency // "USDT" (only supported, maps to USDT0 internally)
	Logger         exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}

// quoteCurrency returns the validated quote currency, defaulting to USDT.
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
	return "", fmt.Errorf("nado: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}
