package config_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/config"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

var registerConfigTestExchange sync.Once

func registerStubExchange() {
	exchanges.Register("CONFIG_TEST", func(ctx context.Context, mt exchanges.MarketType, opts map[string]string) (exchanges.Exchange, error) {
		copied := make(map[string]string, len(opts))
		for key, value := range opts {
			copied[key] = value
		}
		return &stubExchange{
			name:       "CONFIG_TEST",
			marketType: mt,
			options:    copied,
		}, nil
	})
}

type stubExchange struct {
	name       string
	marketType exchanges.MarketType
	options    map[string]string
}

func (s *stubExchange) GetExchange() string { return s.name }

func (s *stubExchange) GetMarketType() exchanges.MarketType { return s.marketType }

func (s *stubExchange) Close() error { return nil }

func (s *stubExchange) FormatSymbol(symbol string) string { return symbol }

func (s *stubExchange) ExtractSymbol(symbol string) string { return symbol }

func (s *stubExchange) ListSymbols() []string { return nil }

func (s *stubExchange) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, nil
}

func (s *stubExchange) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, nil
}

func (s *stubExchange) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, nil
}

func (s *stubExchange) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, nil
}

func (s *stubExchange) PlaceOrder(context.Context, *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) CancelOrder(context.Context, string, string) error { return nil }

func (s *stubExchange) CancelAllOrders(context.Context, string) error { return nil }

func (s *stubExchange) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, nil
}

func (s *stubExchange) FetchAccount(context.Context) (*exchanges.Account, error) { return nil, nil }

func (s *stubExchange) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (s *stubExchange) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, nil
}

func (s *stubExchange) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, nil
}

func (s *stubExchange) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return nil
}

func (s *stubExchange) GetLocalOrderBook(string, int) *exchanges.OrderBook { return nil }

func (s *stubExchange) StopWatchOrderBook(context.Context, string) error { return nil }

func (s *stubExchange) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error { return nil }

func (s *stubExchange) WatchFills(context.Context, exchanges.FillCallback) error { return nil }

func (s *stubExchange) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return nil
}

func (s *stubExchange) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return nil
}

func (s *stubExchange) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return nil
}

func (s *stubExchange) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return nil
}

func (s *stubExchange) StopWatchOrders(context.Context) error { return nil }

func (s *stubExchange) StopWatchFills(context.Context) error { return nil }

func (s *stubExchange) StopWatchPositions(context.Context) error { return nil }

func (s *stubExchange) StopWatchTicker(context.Context, string) error { return nil }

func (s *stubExchange) StopWatchTrades(context.Context, string) error { return nil }

func (s *stubExchange) StopWatchKlines(context.Context, string, exchanges.Interval) error { return nil }

func TestLoadManagerYAMLBuildsAdaptersAndExpandsEnv(t *testing.T) {
	registerConfigTestExchange.Do(registerStubExchange)

	t.Setenv("CONFIG_TEST_API_KEY", "expanded-secret")

	configPath := filepath.Join(t.TempDir(), "exchanges.yaml")
	err := os.WriteFile(configPath, []byte(`
exchanges:
  - name: CONFIG_TEST
    market_type: perp
    options:
      api_key: ${CONFIG_TEST_API_KEY}
  - name: CONFIG_TEST
    market_type: spot
    options:
      quote_currency: USDT
`), 0o600)
	require.NoError(t, err)

	manager, err := config.LoadManager(context.Background(), configPath)
	require.NoError(t, err)

	perp, err := manager.GetAdapter("CONFIG_TEST/perp")
	require.NoError(t, err)
	perpStub, ok := perp.(*stubExchange)
	require.True(t, ok)
	require.Equal(t, "expanded-secret", perpStub.options["api_key"])
	require.Equal(t, exchanges.MarketTypePerp, perpStub.marketType)

	spot, err := manager.GetAdapter("CONFIG_TEST/spot")
	require.NoError(t, err)
	spotStub, ok := spot.(*stubExchange)
	require.True(t, ok)
	require.Equal(t, "USDT", spotStub.options["quote_currency"])
	require.Equal(t, exchanges.MarketTypeSpot, spotStub.marketType)
}

func TestLoadManagerJSONUsesExplicitAlias(t *testing.T) {
	t.Parallel()
	registerConfigTestExchange.Do(registerStubExchange)

	configPath := filepath.Join(t.TempDir(), "exchanges.json")
	err := os.WriteFile(configPath, []byte(`{
  "exchanges": [
    {
      "name": "CONFIG_TEST",
      "alias": "primary",
      "market_type": "perp",
      "options": {
        "quote_currency": "USDC"
      }
    }
  ]
}`), 0o600)
	require.NoError(t, err)

	manager, err := config.LoadManager(context.Background(), configPath)
	require.NoError(t, err)

	adp, err := manager.GetAdapter("primary")
	require.NoError(t, err)
	stub, ok := adp.(*stubExchange)
	require.True(t, ok)
	require.Equal(t, "USDC", stub.options["quote_currency"])
	require.Equal(t, exchanges.MarketTypePerp, stub.marketType)
}

func TestBuildManagerRejectsDuplicateAliases(t *testing.T) {
	t.Parallel()
	registerConfigTestExchange.Do(registerStubExchange)

	_, err := config.BuildManager(context.Background(), config.Config{
		Exchanges: []config.ExchangeConfig{
			{
				Name:       "CONFIG_TEST",
				Alias:      "duplicate",
				MarketType: exchanges.MarketTypePerp,
			},
			{
				Name:       "CONFIG_TEST",
				Alias:      "duplicate",
				MarketType: exchanges.MarketTypeSpot,
			},
		},
	})
	require.ErrorContains(t, err, "duplicate adapter alias")
}
