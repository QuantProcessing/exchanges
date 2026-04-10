package bybit

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.Register("BYBIT", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:        opts["api_key"],
			SecretKey:     opts["secret_key"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}

		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("bybit: unsupported market type %q", mt)
		}
	})
}
