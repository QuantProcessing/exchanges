package binance

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/QuantProcessing/exchanges/model"
)

type bookDelta struct {
	first     int64
	final     int64
	previous  int64
	bids      []model.OrderBookLevel
	asks      []model.OrderBookLevel
	eventTime time.Time
}

type orderBookSnapshot struct {
	sequence  int64
	bids      []model.OrderBookLevel
	asks      []model.OrderBookLevel
	eventTime time.Time
}

type orderBookRuntime struct {
	mu           sync.Mutex
	instrumentID model.InstrumentID
	limit        int
	snapshot     func(context.Context) (orderBookSnapshot, error)
	emit         func(model.OrderBook)

	ready    bool
	sequence int64
	buffer   []bookDelta
	book     localOrderBook
}

func newOrderBookRuntime(id model.InstrumentID, limit int, snapshot func(context.Context) (orderBookSnapshot, error), emit func(model.OrderBook)) *orderBookRuntime {
	return &orderBookRuntime{
		instrumentID: id,
		limit:        limit,
		snapshot:     snapshot,
		emit:         emit,
		book:         newLocalOrderBook(),
	}
}

func (r *orderBookRuntime) BeginBuffer() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = false
	r.buffer = nil
}

func (r *orderBookRuntime) Rebuild(ctx context.Context) error {
	r.mu.Lock()
	if r.ready {
		r.buffer = nil
	}
	r.ready = false
	r.mu.Unlock()

	snapshot, err := r.snapshot(ctx)
	if err != nil {
		return err
	}
	books := r.applySnapshotAndReplay(snapshot)
	for _, book := range books {
		r.emit(book)
	}
	return nil
}

func (r *orderBookRuntime) HandleDelta(ctx context.Context, delta bookDelta) error {
	books, rebuild := r.applyDelta(delta)
	for _, book := range books {
		r.emit(book)
	}
	if rebuild {
		return r.Rebuild(ctx)
	}
	return nil
}

func (r *orderBookRuntime) applySnapshotAndReplay(snapshot orderBookSnapshot) []model.OrderBook {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.book = newLocalOrderBook()
	r.book.Load(snapshot.bids, snapshot.asks)
	r.sequence = snapshot.sequence
	r.ready = true

	books := []model.OrderBook{r.book.Snapshot(r.instrumentID, r.limit, snapshot.eventTime)}
	buffer := r.buffer
	r.buffer = nil

	for _, delta := range buffer {
		applied, _ := r.applyDeltaLocked(delta)
		if applied {
			books = append(books, r.book.Snapshot(r.instrumentID, r.limit, delta.eventTime))
		}
	}
	return books
}

func (r *orderBookRuntime) applyDelta(delta bookDelta) ([]model.OrderBook, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.ready {
		r.buffer = append(r.buffer, delta)
		return nil, false
	}
	applied, gap := r.applyDeltaLocked(delta)
	if gap {
		return nil, true
	}
	if !applied {
		return nil, false
	}
	return []model.OrderBook{r.book.Snapshot(r.instrumentID, r.limit, delta.eventTime)}, false
}

func (r *orderBookRuntime) applyDeltaLocked(delta bookDelta) (applied bool, gap bool) {
	if delta.final <= r.sequence {
		return false, false
	}
	if delta.previous > 0 && delta.previous != r.sequence {
		return false, true
	}
	if delta.first > 0 && delta.first > r.sequence+1 {
		return false, true
	}
	r.book.Apply(delta.bids, delta.asks)
	r.sequence = delta.final
	return true, false
}

type localOrderBook struct {
	bids map[string]model.OrderBookLevel
	asks map[string]model.OrderBookLevel
}

func newLocalOrderBook() localOrderBook {
	return localOrderBook{
		bids: make(map[string]model.OrderBookLevel),
		asks: make(map[string]model.OrderBookLevel),
	}
}

func (b *localOrderBook) Load(bids, asks []model.OrderBookLevel) {
	b.Apply(bids, asks)
}

func (b *localOrderBook) Apply(bids, asks []model.OrderBookLevel) {
	for _, level := range bids {
		applyBookLevel(b.bids, level)
	}
	for _, level := range asks {
		applyBookLevel(b.asks, level)
	}
}

func (b *localOrderBook) Snapshot(id model.InstrumentID, limit int, eventTime time.Time) model.OrderBook {
	bids := sortedBookSide(b.bids, true, limit)
	asks := sortedBookSide(b.asks, false, limit)
	return model.OrderBook{
		InstrumentID: id,
		Bids:         bids,
		Asks:         asks,
		EventTime:    eventTime,
	}
}

func applyBookLevel(side map[string]model.OrderBookLevel, level model.OrderBookLevel) {
	key := level.Price.String()
	if level.Size.IsZero() {
		delete(side, key)
		return
	}
	side[key] = level
}

func sortedBookSide(side map[string]model.OrderBookLevel, desc bool, limit int) []model.OrderBookLevel {
	levels := make([]model.OrderBookLevel, 0, len(side))
	for _, level := range side {
		levels = append(levels, level)
	}
	sort.Slice(levels, func(i, j int) bool {
		cmp := levels[i].Price.Cmp(levels[j].Price)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if limit > 0 && len(levels) > limit {
		return levels[:limit]
	}
	return levels
}
