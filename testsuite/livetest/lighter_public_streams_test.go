package livetest_test

import (
	"context"
	"testing"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/internal/testenv"
	"github.com/QuantProcessing/exchanges/lighter"
	"github.com/stretchr/testify/require"
)

type lighterLiveStreamCase struct {
	name       string
	symbol     string
	newAdapter func(context.Context) (exchanges.Exchange, error)
}

func TestPublicLiveWatchTickerLighter(t *testing.T) {
	testenv.RequireFull(t)

	cases := []lighterLiveStreamCase{
		{
			name:   "perp",
			symbol: envOrDefault("LIGHTER_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return lighter.NewAdapter(ctx, lighter.Options{})
			},
		},
		{
			name:   "spot",
			symbol: envOrDefault("LIGHTER_SPOT_TEST_SYMBOL", "ETH"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return lighter.NewSpotAdapter(ctx, lighter.Options{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			adp, err := tc.newAdapter(ctx)
			require.NoError(t, err)
			defer func() { _ = adp.Close() }()

			symbol, err := resolveLiveOrderBookSymbol(t, adp, tc.symbol)
			require.NoError(t, err)

			updates := make(chan *exchanges.Ticker, 16)
			err = adp.WatchTicker(ctx, symbol, func(tk *exchanges.Ticker) {
				select {
				case updates <- tk:
				default:
				}
			})
			require.NoError(t, err)
			defer func() {
				_ = adp.StopWatchTicker(context.Background(), symbol)
			}()

			var got *exchanges.Ticker
			require.Eventually(t, func() bool {
				select {
				case tk := <-updates:
					if !isValidLighterTicker(tc.name, symbol, tk) {
						return false
					}
					got = tk
					return true
				default:
					return false
				}
			}, 20*time.Second, 200*time.Millisecond)

			require.NotNil(t, got)
			t.Logf("Ticker update received: symbol=%s last=%s bid=%s ask=%s ts=%d",
				got.Symbol, got.LastPrice, got.Bid, got.Ask, got.Timestamp)
		})
	}
}

func TestPublicLiveWatchTradesLighter(t *testing.T) {
	testenv.RequireFull(t)

	cases := []lighterLiveStreamCase{
		{
			name:   "perp",
			symbol: envOrDefault("LIGHTER_PERP_TEST_SYMBOL", "BTC"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return lighter.NewAdapter(ctx, lighter.Options{})
			},
		},
		{
			name:   "spot",
			symbol: envOrDefault("LIGHTER_SPOT_TEST_SYMBOL", "ETH"),
			newAdapter: func(ctx context.Context) (exchanges.Exchange, error) {
				return lighter.NewSpotAdapter(ctx, lighter.Options{})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			adp, err := tc.newAdapter(ctx)
			require.NoError(t, err)
			defer func() { _ = adp.Close() }()

			symbol, err := resolveLiveOrderBookSymbol(t, adp, tc.symbol)
			require.NoError(t, err)

			updates := make(chan *exchanges.Trade, 32)
			err = adp.WatchTrades(ctx, symbol, func(tr *exchanges.Trade) {
				select {
				case updates <- tr:
				default:
				}
			})
			require.NoError(t, err)
			defer func() {
				_ = adp.StopWatchTrades(context.Background(), symbol)
			}()

			var got *exchanges.Trade
			require.Eventually(t, func() bool {
				select {
				case tr := <-updates:
					if tr == nil || tr.Symbol != symbol || !tr.Price.IsPositive() || !tr.Quantity.IsPositive() || tr.Timestamp <= 0 {
						return false
					}
					got = tr
					return true
				default:
					return false
				}
			}, 20*time.Second, 200*time.Millisecond)

			require.NotNil(t, got)
			t.Logf("Trade update received: symbol=%s id=%s price=%s qty=%s side=%s ts=%d",
				got.Symbol, got.ID, got.Price, got.Quantity, got.Side, got.Timestamp)
		})
	}
}

func isValidLighterTicker(kind, symbol string, tk *exchanges.Ticker) bool {
	if tk == nil || tk.Symbol != symbol || tk.Timestamp <= 0 {
		return false
	}

	switch kind {
	case "perp":
		return tk.Bid.IsPositive() && tk.Ask.IsPositive() && tk.LastPrice.IsPositive()
	case "spot":
		return tk.LastPrice.IsPositive() || tk.MidPrice.IsPositive()
	default:
		return tk.LastPrice.IsPositive() || (tk.Bid.IsPositive() && tk.Ask.IsPositive())
	}
}
