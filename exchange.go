package exchanges

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
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
	// FetchHistoricalTrades returns paginated historical trades.
	// opts may be nil; a nil opts means "most recent page, adapter default limit".
	// Adapters that do not support paginated history must return ErrNotSupported.
	FetchHistoricalTrades(ctx context.Context, symbol string, opts *HistoricalTradeOpts) ([]Trade, error)
	FetchKlines(ctx context.Context, symbol string, interval Interval, opts *KlineOpts) ([]Kline, error)

	// === Trading ===
	// Unsuffixed write methods are the adapter's primary non-WS write path.
	// *WS methods are explicit WebSocket submissions and return only transport/ACK errors.
	PlaceOrder(ctx context.Context, params *OrderParams) (*Order, error)
	PlaceOrderWS(ctx context.Context, params *OrderParams) error
	CancelOrder(ctx context.Context, orderID, symbol string) error
	CancelOrderWS(ctx context.Context, orderID, symbol string) error
	CancelAllOrders(ctx context.Context, symbol string) error
	FetchOrderByID(ctx context.Context, orderID, symbol string) (*Order, error)
	FetchOrders(ctx context.Context, symbol string) ([]Order, error)
	FetchOpenOrders(ctx context.Context, symbol string) ([]Order, error)

	// === Account ===
	FetchAccount(ctx context.Context) (*Account, error)
	FetchBalance(ctx context.Context) (decimal.Decimal, error)
	FetchSymbolDetails(ctx context.Context, symbol string) (*SymbolDetails, error)
	FetchFeeRate(ctx context.Context, symbol string) (*FeeRate, error)

	// === Local OrderBook (WS-maintained) ===
	// WatchOrderBook subscribes to orderbook updates and maintains a local copy.
	// The callback is called on every update; pass nil for pull-only mode.
	// depth controls the callback snapshot size. Use depth <= 0 for full depth.
	// This method blocks until the initial snapshot is synced.
	WatchOrderBook(ctx context.Context, symbol string, depth int, cb OrderBookCallback) error
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
	// FetchFundingRateHistory returns historical funding rates for a symbol.
	// opts may be nil for "most recent adapter-default page".
	// Hourly normalization: returned FundingRate entries use the same
	// per-hour convention as FetchFundingRate.
	FetchFundingRateHistory(ctx context.Context, symbol string, opts *FundingRateHistoryOpts) ([]FundingRate, error)
	// FetchOpenInterest returns current open interest for a perp symbol.
	FetchOpenInterest(ctx context.Context, symbol string) (*OpenInterest, error)
	ModifyOrder(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) (*Order, error)
	ModifyOrderWS(ctx context.Context, orderID, symbol string, params *ModifyOrderParams) error
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
	// WatchOrders emits order lifecycle overview updates.
	// Use it for order price, quantity, filled quantity, status, IDs, and timestamp.
	// It does not promise execution-detail fields such as average fill price or last fill price.
	WatchOrders(ctx context.Context, cb OrderUpdateCallback) error
	// WatchFills emits execution detail updates.
	// Use it for execution price, execution quantity, fee, fee asset, and maker/taker attribution.
	WatchFills(ctx context.Context, cb FillCallback) error
	WatchPositions(ctx context.Context, cb PositionUpdateCallback) error
	WatchTicker(ctx context.Context, symbol string, cb TickerCallback) error
	WatchTrades(ctx context.Context, symbol string, cb TradeCallback) error
	WatchKlines(ctx context.Context, symbol string, interval Interval, cb KlineCallback) error
	StopWatchOrders(ctx context.Context) error
	StopWatchFills(ctx context.Context) error
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
type FillCallback func(*Fill)
type PositionUpdateCallback func(*Position)
type TickerCallback func(*Ticker)
type OrderBookCallback func(*OrderBook)
type KlineCallback func(*Kline)
type TradeCallback func(*Trade)

// ============================================================================
// Convenience Functions
// ============================================================================

// PlaceMarketOrder is a convenience function for placing a market order.
// Optionally pass a reference price to avoid adapter-side ticker lookups when supported.
func PlaceMarketOrder(ctx context.Context, adp Exchange, symbol string, side OrderSide, qty decimal.Decimal, price ...decimal.Decimal) (*Order, error) {
	return adp.PlaceOrder(ctx, &OrderParams{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeMarket,
		Quantity: qty,
		Price:    firstDecimal(price),
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
// Optionally pass a reference price to avoid adapter-side ticker lookups when supported.
func PlaceMarketOrderWithSlippage(ctx context.Context, adp Exchange, symbol string, side OrderSide, qty, slippage decimal.Decimal, price ...decimal.Decimal) (*Order, error) {
	return adp.PlaceOrder(ctx, &OrderParams{
		Symbol:   symbol,
		Side:     side,
		Type:     OrderTypeMarket,
		Quantity: qty,
		Price:    firstDecimal(price),
		Slippage: slippage,
	})
}

func firstDecimal(values []decimal.Decimal) decimal.Decimal {
	if len(values) == 0 {
		return decimal.Zero
	}
	return values[0]
}
