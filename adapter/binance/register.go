package binance

import (
	"context"
	"fmt"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/model"
	"github.com/QuantProcessing/exchanges/venue"
)

func init() {
	venue.Register(model.VenueBinance, func(ctx context.Context, opts map[string]string) (venue.Adapter, error) {
		cfg := Options{
			APIKey:          opts["api_key"],
			SecretKey:       opts["secret_key"],
			QuoteCurrency:   exchanges.QuoteCurrency(opts["quote_currency"]),
			AccountID:       model.AccountID(opts["account_id"]),
			BaseURLHTTP:     opts["base_url_http"],
			BaseURLWS:       opts["base_url_ws"],
			BaseURLWSStream: opts["base_url_ws_stream"],
		}
		switch normalizeAccountType(opts["account_type"]) {
		case "spot":
			return NewSpotAdapter(ctx, cfg)
		case "perp":
			return NewPerpAdapter(ctx, cfg)
		default:
			return nil, fmt.Errorf("binance: unsupported account_type %q", opts["account_type"])
		}
	})
}

func normalizeAccountType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "spot", "cash":
		return "spot"
	case "perp", "future", "futures", "usdt_futures", "usdt-futures", "margin":
		return "perp"
	default:
		return raw
	}
}
