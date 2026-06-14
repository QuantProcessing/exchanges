package hyperliquid

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

func init() {
	venue.Register(Venue, func(ctx context.Context, cfg map[string]string) (venue.Adapter, error) {
		opts := Options{PrivateKey: cfg["private_key"], Vault: cfg["vault"], AccountAddress: cfg["account_address"], AccountID: model.AccountID(cfg["account_id"])}
		switch strings.ToLower(strings.TrimSpace(cfg["account_type"])) {
		case "", "perp":
			return NewPerpAdapter(ctx, opts)
		case "spot":
			return NewSpotAdapter(ctx, opts)
		default:
			return nil, fmt.Errorf("%w: hyperliquid account_type %q", model.ErrNotSupported, cfg["account_type"])
		}
	})
}
