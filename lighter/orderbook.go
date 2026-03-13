package lighter

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	exchanges "github.com/QuantProcessing/exchanges"

	"github.com/shopspring/decimal"
)

type OrderBook struct {
	sync.RWMutex
	symbol      string
	bids        map[string]decimal.Decimal
	asks        map[string]decimal.Decimal
	lastNonce   int64
	initialized bool
	readyChan   chan struct{}
	readyOnce   sync.Once
	timestamp   int64
}

// Timestamp satisfies the LocalOrderBook interface
func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}

type WsOrderBookUpdate struct {
	Type      string `json:"type"`
	OrderBook struct {
		BeginNonce *int64 `json:"begin_nonce,omitempty"`
		Nonce      int64  `json:"nonce"`
		Timestamp  int64  `json:"timestamp"` // exchange server time (ms)
		Bids       []struct {
			Price string `json:"price"`
			Size  string `json:"size"`
		} `json:"bids"`
		Asks []struct {
			Price string `json:"price"`
			Size  string `json:"size"`
		} `json:"asks"`
	} `json:"order_book"`
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		symbol:    symbol,
		bids:      make(map[string]decimal.Decimal),
		asks:      make(map[string]decimal.Decimal),
		readyChan: make(chan struct{}),
	}
}

func (ob *OrderBook) ProcessUpdate(data []byte) {
	var update WsOrderBookUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return
	}

	ob.Lock()
	defer ob.Unlock()

	if update.OrderBook.Timestamp > 0 {
		ob.timestamp = update.OrderBook.Timestamp
	} else {
		ob.timestamp = time.Now().UnixMilli()
	}

	if !ob.initialized {
		ob.bids = make(map[string]decimal.Decimal)
		ob.asks = make(map[string]decimal.Decimal)
		for _, b := range update.OrderBook.Bids {
			ob.bids[b.Price] = parseLighterFloat(b.Size)
		}
		for _, as := range update.OrderBook.Asks {
			ob.asks[as.Price] = parseLighterFloat(as.Size)
		}
		ob.lastNonce = update.OrderBook.Nonce
		ob.initialized = true
		ob.readyOnce.Do(func() {
			close(ob.readyChan)
		})
	} else {
		if update.OrderBook.BeginNonce != nil && *update.OrderBook.BeginNonce != ob.lastNonce {
			ob.initialized = false // Reset on nonce gap
			return
		}
		for _, b := range update.OrderBook.Bids {
			p := parseLighterFloat(b.Price)
			s := parseLighterFloat(b.Size)
			if s.IsZero() {
				delete(ob.bids, p.String())
			} else {
				ob.bids[p.String()] = s
			}
		}
		for _, as := range update.OrderBook.Asks {
			p := parseLighterFloat(as.Price)
			s := parseLighterFloat(as.Size)
			if s.IsZero() {
				delete(ob.asks, p.String())
			} else {
				ob.asks[p.String()] = s
			}
		}
		ob.lastNonce = update.OrderBook.Nonce
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

var _ exchanges.LocalOrderBook = (*OrderBook)(nil)
