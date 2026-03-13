package standx

import (
	"context"
	"sort"
	"sync"
	"time"
	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/QuantProcessing/exchanges/standx/sdk"
)

type OrderBook struct {
	mu     sync.RWMutex
	symbol string
	asks   []exchanges.Level
	bids   []exchanges.Level
	time   time.Time
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		asks:   make([]exchanges.Level, 0),
		bids:   make([]exchanges.Level, 0),
	}
}

func (ob *OrderBook) UpdateSnapshot(data standx.WSDepthData) *exchanges.OrderBook {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.time = time.Now()
	ob.asks = CustomParseLevels(data.Asks)
	ob.bids = CustomParseLevels(data.Bids)

	// StandX 文档说明: "The sequence of price levels in the asks and bids arrays is not guaranteed.
	// Please implement local sorting on the client side based on your specific requirements."
	// Asks 按价格升序排列（best ask 在前）
	sort.Slice(ob.asks, func(i, j int) bool {
		return ob.asks[i].Price.LessThan(ob.asks[j].Price)
	})
	// Bids 按价格降序排列（best bid 在前）
	sort.Slice(ob.bids, func(i, j int) bool {
		return ob.bids[i].Price.GreaterThan(ob.bids[j].Price)
	})

	return ob.snapshotUnlocked()
}

func (ob *OrderBook) Snapshot() *exchanges.OrderBook {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.snapshotUnlocked()
}

func (ob *OrderBook) snapshotUnlocked() *exchanges.OrderBook {
	// Deep copy to avoid race conditions
	asks := make([]exchanges.Level, len(ob.asks))
	copy(asks, ob.asks)
	bids := make([]exchanges.Level, len(ob.bids))
	copy(bids, ob.bids)

	return &exchanges.OrderBook{
		Symbol:    ob.symbol,
		Timestamp: ob.time.UnixMilli(),
		Asks:      asks,
		Bids:      bids,
	}
}

func (ob *OrderBook) WaitReady(ctx context.Context, timeout time.Duration) bool {
	// Simple poll or channel based. For simplicity, just poll or check immediate.
	// If asks/bids > 0.
	start := time.Now()
	for {
		ob.mu.RLock()
		ready := len(ob.asks) > 0 || len(ob.bids) > 0
		ob.mu.RUnlock()

		if ready {
			return true
		}

		if time.Since(start) > timeout {
			return false
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

func CustomParseLevels(raw [][]string) []exchanges.Level {
	levels := make([]exchanges.Level, len(raw))
	for i, item := range raw {
		if len(item) < 2 {
			continue
		}
		levels[i] = exchanges.Level{Price: 
			parseDecimal(item[0]), Quantity: // defined in exchanges.go or duplicate here? duplicate to avoid cycle or export
			parseDecimal(item[1]),
		}
	}
	return levels
}

// Timestamp satisfies the LocalOrderBook interface
func (ob *OrderBook) Timestamp() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.time.UnixMilli()
}

// GetDepth satisfies the LocalOrderBook interface
func (ob *OrderBook) GetDepth(depth int) ([]exchanges.Level, []exchanges.Level) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bidLen := len(ob.bids)
	if depth > 0 && depth < bidLen {
		bidLen = depth
	}
	bids := make([]exchanges.Level, bidLen)
	copy(bids, ob.bids[:bidLen])

	askLen := len(ob.asks)
	if depth > 0 && depth < askLen {
		askLen = depth
	}
	asks := make([]exchanges.Level, askLen)
	copy(asks, ob.asks[:askLen])

	return bids, asks
}

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
