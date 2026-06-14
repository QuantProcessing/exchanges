package bybit

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

func init() {
	venue.Register(Venue, func(ctx context.Context, cfg map[string]string) (venue.Adapter, error) {
		opts := Options{APIKey: cfg["api_key"], SecretKey: cfg["secret_key"], AccountID: model.AccountID(cfg["account_id"])}
		switch strings.ToLower(strings.TrimSpace(cfg["account_type"])) {
		case "", "spot":
			return NewSpotAdapter(ctx, opts)
		case "linear", "perp":
			return NewLinearAdapter(ctx, opts)
		default:
			return nil, fmt.Errorf("%w: bybit account_type %q", model.ErrNotSupported, cfg["account_type"])
		}
	})
}
