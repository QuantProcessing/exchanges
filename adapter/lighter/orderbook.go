package lighter

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
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
	state       orderBookState
	readyChan   chan struct{}
	readyOnce   sync.Once
	timestamp   int64
}

var ErrOrderBookResyncRequired = errors.New("lighter orderbook resync required")

type orderBookState int

const (
	orderBookStateCold orderBookState = iota
	orderBookStateReady
	orderBookStateResyncing
)

// Timestamp satisfies the LocalOrderBook interface
func (ob *OrderBook) Timestamp() int64 {
	ob.RLock()
	defer ob.RUnlock()
	return ob.timestamp
}

type WsOrderBookUpdate struct {
	Type          string `json:"type"`
	Channel       string `json:"channel"`
	Timestamp     int64  `json:"timestamp"`       // server broadcast time (ms)
	LastUpdatedAt int64  `json:"last_updated_at"` // book update time (us)
	OrderBook     struct {
		BeginNonce    *int64 `json:"begin_nonce,omitempty"`
		Nonce         int64  `json:"nonce"`
		Timestamp     int64  `json:"timestamp"`       // server broadcast time (ms)
		LastUpdatedAt int64  `json:"last_updated_at"` // book update time (us)
		Bids          []struct {
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
		state:     orderBookStateCold,
		readyChan: make(chan struct{}),
	}
}

func (ob *OrderBook) ProcessUpdate(data []byte) error {
	var update WsOrderBookUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return err
	}

	ob.Lock()
	defer ob.Unlock()

	ob.timestamp = normalizeLighterOrderBookTimestamp(&update)

	if ob.shouldTreatAsSnapshot(&update) {
		ob.replaceWithSnapshotLocked(&update)
		return nil
	}

	if update.OrderBook.BeginNonce != nil && *update.OrderBook.BeginNonce != ob.lastNonce {
		ob.state = orderBookStateResyncing
		return ErrOrderBookResyncRequired
	}

	ob.applyDeltaLocked(&update)
	return nil
}

func (ob *OrderBook) IsReady() bool {
	ob.RLock()
	defer ob.RUnlock()
	return ob.state == orderBookStateReady
}

func (ob *OrderBook) shouldTreatAsSnapshot(update *WsOrderBookUpdate) bool {
	if !ob.initialized {
		return true
	}
	if strings.HasPrefix(update.Type, "subscribed/") {
		return true
	}
	if ob.state == orderBookStateResyncing {
		return update.OrderBook.BeginNonce == nil
	}
	return false
}

func (ob *OrderBook) replaceWithSnapshotLocked(update *WsOrderBookUpdate) {
	ob.bids = make(map[string]decimal.Decimal)
	ob.asks = make(map[string]decimal.Decimal)

	for _, b := range update.OrderBook.Bids {
		size := parseLighterFloat(b.Size)
		if size.IsZero() {
			continue
		}
		ob.bids[normalizeLighterBookPrice(b.Price)] = size
	}
	for _, as := range update.OrderBook.Asks {
		size := parseLighterFloat(as.Size)
		if size.IsZero() {
			continue
		}
		ob.asks[normalizeLighterBookPrice(as.Price)] = size
	}

	ob.lastNonce = update.OrderBook.Nonce
	ob.initialized = true
	ob.state = orderBookStateReady
	ob.readyOnce.Do(func() {
		close(ob.readyChan)
	})
}

func (ob *OrderBook) applyDeltaLocked(update *WsOrderBookUpdate) {
	for _, b := range update.OrderBook.Bids {
		p := normalizeLighterBookPrice(b.Price)
		s := parseLighterFloat(b.Size)
		if s.IsZero() {
			delete(ob.bids, p)
		} else {
			ob.bids[p] = s
		}
	}
	for _, as := range update.OrderBook.Asks {
		p := normalizeLighterBookPrice(as.Price)
		s := parseLighterFloat(as.Size)
		if s.IsZero() {
			delete(ob.asks, p)
		} else {
			ob.asks[p] = s
		}
	}
	ob.lastNonce = update.OrderBook.Nonce
	ob.state = orderBookStateReady
}

func normalizeLighterOrderBookTimestamp(update *WsOrderBookUpdate) int64 {
	switch {
	case update.LastUpdatedAt > 0:
		return update.LastUpdatedAt / 1000
	case update.OrderBook.LastUpdatedAt > 0:
		return update.OrderBook.LastUpdatedAt / 1000
	case update.OrderBook.Timestamp > 0:
		return update.OrderBook.Timestamp
	case update.Timestamp > 0:
		return update.Timestamp
	default:
		return time.Now().UnixMilli()
	}
}

func normalizeLighterBookPrice(raw string) string {
	return parseLighterFloat(raw).String()
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
