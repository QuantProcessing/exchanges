package bybit

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type stubExchange struct{}

func (s stubExchange) GetExchange() string                 { return exchangeName }
func (s stubExchange) GetMarketType() exchanges.MarketType { return exchanges.MarketTypeSpot }
func (s stubExchange) Close() error                        { return nil }
func (s stubExchange) FormatSymbol(symbol string) string   { return symbol + "USDT" }
func (s stubExchange) ExtractSymbol(symbol string) string  { return symbol }
func (s stubExchange) ListSymbols() []string               { return nil }
func (s stubExchange) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) PlaceOrder(context.Context, *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) CancelOrder(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) CancelOrderWS(context.Context, string, string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) CancelAllOrders(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, exchanges.ErrNotSupported
}
func (s stubExchange) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) GetLocalOrderBook(string, int) *exchanges.OrderBook { return nil }
func (s stubExchange) StopWatchOrderBook(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchFills(context.Context, exchanges.FillCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) StopWatchOrders(context.Context) error    { return exchanges.ErrNotSupported }
func (s stubExchange) StopWatchFills(context.Context) error     { return exchanges.ErrNotSupported }
func (s stubExchange) StopWatchPositions(context.Context) error { return exchanges.ErrNotSupported }
func (s stubExchange) StopWatchTicker(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) StopWatchTrades(context.Context, string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func TestToPlaceOrderRequestSetsSpotMarketBuyToBaseUnit(t *testing.T) {
	req, err := toPlaceOrderRequest(context.Background(), stubExchange{}, categorySpot, &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("2"),
	})
	require.NoError(t, err)
	require.Equal(t, "BTCUSDT", req.Symbol)
	require.Equal(t, "baseCoin", req.MarketUnit)
	require.Equal(t, "2", req.Qty)
	require.NotEmpty(t, req.OrderLinkID)
}

func TestToBybitTimeInForceMapsPostOnly(t *testing.T) {
	value := toBybitTimeInForce(&exchanges.OrderParams{Type: exchanges.OrderTypePostOnly})
	require.Equal(t, "PostOnly", value)
}

func TestToPlaceOrderRequestUsesNativePercentSlippageForMarketOrders(t *testing.T) {
	req, err := toPlaceOrderRequest(context.Background(), stubExchange{}, categorySpot, &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("2"),
		Slippage: decimal.RequireFromString("0.01"),
	})
	require.NoError(t, err)
	require.Equal(t, "Percent", req.SlippageToleranceType)
	require.Equal(t, "1", req.SlippageTolerance)
}
