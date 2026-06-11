package bitget

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type stubExchange struct {
	ticker *exchanges.Ticker
}

func (s stubExchange) GetExchange() string                 { return exchangeName }
func (s stubExchange) GetMarketType() exchanges.MarketType { return exchanges.MarketTypeSpot }
func (s stubExchange) Close() error                        { return nil }
func (s stubExchange) FormatSymbol(symbol string) string   { return symbol + "USDT" }
func (s stubExchange) ExtractSymbol(symbol string) string  { return symbol }
func (s stubExchange) ListSymbols() []string               { return nil }
func (s stubExchange) FetchTicker(ctx context.Context, symbol string) (*exchanges.Ticker, error) {
	return s.ticker, nil
}
func (s stubExchange) FetchOrderBook(ctx context.Context, symbol string, limit int) (*exchanges.OrderBook, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchTrades(ctx context.Context, symbol string, limit int) ([]exchanges.Trade, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchHistoricalTrades(context.Context, string, *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	return nil, nil
}
func (s stubExchange) FetchKlines(ctx context.Context, symbol string, interval exchanges.Interval, opts *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) PlaceOrder(ctx context.Context, params *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) PlaceOrderWS(ctx context.Context, params *exchanges.OrderParams) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) CancelOrder(ctx context.Context, orderID, symbol string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) CancelOrderWS(ctx context.Context, orderID, symbol string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) CancelAllOrders(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) FetchOrderByID(ctx context.Context, orderID, symbol string) (*exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchOpenOrders(ctx context.Context, symbol string) ([]exchanges.Order, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchAccount(ctx context.Context) (*exchanges.Account, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchBalance(ctx context.Context) (decimal.Decimal, error) {
	return decimal.Zero, exchanges.ErrNotSupported
}
func (s stubExchange) FetchSymbolDetails(ctx context.Context, symbol string) (*exchanges.SymbolDetails, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) FetchFeeRate(ctx context.Context, symbol string) (*exchanges.FeeRate, error) {
	return nil, exchanges.ErrNotSupported
}
func (s stubExchange) WatchOrderBook(ctx context.Context, symbol string, depth int, cb exchanges.OrderBookCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) GetLocalOrderBook(symbol string, depth int) *exchanges.OrderBook { return nil }
func (s stubExchange) StopWatchOrderBook(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchOrders(ctx context.Context, cb exchanges.OrderUpdateCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchFills(ctx context.Context, cb exchanges.FillCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchPositions(ctx context.Context, cb exchanges.PositionUpdateCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchTicker(ctx context.Context, symbol string, cb exchanges.TickerCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchTrades(ctx context.Context, symbol string, cb exchanges.TradeCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) WatchKlines(ctx context.Context, symbol string, interval exchanges.Interval, cb exchanges.KlineCallback) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) StopWatchOrders(ctx context.Context) error    { return exchanges.ErrNotSupported }
func (s stubExchange) StopWatchFills(ctx context.Context) error     { return exchanges.ErrNotSupported }
func (s stubExchange) StopWatchPositions(ctx context.Context) error { return exchanges.ErrNotSupported }
func (s stubExchange) StopWatchTicker(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) StopWatchTrades(ctx context.Context, symbol string) error {
	return exchanges.ErrNotSupported
}
func (s stubExchange) StopWatchKlines(ctx context.Context, symbol string, interval exchanges.Interval) error {
	return exchanges.ErrNotSupported
}

func TestToPlaceOrderRequestConvertsSpotMarketBuyToQuoteQty(t *testing.T) {
	req, err := toPlaceOrderRequest(context.Background(), stubExchange{
		ticker: &exchanges.Ticker{
			LastPrice: decimal.RequireFromString("10"),
			Ask:       decimal.RequireFromString("11"),
		},
	}, categorySpot, &exchanges.OrderParams{
		Symbol:   "BTC",
		Side:     exchanges.OrderSideBuy,
		Type:     exchanges.OrderTypeMarket,
		Quantity: decimal.RequireFromString("2"),
	})
	require.NoError(t, err)
	require.Equal(t, "22", req.Qty)
}
