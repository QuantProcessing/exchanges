
package edgex

import (
	"context"
	"sort"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/edgex/sdk/perp"

	"github.com/shopspring/decimal"
)

type OrderBook struct {
	sync.RWMutex
	symbol      string
	bids        map[string]decimal.Decimal
	asks        map[string]decimal.Decimal
	initialized bool
	readyChan   chan struct{}
	readyOnce   sync.Once
	timestamp   int64
}

func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol:    symbol,
		bids:      make(map[string]decimal.Decimal),
		asks:      make(map[string]decimal.Decimal),
		readyChan: make(chan struct{}),
		readyOnce: sync.Once{},
	}
}

// ProcessPerpUpdate processes perp depth event
func (ob *OrderBook) ProcessPerpUpdate(e *perp.WsDepthEvent) {
	if len(e.Content.Data) == 0 {
		return
	}
	d := e.Content.Data[0]

	ob.Lock()
	defer ob.Unlock()

	ob.timestamp = time.Now().UnixMilli()

	switch d.DepthType {
	case "SNAPSHOT":
		ob.bids = make(map[string]decimal.Decimal)
		ob.asks = make(map[string]decimal.Decimal)
		for _, b := range d.Bids {
			p := parseEdgexFloat(b.Price)
			s := parseEdgexFloat(b.Size)
			if s.IsPositive() {
				ob.bids[p.String()] = s
			}
		}
		for _, as := range d.Asks {
			p := parseEdgexFloat(as.Price)
			s := parseEdgexFloat(as.Size)
			if s.IsPositive() {
				ob.asks[p.String()] = s
			}
		}
		ob.initialized = true
		ob.readyOnce.Do(func() {
			close(ob.readyChan)
		})
	case "CHANGED":
		if !ob.initialized {
			return
		}
		for _, b := range d.Bids {
			p := parseEdgexFloat(b.Price)
			s := parseEdgexFloat(b.Size)
			if s.IsZero() {
				delete(ob.bids, p.String())
			} else {
				ob.bids[p.String()] = s
			}
		}
		for _, as := range d.Asks {
			p := parseEdgexFloat(as.Price)
			s := parseEdgexFloat(as.Size)
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

func (ob *OrderBook) GetBestBid() (decimal.Decimal, decimal.Decimal) {
	ob.RLock()
	defer ob.RUnlock()
	bestPrice, bestQty := decimal.Zero, decimal.Zero
	first := true
	for pStr, q := range ob.bids {
		p, _ := decimal.NewFromString(pStr)
		if first || p.GreaterThan(bestPrice) {
			bestPrice, bestQty = p, q
			first = false
		}
	}
	return bestPrice, bestQty
}

func (ob *OrderBook) GetBestAsk() (decimal.Decimal, decimal.Decimal) {
	ob.RLock()
	defer ob.RUnlock()
	bestPrice, bestQty := decimal.Zero, decimal.Zero
	first := true
	for pStr, q := range ob.asks {
		p, _ := decimal.NewFromString(pStr)
		if first || p.LessThan(bestPrice) {
			bestPrice, bestQty = p, q
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

func (ob *OrderBook) IsInitialized() bool {
	ob.RLock()
	defer ob.RUnlock()
	return ob.initialized
}

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
