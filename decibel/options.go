package decibel

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures the Decibel perp adapter.
type Options struct {
	APIKey         string
	PrivateKey     string
	SubaccountAddr string
	QuoteCurrency  exchanges.QuoteCurrency
	Logger         exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}

func (o Options) quoteCurrency() exchanges.QuoteCurrency {
	return o.QuoteCurrency
}

func (o Options) validateCredentials() error {
	if o.APIKey == "" && o.PrivateKey == "" && o.SubaccountAddr == "" {
		return exchanges.NewExchangeError("DECIBEL", "", "api_key, private_key, and subaccount_addr are required", exchanges.ErrAuthFailed)
	}
	if o.APIKey == "" || o.PrivateKey == "" || o.SubaccountAddr == "" {
		return exchanges.NewExchangeError("DECIBEL", "", "api_key, private_key, and subaccount_addr must be set together", exchanges.ErrAuthFailed)
	}
	return nil
}
