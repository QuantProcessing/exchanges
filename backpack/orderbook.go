package backpack

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/backpack/sdk"
	"github.com/shopspring/decimal"
)

type OrderBook struct {
	sync.RWMutex
	symbol          string
	bids            map[string]decimal.Decimal
	asks            map[string]decimal.Decimal
	lastUpdateID    int64
	timestamp       int64
	initialized     bool
	buffer          []*sdk.DepthEvent
	pendingSnapshot *sdk.Depth
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make(map[string]decimal.Decimal),
		asks:   make(map[string]decimal.Decimal),
		buffer: make([]*sdk.DepthEvent, 0, 256),
	}
}

func (ob *OrderBook) IsInitialized() bool {
	ob.RLock()
	defer ob.RUnlock()
	return ob.initialized
}

func (ob *OrderBook) ProcessUpdate(event *sdk.DepthEvent) error {
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

	if event.FirstUpdateID != ob.lastUpdateID+1 {
		ob.initialized = false
		ob.buffer = ob.buffer[:0]
		ob.buffer = append(ob.buffer, event)
		return fmt.Errorf("backpack orderbook: gap detected prev=%d next=%d", ob.lastUpdateID, event.FirstUpdateID)
	}

	ob.applyEvent(event)
	return nil
}

func (ob *OrderBook) ApplySnapshot(snap *sdk.Depth) error {
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
	if ob.pendingSnapshot == nil {
		return fmt.Errorf("no pending snapshot")
	}

	lastUpdateID, err := strconv.ParseInt(ob.pendingSnapshot.LastUpdateID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid snapshot update id: %w", err)
	}

	expectedNext := lastUpdateID + 1
	validStartIndex := -1
	for i, event := range ob.buffer {
		if event.FinalUpdateID < expectedNext {
			continue
		}
		if event.FirstUpdateID <= expectedNext && event.FinalUpdateID >= expectedNext {
			validStartIndex = i
			break
		}
	}

	if validStartIndex == -1 {
		if len(ob.buffer) == 0 {
			ob.bids = make(map[string]decimal.Decimal)
			ob.asks = make(map[string]decimal.Decimal)
			for _, level := range ob.pendingSnapshot.Bids {
				if len(level) < 2 {
					continue
				}
				qty := parseDecimal(level[1])
				if qty.IsPositive() {
					ob.bids[level[0]] = qty
				}
			}
			for _, level := range ob.pendingSnapshot.Asks {
				if len(level) < 2 {
					continue
				}
				qty := parseDecimal(level[1])
				if qty.IsPositive() {
					ob.asks[level[0]] = qty
				}
			}
			ob.lastUpdateID = lastUpdateID
			ob.timestamp = microsToMillis(ob.pendingSnapshot.Timestamp)
			return nil
		}
		if len(ob.buffer) > 0 && ob.buffer[len(ob.buffer)-1].FirstUpdateID > expectedNext {
			ob.buffer = ob.buffer[:0]
			return fmt.Errorf("gap detected")
		}
		return fmt.Errorf("too new")
	}

	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)

	for _, level := range ob.pendingSnapshot.Bids {
		if len(level) < 2 {
			continue
		}
		qty := parseDecimal(level[1])
		if qty.IsPositive() {
			ob.bids[level[0]] = qty
		}
	}
	for _, level := range ob.pendingSnapshot.Asks {
		if len(level) < 2 {
			continue
		}
		qty := parseDecimal(level[1])
		if qty.IsPositive() {
			ob.asks[level[0]] = qty
		}
	}
	ob.lastUpdateID = lastUpdateID
	ob.timestamp = microsToMillis(ob.pendingSnapshot.Timestamp)

	for i := validStartIndex; i < len(ob.buffer); i++ {
		event := ob.buffer[i]
		if i > validStartIndex && event.FirstUpdateID != ob.lastUpdateID+1 {
			ob.buffer = ob.buffer[:0]
			return fmt.Errorf("backpack orderbook: buffered gap prev=%d next=%d", ob.lastUpdateID, event.FirstUpdateID)
		}
		ob.applyEvent(event)
	}

	ob.buffer = ob.buffer[:0]
	return nil
}

func (ob *OrderBook) applyEvent(event *sdk.DepthEvent) {
	for _, level := range event.Bids {
		if len(level) < 2 {
			continue
		}
		qty := parseDecimal(level[1])
		if qty.IsZero() {
			delete(ob.bids, level[0])
			continue
		}
		ob.bids[level[0]] = qty
	}
	for _, level := range event.Asks {
		if len(level) < 2 {
			continue
		}
		qty := parseDecimal(level[1])
		if qty.IsZero() {
			delete(ob.asks, level[0])
			continue
		}
		ob.asks[level[0]] = qty
	}

	ob.lastUpdateID = event.FinalUpdateID
	if event.EngineTimestamp > 0 {
		ob.timestamp = microsToMillis(event.EngineTimestamp)
		return
	}
	ob.timestamp = microsToMillis(event.EventTime)
}

func (ob *OrderBook) GetDepth(limit int) ([]exchanges.Level, []exchanges.Level) {
	ob.RLock()
	defer ob.RUnlock()

	bids := make([]exchanges.Level, 0, len(ob.bids))
	for price, qty := range ob.bids {
		bids = append(bids, exchanges.Level{
			Price:    parseDecimal(price),
			Quantity: qty,
		})
	}
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.GreaterThan(bids[j].Price)
	})

	asks := make([]exchanges.Level, 0, len(ob.asks))
	for price, qty := range ob.asks {
		asks = append(asks, exchanges.Level{
			Price:    parseDecimal(price),
			Quantity: qty,
		})
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

func (ob *OrderBook) WaitReady(ctx context.Context, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
			if ob.IsInitialized() {
				return true
			}
		}
	}
}

func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}
