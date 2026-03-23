package backpack

import (
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDC,
}

type Options struct {
	APIKey        string
	PrivateKey    string
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
		return exchanges.QuoteCurrencyUSDC, nil
	}
	for _, supported := range supportedQuoteCurrencies {
		if q == supported {
			return q, nil
		}
	}
	return "", fmt.Errorf("backpack: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}

func (o Options) validateCredentials() error {
	if o.APIKey == "" && o.PrivateKey == "" {
		return nil
	}
	if o.APIKey == "" || o.PrivateKey == "" {
		return exchanges.NewExchangeError("BACKPACK", "", "api_key and private_key must be set together", exchanges.ErrAuthFailed)
	}
	return nil
}
