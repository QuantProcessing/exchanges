package exchanges

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Order Mode
// ============================================================================

// OrderMode controls whether trading operations (PlaceOrder, CancelOrder, etc.)
// use WebSocket or REST/HTTP transport.
type OrderMode string

const (
	// OrderModeWS uses WebSocket for order operations (default, lower latency).
	OrderModeWS OrderMode = "ws"
	// OrderModeREST uses REST/HTTP for order operations (no persistent connection needed).
	OrderModeREST OrderMode = "rest"
)

// ============================================================================
// Core Interface: Exchange
// ============================================================================

// Exchange is the primary interface for strategy developers.
// It provides a unified, CCXT-inspired API for interacting with any exchanges.
//
// Symbol convention: all methods accept a **base currency** symbol (e.g. "BTC",
// "ETH"). The adapter handles conversion to echange-specific formats internally.
//
// Method naming convention: Fetch* = REST query, Watch* = WebSocket subscription.
type Exchange interface {
	// === Identity ===
	GetExchange() string
	GetMarketType() MarketType
	Close() error

	// === Symbol Mapping ===
	// FormatSymbol converts a base symbol (e.g. "BTC") to exchange-specific format.
	FormatSymbol(symbol string) string
	// ExtractSymbol converts an exchange-specific symbol back to base symbol.
	ExtractSymbol(symbol string) string
	// ListSymbols returns all symbols supported by this adapter.
	ListSymbols() []string

	// === Market Data (REST) ===
	FetchTicker(ctx context.Context, symbol string) (*Ticker, error)
	FetchOrderBook(ctx context.Context, symbol string, limit int) (*OrderBook, error)
	FetchTrades(ctx context.Context, symbol string, limit int) ([]Trade, error)
	FetchKlines(ctx context.Context, symbol string, interval Interval, opts *KlineOpts) ([]Kline, error)

	// === Trading ===
	PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
	CancelOrder(ctx context.Context, orderID, symbol string) error
	CancelAllOrders(ctx context.Context, symbol string) error
	FetchOrder(ctx context.Context, orderID, symbol string) (*Order, error)
	FetchOpenOrders(ctx context.Context, symbol string) ([]Order, error)

	// === Account ===
	FetchAccount(ctx context.Context) (*Account, error)
	FetchBalance(ctx context.Context) (decimal.Decimal, error)
	FetchSymbolDetails(ctx context.Context, symbol string) (*SymbolDetails, error)
	FetchFeeRate(ctx context.Context, symbol string) (*FeeRate, error)

	// === Local OrderBook (WS-maintained) ===
	// WatchOrderBook subscribes to orderbook updates and maintains a local copy.
	// The callback is called on every update; pass nil for pull-only mode.
	// This method blocks until the initial snapshot is synced.
	WatchOrderBook(ctx context.Context, symbol string, cb OrderBookCallback) error
	GetLocalOrderBook(symbol string, depth int) *OrderBook
	StopWatchOrderBook(ctx context.Context, symbol string) error

	// === WebSocket Streaming ===
	Streamable
}

// ============================================================================
// Extension Interfaces
// ============================================================================

// PerpExchange extends Exchange with perpetual futures capabilities.
// Use type assertion: if perp, ok := adp.(adapter.PerpExchange); ok { ... }
type PerpExchange interface {
	Exchange
	FetchPositions(ctx context.Context) ([]Position, error)
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	FetchFundingRate(ctx context.Context, symbol string) (*FundingRate, error)
	FetchAllFundingRates(ctx context.Context) ([]FundingRate, error)
	ModifyOrder(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) (*Order, error)
}

// SpotExchange extends Exchange with spot-specific capabilities.
type SpotExchange interface {
	Exchange
	FetchSpotBalances(ctx context.Context) ([]SpotBalance, error)
	TransferAsset(ctx context.Context, params *TransferParams) error
}

// Streamable provides WebSocket streaming capabilities.
// All Watch methods accept a callback. Not all exchanges support all stream types.
type Streamable interface {
	WatchOrders(ctx context.Context, cb OrderUpdateCallback) error
	WatchPositions(ctx context.Context, cb PositionUpdateCallback) error
	WatchTicker(ctx context.Context, symbol string, cb TickerCallback) error
	WatchTrades(ctx context.Context, symbol string, cb TradeCallback) error
	WatchKlines(ctx context.Context, symbol string, interval Interval, cb KlineCallback) error
	StopWatchOrders(ctx context.Context) error
	StopWatchPositions(ctx context.Context) error
	StopWatchTicker(ctx context.Context, symbol string) error
	StopWatchTrades(ctx context.Context, symbol string) error
	StopWatchKlines(ctx context.Context, symbol string, interval Interval) error
}

// ============================================================================
// Optional Parameters
// ============================================================================

// KlineOpts provides optional parameters for FetchKlines.
type KlineOpts struct {
	Start *time.Time
	End   *time.Time
	Limit int
}

// ============================================================================
// Callbacks
// ============================================================================

type OrderUpdateCallback func(*Order)
type PositionUpdateCallback func(*Position)
type TickerCallback func(*Ticker)
type OrderBookCallback func(*OrderBook)
type KlineCallback func(*Kline)
type TradeCallback func(*Trade)

// ============================================================================
// Convenience Functions
// ============================================================================

// PlaceMarketOrder is a convenience function for placing a market order.
func PlaceMarketOrder(ctx context.Context, adp Exchange, symbol string, side OrderSide, qty decimal.Decimal) (*Order, error) {
	return adp.PlaceOrder(ctx, &OrderParams{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeMarket,
		Quantity: qty,
	})
}

// PlaceLimitOrder is a convenience function for placing a limit order.
func PlaceLimitOrder(ctx context.Context, adp Exchange, symbol string, side OrderSide, price, qty decimal.Decimal) (*Order, error) {
	return adp.PlaceOrder(ctx, &OrderParams{
		Symbol:      symbol,
		Side:        side,
		Type:        OrderTypeLimit,
		Quantity:    qty,
		Price:       price,
		TimeInForce: TimeInForceGTC,
	})
}

// PlaceMarketOrderWithSlippage is a convenience function for placing a market order with slippage protection.
func PlaceMarketOrderWithSlippage(ctx context.Context, adp Exchange, symbol string, side OrderSide, qty, slippage decimal.Decimal) (*Order, error) {
	return adp.PlaceOrder(ctx, &OrderParams{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeMarket,
		Quantity: qty,
		Slippage: slippage,
	})
}
