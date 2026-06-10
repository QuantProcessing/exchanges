// Package okx — OptionAdapter skeleton.
//
// This file declares the OKX option adapter surface so consumers can compile
// against the OptionExchange interface even before the full implementation
// lands. Every method returns ErrNotSupported with a TODO marker.
//
// Reference implementation: binance/option_adapter.go.
// OKX uses instType=OPTION on v5 endpoints; instrument IDs look like
// "BTC-USD-251226-100000-C".
package okx

import (
	"context"
	"fmt"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/shopspring/decimal"
)

// OptionAdapter is a not-yet-implemented OKX options adapter.
type OptionAdapter struct {
	*exchanges.BaseAdapter
}

// NewOptionAdapter constructs the skeleton. It does not connect or sign.
func NewOptionAdapter(_ context.Context, opts Options) (*OptionAdapter, error) {
	logger := exchanges.NopLogger
	if opts.Logger != nil {
		logger = opts.Logger
	}
	return &OptionAdapter{
		BaseAdapter: exchanges.NewBaseAdapter("OKX_OPTION", exchanges.MarketTypeOption, logger),
	}, nil
}

func (a *OptionAdapter) Close() error { return nil }

func notSupported(op string) error {
	return fmt.Errorf("okx options %s: %w (TODO: implement against /api/v5 instType=OPTION)", op, exchanges.ErrNotSupported)
}

// =============================================================================
// OptionExchange — all stubs.
// =============================================================================

func (a *OptionAdapter) FetchOptionChain(context.Context, string, *exchanges.OptionChainOpts) ([]exchanges.OptionInstrument, error) {
	return nil, notSupported("FetchOptionChain")
}
func (a *OptionAdapter) FetchExpirations(context.Context, string) ([]time.Time, error) {
	return nil, notSupported("FetchExpirations")
}
func (a *OptionAdapter) FetchGreeks(context.Context, string) (*exchanges.Greeks, error) {
	return nil, notSupported("FetchGreeks")
}
func (a *OptionAdapter) FetchOptionMark(context.Context, string) (*exchanges.OptionMark, error) {
	return nil, notSupported("FetchOptionMark")
}
func (a *OptionAdapter) FetchOptionPositions(context.Context) ([]exchanges.Position, error) {
	return nil, notSupported("FetchOptionPositions")
}
func (a *OptionAdapter) FormatInstrument(*exchanges.OptionInstrument) string { return "" }
func (a *OptionAdapter) ParseInstrument(string) (*exchanges.OptionInstrument, error) {
	return nil, notSupported("ParseInstrument")
}

// =============================================================================
// Exchange — also stubs.
// =============================================================================

func (a *OptionAdapter) FormatSymbol(s string) string  { return s }
func (a *OptionAdapter) ExtractSymbol(s string) string { return s }

func (a *OptionAdapter) FetchTicker(context.Context, string) (*exchanges.Ticker, error) {
	return nil, notSupported("FetchTicker")
}
func (a *OptionAdapter) FetchOrderBook(context.Context, string, int) (*exchanges.OrderBook, error) {
	return nil, notSupported("FetchOrderBook")
}
func (a *OptionAdapter) FetchTrades(context.Context, string, int) ([]exchanges.Trade, error) {
	return nil, notSupported("FetchTrades")
}
func (a *OptionAdapter) FetchHistoricalTrades(context.Context, string, *exchanges.HistoricalTradeOpts) ([]exchanges.Trade, error) {
	return nil, notSupported("FetchHistoricalTrades")
}
func (a *OptionAdapter) FetchKlines(context.Context, string, exchanges.Interval, *exchanges.KlineOpts) ([]exchanges.Kline, error) {
	return nil, notSupported("FetchKlines")
}
func (a *OptionAdapter) PlaceOrder(context.Context, *exchanges.OrderParams) (*exchanges.Order, error) {
	return nil, notSupported("PlaceOrder")
}
func (a *OptionAdapter) PlaceOrderWS(context.Context, *exchanges.OrderParams) error {
	return notSupported("PlaceOrderWS")
}
func (a *OptionAdapter) CancelOrder(context.Context, string, string) error {
	return notSupported("CancelOrder")
}
func (a *OptionAdapter) CancelOrderWS(context.Context, string, string) error {
	return notSupported("CancelOrderWS")
}
func (a *OptionAdapter) CancelAllOrders(context.Context, string) error {
	return notSupported("CancelAllOrders")
}
func (a *OptionAdapter) FetchOrderByID(context.Context, string, string) (*exchanges.Order, error) {
	return nil, notSupported("FetchOrderByID")
}
func (a *OptionAdapter) FetchOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, notSupported("FetchOrders")
}
func (a *OptionAdapter) FetchOpenOrders(context.Context, string) ([]exchanges.Order, error) {
	return nil, notSupported("FetchOpenOrders")
}
func (a *OptionAdapter) FetchAccount(context.Context) (*exchanges.Account, error) {
	return nil, notSupported("FetchAccount")
}
func (a *OptionAdapter) FetchBalance(context.Context) (decimal.Decimal, error) {
	return decimal.Zero, notSupported("FetchBalance")
}
func (a *OptionAdapter) FetchSymbolDetails(context.Context, string) (*exchanges.SymbolDetails, error) {
	return nil, notSupported("FetchSymbolDetails")
}
func (a *OptionAdapter) FetchFeeRate(context.Context, string) (*exchanges.FeeRate, error) {
	return nil, notSupported("FetchFeeRate")
}

func (a *OptionAdapter) WatchOrderBook(context.Context, string, int, exchanges.OrderBookCallback) error {
	return notSupported("WatchOrderBook")
}
func (a *OptionAdapter) StopWatchOrderBook(context.Context, string) error { return nil }

func (a *OptionAdapter) WatchOrders(context.Context, exchanges.OrderUpdateCallback) error {
	return notSupported("WatchOrders")
}
func (a *OptionAdapter) WatchFills(context.Context, exchanges.FillCallback) error {
	return notSupported("WatchFills")
}
func (a *OptionAdapter) WatchPositions(context.Context, exchanges.PositionUpdateCallback) error {
	return notSupported("WatchPositions")
}
func (a *OptionAdapter) WatchTicker(context.Context, string, exchanges.TickerCallback) error {
	return notSupported("WatchTicker")
}
func (a *OptionAdapter) WatchTrades(context.Context, string, exchanges.TradeCallback) error {
	return notSupported("WatchTrades")
}
func (a *OptionAdapter) WatchKlines(context.Context, string, exchanges.Interval, exchanges.KlineCallback) error {
	return notSupported("WatchKlines")
}
func (a *OptionAdapter) StopWatchOrders(context.Context) error           { return nil }
func (a *OptionAdapter) StopWatchFills(context.Context) error            { return nil }
func (a *OptionAdapter) StopWatchPositions(context.Context) error        { return nil }
func (a *OptionAdapter) StopWatchTicker(context.Context, string) error   { return nil }
func (a *OptionAdapter) StopWatchTrades(context.Context, string) error   { return nil }
func (a *OptionAdapter) StopWatchKlines(context.Context, string, exchanges.Interval) error {
	return nil
}
