package backpack

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("BACKPACK", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchPositions:      true,
		FetchOpenOrders:     true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("BACKPACK", exchanges.MarketTypeSpot, exchanges.Capabilities{
		PlaceOrder:          true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		FetchOpenOrders:     true,
		TradingAccountReady: true,
	})
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
