package hyperliquid

import (
	"context"
	"sort"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"

	"github.com/shopspring/decimal"
)

type OrderBook struct {
	sync.RWMutex
	symbol      string
	bids        map[string]decimal.Decimal
	asks        map[string]decimal.Decimal
	timestamp   int64
	initialized bool // Track if snapshot has been received
}

// Timestamp satisfies the LocalOrderBook interface
func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make(map[string]decimal.Decimal),
		asks:   make(map[string]decimal.Decimal),
	}
}

func (ob *OrderBook) ProcessSnapshot(data hyperliquid.WsL2Book) {
	ob.Lock()
	defer ob.Unlock()

	ob.timestamp = data.Time

	// Hyperliquid L2 Book via WS is a snapshot
	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)

	for _, level := range data.Levels[0] {
		sz := parseDecimal(level.Sz)
		if sz.IsPositive() {
			ob.bids[level.Px] = sz
		}
	}

	for _, level := range data.Levels[1] {
		sz := parseDecimal(level.Sz)
		if sz.IsPositive() {
			ob.asks[level.Px] = sz
		}
	}

	// Mark as initialized when we receive first snapshot
	ob.initialized = true
}

func (ob *OrderBook) GetDepth(limit int) ([]exchanges.Level, []exchanges.Level) {
	ob.RLock()
	defer ob.RUnlock()

	// Extract bids
	bids := make([]exchanges.Level, 0, len(ob.bids))
	for pStr, q := range ob.bids {
		p, _ := decimal.NewFromString(pStr)
		bids = append(bids, exchanges.Level{Price: p, Quantity: q})
	}
	// Sort bids: high to low
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.GreaterThan(bids[j].Price)
	})

	// Extract asks
	asks := make([]exchanges.Level, 0, len(ob.asks))
	for pStr, q := range ob.asks {
		p, _ := decimal.NewFromString(pStr)
		asks = append(asks, exchanges.Level{Price: p, Quantity: q})
	}
	// Sort asks: low to high
	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Price.LessThan(asks[j].Price)
	})

	if limit > 0 {
		if len(bids) > limit {
			bids = bids[:limit]
		}
		if len(asks) > limit {
			asks = asks[:limit]
		}
	}

	return bids, asks
}

// GetBestBid gets the best bid (Price, Quantity). Returns 0,0 if no bids.
func (ob *OrderBook) GetBestBid() (decimal.Decimal, decimal.Decimal) {
	ob.RLock()
	defer ob.RUnlock()

	bestPrice, bestQty := decimal.Zero, decimal.Zero
	first := true

	for pStr, q := range ob.bids {
		p, _ := decimal.NewFromString(pStr)
		if first || p.GreaterThan(bestPrice) {
			bestPrice = p
			bestQty = q
			first = false
		}
	}
	return bestPrice, bestQty
}

// GetBestAsk gets the best ask (Price, Quantity). Returns 0,0 if no asks.
func (ob *OrderBook) GetBestAsk() (decimal.Decimal, decimal.Decimal) {
	ob.RLock()
	defer ob.RUnlock()

	bestPrice, bestQty := decimal.Zero, decimal.Zero
	first := true

	for pStr, q := range ob.asks {
		p, _ := decimal.NewFromString(pStr)
		if first || p.LessThan(bestPrice) {
			bestPrice = p
			bestQty = q
			first = false
		}
	}
	return bestPrice, bestQty
}

func (ob *OrderBook) ToAdapterOrderBook(depth int) *exchanges.OrderBook {
	bids, asks := ob.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    ob.symbol,
		Timestamp: ob.timestamp,
		Bids:      bids,
		Asks:      asks,
	}
}

// IsInitialized returns whether the orderbook has received its first snapshot
func (ob *OrderBook) IsInitialized() bool {
	ob.RLock()
	defer ob.RUnlock()
	return ob.initialized
}

// WaitReady 等待 OrderBook 初始化完成
func (ob *OrderBook) WaitReady(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if ob.IsInitialized() {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if time.Now().After(deadline) {
				return false
			}
		}
	}
}

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
