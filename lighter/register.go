package lighter

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.Register("LIGHTER", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			PrivateKey:    opts["private_key"],
			AccountIndex:  opts["account_index"],
			KeyIndex:      opts["key_index"],
			RoToken:       opts["ro_token"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("lighter: unsupported market type %q", mt)
		}
	})
}
