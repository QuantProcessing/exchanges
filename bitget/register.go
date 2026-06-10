package bitget

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("BITGET", exchanges.MarketTypePerp, exchanges.Capabilities{
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
	exchanges.RegisterCapabilities("BITGET", exchanges.MarketTypeSpot, exchanges.Capabilities{
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
	exchanges.Register("BITGET", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:        opts["api_key"],
			SecretKey:     opts["secret_key"],
			Passphrase:    opts["passphrase"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}

		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("bitget: unsupported market type %q", mt)
		}
	})
}
