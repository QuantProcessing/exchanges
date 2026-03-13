package edgex

import (
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

// supportedQuoteCurrencies lists the quote currencies supported by EdgeX.
var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDC,
}

// Options configures an EdgeX adapter.
type Options struct {
	PrivateKey    string
	AccountID     string
	QuoteCurrency exchanges.QuoteCurrency // "USDC" (only supported, contracts use USD format internally)
	Logger        exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}

// quoteCurrency returns the validated quote currency, defaulting to USDC for DEX.
func (o Options) quoteCurrency() (exchanges.QuoteCurrency, error) {
	q := o.QuoteCurrency
	if q == "" {
		return exchanges.QuoteCurrencyUSDC, nil
	}
	for _, supported := range supportedQuoteCurrencies {
		if q == supported {
			return q, nil
		}
	}
	return "", fmt.Errorf("edgex: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}
