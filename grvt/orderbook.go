
package grvt

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/shopspring/decimal"
	"github.com/QuantProcessing/exchanges/grvt/sdk"
)

type OrderBook struct {
	sync.RWMutex
	symbol      string
	bids        map[string]decimal.Decimal
	asks        map[string]decimal.Decimal
	initialized bool
	timestamp   int64
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

func (ob *OrderBook) ProcessUpdate(d *grvt.OrderBook) {
	ob.Lock()
	defer ob.Unlock()

	ts, _ := strconv.ParseInt(d.EventTime, 10, 64)
	ob.timestamp = ts / 1000000

	if !ob.initialized {
		ob.bids = make(map[string]decimal.Decimal)
		ob.asks = make(map[string]decimal.Decimal)
		for _, b := range d.Bids {
			p := parseGrvtFloat(b.Price)
			s := parseGrvtFloat(b.Size)
			if s.IsPositive() {
				ob.bids[p.String()] = s
			}
		}
		for _, as := range d.Asks {
			p := parseGrvtFloat(as.Price)
			s := parseGrvtFloat(as.Size)
			if s.IsPositive() {
				ob.asks[p.String()] = s
			}
		}
		ob.initialized = true
	} else {
		for _, b := range d.Bids {
			p := parseGrvtFloat(b.Price)
			s := parseGrvtFloat(b.Size)
			if s.IsZero() {
				delete(ob.bids, p.String())
			} else {
				ob.bids[p.String()] = s
			}
		}
		for _, as := range d.Asks {
			p := parseGrvtFloat(as.Price)
			s := parseGrvtFloat(as.Size)
			if s.IsZero() {
				delete(ob.asks, p.String())
			} else {
				ob.asks[p.String()] = s
			}
		}
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

// IsInitialized returns whether the orderbook has been initialized
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
