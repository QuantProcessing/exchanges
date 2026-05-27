package okx

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

func init() {
	exchanges.RegisterCapabilities("OKX", exchanges.MarketTypePerp, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchPositions:      true,
		WatchTicker:         true,
		FetchOpenOrders:     true,
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("OKX", exchanges.MarketTypeSpot, exchanges.Capabilities{
		PlaceOrder:          true,
		PlaceOrderWS:        true,
		CancelOrderWS:       true,
		WatchOrderBook:      true,
		WatchOrders:         true,
		WatchFills:          true,
		WatchTicker:         true,
		FetchOpenOrders:     true,
		ModifyOrder:         true,
		TradingAccountReady: true,
	})
	exchanges.RegisterCapabilities("OKX", exchanges.MarketTypeOption, exchanges.Capabilities{
		FetchOptionContracts: true,
		PlaceOrder:           true,
		FetchOpenOrders:      true,
	})
	exchanges.Register("OKX", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		optionFamilies := parseOptionFamilies(opts["option_families"])
		if len(optionFamilies) == 0 {
			optionFamilies = parseOptionFamilies(opts["option_underlyings"])
		}
		o := Options{
			APIKey:         opts["api_key"],
			SecretKey:      opts["secret_key"],
			Passphrase:     opts["passphrase"],
			QuoteCurrency:  exchanges.QuoteCurrency(opts["quote_currency"]),
			OptionFamilies: optionFamilies,
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeSpot:
			return NewSpotAdapter(ctx, o)
		case exchanges.MarketTypeOption:
			return NewOptionAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("okx: unsupported market type %q", mt)
		}
	})
}
