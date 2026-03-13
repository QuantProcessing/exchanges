package aster

import (
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

// supportedQuoteCurrencies lists the quote currencies supported by Aster.
var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDT,
	exchanges.QuoteCurrencyUSDC,
}

// Options configures an Aster adapter.
type Options struct {
	APIKey        string
	SecretKey     string
	QuoteCurrency exchanges.QuoteCurrency // "USDC" (default for DEX) or "USDT"
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
	return "", fmt.Errorf("aster: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}
