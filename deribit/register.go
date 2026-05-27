package deribit

import (
	"context"
	"fmt"

	exchanges "github.com/QuantProcessing/exchanges"
)

const exchangeName = "DERIBIT"

func init() {
	exchanges.RegisterCapabilities(exchangeName, exchanges.MarketTypePerp, exchanges.Capabilities{})
	exchanges.RegisterCapabilities(exchangeName, exchanges.MarketTypeOption, exchanges.Capabilities{
		FetchOptionContracts: true,
		PlaceOrder:           true,
		FetchOpenOrders:      true,
		FetchOrderHistory:    true,
	})
	exchanges.Register(exchangeName, func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		o := Options{
			APIKey:    opts["api_key"],
			SecretKey: opts["secret_key"],
			Currency:  opts["currency"],
		}
		switch mt {
		case exchanges.MarketTypePerp:
			return NewAdapter(ctx, o)
		case exchanges.MarketTypeOption:
			return NewOptionAdapter(ctx, o)
		default:
			return nil, fmt.Errorf("deribit: unsupported market type %q", mt)
		}
	})
}
