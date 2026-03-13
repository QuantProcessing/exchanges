package lighter

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures a Lighter adapter.
type Options struct {
	PrivateKey   string
	AccountIndex string
	KeyIndex     string
	RoToken      string
	Logger       exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
