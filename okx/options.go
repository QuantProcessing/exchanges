package okx

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures an OKX adapter.
type Options struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	Logger     exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
