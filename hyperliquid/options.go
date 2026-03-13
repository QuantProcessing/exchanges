package hyperliquid

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures a Hyperliquid adapter.
type Options struct {
	PrivateKey  string
	AccountAddr string
	Logger      exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
