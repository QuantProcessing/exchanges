package binance

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

func init() {
	venue.Register(Venue, func(ctx context.Context, cfg map[string]string) (venue.Adapter, error) {
		accountType := strings.ToLower(strings.TrimSpace(cfg["account_type"]))
		if accountType == "" || accountType == "spot" {
			return NewSpotAdapter(ctx, Options{
				APIKey:    cfg["api_key"],
				SecretKey: cfg["secret_key"],
				AccountID: model.AccountID(cfg["account_id"]),
			})
		}
		if accountType == "perp" || accountType == "futures" {
			return NewPerpAdapter(ctx, Options{
				APIKey:    cfg["api_key"],
				SecretKey: cfg["secret_key"],
				AccountID: model.AccountID(cfg["account_id"]),
			})
		}
		return nil, fmt.Errorf("%w: binance account_type %q", model.ErrNotSupported, cfg["account_type"])
	})
}
