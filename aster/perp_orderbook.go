package aster

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/aster/sdk/perp"

	"github.com/shopspring/decimal"
)

// OrderBook maintains a local orderbook replica for Aster (Binance-compatible protocol).
// Uses string-keyed maps because decimal.Decimal is not comparable (can't be map key).
type OrderBook struct {
	sync.RWMutex
	symbol          string
	bids            map[string]decimal.Decimal // price_str -> qty
	asks            map[string]decimal.Decimal
	lastUpdateID    int64
	timestamp       int64 // exchange server time (ms)
	initialized     bool
	buffer          []*perp.WsDepthEvent
	pendingSnapshot *perp.DepthResponse
}

// NewOrderBook creates a new OrderBook instance
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol:      symbol,
		bids:        make(map[string]decimal.Decimal),
		asks:        make(map[string]decimal.Decimal),
		buffer:      make([]*perp.WsDepthEvent, 0, 1000),
		initialized: false,
	}
}

// Reset resets the OrderBook state
func (ob *OrderBook) Reset() {
	ob.Lock()
	defer ob.Unlock()
	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)
	ob.lastUpdateID = 0
	ob.initialized = false
	ob.buffer = ob.buffer[:0]
	ob.pendingSnapshot = nil
}

// IsInitialized returns whether the snapshot has been applied
func (ob *OrderBook) IsInitialized() bool {
	ob.RLock()
	defer ob.RUnlock()
	return ob.initialized
}

// ProcessUpdate processes a WebSocket incremental update
func (ob *OrderBook) ProcessUpdate(event *perp.WsDepthEvent) error {
	ob.Lock()
	defer ob.Unlock()

	if !ob.initialized {
		ob.buffer = append(ob.buffer, event)
		if ob.pendingSnapshot != nil {
			if err := ob.tryApplySnapshot(); err == nil {
				ob.initialized = true
				ob.pendingSnapshot = nil
			}
		}
		return nil
	}

	if event.FinalUpdateIDLast != ob.lastUpdateID {
		ob.initialized = false
		ob.buffer = ob.buffer[:0]
		ob.buffer = append(ob.buffer, event)
		return fmt.Errorf("reset required: gap detected, prev_u=%d, curr_pu=%d", ob.lastUpdateID, event.FinalUpdateIDLast)
	}

	ob.applyEvent(event)
	return nil
}

// ApplySnapshot applies a REST API depth snapshot
func (ob *OrderBook) ApplySnapshot(snap *perp.DepthResponse) error {
	ob.Lock()
	defer ob.Unlock()

	if ob.initialized {
		return nil
	}

	ob.pendingSnapshot = snap
	if err := ob.tryApplySnapshot(); err == nil {
		ob.initialized = true
		ob.pendingSnapshot = nil
		return nil
	} else if err.Error() == "gap detected" {
		ob.pendingSnapshot = nil
		return err
	}

	return fmt.Errorf("snapshot too new, buffered")
}

func (ob *OrderBook) tryApplySnapshot() error {
	snap := ob.pendingSnapshot
	if snap == nil {
		return fmt.Errorf("no pending snapshot")
	}

	lastUpdateID := snap.LastUpdateID

	validStartIndex := -1
	for i, event := range ob.buffer {
		U := event.FirstUpdateID
		u := event.FinalUpdateID

		if u < lastUpdateID {
			continue
		}

		if U <= lastUpdateID && u >= lastUpdateID {
			validStartIndex = i
			break
		}
	}

	if validStartIndex == -1 {
		hasGap := false
		if len(ob.buffer) > 0 {
			lastEvent := ob.buffer[len(ob.buffer)-1]
			if lastEvent.FirstUpdateID > lastUpdateID {
				hasGap = true
			}
		}

		if hasGap {
			ob.buffer = ob.buffer[:0]
			return fmt.Errorf("gap detected")
		}
		return fmt.Errorf("too new")
	}

	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)

	for _, bid := range snap.Bids {
		if len(bid) < 2 {
			continue
		}
		priceStr := bid[0]
		qty, _ := decimal.NewFromString(bid[1])
		if qty.IsPositive() {
			ob.bids[priceStr] = qty
		}
	}
	for _, ask := range snap.Asks {
		if len(ask) < 2 {
			continue
		}
		priceStr := ask[0]
		qty, _ := decimal.NewFromString(ask[1])
		if qty.IsPositive() {
			ob.asks[priceStr] = qty
		}
	}

	ob.lastUpdateID = lastUpdateID

	for i := validStartIndex; i < len(ob.buffer); i++ {
		event := ob.buffer[i]

		if i > validStartIndex {
			if event.FinalUpdateIDLast != ob.lastUpdateID {
				ob.buffer = ob.buffer[:0]
				return fmt.Errorf("buffer internal gap: prev=%d, curr=%d", ob.lastUpdateID, event.FinalUpdateIDLast)
			}
		}

		ob.applyEvent(event)
	}

	ob.buffer = ob.buffer[:0]
	return nil
}

func (ob *OrderBook) applyEvent(event *perp.WsDepthEvent) {
	for _, b := range event.Bids {
		if len(b) < 2 {
			continue
		}
		priceStr := fmt.Sprintf("%v", b[0])
		qty := parseDecimalInterface(b[1])

		if qty.IsZero() {
			delete(ob.bids, priceStr)
		} else {
			ob.bids[priceStr] = qty
		}
	}

	for _, a := range event.Asks {
		if len(a) < 2 {
			continue
		}
		priceStr := fmt.Sprintf("%v", a[0])
		qty := parseDecimalInterface(a[1])

		if qty.IsZero() {
			delete(ob.asks, priceStr)
		} else {
			ob.asks[priceStr] = qty
		}
	}

	ob.lastUpdateID = event.FinalUpdateID
	if event.EventTime > 0 {
		ob.timestamp = event.EventTime
	}
}

// GetDepth returns the sorted top `limit` depth levels.
func (ob *OrderBook) GetDepth(limit int) ([]exchanges.Level, []exchanges.Level) {
	ob.RLock()
	defer ob.RUnlock()

	bids := make([]exchanges.Level, 0, len(ob.bids))
	for pStr, q := range ob.bids {
		p, _ := decimal.NewFromString(pStr)
		bids = append(bids, exchanges.Level{Price: p, Quantity: q})
	}
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.GreaterThan(bids[j].Price)
	})

	asks := make([]exchanges.Level, 0, len(ob.asks))
	for pStr, q := range ob.asks {
		p, _ := decimal.NewFromString(pStr)
		asks = append(asks, exchanges.Level{Price: p, Quantity: q})
	}
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

// GetBestBid returns the best bid price and quantity
func (ob *OrderBook) GetBestBid() (decimal.Decimal, decimal.Decimal) {
	ob.RLock()
	defer ob.RUnlock()

	bestPrice := decimal.Zero
	bestQty := decimal.Zero
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

// GetBestAsk returns the best ask price and quantity
func (ob *OrderBook) GetBestAsk() (decimal.Decimal, decimal.Decimal) {
	ob.RLock()
	defer ob.RUnlock()

	bestPrice := decimal.Zero
	bestQty := decimal.Zero
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

// WaitReady waits for the OrderBook to be initialized
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

// Timestamp satisfies the LocalOrderBook interface
func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}

// Ensure OrderBook implements exchanges.LocalOrderBook at compile time
var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
