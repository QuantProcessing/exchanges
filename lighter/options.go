package lighter

import (
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

// supportedQuoteCurrencies lists the quote currencies supported by Lighter.
var supportedQuoteCurrencies = []exchanges.QuoteCurrency{
	exchanges.QuoteCurrencyUSDC,
}

// Options configures a Lighter adapter.
type Options struct {
	PrivateKey    string
	AccountIndex  string
	KeyIndex      string
	RoToken       string
	QuoteCurrency exchanges.QuoteCurrency // "USDC" (only supported)
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
	return "", fmt.Errorf("lighter: unsupported quote currency %q, supported: %v", q, supportedQuoteCurrencies)
}

func (o Options) validateCredentials() error {
	if o.PrivateKey == "" && o.AccountIndex == "" && o.KeyIndex == "" && o.RoToken == "" {
		return nil
	}
	if o.PrivateKey != "" && o.AccountIndex == "" {
		return exchanges.NewExchangeError("LIGHTER", "", "account_index is required when private_key is set", exchanges.ErrAuthFailed)
	}
	if o.KeyIndex != "" && o.PrivateKey == "" {
		return exchanges.NewExchangeError("LIGHTER", "", "key_index requires private_key", exchanges.ErrAuthFailed)
	}
	if o.RoToken != "" && o.AccountIndex == "" {
		return exchanges.NewExchangeError("LIGHTER", "", "account_index is required when ro_token is set", exchanges.ErrAuthFailed)
	}
	return nil
}
