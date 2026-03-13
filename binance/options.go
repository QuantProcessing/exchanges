package binance

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures a Binance adapter.
type Options struct {
	APIKey    string
	SecretKey string
	Logger    exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
