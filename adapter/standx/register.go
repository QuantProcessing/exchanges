package standx

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("STANDX", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchPositions:      true,
		WatchTicker:         true,
		WatchTrades:         true,
		FetchOpenOrders:     true,
		TradingAccountReady: true,
	})
	exchanges.Register("STANDX", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			PrivateKey:    opts["private_key"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("standx: unsupported market type %q (perp only)", mt)
		}
	})
}
