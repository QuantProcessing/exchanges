package aster

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("ASTER", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchPositions:      true,
		WatchTicker:         true,
		WatchTrades:         true,
		WatchKlines:         true,
		FetchOpenOrders:     true,
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("ASTER", exchanges.MarketTypeSpot, exchanges.Capabilities{
		PlaceOrder:          true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchTicker:         true,
		FetchOpenOrders:     true,
		TradingAccountReady: true,
	})
	exchanges.Register("ASTER", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
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
			return nil, fmt.Errorf("aster: unsupported market type %q", mt)
		}
	})
}
