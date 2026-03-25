package decibel

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.Register("DECIBEL", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:         opts["api_key"],
			PrivateKey:     opts["private_key"],
			SubaccountAddr: opts["subaccount_addr"],
			QuoteCurrency:  exchanges.QuoteCurrency(opts["quote_currency"]),
		}

		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("decibel: unsupported market type %q (perp only)", mt)
		}
	})
}
