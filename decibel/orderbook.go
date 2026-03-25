package decibel

import (
	"context"
	"sort"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	decibelws "github.com/QuantProcessing/exchanges/decibel/sdk/ws"
	"github.com/shopspring/decimal"
)

type OrderBook struct {
	mu          sync.RWMutex
	symbol      string
	bids        map[string]decimal.Decimal
	asks        map[string]decimal.Decimal
	timestamp   int64
	initialized bool
	readyChan   chan struct{}
	readyOnce   sync.Once
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol:    symbol,
		bids:      make(map[string]decimal.Decimal),
		asks:      make(map[string]decimal.Decimal),
		readyChan: make(chan struct{}),
	}
}

func (ob *OrderBook) ProcessDepth(msg decibelws.MarketDepthMessage) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if !ob.initialized && msg.IsDelta() {
		return
	}

	if !ob.initialized || !msg.IsDelta() {
		ob.bids = make(map[string]decimal.Decimal, len(msg.Bids))
		ob.asks = make(map[string]decimal.Decimal, len(msg.Asks))
	}
	applyDepthLevels(ob.bids, msg.Bids)
	applyDepthLevels(ob.asks, msg.Asks)

	if msg.Timestamp > 0 {
		ob.timestamp = msg.Timestamp
	} else {
		ob.timestamp = time.Now().UnixMilli()
	}

	ob.initialized = true
	ob.readyOnce.Do(func() {
		close(ob.readyChan)
	})
}

func applyDepthLevels(book map[string]decimal.Decimal, levels []decibelws.DepthLevel) {
	for _, level := range levels {
		key := level.Price.String()
		if !level.Size.IsPositive() {
			delete(book, key)
			continue
		}
		book[key] = level.Size
	}
}

func (ob *OrderBook) GetDepth(limit int) ([]exchanges.Level, []exchanges.Level) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]exchanges.Level, 0, len(ob.bids))
	for priceStr, quantity := range ob.bids {
		price, err := decimal.NewFromString(priceStr)
		if err != nil {
			continue
		}
		bids = append(bids, exchanges.Level{Price: price, Quantity: quantity})
	}
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.GreaterThan(bids[j].Price)
	})

	asks := make([]exchanges.Level, 0, len(ob.asks))
	for priceStr, quantity := range ob.asks {
		price, err := decimal.NewFromString(priceStr)
		if err != nil {
			continue
		}
		asks = append(asks, exchanges.Level{Price: price, Quantity: quantity})
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
		Bids:      bids,
		Asks:      asks,
		Timestamp: ob.Timestamp(),
	}
}

func (ob *OrderBook) WaitReady(ctx context.Context, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ob.readyChan:
		return true
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

func (ob *OrderBook) Timestamp() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.timestamp
}

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
