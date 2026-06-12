package livetest_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/adapter/aster"
	"github.com/QuantProcessing/exchanges/adapter/backpack"
	"github.com/QuantProcessing/exchanges/adapter/bitget"
	"github.com/QuantProcessing/exchanges/adapter/edgex"
	"github.com/QuantProcessing/exchanges/adapter/grvt"
	"github.com/QuantProcessing/exchanges/adapter/hyperliquid"
	"github.com/QuantProcessing/exchanges/adapter/lighter"
	"github.com/QuantProcessing/exchanges/adapter/nado"
	"github.com/QuantProcessing/exchanges/adapter/okx"
	"github.com/QuantProcessing/exchanges/adapter/standx"
	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/testsuite"
)

type liveOrderBookCase struct {
	name       string
	symbol     string
	requireEnv []string
	newAdapter func(context.Context) (exchanges.Exchange, error)
}

func TestPublicLiveWatchOrderBookAdapters(t *testing.T) {
	testenv.RequireFull(t)

	cases := []liveOrderBookCase{
		{
			name:   "aster/perp",
			symbol: envOrDefault("ASTER_PERP_TEST_SYMBOL", "BTC"),
			requireEnv: []string{
				"ASTER_API_KEY",
				"ASTER_SECRET_KEY",
			},
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return aster.NewAdapter(ctx, aster.Options{
					APIKey:    os.Getenv("ASTER_API_KEY"),
					SecretKey: os.Getenv("ASTER_SECRET_KEY"),
				})
			},
		},
		{
			name:   "aster/spot",
			symbol: envOrDefault("ASTER_SPOT_TEST_SYMBOL", "BTC"),
			requireEnv: []string{
				"ASTER_API_KEY",
				"ASTER_SECRET_KEY",
			},
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return aster.NewSpotAdapter(ctx, aster.Options{
					APIKey:    os.Getenv("ASTER_API_KEY"),
					SecretKey: os.Getenv("ASTER_SECRET_KEY"),
				})
			},
		},
		{
			name:   "backpack/perp",
			symbol: envOrDefault("BACKPACK_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return backpack.NewAdapter(ctx, backpack.Options{})
			},
		},
		{
			name:   "backpack/spot",
			symbol: envOrDefault("BACKPACK_SPOT_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return backpack.NewSpotAdapter(ctx, backpack.Options{})
			},
		},
		{
			name:   "bitget/perp",
			symbol: envOrDefault("BITGET_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return bitget.NewAdapter(ctx, bitget.Options{})
			},
		},
		{
			name:   "bitget/spot",
			symbol: envOrDefault("BITGET_SPOT_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return bitget.NewSpotAdapter(ctx, bitget.Options{})
			},
		},
		{
			name:   "edgex/perp",
			symbol: envOrDefault("EDGEX_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return edgex.NewAdapter(ctx, edgex.Options{})
			},
		},
		{
			name:   "grvt/perp",
			symbol: envOrDefault("GRVT_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return grvt.NewAdapter(ctx, grvt.Options{})
			},
		},
		{
			name:   "hyperliquid/perp",
			symbol: envOrDefault("HYPERLIQUID_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return hyperliquid.NewAdapter(ctx, hyperliquid.Options{})
			},
		},
		{
			name:   "hyperliquid/spot",
			symbol: envOrDefault("HYPERLIQUID_SPOT_TEST_SYMBOL", "HYPE"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return hyperliquid.NewSpotAdapter(ctx, hyperliquid.Options{})
			},
		},
		{
			name:   "lighter/perp",
			symbol: envOrDefault("LIGHTER_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return lighter.NewAdapter(ctx, lighter.Options{})
			},
		},
		{
			name:   "lighter/spot",
			symbol: envOrDefault("LIGHTER_SPOT_TEST_SYMBOL", "ETH"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return lighter.NewSpotAdapter(ctx, lighter.Options{})
			},
		},
		{
			name:   "nado/perp",
			symbol: envOrDefault("NADO_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return nado.NewAdapter(ctx, nado.Options{})
			},
		},
		{
			name:   "nado/spot",
			symbol: envOrDefault("NADO_SPOT_TEST_SYMBOL", "KBTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return nado.NewSpotAdapter(ctx, nado.Options{})
			},
		},
		{
			name:   "okx/perp",
			symbol: envOrDefault("OKX_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return okx.NewAdapter(ctx, okx.Options{})
			},
		},
		{
			name:   "okx/spot",
			symbol: envOrDefault("OKX_SPOT_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return okx.NewSpotAdapter(ctx, okx.Options{})
			},
		},
		{
			name:   "standx/perp",
			symbol: envOrDefault("STANDX_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return standx.NewAdapter(ctx, standx.Options{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.requireEnv) > 0 {
				testenv.RequireEnv(t, tc.requireEnv...)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			adp, err := tc.newAdapter(ctx)
			if err != nil {
				t.Fatalf("create adapter: %v", err)
			}
			defer func() {
				_ = adp.Close()
			}()

			symbol, err := resolveLiveOrderBookSymbol(t, adp, tc.symbol)
			if err != nil {
				t.Fatal(err)
			}

			testsuite.TestWatchOrderBook(t, adp, symbol)
		})
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func resolveLiveOrderBookSymbol(t *testing.T, adp exchanges.Exchange, preferred string) (string, error) {
	t.Helper()

	return resolveLiveOrderBookSymbolFromList(preferred, adp.ListSymbols())
}

func resolveLiveOrderBookSymbolFromList(preferred string, symbols []string) (string, error) {
	if len(symbols) == 0 {
		return "", fmt.Errorf("preferred symbol %q unavailable: exchange returned no symbols", preferred)
	}

	for _, symbol := range symbols {
		if strings.EqualFold(symbol, preferred) {
			return symbol, nil
		}
	}

	return "", fmt.Errorf("preferred symbol %q unavailable", preferred)
}
