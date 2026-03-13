
package edgex

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures an EdgeX adapter.
type Options struct {
	PrivateKey string
	AccountID  string
	Logger     exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
