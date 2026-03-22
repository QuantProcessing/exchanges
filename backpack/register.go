package backpack

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.Register("BACKPACK", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:        opts["api_key"],
			PrivateKey:    opts["private_key"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("backpack: unsupported market type %q", mt)
		}
	})
}
