package edgex

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

func init() {
	venue.Register(Venue, func(ctx context.Context, cfg map[string]string) (venue.Adapter, error) {
		opts := Options{
			StarkPrivateKey:   firstConfig(cfg, "stark_private_key", "private_key"),
			ExchangeAccountID: firstConfig(cfg, "exchange_account_id", "edgex_account_id"),
			AccountID:         model.AccountID(cfg["account_id"]),
		}
		switch strings.ToLower(strings.TrimSpace(cfg["account_type"])) {
		case "", "perp", "futures":
			return NewPerpAdapter(ctx, opts)
		default:
			return nil, fmt.Errorf("%w: edgex account_type %q", model.ErrNotSupported, cfg["account_type"])
		}
	})
}

func firstConfig(cfg map[string]string, keys ...string) string {
	for _, key := range keys {
		if cfg[key] != "" {
			return cfg[key]
		}
	}
	return ""
}
