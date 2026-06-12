package edgex

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("EDGEX", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchPositions:      true,
		WatchTicker:         true,
		WatchTrades:         true,
		WatchKlines:         true,
		FetchOpenOrders:     true,
		TradingAccountReady: true,
	})
	exchanges.Register("EDGEX", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			PrivateKey:    opts["private_key"],
			AccountID:     opts["account_id"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("edgex: unsupported market type %q (perp only)", mt)
		}
	})
}
