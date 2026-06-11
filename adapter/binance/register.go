package binance

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

func init() {
	exchanges.RegisterCapabilities("BINANCE", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
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
	exchanges.RegisterCapabilities("BINANCE", exchanges.MarketTypeSpot, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchTicker:         true,
		WatchTrades:         true,
		WatchKlines:         true,
		FetchOpenOrders:     true,
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("BINANCE", exchanges.MarketTypeOption, exchanges.Capabilities{
		FetchOptionContracts: true,
		PlaceOrder:           true,
		FetchOpenOrders:      true,
		FetchOrderHistory:    true,
	})
	exchanges.Register("BINANCE", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
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
		case exchanges.MarketTypeOption:
			return NewOptionAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("binance: unsupported market type %q", mt)
		}
	})
	venue.Register(model.VenueBinance, func(ctx context.Context, opts map[string]string) (venue.Adapter, error) {
		return NewVenueAdapter(ctx, VenueOptions{
			Options: Options{
				APIKey:        opts["api_key"],
				SecretKey:     opts["secret_key"],
				QuoteCurrency: exchanges.QuoteCurrency(opts["quote_currency"]),
			},
			AccountID: model.AccountID(opts["account_id"]),
		})
	})
}
