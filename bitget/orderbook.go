package bitget

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/bitget/sdk"
	"github.com/shopspring/decimal"
)

type OrderBook struct {
	symbol      string
	bids        map[string]decimal.Decimal
	asks        map[string]decimal.Decimal
	lastSeq     int64
	timestamp   int64
	initialized bool
	mu          sync.RWMutex
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make(map[string]decimal.Decimal),
		asks:   make(map[string]decimal.Decimal),
	}
}

func (ob *OrderBook) ProcessUpdate(action string, data *sdk.WSOrderBookData) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	switch action {
	case "snapshot":
		ob.bids = make(map[string]decimal.Decimal)
		ob.asks = make(map[string]decimal.Decimal)
		ob.applyLevels(ob.bids, data.Bids)
		ob.applyLevels(ob.asks, data.Asks)
		ob.lastSeq = data.Seq
		ob.timestamp = parseMillis(data.TS)
		ob.initialized = true
		return nil
	case "update":
		if !ob.initialized {
			return fmt.Errorf("bitget orderbook: update before snapshot")
		}
		if data.PSeq == 0 || data.PSeq != ob.lastSeq {
			ob.initialized = false
			return fmt.Errorf("bitget orderbook: sequence gap prev=%d pseq=%d seq=%d", ob.lastSeq, data.PSeq, data.Seq)
		}
		ob.applyLevels(ob.bids, data.Bids)
		ob.applyLevels(ob.asks, data.Asks)
		ob.lastSeq = data.Seq
		ob.timestamp = parseMillis(data.TS)
		return nil
	default:
		return fmt.Errorf("bitget orderbook: unsupported action %q", action)
	}
}

func (ob *OrderBook) LoadSnapshot(data *sdk.OrderBook) {
	if data == nil {
		return
	}
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)
	ob.applyLevels(ob.bids, data.Bids)
	ob.applyLevels(ob.asks, data.Asks)
	ob.lastSeq = 0
	ob.timestamp = parseMillis(data.TS)
	ob.initialized = true
}

func (ob *OrderBook) applyLevels(side map[string]decimal.Decimal, levels [][]sdk.NumberString) {
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		price := string(level[0])
		qty := parseDecimal(string(level[1]))
		if qty.IsZero() {
			delete(side, price)
			continue
		}
		side[price] = qty
	}
}

func (ob *OrderBook) GetDepth(limit int) ([]exchanges.Level, []exchanges.Level) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

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
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.timestamp
}

func (ob *OrderBook) IsInitialized() bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.initialized
}

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
