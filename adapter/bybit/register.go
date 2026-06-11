package bybit

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("BYBIT", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchPositions:      true,
		FetchOpenOrders:     true,
		FetchOrderHistory:   true,
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("BYBIT", exchanges.MarketTypeSpot, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		FetchOpenOrders:     true,
		FetchOrderHistory:   true,
		TradingAccountReady: true,
	})
	exchanges.Register("BYBIT", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:        opts["api_key"],
			SecretKey:     opts["secret_key"],
			AccountMode:   AccountMode(opts["account_mode"]),
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}

		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		case exchanges.MarketTypeOption:
			return NewOptionAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("bybit: unsupported market type %q", mt)
		}
	})
}
