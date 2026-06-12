package hyperliquid

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("HYPERLIQUID", exchanges.MarketTypePerp, exchanges.Capabilities{
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
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("HYPERLIQUID", exchanges.MarketTypeSpot, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchTicker:         true,
		WatchTrades:         true,
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.Register("HYPERLIQUID", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			PrivateKey:    opts["private_key"],
			AccountAddr:   opts["account_addr"],
			VaultAddress:  opts["vault_address"],
			QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("hyperliquid: unsupported market type %q", mt)
		}
	})
}
