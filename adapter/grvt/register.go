package grvt

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
			APIKey:       cfg["api_key"],
			SubAccountID: firstConfig(cfg, "subaccount_id", "sub_account_id"),
			PrivateKey:   cfg["private_key"],
			AccountID:    model.AccountID(cfg["account_id"]),
		}
		switch strings.ToLower(strings.TrimSpace(cfg["account_type"])) {
		case "", "perp", "futures":
			return NewPerpAdapter(ctx, opts)
		default:
			return nil, fmt.Errorf("%w: grvt account_type %q", model.ErrNotSupported, cfg["account_type"])
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
