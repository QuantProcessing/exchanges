package deribit

import (
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
)

type Options struct {
	APIKey    string
	SecretKey string
	Currency  string
	Logger    exchanges.Logger
}

func (o Options) logger() exchanges.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return exchanges.NopLogger
}

func (o Options) optionCurrencies() []string {
	if strings.TrimSpace(o.Currency) == "" {
		return []string{"any"}
	}
	parts := strings.Split(o.Currency, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.ToUpper(strings.TrimSpace(part)); value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return []string{"any"}
	}
	return out
}
