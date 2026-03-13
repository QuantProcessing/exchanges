//go:build grvt

package grvt

import exchanges "github.com/QuantProcessing/exchanges"

// Options configures a GRVT adapter.
type Options struct {
	APIKey       string
	SubAccountID string
	PrivateKey   string
	Logger       exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}
