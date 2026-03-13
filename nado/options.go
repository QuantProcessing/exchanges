package nado

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures a Nado adapter.
type Options struct {
	PrivateKey     string
	SubAccountName string
	Logger         exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
