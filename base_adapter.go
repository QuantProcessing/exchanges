package exchanges

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// BaseAdapter is designed to be embedded in specific exchange adapters.
// It provides a unified implementation for common adapter requirements:
// - Connection tracking (WS Market, WS Order, WS Account)
// - Local OrderBook mapping and readiness waiting
// - Symbol detail caching
// - Automatic order validation and slippage handling
//
// For account state management (orders, positions, balance), use
// TradingAccount, which wraps the Exchange adapter externally.
type BaseAdapter struct {
	Name       string
	MarketType MarketType
	Logger     Logger    // Logger for this adapter

	// Connection tracking
	marketConnected  bool
	accountConnected bool
	orderConnected   bool
	connMu           sync.RWMutex

	// Symbol Details
	symbolDetails map[string]*SymbolDetails
	symbolMu      sync.RWMutex

	// Local Orderbooks
	orderBooks map[string]LocalOrderBook
	obMu       sync.RWMutex
}

// NewBaseAdapter creates a new initialized BaseAdapter
func NewBaseAdapter(name string, marketType MarketType, logger Logger) *BaseAdapter {
	if logger == nil {
		logger = NopLogger
	}
	return &BaseAdapter{
		Name:          name,
		MarketType:    marketType,
		Logger:        logger,
		symbolDetails: make(map[string]*SymbolDetails),
		orderBooks:    make(map[string]LocalOrderBook),
	}
}

// GetExchange returns the exchange name
func (b *BaseAdapter) GetExchange() string {
	return b.Name
}

// GetMarketType returns the market type
func (b *BaseAdapter) GetMarketType() MarketType {
	return b.MarketType
}

// ================= Connection Tracking =================

// MarkMarketConnected marks the market websocket as connected
func (b *BaseAdapter) MarkMarketConnected() {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	b.marketConnected = true
}

// IsMarketConnected checks if market WS is connected
func (b *BaseAdapter) IsMarketConnected() bool {
	b.connMu.RLock()
	defer b.connMu.RUnlock()
	return b.marketConnected
}

// MarkAccountConnected marks the account websocket as connected
func (b *BaseAdapter) MarkAccountConnected() {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	b.accountConnected = true
}

// IsAccountConnected checks if account WS is connected
func (b *BaseAdapter) IsAccountConnected() bool {
	b.connMu.RLock()
	defer b.connMu.RUnlock()
	return b.accountConnected
}

// MarkOrderConnected marks the order websocket as connected
func (b *BaseAdapter) MarkOrderConnected() {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	b.orderConnected = true
}

// IsOrderConnected checks if order WS is connected
func (b *BaseAdapter) IsOrderConnected() bool {
	b.connMu.RLock()
	defer b.connMu.RUnlock()
	return b.orderConnected
}

// ================= Symbol Details Management =================

// SetSymbolDetails replaces the entire symbol details cache
func (b *BaseAdapter) SetSymbolDetails(details map[string]*SymbolDetails) {
	b.symbolMu.Lock()
	defer b.symbolMu.Unlock()
	b.symbolDetails = details
}

// GetSymbolDetail returns the cached detail for a symbol
func (b *BaseAdapter) GetSymbolDetail(symbol string) (*SymbolDetails, error) {
	b.symbolMu.RLock()
	defer b.symbolMu.RUnlock()
	d, ok := b.symbolDetails[symbol]
	if !ok {
		return nil, fmt.Errorf("symbol details not found in cache for %s", symbol)
	}
	// Return a copy to prevent accidental modifications
	copyDetail := *d
	return &copyDetail, nil
}

// ListSymbols returns all symbols in the symbol details cache.
// These are base currency symbols (e.g. "BTC", "ETH") loaded at adapter init.
func (b *BaseAdapter) ListSymbols() []string {
	b.symbolMu.RLock()
	defer b.symbolMu.RUnlock()
	symbols := make([]string, 0, len(b.symbolDetails))
	for s := range b.symbolDetails {
		symbols = append(symbols, s)
	}
	return symbols
}

// ================= OrderBook Management =================

// SetLocalOrderBook registers an instantiated LocalOrderBook implementation
func (b *BaseAdapter) SetLocalOrderBook(symbol string, ob LocalOrderBook) {
	b.obMu.Lock()
	defer b.obMu.Unlock()
	b.orderBooks[symbol] = ob
}

// RemoveLocalOrderBook removes a local orderbook
func (b *BaseAdapter) RemoveLocalOrderBook(symbol string) {
	b.obMu.Lock()
	defer b.obMu.Unlock()
	delete(b.orderBooks, symbol)
}

// GetLocalOrderBookImplementation returns the underlying LocalOrderBook implementation
func (b *BaseAdapter) GetLocalOrderBookImplementation(symbol string) (LocalOrderBook, bool) {
	b.obMu.RLock()
	defer b.obMu.RUnlock()
	ob, ok := b.orderBooks[symbol]
	return ob, ok
}

// WaitOrderBookReady waits for a specific subscribed orderbook to be ready.
func (b *BaseAdapter) WaitOrderBookReady(ctx context.Context, symbol string) error {
	ob, ok := b.GetLocalOrderBookImplementation(symbol)
	if !ok {
		return fmt.Errorf("orderbook not subscribed for %s", symbol)
	}
	if !ob.WaitReady(ctx, 30*time.Second) {
		return fmt.Errorf("timeout waiting for orderbook ready for %s", symbol)
	}
	return nil
}

// GetLocalOrderBook returns the standard OrderBook struct from the local WS-maintained orderbook.
// This satisfies the Exchange interface requirement.
func (b *BaseAdapter) GetLocalOrderBook(symbol string, depth int) *OrderBook {
	ob, ok := b.GetLocalOrderBookImplementation(symbol)
	if !ok {
		b.Logger.Debugw("GetLocalOrderBook called but book not found", "symbol", symbol)
		return nil
	}

	bids, asks := ob.GetDepth(depth)
	ts := ob.Timestamp()

	return &OrderBook{
		Symbol:    symbol,
		Timestamp: ts,
		Bids:      bids,
		Asks:      asks,
	}
}

// ================= Order Validation =================

// ValidateOrder validates and auto-formats order params using cached symbol details.
// Call this at the start of PlaceOrder in every adapter.
func (b *BaseAdapter) ValidateOrder(params *OrderParams) error {
	detail, err := b.GetSymbolDetail(params.Symbol)
	if err != nil {
		return nil // No cached details, skip validation
	}
	return ValidateAndFormatParams(params, detail)
}

// ================= Slippage Logic =================

// ApplySlippage converts a MARKET order with Slippage>0 into a LIMIT IOC order.
// The fetchTicker function is injected by the concrete adapter.
func (b *BaseAdapter) ApplySlippage(
	ctx context.Context,
	params *OrderParams,
	fetchTicker func(ctx context.Context, symbol string) (*Ticker, error),
) error {
	if params.Slippage.IsZero() || params.Slippage.IsNegative() {
		return nil
	}
	if params.Type != OrderTypeMarket {
		return nil
	}

	one := decimal.NewFromInt(1)
	refPrice := params.Price

	if !refPrice.IsPositive() {
		ticker, err := fetchTicker(ctx, params.Symbol)
		if err != nil {
			return fmt.Errorf("slippage requires ticker: %w", err)
		}

		if params.Side == OrderSideBuy {
			refPrice = ticker.Ask
			if refPrice.IsZero() {
				refPrice = ticker.LastPrice
			}
		} else {
			refPrice = ticker.Bid
			if refPrice.IsZero() {
				refPrice = ticker.LastPrice
			}
		}
	}

	if params.Side == OrderSideBuy {
		params.Price = refPrice.Mul(one.Add(params.Slippage))
	} else {
		params.Price = refPrice.Mul(one.Sub(params.Slippage))
	}

	// Round price to symbol precision if available
	if d, err := b.GetSymbolDetail(params.Symbol); err == nil {
		params.Price = RoundToPrecision(params.Price, d.PricePrecision)
	}

	params.Type = OrderTypeLimit
	if params.TimeInForce == "" {
		params.TimeInForce = TimeInForceIOC
	}
	return nil
}
