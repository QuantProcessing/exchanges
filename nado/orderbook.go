package nado

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/QuantProcessing/exchanges/nado/sdk"

	"github.com/shopspring/decimal"

)

// OrderBook maintains local orderbook replica for Nado
// It strictly follows the official documentation for synchronization:
// 1. Buffer events if not initialized.
// 2. On snapshot, apply it and then replay buffered events with MaxTimestamp > SnapshotTimestamp.
// 3. Ensure continuity of LastMaxTimestamp for incremental updates.
type OrderBook struct {
	sync.RWMutex
	symbol       string
	bids         map[string]decimal.Decimal
	asks         map[string]decimal.Decimal
	timestamp    int64 // Current state timestamp (ms)
	maxTimestamp int64 // The MaxTimestamp of the last applied event (ns)
	initialized  bool
	buffer       []*nado.OrderBook
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make(map[string]decimal.Decimal),
		asks:   make(map[string]decimal.Decimal),
		buffer: make([]*nado.OrderBook, 0, 1000),
	}
}

// Reset clears orderbook state for reinitialization
func (ob *OrderBook) Reset() {
	ob.Lock()
	defer ob.Unlock()
	ob.reset()
}

func (ob *OrderBook) reset() {
	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)
	ob.maxTimestamp = 0
	ob.initialized = false
	// We keep the buffer capacity but reset length
	ob.buffer = ob.buffer[:0]
}

func (ob *OrderBook) IsInitialized() bool {
	ob.RLock()
	defer ob.RUnlock()
	return ob.initialized
}

// ProcessUpdate processes WebSocket incremental updates
func (ob *OrderBook) ProcessUpdate(u *nado.OrderBook) error {
	ob.Lock()
	defer ob.Unlock()

	// 1. If not initialized, buffer and return
	if !ob.initialized {
		ob.buffer = append(ob.buffer, u)
		return nil
	}

	// 2. Check continuity
	var msgLastMaxTS, msgMaxTS int64
	if u.LastMaxTimestamp != "" {
		msgLastMaxTS, _ = strconv.ParseInt(u.LastMaxTimestamp, 10, 64)
	}
	if u.MaxTimestamp != "" {
		msgMaxTS, _ = strconv.ParseInt(u.MaxTimestamp, 10, 64)
	}

	// If this event is older than or equal to what we have, ignore it.
	if msgMaxTS <= ob.maxTimestamp {
		return nil
	}

	// Continuity check:
	// We expect the event to link to our current state (msgLastMaxTS == ob.maxTimestamp).
	// However, we strictly forbid "future" gaps (msgLastMaxTS > ob.maxTimestamp), which means we missed a packet.
	// We ALLOW overlaps (msgLastMaxTS < ob.maxTimestamp) because for an absolute-value-replacement orderbook (map based),
	// applying a delta that starts from an older state but brings us to a newer state (msgMaxTS > ob.maxTimestamp) is generally safe/idempotent,
	// especially common after snapshotting or concurrent buffering.
	if msgLastMaxTS > ob.maxTimestamp {
		// Real Gap detected!
		// TODO: logger.Warn("Nado orderbook gap detected",
			// zap.String("symbol", ob.symbol),
			// zap.Int64("local_last_max_ts", ob.maxTimestamp),
			// zap.Int64("msg_last_max_ts", msgLastMaxTS),
			// zap.Int64("msg_max_ts", msgMaxTS),
			// )

		// Reset state to force resync
		ob.reset()
		// Buffer this event as it is part of the future stream
		ob.buffer = append(ob.buffer, u)

		return fmt.Errorf("gap detected: local=%d < remote_prev=%d", ob.maxTimestamp, msgLastMaxTS)
	}

	// 3. Apply update
	ob.applyUpdateLocked(u)
	return nil
}

// ApplySnapshot applies HTTP snapshot and replays buffered events
func (ob *OrderBook) ApplySnapshot(snap *nado.MarketLiquidity) error {
	ob.Lock()
	defer ob.Unlock()

	var snapTS int64
	if snap != nil && snap.Timestamp != "" {
		snapTS, _ = strconv.ParseInt(snap.Timestamp, 10, 64)
	}

	if snapTS <= 0 {
		return fmt.Errorf("invalid snapshot timestamp: %v", snapTS)
	}

	// 1. Reset maps but KEEP the buffer (since we need to replay it)
	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)

	// 2. Apply snapshot data
	for _, b := range snap.Bids {
		if len(b) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(b[0], 64)
		s, _ := strconv.ParseFloat(b[1], 64)
		price := smartScale(p)
		size := smartScale(s)
		if size.IsPositive() {
			ob.bids[price.String()] = size
		}
	}
	for _, as := range snap.Asks {
		if len(as) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(as[0], 64)
		s, _ := strconv.ParseFloat(as[1], 64)
		price := smartScale(p)
		size := smartScale(s)
		if size.IsPositive() {
			ob.asks[price.String()] = size
		}
	}

	// Set baseline timestamp
	ob.maxTimestamp = snapTS
	ob.timestamp = snapTS / 1e6

	// 3. Replay buffered events strictly newer than snapshot
	appliedCount := 0
	for _, u := range ob.buffer {
		var msgMaxTS int64
		if u.MaxTimestamp != "" {
			msgMaxTS, _ = strconv.ParseInt(u.MaxTimestamp, 10, 64)
		}

		if msgMaxTS > ob.maxTimestamp {
			// Note: We do NOT check LastMaxTimestamp continuity against Snapshot timestamp here
			// because the snapshot typically breaks the chain of "sequence IDs" or "timestamps"
			// unless we are extremely lucky. The logic is "Snapshot is base, any event NEWER than snapshot applies on top".
			// This assumes standard orderbook logic where events are absolute diffs or the snapshot is merely a checkpoint.
			// Nado documentation simply says: "Apply events with max_timestamp > snapshot timestamp."
			ob.applyUpdateLocked(u)
			appliedCount++
		}
	}

	// Clear buffer after replay
	ob.buffer = ob.buffer[:0]
	ob.initialized = true

	// TODO: logger.Info("Nado orderbook snapshot moved to initialized state",
		// zap.String("symbol", ob.symbol),
		// zap.Int64("snapshot_ts", snapTS),
		// zap.Int64("final_max_ts", ob.maxTimestamp),
		// zap.Int("buffered_applied", appliedCount),
	// )

	return nil
}

func (ob *OrderBook) applyUpdateLocked(u *nado.OrderBook) {
	var msgMaxTS int64
	if u.MaxTimestamp != "" {
		msgMaxTS, _ = strconv.ParseInt(u.MaxTimestamp, 10, 64)
	}

	// Apply bids
	for _, b := range u.Bids {
		if len(b) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(b[0], 64)
		s, _ := strconv.ParseFloat(b[1], 64)
		price := smartScale(p)
		size := smartScale(s)
		if size.IsZero() {
			delete(ob.bids, price.String())
		} else {
			ob.bids[price.String()] = size
		}
	}

	// Apply asks
	for _, as := range u.Asks {
		if len(as) < 2 {
			continue
		}
		p, _ := strconv.ParseFloat(as[0], 64)
		s, _ := strconv.ParseFloat(as[1], 64)
		price := smartScale(p)
		size := smartScale(s)
		if size.IsZero() {
			delete(ob.asks, price.String())
		} else {
			ob.asks[price.String()] = size
		}
	}

	// Advance timestamp
	if msgMaxTS > ob.maxTimestamp {
		ob.maxTimestamp = msgMaxTS
		ob.timestamp = msgMaxTS / 1e6
	}
}

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

func (ob *OrderBook) ToAdapterOrderBook(depth int) *exchanges.OrderBook {
	bids, asks := ob.GetDepth(depth)
	return &exchanges.OrderBook{
		Symbol:    ob.symbol,
		Timestamp: ob.timestamp,
		Bids:      bids,
		Asks:      asks,
	}
}

// smartScale handles Nado's specific scaling if needed (kept from original)
func smartScale(v float64) decimal.Decimal {
	if v > 1e12 {
		return decimal.NewFromFloat(v / 1e18)
	}
	return decimal.NewFromFloat(v)
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

// Timestamp satisfies the LocalOrderBook interface
func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
