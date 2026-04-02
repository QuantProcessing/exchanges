package exchanges_test

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type stubExchange struct {
	placed *exchanges.OrderParams
}

func (s *stubExchange) GetExchange() string { return "stub" }

func (s *stubExchange) GetMarketType() exchanges.MarketType { return exchanges.MarketTypeSpot }

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

func (s *stubExchange) PlaceOrder(_ context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	copy := *params
	s.placed = &copy
	return &exchanges.Order{}, nil
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

func (s *stubExchange) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, nil
}

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

func TestPlaceMarketOrderAcceptsOptionalPrice(t *testing.T) {
	t.Parallel()

	adp := &stubExchange{}

	_, err := exchanges.PlaceMarketOrder(
		context.Background(),
		adp,
		"BTC",
		exchanges.OrderSideBuy,
		decimal.RequireFromString("1.25"),
		decimal.RequireFromString("101.5"),
	)
	require.NoError(t, err)
	require.NotNil(t, adp.placed)
	require.Equal(t, exchanges.OrderTypeMarket, adp.placed.Type)
	require.Equal(t, decimal.RequireFromString("101.5"), adp.placed.Price)
}

func TestPlaceMarketOrderWithSlippageAcceptsOptionalPrice(t *testing.T) {
	t.Parallel()

	adp := &stubExchange{}

	_, err := exchanges.PlaceMarketOrderWithSlippage(
		context.Background(),
		adp,
		"BTC",
		exchanges.OrderSideSell,
		decimal.RequireFromString("2"),
		decimal.RequireFromString("0.01"),
		decimal.RequireFromString("99.5"),
	)
	require.NoError(t, err)
	require.NotNil(t, adp.placed)
	require.Equal(t, exchanges.OrderTypeMarket, adp.placed.Type)
	require.Equal(t, decimal.RequireFromString("0.01"), adp.placed.Slippage)
	require.Equal(t, decimal.RequireFromString("99.5"), adp.placed.Price)
}

func TestApplySlippageUsesProvidedMarketReferencePrice(t *testing.T) {
	t.Parallel()

	base := exchanges.NewBaseAdapter("stub", exchanges.MarketTypeSpot, exchanges.NopLogger)
	base.SetSymbolDetails(map[string]*exchanges.SymbolDetails{
		"BTC": {
			Symbol:         "BTC",
			PricePrecision: 2,
		},
	})

	params := &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("1"),
		Price:    decimal.RequireFromString("100"),
		Slippage: decimal.RequireFromString("0.01"),
	}

	err := base.ApplySlippage(context.Background(), params, func(context.Context, string) (*exchanges.Ticker, error) {
		t.Fatal("fetchTicker should not be called when market price is provided")
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, exchanges.OrderTypeLimit, params.Type)
	require.Equal(t, exchanges.TimeInForceIOC, params.TimeInForce)
	require.Equal(t, "101.00", params.Price.StringFixed(2))
}
